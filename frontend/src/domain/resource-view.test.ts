import { describe, expect, it } from "vitest";

import type { ResourceStoreDetail } from "../api";
import { buildCameraBindingRows, issueSeverityRank, resourceDeviceOnlineLabel, sortedResourceIssues } from "./resource-view";

describe("resource view domain", () => {
  it("labels online state from business db values", () => {
    expect(resourceDeviceOnlineLabel(1)).toBe("在线");
    expect(resourceDeviceOnlineLabel(2)).toBe("离线");
    expect(resourceDeviceOnlineLabel(0)).toBe("未知");
  });

  it("sorts error issues before warnings and info", () => {
    const issues = sortedResourceIssues([
      { severity: "info", type: "space_bound_many_cameras", message: "info", entityType: "space", entityId: 1 },
      { severity: "error", type: "missing_camera", message: "error", entityType: "relation", entityId: 2 },
      { severity: "warning", type: "unbound_camera", message: "warning", entityType: "camera", entityId: 3 },
    ]);

    expect(issues.map((issue) => issue.severity)).toEqual(["error", "warning", "info"]);
    expect(issueSeverityRank("error")).toBeLessThan(issueSeverityRank("warning"));
  });

  it("builds camera rows from parent relationships instead of unstable level values", () => {
    const rows = buildCameraBindingRows({
      cameras: [
        camera({ id: 20, nvrId: 9, channelNo: 2 }),
        camera({ id: 10, nvrId: 9, channelNo: 1 }),
      ],
      nvrs: [device({ id: 9, hardwareId: "NVR-001" })],
      spaces: [
        space({ id: 1, name: "治疗区", level: 2 }),
        space({ id: 2, parentId: 1, name: "治疗室 1", level: 1 }),
        space({ id: 3, parentId: 2, name: "床位 1", level: 3 }),
      ],
      relations: [
        { id: 1, deviceId: 10, areaId: 3, functionType: "camera" },
        { id: 2, deviceId: 10, areaId: 3, functionType: "camera" },
      ],
    } as Pick<ResourceStoreDetail, "cameras" | "nvrs" | "spaces" | "relations">);

    expect(rows.map((row) => row.camera.id)).toEqual([10, 20]);
    expect(rows[0]).toMatchObject({
      recorderIdentifier: "NVR-001",
      isBound: true,
      bindingPaths: [{ level1: "治疗区", level2: "治疗室 1", level3: "床位 1", bed: "" }],
    });
    expect(rows[1]).toMatchObject({ isBound: false, bindingPaths: [] });
  });

  it("keeps multiple camera bindings and merges a fourth hierarchy segment into bed", () => {
    const rows = buildCameraBindingRows({
      cameras: [camera({ id: 10, nvrId: 9, channelNo: 1 })],
      nvrs: [device({ id: 9, hardwareId: "NVR-001" })],
      spaces: [
        space({ id: 1, name: "一级" }),
        space({ id: 2, parentId: 1, name: "二级" }),
        space({ id: 3, parentId: 2, name: "三级" }),
        space({ id: 4, parentId: 3, name: "床位 A" }),
        space({ id: 5, parentId: 1, name: "另一房间" }),
      ],
      relations: [
        { id: 1, deviceId: 10, areaId: 4, functionType: "camera" },
        { id: 2, deviceId: 10, areaId: 5, functionType: "camera" },
        { id: 3, deviceId: 999, areaId: 4, functionType: "camera" },
      ],
    } as Pick<ResourceStoreDetail, "cameras" | "nvrs" | "spaces" | "relations">);

    expect(rows[0].bindingPaths).toEqual([
      { level1: "一级", level2: "另一房间", level3: "", bed: "" },
      { level1: "一级", level2: "二级", level3: "三级", bed: "床位 A" },
    ]);
  });

  it("uses the NVRCHANNEL hardware id convention for recorder and channel columns", () => {
    const rows = buildCameraBindingRows({
      cameras: [camera({ id: 10, hardwareId: "NVRCHANNEL:22-10", nvrId: 0 })],
      nvrs: [device({ id: 22, hardwareId: "录像机硬件编号不用于展示" })],
      spaces: [],
      relations: [],
    } as Pick<ResourceStoreDetail, "cameras" | "nvrs" | "spaces" | "relations">);

    expect(rows[0]).toMatchObject({ recorderIdentifier: "22", channelNo: 10 });
  });
});

function device(overrides: Record<string, unknown>) {
  return {
    id: 1,
    parentId: 0,
    tenantId: 10001,
    name: "NVR",
    hardwareId: "",
    category: "nvr",
    status: 1,
    statusText: "启用",
    onlineStatus: 1,
    onlineText: "在线",
    ...overrides,
  };
}

function camera(overrides: Record<string, unknown>) {
  return { ...device({ category: "camera", name: "摄像头", ...overrides }), nvrId: 0, spacePaths: [] };
}

function space(overrides: Record<string, unknown>) {
  return {
    id: 1,
    tenantId: 10001,
    parentId: 0,
    name: "空间",
    level: 1,
    status: 1,
    statusText: "启用",
    dictId: 0,
    sortOrder: 0,
    boundCameraIds: [],
    boundCameraCount: 0,
    ...overrides,
  };
}
