import { useEffect, useState } from "react";
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
  onStoreUpdated: (update: StoreDetailType | ((store: StoreDetailType) => StoreDetailType)) => void;
  onToast: (message: string) => void;
};

export function StoreDetail({ store, initialTab, saving, accounts, onBack, onStoreUpdated, onToast }: StoreDetailProps) {
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

  return (
    <section className="detail-page">
      <header className="detail-header">
        <div>
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
      </header>

      <nav className="tabs" aria-label="门店详情 Tab">
        <button className={activeTab === "design-plan" ? "is-active" : ""} onClick={() => setActiveTab("design-plan")}>
          设计图标注
        </button>
        <button className={activeTab === "channels" ? "is-active" : ""} onClick={() => setActiveTab("channels")}>
          通道映射
        </button>
      </nav>

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
    </section>
  );
}
