import { useEffect, useRef, useState } from "react";
import NVRPlayer from "../vendor/nvr-player/nvr-player.js";
import { H5PlayerControls } from "./H5PlayerControls";
import { PlaybackSegmentSlider, type PlaybackSegmentTiming } from "./PlaybackSegmentSlider";
import type { NVRLabStreamSession } from "../domain/nvr-lab";

export type NVRLabPlayerStatus = {
  stage: "idle" | "connecting" | "connected" | "first-frame" | "error";
  message: string;
};

export type NVRLabPlayerDiagnostics = {
  receivedPackets: number;
  rtpPackets: number;
  videoPayloadPackets: number;
  vpsPackets: number;
  spsPackets: number;
  ppsPackets: number;
  keyFrameNALUnits: number;
  decoderInputFrames: number;
  renderedFrames: number;
};

type NVRLabPlayerProps = {
  session: NVRLabStreamSession | null;
  playbackSegment?: PlaybackSegmentTiming | null;
  playbackCursorUnix?: number | null;
  onStatus?: (status: NVRLabPlayerStatus) => void;
  onPlaybackStateChange?: (playing: boolean) => void;
  onSeekPlayback?: (startTime: number) => void;
  captureOnFirstFrame?: boolean;
  onSnapshotCapture?: (image: Blob | null) => void;
  onRetry: () => void;
};

export function NVRLabPlayer({
  session,
  playbackSegment = null,
  playbackCursorUnix = null,
  onStatus,
  onPlaybackStateChange,
  onSeekPlayback,
  captureOnFirstFrame = false,
  onSnapshotCapture,
  onRetry,
}: NVRLabPlayerProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const playerRef = useRef<NVRPlayer | null>(null);
  const playerShellRef = useRef<HTMLDivElement | null>(null);
  const firstFrameCaptureRef = useRef(false);
  const [status, setStatus] = useState<NVRLabPlayerStatus>({ stage: "idle", message: "播放器准备中" });
  const [playing, setPlaying] = useState(false);
  const [muted, setMuted] = useState(true);
  const [controlsVisible, setControlsVisible] = useState(true);
  const [fullscreen, setFullscreen] = useState(false);
  const [landscape, setLandscape] = useState(false);

  useEffect(() => {
    if (!session || !canvasRef.current) return;
    let disposed = false;
    let firstFrameRendered = false;
    const diagnostics: NVRLabPlayerDiagnostics = {
      receivedPackets: 0,
      rtpPackets: 0,
      videoPayloadPackets: 0,
      vpsPackets: 0,
      spsPackets: 0,
      ppsPackets: 0,
      keyFrameNALUnits: 0,
      decoderInputFrames: 0,
      renderedFrames: 0,
    };
    let firstFrameTimeout: number | null = null;
    const clearFirstFrameTimeout = () => {
      if (firstFrameTimeout !== null) {
        window.clearTimeout(firstFrameTimeout);
        firstFrameTimeout = null;
      }
    };
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
        firstFrameRendered = true;
        clearFirstFrameTimeout();
        setPlaying(true);
        onPlaybackStateChange?.(true);
        report({ stage: "first-frame", message: "画面已开始播放" });
        if (captureOnFirstFrame && !firstFrameCaptureRef.current) {
          firstFrameCaptureRef.current = true;
          canvasRef.current?.toBlob((image) => onSnapshotCapture?.(image), "image/jpeg", 0.82);
        }
      },
      onDisconnected: (details: { code: number | null; wasClean: boolean }) => {
        clearFirstFrameTimeout();
        setPlaying(false);
        onPlaybackStateChange?.(false);
        report({ stage: "error", message: nvrLabDisconnectMessage(details) });
      },
      onError: (error) => {
        clearFirstFrameTimeout();
        setPlaying(false);
        onPlaybackStateChange?.(false);
        report({ stage: "error", message: playerErrorMessage(error) });
      },
      onDiagnostics: (next: NVRLabPlayerDiagnostics) => {
        diagnostics.receivedPackets = next.receivedPackets;
        diagnostics.rtpPackets = next.rtpPackets;
        diagnostics.videoPayloadPackets = next.videoPayloadPackets;
        diagnostics.vpsPackets = next.vpsPackets;
        diagnostics.spsPackets = next.spsPackets;
        diagnostics.ppsPackets = next.ppsPackets;
        diagnostics.keyFrameNALUnits = next.keyFrameNALUnits;
        diagnostics.decoderInputFrames = next.decoderInputFrames;
        diagnostics.renderedFrames = next.renderedFrames;
      },
    });
    playerRef.current = player;
    firstFrameCaptureRef.current = false;
    setPlaying(false);
    onPlaybackStateChange?.(false);
    setMuted(true);
    report({ stage: "connecting", message: "正在连接视频流" });
    void player.play(session.url).catch((error: unknown) => report({ stage: "error", message: playerErrorMessage(error) }));
    if (session.mode === "live") {
      firstFrameTimeout = window.setTimeout(() => {
        if (disposed || firstFrameRendered) return;
        setPlaying(false);
        onPlaybackStateChange?.(false);
        report({ stage: "error", message: nvrLabFirstFrameTimeoutMessage(diagnostics) });
      }, 12_000);
    }

    return () => {
      disposed = true;
      clearFirstFrameTimeout();
      player.stop();
      onPlaybackStateChange?.(false);
      if (playerRef.current === player) playerRef.current = null;
    };
  }, [captureOnFirstFrame, onPlaybackStateChange, onSnapshotCapture, onStatus, session]);

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
      onPlaybackStateChange?.(false);
      return;
    }
    player.resume();
    setPlaying(true);
    onPlaybackStateChange?.(true);
  }

  async function toggleSound() {
    const nextMuted = !muted;
    const player = playerRef.current;
    if (!player) return;
    if (!nextMuted) {
      try {
        await player.enableAudio();
      } catch {
        setStatus({ stage: "error", message: "当前浏览器无法开启声音" });
        return;
      }
    }
    player.setVolume(nextMuted ? 0 : 80);
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
    <>
      <div ref={playerShellRef} className={`h5-player-shell ${landscape ? "is-landscape" : ""}`}>
      <div className="h5-player-rotator">
        <div className="h5-player-wrapper">
          <div className="h5-player-container" onClick={() => setControlsVisible((visible) => !visible)}>
            <canvas ref={canvasRef} aria-label="NVR 视频画面" />
            <H5PlayerControls
              state={{ playing, muted, loading: status.stage === "connecting", failed: status.stage === "error", fullscreen, landscape }}
              visible={controlsVisible}
              center={
                session?.mode === "playback" && playbackSegment && onSeekPlayback ? (
                  <PlaybackSegmentSlider
                    segment={playbackSegment}
                    disabled={status.stage === "connecting" || status.stage === "error"}
                    currentStartTime={playbackCursorUnix}
                    compactControls
                    onCommit={onSeekPlayback}
                  />
                ) : (
                  <span>{session?.mode === "playback" ? "录像" : "实时视频"}</span>
                )
              }
              onTogglePlay={() => void togglePlay()}
              onToggleSound={() => void toggleSound()}
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
    </>
  );
}

