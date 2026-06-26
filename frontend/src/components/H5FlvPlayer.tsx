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

let playerLib: EzuikitFlvConstructor | null = null;
let playerLibPromise: Promise<EzuikitFlvConstructor | null> | null = null;
const DECODER_PATH = `${import.meta.env.BASE_URL.replace(/\/$/, "")}/assets/ezuikit-flv/decoder.js`;

async function loadPlayerLib(): Promise<EzuikitFlvConstructor | null> {
  if (playerLib) return playerLib;
  if (playerLibPromise) return playerLibPromise;

  playerLibPromise = (async () => {
    try {
      const mod = (await import("ezuikit-flv")) as unknown;
      const Ctor = (mod as unknown as { EzuikitFlv?: EzuikitFlvConstructor }).EzuikitFlv
        ?? (mod as unknown as EzuikitFlvConstructor)
        ?? (mod as unknown as { default?: EzuikitFlvConstructor }).default
        ?? null;
      playerLib = Ctor;
      return Ctor;
    } catch {
      // Player lib not available; fallback will be used.
      return null;
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
  const isMock = url.startsWith("mock-");

  useEffect(() => {
    if (isMock) {
      setLoading(false);
      setLoadFailed(false);
      return;
    }
    let cancelled = false;

    async function init() {
      setLoading(true);
      const Ctor = await loadPlayerLib();
      if (cancelled) return;

      if (!Ctor) {
        setLoadFailed(true);
        setLoading(false);
        onError?.("播放器加载失败，请检查网络或刷新页面。");
        return;
      }

      try {
        playerRef.current = new Ctor({
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
        setLoading(false);
      } catch (err) {
        if (cancelled) return;
        setLoading(false);
        setLoadFailed(true);
        onError?.(err instanceof Error ? err.message : "播放器初始化失败");
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
