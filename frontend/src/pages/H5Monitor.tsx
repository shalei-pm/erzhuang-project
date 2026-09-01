import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { h5Api, H5ApiError } from "../api-h5";
import { H5StoreSwitcher } from "../components/H5StoreSwitcher";
import { SystemTopBar } from "../components/SystemTopBar";
import { cameraPlaceholderURL, legacyCameraThumbnailKind } from "../domain/camera-placeholder";
import { h5ChannelDisplayText, h5InitialVisibleCount, h5NextVisibleCount } from "../domain/h5-channel-display";
import { h5MonitorTabs, readH5MonitorActiveTab, storeH5MonitorActiveTab, type H5MonitorTabKey } from "../domain/h5-monitor-active-tab";
import { writeSessionStorage, type AuthState } from "../domain/auth";
import type { H5MonitorChannel, H5MonitorHomeResponse } from "../domain/h5-types";

interface H5MonitorProps {
  externalOrgId: string;
  auth?: AuthState | null;
  loggingOut?: boolean;
  authMessage?: string;
  activeTabFromRoute?: H5MonitorTabKey | null;
  onOpenChannel: (channelId: number, activeTab: H5MonitorTabKey) => void;
  onSelectTab?: (tab: H5MonitorTabKey) => void;
  onSelectStore: (externalOrgId: string) => void;
  onAuthRequired?: (error?: unknown) => void;
  onLogout?: () => void | Promise<void>;
  refreshKey?: number;
}

export function H5Monitor({
  externalOrgId,
  auth,
  loggingOut = false,
  authMessage = "",
  activeTabFromRoute = null,
  onOpenChannel,
  onSelectTab,
  onSelectStore,
  onAuthRequired,
  onLogout,
  refreshKey = 0,
}: H5MonitorProps) {
  const [data, setData] = useState<H5MonitorHomeResponse | null>(null);
  const [activeTab, setActiveTab] = useState<H5MonitorTabKey>(() => activeTabFromRoute ?? readH5MonitorActiveTab(externalOrgId));
  const [visibleCount, setVisibleCount] = useState(() => h5InitialVisibleCount(0, 0));
  const [loading, setLoading] = useState(true);
  const [toast, setToast] = useState("");
  const cameraWallRef = useRef<HTMLElement | null>(null);

  const measureInitialVisibleCount = useCallback(() => {
    const containerWidth = cameraWallRef.current?.clientWidth ?? 0;
    const viewportWidth = typeof window === "undefined" ? 0 : window.innerWidth;
    return h5InitialVisibleCount(containerWidth, viewportWidth);
  }, []);

  useEffect(() => {
    setActiveTab(activeTabFromRoute ?? readH5MonitorActiveTab(externalOrgId));
  }, [activeTabFromRoute, externalOrgId]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    h5Api
      .getMonitorHome(externalOrgId)
      .then((resp) => {
        if (cancelled) return;
        setData(resp);
        setToast("");
      })
      .catch((err) => {
        if (cancelled) return;
        if (err instanceof H5ApiError && err.status === 401) {
          onAuthRequired?.(err);
          return;
        }
        if (err instanceof H5ApiError && err.status === 403) {
          setToast("暂无该门店监控访问权限");
          return;
        }
        setToast(errorMessage(err, "监控数据加载失败"));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [externalOrgId, refreshKey, onAuthRequired]);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      setVisibleCount(measureInitialVisibleCount());
    });
    return () => window.cancelAnimationFrame(frame);
  }, [activeTab, data?.external_org_id, measureInitialVisibleCount]);

  useEffect(() => {
    const updateVisibleRows = () => {
      setVisibleCount((count) => Math.max(measureInitialVisibleCount(), count));
    };
    updateVisibleRows();
    window.addEventListener("resize", updateVisibleRows);
    return () => window.removeEventListener("resize", updateVisibleRows);
  }, [measureInitialVisibleCount]);

  const allChannels = useMemo(() => {
    if (!data) return [];
    return data.groups.flatMap((group) => group.channels);
  }, [data]);

  const visibleTabs = useMemo(() => {
    const categoryCount = new Map<H5MonitorTabKey, number>();
    categoryCount.set("all", allChannels.length);
    for (const channel of allChannels) {
      categoryCount.set(channel.category, (categoryCount.get(channel.category) ?? 0) + 1);
    }
    return h5MonitorTabs.filter((tab) => (categoryCount.get(tab.key) ?? 0) > 0 || tab.key === "all");
  }, [allChannels]);

  useEffect(() => {
    if (!data) return;
    if (!visibleTabs.some((tab) => tab.key === activeTab)) {
      storeH5MonitorActiveTab(externalOrgId, "all");
      onSelectTab?.("all");
      setActiveTab("all");
    }
  }, [activeTab, data, externalOrgId, onSelectTab, visibleTabs]);

  const selectActiveTab = useCallback(
    (tab: H5MonitorTabKey) => {
      storeH5MonitorActiveTab(externalOrgId, tab);
      setActiveTab(tab);
      onSelectTab?.(tab);
    },
    [externalOrgId, onSelectTab],
  );

  const filteredChannels = useMemo(() => {
    if (activeTab === "all") return allChannels;
    return allChannels.filter((channel) => channel.category === activeTab);
  }, [activeTab, allChannels]);

  const shownChannels = filteredChannels.slice(0, visibleCount);
  const hasMore = shownChannels.length < filteredChannels.length;

  if (loading) {
    return (
      <div className="h5-page h5-monitor-page">
        <SystemTopBar auth={auth} loggingOut={loggingOut} onLogout={onLogout} />
        {authMessage ? <div className="h5-auth-message">{authMessage}</div> : null}
        <div className="h5-loading">加载中...</div>
      </div>
    );
  }

  if (toast) {
    return (
      <div className="h5-page h5-monitor-page">
        <SystemTopBar auth={auth} loggingOut={loggingOut} onLogout={onLogout} />
        {authMessage ? <div className="h5-auth-message">{authMessage}</div> : null}
        <div className="h5-error">{toast}</div>
      </div>
    );
  }

  if (!data || allChannels.length === 0) {
    return (
      <div className="h5-page h5-monitor-page">
        <SystemTopBar auth={auth} loggingOut={loggingOut} onLogout={onLogout} />
        {authMessage ? <div className="h5-auth-message">{authMessage}</div> : null}
        <header className="h5-header h5-monitor-header">
          <H5StoreSwitcher
            currentExternalOrgId={externalOrgId}
            currentStoreName={data?.store_name || "门店监控"}
            currentCity={data?.city}
            onAuthRequired={onAuthRequired}
            onSelectStore={onSelectStore}
          />
        </header>
        <div className="h5-empty">当前门店暂无有效监控通道。</div>
      </div>
    );
  }

  return (
    <div className="h5-page h5-monitor-page">
      <SystemTopBar auth={auth} loggingOut={loggingOut} onLogout={onLogout} />
      {authMessage ? <div className="h5-auth-message">{authMessage}</div> : null}
      <header className="h5-header h5-monitor-header">
        <H5StoreSwitcher
          currentExternalOrgId={externalOrgId}
          currentStoreName={data.store_name}
          currentCity={data.city}
          onAuthRequired={onAuthRequired}
          onSelectStore={onSelectStore}
        />
        <span className="h5-channel-count">共 {filteredChannels.length} 路，{filteredChannels.length} 路有效</span>
      </header>

      <nav className="h5-area-tabs" aria-label="区域导航">
        {visibleTabs.map((tab) => (
          <button
            key={tab.key}
            className={activeTab === tab.key ? "active" : ""}
            onClick={() => selectActiveTab(tab.key)}
          >
            {tab.label}
          </button>
        ))}
      </nav>

      <main className="h5-camera-wall" ref={cameraWallRef}>
        {shownChannels.map((channel) => (
          <CameraBubble
            key={channel.id}
            channel={channel}
            onClick={() => {
              writeSessionStorage("h5-monitor-active-channel-name", h5ChannelDisplayText(channel).title);
              onOpenChannel(channel.id, activeTab);
            }}
          />
        ))}
      </main>

      {hasMore && (
        <div className="h5-load-more-row">
          <button
            className="h5-load-more"
            onClick={() => {
              const containerWidth = cameraWallRef.current?.clientWidth ?? 0;
              const viewportWidth = typeof window === "undefined" ? 0 : window.innerWidth;
              setVisibleCount((count) =>
                h5NextVisibleCount({
                  containerWidth,
                  viewportWidth,
                  visibleCount: count,
                  totalCount: filteredChannels.length,
                }),
              );
            }}
          >
            加载更多
          </button>
        </div>
      )}
    </div>
  );
}

