export type MonitorScreenshotWatermarkMetadata = {
  watermarkEnabled: boolean;
  displayName?: string;
  capturedAt?: string;
};

export function watermarkLines(metadata: MonitorScreenshotWatermarkMetadata): string[] {
  if (!metadata.watermarkEnabled) return [];
  const displayName = metadata.displayName?.trim() || "";
  const capturedAt = metadata.capturedAt?.trim() || "";
  if (!displayName || !capturedAt) throw new Error("截图水印信息不完整");
  return [displayName, capturedAt];
}

export async function composeMonitorScreenshot(dataUrl: string, metadata: MonitorScreenshotWatermarkMetadata): Promise<string> {
  const lines = watermarkLines(metadata);
  if (lines.length === 0) return dataUrl;
  const image = await loadImage(dataUrl);
  const canvas = document.createElement("canvas");
  canvas.width = image.naturalWidth || image.width;
  canvas.height = image.naturalHeight || image.height;
  if (canvas.width <= 0 || canvas.height <= 0) throw new Error("截图画面无效");
  const context = canvas.getContext("2d");
  if (!context) throw new Error("当前浏览器无法生成截图水印");
  context.drawImage(image, 0, 0, canvas.width, canvas.height);

  const fontSize = Math.max(16, Math.round(canvas.width / 48));
  const paddingX = Math.max(12, Math.round(fontSize * 0.75));
  const paddingY = Math.max(9, Math.round(fontSize * 0.55));
  const lineHeight = Math.round(fontSize * 1.35);
  const margin = Math.max(16, Math.round(canvas.width / 50));
  context.font = `600 ${fontSize}px sans-serif`;
  const contentWidth = Math.max(...lines.map((line) => context.measureText(line).width));
  const panelWidth = Math.ceil(contentWidth + paddingX * 2);
  const panelHeight = lineHeight * lines.length + paddingY * 2;
  const left = Math.max(0, canvas.width - margin - panelWidth);
  const top = Math.max(0, margin);
  context.fillStyle = "rgba(15, 23, 42, 0.68)";
  context.fillRect(left, top, panelWidth, panelHeight);
  context.fillStyle = "#ffffff";
  context.textAlign = "right";
  context.textBaseline = "middle";
  lines.forEach((line, index) => context.fillText(line, left + panelWidth - paddingX, top + paddingY + lineHeight * index + lineHeight / 2));

  try {
    return canvas.toDataURL("image/png");
  } catch {
    throw new Error("截图水印生成失败");
  }
}

function loadImage(dataUrl: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error("截图画面加载失败"));
    image.src = dataUrl;
  });
}
