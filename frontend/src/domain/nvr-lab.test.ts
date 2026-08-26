import { describe, expect, it } from "vitest";
import { buildNVRLabHourlyPlayback, buildNVRLabPlaybackFromStart, parseNVRLabRoute, validateNVRLabPlayback } from "./nvr-lab";

describe("parseNVRLabRoute", () => {
  it("only accepts the fixed 10001 experiment routes", () => {
    expect(parseNVRLabRoute("/erzhuang-project/h5/nvr-lab/10001")).toEqual({ name: "home" });
    expect(parseNVRLabRoute("/erzhuang-project/h5/nvr-lab/10001/cameras/111")).toEqual({ name: "camera", cameraId: 111 });
    expect(parseNVRLabRoute("/erzhuang-project/h5/nvr-lab/10019")).toBeNull();
  });
});

describe("validateNVRLabPlayback", () => {
  it("requires a complete range in the supported one-hour window", () => {
    expect(validateNVRLabPlayback(0, 100)).toBe("请选择完整的回放时间范围");
    expect(validateNVRLabPlayback(200, 100)).toBe("结束时间必须晚于开始时间");
    expect(validateNVRLabPlayback(100, 3701)).toBe("单次回放最长支持 1 小时");
    expect(validateNVRLabPlayback(100, 3700)).toBe("");
  });

  it("builds a one-hour range for a historical hourly shortcut", () => {
    const expectedStart = Math.floor(new Date("2026-08-20T10:00:00").getTime() / 1000);
    const expectedEnd = Math.floor(new Date("2026-08-20T11:00:00").getTime() / 1000);

    expect(buildNVRLabHourlyPlayback("2026-08-20", 10, new Date("2026-08-26T10:18:00"))).toEqual({
      startAt: "2026-08-20T10:00",
      endAt: "2026-08-20T11:00",
      startTime: expectedStart,
      endTime: expectedEnd,
    });
  });

  it("clips the current hour and rejects future starts", () => {
    expect(buildNVRLabHourlyPlayback("2026-08-26", 10, new Date("2026-08-26T10:18:45"))).toMatchObject({
      startAt: "2026-08-26T10:00",
      endAt: "2026-08-26T10:18",
    });
    expect(buildNVRLabHourlyPlayback("2026-08-26", 11, new Date("2026-08-26T10:18:45"))).toBeNull();
    expect(buildNVRLabPlaybackFromStart("2026-08-26T10:18", new Date("2026-08-26T10:18:45"))).toBeNull();
  });
});
