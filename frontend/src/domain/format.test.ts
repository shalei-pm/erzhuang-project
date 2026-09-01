import { describe, expect, it } from "vitest";

import { ApiError } from "../api";
import { errorMessage } from "./format";

describe("errorMessage", () => {
  it("maps known backend audit errors to user-facing Chinese", () => {
    expect(errorMessage(new ApiError(500, "list audit logs failed"), "fallback")).toBe("操作日志加载失败，请检查审计日志表配置后重试。");
  });
});
