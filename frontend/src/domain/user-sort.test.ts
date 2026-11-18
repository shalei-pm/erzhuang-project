import { describe, expect, it } from "vitest";
import { sortUsersByPinyin, userDisplayLabel, userMatchesQuery } from "./user-sort";

const users = [
  { id: 1, displayName: "张三", username: "zhangsan", email: "zhang@example.com" },
  { id: 2, displayName: "李四", username: "lisi", email: "li@example.com" },
  { id: 3, displayName: "阿芳", username: "afang", email: "a@example.com" },
];

describe("user sorting and search", () => {
  it("sorts Chinese display names by pinyin initial", () => {
    expect(sortUsersByPinyin(users).map((user) => user.displayName)).toEqual(["阿芳", "李四", "张三"]);
  });

  it("does not mutate the input list", () => {
    const input = [...users];
    sortUsersByPinyin(input);
    expect(input.map((user) => user.id)).toEqual([1, 2, 3]);
  });

  it("falls back to username then email for the label", () => {
    expect(userDisplayLabel({ displayName: "", username: "bob", email: "bob@example.com" })).toBe("bob");
    expect(userDisplayLabel({ email: "only@example.com" })).toBe("only@example.com");
  });

  it("matches name, account and email case-insensitively", () => {
    expect(userMatchesQuery(users[0], "张")).toBe(true);
    expect(userMatchesQuery(users[1], "LIS")).toBe(true);
    expect(userMatchesQuery(users[2], "A@EXAMPLE")).toBe(true);
    expect(userMatchesQuery(users[0], "  ")).toBe(true);
    expect(userMatchesQuery(users[0], "不存在")).toBe(false);
  });
});
