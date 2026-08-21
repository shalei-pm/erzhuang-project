import { useCallback, useEffect, useState } from "react";
import { nvrLabApi, NVRLabApiError } from "../api-nvr-lab";
import { NVRLabPlayer, type NVRLabPlayerStatus } from "../components/NVRLabPlayer";
import { SystemTopBar } from "../components/SystemTopBar";
import { nvrLabCameraSubtitle, nvrLabCameraTitle, validateNVRLabPlayback, type NVRLabCamera, type NVRLabMode, type NVRLabStreamSession } from "../domain/nvr-lab";
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
  const [startAt, setStartAt] = useState("");
  const [endAt, setEndAt] = useState("");

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

  function playPlayback() {
    const startTime = localDateTimeToUnix(startAt);
    const endTime = localDateTimeToUnix(endAt);
    const validation = validateNVRLabPlayback(startTime, endTime);
    if (validation) {
      setMessage(validation);
      return;
    }
    void requestSession("playback", startTime, endTime);
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
          <NVRLabPlayer session={session} onRetry={() => void requestSession(mode, localDateTimeToUnix(startAt), localDateTimeToUnix(endAt))} onStatus={setPlayerStatus} />
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
          <div className="nvr-lab-playback-form">
            <label>开始时间<input type="datetime-local" step="1" value={startAt} onChange={(event) => setStartAt(event.target.value)} /></label>
            <label>结束时间<input type="datetime-local" step="1" value={endAt} onChange={(event) => setEndAt(event.target.value)} /></label>
            <button type="button" onClick={playPlayback}>播放</button>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function localDateTimeToUnix(value: string): number {
  if (!value) return 0;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? 0 : Math.floor(date.getTime() / 1000);
}
