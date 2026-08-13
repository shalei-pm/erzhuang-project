import { describe, expect, it } from "vitest";

import { issueSeverityRank, resourceDeviceOnlineLabel, sortedResourceIssues } from "./resource-view";

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
});
