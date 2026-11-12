import { afterEach, describe, expect, it, vi } from "vitest";
import NVRPlayer from "../vendor/nvr-player/nvr-player.js";
import { PcmStreamEngine } from "../vendor/nvr-player/pcm-stream-engine.js";

function audioContext(state = "running", sampleRate = 48000) {
  const gain = {
    gain: { value: 0, setTargetAtTime: vi.fn((target: number) => { gain.gain.value = target; }) },
    connect: vi.fn(),
    disconnect: vi.fn(),
  };
  const sources: Array<{ buffer: AudioBuffer | null; connect: ReturnType<typeof vi.fn>; disconnect: ReturnType<typeof vi.fn>; start: ReturnType<typeof vi.fn>; onended: (() => void) | null }> = [];
  const filters: Array<{ type: string; frequency: { value: number }; Q: { value: number }; connect: ReturnType<typeof vi.fn>; disconnect: ReturnType<typeof vi.fn> }> = [];
  const context = {
    state,
    sampleRate,
    currentTime: 3,
    destination: {},
    createGain: vi.fn(() => gain),
    createBuffer: vi.fn((_channels: number, length: number, rate: number) => {
      const data = new Float32Array(length);
      return {
        length,
        sampleRate: rate,
        duration: length / rate,
        getChannelData: () => data,
      } as unknown as AudioBuffer;
    }),
    createBufferSource: vi.fn(() => {
      const source = { buffer: null, connect: vi.fn(), disconnect: vi.fn(), start: vi.fn(), onended: null };
      sources.push(source);
      return source;
    }),
    createBiquadFilter: vi.fn(() => {
      const filter = { type: "lowpass", frequency: { value: 0 }, Q: { value: 0 }, connect: vi.fn(), disconnect: vi.fn() };
      filters.push(filter);
      return filter;
    }),
    resume: vi.fn(async () => { context.state = "running"; }),
    close: vi.fn(async () => { context.state = "closed"; }),
  };
  return { context, gain, sources, filters };
}

type AudioPlayer = NVRPlayer & {
  audioContext: ReturnType<typeof audioContext>["context"] | null;
  nextAudioTime: number;
  _initAudioContext(): void;
  _playAudioPCM(samples: Int16Array, options?: { packetLoss?: number }): void;
  _decodeRTP(packet: { payloadType: number; sequenceNumber: number; payload: Uint8Array }): void;
  _handleH264Frame(buffer: ArrayBuffer, frameInfo: { nFrameType: number }): void;
};

const players: AudioPlayer[] = [];

function playerWithAudio(mock = audioContext()) {
  vi.stubGlobal("document", { addEventListener: vi.fn(), removeEventListener: vi.fn(), visibilityState: "visible" });
  vi.stubGlobal("navigator", { audioSession: { type: "auto" } });
  vi.stubGlobal("window", { AudioContext: vi.fn(function () { return mock.context; }) });
  const player = Object.assign(Object.create(NVRPlayer.prototype), {
    audioContext: null,
    gainNode: null,
    currentVolume: 80,
    nextAudioTime: 0,
    lastAudioSeq: null,
    audioPacketLoss: 0,
    AUDIO_PAYLOAD_TYPE: 8,
    diagnostics: { audioPayloadPackets: 0 },
    onDiagnostics: vi.fn(),
  }) as AudioPlayer;
  players.push(player);
  return { player, ...mock };
}

afterEach(() => {
  for (const player of players.splice(0)) player.stop();
  vi.unstubAllGlobals();
});

