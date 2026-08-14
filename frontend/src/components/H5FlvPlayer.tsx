import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type Dispatch,
  type MutableRefObject,
  type SetStateAction,
} from "react";
import "ezuikit-flv/style.css";
import {
  h5DecodePathForEnvironment,
  isH5FirstFrameEvent,
  isH5StreamConnectedEvent,
  shouldFallbackH5MSEToSoftDecode,
  shouldUseH5SoftDecode,
  type H5DecodePath,
} from "../domain/h5-player-diagnostics";

// ezuikit-flv is loaded dynamically to avoid SSR issues and to keep
// the decoder path configurable.
interface EzuikitFlvPlayer {
  destroy: () => void;
  closeSound: () => void;
  openSound: () => void;
  play: () => unknown;
  pause: () => unknown;
  currentTime?: number;
  getState?: () => unknown;
  screenshot?: (filename?: string, format?: string, quality?: number, type?: "download" | "base64" | "blob") => unknown;
  capturePicture?: () => unknown;
}

interface EzuikitFlvConstructor {
  new (options: {
    id: string;
    url: string;
    decoder: string;
    autoPlay: boolean;
    isLive: boolean;
    muted: boolean;
    autoWasm: boolean;
    useMSE: boolean;
    useWCS: boolean;
    forceNoOffscreen: boolean;
    wasmDecodeErrorReplay: boolean;
    wasmDecodeAudioSyncVideo: boolean;
    hasVideo: boolean;
    debug: boolean;
    hasAudio: boolean;
    keepScreenOn: boolean;
    scaleMode: 0 | 1 | 2;
    videoBuffer: number;
    themeData: null;
    mutedShowAutoReload: boolean;
  }): EzuikitFlvPlayer;
}

type PlayerDiagnostic = {
  stage: string;
  message: string;
  details: string[];
  severity: "error" | "warning";
};

export type H5PlayerStatus = {
  stage: string;
  message: string;
  details: string[];
  severity: "info" | "warning" | "error";
};

export type H5PlaybackState = {
  playing: boolean;
  muted: boolean;
  loading: boolean;
  failed: boolean;
};

export type H5PlayerScreenshot = {
  dataUrl?: string;
};

export type H5PlayerHandle = {
  play: () => Promise<void>;
  pause: () => Promise<void>;
  openSound: () => Promise<void>;
  closeSound: () => Promise<void>;
  screenshot: () => Promise<H5PlayerScreenshot>;
  getCurrentTime: () => number | null;
  enterFullscreen: () => Promise<void>;
  exitFullscreen: () => Promise<void>;
};

let playerLib: EzuikitFlvConstructor | null = null;
let playerLibPromise: Promise<{ ctor: EzuikitFlvConstructor | null; diagnostics: string[] }> | null = null;
const DECODER_PATH = `${import.meta.env.BASE_URL.replace(/\/$/, "")}/assets/ezuikit-flv/decoder.js`;
const FIRST_FRAME_TIMEOUT_MS = 12000;
const PLAYER_ERROR_EVENTS = [
  "error",
  "streamError",
  "playError",
  "fetchError",
  "audioCodecUnsupported",
  "wasmDecodeError",
  "webcodecsH265NotSupport",
  "webcodecsDecodeError",
  "mediaSourceH265NotSupport",
  "mseSourceBufferError",
  "unrecoverableEarlyEof",
  "webglAlignmentError",
];
const PLAYER_PROGRESS_EVENTS = [
  "streamSuccess",
  "videoInfo",
  "videoFrame",
  "firstFrameDisplay",
  "playToRenderTimes",
  "stats",
  "kBps",
  "playing",
  "loaded",
  "loading",
  "load",
  "start",
  "metadata",
  "mseSourceOpen",
  "decoderWorkerInit",
  "decoderLoaded",
];

