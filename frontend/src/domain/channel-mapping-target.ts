import type { AreaType } from "../api";
import { areaTypeLabels } from "./areas";

export type ChannelMappingTarget = {
  areaType: AreaType | "";
  areaNumber?: string | number | null;
  bedLabel?: string | null;
  areaNote?: string | null;
};

export function requiresBedSplit(type: AreaType | "" | undefined) {
  return type === "treatment" || type === "vip_treatment" || type === "beauty";
}

export function channelMappingTargetLabel(target: ChannelMappingTarget, options: { includeSingleBedSuffix?: boolean } = {}) {
  if (!target.areaType) {
    return clean(target.areaNote);
  }
  const label = areaTypeLabels[target.areaType];
  const areaNumber = cleanPositiveNumber(target.areaNumber);
  const bedLabel = clean(target.bedLabel);
  if (!areaNumber) {
    const note = clean(target.areaNote);
    return note ? `${label}${note}` : label;
  }
  if (bedLabel) return `${label}${areaNumber}-${bedLabel}`;
  return options.includeSingleBedSuffix ? `${label}${areaNumber}号` : `${label}${areaNumber}`;
}

function clean(value: unknown) {
  return String(value ?? "").trim();
}

function cleanPositiveNumber(value: unknown) {
  const text = clean(value);
  if (text === "" || text === "0") return "";
  return text;
}
