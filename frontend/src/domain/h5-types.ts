export type MonitorCategory = "consultation" | "treatment" | "beauty" | "front_waiting" | "other";
export type AreaType = "consultation" | "treatment" | "beauty" | "";

export interface H5MonitorChannel {
  id: number;
  channel_no: number;
  channel_name: string;
  category: MonitorCategory;
  area_type: AreaType;
  scene_type: string;
  area_number: number;
  area_note: string;
  thumbnail_url: string;
}

export interface H5MonitorGroup {
  category: MonitorCategory;
  label: string;
  channels: H5MonitorChannel[];
}

export interface H5MonitorHomeResponse {
  external_org_id: string;
  store_name: string;
  city: string;
  groups: H5MonitorGroup[];
}

export interface H5LiveURLResponse {
  url: string;
  expire_time: string;
  url_id: string;
}

export interface H5RecordSegment {
  start_time: number;
  end_time: number;
  type: string;
  type_label: string;
}

export interface H5RecordSegmentsResponse {
  date: string;
  segments: H5RecordSegment[];
}

export interface H5PlaybackURLResponse {
  url: string;
  expire_time: string;
  url_id: string;
}

export interface H5ApiError {
  error: string;
  fields?: Record<string, string>;
}