async function loadPlayerLib(): Promise<{ ctor: EzuikitFlvConstructor | null; diagnostics: string[] }> {
  if (playerLib) return { ctor: playerLib, diagnostics: ["cached constructor"] };
  if (playerLibPromise) return playerLibPromise;

  playerLibPromise = (async () => {
    try {
      const mod = (await import("ezuikit-flv")) as Record<string, unknown>;
      const candidates = [
        ["default", mod.default],
        ["EzuikitFlv", mod.EzuikitFlv],
        ["module", mod],
      ] as const;
      const diagnostics = candidates.map(([name, value]) => `${name}:${typeof value}`);
      const match = candidates.find(([, value]) => typeof value === "function");
      const Ctor = (match?.[1] as EzuikitFlvConstructor | undefined) ?? null;
      playerLib = Ctor;
      return { ctor: Ctor, diagnostics };
    } catch (err) {
      return { ctor: null, diagnostics: [`import failed:${errorText(err)}`] };
    }
  })();

  return playerLibPromise;
}

export interface H5FlvPlayerProps {
  url: string;
  protocol?: string;
  isLive: boolean;
  onError?: (message: string) => void;
  onStatus?: (status: H5PlayerStatus) => void;
  onPlaybackStateChange?: (state: H5PlaybackState) => void;
}

