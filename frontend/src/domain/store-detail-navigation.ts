import type { StoreDetail, StoreSummary } from "../api";

export type StoreDetailNavigationTab = "design-plan" | "channels";

export const storeDetailCacheTTL = 60_000;

type CacheEntry = {
  detail: StoreDetail;
  loadedTabs: Set<StoreDetailNavigationTab>;
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
    get(storeId: number, updatedAt: string, requiredTab?: StoreDetailNavigationTab, now = Date.now()): StoreDetail | null {
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
      if (requiredTab && !entry.loadedTabs.has(requiredTab)) return null;
      return entry.detail;
    },
    loadedTabs(storeId: number, updatedAt: string, now = Date.now()) {
      const entry = this.getEntry(storeId, updatedAt, now);
      return entry ? new Set(entry.loadedTabs) : new Set<StoreDetailNavigationTab>();
    },
    set(detail: StoreDetail, loadedTabs: Iterable<StoreDetailNavigationTab> = ["design-plan", "channels"], now = Date.now()) {
      const previous = entries.get(detail.id);
      const nextLoadedTabs = new Set(previous?.loadedTabs ?? []);
      for (const tab of loadedTabs) nextLoadedTabs.add(tab);
      entries.set(detail.id, { detail, loadedTabs: nextLoadedTabs, cachedAt: now });
    },
    merge(detail: StoreDetail, tab: StoreDetailNavigationTab, now = Date.now()) {
      const previous = entries.get(detail.id)?.detail;
      const nextDetail = mergeStoreDetailTab(previous ?? makePendingStoreDetail(detail), detail, tab);
      this.set(nextDetail, [tab], now);
      return nextDetail;
    },
    delete(storeId: number) {
      entries.delete(storeId);
    },
    getEntry(storeId: number, updatedAt: string, now = Date.now()): CacheEntry | null {
      const entry = entries.get(storeId);
      if (!entry) return null;
      if (entry.detail.updatedAt !== updatedAt || now - entry.cachedAt > ttlMs) {
        entries.delete(storeId);
        return null;
      }
      return entry;
    },
  };
}

export function mergeStoreDetailTab(current: StoreDetail, incoming: StoreDetail, tab: StoreDetailNavigationTab): StoreDetail {
  if (tab === "channels") {
    return {
      ...current,
      ...summaryFields(incoming),
      recorders: incoming.recorders,
    };
  }
  return {
    ...current,
    ...summaryFields(incoming),
    fileName: incoming.fileName,
    originalPath: incoming.originalPath,
    previewPath: incoming.previewPath,
    thumbnailPath: incoming.thumbnailPath,
    pageCount: incoming.pageCount,
    previewUrl: incoming.previewUrl,
    thumbnailUrl: incoming.thumbnailUrl,
    recognitionResult: incoming.recognitionResult,
    areas: incoming.areas,
  };
}

function summaryFields(store: StoreDetail): StoreSummary {
  return {
    id: store.id,
    city: store.city,
    name: store.name,
    externalOrgId: store.externalOrgId,
    thumbnailUrl: store.thumbnailUrl,
    designPlanStatus: store.designPlanStatus,
    recorderCount: store.recorderCount,
    channelCount: store.channelCount,
    channelsFullyConfirmed: store.channelsFullyConfirmed,
    treatmentCount: store.treatmentCount,
    consultationCount: store.consultationCount,
    beautyCount: store.beautyCount,
    areaCount: store.areaCount,
    status: store.status,
    updatedAt: store.updatedAt,
  };
}
