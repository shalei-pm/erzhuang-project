package nvrsnapshot

import (
	"bytes"
	"testing"
)

func TestDepacketizerReturnsAnnexBForSingleNAL(t *testing.T) {
	d := NewDepacketizer()
	packet := rtpPacket(7, []byte{0x42, 0x01, 0xaa, 0xbb})

	got, code := d.FeedRTP(packet)
	if code != "" {
		t.Fatalf("FeedRTP() code = %q", code)
	}
	want := []byte{0, 0, 0, 1, 0x42, 0x01, 0xaa, 0xbb}
	if !bytes.Equal(got, want) {
		t.Fatalf("FeedRTP() = %x, want %x", got, want)
	}
}

func TestDepacketizerExposesMarkerAndTimestampWithoutChangingFeedRTP(t *testing.T) {
	d := NewDepacketizer()
	packet := rtpTimestampPacket(7, 0x10203040, true, []byte{0x42, 0x01, 0xaa})

	result, code := d.FeedRTPWithMetadata(packet)
	if code != "" {
		t.Fatalf("FeedRTPWithMetadata() code = %q", code)
	}
	if !result.Marker || result.Timestamp != 0x10203040 {
		t.Fatalf("metadata = %+v, want marker and timestamp", result)
	}
	want := []byte{0, 0, 0, 1, 0x42, 0x01, 0xaa}
	if !bytes.Equal(result.NAL, want) {
		t.Fatalf("NAL = %x, want %x", result.NAL, want)
	}

	got, code := NewDepacketizer().FeedRTP(packet)
	if code != "" || !bytes.Equal(got, want) {
		t.Fatalf("FeedRTP() = %x, %q; want %x, empty", got, code, want)
	}
}

func TestDepacketizerRejectsUnsupportedAggregationPacketsAndRecovers(t *testing.T) {
	tests := []struct {
		name    string
		nalType byte
	}{
		{name: "aggregation packet", nalType: 48},
		{name: "paci", nalType: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDepacketizer()
			if got, code := d.FeedRTP(rtpPacket(1, []byte{0x62, 0x01, 0x80 | 19, 0xaa})); got != nil || code != "" {
				t.Fatalf("FU start FeedRTP() = %x, %q", got, code)
			}
			if got, code := d.FeedRTP(rtpPacket(2, []byte{tt.nalType << 1, 0x01, 0xaa, 0xbb})); got != nil || code != ErrorDemuxFailed {
				t.Fatalf("unsupported packet FeedRTP() = %x, %q; want nil, %q", got, code, ErrorDemuxFailed)
			}

			got, code := d.FeedRTP(rtpPacket(3, []byte{0x42, 0x01, 0xcc}))
			want := []byte{0, 0, 0, 1, 0x42, 0x01, 0xcc}
			if code != "" || !bytes.Equal(got, want) {
				t.Fatalf("recovered FeedRTP() = %x, %q; want %x, empty", got, code, want)
			}
		})
	}
}

func TestDepacketizerLimitsSingleNALSizeAndRecovers(t *testing.T) {
	d := NewDepacketizerWithMaxBytes(3)

	got, code := d.FeedRTP(rtpPacket(1, []byte{0x42, 0x01, 0xaa}))
	want := []byte{0, 0, 0, 1, 0x42, 0x01, 0xaa}
	if code != "" || !bytes.Equal(got, want) {
		t.Fatalf("limit-sized FeedRTP() = %x, %q; want %x, empty", got, code, want)
	}

	got, code = d.FeedRTP(rtpPacket(2, []byte{0x42, 0x01, 0xbb, 0xcc}))
	if got != nil || code != ErrorDemuxFailed {
		t.Fatalf("oversized FeedRTP() = %x, %q; want nil, %q", got, code, ErrorDemuxFailed)
	}

	got, code = d.FeedRTP(rtpPacket(3, []byte{0x44, 0x01, 0xdd}))
	want = []byte{0, 0, 0, 1, 0x44, 0x01, 0xdd}
	if code != "" || !bytes.Equal(got, want) {
		t.Fatalf("recovered FeedRTP() = %x, %q; want %x, empty", got, code, want)
	}
}

