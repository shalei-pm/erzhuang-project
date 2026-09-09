import { describe, expect, it } from "vitest";
import { cameraRegion, chooseTwinStore, parseInstitutionID, regionCameras } from "./digital-twin";

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
});
