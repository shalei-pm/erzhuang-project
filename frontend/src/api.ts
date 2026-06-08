import sampleStoreFloorPlanUrl from "../../testdata/design-plans/generated/sample-store-floor-plan.png";

export type AreaType = "treatment" | "consultation" | "beauty";

export type Confidence = "high" | "medium" | "low";
export type StoreStatus = "completed" | "needs_review" | "incomplete";

export type AreaBox = {
  x: number;
  y: number;
  width: number;
  height: number;
};

export type StoreArea = {
  id: string;
  name: string;
  type: AreaType | "";
  number: string;
  confidence: Confidence;
  needsReview: boolean;
  box: AreaBox | null;
};

export type StoreSummary = {
  id: number;
  name: string;
  thumbnailUrl: string;
  treatmentCount: number;
  consultationCount: number;
  beautyCount: number;
  areaCount: number;
  status: StoreStatus;
  updatedAt: string;
};

export type StoreDetail = StoreSummary & {
  fileName: string;
  originalPath: string;
  previewPath: string;
  thumbnailPath: string;
  pageCount: number;
  previewUrl: string;
  areas: StoreArea[];
  recognitionResult?: unknown;
};

export type StoreListResponse = {
  items: StoreSummary[];
  page: number;
  pageSize: number;
  total: number;
};

export type UploadResult = {
  uploadId: string;
  fileName: string;
  pageCount: number;
  originalPath: string;
  previewPath: string;
  thumbnailPath: string;
  previewUrl: string;
  thumbnailUrl: string;
};

export type RecognitionResult = {
  storeName: string;
  storeNameConfidence: Confidence;
  areas: StoreArea[];
  rawNotes: string;
  rawResult?: unknown;
};

export type SaveStorePayload = {
  id?: number;
  name: string;
  fileName: string;
  originalPath?: string;
  previewPath?: string;
  thumbnailPath?: string;
  pageCount?: number;
  previewUrl: string;
  thumbnailUrl: string;
  uploadId?: string;
  recognitionResult?: unknown;
  areas: StoreArea[];
};

type ApiMode = "auto" | "http" | "mock";

type BackendStoreListResponse = {
  items: BackendStoreSummary[];
  page: number;
  page_size: number;
  total: number;
};

type BackendStoreSummary = {
  id: number;
  name: string;
  thumbnail_url?: string;
  treatment_count: number;
  consultation_count: number;
  beauty_count: number;
  area_count: number;
  status: StoreStatus;
  updated_at: string;
};

type BackendStoreDetail = {
  id: number;
  name: string;
  pdf_file_name?: string;
  original_pdf_path?: string;
  preview_image_path?: string;
  thumbnail_path?: string;
  preview_url?: string;
  thumbnail_url?: string;
  page_count?: number;
  status: StoreStatus;
  areas?: BackendStoreArea[];
  recognition_result?: unknown;
  updated_at: string;
};

type BackendUploadResult = {
  upload_id: string;
  file_name: string;
  page_count: number;
  original_pdf_path: string;
  preview_image_path: string;
  thumbnail_path: string;
  preview_url: string;
  thumbnail_url: string;
};

type BackendRecognitionResult = {
  store_name: string;
  store_name_confidence?: Confidence;
  areas?: BackendStoreArea[];
  raw_notes?: string;
  raw_result?: unknown;
};

type BackendStoreArea = {
  id?: number;
  name: string;
  type: AreaType;
  number?: string | number | null;
  confidence?: Confidence;
  needs_review?: boolean;
  box?: AreaBox;
  display_order?: number;
};

type BackendStorePayload = {
  name: string;
  pdf_file_name: string;
  original_pdf_path: string;
  preview_image_path: string;
  thumbnail_path: string;
  page_count: number;
  status?: StoreStatus;
  recognition_result?: unknown;
  areas: BackendStoreArea[];
};