describe("PcmStreamEngine continuous resampling & de-click", () => {
  it("waits for the 80ms jitter buffer threshold before outputting audio and fades in smoothly", () => {
    const engine = new PcmStreamEngine({ inputSampleRate: 8000, outputSampleRate: 48000 });
    const packet = new Int16Array(160).fill(16384); // 20ms constant 0.5 amplitude

    // 3 packets = 60ms < 80ms threshold: should remain silent while pre-buffering
    for (let i = 0; i < 3; i++) engine.pushInt16(packet);
    const early = new Float32Array(480);
    engine.pull(early);
    expect(Math.max(...early.map(Math.abs))).toBe(0);

    // 4th packet reaches 80ms (640 samples): output starts with a smooth fade-in ramp instead of a 0 -> 0.5 step pop
    engine.pushInt16(packet);
    const started = new Float32Array(480);
    engine.pull(started);
    expect(Math.abs(started[0])).toBeLessThan(0.02);
    expect(started[479]).toBeGreaterThan(0.45);
  });

  it("produces zero packet-boundary discontinuities across 20ms 8kHz sine chunks at 48kHz and 44.1kHz", () => {
    for (const outputRate of [48000, 44100]) {
      const engine = new PcmStreamEngine({ inputSampleRate: 8000, outputSampleRate: outputRate });
      const amplitude = 24000; // Loud speech level (~0.73 full scale)
      for (let pkt = 0; pkt < 50; pkt++) {
        const chunk = new Int16Array(160);
        for (let i = 0; i < 160; i++) {
          const sampleIndex = pkt * 160 + i;
          chunk[i] = Math.round(Math.sin((2 * Math.PI * 440 * sampleIndex) / 8000) * amplitude);
        }
        engine.pushInt16(chunk);
      }

      const rendered = new Float32Array(Math.floor(outputRate * 0.8));
      // Render in Web Audio 128-frame quanta
      for (let offset = 0; offset + 128 <= rendered.length; offset += 128) {
        engine.pull(rendered.subarray(offset, offset + 128));
      }

      // Skip initial 15ms fade-in and inspect steady-state across dozens of 20ms packet boundaries
      const startCheck = Math.floor(outputRate * 0.03);
      let maxStep = 0;
      for (let i = startCheck + 1; i < rendered.length; i++) {
        maxStep = Math.max(maxStep, Math.abs(rendered[i] - rendered[i - 1]));
      }
      // Ideal 440Hz step at 44.1kHz with amp 0.733 is 2*pi*440/44100*0.733 ≈ 0.046.
      // Any boundary zero-drop or reset spike would jump > 0.25.
      expect(maxStep).toBeLessThan(0.065);
    }
  });

  it("ramps smoothly to silence on buffer underrun instead of hard-cutting non-zero waveform", () => {
    const engine = new PcmStreamEngine({ inputSampleRate: 8000, outputSampleRate: 48000 });
    const loud = new Int16Array(640).fill(20000); // 80ms loud DC/hold level (~0.61)
    engine.pushInt16(loud);

    // Drain the 80ms (3840 output samples) plus extra 480 samples into underrun
    const out = new Float32Array(3840 + 480);
    for (let offset = 0; offset < out.length; offset += 128) {
      engine.pull(out.subarray(offset, Math.min(out.length, offset + 128)));
    }

    let maxUnderrunJump = 0;
    for (let i = 3700; i < out.length; i++) {
      maxUnderrunJump = Math.max(maxUnderrunJump, Math.abs(out[i] - out[i - 1]));
    }
    expect(maxUnderrunJump).toBeLessThan(0.02);
    expect(out[out.length - 1]).toBe(0);
  });

  it("smooths abrupt sample jumps when RTP packet loss is reported", () => {
    const engine = new PcmStreamEngine({ inputSampleRate: 8000, outputSampleRate: 48000 });
    engine.pushInt16(new Int16Array(640).fill(18000));
    engine.pushInt16(new Int16Array(320).fill(-18000), { packetLoss: 1 });

    const out = new Float32Array(4800);
    engine.pull(out);
    const startCheck = Math.floor(48000 * 0.02);
    let maxJump = 0;
    for (let i = startCheck + 1; i < out.length; i++) {
      maxJump = Math.max(maxJump, Math.abs(out[i] - out[i - 1]));
    }
    // Without packet-loss seam smoothing, +0.55 to -0.55 jumps ~0.18 per 48k sample (1.10 over 6 samples).
    // With 2ms (96 samples @ 48k) seam smoothing, per-sample step stays ~0.012.
    expect(maxJump).toBeLessThan(0.03);
  });
});

