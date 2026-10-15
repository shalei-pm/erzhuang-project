import { afterEach, describe, expect, it, vi } from "vitest";
import NVRPlayer from "../vendor/nvr-player/nvr-player.js";

function audioContext(state = "running") {
  const gain = { gain: { value: 0 }, connect: vi.fn() };
  const source = { buffer: null, connect: vi.fn(), start: vi.fn() };
  const context = {
    state,
    currentTime: 3,
    destination: {},
    createGain: vi.fn(() => gain),
    createBuffer: vi.fn((_channels: number, length: number, sampleRate: number) => ({
      duration: length / sampleRate,
      getChannelData: () => new Float32Array(length),
    })),
    createBufferSource: vi.fn(() => source),
    createBiquadFilter: vi.fn(() => ({ frequency: { value: 0 }, Q: { value: 0 }, connect: vi.fn() })),
    resume: vi.fn(async () => { context.state = "running"; }),
    close: vi.fn(async () => { context.state = "closed"; }),
  };
  return { context, gain, source };
}

type AudioPlayer = NVRPlayer & {
  audioContext: ReturnType<typeof audioContext>["context"] | null;
  nextAudioTime: number;
  _initAudioContext(): void;
  _playAudioPCM(samples: Int16Array): void;
  _decodeRTP(packet: { payloadType: number; sequenceNumber: number; payload: Uint8Array }): void;
  _handleH264Frame(buffer: ArrayBuffer, frameInfo: { nFrameType: number }): void;
};

const players: AudioPlayer[] = [];

function playerWithAudio(context = audioContext()) {
  vi.stubGlobal("document", { addEventListener: vi.fn(), removeEventListener: vi.fn(), visibilityState: "visible" });
  vi.stubGlobal("navigator", { audioSession: { type: "auto" } });
  vi.stubGlobal("window", { AudioContext: vi.fn(function () { return context.context; }) });
  const player = Object.assign(Object.create(NVRPlayer.prototype), {
    audioContext: context.context,
    gainNode: context.gain,
    currentVolume: 80,
    nextAudioTime: 120,
    AUDIO_PAYLOAD_TYPE: 8,
    diagnostics: { audioPayloadPackets: 0 },
    onDiagnostics: vi.fn(),
  }) as AudioPlayer;
  players.push(player);
  return { player, ...context };
}

afterEach(() => {
  for (const player of players.splice(0)) player.stop();
  vi.unstubAllGlobals();
});

describe("NVR audio activation", () => {
  it("resumes an iOS interrupted context on the unmute gesture", async () => {
    const { player, context } = playerWithAudio(audioContext("interrupted"));
    await player.enableAudio();
    expect(context.resume).toHaveBeenCalledOnce();
    expect(context.state).toBe("running");
    expect(player.nextAudioTime).toBe(context.currentTime);
  });

  it("requests playback audio routing before creating the context", async () => {
    const { player, context } = playerWithAudio();
    player.audioContext = null;
    let typeAtCreation = "";
    vi.stubGlobal("window", { AudioContext: vi.fn(function () {
      typeAtCreation = (navigator as unknown as { audioSession: { type: string } }).audioSession.type;
      return context;
    }) });
    await player.enableAudio();
    expect(typeAtCreation).toBe("playback");
  });

  it("does not report successful activation when the browser remains interrupted", async () => {
    const { player, context } = playerWithAudio(audioContext("interrupted"));
    context.resume.mockImplementation(async () => {});
    await expect(player.enableAudio()).rejects.toThrow();
  });

  it("recreates a context closed by the browser", async () => {
    const { player, context } = playerWithAudio(audioContext("closed"));
    const replacement = audioContext();
    vi.stubGlobal("window", { AudioContext: vi.fn(function () { return replacement.context; }) });
    await player.enableAudio();
    expect(player.audioContext).toBe(replacement.context);
    expect(context.resume).not.toHaveBeenCalled();
  });

  it.each(["suspended", "interrupted"])("does not enqueue either live or playback audio while %s", (state) => {
    const { player, context } = playerWithAudio(audioContext(state));
    player._decodeRTP({ payloadType: 8, sequenceNumber: 1, payload: new Uint8Array([0xd5, 0x55]) });
    player._handleH264Frame(new Uint8Array([0xd5, 0x55]).buffer, { nFrameType: 4 });
    expect(context.createBufferSource).not.toHaveBeenCalled();
  });

  it("schedules fresh audio at the current time after resuming", async () => {
    const { player, context, source } = playerWithAudio(audioContext("suspended"));
    await player.enableAudio();
    player._playAudioPCM(new Int16Array(160));
    expect(source.start).toHaveBeenCalledWith(context.currentTime);
  });

  it("works when the optional audio session API is unavailable", async () => {
    const { player, context } = playerWithAudio(audioContext("suspended"));
    vi.stubGlobal("navigator", {});
    await player.enableAudio();
    expect(context.state).toBe("running");
  });

  it("keeps playback routing until all players have muted or stopped", async () => {
    const first = playerWithAudio().player;
    const second = playerWithAudio().player;
    const session = (navigator as unknown as { audioSession: { type: string } }).audioSession;
    await first.enableAudio();
    await second.enableAudio();
    first.stop();
    expect(session.type).toBe("playback");
    second.setVolume(0);
    expect(session.type).toBe("auto");
  });

  it("does not queue muted audio that could be heard after unmuting", () => {
    const { player, context } = playerWithAudio();
    player.setVolume(0);
    player._playAudioPCM(new Int16Array(160));
    expect(context.createBufferSource).not.toHaveBeenCalled();
  });

  it("resumes interrupted sound when the user resumes video", () => {
    const { player, context } = playerWithAudio(audioContext("interrupted"));
    player.pause();
    player.resume();
    expect(context.resume).toHaveBeenCalledOnce();
  });

  it("restores audio routing when the last audible player stops", async () => {
    const { player } = playerWithAudio();
    const session = (navigator as unknown as { audioSession: { type: string } }).audioSession;
    await player.enableAudio();
    expect(session.type).toBe("playback");
    player.stop();
    expect(session.type).toBe("auto");
  });
});
