import type { NVRLabCameraListResponse, NVRLabMode, NVRLabStreamSession, NVRMonitorStoresResponse } from "./domain/nvr-lab";

const API_BASE = apiBase();

export function nvrLabThumbnailURL(value: string | undefined): string {
  const trimmed = value?.trim() || "";
  if (!trimmed.startsWith("/api/")) return trimmed;
  return `${API_BASE}${trimmed.slice("/api".length)}`;
}

export class NVRLabApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, message: string, code = "") {
    super(message);
    this.name = "NVRLabApiError";
    this.status = status;
    this.code = code;
  }
}

export const nvrLabApi = {
  getMonitorMode(): Promise<{ mode: "legacy" | "nvr" }> {
    return requestJSON(`${API_BASE}/h5/monitor-mode`);
  },

  listMonitorStores(): Promise<NVRMonitorStoresResponse> {
    return requestJSON(`${API_BASE}/h5/nvr-monitor/stores`);
  },

  listCameras(externalOrgId: string): Promise<NVRLabCameraListResponse> {
    return requestJSON(`${API_BASE}/h5/nvr-monitor/orgs/${encodeURIComponent(externalOrgId)}/cameras`);
  },

  createStreamSession(externalOrgId: string, cameraId: number, mode: NVRLabMode, startTime?: number, endTime?: number): Promise<NVRLabStreamSession> {
    const payload: Record<string, unknown> = { mode };
    if (mode === "playback") {
      payload.start_time = startTime;
      payload.end_time = endTime;
    }
    return requestJSON(`${API_BASE}/h5/nvr-monitor/orgs/${encodeURIComponent(externalOrgId)}/cameras/${encodeURIComponent(String(cameraId))}/stream-session`, {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  async uploadSnapshot(externalOrgId: string, cameraId: number, image: Blob): Promise<void> {
    const response = await fetch(`${API_BASE}/h5/nvr-monitor/orgs/${encodeURIComponent(externalOrgId)}/cameras/${encodeURIComponent(String(cameraId))}/snapshot`, {
      method: "POST",
      credentials: "include",
      cache: "no-store",
      headers: { "Content-Type": "image/jpeg" },
      body: image,
    });
    if (!response.ok) {
      const contentType = response.headers.get("content-type") || "";
      const data = contentType.includes("application/json") ? await response.json() : {};
      const body = typeof data === "object" && data ? (data as Record<string, unknown>) : {};
      throw new NVRLabApiError(response.status, String(body.error || `HTTP ${response.status}`), String(body.code || ""));
    }
  },
};

function apiBase(): string {
  const configured = import.meta.env.VITE_NVR_LAB_API_BASE;
  if (configured) return configured.replace(/\/+$/, "");
  const base = import.meta.env.BASE_URL || "/erzhuang-project/";
  const segment = base.split("/").filter(Boolean)[0] || "erzhuang-project";
  return `/${segment}/api`;
}

async function requestJSON<T>(url: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    Accept: "application/json",
    ...((options.headers as Record<string, string>) || {}),
  };
  if (typeof options.body === "string") headers["Content-Type"] = "application/json";
  const response = await fetch(url, { ...options, credentials: "include", cache: "no-store", headers });
  const contentType = response.headers.get("content-type") || "";
  const data = contentType.includes("application/json") ? await response.json() : await response.text();
  if (!response.ok) {
    const body = typeof data === "object" && data ? (data as Record<string, unknown>) : {};
    throw new NVRLabApiError(response.status, String(body.error || `HTTP ${response.status}`), String(body.code || ""));
  }
  return data as T;
}
