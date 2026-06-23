import { describe, expect, it } from "vitest";

import { channelRecognitionMessage, recorderRecognitionToast } from "./channel-recognition.js";

const limitResult = {
  status: "recognition_failed",
  message: "ezviz api error code=10028 msg=抓图接口调用次数超限",
  capture_ms: 47,
  recognition_ms: 0,
  total_ms: 47,
};

describe("channel recognition messages", () => {
  it("shows the Ezviz capture limit in channel row messages", () => {
    expect(channelRecognitionMessage({ recognitionResult: limitResult })).toContain("抓图接口调用次数超限");
  });

  it("does not claim success when all recorder channels fail capture or recognition", () => {
    expect(
      recorderRecognitionToast("GG9803685", [
        { recognitionResult: limitResult },
        { recognitionResult: JSON.stringify(limitResult) },
      ]),
    ).toBe("GG9803685 识别完成，但 2 个通道抓图/识别失败：ezviz api error code=10028 msg=抓图接口调用次数超限");
  });

  it("summarizes partial failures after recorder recognition", () => {
    expect(
      recorderRecognitionToast("GG9803685", [
        { recognitionResult: { status: "recognized", total_ms: 1200 } },
        { recognitionResult: limitResult },
      ]),
    ).toBe("GG9803685 识别完成，1/2 个通道抓图/识别失败：ezviz api error code=10028 msg=抓图接口调用次数超限");
  });

  it("keeps the success message when every channel succeeds", () => {
    expect(
      recorderRecognitionToast("GG9803685", [
        { recognitionResult: { status: "recognized", total_ms: 1200 } },
      ]),
    ).toBe("已完成 GG9803685 的通道识别。");
  });
});
