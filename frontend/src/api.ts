import sampleStoreFloorPlanUrl from "../../testdata/design-plans/generated/sample-store-floor-plan.png";

export type AreaType = "treatment" | "consultation" | "beauty";

export type Confidence = "high" | "medium" | "low";
export type StoreStatus = "completed" | "needs_review" | "incomplete";
export type DesignPlanStatus = "not_uploaded" | "pending_recognition" | "pending_annotation" | "completed";
export type RecorderStatus = "online" | "offline";
export type ChannelStatus =
  | "pending_recognition"
  | "pending_confirmation"
  | "confirmed_business"
  | "confirmed_non_business"
  | "recognition_failed"
  | "inactive";
export type NonBusinessSceneType =
  | "front_desk"
  | "corridor"
  | "passage"
  | "waiting_area"
  | "hall"
  | "entrance"
  | "storage"
  | "pharmacy"
  | "machine_room"
  | "unknown";
export type SceneType = AreaType | NonBusinessSceneType;

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
  source?: "manual" | "design_plan" | "video_channel" | "multiple";
  confidence: Confidence;
  needsReview: boolean;
  box: AreaBox | null;
};

export type StoreSummary = {
  id: number;
  city: string;
  name: string;
  externalOrgId: string;
  thumbnailUrl: string;
  designPlanStatus: DesignPlanStatus;
  recorderCount: number;
  channelCount: number;
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
  recorders: VideoRecorder[];
  recognitionResult?: unknown;
};

export type EzvizAccount = {
  id: number;
  accountName: string;
  status: "unverified" | "available" | "unavailable";
  lastVerifiedAt: string;
};

export type CreateEzvizAccountPayload = {
  accountName: string;
};

export type RecorderDraft = {
  id: string;
  ezvizAccountId: number | "";
  deviceCode: string;
};

export type VideoRecorder = {
  id: number;
  storeId: number;
  ezvizAccountId: number;
  accountName: string;
  deviceCode: string;
  status: RecorderStatus;
  effectiveChannelCount: number;
  lastScannedAt: string;
  recognitionProgress?: string;
  channels: VideoChannel[];
};

export type VideoChannel = {
  id: number;
  recorderId: number;
  recorderCode: string;
  channelNo: number;
  channelName: string;
  status: ChannelStatus;
  thumbnailUrl: string;
  fullImageUrl: string;
  fullImageExpiresAt?: string;
  sceneType: SceneType;
  areaType: AreaType | "";
  areaNumber: string;
  areaNote: string;
  recognitionAttempts: number;
  recognitionResult?: unknown;
  confirmedAt?: string;
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
  city?: string;
  name: string;
  externalOrgId?: string;
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
  recorders?: VideoRecorder[];
};

export type CreateStoreSpacePayload = {
  city: string;
  name: string;
  externalOrgId: string;
  designPlan?: UploadResult | null;
  recorders: RecorderDraft[];
};

export type AddRecorderPayload = {
  ezvizAccountId: number | "";
  deviceCode: string;
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
  city?: string;
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
  city?: string;
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
  city?: string;
  name: string;
  reason?: string;
  thumbnail_url?: string;
  treatment_count?: number;
  consultation_count?: number;
  beauty_count?: number;
  area_count?: number;
  status?: StoreStatus;
  overall_status?: string;
  updated_at?: string;
};

type BackendEzvizAccount = {
  id: number;
  account_name?: string;
  accountName?: string;
  status?: EzvizAccount["status"];
  last_verified_at?: string;
  lastVerifiedAt?: string;
};

type BackendVideoRecorder = {
  id: number;
  store_id?: number;
  storeId?: number;
  ezviz_account_id?: number;
  ezvizAccountId?: number;
  account_name?: string;
  accountName?: string;
  device_code?: string;
  deviceCode?: string;
  status?: RecorderStatus;
  effective_channel_count?: number;
  effectiveChannelCount?: number;
  last_scanned_at?: string;
  lastScannedAt?: string;
  recognition_progress?: string;
  recognitionProgress?: string;
  channels?: BackendVideoChannel[];
};

type BackendVideoChannel = {
  id: number;
  recorder_id?: number;
  recorderId?: number;
  recorder_code?: string;
  recorderCode?: string;
  channel_no?: number;
  channelNo?: number;
  channel_name?: string;
  channelName?: string;
  status?: ChannelStatus;
  thumbnail_url?: string;
  thumbnailUrl?: string;
  full_image_url?: string;
  fullImageUrl?: string;
  full_image_expires_at?: string;
  fullImageExpiresAt?: string;
  scene_type?: SceneType;
  sceneType?: SceneType;
  area_type?: AreaType | "";
  areaType?: AreaType | "";
  area_number?: number | string | null;
  areaNumber?: number | string | null;
  area_note?: string;
  areaNote?: string;
  recognition_attempts?: number;
  recognitionAttempts?: number;
  recognition_result?: unknown;
  recognitionResult?: unknown;
  confirmed_at?: string;
  confirmedAt?: string;
};

type BackendStoreSpaceListResponse = {
  items: BackendStoreSpaceSummary[];
  page: number;
  page_size?: number;
  pageSize?: number;
  total: number;
};

type BackendStoreSpaceSummary = {
  id: number;
  city?: string;
  cityName?: string;
  name: string;
  external_org_id?: string;
  externalOrgId?: string;
  design_plan_status?: DesignPlanStatus;
  designPlanStatus?: DesignPlanStatus;
  overall_status?: string;
  overallStatus?: string;
  recorder_count?: number;
  recorderCount?: number;
  channel_count?: number;
  channelCount?: number;
  treatment_count?: number;
  treatmentCount?: number;
  consultation_count?: number;
  consultationCount?: number;
  beauty_count?: number;
  beautyCount?: number;
  area_count?: number;
  areaCount?: number;
  updated_at?: string;
  updatedAt?: string;
};

type BackendStoreSpaceDetail = BackendStoreSpaceSummary & {
  design_plans?: BackendStoreSpaceDesignPlan[];
  designPlans?: BackendStoreSpaceDesignPlan[];
  areas?: BackendStoreSpaceArea[];
  recorders?: BackendStoreSpaceRecorder[];
};

type BackendStoreSpaceDesignPlan = {
  id?: number;
  pdf_file_name?: string;
  pdfFileName?: string;
  original_pdf_path?: string;
  originalPdfPath?: string;
  preview_image_path?: string;
  previewImagePath?: string;
  thumbnail_path?: string;
  thumbnailPath?: string;
  preview_url?: string;
  previewUrl?: string;
  thumbnail_url?: string;
  thumbnailUrl?: string;
  page_count?: number;
  pageCount?: number;
  recognition_result?: unknown;
  recognitionResult?: unknown;
};

