import { useCallback, useEffect, useState } from "react";
import { nvrLabApi, NVRLabApiError } from "../api-nvr-lab";
import { NVRLabPlayer, type NVRLabPlayerStatus } from "../components/NVRLabPlayer";
import { PlaybackDatePicker, initialPlaybackDateTimeValue } from "../components/PlaybackDatePicker";
import { SystemTopBar } from "../components/SystemTopBar";
import {
  NVR_LAB_MAX_PLAYBACK_SECONDS,
  buildNVRLabHourlyPlayback,
  buildNVRLabPlaybackFromStart,
  nvrLabCameraSubtitle,
  nvrLabCameraTitle,
  type NVRLabCamera,
  type NVRLabMode,
  type NVRLabPlaybackRange,
  type NVRLabStreamSession,
} from "../domain/nvr-lab";
import type { AuthState } from "../domain/auth";

type NVRLabCameraProps = {
  cameraId: number;
  auth: AuthState | null;
  loggingOut: boolean;
  authMessage: string;
  onLogout: () => void | Promise<void>;
  onAuthRequired: () => void;
  onBack: () => void;
};

export function NVRLabCamera({ cameraId, auth, loggingOut, authMessage, onLogout, onAuthRequired, onBack }: NVRLabCameraProps) {
  const [camera, setCamera] = useState<NVRLabCamera | null>(null);
  const [mode, setMode] = useState<NVRLabMode>("live");
  const [session, setSession] = useState<NVRLabStreamSession | null>(null);
  const [message, setMessage] = useState("");
  const [playerStatus, setPlayerStatus] = useState<NVRLabPlayerStatus>({ stage: "idle", message: "播放器准备中" });
  const [playbackStartAt, setPlaybackStartAt] = useState(() => playbackQueryValue("start_at") || initialPlaybackDateTimeValue(new Date(Date.now() - NVR_LAB_MAX_PLAYBACK_SECONDS * 1000)));
  const playbackRange = buildNVRLabPlaybackFromStart(playbackStartAt);

  useEffect(() => {
    let cancelled = false;
    nvrLabApi
      .listCameras()
      .then((response) => {
        if (cancelled) return;
        const found = response.cameras.find((item) => item.id === cameraId) || null;
        setCamera(found);
        if (!found) setMessage("未找到可用摄像头");
      })
      .catch((error) => {
        if (!cancelled) setMessage(error instanceof Error ? error.message : "摄像头信息加载失败");
      });
    return () => {
      cancelled = true;
    };
  }, [cameraId]);

  const requestSession = useCallback(
    async (nextMode: NVRLabMode, nextStart?: number, nextEnd?: number) => {
      setSession(null);
      setMessage("");
      try {
        const nextSession = await nvrLabApi.createStreamSession(cameraId, nextMode, nextStart, nextEnd);
        setSession(nextSession);
      } catch (error) {
        if (error instanceof NVRLabApiError && error.status === 401) {
          onAuthRequired();
          return;
        }
        if (error instanceof NVRLabApiError) {
          setMessage(`${error.message}（${error.code || `HTTP ${error.status}`}）`);
          return;
        }
        setMessage(error instanceof Error ? error.message : "播放会话创建失败");
      }
    },
    [cameraId, onAuthRequired],
  );

  useEffect(() => {
    void requestSession("live");
  }, [requestSession]);

  function switchMode(nextMode: NVRLabMode) {
    setMode(nextMode);
    setSession(null);
    setMessage("");
    if (nextMode === "live") void requestSession("live");
  }

  function playPlayback(range: NVRLabPlaybackRange) {
    setMessage("");
    void requestSession("playback", range.startTime, range.endTime);
  }

  function retrySession() {
    if (mode === "live") {
      void requestSession("live");
      return;
    }
    if (playbackRange) playPlayback(playbackRange);
  }

  return (
    <div className="h5-page h5-channel-page nvr-lab-page">
      <SystemTopBar backAction={{ label: "返回监控列表", onClick: onBack }} auth={auth} loggingOut={loggingOut} onLogout={onLogout} />
      {authMessage ? <div className="h5-auth-message">{authMessage}</div> : null}
      <header className="h5-viewer-header">
        <div className="h5-viewer-title">
          <h1>{camera ? nvrLabCameraTitle(camera) : "摄像头监控"}</h1>
          <span>{camera ? nvrLabCameraSubtitle(camera) : "工控机取流实验"}</span>
        </div>
      </header>
      <div className="h5-viewer">
        <div className="h5-viewer-player">
          <NVRLabPlayer session={session} onRetry={retrySession} onStatus={setPlayerStatus} />
        </div>
        {message ? <div className="h5-error">{message}</div> : null}
        <div className="h5-player-status-panel">
          <div className="h5-player-status-head"><div><strong>{playerStatus.message}</strong><span>{mode === "live" ? "实时视频" : "录像"}</span></div></div>
        </div>
        <nav className="nvr-lab-mode-tabs" aria-label="视频模式">
          <button type="button" className={mode === "live" ? "active" : ""} onClick={() => switchMode("live")}>实时视频</button>
          <button type="button" className={mode === "playback" ? "active" : ""} onClick={() => switchMode("playback")}>录像</button>
        </nav>
        {mode === "playback" ? (
          <NVRLabHourlyPlaybackPicker startAt={playbackStartAt} range={playbackRange} onStartAtChange={setPlaybackStartAt} onConfirm={playPlayback} />
        ) : null}
      </div>
    </div>
  );
}