describe("NVR audio activation & persistent filter graph", () => {
  it("resumes an iOS interrupted context synchronously on the unmute gesture", async () => {
    const mock = audioContext("interrupted");
    const { player, context } = playerWithAudio(mock);
    player.audioContext = context;
    await player.enableAudio();
    expect(context.resume).toHaveBeenCalledOnce();
    expect(context.state).toBe("running");
  });

  it("reuses a single persistent Butterworth filter chain across many live and playback packets", async () => {
    const { player, context, filters } = playerWithAudio(audioContext("suspended"));
    await player.enableAudio();

    expect(filters).toHaveLength(2);
    for (const filter of filters) {
      expect(filter.Q.value).toBeCloseTo(Math.SQRT1_2, 3);
    }

    const packet = new Uint8Array(160).fill(0x55);
    for (let seq = 1; seq <= 12; seq++) {
      player._decodeRTP({ payloadType: 8, sequenceNumber: seq, payload: packet });
      player._handleH264Frame(packet.slice().buffer, { nFrameType: 4 });
    }

    // Must NEVER create or disconnect BiquadFilters per audio packet!
    expect(context.createBiquadFilter).toHaveBeenCalledTimes(2);
    expect(filters[0].disconnect).not.toHaveBeenCalled();
    expect(filters[1].disconnect).not.toHaveBeenCalled();
    // Fallback scheduler must wait for jitter buffer and resample at context.sampleRate (48000), not 8000Hz tiny buffers
    expect(context.createBuffer).toHaveBeenCalled();
    for (const call of context.createBuffer.mock.calls) {
      expect(call[2]).toBe(context.sampleRate);
    }
  });

  it("requests playback audio routing before creating the context", async () => {
    const { player, context } = playerWithAudio();
    let typeAtCreation = "";
    vi.stubGlobal("window", { AudioContext: vi.fn(function () {
      typeAtCreation = (navigator as unknown as { audioSession: { type: string } }).audioSession.type;
      return context;
    }) });
    await player.enableAudio();
    expect(typeAtCreation).toBe("playback");
  });

  it("does not report successful activation when the browser remains interrupted", async () => {
    const mock = audioContext("interrupted");
    const { player, context } = playerWithAudio(mock);
    player.audioContext = context;
    context.resume.mockImplementation(async () => {});
    await expect(player.enableAudio()).rejects.toThrow();
  });

  it("recreates a context closed by the browser", async () => {
    const mock = audioContext("closed");
    const { player, context } = playerWithAudio(mock);
    player.audioContext = context;
    const replacement = audioContext();
    vi.stubGlobal("window", { AudioContext: vi.fn(function () { return replacement.context; }) });
    await player.enableAudio();
    expect(player.audioContext).toBe(replacement.context);
    expect(context.resume).not.toHaveBeenCalled();
  });

  it.each(["suspended", "interrupted"])("does not enqueue either live or playback audio while %s", (state) => {
    const mock = audioContext(state);
    const { player, context } = playerWithAudio(mock);
    player.audioContext = context;
    player._decodeRTP({ payloadType: 8, sequenceNumber: 1, payload: new Uint8Array(160).fill(0xd5) });
    player._handleH264Frame(new Uint8Array(160).fill(0x55).buffer, { nFrameType: 4 });
    expect(context.createBufferSource).not.toHaveBeenCalled();
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

  it("does not queue muted audio that could be heard after unmuting", async () => {
    const { player, context } = playerWithAudio();
    await player.enableAudio();
    player.setVolume(0);
    for (let i = 0; i < 6; i++) player._playAudioPCM(new Int16Array(160).fill(12000));
    expect(context.createBufferSource).not.toHaveBeenCalled();
  });

  it("resumes interrupted sound when the user resumes video", async () => {
    const mock = audioContext("interrupted");
    const { player, context } = playerWithAudio(mock);
    player.audioContext = context;
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
