import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import NVRPlayer from "../vendor/nvr-player/nvr-player.js";

class TestWorker {
  onmessage: ((event: { data: { type: string } }) => void) | null = null;
  onerror: ((event: { message: string }) => void) | null = null;
  postMessage = vi.fn();
  terminate = vi.fn();
  emit(type: string) { this.onmessage?.({ data: { type } }); }
}

class TestCanvas {
  getContext() { return { clearRect: vi.fn() }; }
}

class TestSocket {
  static OPEN = 1;
  readyState = TestSocket.OPEN;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn(() => { this.readyState = 3; });
}

type Player = NVRPlayer & {
  _isRunning: boolean;
  _playbackGeneration: number;
  _monitorTimer: ReturnType<typeof setInterval> | null;
  _initWasmDecoder(): Promise<void>;
  _createTransformWorker(): Promise<TestWorker>;
  _decodeH265Wasm(data: ArrayBuffer): void;
  transformWorker: TestWorker | null;
  ws: TestSocket | null;
  wasmRuntimeReady: boolean;
  wasmReady: boolean;
};

const players: Player[] = [];
const packet = (...bytes: number[]) => new Uint8Array(bytes).buffer;

async function setup() {
  const onError = vi.fn();
  const player = new NVRPlayer(new TestCanvas() as unknown as HTMLCanvasElement, { onError }) as Player;
  players.push(player);
  player._isRunning = true;
  player._playbackGeneration = 1;
  await player._initWasmDecoder();
  return { player, worker: player.transformWorker!, onError };
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.stubGlobal("HTMLCanvasElement", TestCanvas);
  vi.stubGlobal("Worker", TestWorker);
  vi.stubGlobal("document", { removeEventListener: vi.fn() });
});

