import type { NVRLabCameraListResponse, NVRLabMode, NVRLabStreamSession } from "./domain/nvr-lab";

const API_BASE = apiBase();

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
  listCameras(): Promise<NVRLabCameraListResponse> {
    return requestJSON(`${API_BASE}/h5/nvr-lab/10001/cameras`);
  },

  createStreamSession(cameraId: number, mode: NVRLabMode, startTime?: number, endTime?: number): Promise<NVRLabStreamSession> {
    const payload: Record<string, unknown> = { mode };
    if (mode === "playback") {
      payload.start_time = startTime;
      payload.end_time = endTime;
    }
    return requestJSON(`${API_BASE}/h5/nvr-lab/10001/cameras/${encodeURIComponent(String(cameraId))}/stream-session`, {
      method: "POST",
      body: JSON.stringify(payload),
    });
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