export function NVRLabHourlyPlaybackPicker({
  startAt,
  range,
  onStartAtChange,
  onConfirm,
}: {
  startAt: string;
  range: NVRLabPlaybackRange | null;
  onStartAtChange: (value: string) => void;
  onConfirm: (range: NVRLabPlaybackRange) => void;
}) {
  const [selectionMessage, setSelectionMessage] = useState("");
  const selectedDate = startAt.slice(0, 10);
  const selectedHour = Number.parseInt(startAt.slice(11, 13), 10);

  function updateStart(nextStartAt: string) {
    setSelectionMessage("");
    onStartAtChange(nextStartAt);
  }

  function selectHour(hour: number) {
    const next = buildNVRLabHourlyPlayback(selectedDate, hour);
    if (!next) {
      setSelectionMessage("请选择当前时刻之前的回放时间");
      return;
    }
    updateStart(next.startAt);
  }

  function confirm(nextStartAt: string) {
    const next = buildNVRLabPlaybackFromStart(nextStartAt);
    if (!next) {
      setSelectionMessage("请选择当前时刻之前的回放时间");
      return;
    }
    setSelectionMessage("");
    onConfirm(next);
  }

  return (
    <section className="nvr-lab-hourly-picker" aria-label="按小时定位回放">
      <PlaybackDatePicker value={startAt} onChange={updateStart} onConfirm={confirm} />
      <p className="nvr-lab-hourly-range">回放范围：{range ? formatRange(range) : "请选择当前时刻之前的回放时间"}</p>
      {selectionMessage ? <p className="nvr-lab-hourly-error">{selectionMessage}</p> : null}
      <div className="nvr-lab-hour-grid" aria-label="固定小时段">
        {Array.from({ length: 24 }, (_, hour) => (
          <button key={hour} type="button" className={hour === selectedHour ? "is-selected" : undefined} onClick={() => selectHour(hour)}>
            {hourLabel(hour)}
          </button>
        ))}
      </div>
    </section>
  );
}

function formatRange(range: NVRLabPlaybackRange): string {
  return `${formatDateTime(range.startAt)} - ${formatDateTime(range.endAt)}`;
}

function formatDateTime(value: string): string {
  return value.replace("T", " ").replaceAll("-", "/");
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
