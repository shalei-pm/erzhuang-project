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
