import type { AreaBox, AreaType, StoreArea } from "../api";

export const areaTypeLabels: Record<AreaType, string> = {
  treatment: "治疗室",
  vip_treatment: "VIP治疗室",
  consultation: "面诊室",
  beauty: "美容室",
};

export function isTreatmentAreaType(type: AreaType | "" | undefined) {
  return type === "treatment" || type === "vip_treatment";
}

export function isAreaNumberOptional(type: AreaType | "" | undefined) {
  return type === "vip_treatment";
}

export function areaDisplayName(area: StoreArea) {
  if (!area.type) return "";
  if (area.number.trim()) return `${areaTypeLabels[area.type]} ${area.number.trim()}`;
  return isAreaNumberOptional(area.type) || area.type === "beauty" ? areaTypeLabels[area.type] : "";
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
  if (!area.number) return isAreaNumberOptional(area.type) || area.type === "beauty" ? label : `${label} · 未编号`;
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
  const numberComplete = isAreaNumberOptional(generated.type) || Boolean(generated.number.trim());
  const isComplete =
    Boolean(generated.type) &&
    Boolean(generated.box) &&
    Boolean(generated.name.trim()) &&
    numberComplete;
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

export function mergeRecognizedAreas(existingAreas: StoreArea[], recognizedAreas: StoreArea[]): StoreArea[] {
  const recognizedById = new Map<string, StoreArea>();
  const recognizedByKey = new Map<string, StoreArea>();

  recognizedAreas.forEach((area) => {
    recognizedById.set(area.id, area);
    const key = businessAreaKey(area);
    if (key) {
      recognizedByKey.set(key, area);
    }
  });

  const matchedKeys = new Set<string>();
  const matchedIds = new Set<string>();
  const mergedExisting = existingAreas.map((existingArea, index) => {
    const key = businessAreaKey(existingArea);
    const recognizedArea = recognizedById.get(existingArea.id) ?? (key ? recognizedByKey.get(key) : undefined);
    if (!recognizedArea) {
      return ensureAnnotationPlaceholder(existingArea, index);
    }
    matchedIds.add(recognizedArea.id);
    if (key) matchedKeys.add(key);
    const recognizedKey = businessAreaKey(recognizedArea);
    if (recognizedKey) matchedKeys.add(recognizedKey);
    return withGeneratedAreaFields({
      ...existingArea,
      name: recognizedArea.name,
      type: recognizedArea.type,
      number: recognizedArea.number,
      source: recognizedArea.source,
      confidence: recognizedArea.confidence,
      needsReview: recognizedArea.needsReview || !recognizedArea.box,
      box: recognizedArea.box ?? existingArea.box ?? pendingAnnotationBox(index),
    });
  });

  const newAreas = recognizedAreas
    .filter((recognizedArea) => {
      if (matchedIds.has(recognizedArea.id)) return false;
      const key = businessAreaKey(recognizedArea);
      return !key || !matchedKeys.has(key);
    })
    .map((recognizedArea, index) =>
      withGeneratedAreaFields({
        ...recognizedArea,
        box: recognizedArea.box ?? pendingAnnotationBox(mergedExisting.length + index),
        needsReview: recognizedArea.needsReview || !recognizedArea.box,
      }),
    );

  return [...mergedExisting, ...newAreas];
}

export function countAreaTypes(areas: StoreArea[]) {
  return areas.reduce(
    (counts, item) => {
      if (isTreatmentAreaType(item.type)) counts.treatment += 1;
      if (item.type === "consultation") counts.consultation += 1;
      if (item.type === "beauty") counts.beauty += 1;
      return counts;
    },
    { treatment: 0, consultation: 0, beauty: 0 },
  );
}

function businessAreaKey(area: StoreArea) {
  if (!area.type) return "";
  if (area.type === "vip_treatment" && !area.number.trim()) return "vip_treatment:";
  if (!area.number.trim()) return "";
  return `${area.type}:${Number(area.number.trim())}`;
}

function ensureAnnotationPlaceholder(area: StoreArea, index: number): StoreArea {
  if (area.box) return area;
  return withGeneratedAreaFields({
    ...area,
    box: pendingAnnotationBox(index),
    needsReview: true,
  });
}

function pendingAnnotationBox(index: number): AreaBox {
  return {
    x: 0.08 + (index % 4) * 0.18,
    y: 0.12 + (Math.floor(index / 4) % 3) * 0.16,
    width: 0.14,
    height: 0.1,
  };
}
