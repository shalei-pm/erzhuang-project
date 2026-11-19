import { describe, expect, it } from "vitest";
import { buildOperatorNameCandidates, buildOperatorOptions } from "./audit-operators";

describe("buildOperatorOptions", () => {
  it("keeps managed users and adds log-only actors by id", () => {
    const users = [{ id: 1, displayName: "张三", username: "zhangsan", email: "zhang@example.com" }];
    const actors = [
      { userId: 1, actorDisplayName: "张三", userEmail: "zhang@example.com" },
      { userId: 9, actorDisplayName: "四喜", userEmail: "sixi@example.com" },
      { userId: null, actorDisplayName: "匿名", userEmail: "" },
    ];
    const options = buildOperatorOptions(users, actors);
    expect(options.map((option) => option.id)).toEqual([1, 9]);
    expect(options[1]).toMatchObject({ id: 9, displayName: "四喜", email: "sixi@example.com" });
  });

  it("falls back to email when a log actor has no display name", () => {
    const options = buildOperatorOptions([], [{ userId: 5, actorDisplayName: "", userEmail: "only@example.com" }]);
    expect(options[0]).toMatchObject({ id: 5, displayName: "", email: "only@example.com" });
  });
});

describe("buildOperatorNameCandidates", () => {
  const users = [
    { id: 1, displayName: "悟空（金星）", username: "jinxing", email: "jinxing@soyoung.com" },
  ];

  it("adds null-id actors as name candidates and dedupes by email or name", () => {
    const actors = [
      { userId: null, actorDisplayName: "金星", userEmail: "jinxing@soyoung.com" },
      { userId: null, actorDisplayName: "海棠（周伟）", userEmail: "" },
      { userId: null, actorDisplayName: "海棠（周伟）", userEmail: "" },
      { userId: null, actorDisplayName: "莫问（赵文华）", userEmail: "zhaowenhua@soyoung.com" },
    ];
    const candidates = buildOperatorNameCandidates(actors, users);
    expect(candidates.map((candidate) => candidate.key)).toEqual(["name:海棠（周伟）", "email:zhaowenhua@soyoung.com"]);
  });

  it("skips actors already covered by managed users via id or email", () => {
    const actors = [
      { userId: 1, actorDisplayName: "悟空（金星）", userEmail: "jinxing@soyoung.com" },
      { userId: 999, actorDisplayName: "金星", userEmail: "JINXING@soyoung.com" },
      { userId: null, actorDisplayName: "", userEmail: "" },
    ];
    expect(buildOperatorNameCandidates(actors, users)).toEqual([]);
  });
});
