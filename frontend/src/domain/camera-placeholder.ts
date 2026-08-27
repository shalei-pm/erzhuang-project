export type CameraThumbnailKind = "consultation" | "treatment" | "reception" | "waiting" | "corridor" | "utility" | "unassigned";

const knownKinds = new Set<CameraThumbnailKind>([
  "consultation",
  "treatment",
  "reception",
  "waiting",
  "corridor",
  "utility",
  "unassigned",
]);

export function cameraPlaceholderURL(kind?: string): string {
  const resolvedKind: CameraThumbnailKind = knownKinds.has(kind as CameraThumbnailKind) ? (kind as CameraThumbnailKind) : "unassigned";
  const configuredBase = import.meta.env.BASE_URL || "";
  const base = (configuredBase === "/" ? "/erzhuang-project/" : configuredBase || "/erzhuang-project/").replace(/\/?$/, "/");
  return `${base}camera-placeholders/${resolvedKind}.png`;
}

export function legacyCameraThumbnailKind(input: { areaType?: string; category?: string; sceneType?: string }): CameraThumbnailKind {
  if (input.areaType === "consultation") return "consultation";
  if (input.areaType === "treatment" || input.areaType === "vip_treatment" || input.areaType === "beauty") return "treatment";

  switch (input.sceneType) {
    case "front_desk":
      return "reception";
    case "waiting_area":
    case "hall":
      return "waiting";
    case "corridor":
    case "passage":
    case "entrance":
      return "corridor";
    case "storage":
    case "pharmacy":
    case "machine_room":
      return "utility";
    default:
      return input.category === "other" ? "utility" : "unassigned";
  }
}
