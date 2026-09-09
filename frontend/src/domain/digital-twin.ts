import type { NVRLabCamera } from "./nvr-lab";

export function parseInstitutionID(value: string): string | null {
  const id = value.trim();
  return /^[1-9][0-9]{0,17}$/.test(id) ? id : null;
}

export function chooseTwinStore(ids: string[], requested: string | null): string | null {
  if (requested !== null) return ids.includes(requested) ? requested : null;
  return ids.includes("10001") ? "10001" : ids[0] ?? null;
}

export function cameraRegion(camera: NVRLabCamera): TwinRegionId | null {
  const area = `${camera.space_type || ""} ${camera.space_name || ""}`;
  if (/术后护理/.test(area)) return null;
  if (/治疗室/.test(area)) return "treatment";
  if (/面诊室/.test(area)) return "consultation";
  if (/等候区|等待区/.test(area)) return "waiting";
  if (/前台|护士站/.test(area)) return "reception";
  return null;
}

export function regionCameras(cameras: NVRLabCamera[], region: TwinRegionId): NVRLabCamera[] {
  const matching = cameras.filter(camera => cameraRegion(camera) === region);
  if (region === "reception") {
    const priority = (camera: NVRLabCamera) => /前台/.test(`${camera.space_type || ""} ${camera.space_name || ""}`) ? 0 : 1;
    matching.sort((a, b) => priority(a) - priority(b));
  }
  return matching;
}

export function digitalTwinPath(): string {
  return `${import.meta.env.BASE_URL}digitaltwin/`;
}
