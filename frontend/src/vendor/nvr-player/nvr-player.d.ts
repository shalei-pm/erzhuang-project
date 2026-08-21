export type NVRPlayerOptions = {
  autoReconnect?: boolean;
  reconnectDelay?: number;
  forceWasm?: boolean;
  wasmWorkerUrl?: string;
  onError?: (error: Error) => void;
  onConnected?: () => void;
  onDisconnected?: () => void;
  onFirstFrame?: () => void;
};

export default class NVRPlayer {
  constructor(canvas: HTMLCanvasElement, options?: NVRPlayerOptions);
  play(url: string): Promise<void>;
  stop(): void;
  pause(): void;
  resume(): void;
  setVolume(volume: number): void;
}
