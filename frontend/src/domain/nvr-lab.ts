export type NVRLabMode = "live" | "playback";
export type NVRLabRoute = { name: "home" } | { name: "camera"; cameraId: number } | null;

export type NVRLabCamera = {
  id: number;
  name: string;
  space_type?: string;
  space_name?: string;
};

export type NVRLabCameraListResponse = {
  tenant_id: number;
  store_name: string;
  cameras: NVRLabCamera[];
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

export function validateNVRLabPlayback(startTime: number, endTime: number): string {
  if (!Number.isFinite(startTime) || !Number.isFinite(endTime) || startTime <= 0 || endTime <= 0) {
    return "请选择完整的回放时间范围";
  }
  if (endTime <= startTime) return "结束时间必须晚于开始时间";
  if (endTime - startTime > 30 * 60) return "单次回放最长支持 30 分钟";
  return "";
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
