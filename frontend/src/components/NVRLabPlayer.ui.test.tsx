// @vitest-environment jsdom
import { act, createElement, type ComponentProps } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { NVRPlayerOptions, PlaybackControlDiagnostics } from "../vendor/nvr-player/nvr-player.js";
import { NVRLabPlayer } from "./NVRLabPlayer";

const { players, MockPlayer } = vi.hoisted(() => {
  const players: FakePlayer[] = [];
  class FakePlayer {
    diagnostic: PlaybackControlDiagnostics = { phase: "idle", requestedRate: 1, verifiedRate: 1, protocol: "json",
      commandSequence: null, ack: "none", observedRate: null, failure: null, timeSource: "container" };
    attempts: { rate: number; resolve: () => void; reject: (error: Error) => void }[] = [];
    pending: FakePlayer["attempts"][number] | null = null;
    constructor(_canvas: unknown, readonly options: NVRPlayerOptions) { players.push(this); }
    play = vi.fn(async () => undefined);
    stop = vi.fn(() => { this.pending?.reject(new Error("stopped")); this.pending = null; });
    setVolume = vi.fn();
    enableAudio = vi.fn(async () => undefined);
    getPlaybackControlDiagnostics = vi.fn(() => ({ ...this.diagnostic }));
    setSpeed = vi.fn((rate: number) => {
      if (this.pending && rate !== 1) return Promise.reject(new Error("pending"));
      this.pending?.reject(new Error("superseded"));
      this.diagnostic = { ...this.diagnostic, phase: "verifying", requestedRate: rate,
        commandSequence: this.attempts.length, ack: "none", failure: null };
      return new Promise<void>((resolve, reject) => {
        this.pending = { rate, resolve, reject };
        this.attempts.push(this.pending);
      });
    });
    settle(phase: "verified" | "unconfirmed", ack: PlaybackControlDiagnostics["ack"] = "accepted") {
      this.diagnostic = { ...this.diagnostic, phase, ack,
        verifiedRate: phase === "verified" ? this.diagnostic.requestedRate : this.diagnostic.verifiedRate };
      this.pending?.resolve();
      this.pending = null;
    }
    fail(failure: "rejected" | "connection" | "send_failed" | "uncorrelated") {
      this.diagnostic = { ...this.diagnostic, phase: "failed", failure };
      this.pending?.reject(new Error(failure));
      this.pending = null;
    }
  }
  return { players, MockPlayer: FakePlayer };
});

vi.mock("../vendor/nvr-player/nvr-player.js", () => ({ default: MockPlayer }));