export function nvrLabFirstFrameTimeoutMessage(diagnostics: Partial<NVRLabPlayerDiagnostics>): string {
  if ((diagnostics.receivedPackets || 0) <= 0) return "视频流已连接，但未收到摄像头媒体数据，请稍后重试";
  if ((diagnostics.rtpPackets || 0) <= 0) return "视频流已收到，但数据格式不是有效 RTP，请确认工控机转发协议";
  if ((diagnostics.videoPayloadPackets || 0) <= 0) return "视频流已收到 RTP 数据，但未收到 PT=96 视频包，请确认工控机转发配置";
  if ((diagnostics.vpsPackets || 0) <= 0 || (diagnostics.spsPackets || 0) <= 0 || (diagnostics.ppsPackets || 0) <= 0) {
    return "视频流已收到 PT=96 视频包，但缺少 H.265 解码参数集，请确认该通道编码";
  }
  if ((diagnostics.keyFrameNALUnits || 0) <= 0) return "视频流已收到，但未收到 H.265 关键帧，请确认工控机关键帧转发";
  if ((diagnostics.decoderInputFrames || 0) <= 0) return "视频流已收到，但无法送入 H.265 解码器，请确认工控机转发格式";
  return "视频流已收到，但当前浏览器无法解码该摄像头画面";
}

export function nvrLabDisconnectMessage(details: { code: number | null; wasClean: boolean }): string {
  if (details.code === 1000) return "视频流已正常结束，当前摄像头可能未持续推流，请重新连接";
  if (details.code === 1008) return "视频流会话被服务拒绝，请重新连接";
  if (details.code === 1011) return "视频流服务异常，请稍后重新连接";
  if (details.code === 1006 || !details.wasClean) return "视频流连接异常中断，请重新连接";
  return "视频流已断开，请重新连接";
}

function playerErrorMessage(error: unknown): string {
  const message = error instanceof Error ? error.message : "";
  if (message.includes("WebCodecs")) return "当前浏览器暂不支持该直播编码格式";
  if (message.includes("WASM")) return "回放解码器加载失败，请稍后重试";
  return "播放器连接失败，请重新连接";
}
