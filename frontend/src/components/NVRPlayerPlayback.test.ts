import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import NVRPlayer, { type PlaybackControlDiagnostics } from "../vendor/nvr-player/nvr-player.js";

class TestFrame {
  displayWidth = 640;
  displayHeight = 360;
  close = vi.fn();
  constructor(public timestamp: number) {}
}

class TestSocket {
  static OPEN = 1;
  readyState = 1;
  binaryType = "";
  onopen: (() => void) | null = null;
  onclose: ((event: { code: number; wasClean: boolean }) => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((event: { data: unknown }) => void) | null = null;
  send = vi.fn();
  close = vi.fn(() => { this.readyState = 3; });
  message(value: unknown) { this.onmessage?.({ data: JSON.stringify(value) }); }
}

class TestDecoder {
  state = "unconfigured";
  decodeQueueSize = 0;
  constructor(public callbacks: { output: (frame: TestFrame) => void }) {}
  configure() { this.state = "configured"; }
  decode(chunk: { timestamp: number }) { this.callbacks.output(new TestFrame(chunk.timestamp)); }
  close() { this.state = "closed"; }
}

class TestCanvas {
  width = 640;
  height = 360;
  ctx = { drawImage: vi.fn(), clearRect: vi.fn() };
  getContext() { return this.ctx; }
}

function audioContext(state = "running") {
  const sources: Array<{ stop: ReturnType<typeof vi.fn>; disconnect: ReturnType<typeof vi.fn>; onended: (() => void) | null }> = [];
  const gain = { gain: { value: 0, setTargetAtTime: vi.fn((value: number) => { gain.gain.value = value; }) }, connect: vi.fn() };
  const context = {
    state, sampleRate: 48000, currentTime: 2, destination: {},
    createGain: () => gain,
    createBiquadFilter: () => ({ type: "", frequency: { value: 0 }, Q: { value: 0 }, connect: vi.fn() }),
    createBuffer: (_channels: number, frames: number, rate: number) => ({ duration: frames / rate, getChannelData: () => new Float32Array(frames) }),
    createBufferSource: () => {
      const source = { buffer: null, connect: vi.fn(), disconnect: vi.fn(), start: vi.fn(), stop: vi.fn(), onended: null };
      sources.push(source);
      return source;
    },
    close: vi.fn(async () => { context.state = "closed"; }),
    resume: vi.fn(async () => { context.state = "running"; }),
  };
  return { context, sources, gain };
}

type Player = NVRPlayer & {
  audioContext: ReturnType<typeof audioContext>["context"] | null;
  ws: TestSocket;
  decoder: TestDecoder;
  useWasm: boolean;
  h264DecoderConfigured: boolean;
  currentVolume: number;
  workletNode: { port: { postMessage: ReturnType<typeof vi.fn> }; disconnect: ReturnType<typeof vi.fn> } | null;
  _renderFrame(frame: TestFrame): void;
  _handleH264InitInternal(nals: Uint8Array[]): void;
  _handleH265Init(nals: Uint8Array[]): void;
  _handleH264Frame(buffer: ArrayBuffer, info: { nFrameType: number; nTimeStamp: number }): void;
  _playAudioPCM(samples: Int16Array): void;
  _resetAudioStream(): void;
  _createTransformWorker(): Promise<{ postMessage: ReturnType<typeof vi.fn>; terminate: ReturnType<typeof vi.fn> }>;
};

const players: Player[] = [];

async function makePlayer(playback = true) {
  const canvas = new TestCanvas();
  const onPlaybackTime = vi.fn();
  const onPlaybackControlDiagnostics = vi.fn<(snapshot: PlaybackControlDiagnostics) => void>();
  const player = new NVRPlayer(canvas as unknown as HTMLCanvasElement, {
    forceWasm: playback, onPlaybackTime, onPlaybackControlDiagnostics,
  }) as Player;
  players.push(player);
  await player.play("ws://test.invalid/signed-session");
  return { player, canvas, onPlaybackTime, onPlaybackControlDiagnostics, socket: player.ws };
}

function present(player: Player, timestamp: number) {
  const frame = new TestFrame(timestamp);
  player._renderFrame(frame);
  return frame;
}

function feed(player: Player, rate: number, duration = 3500, startPTS = 1_000_000) {
  for (let elapsed = 100; elapsed <= duration; elapsed += 100) {
    vi.advanceTimersByTime(100);
    present(player, startPTS + elapsed * 1000 * rate);
  }
}

function outcome(promise: Promise<void>) {
  return promise.then(() => "verified", (error: unknown) => error);
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "setInterval", "clearInterval", "performance", "Date"] });
  vi.stubGlobal("HTMLCanvasElement", TestCanvas);
  vi.stubGlobal("WebSocket", TestSocket);
  vi.stubGlobal("VideoDecoder", TestDecoder);
  vi.stubGlobal("EncodedVideoChunk", class { constructor(init: object) { Object.assign(this, init); } });
  vi.stubGlobal("Worker", class { postMessage = vi.fn(); terminate = vi.fn(); });
  vi.stubGlobal("window", { VideoDecoder: TestDecoder });
  vi.stubGlobal("document", { addEventListener: vi.fn(), removeEventListener: vi.fn(), visibilityState: "visible" });
  vi.stubGlobal("navigator", {});
});

