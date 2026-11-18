import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { AuditLogManagement } from "./AuditLogManagement";

describe("audit session actions", () => {
  it("includes the absolute session timeout in action filters", () => {
    const markup = renderToStaticMarkup(createElement(AuditLogManagement, { onToast: () => undefined }));
    expect(markup).toContain('<option value="auth.absolute_timeout">登录满8小时失效</option>');
  });

  it("renders the operator filter as a searchable combobox", () => {
    const markup = renderToStaticMarkup(createElement(AuditLogManagement, { onToast: () => undefined }));
    expect(markup).toContain('class="operator-select-trigger"');
    expect(markup).toContain('aria-haspopup="listbox"');
    expect(markup).toContain("全部操作人");
  });
});
