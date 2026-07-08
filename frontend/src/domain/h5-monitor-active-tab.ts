import type { MonitorCategory } from "./h5-types";

export type H5MonitorTabKey = "all" | MonitorCategory;

export const h5MonitorTabs: Array<{ key: H5MonitorTabKey; label: string }> = [
  { key: "all", label: "全部" },
  { key: "consultation", label: "面诊室" },
  { key: "treatment", label: "治疗室" },
  { key: "beauty", label: "美容室" },
  { key: "front_waiting", label: "前台/等候区" },
  { key: "other", label: "过道/其他" },
];

type TabStorage = Pick<Storage, "getItem" | "setItem">;

export function h5MonitorActiveTabStorageKey(externalOrgId: string) {
  return `h5-monitor-active-tab:${externalOrgId}`;
}

export function readH5MonitorActiveTab(externalOrgId: string, storage = browserSessionStorage()): H5MonitorTabKey {
  if (!storage) return "all";
  try {
    const value = storage.getItem(h5MonitorActiveTabStorageKey(externalOrgId));
    return isH5MonitorTabKey(value) ? value : "all";
  } catch {
    return "all";
  }
}

export function storeH5MonitorActiveTab(externalOrgId: string, tab: H5MonitorTabKey, storage = browserSessionStorage()) {
  if (!storage) return;
  try {
    storage.setItem(h5MonitorActiveTabStorageKey(externalOrgId), tab);
  } catch {
    // 页面状态恢复是体验增强；存储不可用时保持本次页面内状态即可。
  }
}

export function isH5MonitorTabKey(value: string | null): value is H5MonitorTabKey {
  return h5MonitorTabs.some((tab) => tab.key === value);
}

function browserSessionStorage(): TabStorage | null {
  if (typeof window === "undefined") return null;
  return window.sessionStorage;
}
