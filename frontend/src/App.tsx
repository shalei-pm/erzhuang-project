import { useEffect, useRef, useState } from "react";
import {
  storeSpaceApi,
  type AISettings,
  type CreateStoreSpacePayload,
  type EzvizAccount,
  type StoreDetail as StoreDetailType,
  type StoreSummary,
  type UpdateStoreBasicInfoPayload,
} from "./api";
import { CreateStoreModal } from "./components/CreateStoreModal";
import { EditStoreModal } from "./components/EditStoreModal";
import { EzvizLiveDemo } from "./components/EzvizLiveDemo";
import { StoreDetail, type StoreDetailTab } from "./components/StoreDetail";
import { StoreList } from "./components/StoreList";
import { errorMessage } from "./domain/format";

const PAGE_SIZE = 20;
const APP_VERSION = import.meta.env.VITE_APP_VERSION || "local-dev";

function App() {
  const [stores, setStores] = useState<StoreSummary[]>([]);
  const [accounts, setAccounts] = useState<EzvizAccount[]>([]);
  const [query, setQuery] = useState("");
  const [cityFilter, setCityFilter] = useState("all");
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [editingStore, setEditingStore] = useState<StoreSummary | null>(null);
  const [uploading, setUploading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [toast, setToast] = useState("");
  const [activeStore, setActiveStore] = useState<StoreDetailType | null>(null);
  const [activeTab, setActiveTab] = useState<StoreDetailTab>("design-plan");
  const [aiSettings, setAISettings] = useState<AISettings | null>(null);
  const [switchingAIModel, setSwitchingAIModel] = useState(false);
  const [deletingStoreIds, setDeletingStoreIds] = useState<Set<number>>(() => new Set());
  const [openingStoreIds, setOpeningStoreIds] = useState<Set<number>>(() => new Set());
  const listRequestIdRef = useRef(0);
  const activeStoreRef = useRef<StoreDetailType | null>(null);

  if (new URLSearchParams(window.location.search).get("tool") === "ezviz-live-demo") {
    return <EzvizLiveDemo appVersion={APP_VERSION} />;
  }

  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const cityOptions = uniqueCities(stores);
  const visibleStores = cityFilter === "all" ? stores : stores.filter((store) => storeCity(store) === cityFilter);
  const visibleSummary = summarizeStores(visibleStores);
  const visibleFirstIndex = visibleStores.length === 0 ? 0 : 1;
  const visibleLastIndex = visibleStores.length;

  useEffect(() => {
    void loadStores(query, page);
  }, [page, query]);

  useEffect(() => {
    activeStoreRef.current = activeStore;
  }, [activeStore]);

  useEffect(() => {
    void storeSpaceApi
      .listEzvizAccounts()
      .then(setAccounts)
      .catch((error) => setToast(errorMessage(error, "萤石云账号加载失败。")));
  }, []);

  useEffect(() => {
    void loadAISettings();
  }, []);

  async function loadStores(nextQuery = query, nextPage = page) {
    const requestId = listRequestIdRef.current + 1;
    listRequestIdRef.current = requestId;
    setLoading(true);
    try {
      const response = await storeSpaceApi.listStores(nextQuery.trim(), nextPage, PAGE_SIZE);
      if (listRequestIdRef.current !== requestId) return;
      setStores(response.items);
      setTotal(response.total);
    } catch (error) {
      if (listRequestIdRef.current !== requestId) return;
      setStores([]);
      setTotal(0);
      setToast(errorMessage(error, "门店列表加载失败，请稍后重试。"));
    } finally {
      if (listRequestIdRef.current === requestId) {
        setLoading(false);
      }
    }
  }

  function handleSearch(value: string) {
    setQuery(value);
    setCityFilter("all");
    setPage(1);
  }

  async function openStore(storeId: number, tab?: StoreDetailTab) {
    if (openingStoreIds.has(storeId)) return;
    setOpeningStoreIds((current) => new Set(current).add(storeId));
    try {
      const detail = await storeSpaceApi.getStore(storeId);
      setActiveStore(detail);
      setActiveTab(tab ?? defaultStoreDetailTab(detail));
    } catch (error) {
      setToast(errorMessage(error, "门店详情加载失败。"));
    } finally {
      setOpeningStoreIds((current) => removeIdFromSet(current, storeId));
    }
  }

  async function loadAISettings() {
    try {
      const settings = await storeSpaceApi.getAISettings();
      setAISettings(settings);
    } catch (error) {
      setToast(errorMessage(error, "识别模型状态加载失败。"));
    }
  }

  async function toggleAIModel() {
    if (switchingAIModel) return;
    setSwitchingAIModel(true);
    try {
      const settings = await storeSpaceApi.toggleAISettings();
      setAISettings(settings);
      setToast(`已切换识别模型：${settings.label}`);
    } catch (error) {
      setToast(errorMessage(error, "识别模型切换失败。"));
    } finally {
      setSwitchingAIModel(false);
    }
  }

  async function createStore(payload: CreateStoreSpacePayload) {
    setSaving(true);
    try {
      const duplicate = await storeSpaceApi.checkDuplicate(payload.name);
      if (duplicate.exactMatch) {
        setToast("同名门店已存在，请修改门店名称。");
        return;
      }
      if (duplicate.similarMatches.length > 0) {
        const ok = window.confirm(`发现 ${duplicate.similarMatches.length} 个疑似同名门店，是否继续创建？`);
        if (!ok) return;
      }
      const detail = await storeSpaceApi.createStore(payload);
      setCreateOpen(false);
      setActiveStore(detail);
      setActiveTab(defaultStoreDetailTab(detail));
      setToast("门店已创建，请继续完善空间资源。");
      await loadStores();
    } catch (error) {
      setToast(errorMessage(error, "创建门店失败。"));
    } finally {
      setSaving(false);
    }
  }

  async function updateStoreBasicInfo(payload: UpdateStoreBasicInfoPayload) {
    setSaving(true);
    try {
      const duplicate = await storeSpaceApi.checkDuplicate(payload.name, payload.id);
      if (duplicate.exactMatch) {
        setToast("同名门店已存在，请修改门店名称。");
        return;
      }
      if (duplicate.similarMatches.length > 0) {
        const ok = window.confirm(`发现 ${duplicate.similarMatches.length} 个疑似同名门店，是否继续保存？`);
        if (!ok) return;
      }
      const detail = await storeSpaceApi.updateStoreBasicInfo(payload);
      setEditingStore(null);
      setStores((items) => items.map((item) => (item.id === detail.id ? detail : item)));
      if (activeStore?.id === detail.id) {
        setActiveStore(detail);
      }
      setToast("机构信息已更新。");
      await loadStores();
    } catch (error) {
      setToast(errorMessage(error, "更新机构信息失败。"));
    } finally {
      setSaving(false);
    }
  }

  function defaultStoreDetailTab(store: StoreDetailType): StoreDetailTab {
    if (store.recorderCount > 0 || store.recorders.length > 0) return "channels";
    if (store.designPlanStatus !== "not_uploaded" || Boolean(store.previewUrl || store.thumbnailUrl)) return "design-plan";
    return "channels";
  }

  async function uploadPdf(file: File) {
    setUploading(true);
    try {
      return await storeSpaceApi.uploadPdf(file);
    } finally {
      setUploading(false);
    }
  }

  async function deleteStore(store: StoreSummary) {
    const ok = window.confirm("删除后将清除该门店的设计图、区域、录像机、通道、截图和识别结果，且无法恢复。是否确认删除？");
    if (!ok) return;
    setDeletingStoreIds((current) => new Set(current).add(store.id));
    try {
      await storeSpaceApi.deleteStore(store.id);
      setToast(`已删除：${store.name}`);
      if (activeStore?.id === store.id) {
        setActiveStore(null);
      }
      await loadStores();
    } catch (error) {
      setToast(errorMessage(error, "删除门店失败。"));
    } finally {
      setDeletingStoreIds((current) => removeIdFromSet(current, store.id));
    }
  }

  function handleStoreUpdated(update: StoreDetailType | ((store: StoreDetailType) => StoreDetailType)) {
    const currentStore = activeStoreRef.current;
    if (!currentStore && typeof update === "function") return;
    const nextStore = typeof update === "function" ? update(currentStore as StoreDetailType) : update;
    activeStoreRef.current = nextStore;
    setActiveStore(nextStore);
    setStores((items) => items.map((item) => (item.id === nextStore.id ? nextStore : item)));
  }

  if (activeStore) {
    return (
      <main className="app-shell">
        {toast ? <Toast message={toast} onClose={() => setToast("")} /> : null}
        <StoreDetail
          store={activeStore}
          initialTab={activeTab}
          saving={saving}
          accounts={accounts}
          aiSettings={aiSettings}
          switchingAIModel={switchingAIModel}
          onBack={() => {
            setActiveStore(null);
            void loadStores();
          }}
          onToggleAIModel={toggleAIModel}
          onStoreUpdated={handleStoreUpdated}
          onToast={setToast}
        />
        <footer className="app-version" aria-label="当前版本">
          版本 {APP_VERSION}
        </footer>
      </main>
    );
  }

  return (
    <main className="app-shell">
      <header className="page-header">
        <div>
          <p className="eyebrow">空间资源管理</p>
          <h1>门店空间资源管理系统</h1>
        </div>
        <button className="primary-button" onClick={() => setCreateOpen(true)}>
          <span aria-hidden="true">+</span>
          添加门店
        </button>
      </header>

      <section className="toolbar" aria-label="门店筛选">
        <div className="toolbar-main">
          <label className="search-field">
            <span aria-hidden="true">⌕</span>
            <input value={query} onChange={(event) => handleSearch(event.target.value)} placeholder="搜索门店名称" aria-label="搜索门店名称" />
          </label>
          <div className="toolbar-meta">
            <span>
              共 {visibleSummary.storeCount} 家门店
              {visibleSummary.storeCount > 0 ? `，当前 ${visibleFirstIndex}-${visibleLastIndex}` : ""}
            </span>
            <span>面诊室 {visibleSummary.consultationCount}</span>
            <span>治疗室 {visibleSummary.treatmentCount}</span>
            <span>生美 {visibleSummary.beautyCount}</span>
          </div>
        </div>
        <div className="city-filter" role="radiogroup" aria-label="城市筛选">
          <button type="button" role="radio" aria-checked={cityFilter === "all"} className={cityFilter === "all" ? "is-active" : ""} onClick={() => setCityFilter("all")}>
            全部
          </button>
          {cityOptions.map((city) => (
            <button
              type="button"
              role="radio"
              aria-checked={cityFilter === city}
              className={cityFilter === city ? "is-active" : ""}
              key={city}
              onClick={() => setCityFilter(city)}
            >
              {city}
            </button>
          ))}
        </div>
      </section>

      {toast ? <Toast message={toast} onClose={() => setToast("")} /> : null}

      <StoreList
        stores={visibleStores}
        loading={loading}
        page={cityFilter === "all" ? page : 1}
        pageSize={PAGE_SIZE}
        deletingStoreIds={deletingStoreIds}
        openingStoreIds={openingStoreIds}
        onOpenStore={openStore}
        onEditStore={setEditingStore}
        onDeleteStore={deleteStore}
      />

      <nav className="pagination" aria-label="分页">
        <button disabled={page <= 1} onClick={() => setPage((value) => Math.max(1, value - 1))}>
          上一页
        </button>
        <span>
          第 {page} / {pageCount} 页
        </span>
        <button disabled={page >= pageCount} onClick={() => setPage((value) => Math.min(pageCount, value + 1))}>
          下一页
        </button>
      </nav>

      <footer className="app-version" aria-label="当前版本">
        版本 {APP_VERSION}
      </footer>

      {createOpen ? (
        <CreateStoreModal
          accounts={accounts}
          uploading={uploading}
          saving={saving}
          onUploadPdf={uploadPdf}
          onClose={() => setCreateOpen(false)}
          onSubmit={createStore}
        />
      ) : null}
      {editingStore ? (
        <EditStoreModal
          store={editingStore}
          saving={saving}
          onClose={() => setEditingStore(null)}
          onSubmit={updateStoreBasicInfo}
        />
      ) : null}
    </main>
  );
}

function storeCity(store: StoreSummary) {
  return store.city || "未设置";
}

function uniqueCities(stores: StoreSummary[]) {
  return Array.from(new Set(stores.map(storeCity))).sort((left, right) => left.localeCompare(right, "zh-Hans-CN"));
}

function summarizeStores(stores: StoreSummary[]) {
  return stores.reduce(
    (summary, store) => ({
      storeCount: summary.storeCount + 1,
      consultationCount: summary.consultationCount + store.consultationCount,
      treatmentCount: summary.treatmentCount + store.treatmentCount,
      beautyCount: summary.beautyCount + store.beautyCount,
    }),
    { storeCount: 0, consultationCount: 0, treatmentCount: 0, beautyCount: 0 },
  );
}

function removeIdFromSet(current: Set<number>, id: number) {
  const next = new Set(current);
  next.delete(id);
  return next;
}

function Toast({ message, onClose }: { message: string; onClose: () => void }) {
  return (
    <div className="toast" role="status">
      {message}
      <button onClick={onClose} aria-label="关闭提示">
        x
      </button>
    </div>
  );
}

export default App;
