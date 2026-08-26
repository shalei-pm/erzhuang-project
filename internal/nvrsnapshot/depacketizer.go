// Package nvrsnapshot contains protocol-only pieces used by the one-shot NVR
// thumbnail backfill runner. It intentionally has no network, process, or
// persistence dependencies.
package nvrsnapshot

const (
	rtpVersion           = 2
	rtpHeaderSize        = 12
	h265VideoPayloadType = 96
	h265APType           = 48
	h265FUType           = 49
	h265PACIType         = 50
	defaultMaxFUBytes    = 4 << 20
)

// ErrorCode is safe to report in job summaries. Protocol helpers never attach
// packet data, stream URLs, or upstream responses to an error code.
type ErrorCode string

const (
	ErrorDemuxFailed  ErrorCode = "demux_failed"
	ErrorMediaTimeout ErrorCode = "media_timeout"
)

// RTPPacket is the restricted RTP subset accepted by this capture path.
// Payload aliases the input packet and must be consumed immediately.
type RTPPacket struct {
	SequenceNumber uint16
	Timestamp      uint32
	Marker         bool
	Payload        []byte
}

// DepacketizedNAL is a completed Annex-B NAL together with the RTP access
// unit metadata carried by the packet that completed it.
type DepacketizedNAL struct {
	NAL       []byte
	Timestamp uint32
	Marker    bool
}

// ParseRTP accepts only RTP v2 packets with the fixed 12-byte header emitted
// by the verified NVR stream. RTP header extensions, padding, and CSRC lists
// are deliberately unsupported rather than guessed at.
func ParseRTP(packet []byte) (RTPPacket, ErrorCode) {
	if len(packet) < rtpHeaderSize || packet[0]>>6 != rtpVersion {
		return RTPPacket{}, ErrorDemuxFailed
	}

	const (
		paddingBit   = 0x20
		extensionBit = 0x10
		csrcMask     = 0x0f
		payloadMask  = 0x7f
	)
	if packet[0]&(paddingBit|extensionBit|csrcMask) != 0 || packet[1]&payloadMask != h265VideoPayloadType {
		return RTPPacket{}, ErrorDemuxFailed
	}
	if len(packet) == rtpHeaderSize {
		return RTPPacket{}, ErrorDemuxFailed
	}

	return RTPPacket{
		SequenceNumber: uint16(packet[2])<<8 | uint16(packet[3]),
		Timestamp:      uint32(packet[4])<<24 | uint32(packet[5])<<16 | uint32(packet[6])<<8 | uint32(packet[7]),
		Marker:         packet[1]&0x80 != 0,
		Payload:        packet[rtpHeaderSize:],
	}, ""
}

// Depacketizer reassembles one in-flight H.265 RTP FU at a time. The buffered
// bytes exist only until the matching end fragment is received and are never
// logged, persisted, or exposed by the API.
type Depacketizer struct {
	fuActive    bool
	nextSeq     uint16
	fuIndicator [2]byte
	fuType      byte
	fuTimestamp uint32
	buffer      []byte
	maxBytes    int
}

// NewDepacketizer creates an empty H.265 RTP depacketizer.
func NewDepacketizer() *Depacketizer {
	return NewDepacketizerWithMaxBytes(defaultMaxFUBytes)
}

// NewDepacketizerWithMaxBytes creates an H.265 RTP depacketizer whose
// NAL cannot exceed maxBytes. A zero limit rejects every NAL.
func NewDepacketizerWithMaxBytes(maxBytes int) *Depacketizer {
	if maxBytes < 0 {
		maxBytes = 0
	}
	return &Depacketizer{maxBytes: maxBytes}
}

// FeedRTP converts one RTP packet into a complete Annex-B H.265 NAL. A nil
// NAL and an empty code means that an FU is still being assembled.
func (d *Depacketizer) FeedRTP(packet []byte) ([]byte, ErrorCode) {
	result, code := d.FeedRTPWithMetadata(packet)
	return result.NAL, code
}

