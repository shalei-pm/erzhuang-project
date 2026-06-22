export type ChannelSceneType =
  | "treatment"
  | "consultation"
  | "beauty"
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

const sceneLabels: Record<ChannelSceneType, string> = {
  treatment: "治疗室",
  consultation: "面诊室",
  beauty: "生美",
  front_desk: "前台",
  corridor: "走廊",
  passage: "通道",
  waiting_area: "候诊区",
  hall: "大厅",
  entrance: "门口",
  storage: "库房",
  pharmacy: "药房",
  machine_room: "机房",
  unknown: "其他区域",
};

export function channelSceneLabel(sceneType: ChannelSceneType | string) {
  return sceneLabels[sceneType as ChannelSceneType] ?? "其他区域";
}
