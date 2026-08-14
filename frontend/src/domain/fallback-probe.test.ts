import {
  fallbackProbeChannelNumbers,
  fallbackProbeMaxChannelNo,
  fallbackProbeMinChannelNo,
  fallbackProbeStopAfterConsecutiveFailures,
  shouldStopFallbackProbe,
} from "./fallback-probe.js";

const channels = fallbackProbeChannelNumbers();

assertEqual(fallbackProbeMinChannelNo, 30);
assertEqual(fallbackProbeMaxChannelNo, 64);
assertEqual(fallbackProbeStopAfterConsecutiveFailures, 8);
assertEqual(channels.length, 64);
assertEqual(channels[0], 1);
assertEqual(channels[63], 64);
assertEqual(channels.join(","), Array.from({ length: 64 }, (_, index) => String(index + 1)).join(","));
assertEqual(shouldStopFallbackProbe(29, 99), false);
assertEqual(shouldStopFallbackProbe(30, 7), false);
assertEqual(shouldStopFallbackProbe(30, 8), true);
assertEqual(shouldStopFallbackProbe(40, 8), true);

console.log("fallback-probe tests passed");

function assertEqual(actual: unknown, expected: unknown) {
  if (actual !== expected) {
    throw new Error(`expected ${expected}, got ${actual}`);
  }
}
