import type {
  H5MonitorHomeResponse,
  H5MonitorStoresResponse,
  H5LiveURLResponse,
  H5RecordSegmentsResponse,
  H5PlaybackURLResponse,
  H5StreamQuality,
} from "./domain/h5-types";

function apiBase(): string {
  const configured = import.meta.env.VITE_H5_API_BASE;
  if (configured) return configured.replace(/\/+$/, "");
  const base = import.meta.env.BASE_URL || "/erzhuang/";
  const runtimeBase = runtimeBasePath(base);
  const segments = runtimeBase.split("/").filter(Boolean);
  const root = segments.length > 0 ? `/${segments[0]}` : "";
  return `${root}/api`;
}

const API_BASE = apiBase();

async function requestJSON<T>(url: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    Accept: "application/json",
    ...((options.headers as Record<string, string>) || {}),
  };
  if (options.body && typeof options.body === "string") {
    headers["Content-Type"] = "application/json";
  }

  const response = await fetch(url, { ...options, headers });

  if (response.status === 204) {
    return undefined as T;
  }

  const contentType = response.headers.get("content-type") ?? "";
  const data = contentType.includes("application/json") ? await response.json() : await response.text();

  if (!response.ok) {
    const message =
      typeof data === "object" && data && "error" in data
        ? String((data as Record<string, unknown>).error)
        : `HTTP ${response.status}`;
    const code =
      typeof data === "object" && data && "code" in data
        ? String((data as Record<string, unknown>).code)
        : "";
    const fields =
      typeof data === "object" && data && "fields" in data
        ? (data as Record<string, Record<string, string>>).fields
        : {};
    throw new H5ApiError(response.status, message, fields || {}, code);
  }

  return data as T;
}

function runtimeBasePath(configuredBase: string): string {
  const normalizedBase = configuredBase.startsWith("/") ? configuredBase : `/${configuredBase}`;
  const baseSegment = normalizedBase.split("/").filter(Boolean)[0] || "";
  const currentSegment = typeof window === "undefined" ? "" : window.location.pathname.split("/").filter(Boolean)[0] || "";
  if (currentSegment && currentSegment !== baseSegment) {
    return `/${currentSegment}`;
  }
  return baseSegment ? `/${baseSegment}` : "";
}

export class H5ApiError extends Error {
  status: number;
  fields: Record<string, string>;
  code: string;

  constructor(status: number, message: string, fields: Record<string, string> = {}, code = "") {
    super(message);
    this.name = "H5ApiError";
    this.status = status;
    this.fields = fields;
    this.code = code;
  }
}

