export type NVRLabMode = "live" | "playback";
export type NVRLabRoute = { name: "home" } | { name: "camera"; cameraId: number } | null;
export type NVRMonitorRoute = { name: "home"; externalOrgId: string } | { name: "camera"; externalOrgId: string; cameraId: number } | null;
export const NVR_LAB_MAX_PLAYBACK_SECONDS = 60 * 60;

export type NVRLabPlaybackRange = {
  startAt: string;
  endAt: string;
  startTime: number;
  endTime: number;
};

export type NVRLabPlaybackSession = {
  playbackWindow: NVRLabPlaybackRange;
  startTime: number;
  endTime: number;
};

export type NVRLabCamera = {
  id: number;
  name: string;
  space_type?: string;
  space_name?: string;
};

export type NVRLabCameraListResponse = {
	external_org_id: string;
  tenant_id: number;
  store_name: string;
	city?: string;
  cameras: NVRLabCamera[];
};

export type NVRMonitorStoreInfo = {
  external_org_id: string;
  store_name: string;
  city: string;
  available_camera_count: number;
};

export type NVRMonitorStoresResponse = {
  cities: Array<{ city: string; stores: NVRMonitorStoreInfo[] }>;
};

export type NVRLabStreamSession = {
  url: string;
  mode: NVRLabMode;
};

export type NVRLabTabKey = "all" | "treatment" | "consultation" | "beauty" | "other";

export const nvrLabTabs: Array<{ key: NVRLabTabKey; label: string }> = [
  { key: "all", label: "全部" },
  { key: "consultation", label: "面诊室" },
  { key: "treatment", label: "治疗室" },
  { key: "beauty", label: "美容室" },
  { key: "other", label: "其他" },
];

export function parseNVRLabRoute(pathname: string): NVRLabRoute {
  const parts = pathname.split("/").filter(Boolean);
  const h5Index = parts.lastIndexOf("h5");
  if (h5Index < 0 || parts[h5Index + 1] !== "nvr-lab" || parts[h5Index + 2] !== "10001") return null;
  if (parts.length === h5Index + 3) return { name: "home" };
  if (parts.length !== h5Index + 5 || parts[h5Index + 3] !== "cameras") return null;
  const cameraId = Number(parts[h5Index + 4]);
  return Number.isInteger(cameraId) && cameraId > 0 ? { name: "camera", cameraId } : null;
}

export function parseNVRMonitorRoute(pathname: string): NVRMonitorRoute {
  const parts = pathname.split("/").filter(Boolean);
  const h5Index = parts.lastIndexOf("h5");
  if (h5Index < 0 || parts[h5Index + 1] !== "orgs" || !parts[h5Index + 2] || parts[h5Index + 3] !== "monitor") return null;
  const externalOrgId = decodeURIComponent(parts[h5Index + 2]);
  if (parts.length === h5Index + 4) return { name: "home", externalOrgId };
  if (parts.length !== h5Index + 6 || parts[h5Index + 4] !== "cameras") return null;
  const cameraId = Number(parts[h5Index + 5]);
  return Number.isInteger(cameraId) && cameraId > 0 ? { name: "camera", externalOrgId, cameraId } : null;
}

export function validateNVRLabPlayback(startTime: number, endTime: number): string {
  if (!Number.isFinite(startTime) || !Number.isFinite(endTime) || startTime <= 0 || endTime <= 0) {
    return "请选择完整的回放时间范围";
  }
  if (endTime <= startTime) return "结束时间必须晚于开始时间";
  if (endTime - startTime > NVR_LAB_MAX_PLAYBACK_SECONDS) return "单次回放最长支持 1 小时";
  return "";
}

export function buildNVRLabHourlyPlayback(date: string, hour: number, now = new Date()): NVRLabPlaybackRange | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || !Number.isInteger(hour) || hour < 0 || hour > 23) return null;
  return buildNVRLabPlaybackFromStart(`${date}T${pad2(hour)}:00`, now);
}

export function buildNVRLabPlaybackFromStart(startAt: string, now = new Date()): NVRLabPlaybackRange | null {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(startAt)) return null;
  const start = new Date(`${startAt}:00`);
  if (Number.isNaN(start.getTime())) return null;
  start.setSeconds(0, 0);

  const latest = new Date(now);
  latest.setSeconds(0, 0);
  if (start.getTime() >= latest.getTime()) return null;

  const end = new Date(Math.min(start.getTime() + NVR_LAB_MAX_PLAYBACK_SECONDS * 1000, latest.getTime()));
  const startTime = Math.floor(start.getTime() / 1000);
  const endTime = Math.floor(end.getTime() / 1000);
  if (validateNVRLabPlayback(startTime, endTime)) return null;
  return { startAt: formatDateTimeInput(start), endAt: formatDateTimeInput(end), startTime, endTime };
}

export function buildNVRLabPlaybackSession(playbackWindow: NVRLabPlaybackRange, startTime = playbackWindow.startTime): NVRLabPlaybackSession | null {
  if (!Number.isFinite(startTime) || startTime < playbackWindow.startTime || startTime >= playbackWindow.endTime) return null;
  return { playbackWindow, startTime, endTime: playbackWindow.endTime };
}

export function nvrLabCameraTitle(camera: NVRLabCamera): string {
  return camera.space_name?.trim() || camera.name?.trim() || `摄像头 ${camera.id}`;
}

export function nvrLabCameraSubtitle(camera: NVRLabCamera): string {
  return camera.space_type?.trim() || `摄像头 ${camera.id}`;
}

export function nvrLabCameraTab(camera: NVRLabCamera): NVRLabTabKey {
  const value = `${camera.space_type || ""}${camera.space_name || ""}`;
  if (value.includes("治疗")) return "treatment";
  if (value.includes("面诊") || value.includes("咨询")) return "consultation";
  if (value.includes("美容") || value.includes("生美")) return "beauty";
  return "other";
}

export function nvrLabRoutePath(route: Exclude<NVRLabRoute, null>): string {
  const base = (import.meta.env.BASE_URL || "/erzhuang-project/").replace(/\/$/, "");
  return route.name === "home" ? `${base}/h5/nvr-lab/10001` : `${base}/h5/nvr-lab/10001/cameras/${route.cameraId}`;
}

export function nvrMonitorRoutePath(route: Exclude<NVRMonitorRoute, null>): string {
  const base = nvrMonitorBasePath();
  const externalOrgId = encodeURIComponent(route.externalOrgId);
  return route.name === "home" ? `${base}/h5/orgs/${externalOrgId}/monitor` : `${base}/h5/orgs/${externalOrgId}/monitor/cameras/${route.cameraId}`;
}

function nvrMonitorBasePath(): string {
  if (typeof window !== "undefined") {
    const match = window.location.pathname.match(/^(.*)\/h5\/(?:orgs|nvr-lab)\//);
    if (match?.[1]) return match[1].replace(/\/$/, "");
  }
  return (import.meta.env.BASE_URL || "/erzhuang-project/").replace(/\/$/, "");
}

function formatDateTimeInput(date: Date): string {
  return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}T${pad2(date.getHours())}:${pad2(date.getMinutes())}`;
}

function pad2(value: number): string {
  return `${value}`.padStart(2, "0");
}
