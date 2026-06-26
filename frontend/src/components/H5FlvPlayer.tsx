import { useEffect, useMemo, useRef, useState, type Dispatch, type MutableRefObject, type SetStateAction } from "react";
import "ezuikit-flv/style.css";

// ezuikit-flv is loaded dynamically to avoid SSR issues and to keep
// the decoder path configurable.
interface EzuikitFlvPlayer {
  destroy: () => void;
  closeSound: () => void;
  openSound: () => void;
  play: () => unknown;
  pause: () => unknown;
  getState?: () => unknown;
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
];
const PLAYER_PROGRESS_EVENTS = [
  "streamSuccess",
  "videoInfo",
  "videoFrame",
  "playing",
  "loaded",
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
}

export function H5FlvPlayer({ url, protocol, isLive, onError }: H5FlvPlayerProps) {
  const containerId = useMemo(
    () => `player-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    [url, isLive],
  );
  const playerRef = useRef<EzuikitFlvPlayer | null>(null);
  const warningTimerRef = useRef<number | null>(null);
  const firstFrameTimerRef = useRef<number | null>(null);
  const playerEventsRef = useRef<string[]>([]);
  const [muted, setMuted] = useState(true);
  const [loading, setLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [diagnostic, setDiagnostic] = useState<PlayerDiagnostic | null>(null);
  const isMock = url.startsWith("mock-");
  const preferSoftDecode = isMobilePlaybackContext();
  const isNativeVideo = !isMock && shouldUseNativeVideo(url, protocol) && !preferSoftDecode;

  useEffect(() => {
    if (isMock) {
      clearFirstFrameTimer(firstFrameTimerRef);
      playerEventsRef.current = [];
      setLoading(false);
      setLoadFailed(false);
      setDiagnostic(null);
      return;
    }
    if (isNativeVideo) {
      clearFirstFrameTimer(firstFrameTimerRef);
      playerEventsRef.current = [];
      setLoading(true);
      setLoadFailed(false);
      setDiagnostic(null);
      return;
    }
    let cancelled = false;

    async function init() {
      clearFirstFrameTimer(firstFrameTimerRef);
      playerEventsRef.current = [];
      setLoading(true);
      setLoadFailed(false);
      setDiagnostic(null);
      const loaded = await loadPlayerLib();
      if (cancelled) return;

      const commonDetails = [
        `url=${summarizeUrl(url)}`,
        `decoder=${DECODER_PATH}`,
        `mode=${isLive ? "live" : "playback"}`,
        `protocol=${protocol || inferProtocol(url) || "unknown"}`,
        `decode=${preferSoftDecode ? "mobile-wasm" : "desktop-mse"}`,
        `lib=${loaded.diagnostics.join(",")}`,
      ];
      const Ctor = loaded.ctor;
      if (!Ctor) {
        setLoadFailed(true);
        setLoading(false);
        const nextDiagnostic = {
          stage: "load-player-lib",
          message: "播放器库没有返回可用构造函数",
          details: commonDetails,
          severity: "error" as const,
        };
        setDiagnostic(nextDiagnostic);
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
          useWCS: true,
          hasAudio: true,
          keepScreenOn: preferSoftDecode,
          scaleMode: 2,
          videoBuffer: 1,
          themeData: null,
          mutedShowAutoReload: false,
        });
        playerRef.current = player;
        attachPlayerDiagnostics(player, (message) => {
          const severity: PlayerDiagnostic["severity"] = isRecoverablePlayerEvent(message) ? "warning" : "error";
          const nextDiagnostic = {
            stage: "player-event",
            message,
            details: commonDetails,
            severity,
          };
          setDiagnostic(nextDiagnostic);
          if (severity === "error") {
            onError?.(formatDiagnostic(nextDiagnostic));
          } else {
            scheduleWarningClear(warningTimerRef, setDiagnostic);
          }
        }, (eventName, payload) => {
          notePlayerEvent(playerEventsRef, eventName, payload);
          if (isFirstFrameEvent(eventName)) {
            clearFirstFrameTimer(firstFrameTimerRef);
            setLoading(false);
            setLoadFailed(false);
            setDiagnostic((current) => (current?.stage === "first-frame-timeout" ? null : current));
          }
        });
        startFirstFrameTimer(firstFrameTimerRef, () => {
          if (cancelled) return;
          setLoading(false);
          setLoadFailed(true);
          const nextDiagnostic = {
            stage: "first-frame-timeout",
            message: `播放器已初始化，但 ${FIRST_FRAME_TIMEOUT_MS / 1000} 秒内没有收到首帧/流成功事件`,
            details: [
              ...commonDetails,
              `events=${playerEventsRef.current.length > 0 ? playerEventsRef.current.join(" > ") : "none"}`,
              `state=${safePlayerState(player)}`,
            ],
            severity: "error" as const,
          };
          setDiagnostic(nextDiagnostic);
          onError?.(formatDiagnostic(nextDiagnostic));
        });
      } catch (err) {
        if (cancelled) return;
        setLoading(false);
        setLoadFailed(true);
        const nextDiagnostic = {
          stage: "init-player",
          message: errorText(err),
          details: commonDetails,
          severity: "error" as const,
        };
        setDiagnostic(nextDiagnostic);
        onError?.(formatDiagnostic(nextDiagnostic));
      }
    }

    init();

    return () => {
      cancelled = true;
      clearWarningTimer(warningTimerRef);
      clearFirstFrameTimer(firstFrameTimerRef);
      if (playerRef.current) {
        stopPlayer(playerRef.current, containerId);
        playerRef.current = null;
      }
    };
  }, [url, isLive, onError, isMock, isNativeVideo, containerId, protocol, preferSoftDecode]);

  function toggleMute() {
    const next = !muted;
    setMuted(next);
    if (playerRef.current) {
      try {
        if (next) {
          playerRef.current.closeSound();
        } else {
          playerRef.current.openSound();
        }
      } catch {
        // ignore
      }
    }
  }

  return (
    <div className="h5-player-wrapper">
      {isMock ? (
        <div className="h5-player-container h5-player-mock">
          <img src={`https://picsum.photos/seed/${encodeURIComponent(url)}/960/540`} alt="模拟监控画面" />
          <span className="h5-player-mock-badge">{isLive ? "实时视频" : "录像回放"}</span>
        </div>
      ) : isNativeVideo ? (
        <video
          className="h5-player-container h5-native-video"
          src={url}
          controls
          playsInline
          autoPlay
          muted={muted}
          onLoadedMetadata={() => setLoading(false)}
          onCanPlay={() => setLoading(false)}
          onPlaying={() => setLoading(false)}
          onError={() => {
            setLoading(false);
            setLoadFailed(true);
            const nextDiagnostic = {
              stage: "native-video",
              message: "原生视频播放失败",
              details: [`url=${summarizeUrl(url)}`, `protocol=${protocol || inferProtocol(url)}`, `mode=${isLive ? "live" : "playback"}`],
              severity: "error" as const,
            };
            setDiagnostic(nextDiagnostic);
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
      {!loading && !loadFailed && (
        <button
          className={`h5-sound-toggle ${muted ? "muted" : "unmuted"}`}
          onClick={toggleMute}
          aria-label={muted ? "点击开启声音" : "点击关闭声音"}
        >
          {muted ? "点击开启声音" : "声音已开启"}
        </button>
      )}
    </div>
  );
}

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

function isRecoverablePlayerEvent(message: string) {
  const text = message.toLowerCase();
  return text.includes("mediasource") || text.includes("sourcebuffer") || text.includes("mse");
}

function isFirstFrameEvent(eventName: string) {
  return ["streamSuccess", "videoInfo", "videoFrame", "playing", "loaded"].includes(eventName);
}

function notePlayerEvent(eventRef: MutableRefObject<string[]>, eventName: string, payload: unknown) {
  const text = payload ? `${eventName}:${shortPayload(payload)}` : eventName;
  eventRef.current = [...eventRef.current.slice(-7), text];
}

function shouldUseNativeVideo(url: string, protocol?: string) {
  const normalized = (protocol || inferProtocol(url)).toLowerCase();
  return normalized === "hls" || normalized === "m3u8";
}

function isMobilePlaybackContext() {
  if (typeof navigator === "undefined") return false;
  const ua = navigator.userAgent.toLowerCase();
  return (
    navigator.maxTouchPoints > 1 &&
    /android|iphone|ipad|ipod|mobile|micromessenger|lark|feishu|bytedancewebview/.test(ua)
  );
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
  return [diagnostic.message, `stage=${diagnostic.stage}`, ...diagnostic.details].join(" · ");
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
