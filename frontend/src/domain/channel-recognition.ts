type ChannelRecognitionLike = {
  status?: string;
  recognitionAttempts?: number;
  recognitionResult?: unknown;
};

type ChannelRecognitionResult = {
  status?: string;
  message?: string;
  confidence?: string;
  recognition_ms?: number;
  total_ms?: number;
};

export type ChannelRecognitionRunSummary = {
  total: number;
  completed: number;
  failed: number;
  interrupted: number;
  firstError?: string;
};

export function channelRecognitionMessage(channel: ChannelRecognitionLike) {
  const result = channel.recognitionResult;
  if (!result) return "";
  if (typeof result === "string") {
    try {
      return channelRecognitionMessageFromObject(JSON.parse(result));
    } catch {
      return result;
    }
  }
  if (typeof result === "object" && result) {
    return channelRecognitionMessageFromObject(result);
  }
  return "";
}

export function shouldBatchRecognizeChannel(channel: ChannelRecognitionLike) {
  if (channel.status === "inactive") return false;
  if ((channel.recognitionAttempts ?? 0) >= 2) return false;
  const result = parseRecognitionResult(channel.recognitionResult);
  if (!result) return true;
  if (result.status === "recognized") return false;
  return true;
}

export function isChannelRecognitionFailed(channel: ChannelRecognitionLike) {
  if (channel.status === "recognition_failed") return true;
  const result = parseRecognitionResult(channel.recognitionResult);
  return result?.status === "capture_failed" || result?.status === "recognition_failed";
}

export function channelNetworkErrorMessage(error: unknown) {
  if (!isNetworkFetchError(error)) return "";
  return "识别请求中断，可能是单路识别耗时过长或公司网关超时，已继续识别后续通道。";
}

export function recorderRecognitionRunToast(deviceCode: string, summary: ChannelRecognitionRunSummary) {
  if (summary.total === 0) {
    return `暂无需要识别的通道，${deviceCode} 已识别成功的通道不会重复消耗模型。`;
  }
  if (summary.failed === 0 && summary.interrupted === 0) {
    return `已完成 ${deviceCode} 的通道识别，共 ${summary.completed}/${summary.total} 个。`;
  }
  const parts = [`${deviceCode} 识别完成 ${summary.completed}/${summary.total}`];
  if (summary.failed > 0) parts.push(`失败 ${summary.failed} 个`);
  if (summary.interrupted > 0) parts.push(`请求中断 ${summary.interrupted} 个`);
  if (summary.firstError) parts.push(summary.firstError);
  return combineParts(parts, "，");
}

export function recorderRecognitionToast(deviceCode: string, channels: ChannelRecognitionLike[]) {
  const failedMessages = channels
    .map(channelRecognitionFailureMessage)
    .filter((message): message is string => Boolean(message));

  if (failedMessages.length === 0) {
    return `已完成 ${deviceCode} 的通道识别。`;
  }

  const firstMessage = failedMessages[0];
  if (failedMessages.length === channels.length) {
    return `${deviceCode} 识别完成，但 ${failedMessages.length} 个通道抓图/识别失败：${firstMessage}`;
  }
  return `${deviceCode} 识别完成，${failedMessages.length}/${channels.length} 个通道抓图/识别失败：${firstMessage}`;
}

function channelRecognitionMessageFromObject(value: unknown) {
  if (!value || typeof value !== "object") return "";
  const result = value as ChannelRecognitionResult;
  const timing = recognitionTimingLabel(result);
  if (result.status === "capture_failed" || result.status === "recognition_failed") {
    return combineParts(["失败", result.message, timing], " · ");
  }
  if (result.status === "recognized") {
    const confidence = result.confidence === "low" ? "低置信" : "";
    return combineParts([confidence, timing], " · ");
  }
  if (result.status === "captured") {
    return combineParts(["抓图", timing], " · ");
  }
  return result.message || timing;
}

function channelRecognitionFailureMessage(channel: ChannelRecognitionLike) {
  const result = parseRecognitionResult(channel.recognitionResult);
  if (!result || (result.status !== "capture_failed" && result.status !== "recognition_failed")) {
    return "";
  }
  return result.message || channelRecognitionMessageFromObject(result);
}

export function parseRecognitionResult(result: unknown): ChannelRecognitionResult | null {
  if (!result) return null;
  if (typeof result === "string") {
    try {
      return parseRecognitionResult(JSON.parse(result));
    } catch {
      return { status: "recognition_failed", message: result };
    }
  }
  if (typeof result === "object") {
    return result as ChannelRecognitionResult;
  }
  return null;
}

function isNetworkFetchError(error: unknown) {
  if (!(error instanceof TypeError)) return false;
  return error.message.trim().toLowerCase() === "failed to fetch";
}

function recognitionTimingLabel(result: { recognition_ms?: number; total_ms?: number }) {
  const parts: string[] = [];
  if (typeof result.recognition_ms === "number" && result.recognition_ms > 0) {
    parts.push(`识别 ${formatDuration(result.recognition_ms)}`);
  }
  if (typeof result.total_ms === "number" && result.total_ms > 0) {
    parts.push(`总 ${formatDuration(result.total_ms)}`);
  }
  return combineParts(parts, " / ");
}

function combineParts(parts: Array<string | undefined>, separator: string) {
  let output = "";
  for (const part of parts) {
    if (!part) continue;
    if (output) output += separator;
    output += part;
  }
  return output;
}

function formatDuration(ms: number) {
  if (ms >= 1000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.max(1, Math.round(ms))}ms`;
}