func TestDepacketizerReassemblesH265FU(t *testing.T) {
	d := NewDepacketizer()

	// FU indicator type 49 with an original H.265 IDR type of 19.
	start := rtpPacket(100, []byte{0x62, 0x01, 0x80 | 19, 0xaa})
	if got, code := d.FeedRTP(start); got != nil || code != "" {
		t.Fatalf("start FeedRTP() = %x, %q; want no completed NAL", got, code)
	}
	middle := rtpPacket(101, []byte{0x62, 0x01, 19, 0xbb})
	if got, code := d.FeedRTP(middle); got != nil || code != "" {
		t.Fatalf("middle FeedRTP() = %x, %q; want no completed NAL", got, code)
	}
	end := rtpPacket(102, []byte{0x62, 0x01, 0x40 | 19, 0xcc})

	got, code := d.FeedRTP(end)
	if code != "" {
		t.Fatalf("end FeedRTP() code = %q", code)
	}
	// Restored header follows the NVRPlayer formula: (fuType << 1) | (indicator[0] & 0x81), indicator[1].
	want := []byte{0, 0, 0, 1, 0x26, 0x01, 0xaa, 0xbb, 0xcc}
	if !bytes.Equal(got, want) {
		t.Fatalf("end FeedRTP() = %x, want %x", got, want)
	}
}

func TestDepacketizerRejectsUnsupportedFUInnerTypeAndRecovers(t *testing.T) {
	tests := []struct {
		name   string
		fuType byte
	}{
		{name: "aggregation packet", fuType: h265APType},
		{name: "fragmentation unit", fuType: h265FUType},
		{name: "paci", fuType: h265PACIType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDepacketizer()
			payload := []byte{0x62, 0x01, 0x80 | tt.fuType, 0xaa}
			if got, code := d.FeedRTP(rtpPacket(1, payload)); got != nil || code != ErrorDemuxFailed {
				t.Fatalf("FU start FeedRTP() = %x, %q; want nil, %q", got, code, ErrorDemuxFailed)
			}

			got, code := d.FeedRTP(rtpPacket(2, []byte{0x42, 0x01, 0xbb}))
			want := []byte{0, 0, 0, 1, 0x42, 0x01, 0xbb}
			if code != "" || !bytes.Equal(got, want) {
				t.Fatalf("recovered FeedRTP() = %x, %q; want %x, empty", got, code, want)
			}
		})
	}
}

func TestDepacketizerRejectsFUMetadataMismatchAndRecovers(t *testing.T) {
	tests := []struct {
		name         string
		continuation []byte
	}{
		{
			name:         "fu type",
			continuation: []byte{0x62, 0x01, 0x40 | 20, 0xbb},
		},
		{
			name:         "fu indicator",
			continuation: []byte{0x63, 0x01, 0x40 | 19, 0xbb},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDepacketizer()
			if got, code := d.FeedRTP(rtpPacket(1, []byte{0x62, 0x01, 0x80 | 19, 0xaa})); got != nil || code != "" {
				t.Fatalf("start FeedRTP() = %x, %q", got, code)
			}

			if got, code := d.FeedRTP(rtpPacket(2, tt.continuation)); got != nil || code != ErrorDemuxFailed {
				t.Fatalf("mismatched continuation FeedRTP() = %x, %q; want nil, %q", got, code, ErrorDemuxFailed)
			}

			got, code := d.FeedRTP(rtpPacket(3, []byte{0x42, 0x01, 0xcc}))
			want := []byte{0, 0, 0, 1, 0x42, 0x01, 0xcc}
			if code != "" || !bytes.Equal(got, want) {
				t.Fatalf("recovered FeedRTP() = %x, %q; want %x, empty", got, code, want)
			}
		})
	}
}

func TestDepacketizerAcceptsSequenceNumberWraparound(t *testing.T) {
	d := NewDepacketizer()
	if got, code := d.FeedRTP(rtpPacket(65535, []byte{0x62, 0x01, 0x80 | 19, 0xaa})); got != nil || code != "" {
		t.Fatalf("start FeedRTP() = %x, %q", got, code)
	}

	got, code := d.FeedRTP(rtpPacket(0, []byte{0x62, 0x01, 0x40 | 19, 0xbb}))
	if code != "" {
		t.Fatalf("end FeedRTP() code = %q", code)
	}
	want := []byte{0, 0, 0, 1, 0x26, 0x01, 0xaa, 0xbb}
	if !bytes.Equal(got, want) {
		t.Fatalf("end FeedRTP() = %x, want %x", got, want)
	}
}

func TestDepacketizerAcceptsMarkerBitForVideoPayloadType96(t *testing.T) {
	got, code := NewDepacketizer().FeedRTP(rtpMarkerPacket(1, []byte{0x42, 0x01, 0xaa}))
	if code != "" {
		t.Fatalf("FeedRTP() code = %q", code)
	}
	want := []byte{0, 0, 0, 1, 0x42, 0x01, 0xaa}
	if !bytes.Equal(got, want) {
		t.Fatalf("FeedRTP() = %x, want %x", got, want)
	}
}

