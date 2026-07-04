import { test } from "vitest";

import {
  channelNetworkErrorMessage,
  channelRecognitionMessage,
  isChannelRecognitionFailed,
  recorderRecognitionRunToast,
  recorderRecognitionToast,
  shouldBatchRecognizeChannel,
} from "./channel-recognition.js";

const limitResult = {
  status: "recognition_failed",
  message: "ezviz api error code=10028 msg=抓图接口调用次数超限",
  capture_ms: 47,
  recognition_ms: 0,
  total_ms: 47,
};

test("formats channel recognition messages and batch retry decisions", () => {
  assertIncludes(
    channelRecognitionMessage({ recognitionResult: limitResult }),
    "抓图接口调用次数超限",
  );

  assertEqual(
    recorderRecognitionToast("GG9803685", [
      { recognitionResult: limitResult },
      { recognitionResult: JSON.stringify(limitResult) },
    ]),
    "GG9803685 识别完成，但 2 个通道抓图/识别失败：ezviz api error code=10028 msg=抓图接口调用次数超限",
  );

  assertEqual(
    recorderRecognitionToast("GG9803685", [
      { recognitionResult: { status: "recognized", total_ms: 1200 } },
      { recognitionResult: limitResult },
    ]),
    "GG9803685 识别完成，1/2 个通道抓图/识别失败：ezviz api error code=10028 msg=抓图接口调用次数超限",
  );

  assertEqual(
    recorderRecognitionToast("GG9803685", [
      { recognitionResult: { status: "recognized", total_ms: 1200 } },
    ]),
    "已完成 GG9803685 的通道识别。",
  );

  assertEqual(
    shouldBatchRecognizeChannel({ status: "pending_confirmation", recognitionResult: { status: "recognized" } }) ? "yes" : "no",
    "no",
  );

  assertEqual(
    shouldBatchRecognizeChannel({ status: "recognition_failed", recognitionResult: limitResult }) ? "yes" : "no",
    "yes",
  );

  assertEqual(
    shouldBatchRecognizeChannel({ status: "recognition_failed", recognitionAttempts: 2, recognitionResult: limitResult }) ? "yes" : "no",
    "no",
  );

  assertEqual(
    shouldBatchRecognizeChannel({ status: "pending_recognition", recognitionResult: undefined }) ? "yes" : "no",
    "yes",
  );

  assertEqual(
    isChannelRecognitionFailed({ status: "recognition_failed", recognitionResult: limitResult }) ? "yes" : "no",
    "yes",
  );

  assertEqual(
    channelNetworkErrorMessage(new TypeError("Failed to fetch")),
    "识别请求中断，可能是单路识别耗时过长或公司网关超时，已继续识别后续通道。",
  );

  assertEqual(
    recorderRecognitionRunToast("FK8984413", {
      total: 23,
      completed: 20,
      failed: 1,
      interrupted: 2,
      firstError: "通道 21：识别请求中断，可能是单路识别耗时过长或公司网关超时，已继续识别后续通道。",
    }),
    "FK8984413 识别完成 20/23，失败 1 个，请求中断 2 个，通道 21：识别请求中断，可能是单路识别耗时过长或公司网关超时，已继续识别后续通道。",
  );

  assertEqual(
    recorderRecognitionRunToast("FK8984413", { total: 0, completed: 0, failed: 0, interrupted: 0 }),
    "暂无需要识别的通道，FK8984413 已识别成功的通道不会重复消耗模型。",
  );
});

function assertEqual(actual: string, expected: string) {
  if (actual !== expected) {
    throw new Error(`expected ${expected}, got ${actual}`);
  }
}

function assertIncludes(actual: string, expected: string) {
  if (!actual.includes(expected)) {
    throw new Error(`expected ${actual} to include ${expected}`);
  }
}
