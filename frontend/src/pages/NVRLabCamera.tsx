import { useCallback, useEffect, useRef, useState } from "react";
import { nvrLabApi, NVRLabApiError } from "../api-nvr-lab";
import { NVRLabPlayer, type NVRLabPlayerStatus } from "../components/NVRLabPlayer";
import type { PlaybackSegmentTiming } from "../components/PlaybackSegmentSlider";
import { PlaybackDatePicker, initialPlaybackDateTimeValue } from "../components/PlaybackDatePicker";
import { SystemTopBar } from "../components/SystemTopBar";
import {
  NVR_LAB_MAX_PLAYBACK_SECONDS,
  buildNVRLabHourlyPlayback,
  buildNVRLabPlaybackFromStart,
  buildNVRLabPlaybackSession,
  nvrLabCameraSubtitle,
  nvrLabCameraTitle,
  nvrPresentedPlaybackCursor,
  nvrPlaybackForward,
  type NVRLabCamera,
  type NVRLabMode,
  type NVRLabPlaybackRange,
  type NVRLabStreamSession,
} from "../domain/nvr-lab";
import { isIdleSessionTimeout, type AuthState } from "../domain/auth";
import { composeMonitorScreenshot } from "../domain/screenshot-watermark";

type NVRLabCameraProps = {
	externalOrgId: string;
  cameraId: number;
  auth: AuthState | null;
  loggingOut: boolean;
  authMessage: string;
  onLogout: () => void | Promise<void>;
  onAuthRequired: (error?: unknown) => void;
  onBack: () => void;
};

