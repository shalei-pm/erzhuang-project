import type { ResourceCamera, ResourceIssue, ResourceIssueSeverity, ResourceSpace, ResourceStoreDetail } from "../api";

export type CameraBindingPath = {
  spaceType: string;
  spaceName: string;
};

export type CameraBindingRow = {
  camera: ResourceCamera;
  recorderIdentifier: string;
  channelNo?: number;
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
      const bindingPaths = pathsByCameraID.get(camera.id) ?? [];
      const encodedNvrChannel = parseNvrChannel(camera.hardwareId);
      const nvr = nvrByID.get(encodedNvrChannel?.nvrID ?? camera.nvrId);
      return {
        camera,
        recorderIdentifier: encodedNvrChannel ? `${encodedNvrChannel.nvrID}` : nvr?.hardwareId || nvr?.sn || camera.nvrName || (camera.nvrId ? `${camera.nvrId}` : "-"),
        channelNo: encodedNvrChannel?.channelNo ?? camera.channelNo,
        bindingPaths,
        isBound: bindingPaths.length > 0,
      };
    })
    .sort((left, right) => {
      const recorderDiff = left.recorderIdentifier.localeCompare(right.recorderIdentifier, "zh-CN");
      if (recorderDiff !== 0) return recorderDiff;
      const channelDiff = numericChannel(left.channelNo) - numericChannel(right.channelNo);
      if (channelDiff !== 0) return channelDiff;
      return left.camera.id - right.camera.id;
    });
}

function buildCameraBindingPath(space: ResourceSpace, spacesByID: Map<number, ResourceSpace>): CameraBindingPath {
  const parent = space.parentId ? spacesByID.get(space.parentId) : undefined;

  return {
    spaceType: space.level === 3 ? "治疗室" : parent?.name.trim() || "",
    spaceName: space.name.trim() || `空间 ${space.id}`,
  };
}

function numericChannel(channelNo: number | undefined) {
  return channelNo ?? Number.MAX_SAFE_INTEGER;
}

function parseNvrChannel(hardwareID: string) {
  const match = /(?:^|\s)NVRCHANNEL:(\d+)-(\d+)(?:\s|$)/i.exec(hardwareID.trim());
  if (!match) return null;
  return { nvrID: Number(match[1]), channelNo: Number(match[2]) };
}
