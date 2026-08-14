export const fallbackProbeMinChannelNo = 30;
export const fallbackProbeMaxChannelNo = 64;
export const fallbackProbeStopAfterConsecutiveFailures = 8;

export function fallbackProbeChannelNumbers() {
  return Array.from({ length: fallbackProbeMaxChannelNo }, (_, index) => index + 1);
}

export function shouldStopFallbackProbe(channelNo: number, consecutiveFailures: number) {
  return channelNo >= fallbackProbeMinChannelNo && consecutiveFailures >= fallbackProbeStopAfterConsecutiveFailures;
}
