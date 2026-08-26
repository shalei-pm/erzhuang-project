export function trimTrailingSlash(value: string) {
  return value.replace(/\/+$/, "");
}

export function defaultApiBase(namespace: "design-plan" | "store-space", configuredBase = "/erzhuang/") {
  return `${runtimeBasePath(configuredBase)}/api/${namespace}`;
}

export function displayImageUrl(value: string | undefined, options: { apiBase: string; mockPlanImage: string }) {
  if (!value) {
    return "";
  }
  if (value.startsWith("mock/")) {
    return options.mockPlanImage;
  }
  const storedUploadMatch = value.match(/^uploads\/([^/]+)\/(preview|thumbnail)\.png$/);
  if (storedUploadMatch) {
    return apiPathFromBase(options.apiBase, `/uploads/${storedUploadMatch[1]}/${storedUploadMatch[2]}`);
  }
  if (/^\/api\/design-plan\/uploads\/[^/]+\/(preview|thumbnail)$/.test(value)) {
    return apiPathFromBase(options.apiBase, stripApiPrefix(value, "design-plan"));
  }
  if (/^\/api\/design-plan\/stores\/\d+\/(preview|thumbnail)$/.test(value)) {
    return apiPathFromBase(options.apiBase, stripApiPrefix(value, "design-plan"));
  }
  if (/^\/api\/store-space\/channel-snapshots\/[^/]+$/.test(value)) {
    return apiPathFromBase(options.apiBase, stripApiPrefix(value, "store-space"));
  }
  if (/^\/api\/store-space-resource-view\/stores\/\d+\/cameras\/\d+\/snapshot$/.test(value)) {
    return apiPathFromBase(options.apiBase, value.replace(/^\/api/, ""));
  }
  if (value.startsWith("/api/")) {
    return apiPathFromBase(options.apiBase, value.replace(/^\/api\/[^/]+/, ""));
  }
  return value;
}

export function storedImagePath(value: string, fallback: string) {
  if (!value || value.startsWith("data:") || value.startsWith("blob:")) {
    return fallback;
  }
  const uploadMatch = value.match(/^\/(?:[^/]+\/)?api\/design-plan\/uploads\/([^/]+)\/(preview|thumbnail)$/);
  if (uploadMatch) {
    return `uploads/${uploadMatch[1]}/${uploadMatch[2] === "preview" ? "preview.png" : "thumbnail.png"}`;
  }
  return value;
}

function apiPathFromBase(apiBase: string, path: string) {
  return `${trimTrailingSlash(apiBase)}${path.startsWith("/") ? path : `/${path}`}`;
}

function stripApiPrefix(value: string, namespace: "design-plan" | "store-space") {
  return value.replace(new RegExp(`^/api/${namespace}`), "");
}

function runtimeBasePath(configuredBase: string) {
  if (typeof window === "undefined") {
    return trimTrailingSlash(configuredBase) || "";
  }
  const normalizedBase = configuredBase.startsWith("/") ? configuredBase : `/${configuredBase}`;
  const baseSegments = trimTrailingSlash(normalizedBase).split("/").filter(Boolean);
  if (baseSegments.length === 0) {
    return "";
  }
  const currentSegments = window.location.pathname.split("/").filter(Boolean);
  if (currentSegments.length > 0 && currentSegments[0] !== baseSegments[0]) {
    return `/${currentSegments[0]}`;
  }
  return `/${baseSegments[0]}`;
}
