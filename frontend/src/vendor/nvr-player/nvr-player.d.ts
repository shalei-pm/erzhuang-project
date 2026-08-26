export type NVRPlayerDiagnostics = {
  receivedPackets: number;
  wasmRuntimeReady: boolean;
  wasmReady: boolean;
  wasmOutputInit: number;
  wasmOutputFrames: number;
  decoderInputFrames: number;
  renderedFrames: number;
  closeCode: number | null;
};

export type NVRPlayerOptions = {
  autoReconnect?: boolean;
  reconnectDelay?: number;
  forceWasm?: boolean;
  wasmWorkerUrl?: string;
  onError?: (error: Error) => void;
  onConnected?: () => void;
  onDisconnected?: () => void;
  onFirstFrame?: () => void;
  onDiagnostics?: (diagnostics: NVRPlayerDiagnostics) => void;
};

export default class NVRPlayer {
  constructor(canvas: HTMLCanvasElement, options?: NVRPlayerOptions);
  play(url: string): Promise<void>;
  stop(): void;
  pause(): void;
  resume(): void;
	  enableAudio(): Promise<void>;
  setVolume(volume: number): void;
}
