export type PcmStreamEngineOptions = {
  inputSampleRate?: number;
  outputSampleRate?: number;
  capacity?: number;
  minStartSamples?: number;
  maxBufferedSamples?: number;
  targetTrimSamples?: number;
  fadeSamples?: number;
  seamSmoothSamples?: number;
};

export type PcmPushOptions = {
  packetLoss?: number;
};

export class PcmStreamEngine {
  readonly bufferedSamples: number;
  constructor(options?: PcmStreamEngineOptions);
  reset(): void;
  pushInt16(samples: ArrayLike<number>, options?: PcmPushOptions): void;
  canScheduleFrames(outputFrames: number): boolean;
  pull(output: Float32Array): Float32Array;
}