type Props = ComponentProps<typeof NVRLabPlayer>;
describe("NVRLabPlayer speed lifecycle", () => {
  let root: Root;
  let container: HTMLDivElement;
  let props: Props;
  const fallback = vi.fn();
  const changeRate = vi.fn((rate: number) => {
    props = { ...props, playbackRate: rate };
    root.render(createElement(NVRLabPlayer, props));
  });
  async function render(next: Partial<Props> = {}) {
    props = { ...props, ...next };
    await act(async () => root.render(createElement(NVRLabPlayer, props)));
  }
  function button(label: string) {
    const element = container.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`);
    expect(element, label).not.toBeNull();
    return element!;
  }
  async function click(label: string) { await act(async () => button(label).click()); }
  async function startSpeed(rate = 8) { await render({ playbackRate: rate }); }
  function speedOptions() {
    const dialog = container.querySelector('[role="dialog"][aria-label="选择播放倍速"]');
    expect(dialog).not.toBeNull();
    const options = Array.from(dialog!.querySelectorAll<HTMLButtonElement>("button"));
    expect(options.map(option => option.textContent)).toEqual(["8倍速", "取消"]);
    return options;
  }
  async function selectSpeedOption(label: string) {
    const option = speedOptions().find(option => option.textContent === label);
    expect(option).toBeDefined();
    await act(async () => option!.click());
    expect(container.querySelector('[role="dialog"]')).toBeNull();
  }

  beforeEach(async () => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    players.length = 0;
    fallback.mockClear();
    changeRate.mockClear();
    container = document.createElement("div");
    document.body.append(container);
    root = createRoot(container);
    props = {
      session: { mode: "playback", url: "wss://example.test/replay" },
      playbackSegment: { start_time: 1000, end_time: 4600 },
      playbackMode: true, playbackRate: 1,
      onPlaybackRateChange: changeRate, onPlaybackRateError: fallback,
      onSeekPlayback: () => undefined, onRetry: () => undefined,
    };
    await render();
    await act(async () => players[0].options.onFirstFrame?.());
  });
  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
  });

  it("offers only 8x and cancel and reopens without changing playback after cancel", async () => {
    await click("选择倍速");
    await selectSpeedOption("取消");
    expect(changeRate).not.toHaveBeenCalled();
    expect(players[0].setSpeed).not.toHaveBeenCalled();
    expect(players[0].stop).not.toHaveBeenCalled();
    await click("选择倍速");
    speedOptions();
    expect(changeRate).not.toHaveBeenCalled();
    expect(players[0].setSpeed).not.toHaveBeenCalled();
  });

  it.each(["pending", "verified"])("selects 8x, restores normal speed from %s and reopens to select 8x again", async (phase) => {
    const rate = 8;
    const player = players[0];
    await click("选择倍速");
    await selectSpeedOption(`${rate}倍速`);
    expect(changeRate).toHaveBeenLastCalledWith(rate);
    expect(player.setSpeed).toHaveBeenCalledExactlyOnceWith(rate);
    expect(button("恢复正常速度").textContent).toContain(`${rate}×`);
    if (phase === "verified") await act(async () => player.settle("verified"));
    await click("恢复正常速度");
    expect(container.querySelector('[role="dialog"]')).toBeNull();
    expect(changeRate).toHaveBeenLastCalledWith(1);
    expect(player.setSpeed.mock.calls.map(([value]) => value)).toEqual([rate, 1]);
    await act(async () => player.settle("verified"));
    expect(button("选择倍速").disabled).toBe(false);
    await click("选择倍速");
    await selectSpeedOption(`${rate}倍速`);
    expect(changeRate).toHaveBeenLastCalledWith(rate);
    expect(player.setSpeed.mock.calls.map(([value]) => value)).toEqual([rate, 1, rate]);
    expect(button("恢复正常速度").textContent).toContain(`${rate}×`);
    expect(player.stop).not.toHaveBeenCalled();
    expect(fallback).not.toHaveBeenCalled();
  });

  it("forwards actual Beijing recording time with session identity and ignores stale callbacks", async () => {
    const onPlaybackTime = vi.fn();
    await render({ onPlaybackTime });
    const old = players[0];
    const session = props.session;
    expect(old.options.recordingUTCOffsetMinutes).toBe(480);
    old.options.onPlaybackTime?.(0, 1794794406);
    expect(onPlaybackTime).toHaveBeenLastCalledWith(0, session, 1794794406);
    await render({ session: { mode: "playback", url: "wss://example.test/replay-new" } });
    old.options.onPlaybackTime?.(10, 1794794416);
    expect(onPlaybackTime).toHaveBeenCalledOnce();
    players.at(-1)!.options.onPlaybackTime?.(2, null);
    expect(onPlaybackTime).toHaveBeenLastCalledWith(2, props.session, null);
  });

  it("shows the requested rate and permits restore, pause and screenshots during verification", async () => {
    await startSpeed();
    const restore = button("恢复正常速度");
    expect(restore.disabled).toBe(false);
    expect(restore.textContent).toContain("8×");
    expect(restore.getAttribute("aria-busy")).toBe("true");
    expect(button("暂停").disabled).toBe(false);
    expect(button("截图").disabled).toBe(false);
    expect(button("快进10秒").disabled).toBe(true);
    expect(button("倍速播放静音").disabled).toBe(true);
    expect(container.querySelector<HTMLInputElement>('input[type="range"]')?.disabled).toBe(true);
  });

  it("sends 1x while the last verified rate is still 1 and ignores the superseded rejection", async () => {
    await startSpeed();
    await click("恢复正常速度");
    const player = players[0];
    expect(changeRate).toHaveBeenCalledWith(1);
    expect(player.setSpeed.mock.calls.map(([rate]) => rate)).toEqual([8, 1]);
    expect(fallback).not.toHaveBeenCalled();
    expect(button("选择倍速").disabled).toBe(true);
    expect(button("选择倍速").getAttribute("aria-busy")).toBe("true");
    await act(async () => player.settle("verified"));
    expect(button("选择倍速").disabled).toBe(false);
    expect(button("选择倍速").getAttribute("aria-busy")).toBe("false");
    expect(fallback).not.toHaveBeenCalled();
  });

  it("restores through a fresh host session without sending 1x into the buffered stream", async () => {
    const restore = vi.fn();
    await render({ onRestoreNormalPlayback: restore });
    await startSpeed();
    await click("恢复正常速度");
    expect(restore).toHaveBeenCalledOnce();
    expect(players[0].stop).toHaveBeenCalled();
    expect(players[0].setSpeed.mock.calls.map(([rate]) => rate)).toEqual([8]);
    expect(changeRate).not.toHaveBeenCalled();
    expect(fallback).not.toHaveBeenCalled();
  });

  it("does not prepare a new audio player when restoring the saved paused rate", async () => {
    const restore = vi.fn();
    await render({ playbackRate: 8, playbackPaused: true, onRestoreNormalPlayback: restore });
    const count = players.length;
    await click("恢复正常速度");
    expect(restore).toHaveBeenCalledOnce();
    expect(players).toHaveLength(count);
    expect(changeRate).not.toHaveBeenCalled();
    expect(fallback).not.toHaveBeenCalled();
  });

  it("keeps an unconfirmed requested rate playable with copyable diagnostics and no retry loop", async () => {
    await startSpeed();
    const player = players[0];
    await act(async () => player.settle("unconfirmed"));
    expect(button("恢复正常速度").textContent).toContain("8×");
    expect(button("恢复正常速度").getAttribute("aria-busy")).toBe("false");
    expect(button("快进10秒").disabled).toBe(false);
    expect(button("暂停").disabled).toBe(false);
    expect(container.querySelector<HTMLInputElement>('input[type="range"]')?.disabled).toBe(false);
    const writeText = vi.fn(async (_report: string) => undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    await click("复制倍速诊断信息");
    expect(JSON.parse(writeText.mock.calls[0][0])).toMatchObject({ phase: "unconfirmed", requestedRate: 8, verifiedRate: 1,
      timeSource: "container" });
    await render({ onStatus: () => undefined });
    expect(player.setSpeed).toHaveBeenCalledTimes(1);
    expect(player.stop).not.toHaveBeenCalled();
    expect(fallback).not.toHaveBeenCalled();
    await click("恢复正常速度");
    expect(player.setSpeed.mock.calls.map(([rate]) => rate)).toEqual([8, 1]);
    expect(container.querySelector('[aria-label="复制倍速诊断信息"]')).toBeNull();
  });

  it("copies fresh continued observations without retrying or closing the active session", async () => {
    await startSpeed();
    const player = players[0];
    await act(async () => player.settle("unconfirmed"));
    const observations = [{ elapsedMs: 8500, wallMs: 2000, mediaMs: 2360, frames: 50,
      maxGapMs: 40, observedRate: 1.18 }];
    await act(async () => {
      player.diagnostic = { ...player.diagnostic, observations, observedRate: 1.18 };
      player.options.onPlaybackControlDiagnostics?.(player.getPlaybackControlDiagnostics());
    });
    const writeText = vi.fn(async (_report: string) => undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    await click("复制倍速诊断信息");
    expect(JSON.parse(writeText.mock.calls[0][0])).toMatchObject({ observedRate: 1.18, observations });
    expect(player.setSpeed).toHaveBeenCalledTimes(1);
    expect(player.stop).not.toHaveBeenCalled();
    expect(fallback).not.toHaveBeenCalled();
    await act(async () => {
      player.diagnostic = { ...player.diagnostic, phase: "verified", observedRate: 8, verifiedRate: 8 };
      player.options.onPlaybackControlDiagnostics?.(player.getPlaybackControlDiagnostics());
    });
    expect(container.querySelector('[aria-label="复制倍速诊断信息"]')).toBeNull();
    expect(button("恢复正常速度").disabled).toBe(false);
  });

  it.each(["verified", "pending"])("restores sound after accepted 1x without claiming measured 1x from %s 8x", async (phase) => {
    await click("开启声音");
    const player = players[0];
    expect(player.setVolume).toHaveBeenLastCalledWith(80);
    await startSpeed();
    if (phase === "verified") await act(async () => player.settle("verified"));
    await click("恢复正常速度");
    expect(player.setVolume).toHaveBeenLastCalledWith(0);
    await act(async () => player.settle("unconfirmed"));
    expect(player.setVolume).toHaveBeenLastCalledWith(80);
    expect(button("关闭声音").disabled).toBe(false);
    expect(button("关闭声音").getAttribute("aria-pressed")).toBe("true");
    expect(button("选择倍速").getAttribute("aria-busy")).toBe("false");
    expect(player.getPlaybackControlDiagnostics()).toMatchObject({ phase: "unconfirmed", requestedRate: 1,
      verifiedRate: phase === "verified" ? 8 : 1, ack: "accepted" });
    expect(button("复制倍速诊断信息")).toBeDefined();
    expect(fallback).not.toHaveBeenCalled();
  });

  it("preserves mute after accepted unconfirmed 1x and lets the user enable sound", async () => {
    await startSpeed();
    await click("恢复正常速度");
    const player = players[0];
    await act(async () => player.settle("unconfirmed"));
    expect(player.setVolume).toHaveBeenLastCalledWith(0);
    expect(button("开启声音").disabled).toBe(false);
    expect(button("开启声音").getAttribute("aria-pressed")).toBe("false");
    await click("开启声音");
    expect(player.setVolume).toHaveBeenLastCalledWith(80);
    expect(button("关闭声音").getAttribute("aria-pressed")).toBe("true");
    await click("关闭声音");
    expect(player.setVolume).toHaveBeenLastCalledWith(0);
    expect(button("开启声音").disabled).toBe(false);
    expect(player.getPlaybackControlDiagnostics()).toMatchObject({ phase: "unconfirmed", requestedRate: 1,
      verifiedRate: 1, ack: "accepted" });
    expect(fallback).not.toHaveBeenCalled();
  });

  it("restores sound for measured 1x without an ACK", async () => {
    await click("开启声音");
    await startSpeed();
    const player = players[0];
    await act(async () => player.settle("verified"));
    await click("恢复正常速度");
    await act(async () => player.settle("verified", "none"));
    expect(player.setVolume).toHaveBeenLastCalledWith(80);
    expect(button("关闭声音").disabled).toBe(false);
    expect(button("关闭声音").getAttribute("aria-pressed")).toBe("true");
    expect(player.getPlaybackControlDiagnostics()).toMatchObject({ phase: "verified", verifiedRate: 1, ack: "none" });
    expect(fallback).not.toHaveBeenCalled();
  });

  it("does not restore sound for unconfirmed 1x without an ACK", async () => {
    await click("开启声音");
    await startSpeed();
    const player = players[0];
    await act(async () => player.settle("verified"));
    await click("恢复正常速度");
    await act(async () => player.settle("unconfirmed", "none"));
    expect(player.setVolume).toHaveBeenLastCalledWith(0);
    expect(button("倍速播放静音").disabled).toBe(true);
    expect(button("倍速播放静音").getAttribute("aria-pressed")).toBe("false");
    expect(button("选择倍速").getAttribute("aria-busy")).toBe("false");
    expect(fallback).not.toHaveBeenCalled();
  });

  it.each(["rejected", "connection", "send_failed"] as const)("keeps sound disabled when restoring 1x fails with %s", async (failure) => {
    await click("开启声音");
    await startSpeed();
    const player = players[0];
    await act(async () => player.settle("verified"));
    await click("恢复正常速度");
    await act(async () => player.fail(failure));
    expect(player.setVolume).toHaveBeenLastCalledWith(0);
    expect(button("倍速播放静音").disabled).toBe(true);
    expect(button("倍速播放静音").getAttribute("aria-pressed")).toBe("false");
    expect(fallback).toHaveBeenCalledTimes(1);
  });

  it("restores the user's sound preference once the 1x request verifies", async () => {
    await click("开启声音");
    await startSpeed();
    await click("恢复正常速度");
    const player = players[0];
    await act(async () => player.settle("verified"));
    expect(player.setVolume).toHaveBeenLastCalledWith(80);
    expect(button("关闭声音").disabled).toBe(false);
  });

  it.each(["rejected", "connection", "send_failed"] as const)("still requests recovery for an explicit %s failure", async (failure) => {
    await startSpeed();
    await act(async () => players[0].fail(failure));
    expect(fallback).toHaveBeenCalledTimes(1);
    expect(fallback).toHaveBeenCalledWith(expect.any(String), props.session);
    expect(button("复制倍速诊断信息")).toBeDefined();
  });

  it("requests a fresh normal session without a failure notice for unsequenced restore replies", async () => {
    await startSpeed();
    await click("恢复正常速度");
    await act(async () => players[0].fail("uncorrelated"));
    expect(fallback).toHaveBeenCalledExactlyOnceWith("正在重新连接正常速度回放", props.session);
    expect(container.querySelector('[aria-label="复制倍速诊断信息"]')).toBeNull();
  });

  it.each(["resolve", "reject"] as const)("ignores late %s from an old session and sends speed to the new player", async (settle) => {
    await startSpeed();
    const old = players[0];
    old.stop.mockImplementationOnce(() => { old.pending = null; });
    await render({ session: { mode: "playback", url: "wss://example.test/next" } });
    await act(async () => players[1].options.onFirstFrame?.());
    expect(players[1].setSpeed).toHaveBeenCalledWith(8);
    await act(async () => {
      if (settle === "resolve") old.attempts[0].resolve();
      else old.attempts[0].reject(new Error("stopped"));
    });
    expect(button("恢复正常速度").getAttribute("aria-busy")).toBe("true");
    expect(fallback).not.toHaveBeenCalled();
  });

  it("ignores the stopped request after unmount", async () => {
    await startSpeed();
    await act(async () => root.unmount());
    expect(players[0].stop).toHaveBeenCalled();
    expect(fallback).not.toHaveBeenCalled();
  });

  it("stops the network on pause during verification and reapplies speed on resume", async () => {
    await render({
      onPausePlayback: () => {
        props = { ...props, session: null, playbackPaused: true };
        root.render(createElement(NVRLabPlayer, props));
      },
      onResumePlayback: () => {
        props = { ...props, session: { mode: "playback", url: "wss://example.test/resume" }, playbackPaused: false };
        root.render(createElement(NVRLabPlayer, props));
      },
    });
    await startSpeed();
    await click("暂停");
    expect(players[0].stop).toHaveBeenCalled();
    expect(fallback).not.toHaveBeenCalled();
    expect(button("播放").disabled).toBe(false);
    await click("播放");
    await act(async () => players[1].options.onFirstFrame?.());
    expect(players[1].setSpeed).toHaveBeenCalledWith(8);
    expect(fallback).not.toHaveBeenCalled();
  });
});
