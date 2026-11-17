import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { PlaybackControlFailure, serializePlaybackControlDiagnostic } from "./PlaybackControlFailure";

const diagnostic = {
  phase: "failed" as const, requestedRate: 4, verifiedRate: 1,
  protocol: "json" as const, commandSequence: 0, ack: "accepted" as const,
  observedRate: 1.00321, failure: "timeout" as const, timeSource: "container" as const,
};

describe("recording speed failure notice", () => {
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
});
