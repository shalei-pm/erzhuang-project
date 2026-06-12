import { useState } from "react";
import type { EzvizAccount, StoreDetail as StoreDetailType, VideoRecorder } from "../api";
import { DesignPlanTab } from "./DesignPlanTab";
import { VideoChannelTab } from "./VideoChannelTab";

export type StoreDetailTab = "design-plan" | "channels";

type StoreDetailProps = {
  store: StoreDetailType;
  initialTab: StoreDetailTab;
  saving: boolean;
  accounts: EzvizAccount[];
  onBack: () => void;
  onStoreUpdated: (store: StoreDetailType) => void;
  onToast: (message: string) => void;
};

export function StoreDetail({ store, initialTab, saving, accounts, onBack, onStoreUpdated, onToast }: StoreDetailProps) {
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
