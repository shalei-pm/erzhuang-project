import type { StoreDetail, StoreSummary } from "../api";

export type StoreDetailNavigationTab = "design-plan" | "channels";

export const storeDetailCacheTTL = 60_000;

type CacheEntry = {
  detail: StoreDetail;
  cachedAt: number;
};

export function makePendingStoreDetail(summary: StoreSummary): StoreDetail {
  return {
    ...summary,
    fileName: "",
    originalPath: "",
    previewPath: "",
    thumbnailPath: "",
    pageCount: 0,
    previewUrl: "",
    areas: [],
    recorders: [],
  };
}

export function detailTabFromSummary(summary: StoreSummary): StoreDetailNavigationTab {
  if (summary.recorderCount > 0) return "channels";
  if (summary.designPlanStatus !== "not_uploaded" || Boolean(summary.thumbnailUrl)) return "design-plan";
  return "channels";
}

export function storeDetailTabFromDetail(store: StoreDetail): StoreDetailNavigationTab {
  if (store.recorderCount > 0 || store.recorders.length > 0) return "channels";
  if (store.designPlanStatus !== "not_uploaded" || Boolean(store.previewUrl || store.thumbnailUrl)) return "design-plan";
  return "channels";
}

export function createStoreDetailCache(ttlMs = storeDetailCacheTTL) {
  const entries = new Map<number, CacheEntry>();

  return {
    get(storeId: number, updatedAt: string, now = Date.now()): StoreDetail | null {
      const entry = entries.get(storeId);
      if (!entry) return null;
      if (entry.detail.updatedAt !== updatedAt) {
        entries.delete(storeId);
        return null;
      }
      if (now - entry.cachedAt > ttlMs) {
        entries.delete(storeId);
        return null;
      }
      return entry.detail;
    },
    set(detail: StoreDetail, now = Date.now()) {
      entries.set(detail.id, { detail, cachedAt: now });
    },
    delete(storeId: number) {
      entries.delete(storeId);
    },
  };
}
