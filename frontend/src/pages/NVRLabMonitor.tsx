import { useEffect, useMemo, useState } from "react";
import { nvrLabApi, NVRLabApiError } from "../api-nvr-lab";
import { SystemTopBar } from "../components/SystemTopBar";
import { NVRMonitorStoreSwitcher } from "../components/NVRMonitorStoreSwitcher";
import {
  nvrLabCameraSubtitle,
  nvrLabCameraTab,
  nvrLabCameraTitle,
  nvrLabTabs,
  type NVRLabCamera,
  type NVRLabTabKey,
} from "../domain/nvr-lab";
import type { AuthState } from "../domain/auth";

type NVRLabMonitorProps = {
  externalOrgId: string;
  auth: AuthState | null;
  loggingOut: boolean;
  authMessage: string;
  onLogout: () => void | Promise<void>;
  onAuthRequired: () => void;
  onOpenCamera: (cameraId: number) => void;
  onSelectStore: (externalOrgId: string) => void;
};

export function NVRLabMonitor({ externalOrgId, auth, loggingOut, authMessage, onLogout, onAuthRequired, onOpenCamera, onSelectStore }: NVRLabMonitorProps) {
	const [storeName, setStoreName] = useState("门店监控");
	const [city, setCity] = useState("");
  const [cameras, setCameras] = useState<NVRLabCamera[]>([]);
  const [activeTab, setActiveTab] = useState<NVRLabTabKey>("all");
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");

  useEffect(() => {
    let cancelled = false;
    nvrLabApi
      .listCameras(externalOrgId)
      .then((response) => {
        if (cancelled) return;
		setStoreName(response.store_name || "门店监控");
		setCity(response.city || "");
        setCameras(response.cameras || []);
        setMessage("");
      })
      .catch((error) => {
        if (cancelled) return;
        if (error instanceof NVRLabApiError && error.status === 401) {
          onAuthRequired();
          return;
        }
        setMessage(error instanceof Error ? error.message : "摄像头列表加载失败");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [externalOrgId, onAuthRequired]);

  const visibleTabs = useMemo(() => {
    const count = new Map<NVRLabTabKey, number>([["all", cameras.length]]);
    cameras.forEach((camera) => count.set(nvrLabCameraTab(camera), (count.get(nvrLabCameraTab(camera)) || 0) + 1));
    return nvrLabTabs.filter((tab) => tab.key === "all" || (count.get(tab.key) || 0) > 0);
  }, [cameras]);

  const visibleCameras = activeTab === "all" ? cameras : cameras.filter((camera) => nvrLabCameraTab(camera) === activeTab);

  return (
    <div className="h5-page h5-monitor-page nvr-lab-page">
      <SystemTopBar auth={auth} loggingOut={loggingOut} onLogout={onLogout} />
      {authMessage ? <div className="h5-auth-message">{authMessage}</div> : null}
      <header className="h5-header h5-monitor-header">
        <NVRMonitorStoreSwitcher currentExternalOrgId={externalOrgId} currentStoreName={storeName} currentCity={city} onAuthRequired={onAuthRequired} onSelectStore={onSelectStore} />
        <span className="h5-channel-count">共 {visibleCameras.length} 路</span>
      </header>
      <nav className="h5-area-tabs" aria-label="区域筛选">
        {visibleTabs.map((tab) => (
          <button key={tab.key} type="button" className={activeTab === tab.key ? "active" : ""} onClick={() => setActiveTab(tab.key)}>
            {tab.label}
          </button>
        ))}
      </nav>
      {loading ? <div className="h5-loading">加载中...</div> : null}
      {!loading && message ? <div className="h5-error">{message}</div> : null}
      {!loading && !message && visibleCameras.length === 0 ? <div className="h5-empty">当前门店暂无可用摄像头。</div> : null}
      {!loading && !message ? (
        <main className="h5-camera-wall">
          {visibleCameras.map((camera) => (
            <button key={camera.id} type="button" className="h5-camera-bubble" data-camera-id={camera.id} onClick={() => onOpenCamera(camera.id)} aria-label={`查看${nvrLabCameraTitle(camera)}`}>
              <NVRMonitorThumbnail thumbnailURL={camera.thumbnail_url} />
              <span className="h5-camera-title">{nvrLabCameraTitle(camera)}</span>
              <span className="h5-camera-subtitle">{nvrLabCameraSubtitle(camera)}</span>
            </button>
          ))}
        </main>
      ) : null}
    </div>
  );
}

export function NVRMonitorThumbnail({ thumbnailURL }: { thumbnailURL?: string }) {
  const [failed, setFailed] = useState(false);
  const src = thumbnailURL?.trim();

  return (
    <span className="h5-camera-frame">
      {src && !failed ? (
        <img src={src} alt="摄像头最近截图" loading="lazy" onError={() => setFailed(true)} />
      ) : (
        <span className="h5-camera-placeholder" role="img" aria-label="暂无截图" />
      )}
    </span>
  );
}