export const H5FlvPlayer = forwardRef<H5PlayerHandle, H5FlvPlayerProps>(function H5FlvPlayer(
  { url, protocol, isLive, onError, onStatus, onPlaybackStateChange },
  ref,
) {
  const containerId = useMemo(
    () => `player-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    [url, isLive],
  );
  const wrapperRef = useRef<HTMLDivElement | null>(null);
  const playerRef = useRef<EzuikitFlvPlayer | null>(null);
  const nativeVideoRef = useRef<HTMLVideoElement | null>(null);
  const warningTimerRef = useRef<number | null>(null);
  const firstFrameTimerRef = useRef<number | null>(null);
  const playerEventsRef = useRef<string[]>([]);
  const [playing, setPlaying] = useState(false);
  const [muted, setMuted] = useState(true);
  const [loading, setLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [diagnostic, setDiagnostic] = useState<PlayerDiagnostic | null>(null);
  const [forceSoftDecode, setForceSoftDecode] = useState(false);
  const isMock = url.startsWith("mock-");
  const environmentDecodePath = h5DecodePathForEnvironment(readUserAgent(), readMaxTouchPoints());
  const decodePath: H5DecodePath = forceSoftDecode && environmentDecodePath === "desktop-mse" ? "desktop-wasm" : environmentDecodePath;
  const preferSoftDecode = shouldUseH5SoftDecode(decodePath);
  const isNativeVideo = !isMock && shouldUseNativeVideo(url, protocol) && !preferSoftDecode;

  useEffect(() => {
    setForceSoftDecode(false);
  }, [url, protocol, isLive]);

  useEffect(() => {
    onPlaybackStateChange?.({ playing, muted, loading, failed: loadFailed });
  }, [playing, muted, loading, loadFailed, onPlaybackStateChange]);

  useEffect(() => {
    if (isMock) {
      clearFirstFrameTimer(firstFrameTimerRef);
      playerEventsRef.current = [];
      setPlaying(true);
      setLoading(false);
      setLoadFailed(false);
      setDiagnostic(null);
      reportStatus(onStatus, {
        stage: "mock-ready",
        message: "模拟画面已就绪",
        details: [`mode=${isLive ? "live" : "playback"}`],
        severity: "info",
      });
      return;
    }
    if (isNativeVideo) {
      clearFirstFrameTimer(firstFrameTimerRef);
      playerEventsRef.current = [];
      setPlaying(false);
      setLoading(true);
      setLoadFailed(false);
      setDiagnostic(null);
      reportStatus(onStatus, {
        stage: "native-video-init",
        message: "使用原生 video 播放",
        details: buildCommonDetails(url, protocol, isLive, decodePath, ["native-video"]),
        severity: "info",
      });
      return () => {
        nativeVideoRef.current = null;
        setPlaying(false);
      };
    }
    let cancelled = false;

    async function init() {
      clearFirstFrameTimer(firstFrameTimerRef);
      playerEventsRef.current = [];
      setPlaying(false);
      setLoading(true);
      setLoadFailed(false);
      setDiagnostic(null);
      const loaded = await loadPlayerLib();
      if (cancelled) return;

      const commonDetails = [
        ...buildCommonDetails(url, protocol, isLive, decodePath, []),
        `lib=${combineText(loaded.diagnostics, ",")}`,
      ];
      reportStatus(onStatus, {
        stage: "player-init",
        message: "播放器初始化中，等待首帧事件",
        details: commonDetails,
        severity: "info",
      });
      const Ctor = loaded.ctor;
      if (!Ctor) {
        setPlaying(false);
        setLoadFailed(true);
        setLoading(false);
        const nextDiagnostic = {
          stage: "load-player-lib",
          message: "播放器库没有返回可用构造函数",
          details: commonDetails,
          severity: "error" as const,
        };
        setDiagnostic(nextDiagnostic);
        reportDiagnostic(onStatus, nextDiagnostic);
        onError?.(formatDiagnostic(nextDiagnostic));
        return;
      }

      try {
        const player = new Ctor({
          id: containerId,
          url,
          decoder: DECODER_PATH,
          autoPlay: true,
          isLive,
          muted: true,
          autoWasm: true,
          useMSE: !preferSoftDecode,
          useWCS: !preferSoftDecode,
          forceNoOffscreen: preferSoftDecode,
          wasmDecodeErrorReplay: true,
          wasmDecodeAudioSyncVideo: true,
          hasVideo: true,
          debug: true,
          hasAudio: true,
          keepScreenOn: decodePath === "mobile-wasm",
          scaleMode: 2,
          videoBuffer: 1,
          themeData: null,
          mutedShowAutoReload: false,
        });
        playerRef.current = player;
        attachPlayerDiagnostics(player, (message) => {
          if (shouldFallbackH5MSEToSoftDecode(message, decodePath)) {
            setLoading(true);
            const nextDiagnostic = {
              stage: "decode-fallback",
              message: "当前浏览器不支持该 H.265 硬解码流，正在切换软解码重试",
              details: [...commonDetails, `player=${message}`],
              severity: "warning" as const,
            };
            setDiagnostic(nextDiagnostic);
            reportDiagnostic(onStatus, nextDiagnostic);
            setForceSoftDecode(true);
            return;
          }
          const severity: PlayerDiagnostic["severity"] = isRecoverablePlayerEvent(message) ? "warning" : "error";
          const nextDiagnostic = {
            stage: "player-event",
            message,
            details: commonDetails,
            severity,
          };
          setDiagnostic(nextDiagnostic);
          reportDiagnostic(onStatus, nextDiagnostic);
          if (severity === "error") {
            setPlaying(false);
            onError?.(formatDiagnostic(nextDiagnostic));
          } else {
            scheduleWarningClear(warningTimerRef, setDiagnostic);
          }
        }, (eventName, payload) => {
          notePlayerEvent(playerEventsRef, eventName, payload);
          const currentEvents = `events=${combineText(playerEventsRef.current, " > ")}`;
          reportStatus(onStatus, {
            stage: "player-event",
            message: `播放器事件：${eventName}`,
            details: [...commonDetails, currentEvents],
            severity: "info",
          });
          if (isH5StreamConnectedEvent(eventName)) {
            reportStatus(onStatus, {
              stage: "stream-connected",
              message: "直播流已连接，继续等待视频帧渲染",
              details: [...commonDetails, currentEvents],
              severity: "info",
            });
            return;
          }
          if (isH5FirstFrameEvent(eventName, decodePath)) {
            clearFirstFrameTimer(firstFrameTimerRef);
            setPlaying(true);
            setLoading(false);
            setLoadFailed(false);
            setDiagnostic((current) => (current?.stage === "first-frame-timeout" ? null : current));
            reportStatus(onStatus, {
              stage: "first-frame-ready",
              message: `已收到视频渲染事件：${eventName}`,
              details: [...commonDetails, currentEvents],
              severity: "info",
            });
          }
        });
        startFirstFrameTimer(firstFrameTimerRef, () => {
          if (cancelled) return;
          setPlaying(false);
          setLoading(false);
          setLoadFailed(true);
          const nextDiagnostic = {
            stage: "first-frame-timeout",
            message: firstFrameTimeoutMessage(playerEventsRef.current),
            details: [
              ...commonDetails,
              `events=${playerEventsRef.current.length > 0 ? combineText(playerEventsRef.current, " > ") : "none"}`,
              `state=${safePlayerState(player)}`,
            ],
            severity: "error" as const,
          };
          setDiagnostic(nextDiagnostic);
          reportDiagnostic(onStatus, nextDiagnostic);
          onError?.(formatDiagnostic(nextDiagnostic));
        });
      } catch (err) {
        if (cancelled) return;
        setPlaying(false);
        setLoading(false);
        setLoadFailed(true);
        const nextDiagnostic = {
          stage: "init-player",
          message: errorText(err),
          details: commonDetails,
          severity: "error" as const,
        };
        setDiagnostic(nextDiagnostic);
        reportDiagnostic(onStatus, nextDiagnostic);
        onError?.(formatDiagnostic(nextDiagnostic));
      }
    }

    init();

    return () => {
      cancelled = true;
      clearWarningTimer(warningTimerRef);
      clearFirstFrameTimer(firstFrameTimerRef);
      setPlaying(false);
      if (playerRef.current) {
        stopPlayer(playerRef.current, containerId);
        playerRef.current = null;
      }
    };
  }, [url, isLive, onError, onStatus, isMock, isNativeVideo, containerId, protocol, preferSoftDecode, decodePath]);

  function callPlayer(action: "play" | "pause" | "openSound" | "closeSound") {
    if (playerRef.current) {
      try {
        return Promise.resolve(playerRef.current[action]());
      } catch (err) {
        return Promise.reject(err);
      }
    }
    const video = isNativeVideo ? nativeVideoRef.current : null;
    if (video) {
      if (action === "play") {
        return Promise.resolve(video.play());
      }
      if (action === "pause") {
        video.pause();
      } else if (action === "openSound") {
        video.muted = false;
      } else {
        video.muted = true;
      }
    }
    return Promise.resolve();
  }

  useImperativeHandle(ref, () => ({
    async play() {
      await callPlayer("play");
      setPlaying(true);
      setLoading(false);
    },
    async pause() {
      await callPlayer("pause");
      setPlaying(false);
    },
    async openSound() {
      await callPlayer("openSound");
      setMuted(false);
    },
    async closeSound() {
      await callPlayer("closeSound");
      setMuted(true);
    },
    async screenshot() {
      const player = playerRef.current;
      const candidate = player?.screenshot ?? player?.capturePicture;
      if (typeof candidate === "function") {
        const result = await Promise.resolve(candidate.call(player, `monitor-snapshot-${Date.now()}`, "png", 0.92, "base64"));
        const normalized = await normalizeScreenshotResult(result);
        if (normalized.dataUrl) {
          return normalized;
        }
      }
      const fallback = captureRenderedFrame(wrapperRef.current);
      if (fallback) {
        return { dataUrl: fallback };
      }
      throw new Error("screenshot is not supported by current player");
    },
    getCurrentTime() {
      return readPlayerCurrentTime(playerRef.current, nativeVideoRef.current);
    },
    async enterFullscreen() {
      await requestElementFullscreen(wrapperRef.current);
    },
    async exitFullscreen() {
      if (document.fullscreenElement) {
        await document.exitFullscreen();
      }
    },
  }));

  return (
    <div className="h5-player-wrapper" ref={wrapperRef}>
      {isMock ? (
        <div className="h5-player-container h5-player-mock">
          <img src={`https://picsum.photos/seed/${encodeURIComponent(url)}/960/540`} alt="模拟监控画面" />
          <span className="h5-player-mock-badge">{isLive ? "实时视频" : "录像回放"}</span>
        </div>
      ) : isNativeVideo ? (
        <video
          ref={nativeVideoRef}
          className="h5-player-container h5-native-video"
          src={url}
          playsInline
          autoPlay
          muted={muted}
          onLoadedMetadata={() => setLoading(false)}
          onCanPlay={() => setLoading(false)}
          onPlaying={() => {
            setPlaying(true);
            setLoading(false);
          }}
          onPause={() => setPlaying(false)}
          onError={() => {
            setPlaying(false);
            setLoading(false);
            setLoadFailed(true);
            const nextDiagnostic = {
              stage: "native-video",
              message: "原生视频播放失败",
              details: [`url=${summarizeUrl(url)}`, `protocol=${protocol || inferProtocol(url)}`, `mode=${isLive ? "live" : "playback"}`],
              severity: "error" as const,
            };
            setDiagnostic(nextDiagnostic);
            reportDiagnostic(onStatus, nextDiagnostic);
            onError?.(formatDiagnostic(nextDiagnostic));
          }}
        />
      ) : (
        <div key={containerId} id={containerId} className="h5-player-container" />
      )}
      {loading && (
        <div className="h5-player-overlay">
          <span className="h5-player-loading">加载中…</span>
        </div>
      )}
      {loadFailed && (
        <div className="h5-player-overlay">
          <span className="h5-player-error">播放器加载失败</span>
        </div>
      )}
      {diagnostic && <PlayerDiagnosticPanel diagnostic={diagnostic} />}
    </div>
  );
});

