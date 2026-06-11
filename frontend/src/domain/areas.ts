import type { AreaBox, AreaType, StoreArea } from "../api";

export const areaTypeLabels: Record<AreaType, string> = {
  treatment: "治疗室",
  consultation: "面诊室",
  beauty: "生美",
};

export function areaDisplayName(area: StoreArea) {
  if (!area.type) return "";
  if (area.number.trim()) return `${areaTypeLabels[area.type]} ${area.number.trim()}`;
  return area.type === "beauty" ? areaTypeLabels[area.type] : "";
}

export function areaBoxPrimaryLabel(area: StoreArea) {
  if (area.number.trim()) return area.number.trim();
  if (area.type) return areaTypeLabels[area.type];
  return "未分类";
}

export function areaBoxSecondaryLabel(area: StoreArea) {
  if (!area.type || !area.number.trim()) return "";
  return areaTypeLabels[area.type];
}

export function areaSummary(area: StoreArea) {
  if (!area.type) return "未选择类型";
  const label = areaTypeLabels[area.type];
  if (!area.number) return area.type === "beauty" ? label : `${label} · 未编号`;
  return `${label} · 编号 ${area.number}`;
}

export function withGeneratedAreaFields(area: StoreArea): StoreArea {
  return {
    ...area,
    name: areaDisplayName(area),
  };
}

export function normalizeAreaForSave(area: StoreArea): StoreArea {
  const generated = withGeneratedAreaFields(area);
  const isComplete =
    Boolean(generated.type) &&
    Boolean(generated.box) &&
    Boolean(generated.name.trim()) &&
    Boolean(generated.number.trim());
  return {
    ...generated,
    confidence: isComplete ? "high" : generated.confidence,
    needsReview: isComplete ? false : generated.needsReview,
  };
}

export function boxStyle(box: AreaBox) {
  return {
    left: `${box.x * 100}%`,
    top: `${box.y * 100}%`,
    width: `${box.width * 100}%`,
    height: `${box.height * 100}%`,
  };
}

export function createManualArea(): StoreArea {
  return {
    id: `manual-${Date.now()}`,
    name: "",
    type: "",
    number: "",
    confidence: "high",
    needsReview: false,
    box: {
      x: 0.42,
      y: 0.4,
      width: 0.16,
      height: 0.12,
    },
  };
}

export function countAreaTypes(areas: StoreArea[]) {
  return areas.reduce(
    (counts, item) => {
      if (item.type === "treatment") counts.treatment += 1;
      if (item.type === "consultation") counts.consultation += 1;
      if (item.type === "beauty") counts.beauty += 1;
      return counts;
    },
    { treatment: 0, consultation: 0, beauty: 0 },
  );
}
