import type { ResourceCamera, ResourceIssue, ResourceIssueSeverity, ResourceSpace, ResourceStoreDetail } from "../api";

export type CameraBindingPath = {
  level1: string;
  level2: string;
  level3: string;
  bed: string;
};

export type CameraBindingRow = {
  camera: ResourceCamera;
  recorderIdentifier: string;
  bindingPaths: CameraBindingPath[];
  isBound: boolean;
};

export function resourceDeviceOnlineLabel(value: number | null | undefined) {
  if (value === 1) return "在线";
  if (value === 2) return "离线";
  return "未知";
}

export function issueSeverityRank(severity: ResourceIssueSeverity) {
  if (severity === "error") return 1;
  if (severity === "warning") return 2;
  return 3;
}

export function sortedResourceIssues(issues: ResourceIssue[]) {
  return [...issues].sort((left, right) => {
    const severityDiff = issueSeverityRank(left.severity) - issueSeverityRank(right.severity);
    if (severityDiff !== 0) return severityDiff;
    return left.type.localeCompare(right.type);
  });
}

export function buildCameraBindingRows(store: Pick<ResourceStoreDetail, "cameras" | "nvrs" | "spaces" | "relations">): CameraBindingRow[] {
  const spacesByID = new Map(store.spaces.map((space) => [space.id, space]));
  const camerasByID = new Map(store.cameras.map((camera) => [camera.id, camera]));
  const nvrByID = new Map(store.nvrs.map((nvr) => [nvr.id, nvr]));
  const pathsByCameraID = new Map<number, CameraBindingPath[]>();
  const seenBindings = new Set<string>();

  for (const relation of store.relations) {
    const camera = camerasByID.get(relation.deviceId);
    const space = spacesByID.get(relation.areaId);
    const key = `${relation.deviceId}:${relation.areaId}`;
    if (!camera || !space || seenBindings.has(key)) continue;

    seenBindings.add(key);
    const paths = pathsByCameraID.get(camera.id) ?? [];
    paths.push(buildCameraBindingPath(space, spacesByID));
    pathsByCameraID.set(camera.id, paths);
  }

  return store.cameras
    .map((camera) => {
      const bindingPaths = (pathsByCameraID.get(camera.id) ?? []).sort(compareBindingPaths);
      const nvr = nvrByID.get(camera.nvrId);
      return {
        camera,
        recorderIdentifier: nvr?.hardwareId || nvr?.sn || camera.nvrName || (camera.nvrId ? `NVR ${camera.nvrId}` : "-"),
        bindingPaths,
        isBound: bindingPaths.length > 0,
      };
    })
    .sort((left, right) => {
      const recorderDiff = left.recorderIdentifier.localeCompare(right.recorderIdentifier, "zh-CN");
      if (recorderDiff !== 0) return recorderDiff;
      const channelDiff = numericChannel(left.camera.channelNo) - numericChannel(right.camera.channelNo);
      if (channelDiff !== 0) return channelDiff;
      return left.camera.id - right.camera.id;
    });
}

function buildCameraBindingPath(space: ResourceSpace, spacesByID: Map<number, ResourceSpace>): CameraBindingPath {
  const chain: ResourceSpace[] = [];
  const seenSpaceIDs = new Set<number>();
  let current: ResourceSpace | undefined = space;

  while (current && !seenSpaceIDs.has(current.id)) {
    chain.push(current);
    seenSpaceIDs.add(current.id);
    current = current.parentId ? spacesByID.get(current.parentId) : undefined;
  }

  const names = chain
    .reverse()
    .map((item) => item.name.trim() || `空间 ${item.id}`);

  return {
    level1: names[0] ?? "",
    level2: names[1] ?? "",
    level3: names[2] ?? "",
    bed: names.length > 4 ? names.slice(3).join(" / ") : names[3] ?? "",
  };
}

function compareBindingPaths(left: CameraBindingPath, right: CameraBindingPath) {
  return `${left.level1}\u0000${left.level2}\u0000${left.level3}\u0000${left.bed}`.localeCompare(
    `${right.level1}\u0000${right.level2}\u0000${right.level3}\u0000${right.bed}`,
    "zh-CN",
  );
}

function numericChannel(channelNo: number | undefined) {
  return channelNo ?? Number.MAX_SAFE_INTEGER;
}