function attachPlayerDiagnostics(
  player: EzuikitFlvPlayer,
  onPlayerError: (message: string) => void,
  onPlayerProgress: (eventName: string, payload: unknown) => void,
) {
  const maybeEmitter = player as unknown as { on?: (event: string, handler: (payload: unknown) => void) => void };
  if (typeof maybeEmitter.on !== "function") {
    return;
  }
  for (const eventName of PLAYER_ERROR_EVENTS) {
    try {
      maybeEmitter.on(eventName, (payload) => onPlayerError(`${eventName}:${serializePayload(payload)}`));
    } catch {
      // Some player versions may not support every event name.
    }
  }
  for (const eventName of PLAYER_PROGRESS_EVENTS) {
    try {
      maybeEmitter.on(eventName, (payload) => onPlayerProgress(eventName, payload));
    } catch {
      // Some player versions may not support every event name.
    }
  }
}

function PlayerDiagnosticPanel({ diagnostic }: { diagnostic: PlayerDiagnostic }) {
  return (
    <div className={`h5-player-diagnostic ${diagnostic.severity}`} aria-label="播放器错误详情">
      <strong>{diagnostic.stage}</strong>
      <span>{diagnostic.message}</span>
      {diagnostic.details.map((item) => (
        <code key={item}>{item}</code>
      ))}
    </div>
  );
}

