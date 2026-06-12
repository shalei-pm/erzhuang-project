import { useEffect, useRef, useState } from "react";
import { storeSpaceApi, type CreateStoreSpacePayload, type EzvizAccount, type StoreDetail as StoreDetailType, type StoreSummary } from "./api";
import { CreateStoreModal } from "./components/CreateStoreModal";
import { StoreDetail, type StoreDetailTab } from "./components/StoreDetail";
import { StoreList } from "./components/StoreList";
import { errorMessage } from "./domain/format";

const PAGE_SIZE = 20;
const APP_VERSION = import.meta.env.VITE_APP_VERSION || "local-dev";

function App() {
  const [stores, setStores] = useState<StoreSummary[]>([]);
  const [accounts, setAccounts] = useState<EzvizAccount[]>([]);
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [toast, setToast] = useState("");
  const [activeStore, setActiveStore] = useState<StoreDetailType | null>(null);
  const [activeTab, setActiveTab] = useState<StoreDetailTab>("design-plan");
  const listRequestIdRef = useRef(0);

  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const firstIndex = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const lastIndex = Math.min(total, page * PAGE_SIZE);

  useEffect(() => {
    void loadStores(query, page);
  }, [page, query]);

  useEffect(() => {
    void storeSpaceApi
      .listEzvizAccounts()
      .then(setAccounts)
      .catch((error) => setToast(errorMessage(error, "萤石云账号加载失败。")));
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
    setPage(1);
  }

  async function openStore(storeId: number, tab: StoreDetailTab = "design-plan") {
    try {
      const detail = await storeSpaceApi.getStore(storeId);
      setActiveStore(detail);
      setActiveTab(tab);
    } catch (error) {
      setToast(errorMessage(error, "门店详情加载失败。"));
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
      setActiveTab(payload.designPlan ? "design-plan" : "channels");
      setToast("门店已创建，请继续完善空间资源。");
      await loadStores();
    } catch (error) {
      setToast(errorMessage(error, "创建门店失败。"));
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
    try {
      await storeSpaceApi.deleteStore(store.id);
      setToast(`已删除：${store.name}`);
      if (activeStore?.id === store.id) {
        setActiveStore(null);
      }
      await loadStores();
    } catch (error) {
      setToast(errorMessage(error, "删除门店失败。"));
    }
  }

  function handleStoreUpdated(store: StoreDetailType) {
    setActiveStore(store);
    setStores((items) => items.map((item) => (item.id === store.id ? store : item)));
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
          onBack={() => {
            setActiveStore(null);
            void loadStores();
          }}
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
        <label className="search-field">
          <span aria-hidden="true">⌕</span>
          <input value={query} onChange={(event) => handleSearch(event.target.value)} placeholder="搜索门店名称" aria-label="搜索门店名称" />
        </label>
        <div className="toolbar-meta">
          共 {total} 家门店
          {total > 0 ? `，当前 ${firstIndex}-${lastIndex}` : ""}
        </div>
      </section>

      {toast ? <Toast message={toast} onClose={() => setToast("")} /> : null}

      <StoreList stores={stores} loading={loading} page={page} pageSize={PAGE_SIZE} onOpenStore={openStore} onDeleteStore={deleteStore} />

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
    </main>
  );
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
