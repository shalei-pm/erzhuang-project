import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { PlaybackControlFailure, serializePlaybackControlDiagnostic } from "./PlaybackControlFailure";
import type { PlaybackControlObservation } from "../vendor/nvr-player/nvr-player.js";

const diagnostic = {
  phase: "failed" as const, requestedRate: 4, verifiedRate: 1,
  protocol: "json" as const, commandSequence: 0, ack: "accepted" as const,
  observedRate: 1.00321, failure: "timeout" as const, timeSource: "container" as const,
};

const observation: PlaybackControlObservation = {
  elapsedMs: 6000.12349, wallMs: 2000.12349, mediaMs: 2360.12349,
  frames: 50, maxGapMs: 80.12349, observedRate: 1.18049,
};

describe("recording speed failure notice", () => {
  it("does not claim an accepted reply when continued observation loses a measured rate without ACK", () => {
    const markup = renderToStaticMarkup(createElement(PlaybackControlFailure, {
      diagnostic: { ...diagnostic, phase: "unconfirmed", failure: null, ack: "none" },
    }));
    expect(markup).not.toContain("指令已接收");
    expect(markup).toContain("4倍速画面速率待确认，可点击倍速恢复正常");
  });
  it("keeps an accepted but unmeasurable command distinct from a playback failure", () => {
    const markup = renderToStaticMarkup(createElement(PlaybackControlFailure, {
      diagnostic: { ...diagnostic, phase: "unconfirmed", failure: null },
    }));
    expect(markup).toContain("4倍速指令已接收，画面速率待确认，可点击倍速恢复正常");
    expect(markup).not.toContain("请继续使用正常速度回放");
    expect(markup).toContain('aria-label="复制倍速诊断信息"');
  });

  it("does not ask for another normal-speed command when restoring normal speed is already accepted", () => {
    const markup = renderToStaticMarkup(createElement(PlaybackControlFailure, {
      diagnostic: { ...diagnostic, phase: "unconfirmed", requestedRate: 1, failure: null },
    }));
    expect(markup).toContain("正常速度指令已接收，画面速率待确认");
    expect(markup).not.toContain("可点击倍速恢复正常");
  });
  it("offers a copyable diagnostic beside a clear failure message", () => {
    const markup = renderToStaticMarkup(createElement(PlaybackControlFailure, { diagnostic }));
    expect(markup).toContain('role="status"');
    expect(markup).toContain("未能确认4倍速生效");
    expect(markup).toContain('aria-label="复制倍速诊断信息"');
    expect(markup).not.toContain("wss:");
  });

  it("copies only allowlisted fields and rounds the measured rate", () => {
    const report = serializePlaybackControlDiagnostic({
      ...diagnostic, token: "private-token", url: "wss://private.test?token=private-token", raw: "private-frame",
    } as typeof diagnostic);
    expect(JSON.parse(report)).toMatchObject({
      schema: 1, requestedRate: 4, verifiedRate: 1, observedRate: 1.003,
      phase: "failed", failure: "timeout", ack: "accepted",
    });
    expect(report).not.toMatch(/private|token|wss:|raw/);
  });

  it("retains an absent observation rather than inventing normal playback", () => {
    expect(JSON.parse(serializePlaybackControlDiagnostic({ ...diagnostic, observedRate: null })).observedRate).toBeNull();
  });

  it("preserves schema 1 and omits optional windows for older SDK diagnostics", () => {
    const report = JSON.parse(serializePlaybackControlDiagnostic(diagnostic));
    expect(report.schema).toBe(1);
    expect(report).not.toHaveProperty("observations");
    expect(JSON.parse(serializePlaybackControlDiagnostic({ ...diagnostic, observations: [] })).observations).toEqual([]);
  });

  it("copies only numeric observation fields, rounded to three decimals, without sensitive extras", () => {
    const injected = {
      ...observation, token: "private-token", url: "wss://private.test", raw: "private-frame",
      date: "2026-11-17", nested: { camera: "private-camera" },
      toJSON: () => ({ token: "private-token" }),
    };
    const report = serializePlaybackControlDiagnostic({ ...diagnostic, observations: [injected] });
    expect(JSON.parse(report)).toMatchObject({ schema: 1, observations: [{
      elapsedMs: 6000.123, wallMs: 2000.123, mediaMs: 2360.123,
      frames: 50, maxGapMs: 80.123, observedRate: 1.18,
    }] });
    expect(Object.keys(JSON.parse(report).observations[0]).sort()).toEqual(Object.keys(observation).sort());
    expect(report).not.toMatch(/private|token|wss:|raw|2026-11-17|nested|camera|toJSON/);
    expect(injected.token).toBe("private-token");
  });

  it("keeps at most the latest eight windows in their original order", () => {
    const observations = Array.from({ length: 12 }, (_, index) => ({ ...observation, elapsedMs: index * 2000 }));
    const report = JSON.parse(serializePlaybackControlDiagnostic({ ...diagnostic, observations }));
    expect(report.observations).toHaveLength(8);
    expect(report.observations.map((window: PlaybackControlObservation) => window.elapsedMs))
      .toEqual([8000, 10000, 12000, 14000, 16000, 18000, 20000, 22000]);
    expect(observations).toHaveLength(12);
  });

  it.each(Object.keys(observation) as Array<keyof PlaybackControlObservation>)(
    "drops a window when %s is not finite while retaining valid neighboring windows", (field) => {
      const observations = [
        { ...observation, elapsedMs: 2000 },
        { ...observation, [field]: NaN },
        { ...observation, [field]: Infinity },
        { ...observation, [field]: -Infinity },
        { ...observation, elapsedMs: 8000 },
      ];
      const report = JSON.parse(serializePlaybackControlDiagnostic({ ...diagnostic, observations }));
      expect(report.observations.map((window: PlaybackControlObservation) => window.elapsedMs)).toEqual([2000, 8000]);
    },
  );

  it.each([null, undefined, "private-value", {}, [], { ...observation, wallMs: "private-value" }])(
    "drops malformed observation entries without coercing values: %j", (invalid) => {
      const observations = [invalid, observation] as PlaybackControlObservation[];
      const report = serializePlaybackControlDiagnostic({ ...diagnostic, observations });
      expect(JSON.parse(report).observations).toHaveLength(1);
      expect(report).not.toContain("private");
    },
  );

  it.each([null, "private-value", { token: "private-token" }])("omits a malformed observations list: %j", (invalid) => {
    const report = serializePlaybackControlDiagnostic({
      ...diagnostic, observations: invalid as unknown as PlaybackControlObservation[],
    });
    expect(JSON.parse(report)).not.toHaveProperty("observations");
    expect(report).not.toContain("private");
  });

  it("keeps large finite numbers finite when rounding and retains valid zero values", () => {
    const report = JSON.parse(serializePlaybackControlDiagnostic({
      ...diagnostic, observedRate: Number.MAX_VALUE,
      observations: [{ elapsedMs: 0, wallMs: 0, mediaMs: 0, frames: 0, maxGapMs: 0, observedRate: Number.MAX_VALUE }],
    }));
    expect(report.observedRate).toBe(Number.MAX_VALUE);
    expect(report.observations).toEqual([
      { elapsedMs: 0, wallMs: 0, mediaMs: 0, frames: 0, maxGapMs: 0, observedRate: Number.MAX_VALUE },
    ]);
  });
});
