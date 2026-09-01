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
  type NVRLabCamera,
  type NVRLabMode,
  type NVRLabPlaybackRange,
  type NVRLabStreamSession,
} from "../domain/nvr-lab";
import type { AuthState } from "../domain/auth";

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
  const [playbackPlaying, setPlaybackPlaying] = useState(false);
  const [playbackStartedAtMs, setPlaybackStartedAtMs] = useState<number | null>(null);
  const [playbackElapsedSeconds, setPlaybackElapsedSeconds] = useState(0);
  const [snapshotBackfillState, setSnapshotBackfillState] = useState<"waiting" | "uploading" | "succeeded" | "failed">("waiting");
  const activePlaybackRangeRef = useRef<NVRLabPlaybackRange | null>(null);
  const playbackCursorRef = useRef<number | null>(null);
  const modeRef = useRef<NVRLabMode>("live");
  const playbackRange = buildNVRLabPlaybackFromStart(playbackStartAt);
  const playbackSegment: PlaybackSegmentTiming | null = activePlaybackRange
    ? { start_time: activePlaybackRange.startTime, end_time: activePlaybackRange.endTime }
    : null;
  const snapshotBackfillEnabled = new URLSearchParams(window.location.search).get("snapshot_backfill") === "1";

  useEffect(() => {
    activePlaybackRangeRef.current = activePlaybackRange;
  }, [activePlaybackRange]);

  useEffect(() => {
    playbackCursorRef.current = playbackCursorUnix;
  }, [playbackCursorUnix]);

  useEffect(() => {
    modeRef.current = mode;
  }, [mode]);

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
  }, [cameraId, externalOrgId]);

  const requestSession = useCallback(
    async (nextMode: NVRLabMode, nextStart?: number, nextEnd?: number) => {
      setSession(null);
      setMessage("");
      try {
        const nextSession = await nvrLabApi.createStreamSession(externalOrgId, cameraId, nextMode, nextStart, nextEnd);
        setSession(nextSession);
      } catch (error) {
        if (error instanceof NVRLabApiError && error.status === 401) {
          onAuthRequired(error);
          return;
        }
        if (error instanceof NVRLabApiError) {
          setMessage(`${error.message}（${error.code || `HTTP ${error.status}`}）`);
          return;
        }
        setMessage(error instanceof Error ? error.message : "播放会话创建失败");
      }
    },
	[externalOrgId, cameraId, onAuthRequired],
  );

  useEffect(() => {
    void requestSession("live");
  }, [requestSession]);

  function switchMode(nextMode: NVRLabMode) {
    setMode(nextMode);
    setSession(null);
    setMessage("");
    if (nextMode === "live") resetPlaybackProgress();
    if (nextMode === "live") void requestSession("live");
  }

  function playPlayback(playbackWindow: NVRLabPlaybackRange, startTime = playbackWindow.startTime) {
    const nextSession = buildNVRLabPlaybackSession(playbackWindow, startTime);
    if (!nextSession) return;
    setMessage("");
    setPlayerStatus({ stage: "connecting", message: "正在连接视频流" });
    activePlaybackRangeRef.current = nextSession.playbackWindow;
    playbackCursorRef.current = nextSession.startTime;
    setActivePlaybackRange(nextSession.playbackWindow);
    setPlaybackCursorUnix(nextSession.startTime);
    setPlaybackPlaying(false);
    setPlaybackStartedAtMs(null);
    setPlaybackElapsedSeconds(0);
    void requestSession("playback", nextSession.startTime, nextSession.endTime);
  }

  function retrySession() {
    if (mode === "live") {
      void requestSession("live");
      return;
    }
    const retryRange = activePlaybackRange || playbackRange;
    if (retryRange) playPlayback(retryRange, playbackCursorRef.current ?? retryRange.startTime);
  }

  const handlePlaybackStateChange = useCallback((playing: boolean) => {
    setPlaybackPlaying(playing);
    const range = activePlaybackRangeRef.current;
    if (modeRef.current !== "playback" || !range) return;
    const cursor = playbackCursorRef.current ?? range.startTime;
    const elapsed = Math.min(Math.max(0, cursor - range.startTime), Math.max(0, range.endTime - range.startTime - 1));
    setPlaybackElapsedSeconds(elapsed);
    setPlaybackStartedAtMs(playing ? Date.now() : null);
  }, []);

  const uploadFirstFrameSnapshot = useCallback(async (image: Blob | null) => {
    if (!image || image.type !== "image/jpeg" || image.size === 0) {
      setSnapshotBackfillState("failed");
      return;
    }
    setSnapshotBackfillState("uploading");
    try {
      await nvrLabApi.uploadSnapshot(externalOrgId, cameraId, image);
      setSnapshotBackfillState("succeeded");
    } catch {
      setSnapshotBackfillState("failed");
    }
  }, [cameraId, externalOrgId]);

  const seekPlayback = useCallback((startTime: number) => {
    const range = activePlaybackRangeRef.current;
    if (!range || startTime < range.startTime || startTime >= range.endTime) return;
    playPlayback(range, startTime);
  }, []);

  useEffect(() => {
    if (!activePlaybackRange || !playbackPlaying || playbackStartedAtMs === null) return;
    const tick = () => {
      const elapsed = playbackElapsedSeconds + Math.max(0, Math.floor((Date.now() - playbackStartedAtMs) / 1000));
      const cursor = Math.min(activePlaybackRange.endTime - 1, activePlaybackRange.startTime + elapsed);
      setPlaybackCursorUnix(cursor);
    };
    tick();
    const timer = window.setInterval(tick, 1000);
    return () => window.clearInterval(timer);
  }, [activePlaybackRange, playbackElapsedSeconds, playbackPlaying, playbackStartedAtMs]);

  function resetPlaybackProgress() {
    setActivePlaybackRange(null);
    setPlaybackCursorUnix(null);
    setPlaybackPlaying(false);
    setPlaybackStartedAtMs(null);
    setPlaybackElapsedSeconds(0);
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
            onStatus={setPlayerStatus}
            onPlaybackStateChange={handlePlaybackStateChange}
            onSeekPlayback={seekPlayback}
            captureOnFirstFrame={snapshotBackfillEnabled && mode === "live"}
            onSnapshotCapture={snapshotBackfillEnabled ? uploadFirstFrameSnapshot : undefined}
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
