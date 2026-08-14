export type H5DecodePath = "mobile-wasm" | "desktop-wasm" | "desktop-mse";

const STRICT_FIRST_FRAME_EVENTS = new Set(["videoFrame", "firstFrameDisplay", "playToRenderTimes"]);
const LOOSE_FIRST_FRAME_EVENTS = new Set([...STRICT_FIRST_FRAME_EVENTS, "streamSuccess", "videoInfo", "playing", "loaded"]);

export function isH5FirstFrameEvent(eventName: string, decodePath: H5DecodePath): boolean {
  const candidates = decodePath.endsWith("-wasm") ? STRICT_FIRST_FRAME_EVENTS : LOOSE_FIRST_FRAME_EVENTS;
  return candidates.has(eventName);
}

export function isH5StreamConnectedEvent(eventName: string): boolean {
  return eventName === "streamSuccess";
}

export function h5DecodePathForEnvironment(userAgent: string, maxTouchPoints: number): H5DecodePath {
  const normalizedUA = userAgent.toLowerCase();
  if (isH5MobilePlaybackContext(normalizedUA, maxTouchPoints)) return "mobile-wasm";
  return "desktop-mse";
}

export function shouldUseH5SoftDecode(decodePath: H5DecodePath): boolean {
  return decodePath.endsWith("-wasm");
}

export function shouldFallbackH5MSEToSoftDecode(message: string, decodePath: H5DecodePath): boolean {
  if (decodePath !== "desktop-mse") return false;
  const normalized = message.toLowerCase();
  const mentionsH265 = normalized.includes("hvc1") || normalized.includes("hev1") || normalized.includes("h265") || normalized.includes("hevc");
  const mentionsMSE =
    normalized.includes("mediasource") ||
    normalized.includes("sourcebuffer") ||
    normalized.includes("mse") ||
    normalized.includes("mediasourceh265notsupport");
  const mentionsUnsupported = normalized.includes("unsupported") || normalized.includes("not support");
  return mentionsH265 && mentionsMSE && mentionsUnsupported;
}

function isH5MobilePlaybackContext(normalizedUA: string, maxTouchPoints: number) {
  return (
    maxTouchPoints > 1 &&
    /android|iphone|ipad|ipod|mobile|micromessenger|lark|feishu|bytedancewebview/.test(normalizedUA)
  );
}
