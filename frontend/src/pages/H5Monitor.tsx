import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { h5Api, H5ApiError } from "../api-h5";
import { h5ChannelDisplayText, h5InitialVisibleCount, h5NextVisibleCount } from "../domain/h5-channel-display";
import type { H5MonitorChannel, H5MonitorHomeResponse, MonitorCategory } from "../domain/h5-types";

interface H5MonitorProps {
  externalOrgId: string;
  onOpenChannel: (channelId: number) => void;
  refreshKey?: number;
}

type TabKey = "all" | MonitorCategory;

const TABS: Array<{ key: TabKey; label: string }> = [
  { key: "all", label: "全部" },
  { key: "consultation", label: "面诊室" },
  { key: "treatment", label: "治疗室" },
  { key: "beauty", label: "生美" },
  { key: "front_waiting", label: "前台/等候区" },
  { key: "other", label: "过道/其他" },
];

export function H5Monitor({ externalOrgId, onOpenChannel, refreshKey = 0 }: H5MonitorProps) {
  const [data, setData] = useState<H5MonitorHomeResponse | null>(null);
  const [activeTab, setActiveTab] = useState<TabKey>("all");
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
        setToast(errorMessage(err, "监控数据加载失败"));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [externalOrgId, refreshKey]);

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
    const categoryCount = new Map<TabKey, number>();
    categoryCount.set("all", allChannels.length);
    for (const channel of allChannels) {
      categoryCount.set(channel.category, (categoryCount.get(channel.category) ?? 0) + 1);
    }
    return TABS.filter((tab) => (categoryCount.get(tab.key) ?? 0) > 0 || tab.key === "all");
  }, [allChannels]);

  const filteredChannels = useMemo(() => {
    if (activeTab === "all") return allChannels;
    return allChannels.filter((channel) => channel.category === activeTab);
  }, [activeTab, allChannels]);

  const shownChannels = filteredChannels.slice(0, visibleCount);
  const hasMore = shownChannels.length < filteredChannels.length;

  if (loading) {
    return (
      <div className="h5-page h5-monitor-page">
        <div className="h5-loading">加载中...</div>
      </div>
    );
  }

  if (toast) {
    return (
      <div className="h5-page h5-monitor-page">
        <div className="h5-error">{toast}</div>
      </div>
    );
  }

  if (!data || allChannels.length === 0) {
    return (
      <div className="h5-page h5-monitor-page">
        <header className="h5-header">
          <h1>{data?.store_name || "门店监控"}</h1>
          {data?.city && <span className="h5-city">{data.city}</span>}
        </header>
        <div className="h5-empty">当前门店暂无有效监控通道。</div>
      </div>
    );
  }

  return (
    <div className="h5-page h5-monitor-page">
      <header className="h5-header h5-monitor-header">
        <div>
          <h1>{data.store_name}</h1>
          {data.city && <span className="h5-city">{data.city}</span>}
        </div>
        <span className="h5-channel-count">共 {filteredChannels.length} 路，{filteredChannels.length} 路有效</span>
      </header>

      <nav className="h5-area-tabs" aria-label="区域导航">
        {visibleTabs.map((tab) => (
          <button
            key={tab.key}
            className={activeTab === tab.key ? "active" : ""}
            onClick={() => setActiveTab(tab.key)}
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
              sessionStorage.setItem("h5-monitor-active-channel-name", h5ChannelDisplayText(channel).title);
              onOpenChannel(channel.id);
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

  return (
    <button className="h5-camera-bubble" onClick={onClick} aria-label={`查看${displayText.title}`}>
      <span className="h5-camera-frame">
        {channel.thumbnail_url ? (
          <img src={displayImageURL(channel.thumbnail_url)} alt={displayText.title} loading="lazy" />
        ) : (
          <span className="h5-camera-placeholder">暂无画面</span>
        )}
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
    const fieldMsgs = Object.values(err.fields).filter(Boolean);
    if (fieldMsgs.length > 0) return fieldMsgs.join("；");
    return err.message;
  }
  if (err instanceof Error && err.message.trim()) return err.message;
  return fallback;
}