export function NVRLabCamera({ externalOrgId, cameraId, auth, loggingOut, authMessage, onLogout, onAuthRequired, onBack }: NVRLabCameraProps) {
  const [camera, setCamera] = useState<NVRLabCamera | null>(null);
  const [mode, setMode] = useState<NVRLabMode>("live");
  const [session, setSession] = useState<NVRLabStreamSession | null>(null);
  const [message, setMessage] = useState("");
  const [playerStatus, setPlayerStatus] = useState<NVRLabPlayerStatus>({ stage: "idle", message: "播放器准备中" });
  const [playbackStartAt, setPlaybackStartAt] = useState(() => playbackQueryValue("start_at") || initialPlaybackDateTimeValue(new Date(Date.now() - NVR_LAB_MAX_PLAYBACK_SECONDS * 1000)));
  const [activePlaybackRange, setActivePlaybackRange] = useState<NVRLabPlaybackRange | null>(null);
  const [playbackCursorUnix, setPlaybackCursorUnix] = useState<number | null>(null);
  const [playbackPaused, setPlaybackPaused] = useState(false);
  const [playbackRate, setPlaybackRate] = useState(1);
  const [requestPending, setRequestPending] = useState(false);
  const [snapshotBackfillState, setSnapshotBackfillState] = useState<"waiting" | "uploading" | "succeeded" | "failed">("waiting");
  const activePlaybackRangeRef = useRef<NVRLabPlaybackRange | null>(null);
  const playbackCursorRef = useRef<number | null>(null);
  const modeRef = useRef<NVRLabMode>("live");
  const requestSequenceRef = useRef(0);
  const activeSessionRef = useRef<NVRLabStreamSession | null>(null);
  const playbackAnchorRef = useRef<number | null>(null);
  const playbackPausedRef = useRef(false);
  const lastCursorRenderRef = useRef(0);
  const playbackRange = buildNVRLabPlaybackFromStart(playbackStartAt);
  const playbackSegment: PlaybackSegmentTiming | null = activePlaybackRange
    ? { start_time: activePlaybackRange.startTime, end_time: activePlaybackRange.endTime }
    : null;
  const snapshotBackfillEnabled = new URLSearchParams(window.location.search).get("snapshot_backfill") === "1";

  useEffect(() => {
    let cancelled = false;
    nvrLabApi
      .listCameras(externalOrgId)
      .then((response) => {
        if (cancelled) return;
        const found = response.cameras.find((item) => item.id === cameraId) || null;
        setCamera(found);
        if (!found) setMessage("未找到可用摄像头");
      })
      .catch((error) => {
        if (cancelled) return;
        if (error instanceof NVRLabApiError && error.status === 401) {
          onAuthRequired(error);
          return;
        }
        setMessage(error instanceof Error ? error.message : "摄像头信息加载失败");
      });
    return () => {
      cancelled = true;
    };
  }, [cameraId, externalOrgId, onAuthRequired]);

  const requestSession = useCallback(
    async (nextMode: NVRLabMode, nextStart?: number, nextEnd?: number) => {
      const sequence = ++requestSequenceRef.current;
      activeSessionRef.current = null;
      setSession(null);
      playbackAnchorRef.current = null;
      setRequestPending(true);
      setMessage("");
      try {
        const nextSession = await nvrLabApi.createStreamSession(externalOrgId, cameraId, nextMode, nextStart, nextEnd);
        if (sequence !== requestSequenceRef.current) return;
        playbackAnchorRef.current = nextMode === "playback" ? nextStart ?? null : null;
        activeSessionRef.current = nextSession;
        setSession(nextSession);
      } catch (error) {
        if (sequence !== requestSequenceRef.current) return;
        if (nextMode === "playback") {
          playbackPausedRef.current = true;
          setPlaybackPaused(true);
        }
        if (error instanceof NVRLabApiError && error.status === 401) {
          onAuthRequired(error);
          return;
        }
        if (error instanceof NVRLabApiError) {
          setMessage(`${error.message}（${error.code || `HTTP ${error.status}`}）`);
          return;
        }
        setMessage(error instanceof Error ? error.message : "播放会话创建失败");
      } finally {
        if (sequence === requestSequenceRef.current) setRequestPending(false);
      }
    },
	[externalOrgId, cameraId, onAuthRequired],
  );

  useEffect(() => {
    void requestSession("live");
    return () => {
      requestSequenceRef.current++;
      activeSessionRef.current = null;
    };
  }, [requestSession]);

  function switchMode(nextMode: NVRLabMode) {
    if (nextMode === modeRef.current) return;
    requestSequenceRef.current++;
    activeSessionRef.current = null;
    modeRef.current = nextMode;
    setMode(nextMode);
    setSession(null);
    setRequestPending(false);
    setMessage("");
    resetPlaybackProgress();
    if (nextMode === "live") void requestSession("live");
  }

  const playPlayback = useCallback((playbackWindow: NVRLabPlaybackRange, startTime = playbackWindow.startTime) => {
    const nextSession = buildNVRLabPlaybackSession(playbackWindow, Math.floor(startTime));
    if (!nextSession) return;
    setMessage("");
    setPlayerStatus({ stage: "connecting", message: "正在连接视频流" });
    activePlaybackRangeRef.current = nextSession.playbackWindow;
    playbackCursorRef.current = nextSession.startTime;
    setActivePlaybackRange(nextSession.playbackWindow);
    setPlaybackCursorUnix(nextSession.startTime);
    playbackPausedRef.current = false;
    setPlaybackPaused(false);
    void requestSession("playback", nextSession.startTime, nextSession.endTime);
  }, [requestSession]);

  function retrySession() {
    if (mode === "live") {
      void requestSession("live");
      return;
    }
    const retryRange = activePlaybackRange || playbackRange;
    if (retryRange) playPlayback(retryRange, playbackCursorRef.current ?? retryRange.startTime);
  }

  const handlePlayerStatus = useCallback((status: NVRLabPlayerStatus, sourceSession?: NVRLabStreamSession) => {
    if (sourceSession && sourceSession !== activeSessionRef.current) return;
    setPlayerStatus(status);
  }, []);

  const handlePlaybackTime = useCallback((elapsedSeconds: number, sourceSession: NVRLabStreamSession, recordedUnixSeconds: number | null = null) => {
    const range = activePlaybackRangeRef.current;
    const anchor = playbackAnchorRef.current;
    if (sourceSession !== activeSessionRef.current || modeRef.current !== "playback" || playbackPausedRef.current || !range || anchor === null) return;
    const cursor = nvrPresentedPlaybackCursor(anchor, elapsedSeconds, range.endTime, recordedUnixSeconds, range.startTime);
    if (cursor === null) return;
    playbackCursorRef.current = cursor;
    if (performance.now() - lastCursorRenderRef.current >= 100 || cursor >= range.endTime - 1) {
      lastCursorRenderRef.current = performance.now();
      setPlaybackCursorUnix(cursor);
    }
    if (cursor >= range.endTime - 1) {
      playbackPausedRef.current = true;
      activeSessionRef.current = null;
      setPlaybackPaused(true);
      setSession(null);
    }
  }, []);

  const pausePlayback = useCallback(() => {
    requestSequenceRef.current++;
    activeSessionRef.current = null;
    playbackPausedRef.current = true;
    setPlaybackPaused(true);
    setPlaybackCursorUnix(playbackCursorRef.current);
    setSession(null);
    setRequestPending(false);
    setPlayerStatus({ stage: "idle", message: "录像已暂停" });
  }, []);

  const resumePlayback = useCallback(() => {
    const range = activePlaybackRangeRef.current;
    if (range) playPlayback(range, playbackCursorRef.current ?? range.startTime);
  }, [playPlayback]);

  const restoreNormalPlayback = useCallback(() => {
    setPlaybackRate(1);
    const range = activePlaybackRangeRef.current;
    if (range && !playbackPausedRef.current) playPlayback(range, playbackCursorRef.current ?? range.startTime);
  }, [playPlayback]);

  const handlePlaybackRateError = useCallback((errorMessage: string, sourceSession: NVRLabStreamSession) => {
    if (sourceSession !== activeSessionRef.current) return;
    setPlaybackRate(1);
    const range = activePlaybackRangeRef.current;
    if (range && !playbackPausedRef.current) playPlayback(range, playbackCursorRef.current ?? range.startTime);
    setMessage(errorMessage);
  }, [playPlayback]);

  const uploadFirstFrameSnapshot = useCallback(async (image: Blob | null) => {
    if (!image || image.type !== "image/jpeg" || image.size === 0) {
      setSnapshotBackfillState("failed");
      return;
    }
    setSnapshotBackfillState("uploading");
    try {
      await nvrLabApi.uploadSnapshot(externalOrgId, cameraId, image);
      setSnapshotBackfillState("succeeded");
    } catch (error) {
      if (isIdleSessionTimeout(error)) {
        onAuthRequired(error);
        return;
      }
      setSnapshotBackfillState("failed");
    }
  }, [cameraId, externalOrgId, onAuthRequired]);

  const seekPlayback = useCallback((startTime: number) => {
    const range = activePlaybackRangeRef.current;
    if (!range || !Number.isFinite(startTime) || startTime < range.startTime || startTime >= range.endTime) return false;
    const target = Math.floor(startTime);
    if (target === Math.floor(playbackCursorRef.current ?? -1)) return false;
    if (playbackPausedRef.current) {
      playbackCursorRef.current = target;
      setPlaybackCursorUnix(target);
      return true;
    }
    playPlayback(range, startTime);
    return true;
  }, [playPlayback]);

  const forwardPlayback = useCallback(() => {
    const range = activePlaybackRangeRef.current;
    return range ? seekPlayback(nvrPlaybackForward(playbackCursorRef.current ?? range.startTime, range.startTime, range.endTime)) : false;
  }, [seekPlayback]);

  const handleScreenshot = useCallback(async (dataUrl: string) => {
    try {
      const metadata = await nvrLabApi.getScreenshotMetadata(externalOrgId, cameraId);
      const result = await composeMonitorScreenshot(dataUrl, metadata);
      downloadMonitorScreenshot(result);
      setMessage("");
    } catch (error) {
      if (isIdleSessionTimeout(error)) {
        onAuthRequired(error);
        return;
      }
      setMessage(error instanceof Error ? `截图失败：${error.message}` : "截图失败，请稍后重试");
    }
  }, [cameraId, externalOrgId, onAuthRequired]);

  function resetPlaybackProgress() {
    activePlaybackRangeRef.current = null;
    playbackCursorRef.current = null;
    playbackAnchorRef.current = null;
    playbackPausedRef.current = false;
    setActivePlaybackRange(null);
    setPlaybackCursorUnix(null);
    setPlaybackPaused(false);
    setPlaybackRate(1);
  }

  return (
    <div className="h5-page h5-channel-page nvr-lab-page">
      <SystemTopBar backAction={{ label: "返回监控列表", onClick: onBack }} auth={auth} loggingOut={loggingOut} onLogout={onLogout} />
      {authMessage ? <div className="h5-auth-message">{authMessage}</div> : null}
      <main className="h5-viewer" data-snapshot-backfill-state={snapshotBackfillEnabled ? snapshotBackfillState : undefined}>
        <header className="h5-viewer-header">
          <div className="h5-viewer-title">
            <h1>{camera ? nvrLabCameraTitle(camera) : "摄像头监控"}</h1>
            <span>{camera ? nvrLabCameraSubtitle(camera) : "工控机监控"}</span>
          </div>
        </header>
        <section className="h5-viewer-player" aria-label="监控画面">
          <NVRLabPlayer
            session={session}
            playbackSegment={playbackSegment}
            playbackCursorUnix={playbackCursorUnix}
            onRetry={retrySession}
            onStatus={handlePlayerStatus}
            playbackMode={mode === "playback"}
            playbackPaused={playbackPaused}
            playbackRate={playbackRate}
            playbackRequestPending={requestPending}
            onPlaybackTime={handlePlaybackTime}
            onPausePlayback={pausePlayback}
            onResumePlayback={resumePlayback}
            onRestoreNormalPlayback={restoreNormalPlayback}
            onForwardPlayback={forwardPlayback}
            onPlaybackRateChange={setPlaybackRate}
            onPlaybackRateError={handlePlaybackRateError}
            onSeekPlayback={seekPlayback}
            captureOnFirstFrame={snapshotBackfillEnabled && mode === "live"}
            onSnapshotCapture={snapshotBackfillEnabled ? uploadFirstFrameSnapshot : undefined}
            onScreenshot={handleScreenshot}
          />
        </section>
        {message ? <div className="h5-error">{message}</div> : null}
        <div className="h5-player-status-panel">
          <div className="h5-player-status-head"><div><strong>{playerStatus.message}</strong><span>{mode === "live" ? "实时视频" : "录像"}</span></div></div>
        </div>
        {mode === "playback" ? (
          <NVRLabHourlyPlaybackPicker startAt={playbackStartAt} onStartAtChange={setPlaybackStartAt} onPlay={playPlayback} />
        ) : null}
      </main>
      <nav className="h5-bottom-tabs" aria-label="播放模式">
        <button type="button" className={mode === "live" ? "active" : ""} onClick={() => switchMode("live")}>
          实时视频
        </button>
        <button type="button" className={mode === "playback" ? "active" : ""} onClick={() => switchMode("playback")}>
          录像
        </button>
      </nav>
    </div>
  );
}

