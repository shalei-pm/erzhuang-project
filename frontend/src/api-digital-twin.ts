import { reportSessionAuthError } from "./domain/auth";
import { NVRLabApiError } from "./api-nvr-lab";
import type { NVRLabCameraListResponse, NVRLabStreamSession, NVRMonitorStoresResponse } from "./domain/nvr-lab";

export type DigitalTwinSettings = { store_ids: string[]; version: string };
export type DigitalTwinDashboard = {
  tenant_id: number;
  date: string;
  fetched_at: string;
  overview: { expected_arrival: number; arrived: number; no_consult: number; need_consult: number; non_quick: number };
  duty_staff: { consultants: number; nurses: number; doctors: number };
  traffic_flow: {
    reception_current: number;
    consultation_current: number;
    consultation_served: number;
    waiting: number;
    waiting_no_consult: number;
    waiting_need_consult: number;
    waiting_non_quick: number;
    treatment_current: number;
    treatment_served: number;
    treatment_served_no_consult: number;
    treatment_served_need_consult: number;
    treatment_served_non_quick: number;
    postoperative_care: number;
  };
};
export type DigitalTwinT1BI = {
  tenant_id: number;
  begin_day: string;
  end_day: string;
  fetched_at: string;
  trends: Array<{
    date: string;
    visit_all: number | null;
    visit_no_consult: number | null;
    visit_need_consult: number | null;
    stay_all: number | null;
    wait_all: number | null;
    upgrade_all: number | null;
    redemption_all: number | null;
    service_point_all: number | null;
  }>;
};
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
  dashboard: (id: string, signal?: AbortSignal) => request<DigitalTwinDashboard>(`/digitaltwin/orgs/${encodeURIComponent(id)}/dashboard`, { signal }),
  t1BI: (id: string, signal?: AbortSignal) => request<DigitalTwinT1BI>(`/digitaltwin/orgs/${encodeURIComponent(id)}/t1-bi`, { signal }),
  stream: (id: string, cameraId: number, signal?: AbortSignal) => request<NVRLabStreamSession>(`/digitaltwin/orgs/${encodeURIComponent(id)}/cameras/${cameraId}/stream-session`, { method: "POST", body: JSON.stringify({ mode: "live" }), signal }),
};
