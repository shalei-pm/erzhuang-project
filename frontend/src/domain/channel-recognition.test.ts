import { channelRecognitionMessage, recorderRecognitionToast } from "./channel-recognition.js";

const limitResult = {
  status: "recognition_failed",
  message: "ezviz api error code=10028 msg=抓图接口调用次数超限",
  capture_ms: 47,
  recognition_ms: 0,
  total_ms: 47,
};

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

console.log("channel-recognition tests passed");

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
