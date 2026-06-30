import { Suspense, lazy, useEffect, useRef, useState } from "react";
import {
  storeSpaceApi,
  type AISettings,
  type CreateStoreSpacePayload,
  type EzvizAccount,
  type StoreDetail as StoreDetailType,
  type StoreListSummary,
  type StoreSummary,
  type UpdateStoreBasicInfoPayload,
} from "./api";
import { CreateStoreModal } from "./components/CreateStoreModal";
import { EditStoreModal } from "./components/EditStoreModal";
import { EzvizLiveDemo } from "./components/EzvizLiveDemo";
import { StoreDetail, type StoreDetailTab } from "./components/StoreDetail";
import { StoreList } from "./components/StoreList";
import { errorMessage } from "./domain/format";
import {
  createStoreDetailCache,
  canOpenH5Monitor,
  detailTabFromSummary,
  h5MonitorPath,
  makePendingStoreDetail,
  mergeStoreDetailTab,
  type StoreDetailNavigationTab,
  storeDetailTabFromDetail,
} from "./domain/store-detail-navigation";

const PAGE_SIZE = 20;
const APP_VERSION = import.meta.env.VITE_APP_VERSION || "local-dev";
const EMPTY_STORE_LIST_SUMMARY: StoreListSummary = { storeCount: 0, consultationCount: 0, treatmentCount: 0, beautyCount: 0 };

const H5MonitorPage = lazy(() => import("./pages/H5Monitor").then((module) => ({ default: module.H5Monitor })));
const H5MonitorChannelPage = lazy(() =>
  import("./pages/H5MonitorChannel").then((module) => ({ default: module.H5MonitorChannel })),
);

type H5Route =
  | { name: "home"; externalOrgId: string }
  | { name: "channel"; externalOrgId: string; channelId: number }
  | null;

function App() {
  const h5Route = parseH5Route();
  if (h5Route) {
    return <H5RouteShell initialRoute={h5Route} />;
  }

  return <AdminApp />;
}

