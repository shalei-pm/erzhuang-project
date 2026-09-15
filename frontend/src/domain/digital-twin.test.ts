import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  DIGITAL_TWIN_REFRESH_INTERVAL_MS,
  cameraRegion,
  chooseTwinStore,
  dashboardRefreshFailed,
  dashboardRefreshSucceeded,
  parseInstitutionID,
  regionCameras,
  startDigitalTwinRefreshPolling,
} from "./digital-twin";

describe("digital twin live metric polling", () => {
  let visibility: EventTarget & { visibilityState: string };

  beforeEach(() => {
    vi.useFakeTimers();
    visibility = Object.assign(new EventTarget(), { visibilityState: "visible" });
    vi.stubGlobal("document", visibility);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("refreshes every 30 seconds while the page is visible", async () => {
    const refresh = vi.fn();
    const stop = startDigitalTwinRefreshPolling(refresh);

    await vi.advanceTimersByTimeAsync(DIGITAL_TWIN_REFRESH_INTERVAL_MS - 1);
    expect(refresh).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(refresh).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(DIGITAL_TWIN_REFRESH_INTERVAL_MS);
    expect(refresh).toHaveBeenCalledTimes(2);

    stop();
  });

  it("pauses while hidden and refreshes immediately when visible again", async () => {
    const refresh = vi.fn();
    const stop = startDigitalTwinRefreshPolling(refresh);

    visibility.visibilityState = "hidden";
    visibility.dispatchEvent(new Event("visibilitychange"));
    await vi.advanceTimersByTimeAsync(DIGITAL_TWIN_REFRESH_INTERVAL_MS * 2);
    expect(refresh).not.toHaveBeenCalled();

    visibility.visibilityState = "visible";
    visibility.dispatchEvent(new Event("visibilitychange"));
    expect(refresh).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(DIGITAL_TWIN_REFRESH_INTERVAL_MS - 1);
    expect(refresh).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(refresh).toHaveBeenCalledTimes(2);

    stop();
  });

  it("cleans up its timer and visibility listener", async () => {
    const refresh = vi.fn();
    const stop = startDigitalTwinRefreshPolling(refresh);
    stop();

    await vi.advanceTimersByTimeAsync(DIGITAL_TWIN_REFRESH_INTERVAL_MS);
    visibility.dispatchEvent(new Event("visibilitychange"));
    expect(refresh).not.toHaveBeenCalled();
  });

  it("keeps the last successful dashboard when a refresh fails", () => {
    const previous = dashboardRefreshSucceeded(null, { tenant_id: 10001, arrived: 6 });
    expect(previous).toEqual({ value: { tenant_id: 10001, arrived: 6 }, stale: false });
    expect(dashboardRefreshFailed(previous)).toEqual({ value: previous.value, stale: true });
  });
});

describe("digital twin routing and mapping", () => {
  it("defaults to Poly only when allowed", () => {
    expect(chooseTwinStore(["10042", "10001"], null)).toBe("10001");
    expect(chooseTwinStore(["10042"], null)).toBe("10042");
    expect(chooseTwinStore([], null)).toBeNull();
    expect(chooseTwinStore(["10042"], "10001")).toBeNull();
  });
  it("does not infer camera identity or region from an unbound name", () => {
    expect(cameraRegion({id: 111, name: "治疗室4"})).toBeNull();
    expect(cameraRegion({id: 111, name: "设备", space_type: "治疗室"})).toBe("treatment");
    expect(cameraRegion({id: 72, name: "设备", space_type: "面诊室"})).toBe("consultation");
    expect(cameraRegion({id: 73, name: "设备", space_type: "走廊"})).toBeNull();
    expect(cameraRegion({id: 74, name: "设备", space_type: "术后护理"})).toBeNull();
    expect(cameraRegion({id: 75, name: "设备", space_type: "公共区域", space_name:"护士站"})).toBe("reception");
    expect(cameraRegion({id: 76, name: "设备", space_type: "等待区"})).toBe("waiting");
    expect(cameraRegion({id: 77, name: "设备", space_type: "等候区"})).toBe("waiting");
    expect(cameraRegion({id: 79, name: "设备", space_type: "公共区域", space_name: "候诊区"})).toBe("waiting");
    expect(cameraRegion({id: 78, name: "设备", space_type: "咨询办公室"})).toBeNull();
    expect(regionCameras([
      {id:75,name:"设备",space_name:"护士站"},
      {id:76,name:"设备",space_type:"前台"},
    ], "reception").map(camera=>camera.id)).toEqual([76,75]);
  });
  it("accepts only canonical positive institution IDs", () => {
    expect(parseInstitutionID(" 10001 ")).toBe("10001");
    for (const value of ["0", "01", "1e4", "-1", "10001/other", ""]) expect(parseInstitutionID(value)).toBeNull();
  });
  it("sorts treatment rooms numerically before VIP and special rooms without mutating the source", () => {
    const cameras = ["产研中心治疗室", "VIP治疗室10", "治疗室10", "治疗室2", "vip治疗室2", "治疗室1", "总部实验治疗室"]
      .map((space_name, id) => ({ id, name: "设备", space_type: "治疗室", space_name }));
    const original = cameras.map(camera => camera.id);
    expect(regionCameras(cameras, "treatment").map(camera => camera.space_name)).toEqual([
      "治疗室1", "治疗室2", "治疗室10", "vip治疗室2", "VIP治疗室10", "产研中心治疗室", "总部实验治疗室",
    ]);
    expect(cameras.map(camera => camera.id)).toEqual(original);
  });
});