function CameraBubble({ channel, onClick }: { channel: H5MonitorChannel; onClick: () => void }) {
  const displayText = h5ChannelDisplayText(channel);
  const [thumbnailFailed, setThumbnailFailed] = useState(false);
  const verifiedThumbnailURL = channel.thumbnail_url
    ? displayImageURL(channel.thumbnail_url)
    : "";
  const thumbnailURL = !thumbnailFailed && verifiedThumbnailURL
    ? verifiedThumbnailURL
    : cameraPlaceholderURL(legacyCameraThumbnailKind({ areaType: channel.area_type, category: channel.category, sceneType: channel.scene_type }));

  return (
    <button className="h5-camera-bubble" onClick={onClick} aria-label={`查看${displayText.title}`}>
      <span className="h5-camera-frame">
        <img src={thumbnailURL} alt={!thumbnailFailed && verifiedThumbnailURL ? displayText.title : "摄像头默认缩略图"} loading="lazy" onError={() => setThumbnailFailed(true)} />
      </span>
      <span className="h5-camera-title">{displayText.title}</span>
      <span className="h5-camera-subtitle">{displayText.subtitle}</span>
    </button>
  );
}

function displayImageURL(value: string): string {
  if (!value.startsWith("/api/")) return value;
  const base = (import.meta.env.BASE_URL || "/erzhuang-project/").replace(/\/$/, "");
  return `${base}${value}`;
}

function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof H5ApiError) {
    if (err.status === 403) return "暂无该门店监控访问权限";
    const fieldMsgs = Object.values(err.fields).filter(Boolean);
    if (fieldMsgs.length > 0) return fieldMsgs.reduce((message, fieldMsg) => `${message}；${fieldMsg}`);
    return err.message;
  }
  if (err instanceof Error && err.message.trim()) return err.message;
  return fallback;
}
