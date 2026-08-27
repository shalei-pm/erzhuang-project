import { describe, expect, it, vi } from "vitest";
import NVRPlayer, { parseRTPPacket } from "../vendor/nvr-player/nvr-player.js";

function rtpPacket({
  sequenceNumber,
  timestamp,
  marker = false,
  payload,
  extension = false,
}: {
  sequenceNumber: number;
  timestamp: number;
  marker?: boolean;
  payload: number[];
  extension?: boolean;
}) {
  const headerLength = extension ? 20 : 12;
  const packet = new Uint8Array(headerLength + payload.length);
  packet[0] = 0x80 | (extension ? 0x10 : 0);
  packet[1] = (marker ? 0x80 : 0) | 96;
  packet[2] = sequenceNumber >>> 8;
  packet[3] = sequenceNumber & 0xff;
  packet[4] = timestamp >>> 24;
  packet[5] = (timestamp >>> 16) & 0xff;
  packet[6] = (timestamp >>> 8) & 0xff;
  packet[7] = timestamp & 0xff;
  if (extension) {
    packet[12] = 0xbe;
    packet[13] = 0xde;
    packet[14] = 0;
    packet[15] = 1;
    packet.set([1, 2, 3, 4], 16);
  }
  packet.set(payload, headerLength);
  return packet.buffer;
}

function testPlayer() {
  const player = Object.create(NVRPlayer.prototype) as Record<string, unknown>;
  Object.assign(player, {
    decoder: { state: "configured" },
    VIDEO_PAYLOAD_TYPE: 96,
    AUDIO_PAYLOAD_TYPE: 8,
    decoderConfigured: true,
    waitingForKeyFrame: false,
    needKeyFrame: false,
    frameIndex: 0,
    baseRtpTimestamp: null,
    fuBuffer: null,
    accessUnit: null,
    h265Params: new Map(),
    h265ParamSets: null,
    diagnostics: {
      videoPayloadPackets: 0,
      audioPayloadPackets: 0,
      otherPayloadPackets: 0,
      keyFrameNALUnits: 0,
      markerPackets: 0,
      accessUnits: 0,
      multiNALAccessUnits: 0,
      droppedAccessUnits: 0,
      malformedFuPackets: 0,
      decoderInputFrames: 0,
    },
    _publishDiagnostics: vi.fn(),
    _safeDecode: vi.fn(),
  });
  return player as unknown as NVRPlayer & { _safeDecode: ReturnType<typeof vi.fn> };
}

describe("NVR RTP parsing", () => {
  it("skips a valid RTP extension instead of treating it as H.265 payload", () => {
    const packet = parseRTPPacket(rtpPacket({ sequenceNumber: 4, timestamp: 9000, extension: true, payload: [2, 1, 0xaa] }));

    expect(packet).toMatchObject({ payloadType: 96, marker: false, sequenceNumber: 4, timestamp: 9000 });
    expect([...packet!.payload]).toEqual([2, 1, 0xaa]);
  });

  it("waits for the RTP marker before decoding all NAL units from one access unit", () => {
    const player = testPlayer();
    const first = parseRTPPacket(rtpPacket({ sequenceNumber: 10, timestamp: 90_000, payload: [2, 1, 0xaa] }))!;
    const last = parseRTPPacket(rtpPacket({ sequenceNumber: 11, timestamp: 90_000, marker: true, payload: [2, 1, 0xbb] }))!;

    (player as unknown as { _decodeRTP: (packet: ReturnType<typeof parseRTPPacket>) => void })._decodeRTP(first);
    expect(player._safeDecode).not.toHaveBeenCalled();

    (player as unknown as { _decodeRTP: (packet: ReturnType<typeof parseRTPPacket>) => void })._decodeRTP(last);
    expect(player._safeDecode).toHaveBeenCalledTimes(1);
    const [type, timestamp, data] = player._safeDecode.mock.calls[0] as [string, number, Uint8Array];
    expect(type).toBe("delta");
    expect(timestamp).toBe(0);
    expect([...data]).toEqual([0, 0, 0, 1, 2, 1, 0xaa, 0, 0, 0, 1, 2, 1, 0xbb]);
  });

  it("drops a fragmented NAL when RTP sequence numbers have a gap", () => {
    const player = testPlayer();
    const start = parseRTPPacket(rtpPacket({ sequenceNumber: 20, timestamp: 90_000, payload: [0x62, 1, 0x93, 0xaa] }))!;
    const endWithGap = parseRTPPacket(rtpPacket({ sequenceNumber: 22, timestamp: 90_000, marker: true, payload: [0x62, 1, 0x53, 0xbb] }))!;

    (player as unknown as { _decodeRTP: (packet: ReturnType<typeof parseRTPPacket>) => void })._decodeRTP(start);
    (player as unknown as { _decodeRTP: (packet: ReturnType<typeof parseRTPPacket>) => void })._decodeRTP(endWithGap);

    expect(player._safeDecode).not.toHaveBeenCalled();
    expect((player as unknown as { diagnostics: { malformedFuPackets: number } }).diagnostics.malformedFuPackets).toBe(1);
  });
});