func TestDepacketizerAcceptsH265ParameterSetsAndKeyFrames(t *testing.T) {
	tests := []struct {
		name    string
		nalType byte
	}{
		{name: "vps", nalType: 32},
		{name: "sps", nalType: 33},
		{name: "pps", nalType: 34},
		{name: "key-frame-16", nalType: 16},
		{name: "key-frame-17", nalType: 17},
		{name: "key-frame-18", nalType: 18},
		{name: "key-frame-19", nalType: 19},
		{name: "key-frame-20", nalType: 20},
		{name: "key-frame-21", nalType: 21},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := []byte{tt.nalType << 1, 0x01, tt.nalType}
			got, code := NewDepacketizer().FeedRTP(rtpPacket(1, payload))
			if code != "" {
				t.Fatalf("FeedRTP() code = %q", code)
			}
			want := append([]byte{0, 0, 0, 1}, payload...)
			if !bytes.Equal(got, want) {
				t.Fatalf("FeedRTP() = %x, want %x", got, want)
			}
		})
	}
}

func TestDepacketizerOutputsSingleFragmentFUAndClearsInterruptedState(t *testing.T) {
	d := NewDepacketizer()
	if got, code := d.FeedRTP(rtpPacket(1, []byte{0x62, 0x01, 0x80 | 19, 0xaa})); got != nil || code != "" {
		t.Fatalf("old start FeedRTP() = %x, %q", got, code)
	}

	got, code := d.FeedRTP(rtpPacket(2, []byte{0x62, 0x01, 0xc0 | 19, 0xbb}))
	if code != "" {
		t.Fatalf("single FU FeedRTP() code = %q", code)
	}
	want := []byte{0, 0, 0, 1, 0x26, 0x01, 0xbb}
	if !bytes.Equal(got, want) {
		t.Fatalf("single FU FeedRTP() = %x, want %x", got, want)
	}

	got, code = d.FeedRTP(rtpPacket(3, []byte{0x42, 0x01, 0xcc}))
	if code != "" || !bytes.Equal(got, []byte{0, 0, 0, 1, 0x42, 0x01, 0xcc}) {
		t.Fatalf("post-FU FeedRTP() = %x, %q", got, code)
	}
}

func TestDepacketizerReturnsNonFUWhenItInterruptsActiveFU(t *testing.T) {
	d := NewDepacketizer()
	if got, code := d.FeedRTP(rtpPacket(1, []byte{0x62, 0x01, 0x80 | 19, 0xaa})); got != nil || code != "" {
		t.Fatalf("start FeedRTP() = %x, %q", got, code)
	}

	got, code := d.FeedRTP(rtpPacket(2, []byte{0x42, 0x01, 0xbb}))
	want := []byte{0, 0, 0, 1, 0x42, 0x01, 0xbb}
	if code != "" || !bytes.Equal(got, want) {
		t.Fatalf("non-FU FeedRTP() = %x, %q; want %x, empty", got, code, want)
	}

	got, code = d.FeedRTP(rtpPacket(3, []byte{0x44, 0x01, 0xcc}))
	if code != "" || !bytes.Equal(got, []byte{0, 0, 0, 1, 0x44, 0x01, 0xcc}) {
		t.Fatalf("recovered FeedRTP() = %x, %q", got, code)
	}
}

func TestDepacketizerRejectsFUBufferOverLimitAndRecovers(t *testing.T) {
	d := NewDepacketizerWithMaxBytes(3) // Restored NAL header (2) plus one fragment byte fits exactly.
	if got, code := d.FeedRTP(rtpPacket(1, []byte{0x62, 0x01, 0x80 | 19, 0xaa})); got != nil || code != "" {
		t.Fatalf("start FeedRTP() = %x, %q", got, code)
	}

	got, code := d.FeedRTP(rtpPacket(2, []byte{0x62, 0x01, 0x40 | 19, 0xbb}))
	if got != nil || code != ErrorDemuxFailed {
		t.Fatalf("overflow FeedRTP() = %x, %q; want nil, %q", got, code, ErrorDemuxFailed)
	}

	got, code = d.FeedRTP(rtpPacket(3, []byte{0x42, 0x01, 0xcc}))
	want := []byte{0, 0, 0, 1, 0x42, 0x01, 0xcc}
	if code != "" || !bytes.Equal(got, want) {
		t.Fatalf("recovered FeedRTP() = %x, %q; want %x, empty", got, code, want)
	}
}

