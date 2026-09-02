import sampleStoreFloorPlanUrl from "../../testdata/design-plans/generated/sample-store-floor-plan.png";
import { isTreatmentAreaType } from "./domain/areas";
import type { AuthState } from "./domain/auth";
import { normalizeCityFilter, storeListSearchParams } from "./domain/store-list-query";
import { defaultApiBase, displayImageUrl, storedImagePath, trimTrailingSlash } from "./url-utils";

export type AreaType = "treatment" | "vip_treatment" | "consultation" | "beauty";

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
  shortName: string;
  externalOrgId: string;
  canViewMonitor: boolean;
  thumbnailUrl: string;
  designPlanStatus: DesignPlanStatus;
  recorderCount: number | undefined;
  channelCount: number | undefined;
  channelsFullyConfirmed: boolean | undefined;
  treatmentCount: number | undefined;
  consultationCount: number | undefined;
  beautyCount: number | undefined;
  areaCount: number | undefined;
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
  bedLabel: string;
  areaNote: string;
  recognitionAttempts: number;
  recognitionResult?: unknown;
  confirmedAt?: string;
};

export type ProbeRecognizeChannelResult = {
  active: boolean;
  channel?: VideoChannel;
  message?: string;
};

export type SnapshotDiagnostics = {
  code: string;
  stage: string;
  assetStore: string;
  snapshotName: string;
  snapshotKey: string;
  exists: boolean;
  detail?: string;
};

export type LiveAddressPayload = {
  ezvizAccountId?: number | "";
  accountName?: string;
  deviceSerial: string;
  channelNo: number;
  code?: string;
};

export type LiveAddressResult = {
  url: string;
  urlId: string;
  expireTime: string;
  protocol: string;
};

export type StoreListResponse = {
  items: StoreSummary[];
  page: number;
  pageSize: number;
  total: number;
  summary: StoreListSummary;
  cities: string[];
};

export type StoreListSummary = {
  storeCount: number;
  consultationCount: number;
  treatmentCount: number;
  beautyCount: number;
};

export type ResourceIssueSeverity = "error" | "warning" | "info";
export type ResourceIssueType =
  | "unbound_camera"
  | "inactive_bound_space"
  | "missing_camera"
  | "missing_space"
  | "missing_nvr"
  | "offline_edge"
  | "offline_nvr"
  | "offline_camera"
  | "camera_bound_many_spaces"
  | "space_bound_many_cameras";

export type ResourceStoreListSummary = {
  storeCount: number;
  edgeCount: number;
  nvrCount: number;
  cameraCount: number;
  spaceCount: number;
  consultationCameraCount: number;
  treatmentCameraCount: number;
  boundCameraCount: number;
  unboundCameraCount: number;
  offlineDeviceCount: number;
  warningCount: number;
};

export type ResourceCityOption = {
  cityId: number;
  name: string;
  count: number;
};

export type ResourceStoreSummary = {
  tenantId: number;
  storeName: string;
  hospitalName: string;
  cityId: number;
  cityName: string;
  edgeCount: number;
  nvrCount: number;
  cameraCount: number;
  spaceCount: number;
  consultationCameraCount: number;
  treatmentCameraCount: number;
  boundCameraCount: number;
  unboundCameraCount: number;
  offlineDeviceCount: number;
  warningCount: number;
  camerasFullyBound: boolean;
  updatedAt: string;
  canViewMonitor: boolean;
  monitorUrl?: string;
};

export type ResourceDevice = {
  id: number;
  parentId: number;
  tenantId: number;
  name: string;
  hardwareId: string;
  sn?: string;
  ip?: string;
  category: "edge" | "nvr" | "camera" | string;
  provider?: string;
  status: number;
  statusText: string;
  onlineStatus: number;
  onlineText: string;
  extSummary?: string;
  heartbeatAt?: string;
};

export type ResourceCamera = ResourceDevice & {
  channelNo?: number;
  nvrId: number;
  nvrName?: string;
  spacePaths: string[];
  thumbnailKind?: string;
  thumbnailUrl?: string;
};

export type ResourceSpace = {
  id: number;
  tenantId: number;
  parentId: number;
  name: string;
  code?: string;
  level: number;
  status: number;
  statusText: string;
  dictId: number;
  sortOrder: number;
  boundCameraIds: number[];
  boundCameraCount: number;
};

export type ResourceAreaDeviceRelation = {
  id: number;
  deviceId: number;
  areaId: number;
  functionType: string;
};

export type ResourceSpaceNode = ResourceSpace & {
  boundCameras: ResourceCamera[];
  children: ResourceSpaceNode[];
};

export type ResourceDeviceTree = {
  edges: ResourceDevice[];
  nvrs: Array<ResourceDevice & { cameras: ResourceCamera[] }>;
};

export type ResourceIssue = {
  severity: ResourceIssueSeverity;
  type: ResourceIssueType;
  message: string;
  entityType: string;
  entityId: number;
};

export type ResourceStoreDetail = {
  tenantId: number;
  storeName: string;
  hospitalName: string;
  cityId: number;
  cityName: string;
  camerasFullyBound: boolean;
  updatedAt: string;
  summary: ResourceStoreListSummary;
  edges: ResourceDevice[];
  nvrs: ResourceDevice[];
  cameras: ResourceCamera[];
  spaces: ResourceSpace[];
  relations: ResourceAreaDeviceRelation[];
  spaceTree: ResourceSpaceNode[];
  deviceTree: ResourceDeviceTree;
  issues: ResourceIssue[];
  canViewMonitor: boolean;
  monitorUrl?: string;
};

export type ResourceStoreListResponse = {
  items: ResourceStoreSummary[];
  page: number;
  pageSize: number;
  total: number;
  summary: ResourceStoreListSummary;
  cities: ResourceCityOption[];
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
  shortName: string;
  externalOrgId: string;
  designPlan?: UploadResult | null;
  recorders: RecorderDraft[];
};

export type UpdateStoreBasicInfoPayload = {
  id: number;
  city: string;
  name: string;
  shortName: string;
  externalOrgId: string;
};

export type AddRecorderPayload = {
  ezvizAccountId: number | "";
  deviceCode: string;
};

export type AISettings = {
  provider: "openai" | "minimax";
  model: string;
  label: string;
};

export type MonitorScreenshotWatermarkSettings = {
  enabled: boolean;
};

export type ManagedUserRole = "admin" | "editor" | "viewer";

export type MonitorStoreScope = {
  storeId: number;
  city: string;
  name: string;
  externalOrgId: string;
};

export type ManagedUser = {
  id: number;
  email: string;
  username: string;
  displayName: string;
  role: ManagedUserRole;
  enabled: boolean;
  lastLoginAt?: string;
  monitorStoreScopeCount: number;
  monitorStoreScopes: MonitorStoreScope[];
};

export type ManagedUserPayload = {
  email?: string;
  username: string;
  displayName: string;
  role: ManagedUserRole;
  enabled: boolean;
  monitorStoreScopeIds: number[];
};

export type AuditLogItem = {
  actorDisplayName: string;
  userEmail: string;
  action: string;
  entityType: string;
  entityId?: number;
  externalOrgId: string;
  result: "success" | "denied" | "failed" | string;
  createdAt: string;
  summary: string;
};

export type AuditLogPage = {
  items: AuditLogItem[];
  page: number;
  pageSize: number;
  total: number;
};

type ApiMode = "auto" | "http" | "mock";

type BackendStoreListResponse = {
  items: BackendStoreSummary[];
  page: number;
  page_size: number;
  total: number;
  cities?: string[];
  cityOptions?: string[];
};

