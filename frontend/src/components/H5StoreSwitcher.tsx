import { useEffect, useId, useMemo, useRef, useState } from "react";
import { h5Api, H5ApiError } from "../api-h5";
import type { H5MonitorStoreInfo, H5MonitorStoresResponse } from "../domain/h5-types";

type H5StoreSwitcherProps = {
  currentExternalOrgId: string;
  currentStoreName?: string;
  currentCity?: string;
  onAuthRequired?: (error?: unknown) => void;
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
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const panelId = useId();

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
          onAuthRequired?.(err);
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

  useEffect(() => {
    if (!open) return;

    function handlePointerDown(event: PointerEvent) {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }

    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

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
    <div className="h5-store-dropdown" ref={rootRef}>
      <button
        type="button"
        className="h5-store-trigger"
        aria-label="切换门店"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen((value) => !value)}
      >
        <span className="h5-store-trigger-title-row">
          <span className="h5-store-trigger-main">{displayStore.store_name}</span>
          <span className="h5-store-trigger-chevron" aria-hidden="true">
            <svg className="h5-store-trigger-chevron-icon" viewBox="0 0 16 16" focusable="false">
              <path d="M4.5 6.25 8 9.75l3.5-3.5" />
            </svg>
          </span>
        </span>
        {displayStore.city ? <span className="h5-store-trigger-city">{displayStore.city}</span> : null}
      </button>
      {open ? (
        <div className="h5-store-dropdown-panel" id={panelId} role="menu">
          {loading ? <span className="h5-store-dropdown-status">门店加载中...</span> : null}
          {!loading && error ? <span className="h5-store-dropdown-status error">{error}</span> : null}
          {!loading && !error && currentStore ? null : !loading && !error ? (
            <StoreButton
              store={fallbackStore}
              active
              onSelectStore={onSelectStore}
              onClose={() => setOpen(false)}
            />
          ) : null}
          {!loading && !error
            ? storeGroups.map((group) => (
                <div className="h5-store-dropdown-group" key={group.city || "unknown"}>
                  <span className="h5-store-dropdown-city">{group.city || "未分城市"}</span>
                  <div className="h5-store-dropdown-options">
                    {group.stores.map((store) => (
                      <StoreButton
                        key={store.external_org_id}
                        store={store}
                        active={store.external_org_id === currentExternalOrgId}
                        onSelectStore={onSelectStore}
                        onClose={() => setOpen(false)}
                      />
                    ))}
                  </div>
                </div>
              ))
            : null}
        </div>
      ) : null}
    </div>
  );
}

function StoreButton({
  store,
  active,
  onSelectStore,
  onClose,
}: {
  store: H5MonitorStoreInfo;
  active: boolean;
  onSelectStore: (externalOrgId: string) => void;
  onClose: () => void;
}) {
  return (
    <button
      type="button"
      className={active ? "active" : ""}
      role="menuitem"
      onClick={() => {
        onClose();
        if (!active) onSelectStore(store.external_org_id);
      }}
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