async function requestElementFullscreen(element: HTMLElement | null) {
  if (!element || typeof element.requestFullscreen !== "function") {
    throw new Error("fullscreen is not supported by current browser");
  }
  await element.requestFullscreen();
}

async function normalizeScreenshotResult(result: unknown): Promise<H5PlayerScreenshot> {
  if (typeof result === "string") {
    if (result.startsWith("data:")) {
      return { dataUrl: result };
    }
    return { dataUrl: `data:image/png;base64,${result}` };
  }
  if (typeof Blob !== "undefined" && result instanceof Blob) {
    return { dataUrl: await blobToDataUrl(result) };
  }
  if (result && typeof result === "object" && "dataUrl" in result) {
    return { dataUrl: String((result as { dataUrl?: unknown }).dataUrl || "") };
  }
  return {};
}

function blobToDataUrl(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ""));
    reader.onerror = () => reject(reader.error || new Error("blob read failed"));
    reader.readAsDataURL(blob);
  });
}

function captureRenderedFrame(wrapper: HTMLElement | null): string | null {
  if (!wrapper) return null;
  const canvas = wrapper.querySelector("canvas");
  if (canvas && canvas.width > 0 && canvas.height > 0) {
    try {
      return canvas.toDataURL("image/png");
    } catch {
      // Cross-origin or WebGL contexts may reject readback.
    }
  }
  const video = wrapper.querySelector("video");
  if (video && video.videoWidth > 0 && video.videoHeight > 0) {
    try {
      const captureCanvas = document.createElement("canvas");
      captureCanvas.width = video.videoWidth;
      captureCanvas.height = video.videoHeight;
      const context = captureCanvas.getContext("2d");
      context?.drawImage(video, 0, 0, captureCanvas.width, captureCanvas.height);
      return captureCanvas.toDataURL("image/png");
    } catch {
      // Browser security rules can block drawing signed remote streams.
    }
  }
  return null;
}

