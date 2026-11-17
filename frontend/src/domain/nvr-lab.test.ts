import { describe, expect, it } from "vitest";
import { buildNVRLabHourlyPlayback, buildNVRLabPlaybackFromStart, buildNVRLabPlaybackSession, nvrMonitorRoutePath, parseNVRLabRoute, parseNVRMonitorRoute, validateNVRLabPlayback, nvrPlaybackCursor, nvrPlaybackForward } from "./nvr-lab";

describe("recording playback position", () => {
  it("uses presented media seconds without multiplying the selected rate again", () => {
    expect(nvrPlaybackCursor(1000, 32.5, 4600)).toBe(1032.5);
    expect(nvrPlaybackCursor(1032.5, 2, 4600)).toBe(1034.5);
    expect(nvrPlaybackCursor(1000, 5000, 4600)).toBe(4599);
  });
  it("rejects invalid progress and advances exactly ten recording seconds", () => {
    expect(nvrPlaybackCursor(1000, Number.NaN, 4600)).toBeNull();
    expect(nvrPlaybackCursor(1000, -1, 4600)).toBeNull();
    expect(nvrPlaybackForward(1032.5, 1000, 4600)).toBe(1042.5);
    expect(nvrPlaybackForward(4595, 1000, 4600)).toBe(4599);
  });
});

describe("parseNVRLabRoute", () => {
  it("only accepts the fixed 10001 experiment routes", () => {
    expect(parseNVRLabRoute("/erzhuang-project/h5/nvr-lab/10001")).toEqual({ name: "home" });
    expect(parseNVRLabRoute("/erzhuang-project/h5/nvr-lab/10001/cameras/111")).toEqual({ name: "camera", cameraId: 111 });
    expect(parseNVRLabRoute("/erzhuang-project/h5/nvr-lab/10019")).toBeNull();
  });
});

describe("parseNVRMonitorRoute", () => {
  it("accepts normal organization monitor routes", () => {
    expect(parseNVRMonitorRoute("/erzhuang-project/h5/orgs/10019/monitor")).toEqual({ name: "home", externalOrgId: "10019" });
    expect(parseNVRMonitorRoute("/erzhuang-project/h5/orgs/10019/monitor/cameras/111")).toEqual({ name: "camera", externalOrgId: "10019", cameraId: 111 });
    expect(nvrMonitorRoutePath({ name: "camera", externalOrgId: "10019", cameraId: 111 })).toBe("/h5/orgs/10019/monitor/cameras/111");
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

  it("keeps the selected hour as the slider window when seeking within playback", () => {
    const window = {
      startAt: "2026-08-20T10:00",
      endAt: "2026-08-20T11:00",
      startTime: 1_000,
      endTime: 4_600,
    };

    expect(buildNVRLabPlaybackSession(window, 2_800)).toEqual({
      playbackWindow: window,
      startTime: 2_800,
      endTime: 4_600,
    });
  });
});
