import { useEffect, useMemo, useState } from "react";
import { h5Api, H5ApiError } from "../api-h5";
import type { H5MonitorStoreInfo, H5MonitorStoresResponse } from "../domain/h5-types";

type H5StoreSwitcherProps = {
  currentExternalOrgId: string;
  currentStoreName?: string;
  currentCity?: string;
  onAuthRequired?: () => void;
  onSelectStore: (externalOrgId: string) => void;
};

export function H5StoreSwitcher({
  currentExternalOrgId,
  currentStoreName,
  currentCity,
  onAuthRequired,
  onSelectStore,
}: H5StoreSwitcherProps) {
  const [storesResponse, setStoresResponse] = useState<H5MonitorStoresResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    h5Api
      .listMonitorStores()
      .then((response) => {
        if (cancelled) return;
        setStoresResponse(response);
        setError("");
      })
      .catch((err) => {
        if (cancelled) return;
        if (err instanceof H5ApiError && err.status === 401) {
          onAuthRequired?.();
          return;
        }
        setError(errorMessage(err, "门店列表加载失败"));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [onAuthRequired]);

  const storeGroups = useMemo(() => storesResponse?.cities ?? [], [storesResponse]);
  const allStores = useMemo(() => storeGroups.flatMap((group) => group.stores), [storeGroups]);
  const currentStore = allStores.find((store) => store.external_org_id === currentExternalOrgId);
  const fallbackStore: H5MonitorStoreInfo = {
    external_org_id: currentExternalOrgId,
    store_name: currentStoreName || `机构 ${currentExternalOrgId}`,
    city: currentCity || "",
    available_channel_count: 0,
  };
  const displayStore = currentStore ?? fallbackStore;

  return (
    <section className="h5-store-switcher" aria-label="切换门店">
      <div className="h5-store-switcher-current">
        <span>当前门店</span>
        <strong>{displayStore.store_name}</strong>
        {displayStore.city ? <em>{displayStore.city}</em> : null}
      </div>
      <div className="h5-store-switcher-list">
        {loading ? <span className="h5-store-switcher-status">门店加载中...</span> : null}
        {!loading && error ? <span className="h5-store-switcher-status error">{error}</span> : null}
        {!loading && !error && currentStore ? null : !loading && !error ? (
          <StoreButton store={fallbackStore} active onSelectStore={onSelectStore} />
        ) : null}
        {!loading && !error
          ? storeGroups.map((group) => (
              <div className="h5-store-switcher-group" key={group.city || "unknown"}>
                <span className="h5-store-switcher-city">{group.city || "未分城市"}</span>
                <div className="h5-store-switcher-buttons">
                  {group.stores.map((store) => (
                    <StoreButton
                      key={store.external_org_id}
                      store={store}
                      active={store.external_org_id === currentExternalOrgId}
                      onSelectStore={onSelectStore}
                    />
                  ))}
                </div>
              </div>
            ))
          : null}
      </div>
    </section>
  );
}

function StoreButton({
  store,
  active,
  onSelectStore,
}: {
  store: H5MonitorStoreInfo;
  active: boolean;
  onSelectStore: (externalOrgId: string) => void;
}) {
  return (
    <button
      type="button"
      className={active ? "active" : ""}
      onClick={() => onSelectStore(store.external_org_id)}
      aria-current={active ? "page" : undefined}
    >
      <span>{store.store_name}</span>
      {store.available_channel_count > 0 ? <em>{store.available_channel_count} 路</em> : null}
    </button>
  );
}

function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof H5ApiError) {
    if (err.status === 403) return "暂无访问权限";
    return err.message || fallback;
  }
  if (err instanceof Error && err.message.trim()) return err.message;
  return fallback;
}