func TestDepacketizerRejectsFUWithoutStart(t *testing.T) {
	d := NewDepacketizer()

	got, code := d.FeedRTP(rtpPacket(8, []byte{0x62, 0x01, 0x40 | 19, 0xaa}))
	if got != nil || code != ErrorDemuxFailed {
		t.Fatalf("FeedRTP() = %x, %q; want nil, %q", got, code, ErrorDemuxFailed)
	}
}

func TestDepacketizerRejectsInterruptedFU(t *testing.T) {
	d := NewDepacketizer()
	if got, code := d.FeedRTP(rtpPacket(20, []byte{0x62, 0x01, 0x80 | 19, 0xaa})); got != nil || code != "" {
		t.Fatalf("start FeedRTP() = %x, %q", got, code)
	}

	got, code := d.FeedRTP(rtpPacket(22, []byte{0x62, 0x01, 0x40 | 19, 0xbb}))
	if got != nil || code != ErrorDemuxFailed {
		t.Fatalf("end FeedRTP() = %x, %q; want nil, %q", got, code, ErrorDemuxFailed)
	}
}

func TestDepacketizerRejectsFUTimestampMismatchAndRecovers(t *testing.T) {
	d := NewDepacketizer()
	start := rtpTimestampPacket(10, 100, false, []byte{0x62, 0x01, 0x80 | 19, 0xaa})
	if got, code := d.FeedRTP(start); got != nil || code != "" {
		t.Fatalf("FU start = %x, %q", got, code)
	}

	end := rtpTimestampPacket(11, 101, true, []byte{0x62, 0x01, 0x40 | 19, 0xbb})
	if got, code := d.FeedRTP(end); got != nil || code != ErrorDemuxFailed {
		t.Fatalf("timestamp-mismatched FU = %x, %q; want nil, %q", got, code, ErrorDemuxFailed)
	}

	got, code := d.FeedRTP(rtpTimestampPacket(12, 102, true, h265NAL(19, 0xcc)))
	want := []byte{0, 0, 0, 1, 0x26, 0x01, 0xcc}
	if code != "" || !bytes.Equal(got, want) {
		t.Fatalf("recovered FeedRTP() = %x, %q; want %x, empty", got, code, want)
	}
}

func TestDepacketizerRejectsInvalidRTPHeaderAndPayloadType(t *testing.T) {
	tests := []struct {
		name   string
		packet []byte
	}{
		{name: "short", packet: []byte{0x80, 96}},
		{name: "wrong version", packet: append([]byte{0x40, 96}, make([]byte, 10)...)},
		{name: "csrc", packet: append([]byte{0x81, 96}, make([]byte, 10)...)},
		{name: "extension", packet: append([]byte{0x90, 96}, make([]byte, 10)...)},
		{name: "padding", packet: append([]byte{0xa0, 96}, make([]byte, 10)...)},
		{name: "wrong payload type", packet: append([]byte{0x80, 97}, make([]byte, 10)...)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, code := NewDepacketizer().FeedRTP(tt.packet)
			if got != nil || code != ErrorDemuxFailed {
				t.Fatalf("FeedRTP() = %x, %q; want nil, %q", got, code, ErrorDemuxFailed)
			}
		})
	}
}

func TestDepacketizerRejectsShortFU(t *testing.T) {
	got, code := NewDepacketizer().FeedRTP(rtpPacket(1, []byte{0x62, 0x01, 0x80 | 19}))
	if got != nil || code != ErrorDemuxFailed {
		t.Fatalf("FeedRTP() = %x, %q; want nil, %q", got, code, ErrorDemuxFailed)
	}
}

func rtpPacket(sequence uint16, payload []byte) []byte {
	packet := make([]byte, 12+len(payload))
	packet[0] = 0x80 // RTP v2, no padding, extension, or CSRC entries.
	packet[1] = 96
	packet[2] = byte(sequence >> 8)
	packet[3] = byte(sequence)
	copy(packet[12:], payload)
	return packet
}

func rtpMarkerPacket(sequence uint16, payload []byte) []byte {
	packet := rtpPacket(sequence, payload)
	packet[1] |= 0x80
	return packet
}

func rtpTimestampPacket(sequence uint16, timestamp uint32, marker bool, payload []byte) []byte {
	packet := rtpPacket(sequence, payload)
	packet[4] = byte(timestamp >> 24)
	packet[5] = byte(timestamp >> 16)
	packet[6] = byte(timestamp >> 8)
	packet[7] = byte(timestamp)
	if marker {
		packet[1] |= 0x80
	}
	return packet
}