export const h5Api = {
  async listMonitorStores(): Promise<H5MonitorStoresResponse> {
    if (import.meta.env.DEV) {
      return mockMonitorStores();
    }
    return requestJSON(`${API_BASE}/h5/monitor/stores`);
  },

  async getMonitorHome(externalOrgId: string): Promise<H5MonitorHomeResponse> {
    if (import.meta.env.DEV && externalOrgId === "demo") {
      return mockMonitorHome();
    }
    return requestJSON(`${API_BASE}/h5/orgs/${encodeURIComponent(externalOrgId)}/monitor`);
  },

  async getLiveUrl(
    externalOrgId: string,
    channelId: number,
    userId: string,
    isAdmin: boolean,
    protocol = "flv",
    quality: H5StreamQuality = "sd",
  ): Promise<H5LiveURLResponse> {
    if (import.meta.env.DEV && externalOrgId === "demo") {
      void userId;
      void isAdmin;
      return { url: `mock-live-${channelId}`, expire_time: "local demo", url_id: `demo-live-${channelId}`, protocol };
    }
    return requestJSON(`${API_BASE}/h5/orgs/${encodeURIComponent(externalOrgId)}/monitor/channels/${channelId}/live-url`, {
      method: "POST",
      body: JSON.stringify({ user_id: userId, is_admin: isAdmin, protocol, quality }),
    });
  },

  async getRecordSegments(externalOrgId: string, channelId: number, date: string): Promise<H5RecordSegmentsResponse> {
    if (import.meta.env.DEV && externalOrgId === "demo") {
      void channelId;
      const day = new Date(`${date}T00:00:00`).getTime() / 1000;
      return {
        date,
        segments: [
          { start_time: day + 9 * 3600 + 14 * 60, end_time: day + 9 * 3600 + 38 * 60, type: "PLAN", type_label: "定时录像" },
          { start_time: day + 11 * 3600 + 32 * 60, end_time: day + 12 * 3600 + 10 * 60, type: "ALARM", type_label: "事件录像" },
          { start_time: day + 14 * 3600 + 24 * 60, end_time: day + 14 * 3600 + 52 * 60, type: "PLAN", type_label: "定时录像" },
          { start_time: day + 16 * 3600 + 5 * 60, end_time: day + 16 * 3600 + 36 * 60, type: "PLAN", type_label: "定时录像" },
        ],
      };
    }
    return requestJSON(`${API_BASE}/h5/orgs/${encodeURIComponent(externalOrgId)}/monitor/channels/${channelId}/record-segments?date=${encodeURIComponent(date)}`);
  },

  async getPlaybackUrl(
    externalOrgId: string,
    channelId: number,
    startTime: number,
    stopTime: number,
    userId: string,
    isAdmin: boolean,
  ): Promise<H5PlaybackURLResponse> {
    if (import.meta.env.DEV && externalOrgId === "demo") {
      void startTime;
      void stopTime;
      void userId;
      void isAdmin;
      return { url: `mock-playback-${channelId}`, expire_time: "local demo", url_id: `demo-playback-${channelId}` };
    }
    return requestJSON(`${API_BASE}/h5/orgs/${encodeURIComponent(externalOrgId)}/monitor/channels/${channelId}/playback-url`, {
      method: "POST",
      body: JSON.stringify({ start_time: startTime, stop_time: stopTime, user_id: userId, is_admin: isAdmin }),
    });
  },

  async disableUrl(externalOrgId: string, channelId: number, urlId: string, userId: string): Promise<void> {
    if (import.meta.env.DEV && externalOrgId === "demo") {
      void channelId;
      void urlId;
      void userId;
      return;
    }
    await requestJSON(`${API_BASE}/h5/orgs/${encodeURIComponent(externalOrgId)}/monitor/channels/${channelId}/disable-url?user_id=${encodeURIComponent(userId)}`, {
      method: "POST",
      body: JSON.stringify({ url_id: urlId }),
    });
  },
};

function mockMonitorStores(): H5MonitorStoresResponse {
  return {
    cities: [
      {
        city: "北京",
        stores: [
          {
            external_org_id: "demo",
            store_name: "新氧青春演示门店",
            city: "北京",
            available_channel_count: 36,
          },
          {
            external_org_id: "10030",
            store_name: "新氧青春国贸门店",
            city: "北京",
            available_channel_count: 18,
          },
        ],
      },
      {
        city: "上海",
        stores: [
          {
            external_org_id: "10047",
            store_name: "新氧青春静安门店",
            city: "上海",
            available_channel_count: 24,
          },
          {
            external_org_id: "10031",
            store_name: "新氧青春徐汇门店",
            city: "上海",
            available_channel_count: 12,
          },
        ],
      },
    ],
  };
}

function mockMonitorHome(): H5MonitorHomeResponse {
  const groups: H5MonitorHomeResponse["groups"] = [
    { category: "consultation", label: "面诊室", channels: [] },
    { category: "treatment", label: "治疗室", channels: [] },
    { category: "beauty", label: "美容室", channels: [] },
    { category: "front_waiting", label: "前台/等候区", channels: [] },
    { category: "other", label: "过道/其他", channels: [] },
  ];
  const names = ["面诊室", "治疗室", "美容室", "前台", "等候区", "过道", "仓储间"];
  for (let i = 0; i < 36; i += 1) {
    const group = groups[i % groups.length];
    const label = names[i % names.length];
    group.channels.push({
      id: i + 1,
      channel_no: i + 1,
      channel_name: `${label}${Math.floor(i / groups.length) + 1}`,
      category: group.category,
      area_type:
        group.category === "consultation" || group.category === "treatment" || group.category === "beauty"
          ? group.category
          : "",
      scene_type: group.category === "front_waiting" ? "front_desk" : group.category === "other" ? "corridor" : "",
      area_number: Math.floor(i / groups.length) + 1,
      bed_label: "",
      area_note: "",
      thumbnail_url: `https://picsum.photos/seed/h5-monitor-${i + 1}/320/320`,
    });
  }
  return {
    external_org_id: "demo",
    store_name: "新氧青春演示门店",
    city: "北京",
    groups,
  };
}
