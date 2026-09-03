import { ApiError } from "../api";

export function formatDateTime(value: string) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

export function errorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError) {
    const knownMessages: Record<string, string> = {
      "list audit logs failed": "操作日志加载失败，请检查审计日志表配置后重试。",
    };
    if (knownMessages[error.message]) {
      return knownMessages[error.message];
    }
    if (error.status === 413) {
      return "文件过大，请上传 5MB 以内的 PDF。";
    }
    if (error.status === 504) {
      return "AI 识别超时，请换一张更小或更清晰的 PDF，或稍后重试。";
    }
    const fieldMessages = Object.values(error.fields).filter(Boolean);
    if (fieldMessages.length > 0) {
      return fieldMessages.join("；");
    }
  }
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return fallback;
}