function readPlayerCurrentTime(player: EzuikitFlvPlayer | null, video: HTMLVideoElement | null) {
  if (player && typeof player.currentTime === "number" && Number.isFinite(player.currentTime)) {
    return player.currentTime;
  }
  const nestedTime = readNestedCurrentTime(player);
  if (nestedTime !== null) {
    return nestedTime;
  }
  if (video && Number.isFinite(video.currentTime)) {
    return video.currentTime;
  }
  return null;
}

function readNestedCurrentTime(player: EzuikitFlvPlayer | null) {
  const maybePlayer = player as unknown as {
    player?: {
      video?: {
        currentTime?: number;
        $videoElement?: { currentTime?: number };
      };
    };
  } | null;
  const candidates = [
    maybePlayer?.player?.video?.currentTime,
    maybePlayer?.player?.video?.$videoElement?.currentTime,
  ];
  const match = candidates.find((value): value is number => typeof value === "number" && Number.isFinite(value));
  return match ?? null;
}

function isRecoverablePlayerEvent(message: string) {
  const text = message.toLowerCase();
  return text.includes("mediasource") || text.includes("sourcebuffer") || text.includes("mse");
}

function notePlayerEvent(eventRef: MutableRefObject<string[]>, eventName: string, payload: unknown) {
  const text = payload ? `${eventName}:${shortPayload(payload)}` : eventName;
  eventRef.current = [...eventRef.current.slice(-7), text];
}

function shouldUseNativeVideo(url: string, protocol?: string) {
  const normalized = (protocol || inferProtocol(url)).toLowerCase();
  return normalized === "hls" || normalized === "m3u8";
}

function inferProtocol(url: string) {
  try {
    const parsed = new URL(url);
    if (parsed.pathname.toLowerCase().endsWith(".m3u8")) return "hls";
    if (parsed.pathname.toLowerCase().endsWith(".flv")) return "flv";
  } catch {
    // fall through
  }
  return "";
}

function scheduleWarningClear(
  timerRef: MutableRefObject<number | null>,
  setDiagnostic: Dispatch<SetStateAction<PlayerDiagnostic | null>>,
) {
  clearWarningTimer(timerRef);
  timerRef.current = window.setTimeout(() => {
    setDiagnostic((current) => (current?.severity === "warning" ? null : current));
    timerRef.current = null;
  }, 6000);
}

function clearWarningTimer(timerRef: MutableRefObject<number | null>) {
  if (timerRef.current !== null) {
    window.clearTimeout(timerRef.current);
    timerRef.current = null;
  }
}

function startFirstFrameTimer(timerRef: MutableRefObject<number | null>, onTimeout: () => void) {
  clearFirstFrameTimer(timerRef);
  timerRef.current = window.setTimeout(onTimeout, FIRST_FRAME_TIMEOUT_MS);
}

function clearFirstFrameTimer(timerRef: MutableRefObject<number | null>) {
  if (timerRef.current !== null) {
    window.clearTimeout(timerRef.current);
    timerRef.current = null;
  }
}

