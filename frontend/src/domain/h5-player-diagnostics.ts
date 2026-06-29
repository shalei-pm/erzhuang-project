export type H5DecodePath = "mobile-wasm" | "desktop-mse";

const STRICT_FIRST_FRAME_EVENTS = new Set(["streamSuccess", "videoInfo", "videoFrame"]);
const LOOSE_FIRST_FRAME_EVENTS = new Set([...STRICT_FIRST_FRAME_EVENTS, "playing", "loaded"]);

export function isH5FirstFrameEvent(eventName: string, decodePath: H5DecodePath): boolean {
  const candidates = decodePath === "mobile-wasm" ? STRICT_FIRST_FRAME_EVENTS : LOOSE_FIRST_FRAME_EVENTS;
  return candidates.has(eventName);
}