type BackendStoreSpaceArea = {
  id?: number;
  display_name?: string;
  displayName?: string;
  area_type?: AreaType;
  areaType?: AreaType;
  area_number?: number | string | null;
  areaNumber?: number | string | null;
  status?: "candidate" | "confirmed";
  source?: string;
  confidence?: Confidence;
  needs_review?: boolean;
  needsReview?: boolean;
  box?: AreaBox;
  annotation?: {
    box?: AreaBox;
    status?: "pending" | "confirmed";
  };
};

type BackendStoreSpaceDesignPlanPayload = {
  upload_id?: string;
  pdf_file_name?: string;
  original_pdf_path?: string;
  preview_image_path?: string;
  thumbnail_path?: string;
  page_count?: number;
  recognition_result?: string;
  areas: Array<{
    id?: number;
    display_name?: string;
    area_type: AreaType;
    area_number: string;
    confidence?: Confidence;
    needs_review?: boolean;
    box?: AreaBox;
  }>;
};

type BackendStoreSpaceRecorder = BackendVideoRecorder;

type DuplicateCheckResult = {
  exactMatch: StoreSummary | null;
  similarMatches: StoreSummary[];
};

export class ApiError extends Error {
  status: number;
  fields: Record<string, string>;

  constructor(status: number, message: string, fields: Record<string, string> = {}) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.fields = fields;
  }
}

const DEFAULT_API_BASE = "/erzhuang/api/design-plan";
const API_BASE = trimTrailingSlash(import.meta.env.VITE_DESIGN_PLAN_API_BASE || DEFAULT_API_BASE);
const DEFAULT_STORE_SPACE_API_BASE = "/erzhuang/api/store-space";
const STORE_SPACE_API_BASE = trimTrailingSlash(import.meta.env.VITE_STORE_SPACE_API_BASE || DEFAULT_STORE_SPACE_API_BASE);
const API_MODE = normalizeApiMode(import.meta.env.VITE_DESIGN_PLAN_API_MODE);
const MOCK_PLAN_IMAGE = sampleStoreFloorPlanUrl;
const MOCK_ORIGINAL_PDF_PATH = "mock/uploads/sample-store-floor-plan.pdf";
const MOCK_PREVIEW_IMAGE_PATH = "mock/generated/sample-store-floor-plan.png";
const MOCK_THUMBNAIL_PATH = "mock/generated/sample-store-floor-plan.png";
const PAGE_SIZE = 20;

let warnedFallback = false;
let nextStoreId = 38;
let nextRecorderId = 900;
let nextChannelId = 5000;
const mockUploads = new Map<string, string>();