function downloadMonitorScreenshot(dataUrl: string) {
  const anchor = document.createElement("a");
  anchor.href = dataUrl;
  anchor.download = `monitor-snapshot-${Date.now()}.png`;
  anchor.rel = "noopener";
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
}

export function NVRLabHourlyPlaybackPicker({
  startAt,
  onStartAtChange,
  onPlay,
  now = new Date(),
}: {
  startAt: string;
  onStartAtChange: (value: string) => void;
  onPlay: (range: NVRLabPlaybackRange) => void;
  now?: Date;
}) {
  const selectedDate = startAt.slice(0, 10);
  const selectedHour = Number.parseInt(startAt.slice(11, 13), 10);

  function updateStart(nextStartAt: string) {
    onStartAtChange(nextStartAt);
  }

  function selectHour(hour: number) {
    const next = buildNVRLabHourlyPlayback(selectedDate, hour, now);
    if (!next) return;
    updateStart(next.startAt);
    onPlay(next);
  }

  return (
    <section className="nvr-lab-hourly-picker" aria-label="按小时定位回放">
      <PlaybackDatePicker value={startAt} onChange={updateStart} showTime={false} showConfirm={false} />
      <div className="nvr-lab-hour-grid" aria-label="固定小时段">
        {Array.from({ length: 24 }, (_, hour) => {
          const range = buildNVRLabHourlyPlayback(selectedDate, hour, now);
          return (
            <button key={hour} type="button" className={hour === selectedHour ? "is-selected" : undefined} disabled={!range} onClick={() => selectHour(hour)}>
              {hourLabel(hour)}
            </button>
          );
        })}
      </div>
    </section>
  );
}

function hourLabel(hour: number): string {
  const next = hour === 23 ? "次日 00:00" : `${pad2(hour + 1)}:00`;
  return `${pad2(hour)}:00 - ${next}`;
}

function pad2(value: number): string {
  return `${value}`.padStart(2, "0");
}

function playbackQueryValue(key: "start_at"): string {
  const value = new URLSearchParams(window.location.search).get(key) || "";
  return /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2})?$/.test(value) ? value : "";
}
