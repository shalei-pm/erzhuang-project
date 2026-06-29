export type H5DecodePath = "mobile-wasm" | "desktop-mse";

const STRICT_FIRST_FRAME_EVENTS = new Set(["videoFrame", "firstFrameDisplay", "playToRenderTimes"]);
const LOOSE_FIRST_FRAME_EVENTS = new Set([...STRICT_FIRST_FRAME_EVENTS, "streamSuccess", "videoInfo", "playing", "loaded"]);

export function isH5FirstFrameEvent(eventName: string, decodePath: H5DecodePath): boolean {
  const candidates = decodePath === "mobile-wasm" ? STRICT_FIRST_FRAME_EVENTS : LOOSE_FIRST_FRAME_EVENTS;
  return candidates.has(eventName);
}

export function isH5StreamConnectedEvent(eventName: string): boolean {
  return eventName === "streamSuccess";
}
