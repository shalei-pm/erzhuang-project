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
  pausedAtMs?: number;
  pausedAtUnix?: number;
};

export function estimatePlaybackUnixAt(pausedAtMs: number, session: PlaybackSession | null): number | null {
  if (!session) return null;
  if (session.pausedAtUnix !== undefined) {
    return clampUnix(session.pausedAtUnix, session.startTime, Math.max(session.startTime, session.endTime - 1));
  }
  const elapsedSeconds = Math.max(0, Math.floor((pausedAtMs - session.startedAtMs) / 1000));
  return clampUnix(session.startTime + elapsedSeconds, session.startTime, Math.max(session.startTime, session.endTime - 1));
}

export function playbackUnixFromPlayerTime(currentTime: number | null, session: PlaybackSession | null): number | null {
  if (!session || currentTime === null || !Number.isFinite(currentTime)) return null;
  const elapsedSeconds = Math.max(0, Math.floor(currentTime));
  return clampUnix(session.startTime + elapsedSeconds, session.startTime, Math.max(session.startTime, session.endTime - 1));
}

export async function dataUrlToFile(dataUrl: string, filename: string): Promise<File> {
  const response = await fetch(dataUrl);
  const blob = await response.blob();
  return new File([blob], filename, { type: blob.type || "image/png" });
}

export function shouldFallbackToInlineFullscreen(
  doc: Pick<Document, "fullscreenEnabled">,
  nav: Pick<Navigator, "maxTouchPoints" | "userAgent">,
): boolean {
  const ua = nav.userAgent.toLowerCase();
  const isTouchMobile =
    nav.maxTouchPoints > 0 &&
    /android|iphone|ipad|ipod|mobile|micromessenger|lark|feishu|bytedancewebview/.test(ua);
  return isTouchMobile && !doc.fullscreenEnabled;
}

function clampUnix(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}