type BackendDuplicateMatch = {
  id: number;
  name: string;
  reason?: string;
  thumbnail_url?: string;
  treatment_count?: number;
  consultation_count?: number;
  beauty_count?: number;
  area_count?: number;
  status?: StoreStatus;
  updated_at?: string;
};

type DuplicateCheckResult = {
  exactMatch: StoreSummary | null;
  similarMatches: StoreSummary[];
};

class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

const DEFAULT_API_BASE = "/erzhuang/api/design-plan";
const API_BASE = trimTrailingSlash(import.meta.env.VITE_DESIGN_PLAN_API_BASE || DEFAULT_API_BASE);
const API_MODE = normalizeApiMode(import.meta.env.VITE_DESIGN_PLAN_API_MODE);
const MOCK_PLAN_IMAGE = sampleStoreFloorPlanUrl;
const MOCK_ORIGINAL_PDF_PATH = "mock/uploads/sample-store-floor-plan.pdf";
const MOCK_PREVIEW_IMAGE_PATH = "mock/generated/sample-store-floor-plan.png";
const MOCK_THUMBNAIL_PATH = "mock/generated/sample-store-floor-plan.png";
const PAGE_SIZE = 20;

let warnedFallback = false;
let nextStoreId = 38;
const mockUploads = new Map<string, string>();

let mockStores: StoreDetail[] = [
  createMockStore(1, "杭州西湖旗舰店", "completed", [
    area("a-1", "治疗室 1", "treatment", "1", "high", { x: 0.315, y: 0.195, width: 0.095, height: 0.065 }),
    area("a-2", "治疗室 2", "treatment", "2", "high", { x: 0.315, y: 0.295, width: 0.095, height: 0.07 }),
    area("a-3", "面诊室 1", "consultation", "1", "high", { x: 0.185, y: 0.5, width: 0.09, height: 0.12 }),
    area("a-4", "生美区", "beauty", "", "medium", { x: 0.23, y: 0.685, width: 0.12, height: 0.12 }),
  ]),
  createMockStore(2, "上海静安中心店", "needs_review", [
    area("b-1", "治疗室 1", "treatment", "1", "high", { x: 0.315, y: 0.39, width: 0.095, height: 0.075 }),
    area("b-2", "面诊室 1", "consultation", "1", "low", { x: 0.185, y: 0.68, width: 0.09, height: 0.105 }),
    area("b-3", "面诊室 2", "consultation", "2", "high", { x: 0.345, y: 0.675, width: 0.065, height: 0.1 }),
  ]),
  createMockStore(3, "南京新街口店", "completed", [
    area("c-1", "治疗室 1", "treatment", "1", "high", { x: 0.315, y: 0.295, width: 0.095, height: 0.07 }),
    area("c-2", "生美 1", "beauty", "1", "high", { x: 0.345, y: 0.675, width: 0.065, height: 0.1 }),
  ]),
  ...Array.from({ length: 31 }, (_, index) => {
    const id = index + 4;
    const status: StoreStatus = index % 7 === 0 ? "needs_review" : "completed";
    return createMockStore(id, `演示门店 ${String(id).padStart(2, "0")}`, status, [
      area(`m-${id}-1`, "治疗室 1", "treatment", "1", "high", { x: 0.315, y: 0.195, width: 0.095, height: 0.065 }),
      area(`m-${id}-2`, "面诊室 1", "consultation", "1", status === "needs_review" ? "low" : "high", {
        x: 0.185,
        y: 0.5,
        width: 0.09,
        height: 0.12,
      }),
      ...(index % 3 === 0
        ? [area(`m-${id}-3`, "生美", "beauty", "", "high", { x: 0.23, y: 0.685, width: 0.12, height: 0.12 })]
        : []),
    ]);
  }),
];

