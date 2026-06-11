import type { AreaBox } from "../api";

export type UploadStage = "initial" | "converting" | "recognizing" | "ready" | "failed";
export type ResizeHandle = "nw" | "ne" | "sw" | "se";

export type DragState = {
  areaId: string;
  mode: "move" | "resize";
  handle?: ResizeHandle;
  startX: number;
  startY: number;
  origin: AreaBox;
};

export function clampBox(box: AreaBox): AreaBox {
  const width = Math.min(0.8, Math.max(0.04, box.width));
  const height = Math.min(0.8, Math.max(0.04, box.height));
  return {
    width,
    height,
    x: Math.min(1 - width, Math.max(0, box.x)),
    y: Math.min(1 - height, Math.max(0, box.y)),
  };
}

export function resizeBox(origin: AreaBox, handle: ResizeHandle, dx: number, dy: number): AreaBox {
  switch (handle) {
    case "nw":
      return {
        x: origin.x + dx,
        y: origin.y + dy,
        width: origin.width - dx,
        height: origin.height - dy,
      };
    case "ne":
      return {
        x: origin.x,
        y: origin.y + dy,
        width: origin.width + dx,
        height: origin.height - dy,
      };
    case "sw":
      return {
        x: origin.x + dx,
        y: origin.y,
        width: origin.width - dx,
        height: origin.height + dy,
      };
    case "se":
      return {
        x: origin.x,
        y: origin.y,
        width: origin.width + dx,
        height: origin.height + dy,
      };
  }
}

export function stageText(stage: UploadStage) {
  const stageMap: Record<UploadStage, string> = {
    initial: "待上传",
    converting: "解析图纸中",
    recognizing: "识别区域中",
    ready: "可编辑",
    failed: "识别失败，可手动维护",
  };
  return stageMap[stage];
}

export function planFileNameForStore(storeName: string, currentFileName: string) {
  const baseName = storeName
    .trim()
    .replace(/\s+/g, "-")
    .replace(/[\\/:*?"<>|]/g, "")
    .replace(/^-+|-+$/g, "");
  if (baseName) {
    return `${baseName}-design-plan.pdf`;
  }
  return currentFileName;
}
