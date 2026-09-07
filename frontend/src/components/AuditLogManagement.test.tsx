import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { AuditLogManagement } from "./AuditLogManagement";

describe("audit session actions", () => {
  it("includes the absolute session timeout in action filters", () => {
    const markup = renderToStaticMarkup(createElement(AuditLogManagement, { onToast: () => undefined }));
    expect(markup).toContain('<option value="auth.absolute_timeout">登录满8小时失效</option>');
  });
});
