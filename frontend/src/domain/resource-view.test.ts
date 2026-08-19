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

  it("builds camera rows from the bound space and its direct parent", () => {
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
      bindingPaths: [{ spaceType: "治疗室", spaceName: "床位 1" }],
    });
    expect(rows[1]).toMatchObject({ isBound: false, bindingPaths: [] });
  });

  it("keeps multiple camera bindings as space type and name pairs", () => {
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
      { spaceType: "三级", spaceName: "床位 A" },
      { spaceType: "一级", spaceName: "另一房间" },
    ]);
  });

  it("shows the direct parent name as space type for the business mapping example", () => {
    const rows = buildCameraBindingRows({
      cameras: [camera({ id: 70 })],
      nvrs: [],
      spaces: [
        space({ id: 2387, name: "诊室区域" }),
        space({ id: 2665, parentId: 2387, name: "产研中心" }),
        space({ id: 2667, parentId: 2665, name: "产研中心1-2" }),
      ],
      relations: [{ id: 2, deviceId: 70, areaId: 2667, functionType: "camera" }],
    } as Pick<ResourceStoreDetail, "cameras" | "nvrs" | "spaces" | "relations">);

    expect(rows[0].bindingPaths).toEqual([{ spaceType: "产研中心", spaceName: "产研中心1-2" }]);
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
  return { ...device({ category: "camera", name: "摄像头" }), nvrId: 0, spacePaths: [], ...overrides };
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
