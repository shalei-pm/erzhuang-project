import type { MonitorCategory } from "./h5-types";
import { readSessionStorage, safeSessionStorage, writeSessionStorage } from "./auth";

export type H5MonitorTabKey = "all" | MonitorCategory;

type TabStorage = Pick<Storage, "getItem" | "setItem"> | null;

export const H5_MONITOR_TAB_QUERY_PARAM = "tab";

export const h5MonitorTabs: Array<{ key: H5MonitorTabKey; label: string }> = [
  { key: "all", label: "全部" },
  { key: "consultation", label: "面诊室" },
  { key: "treatment", label: "治疗室" },
  { key: "beauty", label: "美容室" },
  { key: "front_waiting", label: "前台/等候区" },
  { key: "other", label: "过道/其他" },
];

export function h5MonitorActiveTabStorageKey(externalOrgId: string) {
  return `h5-monitor-active-tab:${externalOrgId}`;
}

export function readH5MonitorActiveTab(externalOrgId: string, storage: TabStorage = safeSessionStorage()): H5MonitorTabKey {
  if (!storage) return "all";
  const value = readSessionStorage(h5MonitorActiveTabStorageKey(externalOrgId), storage);
  return isH5MonitorTabKey(value) ? value : "all";
}

export function readH5MonitorActiveTabFromSearch(search: string | URLSearchParams): H5MonitorTabKey | null {
  const params = typeof search === "string" ? new URLSearchParams(search) : search;
  const value = params.get(H5_MONITOR_TAB_QUERY_PARAM);
  return isH5MonitorTabKey(value) ? value : null;
}

export function h5MonitorTabSearch(tab: H5MonitorTabKey | null | undefined) {
  if (!tab || tab === "all") return "";
  const params = new URLSearchParams();
  params.set(H5_MONITOR_TAB_QUERY_PARAM, tab);
  return `?${params.toString()}`;
}

export function storeH5MonitorActiveTab(externalOrgId: string, tab: H5MonitorTabKey, storage: TabStorage = safeSessionStorage()) {
  if (!storage) return;
  writeSessionStorage(h5MonitorActiveTabStorageKey(externalOrgId), tab, storage);
}

export function isH5MonitorTabKey(value: string | null): value is H5MonitorTabKey {
  return h5MonitorTabs.some((tab) => tab.key === value);
}