function AdminApp() {
  const [stores, setStores] = useState<StoreSummary[]>([]);
  const [accounts, setAccounts] = useState<EzvizAccount[]>([]);
  const [query, setQuery] = useState("");
  const [cityFilter, setCityFilter] = useState("all");
  const [cityOptions, setCityOptions] = useState<string[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [listSummary, setListSummary] = useState<StoreListSummary>(EMPTY_STORE_LIST_SUMMARY);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [editingStore, setEditingStore] = useState<StoreSummary | null>(null);
  const [uploading, setUploading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [toast, setToast] = useState("");
  const [activeStore, setActiveStore] = useState<StoreDetailType | null>(null);
  const [loadingDetailTabs, setLoadingDetailTabs] = useState<Set<StoreDetailTab>>(() => new Set());
  const [loadedDetailTabs, setLoadedDetailTabs] = useState<Set<StoreDetailTab>>(() => new Set());
  const [activeTab, setActiveTab] = useState<StoreDetailTab>("design-plan");
  const [aiSettings, setAISettings] = useState<AISettings | null>(null);
  const [switchingAIModel, setSwitchingAIModel] = useState(false);
  const [deletingStoreIds, setDeletingStoreIds] = useState<Set<number>>(() => new Set());
  const [openingStoreIds, setOpeningStoreIds] = useState<Set<number>>(() => new Set());
  const listRequestIdRef = useRef(0);
  const detailRequestIdRef = useRef(0);
  const detailCacheRef = useRef(createStoreDetailCache());
  const activeStoreRef = useRef<StoreDetailType | null>(null);

  if (new URLSearchParams(window.location.search).get("tool") === "ezviz-live-demo") {
    return <EzvizLiveDemo appVersion={APP_VERSION} />;
  }

  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const visibleStores = stores;
  const visibleSummary = listSummary;
  const visibleFirstIndex = visibleStores.length === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const visibleLastIndex = visibleStores.length === 0 ? 0 : visibleFirstIndex + visibleStores.length - 1;

  useEffect(() => {
    void loadStores(query, cityFilter, page);
  }, [page, query, cityFilter]);

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

  async function loadStores(nextQuery = query, nextCityFilter = cityFilter, nextPage = page) {
    const requestId = listRequestIdRef.current + 1;
    listRequestIdRef.current = requestId;
    setLoading(true);
    try {
      const response = await storeSpaceApi.listStores(nextQuery.trim(), nextPage, PAGE_SIZE, nextCityFilter);
      if (listRequestIdRef.current !== requestId) return;
      setStores(response.items);
      setTotal(response.total);
      setListSummary(response.summary);
      setCityOptions(response.cities);
    } catch (error) {
      if (listRequestIdRef.current !== requestId) return;
      setStores([]);
      setTotal(0);
      setListSummary(EMPTY_STORE_LIST_SUMMARY);
      setCityOptions([]);
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

  function handleCityFilter(value: string) {
    setCityFilter(value);
    setPage(1);
  }

  async function openStore(storeId: number, tab?: StoreDetailTab) {
    if (openingStoreIds.has(storeId)) return;
    const summary = stores.find((store) => store.id === storeId);
    const requestId = detailRequestIdRef.current + 1;
    detailRequestIdRef.current = requestId;
    setOpeningStoreIds((current) => new Set(current).add(storeId));
    if (summary) {
      setActiveStore(makePendingStoreDetail(summary));
      const initialTab = tab ?? detailTabFromSummary(summary);
      setActiveTab(initialTab);
      setLoadedDetailTabs(new Set());
      setLoadingDetailTabs(new Set([initialTab]));
    }
    try {
      const initialTab = tab ?? (summary ? detailTabFromSummary(summary) : "channels");
      const cached = summary ? detailCacheRef.current.get(storeId, summary.updatedAt, initialTab) : null;
      if (cached) {
        if (detailRequestIdRef.current !== requestId) return;
        setActiveStore(cached);
        setActiveTab(initialTab);
        setLoadedDetailTabs(detailCacheRef.current.loadedTabs(storeId, cached.updatedAt));
        setLoadingDetailTabs(new Set());
        return;
      }
      const startedAt = performance.now();
      const detail = await loadStoreDetailTab(storeId, initialTab);
      if (detailRequestIdRef.current !== requestId) return;
      const nextDetail = mergeStoreDetailTab(summary ? makePendingStoreDetail(summary) : detail, detail, initialTab);
      detailCacheRef.current.set(nextDetail, [initialTab]);
      console.info(`[store-detail] loaded ${storeId}/${initialTab} in ${Math.round(performance.now() - startedAt)}ms`);
      setActiveStore(nextDetail);
      setActiveTab(initialTab);
      setLoadedDetailTabs(new Set([initialTab]));
      setLoadingDetailTabs(new Set());
    } catch (error) {
      if (detailRequestIdRef.current !== requestId) return;
      setActiveStore(null);
      setLoadedDetailTabs(new Set());
      setLoadingDetailTabs(new Set());
      setToast(errorMessage(error, "门店详情加载失败。"));
    } finally {
      setOpeningStoreIds((current) => removeIdFromSet(current, storeId));
    }
  }

  async function ensureDetailTabLoaded(tab: StoreDetailTab) {
    const currentStore = activeStoreRef.current;
    if (!currentStore || loadingDetailTabs.has(tab) || loadedDetailTabs.has(tab)) return;
    const requestId = detailRequestIdRef.current;
    setActiveTab(tab);
    const cached = detailCacheRef.current.get(currentStore.id, currentStore.updatedAt, tab);
    if (cached) {
      setActiveStore(cached);
      setLoadedDetailTabs(detailCacheRef.current.loadedTabs(currentStore.id, cached.updatedAt));
      return;
    }
    setLoadingDetailTabs((current) => new Set(current).add(tab));
    try {
      const startedAt = performance.now();
      const detail = await loadStoreDetailTab(currentStore.id, tab);
      if (detailRequestIdRef.current !== requestId) return;
      console.info(`[store-detail] loaded ${currentStore.id}/${tab} in ${Math.round(performance.now() - startedAt)}ms`);
      handleStoreUpdated((store) => mergeStoreDetailTab(store, detail, tab), [tab]);
      setLoadedDetailTabs((current) => new Set(current).add(tab));
    } catch (error) {
      if (detailRequestIdRef.current !== requestId) return;
      setToast(errorMessage(error, tab === "channels" ? "通道映射加载失败。" : "设计图标注加载失败。"));
    } finally {
      if (detailRequestIdRef.current === requestId) {
        setLoadingDetailTabs((current) => removeFromSet(current, tab));
      }
    }
  }

  function loadStoreDetailTab(storeId: number, tab: StoreDetailNavigationTab) {
    return tab === "channels" ? storeSpaceApi.getStoreChannelData(storeId) : storeSpaceApi.getStoreDesignPlanData(storeId);
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
      detailCacheRef.current.set(detail, ["design-plan", "channels"]);
      setCreateOpen(false);
      setActiveStore(detail);
      setLoadedDetailTabs(new Set(["design-plan", "channels"]));
      setLoadingDetailTabs(new Set());
      setActiveTab(storeDetailTabFromDetail(detail));
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
      detailCacheRef.current.set(detail, ["design-plan", "channels"]);
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
      detailCacheRef.current.delete(store.id);
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

  function handleStoreUpdated(update: StoreDetailType | ((store: StoreDetailType) => StoreDetailType), loadedTabs: StoreDetailTab[] = ["design-plan", "channels"]) {
    const currentStore = activeStoreRef.current;
    if (!currentStore && typeof update === "function") return;
    const nextStore = typeof update === "function" ? update(currentStore as StoreDetailType) : update;
    activeStoreRef.current = nextStore;
    detailCacheRef.current.set(nextStore, loadedTabs);
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
          loadingTabs={loadingDetailTabs}
          loadedTabs={loadedDetailTabs}
          saving={saving}
          accounts={accounts}
          aiSettings={aiSettings}
          switchingAIModel={switchingAIModel}
          h5MonitorUrl={canOpenH5Monitor(activeStore) ? h5MonitorPath(activeStore.externalOrgId) : undefined}
          onBack={() => {
            detailRequestIdRef.current += 1;
            setActiveStore(null);
            setLoadedDetailTabs(new Set());
            setLoadingDetailTabs(new Set());
            void loadStores();
          }}
          onTabChange={(tab) => void ensureDetailTabLoaded(tab)}
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
            <span>美容室 {visibleSummary.beautyCount}</span>
          </div>
        </div>
        <div className="city-filter" role="radiogroup" aria-label="城市筛选">
          <button type="button" role="radio" aria-checked={cityFilter === "all"} className={cityFilter === "all" ? "is-active" : ""} onClick={() => handleCityFilter("all")}>
            全部
          </button>
          {cityOptions.map((city) => (
            <button
              type="button"
              role="radio"
              aria-checked={cityFilter === city}
              className={cityFilter === city ? "is-active" : ""}
              key={city}
              onClick={() => handleCityFilter(city)}
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
        page={page}
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

function H5RouteShell({ initialRoute }: { initialRoute: H5Route }) {
  const [route, setRoute] = useState<H5Route>(initialRoute);

  useEffect(() => {
    const onPopState = () => setRoute(parseH5Route());
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  if (!route) {
    return (
      <div className="h5-not-found">
        <p>页面不存在。请通过门店监控二维码进入。</p>
      </div>
    );
  }

  if (route.name === "home") {
    return (
      <Suspense fallback={<div className="h5-loading">加载中...</div>}>
        <H5MonitorPage
          externalOrgId={route.externalOrgId}
          onOpenChannel={(channelId) => {
            const url = `${h5RoutePrefix()}/h5/orgs/${encodeURIComponent(route.externalOrgId)}/monitor/channels/${channelId}`;
            window.history.pushState({}, "", url);
            setRoute({ name: "channel", externalOrgId: route.externalOrgId, channelId });
          }}
        />
      </Suspense>
    );
  }

  return (
    <Suspense fallback={<div className="h5-loading">加载中...</div>}>
      <H5MonitorChannelPage
        externalOrgId={route.externalOrgId}
        channelId={route.channelId}
        onBack={() => {
          const url = `${h5RoutePrefix()}/h5/orgs/${encodeURIComponent(route.externalOrgId)}/monitor`;
          window.history.pushState({}, "", url);
          setRoute({ name: "home", externalOrgId: route.externalOrgId });
        }}
      />
    </Suspense>
  );
}

function parseH5Route(): H5Route {
  const path = window.location.pathname;
  const h5Path = stripKnownBasePrefix(path);

  const channelMatch = h5Path.match(/^\/h5\/orgs\/([^/]+)\/monitor\/channels\/([^/]+)$/);
  if (channelMatch) {
    const channelId = Number.parseInt(channelMatch[2], 10);
    if (Number.isInteger(channelId) && channelId > 0) {
      return { name: "channel", externalOrgId: decodeURIComponent(channelMatch[1]), channelId };
    }
    return null;
  }

  const homeMatch = h5Path.match(/^\/h5\/orgs\/([^/]+)\/monitor$/);
  if (homeMatch) {
    return { name: "home", externalOrgId: decodeURIComponent(homeMatch[1]) };
  }

  return null;
}

function stripKnownBasePrefix(path: string) {
  for (const prefix of h5BasePrefixes()) {
    if (prefix && path === prefix) return "/";
    if (prefix && path.startsWith(`${prefix}/`)) {
      return path.slice(prefix.length) || "/";
    }
  }
  return path;
}

function h5RoutePrefix() {
  const currentPrefix = h5BasePrefixes().find((prefix) => prefix && window.location.pathname.startsWith(`${prefix}/h5/`));
  return currentPrefix || normalizeBasePrefix(import.meta.env.BASE_URL || "/erzhuang-project/");
}

function h5BasePrefixes() {
  return Array.from(new Set([normalizeBasePrefix(import.meta.env.BASE_URL || "/erzhuang-project/"), "/erzhuang-project", "/erzhuang"].filter(Boolean)));
}

function normalizeBasePrefix(value: string) {
  const normalized = value.startsWith("/") ? value : `/${value}`;
  const firstSegment = normalized.split("/").filter(Boolean)[0];
  return firstSegment ? `/${firstSegment}` : "";
}

function removeIdFromSet(current: Set<number>, id: number) {
  return removeFromSet(current, id);
}

function removeFromSet<T>(current: Set<T>, value: T) {
  const next = new Set(current);
  next.delete(value);
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
