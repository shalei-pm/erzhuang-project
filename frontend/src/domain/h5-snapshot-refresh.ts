export type H5StreamMode = "live" | "playback";
export type H5StreamReleaseReason = "exit" | "switch" | "stop" | "replace";

export function shouldRefreshSnapshotBeforeRelease(
  mode: H5StreamMode,
  urlId: string | undefined | null,
  reason: H5StreamReleaseReason,
): boolean {
  return mode === "live" && Boolean(urlId) && reason !== "replace";
}
