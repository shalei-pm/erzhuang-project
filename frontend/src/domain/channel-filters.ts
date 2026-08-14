import type { AreaType, SceneType } from "../api";
import { areaTypeLabels, isTreatmentAreaType } from "./areas";
import { channelSceneLabel } from "./channel-labels";

export type ChannelListFilter = "all" | "consultation" | "treatment" | "beauty" | "front_waiting" | "passage_other";

export type ChannelFilterable = {
  id: number;
  recorderCode: string;
  channelNo: number;
  channelName?: string;
  status?: string;
  areaType?: AreaType | "";
  areaNumber?: string | number;
  areaNote?: string;
  sceneType?: SceneType | string;
};

export const channelListFilters: { value: ChannelListFilter; label: string }[] = [
  { value: "all", label: "全部" },
  { value: "consultation", label: "面诊室" },
  { value: "treatment", label: "治疗室" },
  { value: "beauty", label: "美容室" },
  { value: "front_waiting", label: "前台/候诊区" },
  { value: "passage_other", label: "通道/其他" },
];

// Shared by the admin channel-mapping table and the future H5 monitor list.
// Keep grouping/sorting business rules here instead of duplicating them in page components.
export function filterAndSortChannels<T extends ChannelFilterable>(
  channels: T[],
  filter: ChannelListFilter,
  drafts: Record<number, Partial<T>> = {},
) {
  return channels
    .filter((channel) => channel.status !== "inactive")
    .filter((channel) => channelMatchesListFilter(channel, filter, drafts[channel.id]))
    .sort((left, right) => compareChannelsForFilter(left, right, filter, drafts));
}

export function channelMatchesListFilter<T extends ChannelFilterable>(channel: T, filter: ChannelListFilter, draft?: Partial<T>) {
  if (filter === "all") return true;
  const areaType = effectiveAreaType(channel, draft);
  if (filter === "consultation") return areaType === "consultation";
  if (filter === "treatment") return isTreatmentAreaType(areaType);
  if (filter === "beauty") return areaType === "beauty";
  if (filter === "front_waiting") return !isBusinessAreaType(areaType) && isFrontWaitingChannel(channel, draft);
  return !isBusinessAreaType(areaType) && !isFrontWaitingChannel(channel, draft);
}

export function compareChannelsForFilter<T extends ChannelFilterable>(
  left: T,
  right: T,
  filter: ChannelListFilter,
  drafts: Record<number, Partial<T>> = {},
) {
  if (filter === "all") return stableChannelCompare(left, right);
  if (filter === "consultation" || filter === "treatment" || filter === "beauty") {
    return compareBusinessChannel(left, right, drafts[left.id], drafts[right.id]);
  }
  return compareTextChannel(left, right, drafts[left.id], drafts[right.id]);
}

function compareBusinessChannel<T extends ChannelFilterable>(left: T, right: T, leftDraft?: Partial<T>, rightDraft?: Partial<T>) {
  const leftText = channelDisplayText(left, leftDraft);
  const rightText = channelDisplayText(right, rightDraft);
  const leftNumber = firstNumber(effectiveAreaNumber(left, leftDraft) || leftText);
  const rightNumber = firstNumber(effectiveAreaNumber(right, rightDraft) || rightText);
  if (leftNumber !== null && rightNumber !== null && leftNumber !== rightNumber) return leftNumber - rightNumber;
  if (leftNumber !== null && rightNumber === null) return -1;
  if (leftNumber === null && rightNumber !== null) return 1;
  return compareChinese(leftText, rightText) || stableChannelCompare(left, right);
}

function compareTextChannel<T extends ChannelFilterable>(left: T, right: T, leftDraft?: Partial<T>, rightDraft?: Partial<T>) {
  const leftText = channelDisplayText(left, leftDraft);
  const rightText = channelDisplayText(right, rightDraft);
  return frontWaitingRank(leftText) - frontWaitingRank(rightText) || compareChinese(leftText, rightText) || stableChannelCompare(left, right);
}

function stableChannelCompare(left: ChannelFilterable, right: ChannelFilterable) {
  return compareChinese(left.recorderCode, right.recorderCode) || left.channelNo - right.channelNo || left.id - right.id;
}

function effectiveAreaType<T extends ChannelFilterable>(channel: T, draft?: Partial<T>) {
  return draft?.areaType !== undefined ? draft.areaType : channel.areaType;
}

function effectiveAreaNumber<T extends ChannelFilterable>(channel: T, draft?: Partial<T>) {
  return String(draft?.areaNumber ?? channel.areaNumber ?? "").trim();
}

function channelDisplayText<T extends ChannelFilterable>(channel: T, draft?: Partial<T>) {
  const areaType = effectiveAreaType(channel, draft);
  const areaNumber = effectiveAreaNumber(channel, draft);
  const areaNote = String(draft?.areaNote ?? channel.areaNote ?? "").trim();
  return [
    areaType ? areaTypeLabels[areaType as AreaType] : "",
    areaNumber,
    areaNote,
    channel.channelName ?? "",
    channelSceneLabel(draft?.sceneType ?? channel.sceneType ?? ""),
  ]
    .filter(Boolean)
    .join(" ");
}

function isBusinessAreaType(areaType: AreaType | "" | undefined) {
  return areaType === "consultation" || areaType === "beauty" || isTreatmentAreaType(areaType);
}

function isFrontWaitingChannel<T extends ChannelFilterable>(channel: T, draft?: Partial<T>) {
  return /前台|候诊|等候/.test(channelDisplayText(channel, draft));
}

function frontWaitingRank(text: string) {
  if (text.includes("候诊")) return 0;
  if (text.includes("等候")) return 1;
  if (text.includes("前台")) return 2;
  return 3;
}

function firstNumber(value: string) {
  const match = value.match(/\d+/);
  return match ? Number(match[0]) : null;
}

function compareChinese(left: string, right: string) {
  return left.localeCompare(right, "zh-Hans-CN", { numeric: true, sensitivity: "base" });
}
