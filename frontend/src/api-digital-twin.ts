import { reportSessionAuthError } from "./domain/auth";
import { NVRLabApiError } from "./api-nvr-lab";
import type { NVRLabCameraListResponse, NVRLabStreamSession, NVRMonitorStoresResponse } from "./domain/nvr-lab";

export type DigitalTwinSettings = { store_ids: string[]; version: string };
const base = `${import.meta.env.BASE_URL}api`;

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`${base}${path}`, { ...options, credentials: "include", cache: "no-store", headers: { Accept: "application/json", ...(options.body ? { "Content-Type": "application/json" } : {}) } });
  const data = response.headers.get("content-type")?.includes("application/json") ? await response.json() : {};
  if (!response.ok) throw reportSessionAuthError(new NVRLabApiError(response.status, data.error || "数字孪生请求失败", data.code || "", data.login_url || ""));
  if (!response.headers.get("content-type")?.includes("application/json")) throw new Error("服务响应异常，请重新登录后重试");
  return data;
}

export const digitalTwinApi = {
  settings: () => request<DigitalTwinSettings>("/admin/digital-twin-settings"),
  candidates: () => request<NVRMonitorStoresResponse>("/admin/digital-twin-candidates"),
  save: (settings: DigitalTwinSettings) => request<DigitalTwinSettings>("/admin/digital-twin-settings", { method: "PUT", body: JSON.stringify(settings) }),
  stores: () => request<NVRMonitorStoresResponse>("/digitaltwin/stores"),
  cameras: (id: string, signal?: AbortSignal) => request<NVRLabCameraListResponse>(`/digitaltwin/orgs/${encodeURIComponent(id)}/cameras`, { signal }),
  stream: (id: string, cameraId: number, signal?: AbortSignal) => request<NVRLabStreamSession>(`/digitaltwin/orgs/${encodeURIComponent(id)}/cameras/${cameraId}/stream-session`, { method: "POST", body: JSON.stringify({ mode: "live" }), signal }),
};
