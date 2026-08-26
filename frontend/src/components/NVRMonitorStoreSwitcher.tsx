import { useEffect, useId, useMemo, useRef, useState } from "react";
import { nvrLabApi, NVRLabApiError } from "../api-nvr-lab";
import type { NVRMonitorStoreInfo, NVRMonitorStoresResponse } from "../domain/nvr-lab";

type Props = {
  currentExternalOrgId: string;
  currentStoreName?: string;
  currentCity?: string;
  onAuthRequired?: () => void;
  onSelectStore: (externalOrgId: string) => void;
};

export function NVRMonitorStoreSwitcher({ currentExternalOrgId, currentStoreName, currentCity, onAuthRequired, onSelectStore }: Props) {
  const [response, setResponse] = useState<NVRMonitorStoresResponse | null>(null);
  const [open, setOpen] = useState(false);
  const [message, setMessage] = useState("");
  const rootRef = useRef<HTMLDivElement | null>(null);
  const panelId = useId();

  useEffect(() => {
    let cancelled = false;
    nvrLabApi.listMonitorStores().then((next) => {
      if (!cancelled) setResponse(next);
    }).catch((error) => {
      if (cancelled) return;
      if (error instanceof NVRLabApiError && error.status === 401) onAuthRequired?.();
      else setMessage(error instanceof Error ? error.message : "门店列表加载失败");
    });
    return () => { cancelled = true; };
  }, [onAuthRequired]);

  useEffect(() => {
    if (!open) return;
    const close = (event: PointerEvent) => { if (!rootRef.current?.contains(event.target as Node)) setOpen(false); };
    document.addEventListener("pointerdown", close);
    return () => document.removeEventListener("pointerdown", close);
  }, [open]);

  const stores = useMemo(() => response?.cities.flatMap((city) => city.stores) ?? [], [response]);
  const current = stores.find((store) => store.external_org_id === currentExternalOrgId) ?? {
    external_org_id: currentExternalOrgId,
    store_name: currentStoreName || `机构 ${currentExternalOrgId}`,
    city: currentCity || "",
    available_camera_count: 0,
  };
  return <div className="h5-store-dropdown" ref={rootRef}>
    <button type="button" className="h5-store-trigger" aria-expanded={open} aria-controls={panelId} onClick={() => setOpen((value) => !value)}>
      <span className="h5-store-trigger-title-row"><span className="h5-store-trigger-main">{current.store_name}</span><span className="h5-store-trigger-chevron" aria-hidden="true">v</span></span>
      {current.city ? <span className="h5-store-trigger-city">{current.city}</span> : null}
    </button>
    {open ? <div className="h5-store-dropdown-panel" id={panelId} role="menu">
      {message ? <span className="h5-store-dropdown-status error">{message}</span> : null}
      {response?.cities.map((group) => <div className="h5-store-dropdown-group" key={group.city}>
        <span className="h5-store-dropdown-city">{group.city}</span><div className="h5-store-dropdown-options">
          {group.stores.map((store) => <StoreButton key={store.external_org_id} store={store} active={store.external_org_id === currentExternalOrgId} onSelect={() => { setOpen(false); onSelectStore(store.external_org_id); }} />)}
        </div>
      </div>)}
    </div> : null}
  </div>;
}

function StoreButton({ store, active, onSelect }: { store: NVRMonitorStoreInfo; active: boolean; onSelect: () => void }) {
  return <button type="button" className={active ? "active" : ""} role="menuitem" aria-current={active ? "page" : undefined} onClick={onSelect}><span>{store.store_name}</span><em>{store.available_camera_count} 路</em></button>;
}
