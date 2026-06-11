import { useState } from "react";
import type { EzvizAccount, StoreDetail as StoreDetailType, VideoRecorder } from "../api";
import { DesignPlanTab } from "./DesignPlanTab";
import { VideoChannelTab } from "./VideoChannelTab";

export type StoreDetailTab = "design-plan" | "channels";

type StoreDetailProps = {
  store: StoreDetailType;
  accounts: EzvizAccount[];
  initialTab: StoreDetailTab;
  saving: boolean;
  onBack: () => void;
  onStoreUpdated: (store: StoreDetailType) => void;
  onToast: (message: string) => void;
};

export function StoreDetail({ store, accounts, initialTab, saving, onBack, onStoreUpdated, onToast }: StoreDetailProps) {
  const [activeTab, setActiveTab] = useState<StoreDetailTab>(initialTab);

  function updateRecorder(nextRecorder: VideoRecorder) {
    onStoreUpdated({
      ...store,
      recorders: store.recorders.map((recorder) => (recorder.id === nextRecorder.id ? nextRecorder : recorder)),
      recorderCount: store.recorders.length,
      channelCount: store.recorders.reduce(
        (total, recorder) =>
          total + (recorder.id === nextRecorder.id ? nextRecorder.channels.length : recorder.channels.length),
        0,
      ),
    });
  }

  return (
    <section className="detail-page">
      <header className="detail-header">
        <div>
          <button className="plain-button" onClick={onBack}>
            返回列表
          </button>
          <p className="eyebrow">门店详情</p>
          <h1>{store.name}</h1>
          <div className="detail-meta">
            <span>新氧机构 ID：{store.externalOrgId || "-"}</span>
            <span>录像机 {store.recorderCount}</span>
            <span>有效通道 {store.channelCount}</span>
            <span>业务区域 {store.areaCount}</span>
          </div>
        </div>
      </header>

      <nav className="tabs" aria-label="门店详情 Tab">
        <button className={activeTab === "design-plan" ? "is-active" : ""} onClick={() => setActiveTab("design-plan")}>
          设计图标注
        </button>
        <button className={activeTab === "channels" ? "is-active" : ""} onClick={() => setActiveTab("channels")}>
          通道映射
        </button>
      </nav>

      {activeTab === "design-plan" ? (
        <DesignPlanTab store={store} saving={saving} onStoreUpdated={onStoreUpdated} onToast={onToast} />
      ) : (
        <VideoChannelTab
          store={store}
          accounts={accounts}
          onStoreUpdated={onStoreUpdated}
          onRecorderUpdated={updateRecorder}
          onToast={onToast}
        />
      )}
    </section>
  );
}
