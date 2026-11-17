export type NVRPlayerDiagnostics = {
  receivedPackets: number;
  rtpPackets: number;
  videoPayloadPackets: number;
  audioPayloadPackets: number;
  otherPayloadPackets: number;
  vpsPackets: number;
  spsPackets: number;
  ppsPackets: number;
  keyFrameNALUnits: number;
  wasmRuntimeReady: boolean;
  wasmReady: boolean;
  wasmOutputInit: number;
  wasmOutputFrames: number;
  decoderInputFrames: number;
  renderedFrames: number;
  markerPackets: number;
  accessUnits: number;
  multiNALAccessUnits: number;
  droppedAccessUnits: number;
  malformedFuPackets: number;
  decoderErrors: number;
  lastDecoderError: string;
  closeCode: number | null;
};

export type PlaybackControlDiagnostics = {
  phase: "idle" | "sent" | "verifying" | "verified" | "failed";
  requestedRate: number;
  verifiedRate: number;
  protocol: "json";
  commandSequence: number | null;
  ack: "none" | "accepted" | "rejected";
  observedRate: number | null;
  failure: "timeout" | "rejected" | "connection" | "stopped" | "unsupported" | "send_failed" | null;
};

export type RTPPacket = {
  payloadType: number;
  marker: boolean;
  sequenceNumber: number;
  timestamp: number;
  payload: Uint8Array;
};

export function parseRTPPacket(data: ArrayBuffer | Uint8Array): RTPPacket | null;

export type NVRPlayerOptions = {
  autoReconnect?: boolean;
  reconnectDelay?: number;
  forceWasm?: boolean;
  wasmWorkerUrl?: string;
  audioWorkletUrl?: string;
  onError?: (error: Error) => void;
  onConnected?: () => void;
  onDisconnected?: (details: { code: number | null; wasClean: boolean }) => void;
  onFirstFrame?: () => void;
  /** Relative seconds of the last presented WASM media frame; absent for live. */
  onPlaybackTime?: (elapsedSeconds: number) => void;
  onDiagnostics?: (diagnostics: NVRPlayerDiagnostics) => void;
  onPlaybackControlDiagnostics?: (snapshot: PlaybackControlDiagnostics) => void;
};

export default class NVRPlayer {
  constructor(canvas: HTMLCanvasElement, options?: NVRPlayerOptions);
  play(url: string): Promise<void>;
  stop(): void;
  pause(): void;
  resume(): void;
  /** Call in the user gesture; prepares audio even while speed-muted. */
  enableAudio(): Promise<void>;
  setVolume(volume: number): void;
  /** Null until a valid playback frame is presented, and after stop. */
  getPlaybackTime(): number | null;
  /** A copy of the last control attempt; failures survive stop, and play resets it. */
  getPlaybackControlDiagnostics(): PlaybackControlDiagnostics;
  /** Active WASM playback only: 1/2/4/8/16. Verifies stable media PTS cadence within 6s.
   * Rejects unsupported, pending, disconnected, stopped or unverified requests.
   * On failure the host must create a fresh normal-speed session.
   */
  setSpeed(rate: number): Promise<void>;
}
