import { useEffect, useState } from "react";
import type { AISettings, EzvizAccount, StoreDetail as StoreDetailType, VideoRecorder } from "../api";
import { DesignPlanTab } from "./DesignPlanTab";
import { VideoChannelTab } from "./VideoChannelTab";

export type StoreDetailTab = "design-plan" | "channels";

type StoreDetailProps = {
  store: StoreDetailType;
  initialTab: StoreDetailTab;
  loadingTabs: Set<StoreDetailTab>;
  loadedTabs: Set<StoreDetailTab>;
  saving: boolean;
  accounts: EzvizAccount[];
  aiSettings: AISettings | null;
  switchingAIModel: boolean;
  h5MonitorUrl?: string;
  onBack: () => void;
  onTabChange: (tab: StoreDetailTab) => void;
  onToggleAIModel: () => void;
  onStoreUpdated: (update: StoreDetailType | ((store: StoreDetailType) => StoreDetailType)) => void;
  onToast: (message: string) => void;
};

export function StoreDetail({
  store,
  initialTab,
  loadingTabs,
  loadedTabs,
  saving,
  accounts,
  aiSettings,
  switchingAIModel,
  h5MonitorUrl,
  onBack,
  onTabChange,
  onToggleAIModel,
  onStoreUpdated,
  onToast,
}: StoreDetailProps) {
  const [activeTab, setActiveTab] = useState<StoreDetailTab>(initialTab);

  useEffect(() => {
    setActiveTab(initialTab);
  }, [initialTab, store.id]);

  function updateRecorder(nextRecorder: VideoRecorder) {
    onStoreUpdated((currentStore) => ({
      ...currentStore,
      recorders: currentStore.recorders.map((recorder) => (recorder.id === nextRecorder.id ? nextRecorder : recorder)),
      recorderCount: currentStore.recorders.length,
      channelCount: currentStore.recorders.reduce(
        (total, recorder) =>
          total + (recorder.id === nextRecorder.id ? nextRecorder.channels.length : recorder.channels.length),
        0,
      ),
    }));
  }

  function selectTab(tab: StoreDetailTab) {
    setActiveTab(tab);
    onTabChange(tab);
  }

  const isActiveTabLoading = loadingTabs.has(activeTab) || !loadedTabs.has(activeTab);

  return (
    <section className="detail-page">
      <header className="detail-header">
        <div className="detail-header-main">
          <button className="detail-back-button" onClick={onBack} aria-label="返回机构列表">
            <span aria-hidden="true">←</span>
            <span>返回列表</span>
          </button>
          <h1>{store.name}</h1>
          <div className="detail-metrics" aria-label="门店资源概览">
            <div>
              <span>新氧机构 ID</span>
              <strong>{store.externalOrgId || "-"}</strong>
            </div>
            <div>
              <span>录像机</span>
              <strong>{store.recorderCount}</strong>
            </div>
            <div>
              <span>有效通道</span>
              <strong>{store.channelCount}</strong>
            </div>
            <div>
              <span>业务区域</span>
              <strong>{store.areaCount}</strong>
            </div>
          </div>
        </div>
        {h5MonitorUrl ? (
          <div className="detail-header-side">
            <button className="detail-back-button" onClick={() => window.location.assign(h5MonitorUrl)}>
              查看监控
            </button>
          </div>
        ) : null}
      </header>

      <div className="detail-tabs-row">
        <nav className="tabs" aria-label="门店详情 Tab">
          <button className={activeTab === "design-plan" ? "is-active" : ""} onClick={() => selectTab("design-plan")}>
            设计图标注
          </button>
          <button className={activeTab === "channels" ? "is-active" : ""} onClick={() => selectTab("channels")}>
            通道映射
          </button>
        </nav>
        <div className="ai-model-switcher" aria-label="识别模型设置">
          <span>当前识别模型：{aiSettings?.label ?? "加载中"}</span>
          <button type="button" className="secondary-action-button" disabled={switchingAIModel || !aiSettings} onClick={onToggleAIModel}>
            {switchingAIModel ? "切换中" : "切换识别模型"}
          </button>
        </div>
      </div>

      {isActiveTabLoading ? (
        <div className="detail-loading-panel" role="status" aria-live="polite">
          <span aria-hidden="true" />
          <strong>{activeTab === "channels" ? "正在加载通道映射" : "正在加载设计图标注"}</strong>
          <p>已进入详情页，当前 Tab 数据加载完成后会自动展示。</p>
        </div>
      ) : (
        <>
          <div hidden={activeTab !== "design-plan"}>
            <DesignPlanTab store={store} saving={saving} onStoreUpdated={onStoreUpdated} onToast={onToast} />
          </div>
          <div hidden={activeTab !== "channels"}>
            <VideoChannelTab
              store={store}
              accounts={accounts}
              onStoreUpdated={onStoreUpdated}
              onRecorderUpdated={updateRecorder}
              onToast={onToast}
            />
          </div>
        </>
      )}
    </section>
  );
}
