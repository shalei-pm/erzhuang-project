import { useEffect, useRef, useState } from "react";
import NVRPlayer from "../vendor/nvr-player/nvr-player.js";
import { H5PlayerControls } from "./H5PlayerControls";
import type { NVRLabStreamSession } from "../domain/nvr-lab";

export type NVRLabPlayerStatus = {
  stage: "idle" | "connecting" | "connected" | "first-frame" | "error";
  message: string;
};

type NVRLabPlayerProps = {
  session: NVRLabStreamSession | null;
  onStatus?: (status: NVRLabPlayerStatus) => void;
  onRetry: () => void;
};

export function NVRLabPlayer({ session, onStatus, onRetry }: NVRLabPlayerProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const playerRef = useRef<NVRPlayer | null>(null);
  const playerShellRef = useRef<HTMLDivElement | null>(null);
  const [status, setStatus] = useState<NVRLabPlayerStatus>({ stage: "idle", message: "播放器准备中" });
  const [playing, setPlaying] = useState(false);
  const [muted, setMuted] = useState(true);
  const [controlsVisible, setControlsVisible] = useState(true);
  const [fullscreen, setFullscreen] = useState(false);
  const [landscape, setLandscape] = useState(false);

  useEffect(() => {
    if (!session || !canvasRef.current) return;
    let disposed = false;
    const report = (next: NVRLabPlayerStatus) => {
      if (disposed) return;
      setStatus(next);
      onStatus?.(next);
    };
    const player = new NVRPlayer(canvasRef.current, {
      autoReconnect: false,
      forceWasm: session.mode === "playback",
      wasmWorkerUrl: `${import.meta.env.BASE_URL.replace(/\/$/, "")}/nvr-player/wasm/systemTransform-worker.js`,
      onConnected: () => report({ stage: "connected", message: "视频流已连接，等待画面" }),
      onFirstFrame: () => {
        setPlaying(true);
        report({ stage: "first-frame", message: "画面已开始播放" });
      },
      onDisconnected: () => {
        setPlaying(false);
        report({ stage: "error", message: "视频流已断开，请重新连接" });
      },
      onError: (error) => {
        setPlaying(false);
        report({ stage: "error", message: playerErrorMessage(error) });
      },
    });
    playerRef.current = player;
    setPlaying(false);
    setMuted(true);
    report({ stage: "connecting", message: "正在连接视频流" });
    void player.play(session.url).catch((error: unknown) => report({ stage: "error", message: playerErrorMessage(error) }));

    return () => {
      disposed = true;
      player.stop();
      if (playerRef.current === player) playerRef.current = null;
    };
  }, [session, onStatus]);

  useEffect(() => {
    const onFullscreenChange = () => setFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener("fullscreenchange", onFullscreenChange);
    return () => document.removeEventListener("fullscreenchange", onFullscreenChange);
  }, []);

  async function togglePlay() {
    const player = playerRef.current;
    if (!player || status.stage === "error") return;
    if (playing) {
      player.pause();
      setPlaying(false);
      return;
    }
    player.resume();
    setPlaying(true);
  }

  function toggleSound() {
    const nextMuted = !muted;
    playerRef.current?.setVolume(nextMuted ? 0 : 80);
    setMuted(nextMuted);
  }

  function screenshot() {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const anchor = document.createElement("a");
    anchor.href = canvas.toDataURL("image/png");
    anchor.download = "nvr-camera.png";
    anchor.click();
  }

  async function toggleFullscreen() {
    const shell = playerShellRef.current;
    if (!shell) return;
    if (document.fullscreenElement) {
      await document.exitFullscreen?.();
      return;
    }
    await shell.requestFullscreen?.();
  }

  return (
    <div ref={playerShellRef} className={`h5-player-shell ${landscape ? "is-landscape" : ""}`}>
      <div className="h5-player-rotator">
        <div className="h5-player-wrapper">
          <div className="h5-player-container" onClick={() => setControlsVisible((visible) => !visible)}>
            <canvas ref={canvasRef} aria-label="NVR 视频画面" />
            <H5PlayerControls
              state={{ playing, muted, loading: status.stage === "connecting", failed: status.stage === "error", fullscreen, landscape }}
              visible={controlsVisible}
              center={<span>{session?.mode === "playback" ? "录像" : "实时视频"}</span>}
              onTogglePlay={() => void togglePlay()}
              onToggleSound={toggleSound}
              onScreenshot={screenshot}
              onToggleLandscape={() => setLandscape((value) => !value)}
              onToggleFullscreen={() => void toggleFullscreen()}
            />
            {status.stage === "connecting" ? <div className="h5-player-loading">正在连接视频流...</div> : null}
            {status.stage === "error" ? (
              <div className="h5-player-error">
                <span>{status.message}</span>
                <button type="button" onClick={onRetry}>重新连接</button>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}

function playerErrorMessage(error: unknown): string {
  const message = error instanceof Error ? error.message : "";
  if (message.includes("WebCodecs")) return "当前浏览器暂不支持该直播编码格式";
  if (message.includes("WASM")) return "回放解码器加载失败，请稍后重试";
  return "播放器连接失败，请重新连接";
}
