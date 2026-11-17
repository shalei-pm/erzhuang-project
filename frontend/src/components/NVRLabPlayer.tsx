import { useEffect, useRef, useState } from "react";
import NVRPlayer, { type PlaybackControlDiagnostics } from "../vendor/nvr-player/nvr-player.js";
import { H5PlayerControls } from "./H5PlayerControls";
import { PlaybackSegmentSlider, type PlaybackSegmentTiming } from "./PlaybackSegmentSlider";
import { PlaybackSpeedSheet } from "./PlaybackSpeedSheet";
import { PlaybackControlFailure } from "./PlaybackControlFailure";
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
  onStatus?: (status: NVRLabPlayerStatus, sourceSession?: NVRLabStreamSession) => void;
  onPlaybackStateChange?: (playing: boolean) => void;
  onSeekPlayback?: (startTime: number) => boolean | void;
  playbackMode?: boolean;
  playbackPaused?: boolean;
  playbackRate?: number;
  playbackRequestPending?: boolean;
  onPlaybackTime?: (elapsedSeconds: number, sourceSession: NVRLabStreamSession) => void;
  onPlaybackRateChange?: (rate: number) => void;
  onPlaybackRateError?: (message: string, sourceSession: NVRLabStreamSession) => void;
  onPausePlayback?: () => void;
  onResumePlayback?: () => void;
  onForwardPlayback?: () => boolean | void;
  captureOnFirstFrame?: boolean;
  onSnapshotCapture?: (image: Blob | null) => void;
  onScreenshot?: (dataUrl: string) => void | Promise<void>;
  onRetry: () => void;
};

