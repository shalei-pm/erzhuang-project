import { describe, expect, it } from "vitest";
import { parseNVRLabRoute, validateNVRLabPlayback } from "./nvr-lab";

describe("parseNVRLabRoute", () => {
  it("only accepts the fixed 10001 experiment routes", () => {
    expect(parseNVRLabRoute("/erzhuang-project/h5/nvr-lab/10001")).toEqual({ name: "home" });
    expect(parseNVRLabRoute("/erzhuang-project/h5/nvr-lab/10001/cameras/111")).toEqual({ name: "camera", cameraId: 111 });
    expect(parseNVRLabRoute("/erzhuang-project/h5/nvr-lab/10019")).toBeNull();
  });
});

describe("validateNVRLabPlayback", () => {
  it("requires a complete range in the supported thirty-minute window", () => {
    expect(validateNVRLabPlayback(0, 100)).toBe("请选择完整的回放时间范围");
    expect(validateNVRLabPlayback(200, 100)).toBe("结束时间必须晚于开始时间");
    expect(validateNVRLabPlayback(100, 1901)).toBe("单次回放最长支持 30 分钟");
    expect(validateNVRLabPlayback(100, 1900)).toBe("");
  });
});