let mockEzvizAccounts: EzvizAccount[] = [
  { id: 1, accountName: "华北", status: "available", lastVerifiedAt: "2026-06-10T10:30:00.000Z" },
  { id: 2, accountName: "华东", status: "available", lastVerifiedAt: "2026-06-10T10:30:00.000Z" },
  { id: 3, accountName: "华南", status: "available", lastVerifiedAt: "2026-06-10T10:30:00.000Z" },
  { id: 4, accountName: "华中", status: "available", lastVerifiedAt: "2026-06-10T10:30:00.000Z" },
];

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
    const filtered = mockStores
      .filter((store) => matchesStoreSearch(store.name, query))
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

  async deleteRecorder(storeId: number, recorderId: number): Promise<StoreDetail> {
    await delay(180);
    const store = mockStores.find((item) => item.id === storeId);
    if (!store) throw new Error("门店不存在");
    const recorders = store.recorders.filter((recorder) => recorder.id !== recorderId);
    const nextStore = {
      ...store,
      recorders,
      recorderCount: recorders.length,
      channelCount: countChannels(recorders),
      updatedAt: new Date().toISOString(),
    };
    mockStores = mockStores.map((item) => (item.id === storeId ? nextStore : item));
    return clone(nextStore);
  },

  async deleteChannel(storeId: number, channelId: number): Promise<StoreDetail> {
    await delay(160);
    const store = mockStores.find((item) => item.id === storeId);
    if (!store) throw new Error("门店不存在");
    const recorders = store.recorders.map((recorder) => {
      const channels = recorder.channels.filter((channel) => channel.id !== channelId);
      return {
        ...recorder,
        channels,
        effectiveChannelCount: channels.filter((channel) => channel.status !== "inactive").length,
      };
    });
    const nextStore = {
      ...store,
      recorders,
      channelCount: countChannels(recorders),
      areas: mergeAreasFromConfirmedChannels(
        store.areas.filter((areaItem) => !areaItem.id.startsWith(`channel-${channelId}`)),
        recorders,
      ),
      updatedAt: new Date().toISOString(),
    };
    mockStores = mockStores.map((item) => (item.id === storeId ? nextStore : item));
    return clone(nextStore);
  },

  async addRecorder(storeId: number, payload: AddRecorderPayload): Promise<StoreDetail> {
    await delay(180);
    const store = mockStores.find((item) => item.id === storeId);
    if (!store) throw new Error("门店不存在");
    const cleanCode = payload.deviceCode.trim().toUpperCase();
    if (!cleanCode) throw new Error("录像机设备编码必填");
    if (store.recorders.length >= 3) throw new Error("单门店最多 3 台录像机");
    const exists = mockStores.some((item) => item.recorders.some((recorder) => recorder.deviceCode === cleanCode));
    if (exists) throw new Error("录像机设备编码已存在");
    const recorders = [...store.recorders, createMockRecorder(storeId, cleanCode, Number(payload.ezvizAccountId) || 0)];
    const nextStore = {
      ...store,
      recorders,
      recorderCount: recorders.length,
      channelCount: countChannels(recorders),
      updatedAt: new Date().toISOString(),
    };
    mockStores = mockStores.map((item) => (item.id === storeId ? nextStore : item));
    return clone(nextStore);
  },

  async listEzvizAccounts(): Promise<EzvizAccount[]> {
    await delay(120);
    return clone(mockEzvizAccounts);
  },

  async createStoreSpace(payload: CreateStoreSpacePayload): Promise<StoreDetail> {
    await delay(260);
    const now = new Date().toISOString();
    const storeId = nextStoreId++;
    const recorders = payload.recorders
      .filter((item) => item.deviceCode.trim() && item.ezvizAccountId)
      .map((item) => createMockRecorder(storeId, item.deviceCode.trim(), Number(item.ezvizAccountId)));
    const detail = createMockStore(storeId, payload.name.trim(), "incomplete", []);
    detail.city = payload.city.trim();
    detail.externalOrgId = payload.externalOrgId.trim();
    detail.fileName = payload.designPlan?.fileName ?? "";
    detail.originalPath = payload.designPlan?.originalPath ?? "";
    detail.previewPath = payload.designPlan?.previewPath ?? "";
    detail.thumbnailPath = payload.designPlan?.thumbnailPath ?? "";
    detail.pageCount = payload.designPlan?.pageCount ?? 0;
    detail.previewUrl = payload.designPlan?.previewUrl ?? "";
    detail.thumbnailUrl = payload.designPlan?.thumbnailUrl ?? "";
    detail.designPlanStatus = payload.designPlan ? "pending_annotation" : "not_uploaded";
    detail.recorders = recorders;
    detail.recorderCount = recorders.length;
    detail.channelCount = countChannels(recorders);
    detail.updatedAt = now;
    mockStores = [detail, ...mockStores];
    return clone(detail);
  },

  async scanRecorder(storeId: number, recorderId: number): Promise<VideoRecorder> {
    await delay(360);
    const store = mockStores.find((item) => item.id === storeId);
    const recorder = store?.recorders.find((item) => item.id === recorderId);
    if (!store || !recorder) {
      throw new Error("录像机不存在");
    }
    const scanned = {
      ...recorder,
      status: "online" as RecorderStatus,
      lastScannedAt: new Date().toISOString(),
      channels: recorder.channels.length > 0 ? recorder.channels : createMockChannels(recorder.id, recorder.deviceCode),
    };
    scanned.effectiveChannelCount = scanned.channels.filter((item) => item.status !== "inactive").length;
    replaceMockRecorder(storeId, scanned);
    return clone(scanned);
  },

  async recognizeRecorder(storeId: number, recorderId: number): Promise<VideoRecorder> {
    await delay(520);
    const store = mockStores.find((item) => item.id === storeId);
    const recorder = store?.recorders.find((item) => item.id === recorderId);
    if (!store || !recorder) {
      throw new Error("录像机不存在");
    }
    const recognizedChannels = ensureRecorderChannels(recorder).map((channel, index) => {
      if (channel.status === "confirmed_business" || channel.status === "confirmed_non_business") return channel;
      const presets = [
        { sceneType: "consultation" as SceneType, areaType: "consultation" as AreaType, areaNumber: "1" },
        { sceneType: "treatment" as SceneType, areaType: "treatment" as AreaType, areaNumber: "2" },
        { sceneType: "front_desk" as SceneType, areaType: "" as const, areaNumber: "" },
        { sceneType: "corridor" as SceneType, areaType: "" as const, areaNumber: "" },
      ];
      const preset = presets[index % presets.length];
      return {
        ...channel,
        ...preset,
        status: "pending_confirmation" as ChannelStatus,
        thumbnailUrl: channel.thumbnailUrl || `https://picsum.photos/seed/${recorder.deviceCode}-${channel.channelNo}/240/160`,
        fullImageUrl: channel.fullImageUrl || `https://picsum.photos/seed/${recorder.deviceCode}-${channel.channelNo}/960/640`,
        fullImageExpiresAt: channel.fullImageExpiresAt || new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString(),
        recognitionResult: { status: "captured" },
        recognitionAttempts: channel.recognitionAttempts + 1,
      };
    });
    const recognized = {
      ...recorder,
      channels: recognizedChannels,
      effectiveChannelCount: recognizedChannels.filter((item) => item.status !== "inactive").length,
      recognitionProgress: `已完成 ${recognizedChannels.length}/${recognizedChannels.length}`,
    };
    replaceMockRecorder(storeId, recognized);
    return clone(recognized);
  },

  async recognizeChannel(storeId: number, channelId: number): Promise<VideoChannel> {
    await delay(520);
    const store = mockStores.find((item) => item.id === storeId);
    if (!store) throw new Error("门店不存在");
    const recorder = store.recorders.find((item) => item.channels.some((channel) => channel.id === channelId));
    const channel = recorder?.channels.find((item) => item.id === channelId);
    if (!recorder || !channel) throw new Error("通道不存在");
    if (channel.status === "inactive") throw new Error("通道已失效，无法识别");
    const index = Math.max(0, recorder.channels.findIndex((item) => item.id === channelId));
    const presets = [
      { sceneType: "consultation" as SceneType, areaType: "consultation" as AreaType, areaNumber: "1" },
      { sceneType: "treatment" as SceneType, areaType: "treatment" as AreaType, areaNumber: "2" },
      { sceneType: "beauty" as SceneType, areaType: "beauty" as AreaType, areaNumber: "3" },
      { sceneType: "corridor" as SceneType, areaType: "" as const, areaNumber: "" },
    ];
    const preset = presets[index % presets.length];
	  const recognized: VideoChannel = {
	    ...channel,
	    ...preset,
	    status: channel.status === "confirmed_business" || channel.status === "confirmed_non_business" ? channel.status : "pending_confirmation",
	    areaNote: preset.areaType ? "" : nonBusinessSceneLabel(preset.sceneType),
	    thumbnailUrl: `https://picsum.photos/seed/${recorder.deviceCode}-${channel.channelNo}-${Date.now()}/240/160`,
      fullImageUrl: `https://picsum.photos/seed/${recorder.deviceCode}-${channel.channelNo}-${Date.now()}/960/640`,
      fullImageExpiresAt: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString(),
      recognitionResult: {
        status: "recognized",
        area_type: preset.areaType,
	    area_number: preset.areaType ? preset.areaNumber : nonBusinessSceneLabel(preset.sceneType),
        confidence: "medium",
        capture_ms: 380,
        recognition_ms: 520,
        total_ms: 900,
      },
      recognitionAttempts: channel.recognitionAttempts + 1,
    };
    const nextRecorder = {
      ...recorder,
      channels: recorder.channels.map((item) => (item.id === channelId ? recognized : item)),
      effectiveChannelCount: recorder.channels.filter((item) => item.status !== "inactive").length,
    };
    replaceMockRecorder(storeId, nextRecorder);
    return clone(recognized);
  },

  async confirmChannel(storeId: number, channelId: number, patch: Partial<VideoChannel>): Promise<StoreDetail> {
    await delay(180);
    const store = mockStores.find((item) => item.id === storeId);
    if (!store) throw new Error("门店不存在");
    const now = new Date().toISOString();
    const nextRecorders: VideoRecorder[] = store.recorders.map((recorder) => ({
      ...recorder,
      channels: recorder.channels.map((channel) => {
        if (channel.id !== channelId) return channel;
        const isBusiness = Boolean(patch.areaType);
        const status: ChannelStatus = isBusiness ? "confirmed_business" : "confirmed_non_business";
        const sceneType: SceneType = patch.sceneType || (patch.areaType ? patch.areaType : "unknown");
	    return {
	      ...channel,
	      ...patch,
	      sceneType,
	      areaNote: isBusiness ? "" : String(patch.areaNote ?? patch.areaNumber ?? ""),
	      status,
	      confirmedAt: now,
	    };
      }),
    }));
    const nextStore = {
      ...store,
      recorders: nextRecorders,
      areas: mergeAreasFromConfirmedChannels(store.areas, nextRecorders),
      updatedAt: now,
    };
    const counts = countAreas(nextStore.areas);
    nextStore.treatmentCount = counts.treatment;
    nextStore.consultationCount = counts.consultation;
    nextStore.beautyCount = counts.beauty;
    nextStore.areaCount = nextStore.areas.length;
    mockStores = mockStores.map((item) => (item.id === storeId ? nextStore : item));
    return clone(nextStore);
  },

  async unlockChannelForEdit(storeId: number, channelId: number): Promise<VideoChannel> {
    await delay(160);
    const store = mockStores.find((item) => item.id === storeId);
    if (!store) throw new Error("门店不存在");
    const nextRecorders: VideoRecorder[] = store.recorders.map((recorder) => ({
      ...recorder,
      channels: recorder.channels.map((channel) =>
        channel.id === channelId
          ? {
              ...channel,
              status: "pending_confirmation" as ChannelStatus,
              confirmedAt: "",
            }
          : channel,
      ),
    }));
    let unlockedChannel: VideoChannel | null = null;
    const nextStore = {
      ...store,
      recorders: nextRecorders.map((recorder) => ({
        ...recorder,
        channels: recorder.channels.map((channel) => {
          if (channel.id === channelId) unlockedChannel = channel;
          return channel;
        }),
      })),
      updatedAt: new Date().toISOString(),
    };
    mockStores = mockStores.map((item) => (item.id === storeId ? nextStore : item));
    if (!unlockedChannel) throw new Error("通道不存在");
    return clone(unlockedChannel);
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

const storeSpaceHttpAdapter = {
  async createStore(payload: CreateStoreSpacePayload): Promise<StoreDetail> {
    const response = await requestJSON<BackendStoreSpaceDetail>(`${STORE_SPACE_API_BASE}/stores`, {
      method: "POST",
      body: JSON.stringify(toStoreSpaceCreatePayload(payload)),
    });
    return mapStoreSpaceDetail(response);
  },

  async checkDuplicate(name: string, excludeStoreId?: number): Promise<DuplicateCheckResult> {
    const response = await requestJSON<{
      exact_match: BackendDuplicateMatch | null;
      similar_matches: BackendDuplicateMatch[] | null;
    }>(`${STORE_SPACE_API_BASE}/stores/check-duplicate`, {
      method: "POST",
      body: JSON.stringify({
        name,
        exclude_store_id: excludeStoreId,
      }),
    });

    return {
      exactMatch: response.exact_match ? duplicateMatchToStoreSpaceSummary(response.exact_match) : null,
      similarMatches: (response.similar_matches ?? []).map(duplicateMatchToStoreSpaceSummary),
    };
  },

  async listStores(query: string, page: number, pageSize = PAGE_SIZE): Promise<StoreListResponse> {
    const search = new URLSearchParams({
      q: query,
      page: String(page),
      page_size: String(pageSize),
    });
    const response = await requestJSON<BackendStoreSpaceListResponse>(`${STORE_SPACE_API_BASE}/stores?${search.toString()}`);
    return {
      items: response.items.map(mapStoreSpaceSummary),
      page: response.page,
      pageSize: response.page_size ?? response.pageSize ?? pageSize,
      total: response.total,
    };
  },

  async getStore(id: number): Promise<StoreDetail> {
    const response = await requestJSON<BackendStoreSpaceDetail>(`${STORE_SPACE_API_BASE}/stores/${id}`);
    return mapStoreSpaceDetail(response);
  },

  async saveStore(payload: SaveStorePayload): Promise<StoreDetail> {
    if (!payload.id) {
      throw new ApiError(400, "保存设计图标注需要门店 ID");
    }
    const response = await requestJSON<BackendStoreSpaceDetail>(`${STORE_SPACE_API_BASE}/stores/${payload.id}/design-plan`, {
      method: "PUT",
      body: JSON.stringify(toStoreSpaceDesignPlanPayload(payload)),
    });
    return mapStoreSpaceDetail(response);
  },

  async listEzvizAccounts(): Promise<EzvizAccount[]> {
    const response = await requestJSON<BackendEzvizAccount[]>(`${STORE_SPACE_API_BASE}/ezviz-accounts`);
    return response.map(mapBackendEzvizAccount);
  },

  async createEzvizAccount(payload: CreateEzvizAccountPayload): Promise<EzvizAccount> {
    const response = await requestJSON<BackendEzvizAccount>(`${STORE_SPACE_API_BASE}/ezviz-accounts`, {
      method: "POST",
      body: JSON.stringify({ account_name: payload.accountName.trim() }),
    });
    return mapBackendEzvizAccount(response);
  },

  async scanRecorder(recorderId: number): Promise<VideoRecorder> {
    const response = await requestJSON<BackendVideoRecorder>(`${STORE_SPACE_API_BASE}/recorders/${recorderId}/scan-channels`, {
      method: "POST",
    });
    return mapBackendRecorder(response);
  },

  async recognizeRecorder(recorderId: number): Promise<VideoRecorder> {
    const response = await requestJSON<BackendVideoRecorder>(`${STORE_SPACE_API_BASE}/recorders/${recorderId}/recognize-channels`, {
      method: "POST",
    });
    return mapBackendRecorder(response);
  },

  async recognizeChannel(channelId: number): Promise<VideoChannel> {
    const response = await requestJSON<BackendVideoChannel>(`${STORE_SPACE_API_BASE}/channels/${channelId}/recognize`, {
      method: "POST",
    });
    return mapBackendChannel(response, 0, "");
  },

  async confirmChannel(channelId: number, patch: Partial<VideoChannel>): Promise<StoreDetail> {
    const response = await requestJSON<BackendStoreSpaceDetail>(`${STORE_SPACE_API_BASE}/channels/${channelId}/confirmation`, {
      method: "PUT",
      body: JSON.stringify(toStoreSpaceChannelConfirmationPayload(patch)),
    });
    return mapStoreSpaceDetail(response);
  },

  async unlockChannelForEdit(channelId: number): Promise<VideoChannel> {
    const response = await requestJSON<BackendVideoChannel>(`${STORE_SPACE_API_BASE}/channels/${channelId}/unlock`, {
      method: "POST",
    });
    return mapBackendChannel(response, 0, "");
  },

  async deleteStore(id: number): Promise<void> {
    await requestJSON<void>(`${STORE_SPACE_API_BASE}/stores/${id}`, { method: "DELETE" });
  },

  async deleteRecorder(recorderId: number): Promise<void> {
    await requestJSON<void>(`${STORE_SPACE_API_BASE}/recorders/${recorderId}`, { method: "DELETE" });
  },

  async deleteChannel(channelId: number): Promise<StoreDetail> {
    const response = await requestJSON<BackendStoreSpaceDetail>(`${STORE_SPACE_API_BASE}/channels/${channelId}`, {
      method: "DELETE",
    });
    return mapStoreSpaceDetail(response);
  },

  async addRecorder(storeId: number, payload: AddRecorderPayload): Promise<StoreDetail> {
    const response = await requestJSON<BackendStoreSpaceDetail>(`${STORE_SPACE_API_BASE}/stores/${storeId}/recorders`, {
      method: "POST",
      body: JSON.stringify({
        ezviz_account_id: payload.ezvizAccountId ? Number(payload.ezvizAccountId) : 0,
        device_code: payload.deviceCode.trim(),
      }),
    });
    return mapStoreSpaceDetail(response);
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

export const storeSpaceApi = {
  endpoints: {
    base: STORE_SPACE_API_BASE,
  },

  async listStores(query: string, page: number, pageSize = PAGE_SIZE): Promise<StoreListResponse> {
    if (API_MODE === "mock") {
      return mockAdapter.listStores(query, page, pageSize);
    }
    return storeSpaceHttpAdapter.listStores(query, page, pageSize);
  },

  async getStore(id: number): Promise<StoreDetail> {
    if (API_MODE === "mock") {
      return mockAdapter.getStore(id);
    }
    return storeSpaceHttpAdapter.getStore(id);
  },

  async uploadPdf(file: File | string = "mock-design-plan.pdf"): Promise<UploadResult> {
    return designPlanApi.uploadPdf(file);
  },

  async recognizeUpload(uploadId: string): Promise<RecognitionResult> {
    return designPlanApi.recognizeUpload(uploadId);
  },

  async checkDuplicate(name: string, excludeStoreId?: number): Promise<DuplicateCheckResult> {
    if (API_MODE === "mock") {
      return mockAdapter.checkDuplicate(name, excludeStoreId);
    }
    return storeSpaceHttpAdapter.checkDuplicate(name, excludeStoreId);
  },

  async saveStore(payload: SaveStorePayload): Promise<StoreDetail> {
    if (API_MODE === "mock") {
      return mockAdapter.saveStore(payload);
    }
    return storeSpaceHttpAdapter.saveStore(payload);
  },

  async createStore(payload: CreateStoreSpacePayload): Promise<StoreDetail> {
    if (API_MODE === "mock") {
      return mockAdapter.createStoreSpace(payload);
    }
    return storeSpaceHttpAdapter.createStore(payload);
  },

  async deleteStore(id: number): Promise<void> {
    if (API_MODE === "mock") {
      return mockAdapter.deleteStore(id);
    }
    return storeSpaceHttpAdapter.deleteStore(id);
  },

  async listEzvizAccounts(): Promise<EzvizAccount[]> {
    if (API_MODE === "mock") {
      return mockAdapter.listEzvizAccounts();
    }
    return storeSpaceHttpAdapter.listEzvizAccounts();
  },

  async createEzvizAccount(payload: CreateEzvizAccountPayload): Promise<EzvizAccount> {
    if (API_MODE === "mock") {
      const account: EzvizAccount = {
        id: Date.now(),
        accountName: payload.accountName.trim(),
        status: "unverified",
        lastVerifiedAt: "",
      };
      mockEzvizAccounts = [...mockEzvizAccounts, account];
      return clone(account);
    }
    return storeSpaceHttpAdapter.createEzvizAccount(payload);
  },

  async scanRecorder(storeId: number, recorderId: number): Promise<VideoRecorder> {
    if (API_MODE === "mock") {
      return mockAdapter.scanRecorder(storeId, recorderId);
    }
    return storeSpaceHttpAdapter.scanRecorder(recorderId);
  },

  async recognizeRecorder(storeId: number, recorderId: number): Promise<VideoRecorder> {
    if (API_MODE === "mock") {
      return mockAdapter.recognizeRecorder(storeId, recorderId);
    }
    return storeSpaceHttpAdapter.recognizeRecorder(recorderId);
  },

  async recognizeChannel(storeId: number, channelId: number): Promise<VideoChannel> {
    if (API_MODE === "mock") {
      return mockAdapter.recognizeChannel(storeId, channelId);
    }
    return storeSpaceHttpAdapter.recognizeChannel(channelId);
  },

  async confirmChannel(storeId: number, channelId: number, patch: Partial<VideoChannel>): Promise<StoreDetail> {
    if (API_MODE === "mock") {
      return mockAdapter.confirmChannel(storeId, channelId, patch);
    }
    return storeSpaceHttpAdapter.confirmChannel(channelId, patch);
  },

  async unlockChannelForEdit(storeId: number, channelId: number): Promise<VideoChannel> {
    if (API_MODE === "mock") {
      return mockAdapter.unlockChannelForEdit(storeId, channelId);
    }
    return storeSpaceHttpAdapter.unlockChannelForEdit(channelId);
  },

  async deleteRecorder(storeId: number, recorderId: number): Promise<StoreDetail> {
    if (API_MODE === "mock") {
      return mockAdapter.deleteRecorder(storeId, recorderId);
    }
    await storeSpaceHttpAdapter.deleteRecorder(recorderId);
    return storeSpaceHttpAdapter.getStore(storeId);
  },

  async deleteChannel(storeId: number, channelId: number): Promise<StoreDetail> {
    if (API_MODE === "mock") {
      return mockAdapter.deleteChannel(storeId, channelId);
    }
    return storeSpaceHttpAdapter.deleteChannel(channelId);
  },

  async addRecorder(storeId: number, payload: AddRecorderPayload): Promise<StoreDetail> {
    if (API_MODE === "mock") {
      return mockAdapter.addRecorder(storeId, payload);
    }
    return storeSpaceHttpAdapter.addRecorder(storeId, payload);
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
    const fields =
      typeof data === "object" && data && "fields" in data && isStringRecord(data.fields)
        ? data.fields
        : {};
    const message =
      typeof data === "object" && data && "message" in data
        ? String(data.message)
        : typeof data === "object" && data && "error" in data
          ? String(data.error)
          : `HTTP ${response.status}`;
    throw new ApiError(response.status, message, fields);
  }

  return data as T;
}

function mapBackendSummary(item: BackendStoreSummary): StoreSummary {
  return {
    id: item.id,
    city: item.city ?? "",
    name: item.name,
    externalOrgId: "",
    thumbnailUrl: toDisplayImageUrl(item.thumbnail_url),
    designPlanStatus: item.thumbnail_url ? "completed" : "not_uploaded",
    recorderCount: 0,
    channelCount: 0,
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
    city: store.city ?? "",
    name: store.name,
    fileName: displayPlanFileName(store.pdf_file_name, store.original_pdf_path, store.name),
    originalPath: store.original_pdf_path || MOCK_ORIGINAL_PDF_PATH,
    previewPath: store.preview_image_path || MOCK_PREVIEW_IMAGE_PATH,
    thumbnailPath: store.thumbnail_path || MOCK_THUMBNAIL_PATH,
    pageCount: store.page_count || 1,
    thumbnailUrl: toDisplayImageUrl(store.thumbnail_url || store.thumbnail_path),
    previewUrl: toDisplayImageUrl(store.preview_url || store.preview_image_path),
    externalOrgId: "",
    designPlanStatus: store.preview_url || store.preview_image_path ? "completed" : "not_uploaded",
    recorderCount: 0,
    channelCount: 0,
    treatmentCount: counts.treatment,
    consultationCount: counts.consultation,
    beautyCount: counts.beauty,
    areaCount: areas.length,
    status: store.status,
    updatedAt: store.updated_at,
    recognitionResult: store.recognition_result,
    areas,
    recorders: [],
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

function mapStoreSpaceSummary(store: BackendStoreSpaceSummary): StoreSummary {
  return {
    id: store.id,
    city: store.city ?? store.cityName ?? "",
    name: store.name,
    externalOrgId: store.external_org_id ?? store.externalOrgId ?? "",
    thumbnailUrl: "",
    designPlanStatus: store.design_plan_status ?? store.designPlanStatus ?? "not_uploaded",
    recorderCount: store.recorder_count ?? store.recorderCount ?? 0,
    channelCount: store.channel_count ?? store.channelCount ?? 0,
    treatmentCount: store.treatment_count ?? store.treatmentCount ?? 0,
    consultationCount: store.consultation_count ?? store.consultationCount ?? 0,
    beautyCount: store.beauty_count ?? store.beautyCount ?? 0,
    areaCount: store.area_count ?? store.areaCount ?? 0,
    status: mapStoreSpaceOverallStatus(store.overall_status ?? store.overallStatus),
    updatedAt: store.updated_at ?? store.updatedAt ?? new Date().toISOString(),
  };
}

function mapStoreSpaceDetail(store: BackendStoreSpaceDetail): StoreDetail {
  const designPlan = firstStoreSpaceDesignPlan(store);
  const areas = (store.areas ?? []).map(mapStoreSpaceArea);
  const recorders = (store.recorders ?? []).map(mapBackendRecorder);
  const counts = countAreas(areas);
  const summary = mapStoreSpaceSummary({
    ...store,
    treatment_count: store.treatment_count ?? store.treatmentCount ?? counts.treatment,
    consultation_count: store.consultation_count ?? store.consultationCount ?? counts.consultation,
    beauty_count: store.beauty_count ?? store.beautyCount ?? counts.beauty,
    area_count: store.area_count ?? store.areaCount ?? areas.length,
    recorder_count: store.recorder_count ?? store.recorderCount ?? recorders.length,
    channel_count: store.channel_count ?? store.channelCount ?? countChannels(recorders),
  });

  return {
    ...summary,
    fileName: displayPlanFileName(designPlan?.pdf_file_name ?? designPlan?.pdfFileName, designPlan?.original_pdf_path ?? designPlan?.originalPdfPath, store.name),
    originalPath: designPlan?.original_pdf_path ?? designPlan?.originalPdfPath ?? "",
    previewPath: designPlan?.preview_image_path ?? designPlan?.previewImagePath ?? "",
    thumbnailPath: designPlan?.thumbnail_path ?? designPlan?.thumbnailPath ?? "",
    pageCount: designPlan?.page_count ?? designPlan?.pageCount ?? 0,
    previewUrl: toDisplayImageUrl(designPlan?.preview_url ?? designPlan?.previewUrl ?? designPlan?.preview_image_path ?? designPlan?.previewImagePath),
    thumbnailUrl: toDisplayImageUrl(designPlan?.thumbnail_url ?? designPlan?.thumbnailUrl ?? designPlan?.thumbnail_path ?? designPlan?.thumbnailPath),
    recognitionResult: designPlan?.recognition_result ?? designPlan?.recognitionResult,
    areas,
    recorders,
  };
}

function mapStoreSpaceArea(areaItem: BackendStoreSpaceArea): StoreArea {
  const type = areaItem.area_type ?? areaItem.areaType ?? "";
  const number = areaItem.area_number == null && areaItem.areaNumber == null ? "" : String(areaItem.area_number ?? areaItem.areaNumber);
  const source = normalizeAreaSource(areaItem.source);
  const hasPendingAnnotation = areaItem.annotation?.status === "pending";
  return {
    id: areaItem.id ? String(areaItem.id) : `area-${type || "unknown"}-${number || Date.now()}`,
    name: areaItem.display_name ?? areaItem.displayName ?? areaDisplayNameFromParts(type, number),
    type,
    number,
    source,
    confidence: areaItem.confidence ?? "high",
    needsReview:
      source === "video_channel" || source === "multiple"
        ? false
        : Boolean(areaItem.needs_review ?? areaItem.needsReview) || areaItem.status === "candidate" || hasPendingAnnotation,
    box: areaItem.box ?? areaItem.annotation?.box ?? null,
  };
}

function normalizeAreaSource(source: string | undefined): StoreArea["source"] {
  if (source === "manual" || source === "design_plan" || source === "video_channel" || source === "multiple") return source;
  return undefined;
}

function firstStoreSpaceDesignPlan(store: BackendStoreSpaceDetail) {
  return (store.design_plans ?? store.designPlans ?? [])[0];
}

function mapStoreSpaceOverallStatus(status: string | undefined): StoreStatus {
  if (status === "completed") return "completed";
  if (status === "incomplete") return "incomplete";
  return "needs_review";
}

function areaDisplayNameFromParts(type: AreaType | "", number: string) {
  if (!type) return "";
  const labels: Record<AreaType, string> = {
    treatment: "治疗室",
    consultation: "面诊室",
    beauty: "生美",
  };
  return number.trim() ? `${labels[type]} ${number.trim()}` : labels[type];
}

function mapBackendEzvizAccount(account: BackendEzvizAccount): EzvizAccount {
  return {
    id: account.id,
    accountName: account.account_name ?? account.accountName ?? `账号 ${account.id}`,
    status: account.status ?? "unverified",
    lastVerifiedAt: account.last_verified_at ?? account.lastVerifiedAt ?? "",
  };
}

function mapBackendRecorder(recorder: BackendVideoRecorder): VideoRecorder {
  const id = recorder.id;
  const deviceCode = recorder.device_code ?? recorder.deviceCode ?? "";
  return {
    id,
    storeId: recorder.store_id ?? recorder.storeId ?? 0,
    ezvizAccountId: recorder.ezviz_account_id ?? recorder.ezvizAccountId ?? 0,
    accountName: recorder.account_name ?? recorder.accountName ?? "",
    deviceCode,
    status: recorder.status ?? "offline",
    effectiveChannelCount: recorder.effective_channel_count ?? recorder.effectiveChannelCount ?? recorder.channels?.length ?? 0,
    lastScannedAt: recorder.last_scanned_at ?? recorder.lastScannedAt ?? "",
    recognitionProgress: recorder.recognition_progress ?? recorder.recognitionProgress,
    channels: (recorder.channels ?? []).map((channel) => mapBackendChannel(channel, id, deviceCode)),
  };
}

function mapBackendChannel(channel: BackendVideoChannel, recorderId: number, recorderCode: string): VideoChannel {
  return {
    id: channel.id,
    recorderId: channel.recorder_id ?? channel.recorderId ?? recorderId,
    recorderCode: channel.recorder_code ?? channel.recorderCode ?? recorderCode,
    channelNo: channel.channel_no ?? channel.channelNo ?? 0,
    channelName: channel.channel_name ?? channel.channelName ?? "",
    status: channel.status ?? "pending_recognition",
    thumbnailUrl: toDisplayImageUrl(channel.thumbnail_url ?? channel.thumbnailUrl),
    fullImageUrl: toDisplayImageUrl(channel.full_image_url ?? channel.fullImageUrl ?? channel.thumbnail_url ?? channel.thumbnailUrl),
    fullImageExpiresAt: channel.full_image_expires_at ?? channel.fullImageExpiresAt,
    sceneType: channel.scene_type ?? channel.sceneType ?? "unknown",
    areaType: channel.area_type ?? channel.areaType ?? "",
    areaNumber: channel.area_number == null && channel.areaNumber == null ? "" : String(channel.area_number ?? channel.areaNumber),
    areaNote: channel.area_note ?? channel.areaNote ?? "",
    recognitionAttempts: channel.recognition_attempts ?? channel.recognitionAttempts ?? 0,
    recognitionResult: channel.recognition_result ?? channel.recognitionResult,
    confirmedAt: channel.confirmed_at ?? channel.confirmedAt,
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
    city: match.city ?? "",
    name: match.name,
    externalOrgId: "",
    thumbnailUrl: toDisplayImageUrl(match.thumbnail_url),
    designPlanStatus: match.thumbnail_url ? "completed" : "not_uploaded",
    recorderCount: 0,
    channelCount: 0,
    treatmentCount: match.treatment_count ?? 0,
    consultationCount: match.consultation_count ?? 0,
    beautyCount: match.beauty_count ?? 0,
    areaCount: match.area_count ?? 0,
    status: match.status ?? "needs_review",
    updatedAt: match.updated_at ?? new Date().toISOString(),
  };
}

function duplicateMatchToStoreSpaceSummary(match: BackendDuplicateMatch): StoreSummary {
  return {
    id: match.id,
    city: match.city ?? "",
    name: match.name,
    externalOrgId: "",
    thumbnailUrl: "",
    designPlanStatus: "not_uploaded",
    recorderCount: 0,
    channelCount: 0,
    treatmentCount: match.treatment_count ?? 0,
    consultationCount: match.consultation_count ?? 0,
    beautyCount: match.beauty_count ?? 0,
    areaCount: match.area_count ?? 0,
    status: mapStoreSpaceOverallStatus(match.overall_status ?? match.status),
    updatedAt: match.updated_at ?? new Date().toISOString(),
  };
}

function isStringRecord(value: unknown): value is Record<string, string> {
  if (!value || typeof value !== "object") return false;
  return Object.values(value).every((item) => typeof item === "string");
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

function toStoreSpaceCreatePayload(payload: CreateStoreSpacePayload) {
  return {
    city: payload.city,
    name: payload.name,
    external_org_id: payload.externalOrgId,
    design_plan_upload_id: payload.designPlan?.uploadId ?? "",
    recorders: payload.recorders
      .filter((recorder) => recorder.deviceCode.trim() && recorder.ezvizAccountId)
      .map((recorder) => ({
        ezviz_account_id: Number(recorder.ezvizAccountId),
        device_code: recorder.deviceCode.trim(),
      })),
  };
}

function toStoreSpaceDesignPlanPayload(payload: SaveStorePayload): BackendStoreSpaceDesignPlanPayload {
  return {
    upload_id: payload.uploadId ?? "",
    pdf_file_name: normalizePlanFileName(payload.fileName, payload.name),
    original_pdf_path: payload.originalPath || payload.fileName || MOCK_ORIGINAL_PDF_PATH,
    preview_image_path: payload.previewPath || toStoredPath(payload.previewUrl, MOCK_PREVIEW_IMAGE_PATH),
    thumbnail_path: payload.thumbnailPath || toStoredPath(payload.thumbnailUrl, MOCK_THUMBNAIL_PATH),
    page_count: payload.pageCount || 1,
    recognition_result: JSON.stringify(payload.recognitionResult ?? null),
    areas: payload.areas.map((areaItem) => ({
      id: numericId(areaItem.id),
      display_name: areaItem.name,
      area_type: areaItem.type as AreaType,
      area_number: areaItem.number,
      confidence: areaItem.confidence || "high",
      needs_review: areaItem.needsReview || areaItem.confidence === "low",
      box: areaItem.box ?? undefined,
    })),
  };
}

function toStoreSpaceChannelConfirmationPayload(patch: Partial<VideoChannel>) {
  if (patch.areaType) {
    return {
      kind: "business",
      area_type: patch.areaType,
      area_number: patch.areaNumber ? String(patch.areaNumber) : undefined,
    };
  }
  return {
    kind: "non_business",
    scene_type: patch.sceneType ?? "unknown",
    area_note: patch.areaNote ?? patch.areaNumber ?? "",
  };
}

function toDisplayImageUrl(value?: string) {
  if (!value) {
    return "";
  }
  if (value.startsWith("mock/")) {
    return MOCK_PLAN_IMAGE;
  }
  const storedUploadMatch = value.match(/^uploads\/([^/]+)\/(preview|thumbnail)\.png$/);
  if (storedUploadMatch) {
    return `/api/design-plan/uploads/${storedUploadMatch[1]}/${storedUploadMatch[2]}`;
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

function inferMockCity(name: string) {
  const cities = ["北京", "上海", "广州", "深圳", "成都", "杭州", "重庆", "武汉", "苏州", "西安", "南京", "长沙", "天津", "郑州", "东莞", "青岛", "昆明", "宁波", "合肥", "佛山"];
  return cities.find((city) => name.includes(city)) ?? "";
}

function createMockStore(id: number, name: string, status: StoreStatus, areas: StoreArea[]): StoreDetail {
  const now = new Date(Date.now() - id * 18 * 60 * 60 * 1000).toISOString();
  const counts = countAreas(areas);
  const recorders = id <= 3 ? [createMockRecorder(id, `EZVIZ-${String(860000 + id)}`, 1)] : [];
  return {
    id,
    city: inferMockCity(name),
    name,
    externalOrgId: id <= 3 ? `XY${String(10000 + id)}` : "",
    thumbnailUrl: MOCK_PLAN_IMAGE,
    previewUrl: MOCK_PLAN_IMAGE,
    originalPath: MOCK_ORIGINAL_PDF_PATH,
    previewPath: MOCK_PREVIEW_IMAGE_PATH,
    thumbnailPath: MOCK_THUMBNAIL_PATH,
    pageCount: 1,
    fileName: `${name}-design.pdf`,
    designPlanStatus: "completed",
    recorderCount: recorders.length,
    channelCount: countChannels(recorders),
    status,
    updatedAt: now,
    areaCount: areas.length,
    treatmentCount: counts.treatment,
    consultationCount: counts.consultation,
    beautyCount: counts.beauty,
    areas,
    recorders,
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
    city: payload.city?.trim() ?? "",
    name: payload.name.trim(),
    externalOrgId: payload.externalOrgId?.trim() ?? "",
    fileName: payload.fileName,
    originalPath: payload.originalPath || MOCK_ORIGINAL_PDF_PATH,
    previewPath: payload.previewPath || MOCK_PREVIEW_IMAGE_PATH,
    thumbnailPath: payload.thumbnailPath || MOCK_THUMBNAIL_PATH,
    pageCount: payload.pageCount || 1,
    thumbnailUrl: payload.thumbnailUrl,
    previewUrl: payload.previewUrl,
    designPlanStatus: payload.previewUrl ? "pending_annotation" : "not_uploaded",
    recorderCount: payload.recorders?.length ?? 0,
    channelCount: countChannels(payload.recorders ?? []),
    treatmentCount: counts.treatment,
    consultationCount: counts.consultation,
    beautyCount: counts.beauty,
    areaCount: areas.length,
    status,
    updatedAt,
    areas,
    recorders: payload.recorders ?? [],
  };
}

function createMockRecorder(storeId: number, deviceCode: string, ezvizAccountId: number): VideoRecorder {
  const account = mockEzvizAccounts.find((item) => item.id === ezvizAccountId) ?? mockEzvizAccounts[0];
  const id = nextRecorderId++;
  const channels = storeId <= 3 ? createMockChannels(id, deviceCode) : [];
  return {
    id,
    storeId,
    ezvizAccountId: account?.id ?? 0,
    accountName: account?.accountName ?? "未选择区域",
    deviceCode,
    status: storeId <= 3 ? "online" : "offline",
    effectiveChannelCount: channels.length,
    lastScannedAt: storeId <= 3 ? new Date(Date.now() - storeId * 60 * 60 * 1000).toISOString() : "",
    recognitionProgress: storeId <= 3 ? "等待人工确认" : "",
    channels,
  };
}

function createMockChannels(recorderId: number, recorderCode: string): VideoChannel[] {
  const scenes: Array<Pick<VideoChannel, "sceneType" | "areaType" | "areaNumber" | "areaNote" | "status">> = [
    { sceneType: "consultation", areaType: "consultation", areaNumber: "1", areaNote: "", status: "pending_confirmation" },
    { sceneType: "treatment", areaType: "treatment", areaNumber: "1", areaNote: "", status: "confirmed_business" },
    { sceneType: "front_desk", areaType: "", areaNumber: "", areaNote: "前台", status: "confirmed_non_business" },
  ];
  return scenes.map((scene, index) => ({
    id: nextChannelId++,
    recorderId,
    recorderCode,
    channelNo: index + 1,
    channelName: `通道 ${index + 1}`,
    thumbnailUrl: "",
    fullImageUrl: "",
    recognitionAttempts: scene.status === "pending_confirmation" ? 1 : 2,
    confirmedAt: scene.status.startsWith("confirmed") ? new Date(Date.now() - (index + 1) * 32 * 60 * 1000).toISOString() : undefined,
    ...scene,
  }));
}

function ensureRecorderChannels(recorder: VideoRecorder): VideoChannel[] {
  return recorder.channels.length > 0 ? recorder.channels : createMockChannels(recorder.id, recorder.deviceCode);
}

function replaceMockRecorder(storeId: number, nextRecorder: VideoRecorder) {
  mockStores = mockStores.map((store) => {
    if (store.id !== storeId) return store;
    const recorders = store.recorders.map((recorder) => (recorder.id === nextRecorder.id ? nextRecorder : recorder));
    return {
      ...store,
      recorders,
      recorderCount: recorders.length,
      channelCount: countChannels(recorders),
      updatedAt: new Date().toISOString(),
    };
  });
}

function mergeAreasFromConfirmedChannels(areas: StoreArea[], recorders: VideoRecorder[]): StoreArea[] {
  const nextAreas = [...areas];
  for (const recorder of recorders) {
    for (const channel of recorder.channels) {
      if (channel.status !== "confirmed_business" || !channel.areaType || !channel.areaNumber.trim()) continue;
      const exists = nextAreas.some((areaItem) => areaItem.type === channel.areaType && areaItem.number === channel.areaNumber);
      if (!exists) {
        nextAreas.push(
          area(
            `channel-${channel.id}`,
            "",
            channel.areaType,
            channel.areaNumber,
            "medium",
            { x: 0.38, y: 0.32, width: 0.16, height: 0.11 },
          ),
        );
      }
    }
  }
  return nextAreas;
}

function countChannels(recorders: VideoRecorder[]) {
  return recorders.reduce(
    (total, recorder) => total + recorder.channels.filter((channel) => channel.status !== "inactive").length,
    0,
  );
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

function normalizeStoreName(value: string) {
  return normalizeName(value)
    .replace(/[\s\-_·・()（）]/g, "")
    .replaceAll("新氧青春", "")
    .replaceAll("门店", "")
    .replaceAll("店", "")
    .replaceAll("分院", "")
    .replaceAll("院", "");
}

function matchesStoreSearch(name: string, query: string) {
  const rawQuery = normalizeName(query).replace(/\s+/g, "");
  if (!rawQuery) return true;
  if (normalizeName(name).replace(/\s+/g, "").includes(rawQuery)) {
    return true;
  }
  const normalizedQuery = normalizeStoreName(query);
  return Boolean(normalizedQuery) && normalizeStoreName(name).includes(normalizedQuery);
}

function nonBusinessSceneLabel(sceneType: SceneType) {
  const labels: Partial<Record<SceneType, string>> = {
    front_desk: "前台",
    corridor: "走廊",
    passage: "通道",
    waiting_area: "候诊区",
    hall: "大厅",
    entrance: "门口",
    storage: "库房",
    pharmacy: "药房",
    machine_room: "机房",
    unknown: "",
  };
  return labels[sceneType] ?? "";
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