type BackendStoreSummary = {
  id: number;
  city?: string;
  name: string;
  short_name?: string;
  shortName?: string;
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
  short_name?: string;
  shortName?: string;
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

type BackendLiveAddressResult = {
  url: string;
  url_id?: string;
  urlId?: string;
  expire_time?: string;
  expireTime?: string;
  protocol?: string;
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
  short_name?: string;
  shortName?: string;
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
  bed_label?: string;
  bedLabel?: string;
  area_note?: string;
  areaNote?: string;
  recognition_attempts?: number;
  recognitionAttempts?: number;
  recognition_result?: unknown;
  recognitionResult?: unknown;
  confirmed_at?: string;
  confirmedAt?: string;
};

type BackendProbeRecognizeChannelResult = {
  active?: boolean;
  channel?: BackendVideoChannel;
  message?: string;
};

type BackendSnapshotDiagnostics = {
  code?: string;
  stage?: string;
  asset_store?: string;
  assetStore?: string;
  snapshot_name?: string;
  snapshotName?: string;
  snapshot_key?: string;
  snapshotKey?: string;
  exists?: boolean;
  detail?: string;
};

type BackendManagedUser = {
  id: number;
  email: string;
  username?: string;
  display_name?: string;
  displayName?: string;
  role?: ManagedUserRole | string;
  enabled?: boolean;
  last_login_at?: string;
  lastLoginAt?: string;
  monitor_store_scope_count?: number;
  monitorStoreScopeCount?: number;
  monitor_store_scopes?: BackendMonitorStoreScope[];
  monitorStoreScopes?: BackendMonitorStoreScope[];
};

type BackendMonitorStoreScope = {
  store_id?: number;
  storeId?: number;
  city?: string;
  name?: string;
  external_org_id?: string;
  externalOrgId?: string;
};

type BackendAuditLogItem = {
  actor_display_name?: string;
  user_email?: string;
  action?: string;
  entity_type?: string;
  entity_id?: number;
  external_org_id?: string;
  result?: string;
  created_at?: string;
  summary?: string;
};

type BackendStoreSpaceListResponse = {
  items: BackendStoreSpaceSummary[];
  page: number;
  page_size?: number;
  pageSize?: number;
  total: number;
  summary?: BackendStoreListSummary;
  cities?: string[];
  cityOptions?: string[];
};

type BackendStoreListSummary = {
  store_count?: number;
  storeCount?: number;
  consultation_count?: number;
  consultationCount?: number;
  treatment_count?: number;
  treatmentCount?: number;
  beauty_count?: number;
  beautyCount?: number;
};

type BackendResourceStoreListResponse = {
  items?: BackendResourceStoreSummary[];
  page?: number;
  page_size?: number;
  pageSize?: number;
  total?: number;
  summary?: BackendResourceStoreListSummary;
  cities?: BackendResourceCityOption[];
};

type BackendResourceCityOption = {
  city_id?: number;
  cityId?: number;
  name?: string;
  count?: number;
};

type BackendResourceStoreListSummary = {
  store_count?: number;
  storeCount?: number;
  edge_count?: number;
  edgeCount?: number;
  nvr_count?: number;
  nvrCount?: number;
  camera_count?: number;
  cameraCount?: number;
  space_count?: number;
  spaceCount?: number;
  consultation_camera_count?: number;
  consultationCameraCount?: number;
  treatment_camera_count?: number;
  treatmentCameraCount?: number;
  bound_camera_count?: number;
  boundCameraCount?: number;
  unbound_camera_count?: number;
  unboundCameraCount?: number;
  offline_device_count?: number;
  offlineDeviceCount?: number;
  warning_count?: number;
  warningCount?: number;
};

type BackendResourceStoreSummary = {
  tenant_id?: number;
  tenantId?: number;
  store_name?: string;
  storeName?: string;
  hospital_name?: string;
  hospitalName?: string;
  city_id?: number;
  cityId?: number;
  city_name?: string;
  cityName?: string;
  edge_count?: number;
  edgeCount?: number;
  nvr_count?: number;
  nvrCount?: number;
  camera_count?: number;
  cameraCount?: number;
  space_count?: number;
  spaceCount?: number;
  consultation_camera_count?: number;
  consultationCameraCount?: number;
  treatment_camera_count?: number;
  treatmentCameraCount?: number;
  bound_camera_count?: number;
  boundCameraCount?: number;
  unbound_camera_count?: number;
  unboundCameraCount?: number;
  offline_device_count?: number;
  offlineDeviceCount?: number;
  warning_count?: number;
  warningCount?: number;
  cameras_fully_bound?: boolean;
  camerasFullyBound?: boolean;
  updated_at?: string;
  updatedAt?: string;
  can_view_monitor?: boolean;
  canViewMonitor?: boolean;
  monitor_url?: string;
  monitorUrl?: string;
};

type BackendResourceDevice = {
  id?: number;
  parent_id?: number;
  parentId?: number;
  tenant_id?: number;
  tenantId?: number;
  name?: string;
  hardware_id?: string;
  hardwareId?: string;
  sn?: string;
  ip?: string;
  category?: string;
  provider?: string;
  status?: number;
  status_text?: string;
  statusText?: string;
  online_status?: number;
  onlineStatus?: number;
  online_text?: string;
  onlineText?: string;
  ext_summary?: string;
  extSummary?: string;
  heartbeat_at?: string;
  heartbeatAt?: string;
};

type BackendResourceCamera = BackendResourceDevice & {
  channel_no?: number;
  channelNo?: number;
  nvr_id?: number;
  nvrId?: number;
  nvr_name?: string;
  nvrName?: string;
  space_paths?: string[];
  spacePaths?: string[];
  thumbnail_kind?: string;
  thumbnailKind?: string;
  thumbnail_url?: string;
  thumbnailUrl?: string;
};

type BackendResourceSpace = {
  id?: number;
  tenant_id?: number;
  tenantId?: number;
  parent_id?: number;
  parentId?: number;
  name?: string;
  code?: string;
  level?: number;
  status?: number;
  status_text?: string;
  statusText?: string;
  dict_id?: number;
  dictId?: number;
  sort_order?: number;
  sortOrder?: number;
  bound_camera_ids?: number[];
  boundCameraIds?: number[];
  bound_camera_count?: number;
  boundCameraCount?: number;
};

type BackendResourceAreaDeviceRelation = {
  id?: number;
  device_id?: number;
  deviceId?: number;
  area_id?: number;
  areaId?: number;
  function_type?: string;
  functionType?: string;
};

type BackendResourceSpaceNode = BackendResourceSpace & {
  bound_cameras?: BackendResourceCamera[];
  boundCameras?: BackendResourceCamera[];
  children?: BackendResourceSpaceNode[];
};

type BackendResourceDeviceTree = {
  edges?: BackendResourceDevice[];
  nvrs?: Array<BackendResourceDevice & { cameras?: BackendResourceCamera[] }>;
};

type BackendResourceIssue = {
  severity?: ResourceIssueSeverity;
  type?: ResourceIssueType;
  message?: string;
  entity_type?: string;
  entityType?: string;
  entity_id?: number;
  entityId?: number;
};

type BackendResourceStoreDetail = BackendResourceStoreSummary & {
  summary?: BackendResourceStoreListSummary;
  edges?: BackendResourceDevice[];
  nvrs?: BackendResourceDevice[];
  cameras?: BackendResourceCamera[];
  spaces?: BackendResourceSpace[];
  relations?: BackendResourceAreaDeviceRelation[];
  space_tree?: BackendResourceSpaceNode[];
  spaceTree?: BackendResourceSpaceNode[];
  device_tree?: BackendResourceDeviceTree;
  deviceTree?: BackendResourceDeviceTree;
  issues?: BackendResourceIssue[];
};

type BackendStoreSpaceSummary = {
  id: number;
  city?: string;
  cityName?: string;
  name: string;
  short_name?: string;
  shortName?: string;
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
  channels_fully_confirmed?: boolean;
  channelsFullyConfirmed?: boolean;
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
  can_view_monitor?: boolean;
  canViewMonitor?: boolean;
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
  code: string;
  stage: string;
  detail: string;

  constructor(status: number, message: string, fields: Record<string, string> = {}, code = "", stage = "", detail = "") {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.fields = fields;
    this.code = code;
    this.stage = stage;
    this.detail = detail;
  }
}

const DEFAULT_API_BASE = defaultApiBase("design-plan", import.meta.env.BASE_URL);
const API_BASE = trimTrailingSlash(import.meta.env.VITE_DESIGN_PLAN_API_BASE || DEFAULT_API_BASE);
const DEFAULT_STORE_SPACE_API_BASE = defaultApiBase("store-space", import.meta.env.BASE_URL);
const STORE_SPACE_API_BASE = trimTrailingSlash(import.meta.env.VITE_STORE_SPACE_API_BASE || DEFAULT_STORE_SPACE_API_BASE);
const APP_API_BASE = STORE_SPACE_API_BASE.replace(/\/api\/store-space$/, "/api");
const RESOURCE_VIEW_API_BASE = `${APP_API_BASE}/store-space-resource-view`;
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
let mockAISettings: AISettings = { provider: "openai", model: "gpt-5.5", label: "OpenAI / gpt-5.5" };
let mockManagedUsers: ManagedUser[] = [
  { id: 1, email: "shalei@soyoung.com", username: "shalei", displayName: "沙磊", role: "admin", enabled: true, monitorStoreScopeCount: 0, monitorStoreScopes: [] },
  { id: 2, email: "maming@soyoung.com", username: "maming", displayName: "马明", role: "admin", enabled: true, monitorStoreScopeCount: 0, monitorStoreScopes: [] },
  { id: 3, email: "changwenxia@soyoung.com", username: "changwenxia", displayName: "常文霞", role: "editor", enabled: true, monitorStoreScopeCount: 0, monitorStoreScopes: [] },
  { id: 4, email: "wangxiaofan@soyoung.com", username: "wangxiaofan", displayName: "王晓凡", role: "editor", enabled: true, monitorStoreScopeCount: 0, monitorStoreScopes: [] },
];

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
    area("a-4", "美容室", "beauty", "", "medium", { x: 0.23, y: 0.685, width: 0.12, height: 0.12 }),
  ]),
  createMockStore(2, "上海静安中心店", "needs_review", [
    area("b-1", "治疗室 1", "treatment", "1", "high", { x: 0.315, y: 0.39, width: 0.095, height: 0.075 }),
    area("b-2", "面诊室 1", "consultation", "1", "low", { x: 0.185, y: 0.68, width: 0.09, height: 0.105 }),
    area("b-3", "面诊室 2", "consultation", "2", "high", { x: 0.345, y: 0.675, width: 0.065, height: 0.1 }),
  ]),
  createMockStore(3, "南京新街口店", "completed", [
    area("c-1", "治疗室 1", "treatment", "1", "high", { x: 0.315, y: 0.295, width: 0.095, height: 0.07 }),
    area("c-2", "美容室 1", "beauty", "1", "high", { x: 0.345, y: 0.675, width: 0.065, height: 0.1 }),
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
        ? [area(`m-${id}-3`, "美容室", "beauty", "", "high", { x: 0.23, y: 0.685, width: 0.12, height: 0.12 })]
        : []),
    ]);
  }),
];

