import type { ResourceIssue, ResourceIssueSeverity } from "../api";

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
