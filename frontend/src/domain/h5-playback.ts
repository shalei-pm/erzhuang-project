export function segmentDurationSeconds(startTime: number, endTime: number): number {
  return Math.max(0, endTime - startTime);
}

export function shouldShowSegmentSlider(startTime: number, endTime: number): boolean {
  return segmentDurationSeconds(startTime, endTime) >= 2;
}

export function clampSegmentOffset(offset: number, startTime: number, endTime: number): number {
  const duration = segmentDurationSeconds(startTime, endTime);
  if (!Number.isFinite(offset)) return 0;
  return Math.min(duration, Math.max(0, Math.floor(offset)));
}

export function segmentOffsetToUnix(startTime: number, endTime: number, offset: number): number {
  return startTime + clampSegmentOffset(offset, startTime, endTime);
}

export type PlaybackSession = {
  startTime: number;
  endTime: number;
  startedAtMs: number;
};

export function estimatePlaybackUnixAt(pausedAtMs: number, session: PlaybackSession | null): number | null {
  if (!session) return null;
  const elapsedSeconds = Math.max(0, Math.floor((pausedAtMs - session.startedAtMs) / 1000));
  return clampUnix(session.startTime + elapsedSeconds, session.startTime, Math.max(session.startTime, session.endTime - 1));
}

export async function dataUrlToFile(dataUrl: string, filename: string): Promise<File> {
  const response = await fetch(dataUrl);
  const blob = await response.blob();
  return new File([blob], filename, { type: blob.type || "image/png" });
}

function clampUnix(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}
