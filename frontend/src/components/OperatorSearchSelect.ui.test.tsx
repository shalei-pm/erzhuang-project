// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import { OperatorSearchSelect } from "./OperatorSearchSelect";

const users = [
  { id: 1, displayName: "张三", username: "zhangsan", email: "zhang@example.com" },
  { id: 2, displayName: "李四", username: "lisi", email: "li@example.com" },
  { id: 3, displayName: "阿芳", username: "afang", email: "a@example.com" },
];

describe("OperatorSearchSelect", () => {
  let root: Root;
  let container: HTMLDivElement;
  let onChange: Mock<(value: string) => void>;
  let value: string;

  beforeEach(() => {
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    onChange = vi.fn((next: string) => {
      value = next;
    });
    value = "";
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
  });

  async function render() {
    await act(async () => root.render(createElement(OperatorSearchSelect, { users, value, onChange })));
  }

  function trigger() {
    return container.querySelector<HTMLButtonElement>(".operator-select-trigger")!;
  }

  function searchInput() {
    return container.querySelector<HTMLInputElement>(".operator-select-search")!;
  }

  function optionLabels() {
    return Array.from(container.querySelectorAll(".operator-select-option")).map((node) => node.textContent);
  }

  async function open() {
    await render();
    await act(async () => trigger().dispatchEvent(new MouseEvent("click", { bubbles: true })));
  }

  async function type(queryText: string) {
    const input = searchInput();
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")!.set!;
    await act(async () => {
      setter.call(input, queryText);
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
  }

  async function press(key: string) {
    await act(async () => searchInput().dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true })));
  }

  it("lists all operators sorted by pinyin initial", async () => {
    await open();
    expect(optionLabels()).toEqual(["全部操作人", "阿芳", "李四", "张三"]);
    expect(searchInput()).toBeTruthy();
  });

  it("filters by name, account and email", async () => {
    await open();
    await type("张");
    expect(optionLabels()).toEqual(["全部操作人", "张三"]);
    await type("lisi");
    expect(optionLabels()).toEqual(["全部操作人", "李四"]);
    await type("A@EXAMPLE");
    expect(optionLabels()).toEqual(["全部操作人", "阿芳"]);
  });

  it("shows an empty hint when nothing matches", async () => {
    await open();
    await type("不存在");
    expect(container.querySelector(".operator-select-empty")?.textContent).toBe("无匹配操作人");
  });

  it("selects with click and keyboard, then closes", async () => {
    await open();
    await act(async () => container.querySelectorAll(".operator-select-option")[3].dispatchEvent(new MouseEvent("click", { bubbles: true })));
    expect(onChange).toHaveBeenCalledWith("1");
    expect(container.querySelector(".operator-select-panel")).toBeNull();
    await render();
    expect(trigger().textContent).toContain("张三");

    await act(async () => trigger().dispatchEvent(new MouseEvent("click", { bubbles: true })));
    await press("ArrowUp");
    await press("ArrowUp");
    await press("Enter");
    expect(onChange).toHaveBeenCalledWith("3");
  });

  it("closes on escape and outside click", async () => {
    await open();
    await press("Escape");
    expect(container.querySelector(".operator-select-panel")).toBeNull();
    await act(async () => trigger().dispatchEvent(new MouseEvent("click", { bubbles: true })));
    await act(async () => document.dispatchEvent(new MouseEvent("mousedown", { bubbles: true })));
    expect(container.querySelector(".operator-select-panel")).toBeNull();
  });
});