afterEach(() => {
  for (const player of players.splice(0)) player.stop();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe("NVR playback presented media time", () => {
  it("reports elapsed seconds only after successful canvas presentation", async () => {
    const { player, canvas, onPlaybackTime } = await makePlayer();
    expect(player.getPlaybackTime()).toBeNull();
    present(player, 50_000_000);
    present(player, 52_500_000);
    expect(onPlaybackTime.mock.calls).toEqual([[0], [2.5]]);
    expect(player.getPlaybackTime()).toBe(2.5);
    expect(canvas.ctx.drawImage).toHaveBeenCalledTimes(2);
    canvas.ctx.drawImage.mockImplementationOnce(() => { throw new Error("draw failed"); });
    const failed = new TestFrame(54_000_000);
    expect(() => player._renderFrame(failed)).toThrow("draw failed");
    expect(failed.close).toHaveBeenCalledOnce();
    expect(player.getPlaybackTime()).toBe(2.5);
    vi.advanceTimersByTime(30_000);
    expect(player.getPlaybackTime()).toBe(2.5);
  });

  it("ignores invalid, repeated, backward and paused PTS without moving the anchor", async () => {
    const { player, onPlaybackTime } = await makePlayer();
    for (const pts of [NaN, Infinity, -1, Number.MAX_SAFE_INTEGER + 1]) present(player, pts);
    expect(player.getPlaybackTime()).toBeNull();
    present(player, 0);
    present(player, 2_000_000);
    for (const pts of [0, 1_000_000, 2_000_000]) present(player, pts);
    player.pause();
    expect(present(player, 8_000_000).close).toHaveBeenCalledOnce();
    expect(player.getPlaybackTime()).toBe(2);
    expect(onPlaybackTime.mock.calls).toEqual([[0], [2]]);
    player.resume();
    present(player, 3_000_000);
    expect(player.getPlaybackTime()).toBe(3);
  });

  it.each(["h264", "h265"])("excludes %s initialization IDR and normalizes WASM media milliseconds", async (codec) => {
    const { player, onPlaybackTime } = await makePlayer();
    if (codec === "h264") {
      player._handleH264InitInternal([new Uint8Array([7, 0x64, 0, 0x29]), new Uint8Array([8, 1]), new Uint8Array([5, 1])]);
    } else {
      player._handleH265Init([32, 33, 34, 19].map((type) => new Uint8Array([type << 1, 1])));
    }
    expect(onPlaybackTime).not.toHaveBeenCalled();
    const buffer = new Uint8Array([0, 0, 0, 2, 2, 1]).buffer;
    player._handleH264Frame(buffer, { nFrameType: 2, nTimeStamp: 5000 });
    player._handleH264Frame(buffer, { nFrameType: 2, nTimeStamp: 6250 });
    expect(onPlaybackTime.mock.calls).toEqual([[0], [1.25]]);
  });

  it("resets replay time on stop and replay, keeps the canvas, and drops an old decoder output", async () => {
    const { player, canvas } = await makePlayer();
    player._handleH264InitInternal([new Uint8Array([7, 0x64, 0, 0x29]), new Uint8Array([8, 1])]);
    const oldOutput = player.decoder.callbacks.output;
    present(player, 1_000_000);
    present(player, 2_000_000);
    player.stop();
    expect(player.getPlaybackTime()).toBeNull();
    expect(canvas.ctx.clearRect).not.toHaveBeenCalled();
    await player.play("ws://test.invalid/new-session");
    const stale = new TestFrame(99_000_000);
    oldOutput(stale);
    expect(stale.close).toHaveBeenCalledOnce();
    expect(player.getPlaybackTime()).toBeNull();
    present(player, 9_000_000);
    expect(player.getPlaybackTime()).toBe(0);
  });

  it("does not emit playback time for live RTP frames", async () => {
    const { player, onPlaybackTime } = await makePlayer(false);
    present(player, 0);
    present(player, 1_000_000);
    expect(player.getPlaybackTime()).toBeNull();
    expect(onPlaybackTime).not.toHaveBeenCalled();
  });

  it("does not revive a stopped session when asynchronous WASM initialization finishes", async () => {
    const { player } = await makePlayer();
    player.stop();
    const worker = { postMessage: vi.fn(), terminate: vi.fn() };
    let finish!: (value: typeof worker) => void;
    vi.spyOn(player, "_createTransformWorker").mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
    const starting = player.play("ws://test.invalid/pending");
    player.stop();
    finish(worker);
    await starting;
    expect(worker.terminate).toHaveBeenCalledOnce();
    expect(player.ws).toBeNull();
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe("NVR verified speed control", () => {
  it.each([1, 2, 4, 8, 16])("verifies %ix using multiple presented PTS windows, never send or ACK alone", async (rate) => {
    const { player, socket } = await makePlayer();
    present(player, 1_000_000);
    const settled = vi.fn();
    const result = outcome(player.setSpeed(rate));
    void result.then(settled);
    expect(JSON.parse(socket.send.mock.calls[0][0] as string)).toMatchObject({ cmd: "speed", rate, sequence: 0 });
    socket.message({ statusString: "OK" });
    socket.message({ status: "200" });
    socket.message({ res: 0 });
    await Promise.resolve();
    expect(settled).not.toHaveBeenCalled();
    feed(player, rate);
    expect(await result).toBe("verified");
    expect(vi.getTimerCount()).toBe(1);
  });

  it.each([1, 2, 4, 8, 16])("rejects %ix when the observed rate is wrong", async (rate) => {
    const { player } = await makePlayer();
    const result = outcome(player.setSpeed(rate));
    feed(player, rate === 1 ? 2 : 1, 7000);
    expect(await result).toBeInstanceOf(Error);
    expect(vi.getTimerCount()).toBe(1);
  });

  it.each([1, 2, 4, 8, 16])("accepts sustained %ix with ordinary delivery jitter", async (rate) => {
    const { player } = await makePlayer();
    const result = outcome(player.setSpeed(rate));
    present(player, 1_000_000);
    const delays = [32, 48, 36, 44];
    for (let frame = 1; frame <= 90; frame++) {
      vi.advanceTimersByTime(delays[(frame - 1) % delays.length]);
      present(player, 1_000_000 + frame * 40_000 * rate);
    }
    expect(await result).toBe("verified");
  });

  it.each([0, -1, 3, 32, NaN])("rejects unsupported rate %s without sending", async (rate) => {
    const { player, socket } = await makePlayer();
    expect(await outcome(player.setSpeed(rate))).toBeInstanceOf(Error);
    expect(socket.send).not.toHaveBeenCalled();
  });

  it("rejects live, stopped and unavailable sockets", async () => {
    const { player, socket } = await makePlayer(false);
    expect(await outcome(player.setSpeed(2))).toBeInstanceOf(Error);
    player.useWasm = true;
    socket.readyState = 3;
    expect(await outcome(player.setSpeed(2))).toBeInstanceOf(Error);
    player.stop();
    expect(await outcome(player.setSpeed(2))).toBeInstanceOf(Error);
  });

  it.each([1, 2, 4, 8, 16])("times out at %ix without frames or with frozen PTS and releases its timer", async (rate) => {
    for (const frozen of [false, true]) {
      const { player } = await makePlayer();
      const timerCount = vi.getTimerCount();
      const result = outcome(player.setSpeed(rate));
      if (frozen) present(player, 1_000_000);
      for (let i = 0; i < 70; i++) {
        vi.advanceTimersByTime(100);
        if (frozen) present(player, 1_000_000);
      }
      expect(await result).toBeInstanceOf(Error);
      expect(vi.getTimerCount()).toBe(timerCount);
    }
  });

  it("cannot verify a burst or a single timestamp jump as sustained speed", async () => {
    const { player } = await makePlayer();
    const result = outcome(player.setSpeed(16));
    for (let i = 0; i < 40; i++) present(player, i * 16_000_000);
    vi.advanceTimersByTime(1500);
    present(player, 640_000_000);
    vi.advanceTimersByTime(6000);
    expect(await result).toBeInstanceOf(Error);
  });

  it.each(["stop", "close", "error", "pause", "replay"])("cancels pending verification on %s", async (action) => {
    const { player, socket } = await makePlayer();
    const result = outcome(player.setSpeed(4));
    if (action === "stop") player.stop();
    if (action === "close") socket.onclose?.({ code: 1006, wasClean: false });
    if (action === "error") socket.onerror?.();
    if (action === "pause") player.pause();
    if (action === "replay") await player.play("ws://test.invalid/replay");
    expect(await result).toBeInstanceOf(Error);
    expect(vi.getTimerCount()).toBe(action === "stop" ? 0 : 1);
  });

  it("serializes commands by rejecting another request while verification is pending", async () => {
    const { player, socket } = await makePlayer();
    const first = outcome(player.setSpeed(2));
    expect(await outcome(player.setSpeed(4))).toBeInstanceOf(Error);
    expect(socket.send).toHaveBeenCalledOnce();
    feed(player, 2);
    expect(await first).toBe("verified");
  });

  it("rejects a socket that closes before its close event can arrive", async () => {
    const { player, socket } = await makePlayer();
    const result = outcome(player.setSpeed(2));
    socket.readyState = 3;
    feed(player, 2);
    expect(await result).toBeInstanceOf(Error);
    expect(vi.getTimerCount()).toBe(1);
  });

  it("rejects a send failure and clears pending verification", async () => {
    const { player, socket } = await makePlayer();
    socket.send.mockImplementationOnce(() => { throw new Error("send failed"); });
    expect(await outcome(player.setSpeed(2))).toBeInstanceOf(Error);
    expect(vi.getTimerCount()).toBe(1);
  });

  it("does not accept one matching window followed by a normal-speed window", async () => {
    const { player } = await makePlayer();
    const result = outcome(player.setSpeed(4));
    feed(player, 4, 1600);
    feed(player, 1, 5400, 7_400_000);
    expect(await result).toBeInstanceOf(Error);
  });

  it.each([2, 4, 8, 16])("does not mistake periodic PTS jumps with otherwise 1x cadence for %ix", async (rate) => {
    const { player } = await makePlayer();
    const result = outcome(player.setSpeed(rate));
    let pts = 1_000_000;
    present(player, pts);
    for (let elapsed = 100; elapsed <= 7000; elapsed += 100) {
      vi.advanceTimersByTime(100);
      pts += 100_000;
      if (elapsed % 1000 === 0) pts += (rate - 1) * 1_000_000;
      present(player, pts);
    }
    expect(await result).toBeInstanceOf(Error);
  });

  it("rejects alternating 1x/7x PTS steps whose average impersonates 4x", async () => {
    const { player } = await makePlayer();
    const result = outcome(player.setSpeed(4));
    let pts = 1_000_000;
    for (let elapsed = 100; elapsed <= 7000; elapsed += 100) {
      vi.advanceTimersByTime(100);
      pts += elapsed % 200 === 0 ? 700_000 : 100_000;
      present(player, pts);
    }
    expect(await result).toBeInstanceOf(Error);
  });

  it.each([{ statusString: "Error" }, { status: "500" }, { res: -1 }, { status: 200, res: 5 }])("rejects explicit protocol failure %j without requiring sequence or cmd", async (reply) => {
    const { player, socket } = await makePlayer();
    const result = outcome(player.setSpeed(2));
    socket.message(reply);
    expect(await result).toBeInstanceOf(Error);
    expect(vi.getTimerCount()).toBe(1);
  });

  it.each([1, 2, 4, 8, 16])("uses the documented 36-byte big-endian command after the 0.1 handshake (%ix)", async (rate) => {
    const { player, socket } = await makePlayer();
    socket.message({ version: "0.1" });
    const result = outcome(player.setSpeed(rate));
    const bytes = new Uint8Array(socket.send.mock.calls[0][0] as ArrayBuffer);
    const expected = new Uint8Array(36);
    const view = new DataView(expected.buffer);
    view.setUint32(0, 36);
    view.setUint32(12, 0x0003012f);
    view.setUint32(32, rate);
    expect(bytes).toEqual(expected);
    feed(player, rate);
    expect(await result).toBe("verified");
  });

  it("rejects unknown protocol versions and ignores malformed/unrelated text without logging it", async () => {
    const { player, socket } = await makePlayer();
    const log = vi.spyOn(console, "log");
    socket.onmessage?.({ data: "not-json-sensitive" });
    socket.message({ unrelated: true });
    socket.message({ version: "future" });
    expect(await outcome(player.setSpeed(2))).toBeInstanceOf(Error);
    expect(socket.send).not.toHaveBeenCalled();
    expect(log).not.toHaveBeenCalled();
  });

  it("cancels a command on protocol change and uses JSON again after a new play", async () => {
    const { player, socket } = await makePlayer();
    const pending = outcome(player.setSpeed(2));
    socket.message({ version: "0.1" });
    expect(await pending).toBeInstanceOf(Error);
    await player.play("ws://test.invalid/new-session");
    const normal = outcome(player.setSpeed(1));
    expect(JSON.parse(player.ws.send.mock.calls[0][0] as string)).toMatchObject({ cmd: "speed", rate: 1 });
    feed(player, 1);
    expect(await normal).toBe("verified");
  });
});

describe("NVR playback control diagnostics", () => {
  const idle: PlaybackControlDiagnostics = {
    phase: "idle", requestedRate: 1, verifiedRate: 1, protocol: "json",
    commandSequence: null, ack: "none", observedRate: null, failure: null,
  };

  it("retains an accepted ACK and measured 1x when 2x times out at 6000ms", async () => {
    const { player, socket, onPlaybackControlDiagnostics: report } = await makePlayer();
    expect(player.getPlaybackControlDiagnostics()).toEqual(idle);
    const result = outcome(player.setSpeed(2));
    expect(report).toHaveBeenLastCalledWith({
      ...idle, phase: "sent", requestedRate: 2, commandSequence: 0,
    });
    socket.message({ sequence: 0, cmd: "speed", status: 200 });
    expect(report).toHaveBeenLastCalledWith(expect.objectContaining({ ack: "accepted", phase: "sent" }));
    feed(player, 1, 5900);
    expect(player.getPlaybackControlDiagnostics()).toMatchObject({ phase: "verifying", observedRate: 1 });
    expect(report.mock.calls.filter(([s]) => s.phase === "verifying")).toHaveLength(5);
    vi.advanceTimersByTime(100);
    expect(await result).toBeInstanceOf(Error);
    const failed = { ...idle, phase: "failed", requestedRate: 2, commandSequence: 0,
      ack: "accepted", observedRate: 1, failure: "timeout" };
    expect(player.getPlaybackControlDiagnostics()).toEqual(failed);
    expect(report).toHaveBeenLastCalledWith(failed);
    player.stop();
    expect(player.getPlaybackControlDiagnostics()).toEqual(failed);
    await player.play("ws://test.invalid/new-session?token=never-in-diagnostics");
    expect(player.getPlaybackControlDiagnostics()).toEqual(idle);
    expect(report).toHaveBeenLastCalledWith(idle);
  });

  it("records no ACK and no observation on a silent timeout", async () => {
    const { player, onPlaybackControlDiagnostics: report } = await makePlayer();
    const result = outcome(player.setSpeed(4));
    vi.advanceTimersByTime(6000);
    expect(await result).toBeInstanceOf(Error);
    expect(player.getPlaybackControlDiagnostics()).toEqual({
      ...idle, phase: "failed", requestedRate: 4, commandSequence: 0, failure: "timeout",
    });
    expect(report).toHaveBeenLastCalledWith(player.getPlaybackControlDiagnostics());
  });

  it.each([{ statusString: "Error" }, { status: 500 }, { res: 5 }, { status: 200, res: -1 }])(
    "records explicit rejection %j without storing reply contents", async (reply) => {
      const { player, socket } = await makePlayer();
      const result = outcome(player.setSpeed(2));
      socket.message({ ...reply, url: "wss://secret", token: "private", raw: "private" });
      expect(await result).toBeInstanceOf(Error);
      expect(player.getPlaybackControlDiagnostics()).toEqual({
        ...idle, phase: "failed", requestedRate: 2, commandSequence: 0, ack: "rejected", failure: "rejected",
      });
    },
  );

  it("ignores late old-sequence and unrelated-command replies, and requires a status for ACK", async () => {
    const { player, socket, onPlaybackControlDiagnostics: report } = await makePlayer();
    const first = outcome(player.setSpeed(2));
    feed(player, 2);
    expect(await first).toBe("verified");
    const second = outcome(player.setSpeed(4));
    const sent = { ...idle, phase: "sent", requestedRate: 4, verifiedRate: 2, commandSequence: 1 };
    expect(player.getPlaybackControlDiagnostics()).toEqual(sent);
    report.mockClear();
    socket.message({ sequence: 0, cmd: "speed", status: 500 });
    socket.message({ sequence: 0, cmd: "speed", status: 200 });
    socket.message({ sequence: 1, cmd: "pause", status: 200 });
    socket.message({ sequence: 1, cmd: "speed", token: "private" });
    socket.message({ unrelated: true });
    expect(player.getPlaybackControlDiagnostics()).toEqual(sent);
    expect(report).not.toHaveBeenCalled();
    socket.message({ sequence: 1, cmd: "speed", res: 0 });
    expect(player.getPlaybackControlDiagnostics().ack).toBe("accepted");
    feed(player, 4, 3500, 20_000_000);
    expect(await second).toBe("verified");
    const verified = player.getPlaybackControlDiagnostics();
    expect(verified).toEqual({ ...sent, phase: "verified", ack: "accepted", observedRate: 4, verifiedRate: 4 });
    socket.message({ sequence: 1, cmd: "speed", status: 500 });
    expect(player.getPlaybackControlDiagnostics()).toEqual(verified);
  });

  it.each([{ statusString: "OK" }, { status: "200" }, { res: 0 }])("publishes accepted status %j without requiring sequence/cmd", async (reply) => {
    const { player, socket } = await makePlayer();
    const result = outcome(player.setSpeed(2));
    socket.message(reply);
    expect(player.getPlaybackControlDiagnostics().ack).toBe("accepted");
    feed(player, 2);
    expect(await result).toBe("verified");
    expect(player.getPlaybackControlDiagnostics()).toMatchObject({ phase: "verified", verifiedRate: 2, observedRate: 2 });
  });

  it("reports 0.1 commands with no wire sequence and preserves the existing binary message", async () => {
    const { player, socket } = await makePlayer();
    socket.message({ version: "0.1" });
    const result = outcome(player.setSpeed(4));
    expect(player.getPlaybackControlDiagnostics()).toEqual({
      ...idle, phase: "sent", requestedRate: 4, protocol: "0.1", commandSequence: null,
    });
    expect(new Uint8Array(socket.send.mock.calls[0][0] as ArrayBuffer)).toHaveLength(36);
    feed(player, 4);
    expect(await result).toBe("verified");
    expect(player.getPlaybackControlDiagnostics()).toMatchObject({ phase: "verified", protocol: "0.1", ack: "none" });
  });

  it.each(["stop", "pause", "close", "error", "protocol", "send"])("uses a bounded failure code for %s", async (action) => {
    const { player, socket } = await makePlayer();
    if (action === "send") socket.send.mockImplementationOnce(() => { throw new Error("token=private wss://secret"); });
    const result = outcome(player.setSpeed(2));
    if (action === "stop") player.stop();
    if (action === "pause") player.pause();
    if (action === "close") socket.onclose?.({ code: 1006, wasClean: false });
    if (action === "error") socket.onerror?.();
    if (action === "protocol") socket.message({ version: "private-future-version" });
    expect(await result).toBeInstanceOf(Error);
    const failures = { stop: "stopped", pause: "stopped", close: "connection", error: "connection", protocol: "protocol", send: "send_failed" };
    const failed = player.getPlaybackControlDiagnostics();
    expect(failed).toMatchObject({ phase: "failed", failure: failures[action as keyof typeof failures] });
    expect(JSON.stringify(failed)).not.toMatch(/private|secret|token|wss/);
    player.stop();
    expect(player.getPlaybackControlDiagnostics()).toEqual(failed);
  });

  it("reports unsupported requests and disconnected preconditions without sending", async () => {
    const { player, socket } = await makePlayer();
    expect(await outcome(player.setSpeed(3))).toBeInstanceOf(Error);
    expect(player.getPlaybackControlDiagnostics()).toMatchObject({ phase: "failed", failure: "unsupported", requestedRate: 3, commandSequence: null });
    socket.readyState = 3;
    expect(await outcome(player.setSpeed(2))).toBeInstanceOf(Error);
    expect(player.getPlaybackControlDiagnostics()).toMatchObject({ phase: "failed", failure: "connection", requestedRate: 2 });
    expect(socket.send).not.toHaveBeenCalled();
  });

  it.each(["live", "stopped", "paused", "protocol", "NaN", "Infinity", "string"])("captures the %s early rejection before catch runs", async (reason) => {
    const { player, socket, onPlaybackControlDiagnostics: report } = await makePlayer(reason !== "live");
    if (reason === "stopped") player.stop();
    if (reason === "paused") player.pause();
    if (reason === "protocol") socket.message({ version: "future-private" });
    const rate = reason === "NaN" ? NaN : reason === "Infinity" ? Infinity : reason === "string" ? "token=private" as unknown as number : 2;
    const failure = reason === "protocol" ? "protocol" : ["stopped", "paused"].includes(reason) ? "stopped" : "unsupported";
    await player.setSpeed(rate).then(() => { throw new Error("must reject"); }, () => {
      const snapshot = player.getPlaybackControlDiagnostics();
      expect(snapshot).toEqual({
        ...idle, phase: "failed", requestedRate: Number.isFinite(rate) ? rate : 1,
        protocol: reason === "protocol" ? "unsupported" : "json", failure,
      });
      expect(report).toHaveBeenLastCalledWith(snapshot);
      expect(JSON.stringify(snapshot)).not.toMatch(/private|token|NaN|Infinity/);
    });
    expect(socket.send).not.toHaveBeenCalled();
  });

  it("does not overwrite a pending request for duplicate or invalid calls", async () => {
    const { player, socket, onPlaybackControlDiagnostics: report } = await makePlayer();
    const result = outcome(player.setSpeed(2));
    socket.message({ status: 200 });
    const snapshot = player.getPlaybackControlDiagnostics();
    report.mockClear();
    expect(await outcome(player.setSpeed(4))).toBeInstanceOf(Error);
    expect(await outcome(player.setSpeed(NaN))).toBeInstanceOf(Error);
    expect(player.getPlaybackControlDiagnostics()).toEqual(snapshot);
    expect(report).not.toHaveBeenCalled();
    expect(socket.send).toHaveBeenCalledOnce();
    feed(player, 2);
    expect(await result).toBe("verified");
  });

  it("returns only whitelisted primitives as copies and isolates callback failures", async () => {
    const { player, socket, onPlaybackControlDiagnostics: report } = await makePlayer();
    const keys = Object.keys(idle).sort();
    report.mockImplementation((snapshot) => {
      expect(Object.keys(snapshot).sort()).toEqual(keys);
      snapshot.ack = "rejected";
      (snapshot as unknown as Record<string, unknown>).raw = "token=private";
      throw new Error("callback must not change playback");
    });
    const result = outcome(player.setSpeed(2));
    socket.message({ status: 200, token: "private", url: "wss://private", raw: "private", userId: "private" });
    const copy = player.getPlaybackControlDiagnostics();
    expect(copy.ack).toBe("accepted");
    copy.ack = "rejected";
    (copy as unknown as Record<string, unknown>).token = "private";
    expect(player.getPlaybackControlDiagnostics().ack).toBe("accepted");
    feed(player, 2);
    expect(await result).toBe("verified");
    expect(Object.keys(player.getPlaybackControlDiagnostics()).sort()).toEqual(keys);
    expect(JSON.stringify(player.getPlaybackControlDiagnostics())).not.toMatch(/private|secret|token|raw|url|userId|wss/);
  });
});

describe("NVR playback audio cancellation", () => {
  async function audiblePlayer(playback = true) {
    const mock = audioContext();
    vi.stubGlobal("window", { VideoDecoder: TestDecoder, AudioContext: vi.fn(function () { return mock.context; }) });
    const { player } = await makePlayer(playback);
    player.setVolume(65);
    await player.enableAudio();
    return { player, ...mock };
  }

  function queueAudio(player: Player) {
    for (let i = 0; i < 12; i++) player._playAudioPCM(new Int16Array(160).fill(12000));
  }

  it.each(["reset", "stop", "pause"])("cancels all scheduled fallback sources on %s", async (action) => {
    const { player, sources } = await audiblePlayer(false);
    queueAudio(player);
    expect(sources.length).toBeGreaterThan(0);
    if (action === "reset") player._resetAudioStream();
    if (action === "stop") player.stop();
    if (action === "pause") player.pause();
    for (const source of sources) {
      expect(source.stop).toHaveBeenCalledOnce();
      expect(source.disconnect).toHaveBeenCalledOnce();
    }
    player._resetAudioStream();
    expect(sources[0].stop).toHaveBeenCalledOnce();
  });

  it("mutes and clears audio during non-1x verification, restoring the caller volume only after verified 1x", async () => {
    const { player, sources, gain } = await audiblePlayer();
    queueAudio(player);
    const result = outcome(player.setSpeed(4));
    expect(gain.gain.value).toBe(0);
    expect(sources[0].stop).toHaveBeenCalledOnce();
    const count = sources.length;
    queueAudio(player);
    await player.enableAudio();
    player.setVolume(35);
    expect(sources).toHaveLength(count);
    expect(gain.gain.value).toBe(0);
    feed(player, 4);
    expect(await result).toBe("verified");
    const normal = outcome(player.setSpeed(1));
    queueAudio(player);
    expect(sources).toHaveLength(count);
    feed(player, 1, 3500, 15_000_000);
    expect(await normal).toBe("verified");
    expect(player.currentVolume).toBe(35);
    expect(gain.gain.value).toBe(0.35);
    queueAudio(player);
    expect(sources.length).toBeGreaterThan(count);
  });

  it("resets worklet queues and never pushes speed-muted PCM", async () => {
    const { player } = await audiblePlayer();
    const port = { postMessage: vi.fn() };
    player.workletNode = { port, disconnect: vi.fn() };
    const pending = outcome(player.setSpeed(2));
    queueAudio(player);
    expect(port.postMessage).toHaveBeenCalledWith({ type: "reset" });
    expect(port.postMessage.mock.calls.some(([message]) => message.type === "push")).toBe(false);
    player.stop();
    expect(await pending).toBeInstanceOf(Error);
  });

  it("continues live audio and local pause/resume after unsupported speed control", async () => {
    const { player, sources, gain } = await audiblePlayer(false);
    expect(await outcome(player.setSpeed(2))).toBeInstanceOf(Error);
    queueAudio(player);
    expect(gain.gain.value).toBe(0.65);
    expect(sources.length).toBeGreaterThan(0);
    player.pause();
    const count = sources.length;
    queueAudio(player);
    expect(sources).toHaveLength(count);
    player.resume();
    queueAudio(player);
    expect(sources.length).toBeGreaterThan(count);
  });

  it("releases naturally ended sources without stopping or disconnecting them twice", async () => {
    const { player, sources } = await audiblePlayer(false);
    queueAudio(player);
    const source = sources[0];
    source.onended?.();
    player._resetAudioStream();
    expect(source.disconnect).toHaveBeenCalledOnce();
    expect(source.stop).not.toHaveBeenCalled();
  });

  it.each(["first", "suspended", "interrupted"])("prepares %s audio during a muted speed gesture, then restores it at verified 1x", async (scenario) => {
    const mock = audioContext();
    const audioSession = { type: "auto" };
    vi.stubGlobal("navigator", { audioSession });
    vi.stubGlobal("window", { VideoDecoder: TestDecoder, AudioContext: vi.fn(function () { return mock.context; }) });
    const { player } = await makePlayer();
    player.setVolume(80);
    if (scenario !== "first") await player.enableAudio();
    const fast = outcome(player.setSpeed(4));
    feed(player, 4);
    expect(await fast).toBe("verified");
    if (scenario === "first") expect(player.audioContext).toBeNull();
    mock.context.state = scenario === "interrupted" ? "interrupted" : "suspended";
    const enabling = player.enableAudio();
    // Creation/resume must occur synchronously inside the gesture, before worklet awaits.
    expect(player.audioContext).toBe(mock.context);
    expect(mock.context.resume).toHaveBeenCalledOnce();
    expect(mock.gain.gain.value).toBe(0);
    expect(audioSession.type).toBe("auto");
    await enabling;
    queueAudio(player);
    expect(mock.sources).toHaveLength(0);
    const normal = outcome(player.setSpeed(1));
    feed(player, 1, 3500, 15_000_000);
    expect(await normal).toBe("verified");
    expect(mock.context.state).toBe("running");
    expect(mock.gain.gain.value).toBe(0.8);
    expect(audioSession.type).toBe("playback");
    queueAudio(player);
    expect(mock.sources.length).toBeGreaterThan(0);
  });

  it("retains gesture-prepared audio when the new player's session starts afterwards", async () => {
    const mock = audioContext("suspended");
    vi.stubGlobal("window", { VideoDecoder: TestDecoder, AudioContext: vi.fn(function () { return mock.context; }) });
    const player = new NVRPlayer(new TestCanvas() as unknown as HTMLCanvasElement, { forceWasm: true }) as Player;
    players.push(player);
    player.setVolume(80);
    const enabling = player.enableAudio();
    expect(player.audioContext).toBe(mock.context);
    expect(mock.context.resume).toHaveBeenCalledOnce();
    await enabling;
    await player.play("ws://test.invalid/resumed-session");
    expect(player.audioContext).toBe(mock.context);
    expect(mock.context.close).not.toHaveBeenCalled();
    queueAudio(player);
    expect(mock.gain.gain.value).toBe(0.8);
    expect(mock.sources.length).toBeGreaterThan(0);
  });
});