const mockAdapter = {
  async listStores(query: string, page: number, pageSize = PAGE_SIZE): Promise<StoreListResponse> {
    await delay(160);
    const normalizedQuery = normalizeName(query);
    const filtered = mockStores
      .filter((store) => normalizeName(store.name).includes(normalizedQuery))
      .sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
    const start = (page - 1) * pageSize;

    return {
      items: filtered.slice(start, start + pageSize).map(toSummary),
      page,
      pageSize,
      total: filtered.length,
    };
  },

  async getStore(id: number): Promise<StoreDetail> {
    await delay(120);
    const store = mockStores.find((item) => item.id === id);
    if (!store) {
      throw new Error("门店不存在");
    }
    return clone(store);
  },

  async uploadPdf(fileName = "mock-design-plan.pdf"): Promise<UploadResult> {
    await delay(420);
    const uploadId = `tmp_${Date.now()}`;
    mockUploads.set(uploadId, fileName);
    return {
      uploadId,
      fileName,
      pageCount: 1,
      originalPath: MOCK_ORIGINAL_PDF_PATH,
      previewPath: MOCK_PREVIEW_IMAGE_PATH,
      thumbnailPath: MOCK_THUMBNAIL_PATH,
      previewUrl: MOCK_PLAN_IMAGE,
      thumbnailUrl: MOCK_PLAN_IMAGE,
    };
  },

  async recognizeUpload(uploadId: string): Promise<RecognitionResult> {
    await delay(620);
    const fileName = mockUploads.get(uploadId) ?? "";
    return {
      storeName: inferMockStoreName(fileName),
      storeNameConfidence: "medium",
      rawNotes: `mock recognition result for ${uploadId}`,
      areas: [
        area(`new-${Date.now()}-1`, "治疗室 1", "treatment", "1", "high", {
          x: 0.315,
          y: 0.195,
          width: 0.095,
          height: 0.065,
        }),
        area(`new-${Date.now()}-2`, "治疗室 2", "treatment", "2", "high", {
          x: 0.315,
          y: 0.295,
          width: 0.095,
          height: 0.07,
        }),
        area(`new-${Date.now()}-3`, "面诊室 1", "consultation", "1", "low", {
          x: 0.185,
          y: 0.5,
          width: 0.09,
          height: 0.12,
        }),
        area(`new-${Date.now()}-4`, "生美区", "beauty", "", "medium", {
          x: 0.23,
          y: 0.685,
          width: 0.12,
          height: 0.12,
        }),
      ],
    };
  },

  async checkDuplicate(name: string, excludeStoreId?: number): Promise<DuplicateCheckResult> {
    await delay(120);
    const normalized = normalizeName(name);
    const exactMatch = mockStores.find(
      (store) => store.id !== excludeStoreId && normalizeName(store.name) === normalized,
    );
    const similarMatches = mockStores
      .filter((store) => store.id !== excludeStoreId)
      .filter((store) => {
        const storeName = normalizeName(store.name);
        return !exactMatch && normalized.length > 1 && (storeName.includes(normalized) || normalized.includes(storeName));
      })
      .slice(0, 3)
      .map(toSummary);

    return {
      exactMatch: exactMatch ? toSummary(exactMatch) : null,
      similarMatches,
    };
  },

  async saveStore(payload: SaveStorePayload): Promise<StoreDetail> {
    await delay(260);
    const now = new Date().toISOString();
    const detail = buildDetailFromPayload(payload, now);

    if (payload.id) {
      mockStores = mockStores.map((store) => (store.id === payload.id ? detail : store));
      return clone(detail);
    }

    mockStores = [detail, ...mockStores];
    return clone(detail);
  },

  async deleteStore(id: number): Promise<void> {
    await delay(180);
    mockStores = mockStores.filter((store) => store.id !== id);
  },
};