// FeedRTPWithMetadata converts one RTP packet into a completed Annex-B H.265
// NAL and preserves the timestamp and marker needed to assemble access units.
// A nil NAL and an empty code means that an FU is still being assembled.
func (d *Depacketizer) FeedRTPWithMetadata(packet []byte) (DepacketizedNAL, ErrorCode) {
	rtp, code := ParseRTP(packet)
	if code != "" {
		d.reset()
		return DepacketizedNAL{}, code
	}
	nal, code := d.feedH265(rtp.SequenceNumber, rtp.Timestamp, rtp.Payload)
	if code != "" {
		return DepacketizedNAL{}, code
	}
	return DepacketizedNAL{NAL: nal, Timestamp: rtp.Timestamp, Marker: rtp.Marker}, ""
}

func (d *Depacketizer) feedH265(sequence uint16, timestamp uint32, payload []byte) ([]byte, ErrorCode) {
	if len(payload) < 2 {
		d.reset()
		return nil, ErrorDemuxFailed
	}

	nalType := (payload[0] >> 1) & 0x3f
	if nalType == h265APType || nalType == h265PACIType {
		d.reset()
		return nil, ErrorDemuxFailed
	}
	if nalType != h265FUType {
		if d.fuActive {
			d.reset()
		}
		if !d.fitsNALBytes(0, len(payload)) {
			return nil, ErrorDemuxFailed
		}
		return annexB(payload), ""
	}

	// FU payload is a two-byte indicator, one-byte FU header, then media data.
	if len(payload) < 4 {
		d.reset()
		return nil, ErrorDemuxFailed
	}

	fuHeader := payload[2]
	start := fuHeader&0x80 != 0
	end := fuHeader&0x40 != 0
	fuType := fuHeader & 0x3f
	fragment := payload[3:]

	if start {
		if d.fuActive {
			d.reset()
		}
		if fuType == h265APType || fuType == h265FUType || fuType == h265PACIType {
			d.reset()
			return nil, ErrorDemuxFailed
		}
		if !d.fitsNALBytes(2, len(fragment)) {
			d.reset()
			return nil, ErrorDemuxFailed
		}
		// This matches the NVRPlayer reconstruction formula. The second NAL
		// header byte is preserved from the FU indicator.
		d.buffer = make([]byte, 0, 2+len(fragment))
		d.buffer = append(d.buffer, (fuType<<1)|(payload[0]&0x81), payload[1])
		d.buffer = append(d.buffer, fragment...)
		d.fuActive = true
		d.nextSeq = sequence + 1
		d.fuIndicator = [2]byte{payload[0], payload[1]}
		d.fuType = fuType
		d.fuTimestamp = timestamp
		if !end {
			return nil, ""
		}
		return d.finishFU()
	}

	if !d.fuActive || sequence != d.nextSeq {
		d.reset()
		return nil, ErrorDemuxFailed
	}
	if payload[0] != d.fuIndicator[0] || payload[1] != d.fuIndicator[1] || fuType != d.fuType || timestamp != d.fuTimestamp {
		d.reset()
		return nil, ErrorDemuxFailed
	}
	if !d.fitsNALBytes(len(d.buffer), len(fragment)) {
		d.reset()
		return nil, ErrorDemuxFailed
	}

	d.buffer = append(d.buffer, fragment...)
	d.nextSeq = sequence + 1
	if !end {
		return nil, ""
	}
	return d.finishFU()
}

func (d *Depacketizer) fitsNALBytes(prefix, suffix int) bool {
	limit := d.maxBytes
	maxAnnexBNALBytes := int(^uint(0)>>1) - 4
	if limit > maxAnnexBNALBytes {
		limit = maxAnnexBNALBytes
	}
	return prefix >= 0 && prefix <= limit && suffix >= 0 && suffix <= limit-prefix
}

func (d *Depacketizer) finishFU() ([]byte, ErrorCode) {
	nal := annexB(d.buffer)
	d.reset()
	return nal, ""
}

func (d *Depacketizer) reset() {
	clear(d.buffer)
	d.fuActive = false
	d.nextSeq = 0
	d.fuIndicator = [2]byte{}
	d.fuType = 0
	d.fuTimestamp = 0
	d.buffer = nil
}

func annexB(nal []byte) []byte {
	output := make([]byte, 4+len(nal))
	output[3] = 1
	copy(output[4:], nal)
	return output
}
