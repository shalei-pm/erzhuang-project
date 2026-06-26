import { useEffect, useRef, useState } from "react";

// ezuikit-flv is loaded dynamically to avoid SSR issues and to keep
// the decoder path configurable.
interface EzuikitFlvPlayer {
  destroy: () => void;
  closeSound: () => void;
  openSound: () => void;
  play: () => void;
  pause: () => void;
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
  }): EzuikitFlvPlayer;
}

type PlayerDiagnostic = {
  stage: string;
  message: string;
  details: string[];
};

let playerLib: EzuikitFlvConstructor | null = null;
let playerLibPromise: Promise<{ ctor: EzuikitFlvConstructor | null; diagnostics: string[] }> | null = null;
const DECODER_PATH = `${import.meta.env.BASE_URL.replace(/\/$/, "")}/assets/ezuikit-flv/decoder.js`;

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
  isLive: boolean;
  onError?: (message: string) => void;
}

export function H5FlvPlayer({ url, isLive, onError }: H5FlvPlayerProps) {
  const containerId = useRef(`player-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`);
  const playerRef = useRef<EzuikitFlvPlayer | null>(null);
  const [muted, setMuted] = useState(true);
  const [loading, setLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [diagnostic, setDiagnostic] = useState<PlayerDiagnostic | null>(null);
  const isMock = url.startsWith("mock-");

  useEffect(() => {
    if (isMock) {
      setLoading(false);
      setLoadFailed(false);
      setDiagnostic(null);
      return;
    }
    let cancelled = false;

    async function init() {
      setLoading(true);
      setLoadFailed(false);
      setDiagnostic(null);
      const loaded = await loadPlayerLib();
      if (cancelled) return;

      const commonDetails = [
        `url=${summarizeUrl(url)}`,
        `decoder=${DECODER_PATH}`,
        `mode=${isLive ? "live" : "playback"}`,
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
        };
        setDiagnostic(nextDiagnostic);
        onError?.(formatDiagnostic(nextDiagnostic));
        return;
      }

      try {
        const player = new Ctor({
          id: containerId.current,
          url,
          decoder: DECODER_PATH,
          autoPlay: true,
          isLive,
          muted: true,
          autoWasm: true,
          useMSE: true,
          useWCS: true,
        });
        playerRef.current = player;
        attachPlayerDiagnostics(player, (message) => {
          const nextDiagnostic = {
            stage: "player-event",
            message,
            details: commonDetails,
          };
          setDiagnostic(nextDiagnostic);
          onError?.(formatDiagnostic(nextDiagnostic));
        });
        setLoading(false);
      } catch (err) {
        if (cancelled) return;
        setLoading(false);
        setLoadFailed(true);
        const nextDiagnostic = {
          stage: "init-player",
          message: errorText(err),
          details: commonDetails,
        };
        setDiagnostic(nextDiagnostic);
        onError?.(formatDiagnostic(nextDiagnostic));
      }
    }

    init();

    return () => {
      cancelled = true;
      if (playerRef.current) {
        try {
          playerRef.current.destroy();
        } catch {
          // ignore
        }
        playerRef.current = null;
      }
    };
  }, [url, isLive, onError, isMock]);

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
      ) : (
        <div id={containerId.current} className="h5-player-container" />
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

function attachPlayerDiagnostics(player: EzuikitFlvPlayer, onPlayerError: (message: string) => void) {
  const maybeEmitter = player as unknown as { on?: (event: string, handler: (payload: unknown) => void) => void };
  if (typeof maybeEmitter.on !== "function") {
    return;
  }
  for (const eventName of ["error", "streamError", "playError", "fetchError", "audioCodecUnsupported"]) {
    try {
      maybeEmitter.on(eventName, (payload) => onPlayerError(`${eventName}:${serializePayload(payload)}`));
    } catch {
      // Some player versions may not support every event name.
    }
  }
}

function PlayerDiagnosticPanel({ diagnostic }: { diagnostic: PlayerDiagnostic }) {
  return (
    <div className="h5-player-diagnostic" aria-label="播放器错误详情">
      <strong>{diagnostic.stage}</strong>
      <span>{diagnostic.message}</span>
      {diagnostic.details.map((item) => (
        <code key={item}>{item}</code>
      ))}
    </div>
  );
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

function redactSignedUrls(value: string) {
  return value.replace(/https?:\/\/[^\s"'<>]+/g, (rawUrl) => summarizeUrl(rawUrl));
}