const httpAdapter = {
  async listStores(query: string, page: number, pageSize = PAGE_SIZE): Promise<StoreListResponse> {
    const search = new URLSearchParams({
      q: query,
      page: String(page),
      page_size: String(pageSize),
    });
    const response = await requestJSON<BackendStoreListResponse>(`${API_BASE}/stores?${search.toString()}`);
    return {
      items: response.items.map(mapBackendSummary),
      page: response.page,
      pageSize: response.page_size,
      total: response.total,
    };
  },

  async getStore(id: number): Promise<StoreDetail> {
    const response = await requestJSON<BackendStoreDetail>(`${API_BASE}/stores/${id}`);
    return mapBackendDetail(response);
  },

  async uploadPdf(file: File): Promise<UploadResult> {
    const body = new FormData();
    body.append("file", file);
    const response = await requestJSON<BackendUploadResult>(`${API_BASE}/uploads`, {
      method: "POST",
      body,
    });
    return mapBackendUpload(response);
  },

  async recognizeUpload(uploadId: string): Promise<RecognitionResult> {
    const response = await requestJSON<BackendRecognitionResult>(`${API_BASE}/uploads/${uploadId}/recognize`, {
      method: "POST",
    });
    return mapBackendRecognition(response);
  },

  async checkDuplicate(name: string, excludeStoreId?: number): Promise<DuplicateCheckResult> {
    const response = await requestJSON<{
      exact_match: BackendDuplicateMatch | null;
      similar_matches: BackendDuplicateMatch[] | null;
    }>(`${API_BASE}/stores/check-duplicate`, {
      method: "POST",
      body: JSON.stringify({
        name,
        exclude_store_id: excludeStoreId,
      }),
    });

    return {
      exactMatch: response.exact_match ? duplicateMatchToSummary(response.exact_match) : null,
      similarMatches: (response.similar_matches ?? []).map(duplicateMatchToSummary),
    };
  },

  async saveStore(payload: SaveStorePayload): Promise<StoreDetail> {
    const body = JSON.stringify(toBackendPayload(payload));
    const response = await requestJSON<BackendStoreDetail>(payload.id ? `${API_BASE}/stores/${payload.id}` : `${API_BASE}/stores`, {
      method: payload.id ? "PUT" : "POST",
      body,
    });
    return mapBackendDetail(response);
  },

  async deleteStore(id: number): Promise<void> {
    await requestJSON<void>(`${API_BASE}/stores/${id}`, { method: "DELETE" });
  },
};

export const designPlanApi = {
  endpoints: {
    base: API_BASE,
    stores: `${API_BASE}/stores`,
    uploads: `${API_BASE}/uploads`,
    recognize: (uploadId: string) => `${API_BASE}/uploads/${uploadId}/recognize`,
    checkDuplicate: `${API_BASE}/stores/check-duplicate`,
  },

  async listStores(query: string, page: number, pageSize = PAGE_SIZE): Promise<StoreListResponse> {
    return withFallback(() => httpAdapter.listStores(query, page, pageSize), () => mockAdapter.listStores(query, page, pageSize));
  },

  async getStore(id: number): Promise<StoreDetail> {
    return withFallback(() => httpAdapter.getStore(id), () => mockAdapter.getStore(id));
  },

  async uploadPdf(file: File | string = "mock-design-plan.pdf"): Promise<UploadResult> {
    if (API_MODE === "mock" || typeof file === "string") {
      return mockAdapter.uploadPdf(typeof file === "string" ? file : file.name);
    }
    return httpAdapter.uploadPdf(file);
  },

  async recognizeUpload(uploadId: string): Promise<RecognitionResult> {
    if (API_MODE === "mock") {
      return mockAdapter.recognizeUpload(uploadId);
    }
    return httpAdapter.recognizeUpload(uploadId);
  },

  async checkDuplicate(name: string, excludeStoreId?: number): Promise<DuplicateCheckResult> {
    return withFallback(
      () => httpAdapter.checkDuplicate(name, excludeStoreId),
      () => mockAdapter.checkDuplicate(name, excludeStoreId),
    );
  },

  async saveStore(payload: SaveStorePayload): Promise<StoreDetail> {
    return withFallback(() => httpAdapter.saveStore(payload), () => mockAdapter.saveStore(payload));
  },

  async deleteStore(id: number): Promise<void> {
    return withFallback(() => httpAdapter.deleteStore(id), () => mockAdapter.deleteStore(id));
  },
};