function stopPlayer(player: EzuikitFlvPlayer, containerId: string) {
  try {
    const paused = player.pause();
    if (paused && typeof (paused as Promise<unknown>).finally === "function") {
      void (paused as Promise<unknown>).finally(() => destroyPlayer(player, containerId));
      return;
    }
  } catch {
    // continue to destroy
  }
  destroyPlayer(player, containerId);
}

function destroyPlayer(player: EzuikitFlvPlayer, containerId: string) {
  try {
    player.destroy();
  } catch {
    // ignore
  }
  const container = document.getElementById(containerId);
  if (container) {
    container.innerHTML = "";
  }
}

function formatDiagnostic(diagnostic: PlayerDiagnostic) {
  return combineText([diagnostic.message, `stage=${diagnostic.stage}`, ...diagnostic.details], " · ");
}

function combineText(items: string[], separator: string) {
  return items.reduce((text, item) => (text ? `${text}${separator}${item}` : item), "");
}

function firstFrameTimeoutMessage(events: string[]) {
  const seconds = FIRST_FRAME_TIMEOUT_MS / 1000;
  const streamConnected = events.some((event) => event.startsWith("streamSuccess"));
  return streamConnected
    ? `直播流已连接，但 ${seconds} 秒内没有收到视频帧渲染事件`
    : `播放器已初始化，但 ${seconds} 秒内没有收到流连接或视频帧渲染事件`;
}

function buildCommonDetails(
  url: string,
  protocol: string | undefined,
  isLive: boolean,
  decodePath: H5DecodePath,
  extra: string[],
) {
  return [
    `app=${import.meta.env.VITE_APP_VERSION || "local-dev"}`,
    `url=${summarizeUrl(url)}`,
    `decoder=${DECODER_PATH}`,
    `mode=${isLive ? "live" : "playback"}`,
    `protocol=${protocol || inferProtocol(url) || "unknown"}`,
    `decode=${decodePath}`,
    `ua=${summarizeUserAgent()}`,
    ...extra,
  ];
}

function reportDiagnostic(onStatus: H5FlvPlayerProps["onStatus"], diagnostic: PlayerDiagnostic) {
  reportStatus(onStatus, {
    ...diagnostic,
    severity: diagnostic.severity,
  });
}

function reportStatus(onStatus: H5FlvPlayerProps["onStatus"], status: H5PlayerStatus) {
  onStatus?.(status);
}

function summarizeUserAgent() {
  if (typeof navigator === "undefined") return "server";
  return navigator.userAgent.slice(0, 120);
}

function readUserAgent() {
  return typeof navigator === "undefined" ? "" : navigator.userAgent;
}

function readMaxTouchPoints() {
  return typeof navigator === "undefined" ? 0 : navigator.maxTouchPoints || 0;
}

function summarizeUrl(value: string) {
  try {
    const parsed = new URL(value);
    const expire = parsed.searchParams.get("expire") || parsed.searchParams.get("ev") || "";
    return `${parsed.origin}${parsed.pathname}${expire ? `?expire=${expire}` : ""}`;
  } catch {
    return value.slice(0, 120);
  }
}

function errorText(err: unknown) {
  if (err instanceof Error) {
    return `${err.name}: ${err.message}`;
  }
  return String(err || "unknown error");
}

function serializePayload(payload: unknown) {
  if (payload instanceof Error) {
    return errorText(payload);
  }
  if (typeof payload === "string") {
    return redactSignedUrls(payload);
  }
  try {
    return redactSignedUrls(JSON.stringify(payload));
  } catch {
    return redactSignedUrls(String(payload));
  }
}

function shortPayload(payload: unknown) {
  const text = serializePayload(payload);
  return text.length > 120 ? `${text.slice(0, 120)}...` : text;
}

function safePlayerState(player: EzuikitFlvPlayer) {
  if (typeof player.getState !== "function") return "unavailable";
  try {
    return shortPayload(player.getState());
  } catch (err) {
    return errorText(err);
  }
}

function redactSignedUrls(value: string) {
  return value.replace(/https?:\/\/[^\s"'<>]+/g, (rawUrl) => summarizeUrl(rawUrl));
}