const mockAdapter = {
  async listStores(query: string, page: number, pageSize = PAGE_SIZE, cityFilter = "all"): Promise<StoreListResponse> {
    await delay(160);
    const city = normalizeCityFilter(cityFilter);
    const filtered = mockStores
      .filter((store) => matchesStoreSearch(store.name, query))
      .filter((store) => !city || storeCityName(store.city) === city)
      .sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
    const cityOptions = mockStores.filter((store) => matchesStoreSearch(store.name, query)).map((store) => storeCityName(store.city));
    const start = (page - 1) * pageSize;

    return {
      items: filtered.slice(start, start + pageSize).map(toSummary),
      page,
      pageSize,
      total: filtered.length,
      summary: summarizeStoreSummaries(filtered.map(toSummary)),
      cities: uniqueSorted(cityOptions),
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
        area(`new-${Date.now()}-4`, "美容室", "beauty", "", "medium", {
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

  async updateStoreBasicInfo(payload: UpdateStoreBasicInfoPayload): Promise<StoreDetail> {
    await delay(180);
    const existing = mockStores.find((item) => item.id === payload.id);
    if (!existing) throw new Error("门店不存在");
    const nextStore = {
      ...existing,
      city: payload.city.trim(),
      name: payload.name.trim(),
      externalOrgId: payload.externalOrgId.trim(),
      updatedAt: new Date().toISOString(),
    };
    mockStores = mockStores.map((item) => (item.id === payload.id ? nextStore : item));
    return clone(nextStore);
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

  async probeRecognizeChannel(storeId: number, recorderId: number, channelNo: number): Promise<ProbeRecognizeChannelResult> {
    await delay(420);
    const store = mockStores.find((item) => item.id === storeId);
    const recorder = store?.recorders.find((item) => item.id === recorderId);
    if (!store || !recorder) {
      throw new Error("录像机不存在");
    }
    if (channelNo > 4) {
      return { active: false, message: "模拟通道无画面" };
    }
    const presets = [
      { sceneType: "consultation" as SceneType, areaType: "consultation" as AreaType, areaNumber: "1", bedLabel: "", areaNote: "" },
      { sceneType: "treatment" as SceneType, areaType: "treatment" as AreaType, areaNumber: "2", bedLabel: "", areaNote: "" },
      { sceneType: "beauty" as SceneType, areaType: "beauty" as AreaType, areaNumber: "3", bedLabel: "", areaNote: "" },
      { sceneType: "front_desk" as SceneType, areaType: "" as const, areaNumber: "", bedLabel: "", areaNote: "前台" },
    ];
    const preset = presets[(channelNo - 1) % presets.length];
    const existing = recorder.channels.find((item) => item.channelNo === channelNo);
    const channel: VideoChannel = {
      id: existing?.id ?? Date.now() + channelNo,
      recorderId,
      recorderCode: recorder.deviceCode,
      channelNo,
      channelName: `通道${channelNo}`,
      status: "pending_confirmation",
      thumbnailUrl: `https://picsum.photos/seed/${recorder.deviceCode}-probe-${channelNo}/240/160`,
      fullImageUrl: `https://picsum.photos/seed/${recorder.deviceCode}-probe-${channelNo}/960/640`,
      fullImageExpiresAt: "",
      recognitionAttempts: (existing?.recognitionAttempts ?? 0) + 1,
      recognitionResult: {
        status: "recognized",
        area_type: preset.areaType,
        area_number: preset.areaType ? preset.areaNumber : preset.areaNote,
        confidence: "medium",
        capture_ms: 420,
        recognition_ms: 520,
        total_ms: 940,
      },
      confirmedAt: existing?.confirmedAt,
      ...preset,
    };
    const channels = [...recorder.channels.filter((item) => item.channelNo !== channelNo), channel].sort((a, b) => a.channelNo - b.channelNo);
    replaceMockRecorder(storeId, {
      ...recorder,
      status: "online",
      lastScannedAt: new Date().toISOString(),
      effectiveChannelCount: channels.filter((item) => item.status !== "inactive").length,
      channels,
    });
    return { active: true, channel: clone(channel) };
  },

  async refreshChannelSnapshot(storeId: number, channelId: number): Promise<VideoChannel> {
    return this.recognizeChannel(storeId, channelId);
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
	      bedLabel: isBusiness ? String(patch.bedLabel ?? channel.bedLabel ?? "").trim() : "",
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

const mockResourceViewAdapter = {
  async listStores(query = "", page = 1, pageSize = PAGE_SIZE, cityId: number | "all" = "all"): Promise<ResourceStoreListResponse> {
    await delay(120);
    const filtered = mockStores
      .map(mockStoreToResourceDetail)
      .filter((store) => {
        const queryValue = query.trim().toLowerCase();
        return !queryValue || `${store.storeName} ${store.hospitalName} ${store.tenantId}`.toLowerCase().includes(queryValue);
      })
      .filter((store) => cityId === "all" || store.cityId === cityId)
      .sort((left, right) => left.tenantId - right.tenantId);
    const cityCounts = new Map<number, ResourceCityOption>();
    for (const store of filtered) {
      if (!store.cityId) continue;
      const current = cityCounts.get(store.cityId);
      cityCounts.set(store.cityId, {
        cityId: store.cityId,
        name: store.cityName,
        count: (current?.count ?? 0) + 1,
      });
    }
    const start = (page - 1) * pageSize;
    return {
      items: filtered.slice(start, start + pageSize).map(resourceDetailToSummary),
      page,
      pageSize,
      total: filtered.length,
      summary: summarizeResourceStoreSummaries(filtered.map(resourceDetailToSummary)),
      cities: [...cityCounts.values()].sort((left, right) => left.cityId - right.cityId),
    };
  },

  async getStore(tenantId: number): Promise<ResourceStoreDetail> {
    await delay(100);
    const detail = mockStores.map(mockStoreToResourceDetail).find((store) => store.tenantId === tenantId);
    if (!detail) throw new ApiError(404, "门店不存在");
    return clone(detail);
  },
};

function mockStoreToResourceDetail(store: StoreDetail): ResourceStoreDetail {
  const tenantId = Number(store.externalOrgId.replace(/\D/g, "")) || 10000 + store.id;
  const edge: ResourceDevice = {
    id: tenantId * 10 + 1,
    parentId: 0,
    tenantId,
    name: `${store.name} 工控机`,
    hardwareId: `EDGE-${tenantId}`,
    category: "edge",
    status: 1,
    statusText: "启用",
    onlineStatus: 1,
    onlineText: "在线",
  };
  const nvr: ResourceDevice = {
    id: tenantId * 10 + 2,
    parentId: 0,
    tenantId,
    name: `${store.name} NVR`,
    hardwareId: `NVR-${tenantId}`,
    category: "nvr",
    status: 1,
    statusText: "启用",
    onlineStatus: store.status === "needs_review" ? 2 : 1,
    onlineText: store.status === "needs_review" ? "离线" : "在线",
  };
  const cameras: ResourceCamera[] = store.areas.slice(0, 6).map((areaItem, index) => ({
    id: tenantId * 100 + index + 1,
    parentId: nvr.id,
    tenantId,
    name: `摄像头 ${index + 1}`,
    hardwareId: `NVRCHANNEL:${nvr.id}-${index + 1}`,
    category: "camera",
    status: 1,
    statusText: "启用",
    onlineStatus: 1,
    onlineText: "在线",
    channelNo: index + 1,
    nvrId: nvr.id,
    nvrName: nvr.name,
    spacePaths: [`${areaItem.type || "业务空间"} / ${areaItem.name}`],
  }));
  const spaces: ResourceSpaceNode[] = store.areas.map((areaItem, index) => {
    const camera = cameras[index % Math.max(1, cameras.length)];
    return {
      id: tenantId * 1000 + index + 1,
      tenantId,
      parentId: 0,
      name: areaItem.name,
      code: areaItem.number,
      level: 2,
      status: 1,
      statusText: "启用",
      dictId: 0,
      sortOrder: index + 1,
      boundCameraIds: camera ? [camera.id] : [],
      boundCameraCount: camera ? 1 : 0,
      boundCameras: camera ? [camera] : [],
      children: [],
    };
  });
  const issues: ResourceIssue[] =
    store.status === "needs_review"
      ? [
          {
            severity: "warning",
            type: "offline_nvr",
            message: "NVR 离线，请检查业务库设备状态",
            entityType: "device",
            entityId: nvr.id,
          },
        ]
      : [];
  const summary = summarizeResourceStoreSummaries([
    {
      tenantId,
      storeName: store.name,
      hospitalName: store.name,
      cityId: mockCityId(store.city),
      cityName: store.city || "未分城市",
      edgeCount: 1,
      nvrCount: 1,
      cameraCount: cameras.length,
      spaceCount: store.areas.length,
      consultationCameraCount: store.areas.slice(0, 6).filter((area) => area.type === "consultation").length,
      treatmentCameraCount: store.areas.slice(0, 6).filter((area) => area.type === "treatment" || area.type === "vip_treatment").length,
      boundCameraCount: cameras.length,
      unboundCameraCount: 0,
      offlineDeviceCount: store.status === "needs_review" ? 1 : 0,
      warningCount: issues.length,
      camerasFullyBound: cameras.length > 0,
      updatedAt: store.updatedAt,
      canViewMonitor: store.canViewMonitor,
      monitorUrl: store.canViewMonitor ? `/erzhuang-project/h5/orgs/${tenantId}/monitor` : undefined,
    },
  ]);
  return {
    tenantId,
    storeName: store.name,
    hospitalName: store.name,
    cityId: mockCityId(store.city),
    cityName: store.city || "未分城市",
    camerasFullyBound: cameras.length > 0,
    updatedAt: store.updatedAt,
    summary,
    edges: [edge],
    nvrs: [nvr],
    cameras,
    spaces,
    relations: spaces.flatMap((space) => space.boundCameraIds.map((cameraId) => ({ id: cameraId + space.id, deviceId: cameraId, areaId: space.id, functionType: "camera" }))),
    spaceTree: spaces,
    deviceTree: { edges: [edge], nvrs: [{ ...nvr, cameras }] },
    issues,
    canViewMonitor: store.canViewMonitor,
    monitorUrl: store.canViewMonitor ? `/erzhuang-project/h5/orgs/${tenantId}/monitor` : undefined,
  };
}

function resourceDetailToSummary(detail: ResourceStoreDetail): ResourceStoreSummary {
  return {
    tenantId: detail.tenantId,
    storeName: detail.storeName,
    hospitalName: detail.hospitalName,
    cityId: detail.cityId,
    cityName: detail.cityName,
    edgeCount: detail.summary.edgeCount,
    nvrCount: detail.summary.nvrCount,
    cameraCount: detail.summary.cameraCount,
    spaceCount: detail.summary.spaceCount,
    consultationCameraCount: detail.summary.consultationCameraCount,
    treatmentCameraCount: detail.summary.treatmentCameraCount,
    boundCameraCount: detail.summary.boundCameraCount,
    unboundCameraCount: detail.summary.unboundCameraCount,
    offlineDeviceCount: detail.summary.offlineDeviceCount,
    warningCount: detail.summary.warningCount,
    camerasFullyBound: detail.camerasFullyBound,
    updatedAt: detail.updatedAt,
    canViewMonitor: detail.canViewMonitor,
    monitorUrl: detail.monitorUrl,
  };
}

function mockCityId(city: string) {
  const known: Record<string, number> = {
    北京: 1,
    上海: 9,
    杭州: 175,
    广州: 289,
    深圳: 291,
    成都: 385,
    南京: 125,
  };
  return known[city] ?? 0;
}

const httpAdapter = {
  async listStores(query: string, page: number, pageSize = PAGE_SIZE, cityFilter = "all"): Promise<StoreListResponse> {
    const search = storeListSearchParams({ query, cityFilter, page, pageSize });
    const response = await requestJSON<BackendStoreListResponse>(`${API_BASE}/stores?${search.toString()}`);
    const items = response.items.map(mapBackendSummary);
    return {
      items,
      page: response.page,
      pageSize: response.page_size,
      total: response.total,
      summary: summarizeStoreSummaries(items),
      cities: response.cities ?? response.cityOptions ?? uniqueSorted(items.map((item) => storeCityName(item.city))),
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
  async getAISettings(): Promise<AISettings> {
    return requestJSON<AISettings>(`${APP_API_BASE}/ai-settings`);
  },

  async toggleAISettings(): Promise<AISettings> {
    return requestJSON<AISettings>(`${APP_API_BASE}/ai-settings/toggle`, { method: "POST" });
  },

  async getMonitorScreenshotWatermarkSettings(): Promise<MonitorScreenshotWatermarkSettings> {
    return requestJSON<MonitorScreenshotWatermarkSettings>(`${APP_API_BASE}/monitor-screenshot-watermark-settings`);
  },

  async setMonitorScreenshotWatermarkSettings(enabled: boolean): Promise<MonitorScreenshotWatermarkSettings> {
    return requestJSON<MonitorScreenshotWatermarkSettings>(`${APP_API_BASE}/monitor-screenshot-watermark-settings`, {
      method: "POST",
      body: JSON.stringify({ enabled }),
    });
  },

  async getAuthMe(): Promise<AuthState> {
    return requestJSON<AuthState>(`${APP_API_BASE}/auth/me`);
  },

  async logout(): Promise<void> {
    await requestJSON<void>(`${APP_API_BASE}/auth/logout`, { method: "POST" });
  },

  async listUsers(): Promise<ManagedUser[]> {
    const response = await requestJSON<{ users: BackendManagedUser[] }>(`${APP_API_BASE}/users`);
    return (response.users ?? []).map(mapManagedUser);
  },

  async createUser(payload: ManagedUserPayload): Promise<ManagedUser> {
    const response = await requestJSON<BackendManagedUser>(`${APP_API_BASE}/users`, {
      method: "POST",
      body: JSON.stringify(toManagedUserPayload(payload)),
    });
    return mapManagedUser(response);
  },

  async updateUser(id: number, payload: ManagedUserPayload): Promise<ManagedUser> {
    const response = await requestJSON<BackendManagedUser>(`${APP_API_BASE}/users/${id}`, {
      method: "PUT",
      body: JSON.stringify(toManagedUserPayload(payload)),
    });
    return mapManagedUser(response);
  },

  async listMonitorStoreScopeCandidates(): Promise<MonitorStoreScope[]> {
    const response = await requestJSON<{ stores: BackendMonitorStoreScope[] }>(`${APP_API_BASE}/users/monitor-store-scope-candidates`);
    return (response.stores ?? []).map(mapMonitorStoreScope);
  },

  async listAuditLogs(startDate: string, endDate: string, userId = "", action = "", page = 1): Promise<AuditLogPage> {
    const params = new URLSearchParams({ start_time: startDate, end_time: endDate, page: String(page), page_size: "20" });
    if (userId) params.set("user_id", userId);
    if (action) params.set("action", action);
    const response = await requestJSON<{ items?: BackendAuditLogItem[]; page?: number; page_size?: number; total?: number }>(`${APP_API_BASE}/admin/audit-logs?${params.toString()}`);
    return {
      items: (response.items ?? []).map((item) => ({
        actorDisplayName: item.actor_display_name ?? "",
        userEmail: item.user_email ?? "",
        action: item.action ?? "",
        entityType: item.entity_type ?? "",
        entityId: item.entity_id,
        externalOrgId: item.external_org_id ?? "",
        result: item.result ?? "",
        createdAt: item.created_at ?? "",
        summary: item.summary ?? "",
      })),
      page: response.page ?? page,
      pageSize: response.page_size ?? 20,
      total: response.total ?? 0,
    };
  },

  async createStore(payload: CreateStoreSpacePayload): Promise<StoreDetail> {
    const response = await requestJSON<BackendStoreSpaceDetail>(`${STORE_SPACE_API_BASE}/stores`, {
      method: "POST",
      body: JSON.stringify(toStoreSpaceCreatePayload(payload)),
    });
    return mapStoreSpaceDetail(response);
  },

  async updateStoreBasicInfo(payload: UpdateStoreBasicInfoPayload): Promise<StoreDetail> {
    const response = await requestJSON<BackendStoreSpaceDetail>(`${STORE_SPACE_API_BASE}/stores/${payload.id}`, {
      method: "PATCH",
      body: JSON.stringify(toStoreSpaceBasicInfoPayload(payload)),
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

  async listStores(query: string, page: number, pageSize = PAGE_SIZE, cityFilter = "all"): Promise<StoreListResponse> {
    const search = storeListSearchParams({ query, cityFilter, page, pageSize });
    const response = await requestJSON<BackendStoreSpaceListResponse>(`${STORE_SPACE_API_BASE}/stores?${search.toString()}`);
    const items = response.items.map(mapStoreSpaceSummary);
    return {
      items,
      page: response.page,
      pageSize: response.page_size ?? response.pageSize ?? pageSize,
      total: response.total,
      summary: response.summary ? mapStoreListSummary(response.summary) : summarizeStoreSummaries(items),
      cities: response.cities ?? response.cityOptions ?? uniqueSorted(items.map((item) => storeCityName(item.city))),
    };
  },

  async getStore(id: number): Promise<StoreDetail> {
    const response = await requestJSON<BackendStoreSpaceDetail>(`${STORE_SPACE_API_BASE}/stores/${id}`);
    return mapStoreSpaceDetail(response);
  },

  async getStoreDesignPlanData(id: number): Promise<StoreDetail> {
    const response = await requestJSON<BackendStoreSpaceDetail>(`${STORE_SPACE_API_BASE}/stores/${id}/design-plan-data`);
    return mapStoreSpaceDetail(response);
  },

  async getStoreChannelData(id: number): Promise<StoreDetail> {
    const response = await requestJSON<BackendStoreSpaceDetail>(`${STORE_SPACE_API_BASE}/stores/${id}/channel-data`);
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

  async probeRecognizeChannel(recorderId: number, recorderCode: string, channelNo: number): Promise<ProbeRecognizeChannelResult> {
    const response = await requestJSON<BackendProbeRecognizeChannelResult>(`${STORE_SPACE_API_BASE}/recorders/${recorderId}/probe-recognize-channel`, {
      method: "POST",
      body: JSON.stringify({ channel_no: channelNo }),
    });
    return {
      active: Boolean(response.active),
      channel: response.channel ? mapBackendChannel(response.channel, recorderId, recorderCode) : undefined,
      message: response.message ?? "",
    };
  },

  async refreshChannelSnapshot(channelId: number): Promise<VideoChannel> {
    const response = await requestJSON<BackendVideoChannel>(`${STORE_SPACE_API_BASE}/channels/${channelId}/snapshot`, {
      method: "POST",
    });
    return mapBackendChannel(response, 0, "");
  },

  async recordChannelSnapshotView(storeId: number, channelId: number): Promise<void> {
    await requestJSON<void>(`${STORE_SPACE_API_BASE}/stores/${storeId}/channels/${channelId}/snapshot/view`, {
      method: "POST",
    });
  },

  async diagnoseChannelSnapshot(snapshotName: string): Promise<SnapshotDiagnostics> {
    const response = await requestJSON<BackendSnapshotDiagnostics>(
      `${STORE_SPACE_API_BASE}/channel-snapshots/${encodeURIComponent(snapshotName)}/diagnostics`,
    );
    return mapBackendSnapshotDiagnostics(response);
  },

  async getLiveAddress(payload: LiveAddressPayload): Promise<LiveAddressResult> {
    const response = await requestJSON<BackendLiveAddressResult>(`${STORE_SPACE_API_BASE}/diagnostics/ezviz/live-address`, {
      method: "POST",
      body: JSON.stringify({
        ezviz_account_id: payload.ezvizAccountId ? Number(payload.ezvizAccountId) : 0,
        account_name: payload.accountName?.trim() ?? "",
        device_serial: payload.deviceSerial.trim(),
        channel_no: Number(payload.channelNo),
        code: payload.code?.trim() ?? "",
      }),
    });
    return mapBackendLiveAddress(response);
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

  async exportChannelMappings(storeId: number): Promise<void> {
    await downloadFile(`${STORE_SPACE_API_BASE}/stores/${storeId}/channel-mappings/export.xlsx`);
  },
};

const resourceViewHttpAdapter = {
  async listStores(query = "", page = 1, pageSize = PAGE_SIZE, cityId: number | "all" = "all"): Promise<ResourceStoreListResponse> {
    const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
    const cleanQuery = query.trim();
    if (cleanQuery) params.set("q", cleanQuery);
    if (cityId !== "all") params.set("city_id", String(cityId));
    const response = await requestJSON<BackendResourceStoreListResponse>(`${resourceViewStoresPath()}?${params.toString()}`);
    return mapResourceStoreListResponse(response, page, pageSize);
  },

  async getStore(tenantId: number): Promise<ResourceStoreDetail> {
    const response = await requestJSON<BackendResourceStoreDetail>(resourceViewStorePath(tenantId));
    return mapResourceStoreDetail(response);
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

  async listStores(query: string, page: number, pageSize = PAGE_SIZE, cityFilter = "all"): Promise<StoreListResponse> {
    return withFallback(
      () => httpAdapter.listStores(query, page, pageSize, cityFilter),
      () => mockAdapter.listStores(query, page, pageSize, cityFilter),
    );
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

  async getAISettings(): Promise<AISettings> {
    if (API_MODE === "mock") {
      return mockAISettings;
    }
    return storeSpaceHttpAdapter.getAISettings();
  },

  async toggleAISettings(): Promise<AISettings> {
    if (API_MODE === "mock") {
      mockAISettings =
        mockAISettings.provider === "minimax"
          ? { provider: "openai", model: "gpt-5.5", label: "OpenAI / gpt-5.5" }
          : { provider: "minimax", model: "MiniMax-M3", label: "MiniMax / MiniMax-M3" };
      return clone(mockAISettings);
    }
    return storeSpaceHttpAdapter.toggleAISettings();
  },

  async getMonitorScreenshotWatermarkSettings(): Promise<MonitorScreenshotWatermarkSettings> {
    if (API_MODE === "mock") return { enabled: true };
    return storeSpaceHttpAdapter.getMonitorScreenshotWatermarkSettings();
  },

  async setMonitorScreenshotWatermarkSettings(enabled: boolean): Promise<MonitorScreenshotWatermarkSettings> {
    if (API_MODE === "mock") return { enabled };
    return storeSpaceHttpAdapter.setMonitorScreenshotWatermarkSettings(enabled);
  },

  async getAuthMe(): Promise<AuthState> {
    if (API_MODE === "mock") {
      return {
        enabled: false,
        authenticated: true,
        user: { email: "local-admin@example.com", username: "local-admin", display_name: "本地管理员", role: "admin" },
        permissions: ["admin"],
      };
    }
    return storeSpaceHttpAdapter.getAuthMe();
  },

  async logout(): Promise<void> {
    if (API_MODE === "mock") return;
    return storeSpaceHttpAdapter.logout();
  },

  async listUsers(): Promise<ManagedUser[]> {
    if (API_MODE === "mock") {
      return clone(mockManagedUsers);
    }
    return storeSpaceHttpAdapter.listUsers();
  },

  async createUser(payload: ManagedUserPayload): Promise<ManagedUser> {
    if (API_MODE === "mock") {
      const email = (payload.email ?? "").trim().toLowerCase();
      if (!email) throw new ApiError(400, "企业邮箱不能为空");
      if (mockManagedUsers.some((user) => user.email === email)) throw new ApiError(400, "用户已存在");
      const user: ManagedUser = {
        id: Math.max(0, ...mockManagedUsers.map((item) => item.id)) + 1,
        email,
        username: payload.username.trim() || email.split("@")[0],
        displayName: payload.displayName.trim(),
        role: payload.role,
        enabled: payload.enabled,
        monitorStoreScopeCount: payload.monitorStoreScopeIds.length,
        monitorStoreScopes: mockMonitorScopesByIds(payload.monitorStoreScopeIds),
      };
      mockManagedUsers = [...mockManagedUsers, user];
      return clone(user);
    }
    return storeSpaceHttpAdapter.createUser(payload);
  },

  async updateUser(id: number, payload: ManagedUserPayload): Promise<ManagedUser> {
    if (API_MODE === "mock") {
      const index = mockManagedUsers.findIndex((user) => user.id === id);
      if (index < 0) throw new ApiError(404, "用户不存在");
      const user: ManagedUser = {
        ...mockManagedUsers[index],
        username: payload.username.trim(),
        displayName: payload.displayName.trim(),
        role: payload.role,
        enabled: payload.enabled,
        monitorStoreScopeCount: payload.monitorStoreScopeIds.length,
        monitorStoreScopes: mockMonitorScopesByIds(payload.monitorStoreScopeIds),
      };
      mockManagedUsers = mockManagedUsers.map((item) => (item.id === id ? user : item));
      return clone(user);
    }
    return storeSpaceHttpAdapter.updateUser(id, payload);
  },

  async listMonitorStoreScopeCandidates(): Promise<MonitorStoreScope[]> {
    if (API_MODE === "mock") {
      return mockStores
        .filter((store) => store.externalOrgId.trim())
        .map((store) => ({ storeId: store.id, city: store.city, name: store.name, externalOrgId: store.externalOrgId }));
    }
    return storeSpaceHttpAdapter.listMonitorStoreScopeCandidates();
  },

  async listAuditLogs(startDate: string, endDate: string, userId = "", action = "", page = 1): Promise<AuditLogPage> {
    if (API_MODE === "mock") {
      return { items: [], page, pageSize: 20, total: 0 };
    }
    return storeSpaceHttpAdapter.listAuditLogs(startDate, endDate, userId, action, page);
  },

  async listStores(query: string, page: number, pageSize = PAGE_SIZE, cityFilter = "all"): Promise<StoreListResponse> {
    if (API_MODE === "mock") {
      return mockAdapter.listStores(query, page, pageSize, cityFilter);
    }
    return storeSpaceHttpAdapter.listStores(query, page, pageSize, cityFilter);
  },

  async getStore(id: number): Promise<StoreDetail> {
    if (API_MODE === "mock") {
      return mockAdapter.getStore(id);
    }
    return storeSpaceHttpAdapter.getStore(id);
  },

  async getStoreDesignPlanData(id: number): Promise<StoreDetail> {
    if (API_MODE === "mock") {
      const detail = await mockAdapter.getStore(id);
      return { ...detail, recorders: [] };
    }
    return storeSpaceHttpAdapter.getStoreDesignPlanData(id);
  },

  async getStoreChannelData(id: number): Promise<StoreDetail> {
    if (API_MODE === "mock") {
      const detail = await mockAdapter.getStore(id);
      return {
        ...detail,
        fileName: "",
        originalPath: "",
        previewPath: "",
        thumbnailPath: "",
        pageCount: 0,
        previewUrl: "",
        areas: [],
      };
    }
    return storeSpaceHttpAdapter.getStoreChannelData(id);
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

  async updateStoreBasicInfo(payload: UpdateStoreBasicInfoPayload): Promise<StoreDetail> {
    if (API_MODE === "mock") {
      return mockAdapter.updateStoreBasicInfo(payload);
    }
    return storeSpaceHttpAdapter.updateStoreBasicInfo(payload);
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

  async probeRecognizeChannel(storeId: number, recorder: VideoRecorder, channelNo: number): Promise<ProbeRecognizeChannelResult> {
    if (API_MODE === "mock") {
      return mockAdapter.probeRecognizeChannel(storeId, recorder.id, channelNo);
    }
    return storeSpaceHttpAdapter.probeRecognizeChannel(recorder.id, recorder.deviceCode, channelNo);
  },

  async refreshChannelSnapshot(storeId: number, channelId: number): Promise<VideoChannel> {
    if (API_MODE === "mock") {
      return mockAdapter.refreshChannelSnapshot(storeId, channelId);
    }
    return storeSpaceHttpAdapter.refreshChannelSnapshot(channelId);
  },

  async recordChannelSnapshotView(storeId: number, channelId: number): Promise<void> {
    if (API_MODE === "mock") return;
    return storeSpaceHttpAdapter.recordChannelSnapshotView(storeId, channelId);
  },

  async diagnoseChannelSnapshot(snapshotName: string): Promise<SnapshotDiagnostics> {
    if (API_MODE === "mock") {
      return {
        code: "snapshot_open_ok",
        stage: "open_snapshot",
        assetStore: "mock",
        snapshotName,
        snapshotKey: snapshotName ? `channel-snapshots/${snapshotName}` : "",
        exists: true,
      };
    }
    return storeSpaceHttpAdapter.diagnoseChannelSnapshot(snapshotName);
  },

  async getLiveAddress(payload: LiveAddressPayload): Promise<LiveAddressResult> {
    if (API_MODE === "mock") {
      return {
        url: "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8",
        urlId: "mock-url-id",
        expireTime: "mock",
        protocol: "hls",
      };
    }
    return storeSpaceHttpAdapter.getLiveAddress(payload);
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

  async exportChannelMappings(storeId: number): Promise<void> {
    if (API_MODE === "mock") {
      const store = mockStores.find((item) => item.id === storeId);
      const fileName = `${store?.name ?? "门店"}-通道映射确认表-mock.xlsx`;
      triggerDownload(new Blob(["mock channel mapping export"], { type: channelMappingExcelMime }), fileName);
      return;
    }
    return storeSpaceHttpAdapter.exportChannelMappings(storeId);
  },
};

export const storeSpaceResourceViewApi = {
  endpoints: {
    base: RESOURCE_VIEW_API_BASE,
    stores: resourceViewStoresPath(),
    store: resourceViewStorePath,
  },

  async listStores(query = "", page = 1, pageSize = PAGE_SIZE, cityId: number | "all" = "all"): Promise<ResourceStoreListResponse> {
    if (API_MODE === "mock") {
      return mockResourceViewAdapter.listStores(query, page, pageSize, cityId);
    }
    return resourceViewHttpAdapter.listStores(query, page, pageSize, cityId);
  },

  async getStore(tenantId: number): Promise<ResourceStoreDetail> {
    if (API_MODE === "mock") {
      return mockResourceViewAdapter.getStore(tenantId);
    }
    return resourceViewHttpAdapter.getStore(tenantId);
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
    ...options,
    credentials: "include",
    headers: {
      Accept: "application/json",
      ...(options.body && !(options.body instanceof FormData) ? { "Content-Type": "application/json" } : {}),
      ...options.headers,
    },
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
    const code = typeof data === "object" && data && "code" in data ? String(data.code) : "";
    const stage = typeof data === "object" && data && "stage" in data ? String(data.stage) : "";
    const detail = typeof data === "object" && data && "detail" in data ? String(data.detail) : "";
    throw new ApiError(response.status, message, fields, code, stage, detail);
  }

  return data as T;
}

const channelMappingExcelMime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";

async function downloadFile(url: string): Promise<void> {
  const response = await fetch(url, { credentials: "include", headers: { Accept: channelMappingExcelMime } });
  if (!response.ok) {
    const contentType = response.headers.get("content-type") ?? "";
    if (contentType.includes("application/json")) {
      const data = await response.json();
      const fields = data && typeof data === "object" && "fields" in data && isStringRecord(data.fields) ? data.fields : {};
      const message =
        data && typeof data === "object" && "message" in data
          ? String(data.message)
          : data && typeof data === "object" && "error" in data
            ? String(data.error)
            : `HTTP ${response.status}`;
      const code = data && typeof data === "object" && "code" in data ? String(data.code) : "";
      const stage = data && typeof data === "object" && "stage" in data ? String(data.stage) : "";
      const detail = data && typeof data === "object" && "detail" in data ? String(data.detail) : "";
      throw new ApiError(response.status, message, fields, code, stage, detail);
    }
    throw new ApiError(response.status, `HTTP ${response.status}`);
  }
  const blob = await response.blob();
  triggerDownload(blob, fileNameFromDisposition(response.headers.get("content-disposition")) ?? "通道映射确认表.xlsx");
}

function triggerDownload(blob: Blob, fileName: string) {
  const href = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = href;
  link.download = fileName;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(href);
}

function fileNameFromDisposition(value: string | null): string | null {
  if (!value) return null;
  const encoded = value.match(/filename\\*=UTF-8''([^;]+)/i)?.[1];
  if (encoded) {
    try {
      return decodeURIComponent(encoded);
    } catch {
      return encoded;
    }
  }
  return value.match(/filename=\"?([^\";]+)\"?/i)?.[1] ?? null;
}

function mapBackendSummary(item: BackendStoreSummary): StoreSummary {
  return {
    id: item.id,
    city: item.city ?? "",
    name: item.name,
    shortName: item.short_name ?? item.shortName ?? "",
    externalOrgId: "",
    thumbnailUrl: toDisplayImageUrl(item.thumbnail_url),
    designPlanStatus: item.thumbnail_url ? "completed" : "not_uploaded",
    recorderCount: 0,
    channelCount: 0,
    channelsFullyConfirmed: false,
    treatmentCount: item.treatment_count,
    consultationCount: item.consultation_count,
    beautyCount: item.beauty_count,
    areaCount: item.area_count,
    status: item.status,
    updatedAt: item.updated_at,
    canViewMonitor: false,
  };
}

function mapManagedUser(user: BackendManagedUser): ManagedUser {
  const scopes = (user.monitor_store_scopes ?? user.monitorStoreScopes ?? []).map(mapMonitorStoreScope);
  return {
    id: user.id,
    email: user.email,
    username: user.username ?? "",
    displayName: user.display_name ?? user.displayName ?? "",
    role: normalizeManagedRole(user.role),
    enabled: user.enabled ?? false,
    lastLoginAt: user.last_login_at ?? user.lastLoginAt,
    monitorStoreScopeCount: user.monitor_store_scope_count ?? user.monitorStoreScopeCount ?? scopes.length,
    monitorStoreScopes: scopes,
  };
}

function mapMonitorStoreScope(scope: BackendMonitorStoreScope): MonitorStoreScope {
  return {
    storeId: Number(scope.store_id ?? scope.storeId ?? 0),
    city: scope.city ?? "",
    name: scope.name ?? "",
    externalOrgId: scope.external_org_id ?? scope.externalOrgId ?? "",
  };
}

function normalizeManagedRole(role: string | undefined): ManagedUserRole {
  if (role === "admin" || role === "editor" || role === "viewer") {
    return role;
  }
  return "viewer";
}

function toManagedUserPayload(payload: ManagedUserPayload) {
  return {
    email: payload.email,
    username: payload.username,
    display_name: payload.displayName,
    role: payload.role,
    enabled: payload.enabled,
    monitor_store_scope_ids: payload.monitorStoreScopeIds,
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
    shortName: store.short_name ?? store.shortName ?? "",
    fileName: displayPlanFileName(store.pdf_file_name, store.original_pdf_path, store.name),
    originalPath: store.original_pdf_path || MOCK_ORIGINAL_PDF_PATH,
    previewPath: store.preview_image_path || MOCK_PREVIEW_IMAGE_PATH,
    thumbnailPath: store.thumbnail_path || MOCK_THUMBNAIL_PATH,
    pageCount: store.page_count || 1,
    thumbnailUrl: toDisplayImageUrl(store.thumbnail_url || store.thumbnail_path),
    previewUrl: toDisplayImageUrl(store.preview_url || store.preview_image_path),
    externalOrgId: "",
    canViewMonitor: false,
    designPlanStatus: store.preview_url || store.preview_image_path ? "completed" : "not_uploaded",
    recorderCount: 0,
    channelCount: 0,
    channelsFullyConfirmed: false,
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
  const externalOrgId = store.external_org_id ?? store.externalOrgId ?? "";
  return {
    id: store.id,
    city: store.city ?? store.cityName ?? "",
    name: store.name,
    shortName: store.short_name ?? store.shortName ?? "",
    externalOrgId,
    canViewMonitor: store.can_view_monitor ?? store.canViewMonitor ?? Boolean(externalOrgId),
    thumbnailUrl: "",
    designPlanStatus: store.design_plan_status ?? store.designPlanStatus ?? "not_uploaded",
    recorderCount: store.recorder_count ?? store.recorderCount ?? 0,
    channelCount: store.channel_count ?? store.channelCount ?? 0,
    channelsFullyConfirmed: store.channels_fully_confirmed ?? store.channelsFullyConfirmed ?? false,
    treatmentCount: store.treatment_count ?? store.treatmentCount ?? 0,
    consultationCount: store.consultation_count ?? store.consultationCount ?? 0,
    beautyCount: store.beauty_count ?? store.beautyCount ?? 0,
    areaCount: store.area_count ?? store.areaCount ?? 0,
    status: mapStoreSpaceOverallStatus(store.overall_status ?? store.overallStatus),
    updatedAt: store.updated_at ?? store.updatedAt ?? new Date().toISOString(),
  };
}

function mapStoreListSummary(summary: BackendStoreListSummary): StoreListSummary {
  return {
    storeCount: summary.store_count ?? summary.storeCount ?? 0,
    treatmentCount: summary.treatment_count ?? summary.treatmentCount ?? 0,
    consultationCount: summary.consultation_count ?? summary.consultationCount ?? 0,
    beautyCount: summary.beauty_count ?? summary.beautyCount ?? 0,
  };
}

function summarizeStoreSummaries(stores: StoreSummary[]): StoreListSummary {
  return stores.reduce(
    (summary, store) => ({
      storeCount: summary.storeCount + 1,
      consultationCount: summary.consultationCount + (store.consultationCount ?? 0),
      treatmentCount: summary.treatmentCount + (store.treatmentCount ?? 0),
      beautyCount: summary.beautyCount + (store.beautyCount ?? 0),
    }),
    { storeCount: 0, consultationCount: 0, treatmentCount: 0, beautyCount: 0 },
  );
}

function mapResourceStoreListResponse(response: BackendResourceStoreListResponse, fallbackPage = 1, fallbackPageSize = PAGE_SIZE): ResourceStoreListResponse {
  const items = (response.items ?? []).map(mapResourceStoreSummary);
  return {
    items,
    page: response.page ?? fallbackPage,
    pageSize: response.page_size ?? response.pageSize ?? fallbackPageSize,
    total: response.total ?? items.length,
    summary: response.summary ? mapResourceStoreListSummary(response.summary) : summarizeResourceStoreSummaries(items),
    cities: (response.cities ?? []).map(mapResourceCityOption),
  };
}

function mapResourceCityOption(option: BackendResourceCityOption): ResourceCityOption {
  return {
    cityId: option.city_id ?? option.cityId ?? 0,
    name: option.name ?? "",
    count: option.count ?? 0,
  };
}

function mapResourceStoreSummary(store: BackendResourceStoreSummary): ResourceStoreSummary {
  return {
    tenantId: store.tenant_id ?? store.tenantId ?? 0,
    storeName: store.store_name ?? store.storeName ?? "",
    hospitalName: store.hospital_name ?? store.hospitalName ?? "",
    cityId: store.city_id ?? store.cityId ?? 0,
    cityName: store.city_name ?? store.cityName ?? "",
    edgeCount: store.edge_count ?? store.edgeCount ?? 0,
    nvrCount: store.nvr_count ?? store.nvrCount ?? 0,
    cameraCount: store.camera_count ?? store.cameraCount ?? 0,
    spaceCount: store.space_count ?? store.spaceCount ?? 0,
    consultationCameraCount: store.consultation_camera_count ?? store.consultationCameraCount ?? 0,
    treatmentCameraCount: store.treatment_camera_count ?? store.treatmentCameraCount ?? 0,
    boundCameraCount: store.bound_camera_count ?? store.boundCameraCount ?? 0,
    unboundCameraCount: store.unbound_camera_count ?? store.unboundCameraCount ?? 0,
    offlineDeviceCount: store.offline_device_count ?? store.offlineDeviceCount ?? 0,
    warningCount: store.warning_count ?? store.warningCount ?? 0,
    camerasFullyBound: store.cameras_fully_bound ?? store.camerasFullyBound ?? false,
    updatedAt: store.updated_at ?? store.updatedAt ?? "",
    canViewMonitor: store.can_view_monitor ?? store.canViewMonitor ?? false,
    monitorUrl: store.monitor_url ?? store.monitorUrl,
  };
}

function mapResourceStoreListSummary(summary: BackendResourceStoreListSummary): ResourceStoreListSummary {
  return {
    storeCount: summary.store_count ?? summary.storeCount ?? 0,
    edgeCount: summary.edge_count ?? summary.edgeCount ?? 0,
    nvrCount: summary.nvr_count ?? summary.nvrCount ?? 0,
    cameraCount: summary.camera_count ?? summary.cameraCount ?? 0,
    spaceCount: summary.space_count ?? summary.spaceCount ?? 0,
    consultationCameraCount: summary.consultation_camera_count ?? summary.consultationCameraCount ?? 0,
    treatmentCameraCount: summary.treatment_camera_count ?? summary.treatmentCameraCount ?? 0,
    boundCameraCount: summary.bound_camera_count ?? summary.boundCameraCount ?? 0,
    unboundCameraCount: summary.unbound_camera_count ?? summary.unboundCameraCount ?? 0,
    offlineDeviceCount: summary.offline_device_count ?? summary.offlineDeviceCount ?? 0,
    warningCount: summary.warning_count ?? summary.warningCount ?? 0,
  };
}

function summarizeResourceStoreSummaries(stores: ResourceStoreSummary[]): ResourceStoreListSummary {
  return stores.reduce(
    (summary, store) => ({
      storeCount: summary.storeCount + 1,
      edgeCount: summary.edgeCount + store.edgeCount,
      nvrCount: summary.nvrCount + store.nvrCount,
      cameraCount: summary.cameraCount + store.cameraCount,
      spaceCount: summary.spaceCount + store.spaceCount,
      consultationCameraCount: summary.consultationCameraCount + store.consultationCameraCount,
      treatmentCameraCount: summary.treatmentCameraCount + store.treatmentCameraCount,
      boundCameraCount: summary.boundCameraCount + store.boundCameraCount,
      unboundCameraCount: summary.unboundCameraCount + store.unboundCameraCount,
      offlineDeviceCount: summary.offlineDeviceCount + store.offlineDeviceCount,
      warningCount: summary.warningCount + store.warningCount,
    }),
    {
      storeCount: 0,
      edgeCount: 0,
      nvrCount: 0,
      cameraCount: 0,
      spaceCount: 0,
      consultationCameraCount: 0,
      treatmentCameraCount: 0,
      boundCameraCount: 0,
      unboundCameraCount: 0,
      offlineDeviceCount: 0,
      warningCount: 0,
    },
  );
}

function mapResourceStoreDetail(store: BackendResourceStoreDetail): ResourceStoreDetail {
  return {
    ...mapResourceStoreSummary(store),
    summary: store.summary ? mapResourceStoreListSummary(store.summary) : summarizeResourceStoreSummaries([mapResourceStoreSummary(store)]),
    edges: (store.edges ?? []).map(mapResourceDevice),
    nvrs: (store.nvrs ?? []).map(mapResourceDevice),
    cameras: (store.cameras ?? []).map(mapResourceCamera),
    spaces: (store.spaces ?? []).map(mapResourceSpace),
    relations: (store.relations ?? []).map(mapResourceRelation),
    spaceTree: (store.space_tree ?? store.spaceTree ?? []).map(mapResourceSpaceNode),
    deviceTree: mapResourceDeviceTree(store.device_tree ?? store.deviceTree),
    issues: (store.issues ?? []).map(mapResourceIssue),
  };
}

function mapResourceDevice(device: BackendResourceDevice): ResourceDevice {
  return {
    id: device.id ?? 0,
    parentId: device.parent_id ?? device.parentId ?? 0,
    tenantId: device.tenant_id ?? device.tenantId ?? 0,
    name: device.name ?? "",
    hardwareId: device.hardware_id ?? device.hardwareId ?? "",
    sn: device.sn,
    ip: device.ip,
    category: device.category ?? "",
    provider: device.provider,
    status: device.status ?? 0,
    statusText: device.status_text ?? device.statusText ?? "",
    onlineStatus: device.online_status ?? device.onlineStatus ?? 0,
    onlineText: device.online_text ?? device.onlineText ?? "",
    extSummary: device.ext_summary ?? device.extSummary,
    heartbeatAt: device.heartbeat_at ?? device.heartbeatAt,
  };
}

function mapResourceCamera(camera: BackendResourceCamera): ResourceCamera {
  return {
    ...mapResourceDevice(camera),
    channelNo: camera.channel_no ?? camera.channelNo,
    nvrId: camera.nvr_id ?? camera.nvrId ?? 0,
    nvrName: camera.nvr_name ?? camera.nvrName,
    spacePaths: camera.space_paths ?? camera.spacePaths ?? [],
    thumbnailKind: camera.thumbnail_kind ?? camera.thumbnailKind,
    thumbnailUrl: toDisplayImageUrl(camera.thumbnail_url ?? camera.thumbnailUrl),
  };
}

function mapResourceSpace(space: BackendResourceSpace): ResourceSpace {
  return {
    id: space.id ?? 0,
    tenantId: space.tenant_id ?? space.tenantId ?? 0,
    parentId: space.parent_id ?? space.parentId ?? 0,
    name: space.name ?? "",
    code: space.code,
    level: space.level ?? 0,
    status: space.status ?? 0,
    statusText: space.status_text ?? space.statusText ?? "",
    dictId: space.dict_id ?? space.dictId ?? 0,
    sortOrder: space.sort_order ?? space.sortOrder ?? 0,
    boundCameraIds: space.bound_camera_ids ?? space.boundCameraIds ?? [],
    boundCameraCount: space.bound_camera_count ?? space.boundCameraCount ?? 0,
  };
}

function mapResourceRelation(relation: BackendResourceAreaDeviceRelation): ResourceAreaDeviceRelation {
  return {
    id: relation.id ?? 0,
    deviceId: relation.device_id ?? relation.deviceId ?? 0,
    areaId: relation.area_id ?? relation.areaId ?? 0,
    functionType: relation.function_type ?? relation.functionType ?? "",
  };
}

function mapResourceSpaceNode(node: BackendResourceSpaceNode): ResourceSpaceNode {
  return {
    ...mapResourceSpace(node),
    boundCameras: (node.bound_cameras ?? node.boundCameras ?? []).map(mapResourceCamera),
    children: (node.children ?? []).map(mapResourceSpaceNode),
  };
}

function mapResourceDeviceTree(tree: BackendResourceDeviceTree | undefined): ResourceDeviceTree {
  return {
    edges: (tree?.edges ?? []).map(mapResourceDevice),
    nvrs: (tree?.nvrs ?? []).map((nvr) => ({
      ...mapResourceDevice(nvr),
      cameras: (nvr.cameras ?? []).map(mapResourceCamera),
    })),
  };
}

function mapResourceIssue(issue: BackendResourceIssue): ResourceIssue {
  return {
    severity: issue.severity ?? "info",
    type: issue.type ?? "unbound_camera",
    message: issue.message ?? "",
    entityType: issue.entity_type ?? issue.entityType ?? "",
    entityId: issue.entity_id ?? issue.entityId ?? 0,
  };
}

function mapStoreSpaceDetail(store: BackendStoreSpaceDetail): StoreDetail {
  const designPlan = firstStoreSpaceDesignPlan(store);
  const hasAreas = store.areas !== undefined;
  const hasRecorders = store.recorders !== undefined;
  const areas = (store.areas ?? []).map(mapStoreSpaceArea);
  const recorders = (store.recorders ?? []).map(mapBackendRecorder);
  const counts = countAreas(areas);
  const summary = mapStoreSpaceDetailSummary(store, {
    treatmentCount: hasAreas ? counts.treatment : undefined,
    consultationCount: hasAreas ? counts.consultation : undefined,
    beautyCount: hasAreas ? counts.beauty : undefined,
    areaCount: hasAreas ? areas.length : undefined,
    recorderCount: hasRecorders ? recorders.length : undefined,
    channelCount: hasRecorders ? countChannels(recorders) : undefined,
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

function mapStoreSpaceDetailSummary(
  store: BackendStoreSpaceSummary,
  inferred: Partial<Pick<StoreSummary, "recorderCount" | "channelCount" | "treatmentCount" | "consultationCount" | "beautyCount" | "areaCount">>,
): StoreSummary {
  const externalOrgId = store.external_org_id ?? store.externalOrgId ?? "";
  return {
    id: store.id,
    city: store.city ?? store.cityName ?? "",
    name: store.name,
    shortName: store.short_name ?? store.shortName ?? "",
    externalOrgId,
    canViewMonitor: store.can_view_monitor ?? store.canViewMonitor ?? Boolean(externalOrgId),
    thumbnailUrl: "",
    designPlanStatus: store.design_plan_status ?? store.designPlanStatus ?? "not_uploaded",
    recorderCount: store.recorder_count ?? store.recorderCount ?? inferred.recorderCount,
    channelCount: store.channel_count ?? store.channelCount ?? inferred.channelCount,
    channelsFullyConfirmed: store.channels_fully_confirmed ?? store.channelsFullyConfirmed,
    treatmentCount: store.treatment_count ?? store.treatmentCount ?? inferred.treatmentCount,
    consultationCount: store.consultation_count ?? store.consultationCount ?? inferred.consultationCount,
    beautyCount: store.beauty_count ?? store.beautyCount ?? inferred.beautyCount,
    areaCount: store.area_count ?? store.areaCount ?? inferred.areaCount,
    status: mapStoreSpaceOverallStatus(store.overall_status ?? store.overallStatus),
    updatedAt: store.updated_at ?? store.updatedAt ?? new Date().toISOString(),
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
    vip_treatment: "VIP治疗室",
    consultation: "面诊室",
    beauty: "美容室",
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

function mapBackendSnapshotDiagnostics(diagnostics: BackendSnapshotDiagnostics): SnapshotDiagnostics {
  return {
    code: diagnostics.code ?? "",
    stage: diagnostics.stage ?? "",
    assetStore: diagnostics.asset_store ?? diagnostics.assetStore ?? "",
    snapshotName: diagnostics.snapshot_name ?? diagnostics.snapshotName ?? "",
    snapshotKey: diagnostics.snapshot_key ?? diagnostics.snapshotKey ?? "",
    exists: Boolean(diagnostics.exists),
    detail: diagnostics.detail ?? "",
  };
}

function mapBackendLiveAddress(result: BackendLiveAddressResult): LiveAddressResult {
  return {
    url: result.url ?? "",
    urlId: result.url_id ?? result.urlId ?? "",
    expireTime: result.expire_time ?? result.expireTime ?? "",
    protocol: result.protocol ?? "hls",
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
    bedLabel: channel.bed_label ?? channel.bedLabel ?? "",
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
    shortName: match.short_name ?? match.shortName ?? "",
    externalOrgId: "",
    canViewMonitor: false,
    thumbnailUrl: toDisplayImageUrl(match.thumbnail_url),
    designPlanStatus: match.thumbnail_url ? "completed" : "not_uploaded",
    recorderCount: 0,
    channelCount: 0,
    channelsFullyConfirmed: false,
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
    shortName: match.short_name ?? match.shortName ?? "",
    externalOrgId: "",
    canViewMonitor: false,
    thumbnailUrl: "",
    designPlanStatus: "not_uploaded",
    recorderCount: 0,
    channelCount: 0,
    channelsFullyConfirmed: false,
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
    short_name: payload.shortName,
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

function toStoreSpaceBasicInfoPayload(payload: UpdateStoreBasicInfoPayload) {
  return {
    city: payload.city,
    name: payload.name,
    short_name: payload.shortName,
    external_org_id: payload.externalOrgId,
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
      bed_label: patch.bedLabel ? String(patch.bedLabel) : undefined,
    };
  }
  return {
    kind: "non_business",
    scene_type: patch.sceneType ?? "unknown",
    area_note: patch.areaNote ?? patch.areaNumber ?? "",
  };
}

function toDisplayImageUrl(value?: string) {
  const apiBase = value?.startsWith("/api/store-space/") ? STORE_SPACE_API_BASE : value?.startsWith("/api/h5/nvr-monitor/") ? APP_API_BASE : API_BASE;
  return displayImageUrl(value, { apiBase, mockPlanImage: MOCK_PLAN_IMAGE });
}

function resourceViewStoresPath() {
  return `${RESOURCE_VIEW_API_BASE}/stores`;
}

function resourceViewStorePath(tenantId: number) {
  return `${resourceViewStoresPath()}/${tenantId}`;
}

function toStoredPath(value: string, fallback: string) {
  if (value === MOCK_PLAN_IMAGE) {
    return fallback;
  }
  return storedImagePath(value, fallback);
}

export const __testing = {
  resourceViewStoresPath,
  resourceViewStorePath,
  summarizeStoreSummaries,
  toDisplayImageUrl,
  toStoredPath,
};

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
    shortName: "",
    externalOrgId: id <= 3 ? `XY${String(10000 + id)}` : "",
    canViewMonitor: id <= 3,
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
    channelsFullyConfirmed: false,
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
    shortName: "",
    externalOrgId: payload.externalOrgId?.trim() ?? "",
    canViewMonitor: Boolean(payload.externalOrgId?.trim()),
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
    channelsFullyConfirmed: false,
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
  const scenes: Array<Pick<VideoChannel, "sceneType" | "areaType" | "areaNumber" | "bedLabel" | "areaNote" | "status">> = [
    { sceneType: "consultation", areaType: "consultation", areaNumber: "1", bedLabel: "", areaNote: "", status: "pending_confirmation" },
    { sceneType: "treatment", areaType: "treatment", areaNumber: "1", bedLabel: "", areaNote: "", status: "confirmed_business" },
    { sceneType: "front_desk", areaType: "", areaNumber: "", bedLabel: "", areaNote: "前台", status: "confirmed_non_business" },
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
      if (isTreatmentAreaType(item.type)) counts.treatment += 1;
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

function mockMonitorScopesByIds(ids: number[]): MonitorStoreScope[] {
  const idSet = new Set(ids);
  return mockStores
    .filter((store) => idSet.has(store.id))
    .map((store) => ({ storeId: store.id, city: store.city, name: store.name, externalOrgId: store.externalOrgId }));
}

function storeCityName(city: string) {
  return city.trim() || "未设置";
}

function uniqueSorted(values: string[]) {
  return Array.from(new Set(values.map(storeCityName))).sort((left, right) => left.localeCompare(right, "zh-Hans-CN"));
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