async function withFallback<T>(httpCall: () => Promise<T>, mockCall: () => Promise<T>): Promise<T> {
  if (API_MODE === "mock") {
    return mockCall();
  }

  try {
    return await httpCall();
  } catch (error) {
    if (API_MODE === "http") {
      throw error;
    }
    if (!shouldFallback(error)) {
      throw error;
    }
    warnFallback(error);
    return mockCall();
  }
}

async function requestJSON<T>(url: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(url, {
    headers: {
      Accept: "application/json",
      ...(options.body && !(options.body instanceof FormData) ? { "Content-Type": "application/json" } : {}),
      ...options.headers,
    },
    ...options,
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const contentType = response.headers.get("content-type") ?? "";
  const data = contentType.includes("application/json") ? await response.json() : await response.text();

  if (!response.ok) {
    const message =
      typeof data === "object" && data && "message" in data
        ? String(data.message)
        : typeof data === "object" && data && "error" in data
          ? String(data.error)
          : `HTTP ${response.status}`;
    throw new ApiError(response.status, message);
  }

  return data as T;
}

function mapBackendSummary(item: BackendStoreSummary): StoreSummary {
  return {
    id: item.id,
    name: item.name,
    thumbnailUrl: toDisplayImageUrl(item.thumbnail_url),
    treatmentCount: item.treatment_count,
    consultationCount: item.consultation_count,
    beautyCount: item.beauty_count,
    areaCount: item.area_count,
    status: item.status,
    updatedAt: item.updated_at,
  };
}

function mapBackendDetail(store: BackendStoreDetail): StoreDetail {
  const areas = (store.areas ?? [])
    .slice()
    .sort((left, right) => (left.display_order ?? 0) - (right.display_order ?? 0))
    .map(mapBackendArea);
  const counts = countAreas(areas);

  return {
    id: store.id,
    name: store.name,
    fileName: displayPlanFileName(store.pdf_file_name, store.original_pdf_path, store.name),
    originalPath: store.original_pdf_path || MOCK_ORIGINAL_PDF_PATH,
    previewPath: store.preview_image_path || MOCK_PREVIEW_IMAGE_PATH,
    thumbnailPath: store.thumbnail_path || MOCK_THUMBNAIL_PATH,
    pageCount: store.page_count || 1,
    thumbnailUrl: toDisplayImageUrl(store.thumbnail_url || store.thumbnail_path),
    previewUrl: toDisplayImageUrl(store.preview_url || store.preview_image_path),
    treatmentCount: counts.treatment,
    consultationCount: counts.consultation,
    beautyCount: counts.beauty,
    areaCount: areas.length,
    status: store.status,
    updatedAt: store.updated_at,
    recognitionResult: store.recognition_result,
    areas,
  };
}

function mapBackendUpload(upload: BackendUploadResult): UploadResult {
  return {
    uploadId: upload.upload_id,
    fileName: upload.file_name,
    pageCount: upload.page_count,
    originalPath: upload.original_pdf_path,
    previewPath: upload.preview_image_path,
    thumbnailPath: upload.thumbnail_path,
    previewUrl: toDisplayImageUrl(upload.preview_url),
    thumbnailUrl: toDisplayImageUrl(upload.thumbnail_url),
  };
}

function mapBackendRecognition(result: BackendRecognitionResult): RecognitionResult {
  return {
    storeName: result.store_name || "",
    storeNameConfidence: result.store_name_confidence || "medium",
    areas: (result.areas ?? []).map(mapBackendArea),
    rawNotes: result.raw_notes || "",
    rawResult: result.raw_result,
  };
}

function mapBackendArea(areaItem: BackendStoreArea): StoreArea {
  const confidence = areaItem.confidence || "high";
  return {
    id: areaItem.id ? String(areaItem.id) : `area-${areaItem.display_order ?? Date.now()}`,
    name: areaItem.name,
    type: areaItem.type,
    number: areaItem.number == null ? "" : String(areaItem.number),
    confidence,
    needsReview: Boolean(areaItem.needs_review) || confidence === "low",
    box: areaItem.box ?? null,
  };
}

function duplicateMatchToSummary(match: BackendDuplicateMatch): StoreSummary {
  return {
    id: match.id,
    name: match.name,
    thumbnailUrl: toDisplayImageUrl(match.thumbnail_url),
    treatmentCount: match.treatment_count ?? 0,
    consultationCount: match.consultation_count ?? 0,
    beautyCount: match.beauty_count ?? 0,
    areaCount: match.area_count ?? 0,
    status: match.status ?? "needs_review",
    updatedAt: match.updated_at ?? new Date().toISOString(),
  };
}

function toBackendPayload(payload: SaveStorePayload): BackendStorePayload {
  return {
    name: payload.name,
    pdf_file_name: normalizePlanFileName(payload.fileName, payload.name),
    original_pdf_path: payload.originalPath || payload.fileName || MOCK_ORIGINAL_PDF_PATH,
    preview_image_path: payload.previewPath || toStoredPath(payload.previewUrl, MOCK_PREVIEW_IMAGE_PATH),
    thumbnail_path: payload.thumbnailPath || toStoredPath(payload.thumbnailUrl, MOCK_THUMBNAIL_PATH),
    page_count: payload.pageCount || 1,
    recognition_result: payload.recognitionResult,
    areas: payload.areas.map((areaItem, index) => ({
      id: numericId(areaItem.id),
      name: areaItem.name,
      type: areaItem.type as AreaType,
      number: areaItem.number || undefined,
      confidence: areaItem.confidence || "high",
      needs_review: areaItem.needsReview || areaItem.confidence === "low",
      box: areaItem.box ?? undefined,
      display_order: index + 1,
    })),
  };
}

function toDisplayImageUrl(value?: string) {
  if (!value || value.startsWith("mock/")) {
    return MOCK_PLAN_IMAGE;
  }
  if (/^\/api\/design-plan\/uploads\/[^/]+\/(preview|thumbnail)$/.test(value)) {
    return `/erzhuang${value}`;
  }
  if (/^\/api\/design-plan\/stores\/\d+\/(preview|thumbnail)$/.test(value)) {
    return `/erzhuang${value}`;
  }
  if (value.startsWith("/api/")) {
    return `/erzhuang${value}`;
  }
  return value;
}

function toStoredPath(value: string, fallback: string) {
  if (!value || value === MOCK_PLAN_IMAGE || value.startsWith("data:") || value.startsWith("blob:")) {
    return fallback;
  }
  const uploadMatch = value.match(/^\/(?:erzhuang\/)?api\/design-plan\/uploads\/([^/]+)\/(preview|thumbnail)$/);
  if (uploadMatch) {
    return `uploads/${uploadMatch[1]}/${uploadMatch[2] === "preview" ? "preview.png" : "thumbnail.png"}`;
  }
  return value;
}

function displayFileName(value: string) {
  const parts = value.split(/[\\/]/);
  return parts[parts.length - 1] || value;
}

function displayPlanFileName(fileName: string | undefined, originalPath: string | undefined, storeName: string) {
  const candidate = (fileName || displayFileName(originalPath || "")).trim();
  if (candidate && candidate !== "original.pdf") {
    return candidate;
  }
  return normalizePlanFileName("", storeName);
}

function normalizePlanFileName(fileName: string | undefined, storeName: string) {
  const candidate = (fileName || "").trim();
  if (candidate && candidate !== "original.pdf") {
    return candidate;
  }
  const safeName = storeName
    .trim()
    .replace(/\s+/g, "-")
    .replace(/[\\/:*?"<>|]/g, "")
    .replace(/^-+|-+$/g, "");
  return `${safeName || "store"}-design-plan.pdf`;
}

function numericId(value: string) {
  if (!/^\d+$/.test(value)) {
    return undefined;
  }
  return Number(value);
}

function inferMockStoreName(fileName: string) {
  const baseName = fileName
    .replace(/\.[^.]+$/, "")
    .replace(/[-_ ]?(设计图|装修图|平面图|floor[\s_-]?plan|design[\s_-]?plan)$/i, "")
    .trim();
  return baseName || "成都太古里体验店";
}

function createMockStore(id: number, name: string, status: StoreStatus, areas: StoreArea[]): StoreDetail {
  const now = new Date(Date.now() - id * 18 * 60 * 60 * 1000).toISOString();
  const counts = countAreas(areas);
  return {
    id,
    name,
    thumbnailUrl: MOCK_PLAN_IMAGE,
    previewUrl: MOCK_PLAN_IMAGE,
    originalPath: MOCK_ORIGINAL_PDF_PATH,
    previewPath: MOCK_PREVIEW_IMAGE_PATH,
    thumbnailPath: MOCK_THUMBNAIL_PATH,
    pageCount: 1,
    fileName: `${name}-design.pdf`,
    status,
    updatedAt: now,
    areaCount: areas.length,
    treatmentCount: counts.treatment,
    consultationCount: counts.consultation,
    beautyCount: counts.beauty,
    areas,
  };
}

function buildDetailFromPayload(payload: SaveStorePayload, updatedAt: string): StoreDetail {
  const areas = payload.areas.map((item) => ({ ...item, needsReview: item.confidence === "low" || item.needsReview }));
  const counts = countAreas(areas);
  const status: StoreStatus = areas.some((item) => item.needsReview || item.confidence === "low")
    ? "needs_review"
    : "completed";

  return {
    id: payload.id ?? nextStoreId++,
    name: payload.name.trim(),
    fileName: payload.fileName,
    originalPath: payload.originalPath || MOCK_ORIGINAL_PDF_PATH,
    previewPath: payload.previewPath || MOCK_PREVIEW_IMAGE_PATH,
    thumbnailPath: payload.thumbnailPath || MOCK_THUMBNAIL_PATH,
    pageCount: payload.pageCount || 1,
    thumbnailUrl: payload.thumbnailUrl,
    previewUrl: payload.previewUrl,
    treatmentCount: counts.treatment,
    consultationCount: counts.consultation,
    beautyCount: counts.beauty,
    areaCount: areas.length,
    status,
    updatedAt,
    areas,
  };
}

function area(
  id: string,
  name: string,
  type: AreaType,
  number: string,
  confidence: Confidence,
  box: AreaBox,
): StoreArea {
  return {
    id,
    name,
    type,
    number,
    confidence,
    needsReview: confidence === "low",
    box,
  };
}

function countAreas(areas: StoreArea[]) {
  return areas.reduce(
    (counts, item) => {
      if (item.type === "treatment") counts.treatment += 1;
      if (item.type === "consultation") counts.consultation += 1;
      if (item.type === "beauty") counts.beauty += 1;
      return counts;
    },
    { treatment: 0, consultation: 0, beauty: 0 },
  );
}

function toSummary(store: StoreDetail): StoreSummary {
  const { areas: _areas, fileName: _fileName, previewUrl: _previewUrl, ...summary } = store;
  return { ...summary };
}

function normalizeApiMode(value: string | undefined): ApiMode {
  if (value === "http" || value === "mock") {
    return value;
  }
  return "auto";
}

function normalizeName(value: string) {
  return value.trim().replace(/\s+/g, " ").toLocaleLowerCase();
}

function trimTrailingSlash(value: string) {
  return value.replace(/\/+$/, "");
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function delay(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

function warnFallback(error: unknown) {
  if (warnedFallback) {
    return;
  }
  warnedFallback = true;
  console.info("[designPlanApi] backend unavailable, falling back to mock adapter", error);
}

function shouldFallback(error: unknown) {
  if (error instanceof ApiError) {
    return error.status === 404 || error.status >= 500;
  }
  return true;
}
