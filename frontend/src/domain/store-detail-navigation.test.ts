import type { StoreDetail, StoreSummary } from "../api.js";
import {
  createStoreDetailCache,
  detailTabFromSummary,
  makePendingStoreDetail,
  mergeStoreDetailTab,
  storeDetailTabFromDetail,
} from "./store-detail-navigation.js";

const baseSummary: StoreSummary = {
  id: 7,
  city: "上海",
  name: "新氧青春诊所 上海测试店",
  externalOrgId: "SOY-7",
  thumbnailUrl: "",
  designPlanStatus: "not_uploaded",
  recorderCount: 0,
  channelCount: 0,
  channelsFullyConfirmed: false,
  treatmentCount: 0,
  consultationCount: 0,
  beautyCount: 0,
  areaCount: 0,
  status: "needs_review",
  updatedAt: "2026-06-26T08:00:00.000Z",
};

function summary(patch: Partial<StoreSummary>): StoreSummary {
  return { ...baseSummary, ...patch };
}

function detail(patch: Partial<StoreDetail>): StoreDetail {
  return {
    ...baseSummary,
    fileName: "",
    originalPath: "",
    previewPath: "",
    thumbnailPath: "",
    pageCount: 0,
    previewUrl: "",
    areas: [],
    recorders: [],
    ...patch,
  };
}

const pending = makePendingStoreDetail(summary({ recorderCount: 2, channelCount: 12 }));

assertEqual(pending.name, "新氧青春诊所 上海测试店");
assertEqual(pending.recorderCount, 2);
assertEqual(pending.channelCount, 12);
assertEqual(JSON.stringify(pending.recorders), "[]");
assertEqual(JSON.stringify(pending.areas), "[]");
assertEqual(pending.previewUrl, "");

assertEqual(detailTabFromSummary(summary({ recorderCount: 1, designPlanStatus: "not_uploaded" })), "channels");

assertEqual(detailTabFromSummary(summary({ recorderCount: 0, designPlanStatus: "completed" })), "design-plan");

assertEqual(
  storeDetailTabFromDetail(
    detail({
      recorderCount: 0,
      recorders: [
        {
          id: 1,
          storeId: 7,
          ezvizAccountId: 1,
          accountName: "华东",
          deviceCode: "K123",
          status: "online",
          effectiveChannelCount: 1,
          lastScannedAt: "",
          channels: [],
        },
      ],
    }),
  ),
  "channels",
);

const cache = createStoreDetailCache(1000);
const loaded = detail({ id: 7, updatedAt: "2026-06-26T08:00:00.000Z", recorderCount: 1 });
cache.set(loaded, ["channels"], 10_000);

assertEqual(cache.get(7, loaded.updatedAt, "channels", 10_500), loaded);
assertEqual(cache.get(7, loaded.updatedAt, "design-plan", 10_500), null);
assertEqual(cache.get(7, loaded.updatedAt, "channels", 11_100), null);
assertEqual(cache.get(7, "2026-06-26T08:01:00.000Z", "channels", 10_500), null);

const channelDetail = detail({
  recorders: [
    {
      id: 1,
      storeId: 7,
      ezvizAccountId: 1,
      accountName: "华东",
      deviceCode: "K123",
      status: "online",
      effectiveChannelCount: 1,
      lastScannedAt: "",
      channels: [],
    },
  ],
});
const designPlanDetail = detail({
  fileName: "plan.pdf",
  previewUrl: "/plan.png",
  areas: [{ id: "area-1", name: "治疗室 1", type: "treatment", number: "1", confidence: "high", needsReview: false, box: null }],
});
const mergedWithChannels = mergeStoreDetailTab(makePendingStoreDetail(baseSummary), channelDetail, "channels");
const mergedWithDesignPlan = mergeStoreDetailTab(mergedWithChannels, designPlanDetail, "design-plan");
assertEqual(mergedWithDesignPlan.recorders.length, 1);
assertEqual(mergedWithDesignPlan.fileName, "plan.pdf");
assertEqual(mergedWithDesignPlan.areas.length, 1);

const summaryWithCounts = summary({
  recorderCount: 2,
  channelCount: 24,
  areaCount: 13,
  treatmentCount: 6,
  consultationCount: 4,
  beautyCount: 3,
});
const channelOnlyDetail = detail({
  recorderCount: 2,
  channelCount: 24,
  areaCount: 0,
  treatmentCount: 6,
  consultationCount: 4,
  beautyCount: 3,
  recorders: channelDetail.recorders,
});
const designPlanOnlyDetail = detail({
  recorderCount: 0,
  channelCount: 0,
  areaCount: 13,
  treatmentCount: 0,
  consultationCount: 0,
  beautyCount: 0,
  fileName: "plan.pdf",
  previewUrl: "/plan.png",
  areas: designPlanDetail.areas,
});
const mergedChannelOnly = mergeStoreDetailTab(makePendingStoreDetail(summaryWithCounts), channelOnlyDetail, "channels");
assertEqual(mergedChannelOnly.recorderCount, 2);
assertEqual(mergedChannelOnly.channelCount, 24);
assertEqual(mergedChannelOnly.areaCount, 13);
const mergedBothTabs = mergeStoreDetailTab(mergedChannelOnly, designPlanOnlyDetail, "design-plan");
assertEqual(mergedBothTabs.recorderCount, 2);
assertEqual(mergedBothTabs.channelCount, 24);
assertEqual(mergedBothTabs.areaCount, 13);
assertEqual(mergedBothTabs.treatmentCount, 6);
assertEqual(mergedBothTabs.consultationCount, 4);
assertEqual(mergedBothTabs.beautyCount, 3);

console.log("store-detail-navigation tests passed");

function assertEqual(actual: unknown, expected: unknown) {
  if (actual !== expected) {
    throw new Error(`expected ${String(expected)}, got ${String(actual)}`);
  }
}
