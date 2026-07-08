import { describe, expect, it } from "vitest";
import {
  h5MonitorActiveTabStorageKey,
  h5MonitorTabSearch,
  readH5MonitorActiveTab,
  readH5MonitorActiveTabFromSearch,
  storeH5MonitorActiveTab,
} from "./h5-monitor-active-tab";

function memoryStorage(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial));
  return {
    getItem(key: string) {
      return values.get(key) ?? null;
    },
    setItem(key: string, value: string) {
      values.set(key, value);
    },
  };
}

describe("H5 monitor active tab persistence", () => {
  it("stores and restores the active tab per external org", () => {
    const storage = memoryStorage();

    storeH5MonitorActiveTab("10030", "treatment", storage);
    storeH5MonitorActiveTab("10081", "beauty", storage);

    expect(readH5MonitorActiveTab("10030", storage)).toBe("treatment");
    expect(readH5MonitorActiveTab("10081", storage)).toBe("beauty");
  });

  it("falls back to all for missing or invalid stored values", () => {
    const storage = memoryStorage({
      [h5MonitorActiveTabStorageKey("10030")]: "unknown",
    });

    expect(readH5MonitorActiveTab("10019", storage)).toBe("all");
    expect(readH5MonitorActiveTab("10030", storage)).toBe("all");
  });

  it("does not break when storage is unavailable", () => {
    const failingStorage = {
      getItem() {
        throw new Error("blocked");
      },
      setItem() {
        throw new Error("blocked");
      },
    };

    expect(readH5MonitorActiveTab("10030", failingStorage)).toBe("all");
    expect(() => storeH5MonitorActiveTab("10030", "consultation", failingStorage)).not.toThrow();
  });

  it("reads valid active tabs from URL search params", () => {
    expect(readH5MonitorActiveTabFromSearch("?tab=treatment")).toBe("treatment");
    expect(readH5MonitorActiveTabFromSearch(new URLSearchParams("tab=beauty"))).toBe("beauty");
    expect(readH5MonitorActiveTabFromSearch("?tab=unknown")).toBeNull();
    expect(readH5MonitorActiveTabFromSearch("")).toBeNull();
  });

  it("serializes only non-default tabs into URL search", () => {
    expect(h5MonitorTabSearch("treatment")).toBe("?tab=treatment");
    expect(h5MonitorTabSearch("all")).toBe("");
    expect(h5MonitorTabSearch(null)).toBe("");
  });
});