export function NVRLabPlayer({
  session,
  playbackSegment = null,
  playbackCursorUnix = null,
  onStatus,
  onPlaybackStateChange,
  onSeekPlayback,
  playbackMode = false,
  playbackPaused = false,
  playbackRate = 1,
  playbackRequestPending = false,
  onPlaybackTime,
  onPlaybackRateChange,
  onPlaybackRateError,
  onPausePlayback,
  onResumePlayback,
  onForwardPlayback,
  captureOnFirstFrame = false,
  onSnapshotCapture,
  onScreenshot,
  onRetry,
}: NVRLabPlayerProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const playerRef = useRef<NVRPlayer | null>(null);
  const preparedPlayerRef = useRef<NVRPlayer | null>(null);
  const playerShellRef = useRef<HTMLDivElement | null>(null);
  const firstFrameCaptureRef = useRef(false);
  const mutedRef = useRef(true);
  const commandedRateRef = useRef(1);
  const speedCommandRef = useRef(0);
  const normalSpeedReadyRef = useRef(true);
  const readyPlayerRef = useRef<NVRPlayer | null>(null);
  const callbacks = useRef({ onStatus, onPlaybackStateChange, onSnapshotCapture, onPlaybackTime, onPlaybackRateError });
  callbacks.current = { onStatus, onPlaybackStateChange, onSnapshotCapture, onPlaybackTime, onPlaybackRateError };
  const [status, setStatus] = useState<NVRLabPlayerStatus>({ stage: "idle", message: "播放器准备中" });
  const [playing, setPlaying] = useState(false);
  const [muted, setMuted] = useState(true);
  const [audioMessage, setAudioMessage] = useState("");
  const [controlsVisible, setControlsVisible] = useState(true);
  const [fullscreen, setFullscreen] = useState(false);
  const [landscape, setLandscape] = useState(false);
  const [speedOpen, setSpeedOpen] = useState(false);
  const [speedPending, setSpeedPending] = useState(false);
  const [speedFailure, setSpeedFailure] = useState<PlaybackControlDiagnostics | null>(null);
  const isPlayback = playbackMode || session?.mode === "playback";
  const playbackAvailable = Boolean(playbackSegment && (session || playbackPaused));
  const ratePending = speedPending || Boolean(isPlayback && session && !playbackPaused && playbackRate !== commandedRateRef.current);
  const speedMuted = isPlayback && (playbackRate !== 1 || ratePending || !normalSpeedReadyRef.current);

  useEffect(() => {
    setPlaying(false);
    setSpeedPending(false);
    commandedRateRef.current = 1;
    normalSpeedReadyRef.current = true;
    readyPlayerRef.current = null;
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
      callbacks.current.onStatus?.(next, session);
    };
    const options = {
      autoReconnect: false,
      forceWasm: session.mode === "playback",
      wasmWorkerUrl: `${import.meta.env.BASE_URL.replace(/\/$/, "")}/nvr-player/wasm/systemTransform-worker.js`,
      audioWorkletUrl: `${import.meta.env.BASE_URL.replace(/\/$/, "")}/nvr-player/pcm-stream-worklet.js`,
      onConnected: () => report({ stage: "connected", message: "视频流已连接，等待画面" }),
      onFirstFrame: () => {
        if (disposed) return;
        firstFrameRendered = true;
        readyPlayerRef.current = player;
        clearFirstFrameTimeout();
        setPlaying(true);
        callbacks.current.onPlaybackStateChange?.(true);
        report({ stage: "first-frame", message: "画面已开始播放" });
        if (captureOnFirstFrame && !firstFrameCaptureRef.current) {
          firstFrameCaptureRef.current = true;
          canvasRef.current?.toBlob((image) => { if (!disposed) callbacks.current.onSnapshotCapture?.(image); }, "image/jpeg", 0.82);
        }
      },
      onDisconnected: (details: { code: number | null; wasClean: boolean }) => {
        if (disposed) return;
        clearFirstFrameTimeout();
        setPlaying(false);
        callbacks.current.onPlaybackStateChange?.(false);
        report({ stage: "error", message: nvrLabDisconnectMessage(details) });
      },
      onError: (error) => {
        if (disposed) return;
        clearFirstFrameTimeout();
        setPlaying(false);
        callbacks.current.onPlaybackStateChange?.(false);
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
      onPlaybackTime: (elapsedSeconds: number) => {
        if (!disposed) callbacks.current.onPlaybackTime?.(elapsedSeconds, session);
      },
    } satisfies ConstructorParameters<typeof NVRPlayer>[1];
    const prepared = preparedPlayerRef.current;
    preparedPlayerRef.current = null;
    const player = prepared && session.mode === "playback"
      ? Object.assign(prepared, options)
      : new NVRPlayer(canvasRef.current, options);
    if (prepared && prepared !== player) prepared.stop();
    player.setVolume(mutedRef.current || (session.mode === "playback" && playbackRate !== 1) ? 0 : 80);
    playerRef.current = player;
    firstFrameCaptureRef.current = false;
    setPlaying(false);
    callbacks.current.onPlaybackStateChange?.(false);
    setAudioMessage("");
    report({ stage: "connecting", message: "正在连接视频流" });
    void player.play(session.url).catch((error: unknown) => report({ stage: "error", message: playerErrorMessage(error) }));
    if (session.mode === "live") {
      firstFrameTimeout = window.setTimeout(() => {
        if (disposed || firstFrameRendered) return;
        setPlaying(false);
        callbacks.current.onPlaybackStateChange?.(false);
        report({ stage: "error", message: nvrLabFirstFrameTimeoutMessage(diagnostics) });
      }, 12_000);
    }

    return () => {
      disposed = true;
      clearFirstFrameTimeout();
      player.stop();
      callbacks.current.onPlaybackStateChange?.(false);
      if (playerRef.current === player) playerRef.current = null;
    };
  }, [captureOnFirstFrame, session]);

  useEffect(() => {
    if (preparedPlayerRef.current && (!isPlayback || (!playbackRequestPending && !session))) {
      preparedPlayerRef.current.stop();
      preparedPlayerRef.current = null;
    }
  });

  useEffect(() => () => {
    preparedPlayerRef.current?.stop();
    preparedPlayerRef.current = null;
  }, []);

  useEffect(() => {
    const player = playerRef.current;
    if (!session || !player || readyPlayerRef.current !== player || !isPlayback || status.stage !== "first-frame" || commandedRateRef.current === playbackRate) return;
    let cancelled = false;
    const command = ++speedCommandRef.current;
    // A pending 4x command may already affect the stream even while verifiedRate is still 1.
    commandedRateRef.current = playbackRate;
    normalSpeedReadyRef.current = false;
    setSpeedPending(true);
    setSpeedFailure(null);
    player.setVolume(0);
    void player.setSpeed(playbackRate).then(() => {
      if (cancelled || speedCommandRef.current !== command || playerRef.current !== player) return;
      const diagnostic = player.getPlaybackControlDiagnostics();
      // Measured 1x needs no ACK; accepted unconfirmed 1x does not claim a measured rate.
      normalSpeedReadyRef.current = playbackRate === 1 && (
        (diagnostic.phase === "verified" && diagnostic.verifiedRate === 1)
        || (diagnostic.phase === "unconfirmed" && diagnostic.requestedRate === 1 && diagnostic.ack === "accepted")
      );
      if (diagnostic.phase === "unconfirmed") setSpeedFailure(diagnostic);
      player.setVolume(normalSpeedReadyRef.current && !mutedRef.current ? 80 : 0);
      setSpeedPending(false);
    }).catch(() => {
      if (cancelled || speedCommandRef.current !== command || playerRef.current !== player) return;
      const diagnostic = player.getPlaybackControlDiagnostics();
      if (diagnostic.phase === "failed" && diagnostic.failure !== "uncorrelated") setSpeedFailure(diagnostic);
      setSpeedPending(false);
      callbacks.current.onPlaybackRateError?.(diagnostic.failure === "uncorrelated"
        ? "正在重新连接正常速度回放" : "录像倍速切换失败，正在重新连接正常速度回放", session);
    });
    return () => { cancelled = true; };
  }, [isPlayback, playbackRate, session, status.stage]);

  useEffect(() => { setSpeedOpen(false); }, [session, playbackMode]);
  useEffect(() => { if (!isPlayback) setSpeedFailure(null); }, [isPlayback]);

  useEffect(() => {
    const onFullscreenChange = () => setFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener("fullscreenchange", onFullscreenChange);
    return () => document.removeEventListener("fullscreenchange", onFullscreenChange);
  }, []);

  async function togglePlay() {
    if (isPlayback && onPausePlayback && onResumePlayback) {
      if (playbackPaused) {
        preparePlaybackAudio();
        onResumePlayback();
      }
      else {
        speedCommandRef.current += 1;
        setSpeedPending(false);
        playerRef.current?.stop();
        onPausePlayback();
      }
      return;
    }
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

  function preparePlaybackAudio(): NVRPlayer | null {
    if (mutedRef.current || !canvasRef.current) return null;
    preparedPlayerRef.current?.stop();
    const player = new NVRPlayer(canvasRef.current, {
      autoReconnect: false,
      forceWasm: true,
      wasmWorkerUrl: `${import.meta.env.BASE_URL.replace(/\/$/, "")}/nvr-player/wasm/systemTransform-worker.js`,
      audioWorkletUrl: `${import.meta.env.BASE_URL.replace(/\/$/, "")}/nvr-player/pcm-stream-worklet.js`,
    });
    preparedPlayerRef.current = player;
    player.setVolume(0);
    // The next signed session adopts this instance after the user gesture has unlocked audio.
    void player.enableAudio().catch(() => {
      if (preparedPlayerRef.current === player || playerRef.current === player) {
        mutedRef.current = true;
        setMuted(true);
        playerRef.current?.setVolume(0);
        preparedPlayerRef.current?.setVolume(0);
        setAudioMessage("声音暂未开启，请再次点击声音按钮");
      }
    });
    return player;
  }

  function discardPreparedAudio(player: NVRPlayer | null) {
    if (!player || preparedPlayerRef.current !== player) return;
    preparedPlayerRef.current = null;
    player.stop();
  }

  async function toggleSound() {
    if (speedMuted) return;
    const nextMuted = !muted;
    const player = playerRef.current;
    if (!player) return;
    if (!nextMuted) {
      try {
        await player.enableAudio();
      } catch {
        if (playerRef.current === player) setAudioMessage("声音暂未开启，请再次点击声音按钮");
        return;
      }
    }
    if (playerRef.current !== player) return;
    player.setVolume(nextMuted ? 0 : 80);
    setMuted(nextMuted);
    mutedRef.current = nextMuted;
    setAudioMessage("");
  }

  function screenshot() {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const dataUrl = canvas.toDataURL("image/png");
    if (onScreenshot) {
      void onScreenshot(dataUrl);
      return;
    }
    const anchor = document.createElement("a");
    anchor.href = dataUrl;
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
              state={{ playing: playbackPaused ? false : playing, muted: muted || speedMuted, loading: playbackRequestPending || Boolean(session && status.stage === "connecting"), failed: Boolean(session && status.stage === "error"), fullscreen, landscape }}
              visible={controlsVisible || playbackPaused || speedOpen}
              playback={isPlayback ? {
                rate: playbackRate,
                pending: ratePending,
                soundDisabled: speedMuted,
                disabled: !playbackAvailable,
                onForward: () => {
                  const prepared = playbackPaused ? null : preparePlaybackAudio();
                  if (onForwardPlayback?.() === false) discardPreparedAudio(prepared);
                },
                onSpeed: () => {
                  setSpeedFailure(null);
                  if (playbackRate !== 1) {
                    // Prepare Safari audio inside the click; keep it muted until 1x is measured or accepted.
                    const player = playerRef.current;
                    if (!mutedRef.current && player) {
                      void player.enableAudio().catch(() => {
                        if (playerRef.current === player) setAudioMessage("声音暂未开启，请再次点击声音按钮");
                      });
                    }
                    onPlaybackRateChange?.(1);
                  } else setSpeedOpen(true);
                },
              } : undefined}
              center={
                isPlayback && playbackSegment && onSeekPlayback ? (
                  <PlaybackSegmentSlider
                    segment={playbackSegment}
                    disabled={playbackRequestPending || ratePending || Boolean(session && (status.stage === "connecting" || status.stage === "error"))}
                    currentStartTime={playbackCursorUnix}
                    compactControls
                    onCommit={(target) => {
                      const prepared = playbackPaused ? null : preparePlaybackAudio();
                      if (onSeekPlayback(target) === false) discardPreparedAudio(prepared);
                    }}
                  />
                ) : (
                  <span>{isPlayback ? "录像" : "实时视频"}</span>
                )
              }
              onTogglePlay={() => void togglePlay()}
              onToggleSound={() => void toggleSound()}
              onScreenshot={screenshot}
              onToggleLandscape={() => setLandscape((value) => !value)}
              onToggleFullscreen={() => void toggleFullscreen()}
            />
            {playbackRequestPending || (session && status.stage === "connecting") ? <div className="h5-player-loading">正在连接视频流...</div> : null}
            {session && status.stage === "error" ? (
              <div className="h5-player-error">
                <span>{status.message}</span>
                <button type="button" onClick={onRetry}>重新连接</button>
              </div>
            ) : null}
          </div>
        </div>
      </div>
      {isPlayback && speedFailure ? <PlaybackControlFailure key={`${speedFailure.commandSequence}-${speedFailure.requestedRate}`}
        diagnostic={speedFailure} /> : null}
      {speedOpen ? <PlaybackSpeedSheet onClose={() => setSpeedOpen(false)} onSelect={(rate) => {
        setSpeedOpen(false);
        onPlaybackRateChange?.(rate);
      }} /> : null}
      </div>
      {audioMessage ? <div className="h5-error" role="status">{audioMessage}</div> : null}
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