afterEach(() => {
  for (const player of players.splice(0)) player.stop();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe("NVR WASM initialization input", () => {
  it("keeps the first header and delivers all early packets in order only after created", async () => {
    const { player, worker } = await setup();
    const header = packet(72, 68, 82);
    const beforeLoaded = packet(1, 2);
    const beforeCreated = packet(3, 4);
    player._decodeH265Wasm(header);
    player._decodeH265Wasm(beforeLoaded);
    expect(worker.postMessage).not.toHaveBeenCalled();
    worker.emit("loaded");
    expect(worker.postMessage.mock.calls).toEqual([[{ type: "create", buf: header, len: 3, packType: 11 }]]);
    player._decodeH265Wasm(beforeCreated);
    expect(worker.postMessage).toHaveBeenCalledTimes(1);
    worker.emit("created");
    player._decodeH265Wasm(packet(5));
    expect(worker.postMessage.mock.calls.map(([message]) => [message.type, [...new Uint8Array(message.buf)]])).toEqual([
      ["create", [72, 68, 82]], ["inputData", [1, 2]], ["inputData", [3, 4]], ["inputData", [5]],
    ]);
    worker.emit("loaded");
    worker.emit("created");
    expect(worker.postMessage).toHaveBeenCalledTimes(4);
  });

  it("copies queued input so later mutations cannot corrupt the header or media", async () => {
    const { player, worker } = await setup();
    const header = new Uint8Array([9]);
    const media = new Uint8Array([1]);
    player._decodeH265Wasm(header.buffer);
    player._decodeH265Wasm(media.buffer);
    header[0] = 99;
    media[0] = 99;
    worker.emit("loaded");
    worker.emit("created");
    expect(worker.postMessage.mock.calls.map(([message]) => [...new Uint8Array(message.buf)])).toEqual([[9], [1]]);
  });

  it.each(["packets", "bytes"])("stops the session on a pending %s limit before created and preserves the failure", async (limit) => {
    const { player, worker, onError } = await setup();
    const socket = new TestSocket();
    player.ws = socket;
    const monitor = player._monitorTimer;
    const clearMonitor = vi.spyOn(globalThis, "clearInterval");
    const oldWorkerMessage = worker.onmessage!;
    const oldWorkerError = worker.onerror!;
    player._decodeH265Wasm(packet(9));
    worker.emit("loaded");
    const size = limit === "bytes" ? 1024 * 1024 : 1;
    const count = limit === "bytes" ? 8 : 512;
    for (let index = 0; index < count; index++) player._decodeH265Wasm(new ArrayBuffer(size));
    expect(onError).not.toHaveBeenCalled();
    expect(socket.readyState).toBe(TestSocket.OPEN);
    expect(player._isRunning).toBe(true);
    expect(monitor).not.toBeNull();
    expect(vi.getTimerCount()).toBe(1);
    player._decodeH265Wasm(new ArrayBuffer(size));
    expect(onError).toHaveBeenCalledTimes(1);
    const failureMessage = "[NVRPlayer] WASM initialization buffer limit exceeded";
    expect(onError.mock.calls[0][0].message).toBe(failureMessage);
    expect(worker.terminate).toHaveBeenCalledTimes(1);
    expect.soft(socket.close).toHaveBeenCalledTimes(1);
    expect.soft(socket.readyState).toBe(3);
    expect.soft(player._isRunning).toBe(false);
    expect.soft(player._monitorTimer).toBeNull();
    expect.soft(clearMonitor).toHaveBeenCalledWith(monitor);
    expect.soft(vi.getTimerCount()).toBe(0);
    oldWorkerMessage({ data: { type: "loaded" } });
    oldWorkerMessage({ data: { type: "created" } });
    oldWorkerError({ message: "late error from failed worker" });
    player._decodeH265Wasm(packet(7));
    expect(player.transformWorker).toBeNull();
    expect(player.wasmRuntimeReady).toBe(false);
    expect(player.wasmReady).toBe(false);
    expect(worker.postMessage).toHaveBeenCalledTimes(1);
    expect(onError).toHaveBeenCalledTimes(1);
    expect(onError.mock.calls[0][0].message).toBe(failureMessage);
  });

  it("ignores late messages and errors from a replaced worker in the same playback generation", async () => {
    const { player, worker: oldWorker, onError } = await setup();
    player._decodeH265Wasm(packet(8));
    player._decodeH265Wasm(packet(1));
    await player._initWasmDecoder();
    const worker = player.transformWorker!;
    player._decodeH265Wasm(packet(9));
    player._decodeH265Wasm(packet(2));
    oldWorker.emit("loaded");
    oldWorker.emit("created");
    oldWorker.onerror?.({ message: "stale" });
    expect(worker.postMessage).not.toHaveBeenCalled();
    expect(onError).not.toHaveBeenCalled();
    worker.emit("loaded");
    worker.emit("created");
    expect(worker.postMessage.mock.calls.map(([message]) => [...new Uint8Array(message.buf)])).toEqual([[9], [2]]);
  });

  it("discards queued data across stop and restart and ignores previous-generation callbacks", async () => {
    const { player, worker: oldWorker, onError } = await setup();
    player._decodeH265Wasm(packet(8));
    player._decodeH265Wasm(packet(1));
    player.stop();
    player._isRunning = true;
    await player._initWasmDecoder();
    const worker = player.transformWorker!;
    oldWorker.emit("loaded");
    oldWorker.emit("created");
    oldWorker.onerror?.({ message: "stale" });
    player._decodeH265Wasm(packet(9));
    worker.emit("loaded");
    worker.emit("created");
    expect(worker.postMessage.mock.calls.map(([message]) => [...new Uint8Array(message.buf)])).toEqual([[9]]);
    expect(onError).not.toHaveBeenCalled();
  });

  it("terminates an asynchronously created worker when stopped before creation completes", async () => {
    const { player } = await setup();
    const lateWorker = new TestWorker();
    let resolveWorker!: (worker: TestWorker) => void;
    vi.spyOn(player, "_createTransformWorker").mockImplementation(() => new Promise((resolve) => { resolveWorker = resolve; }));
    const pending = player._initWasmDecoder();
    player.stop();
    resolveWorker(lateWorker);
    await pending;
    expect(lateWorker.terminate).toHaveBeenCalledTimes(1);
    expect(player.transformWorker).toBeNull();
  });

  it("does not let an older async initialization replace the newer worker", async () => {
    const { player } = await setup();
    const lateWorker = new TestWorker();
    const currentWorker = new TestWorker();
    let resolveWorker!: (worker: TestWorker) => void;
    vi.spyOn(player, "_createTransformWorker")
      .mockImplementationOnce(() => new Promise((resolve) => { resolveWorker = resolve; }))
      .mockResolvedValueOnce(currentWorker);
    const pending = player._initWasmDecoder();
    await player._initWasmDecoder();
    resolveWorker(lateWorker);
    await pending;
    expect(lateWorker.terminate).toHaveBeenCalledTimes(1);
    expect(player.transformWorker).toBe(currentWorker);
  });
});
