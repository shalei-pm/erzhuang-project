// Synthetic media boundary only: session HTTP, WebSocket control and SDK render/speed logic stay real.
import { createServer, request as proxyRequest } from "node:http";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

export const base = "/erzhuang-project/";
export const previewPath = `${base}__playback-preview`;

const bootstrap = String.raw`
import React from '/erzhuang-project/node_modules/.vite/deps/react.js';
import ReactDOM from '/erzhuang-project/node_modules/.vite/deps/react-dom_client.js';
import NVRPlayer from '/erzhuang-project/src/vendor/nvr-player/nvr-player.js';
import { NVRLabCamera } from '/erzhuang-project/src/pages/NVRLabCamera.tsx';
import { NVRLabPlayer } from '/erzhuang-project/src/components/NVRLabPlayer.tsx';
import '/erzhuang-project/src/styles.css';
const { createRoot } = ReactDOM;
const source = new URLSearchParams(location.search).get('fixture') || 'manual';
const recordingUTCOffsetMs = 480 * 60 * 1000;
const qa = window.__playbackQA = { players: [], events: [], renders: 0, speedRequests: [], unhandledRejections: [] };
window.addEventListener('unhandledrejection', event => qa.unhandledRejections.push(String(event.reason?.message || event.reason)));
function registerPlayer(player) {
  if (!qa.players.includes(player)) {
    player.__qaId = qa.players.length + 1;
    qa.players.push(player);
  }
}
const paint = document.createElement('canvas');
paint.width = 960; paint.height = 540;
const ctx = paint.getContext('2d');
let audioWave;
function feedSyntheticAudio(player) {
  if (!player.audioContext) return;
  if (!audioWave) {
    const decoded = player._decodePCMA(Uint8Array.from({ length: 256 }, (_, i) => i));
    // 440Hz at 8kHz repeats every 200 samples; encode once as real PCMA input.
    audioWave = Uint8Array.from({ length: 200 }, (_, i) => {
      const sample = Math.sin(2 * Math.PI * 440 * i / 8000) * 18000;
      let nearest = 0;
      for (let code = 1; code < 256; code++) {
        if (Math.abs(decoded[code] - sample) < Math.abs(decoded[nearest] - sample)) nearest = code;
      }
      return nearest;
    });
  }
  const offset = player.__qaAudioSamples || 0;
  const packet = Uint8Array.from({ length: 320 }, (_, i) => audioWave[(offset + i) % audioWave.length]);
  player.__qaAudioSamples = offset + packet.length;
  player.__qaAudioPackets = (player.__qaAudioPackets || 0) + 1;
  player._handleH264Frame(packet.buffer, { nFrameType: 4 });
}
function audioSignal(player) {
  if (!player.gainNode || player.audioContext?.state !== 'running') return null;
  if (player.__qaAudioProbe?.gain !== player.gainNode) {
    player.__qaAudioProbe?.analyser.disconnect();
    const analyser = player.audioContext.createAnalyser();
    analyser.fftSize = 2048;
    player.gainNode.connect(analyser);
    player.__qaAudioProbe = { gain: player.gainNode, analyser, samples: new Float32Array(analyser.fftSize) };
  }
  const { analyser, samples } = player.__qaAudioProbe;
  analyser.getFloatTimeDomainData(samples);
  let peak = 0, squares = 0;
  for (const sample of samples) { peak = Math.max(peak, Math.abs(sample)); squares += sample * sample; }
  return { peak, rms: Math.sqrt(squares / samples.length) };
}
NVRPlayer.prototype._initWasmDecoder = async function () {};
function presentSyntheticFrame(player, frame, packet) {
  const before = player.diagnostics.renderedFrames;
  player._renderFrame(frame);
  if (player.diagnostics.renderedFrames === before) return;
  if (packet.timeSource === 'recorded' && player.__qaPresented?.recordedPTS === (packet.recordedAt + recordingUTCOffsetMs) * 1000) {
    player.__qaRepeatedRecordedFrames = (player.__qaRepeatedRecordedFrames || 0) + 1;
  }
  player.__qaPresented = { sourcePTS: packet.pts, containerPTS: packet.containerPTS,
    recordedPTS: packet.timeSource === 'recorded' ? (packet.recordedAt + recordingUTCOffsetMs) * 1000 : null,
    recordedUnixSeconds: packet.timeSource === 'recorded' ? packet.recordedAt / 1000 : null,
    sourceRate: packet.rate, sourceFrame: packet.frame, presentedAt: performance.now() };
  qa.renders++;
}
function recordedFrameInfo(packet) {
  const time = new Date(packet.recordedAt + recordingUTCOffsetMs);
  return { nFrameType: 1, nTimeStamp: packet.containerPTS / 1000,
    nYear: time.getUTCFullYear(), nMonth: time.getUTCMonth() + 1, nDay: time.getUTCDate(),
    nHour: time.getUTCHours(), nMinute: time.getUTCMinutes(), nSecond: time.getUTCSeconds(),
    nMilliSecond: time.getUTCMilliseconds() };
}
function decodeRecordedFrame(player, packet) {
  if (!player.decoder?.__qaSyntheticDecoder) {
    // Stub compressed decode only. Real _handleH264Frame/_safeDecode/_renderFrame own frame timing.
    const outputs = new Map();
    player.decoder = { state: 'configured', __qaSyntheticDecoder: true,
      decode(chunk) {
        const sourcePacket = player.__qaDecodePacket;
        const frame = new VideoFrame(paint, { timestamp: chunk.timestamp });
        // Several metadata entries coexist before output, as with an asynchronous decoder.
        const timer = setTimeout(() => {
          outputs.delete(timer);
          presentSyntheticFrame(player, frame, sourcePacket);
        }, 80);
        outputs.set(timer, frame);
      },
      close() {
        this.state = 'closed';
        for (const [timer, frame] of outputs) { clearTimeout(timer); frame.close(); }
        outputs.clear();
      },
    };
    player.h264DecoderConfigured = true;
  }
  player.__qaDecodePacket = packet;
  player._handleH264Frame(Uint8Array.of(0, 0, 0, 1, 0x41).buffer, recordedFrameInfo(packet));
}
// Replace compressed-media decode only, keeping SDK sockets, generation, render PTS and speed verification.
NVRPlayer.prototype._decodeH265 = function (buffer) {
  const packet = JSON.parse(new TextDecoder().decode(buffer));
  if (packet.type !== 'frame') return;
  const t = packet.pts / 1000000;
  ctx.fillStyle = '#dce5df'; ctx.fillRect(0, 0, 960, 540);
  ctx.fillStyle = '#748e82'; ctx.fillRect(0, 330, 960, 210);
  ctx.fillStyle = '#f6f8f6'; ctx.fillRect(50, 65, 250, 215);
  ctx.fillStyle = '#557f91'; ctx.fillRect(65, 80, 220, 185);
  ctx.fillStyle = '#b7c6bf'; ctx.fillRect(480, 90, 340, 225);
  ctx.fillStyle = '#ffffff'; ctx.fillRect(505, 110, 290, 185);
  ctx.fillStyle = '#36594a'; ctx.fillRect(390 + Math.sin(t) * 150, 380, 100, 70);
  ctx.fillStyle = '#162c25'; ctx.font = '22px sans-serif';
  ctx.fillText('SYNTHETIC SOURCE / NOT A REAL CAMERA', 28, 35);
  ctx.fillText('SOURCE PTS ' + t.toFixed(3) + ' s  |  ' + packet.rate + 'x', 28, 510);
  ctx.fillText('CONTAINER ' + (packet.containerPTS / 1000000).toFixed(3) + ' s  |  ' + packet.timeSource, 28, 535);
  // Encode real source milliseconds in visible pixels so regressions can inspect the presented frame.
  const sourceMs = Math.floor(packet.pts / 1000);
  for (let bit = 0; bit < 32; bit++) {
    ctx.fillStyle = (sourceMs >>> bit) & 1 ? '#ffffff' : '#000000';
    ctx.fillRect(28 + bit * 8, 45, 8, 8);
  }
  // Independent absolute clock in the actual picture; decoded metadata cannot redefine this oracle.
  for (let bit = 0; bit < 42; bit++) {
    ctx.fillStyle = Math.floor(packet.recordedAt / 2 ** bit) % 2 ? '#ffffff' : '#000000';
    ctx.fillRect(28 + bit * 8, 57, 8, 8);
  }
  if (packet.timeSource === 'recorded') decodeRecordedFrame(this, packet);
  else presentSyntheticFrame(this, new VideoFrame(paint, { timestamp: packet.containerPTS }), packet);
  feedSyntheticAudio(this);
};
const originalSetSpeed = NVRPlayer.prototype.setSpeed;
NVRPlayer.prototype.setSpeed = function (rate) {
  const observation = { playerId: this.__qaId, rate, at: performance.now(), status: 'pending' };
  qa.speedRequests.push(observation);
  const result = originalSetSpeed.call(this, rate);
  result.then(() => Object.assign(observation, { status: 'resolved', settledAt: performance.now(),
    diagnostic: this.getPlaybackControlDiagnostics() }), error => Object.assign(observation, {
      status: 'rejected', settledAt: performance.now(), error: String(error?.message || error),
      diagnostic: this.getPlaybackControlDiagnostics() }));
  return result;
};
const originalPlay = NVRPlayer.prototype.play;
NVRPlayer.prototype.play = function (url) {
  registerPlayer(this);
  if (!this.__qaObservedPlaybackTime) {
    const callback = this.onPlaybackTime;
    this.onPlaybackTime = (...args) => {
      this.__qaPlaybackCallback = { elapsedSeconds: args[0], recordedUnixSeconds: args[1] ?? null };
      // Explicit local fault injection only; normal runs forward every production callback argument.
      return callback.apply(this, window.__qaOmitRecordedUnix ? [args[0]] : args);
    };
    this.__qaObservedPlaybackTime = true;
  }
  this.__qaPlayCount = (this.__qaPlayCount || 0) + 1;
  qa.events.push({ type: 'play', url, at: performance.now() });
  return originalPlay.call(this, url);
};
const originalStop = NVRPlayer.prototype.stop;
NVRPlayer.prototype.stop = function () {
  this.__qaStopCount = (this.__qaStopCount || 0) + 1;
  qa.events.push({ type: 'stop', url: this.wsUrl, mediaTime: this.getPlaybackTime(),
    recordedUnixSeconds: this.__qaPresented?.recordedUnixSeconds ?? null, at: performance.now() });
  return originalStop.call(this);
};
const originalEnableAudio = NVRPlayer.prototype.enableAudio;
NVRPlayer.prototype.enableAudio = function (...args) {
  registerPlayer(this);
  const injectFailure = qa.failNextPreparedUnlock && !this.__qaPlayCount;
  if (injectFailure) qa.failNextPreparedUnlock = false;
  const observation = { gesture: navigator.userActivation.isActive, at: performance.now(), injectedFailure: Boolean(injectFailure), state: 'pending' };
  (this.__qaAudioPreparations ||= []).push(observation);
  const unlocked = originalEnableAudio.apply(this, args);
  if (this.audioContext && !(this.__qaAudioContexts ||= []).includes(this.audioContext)) {
    this.__qaAudioContexts.push(this.audioContext);
  }
  return unlocked.then(async result => {
    if (injectFailure) {
      await new Promise(resolve => setTimeout(resolve, 120));
      throw new Error('Synthetic prepared audio unlock failure');
    }
    observation.state = 'resolved';
    return result;
  }).catch(error => { observation.state = 'rejected'; throw error; });
};
qa.snapshot = () => qa.players.map(p => ({ id: p.__qaId, hasPlayed: Boolean(p.__qaPlayCount),
  stopCount: p.__qaStopCount || 0, monitor: Boolean(p._monitorTimer),
  audioGestureListener: Boolean(p._audioGestureHandler), socket: Boolean(p.ws),
  audioContexts: (p.__qaAudioContexts || []).map(context => ({ state: context.state, time: context.currentTime })),
  running: p._isRunning, url: p.wsUrl,
  time: p.getPlaybackTime(), pts: p._playbackLastPTS, volume: p.currentVolume,
  gain: p.gainNode?.gain.value, mutedBySpeed: p._speedAudioMuted,
  pendingRate: p._pendingSpeed?.rate ?? null, renders: p.diagnostics.renderedFrames,
  control: p.getPlaybackControlDiagnostics(), ...p.__qaPresented,
  playbackCallback: p.__qaPlaybackCallback,
  repeatedRecordedFrames: p.__qaRepeatedRecordedFrames || 0,
  audioState: p.audioContext?.state, audioTime: p.audioContext?.currentTime,
  worklet: Boolean(p.workletNode),
  audioPackets: p.__qaAudioPackets || 0, audioSignal: audioSignal(p),
  audioGestureReady: p._audioGestureReady, audioPreparations: p.__qaAudioPreparations || [] }));
const root = createRoot(document.getElementById('root'));
const noop = () => {};
if (new URLSearchParams(location.search).has('component')) {
  const session = { mode: 'playback', url: 'ws://' + location.host + '${base}__fixture/stream?fixture=' + source + '&id=component' };
  let revision = 0;
  qa.rerenderCallbacks = () => {
    const value = ++revision;
    root.render(React.createElement(NVRLabPlayer, { session,
      playbackSegment: { start_time: 1000, end_time: 4600 }, playbackMode: true,
      ...(new URLSearchParams(location.search).has('noop_forward') ? {
        playbackCursorUnix: 1000,
        onSeekPlayback() { return false; },
        onForwardPlayback() { qa.forwardNoops = (qa.forwardNoops || 0) + 1; return false; },
      } : {}),
      onRetry() {}, onPlaybackTime() { qa.callbackRevision = value; },
      onStatus() {}, onPlaybackStateChange() {}, onPlaybackRateError() {},
    }));
  };
  qa.rerenderCallbacks();
} else {
  root.render(React.createElement(NVRLabCamera, { externalOrgId: 'synthetic', cameraId: 1,
    auth: null, loggingOut: false, authMessage: '合成回放源 · 本地回归预览 · 非真实设备',
    onLogout: noop, onAuthRequired: noop, onBack: noop }));
}
`;

export async function startPlaybackPreview({ port = 5194, host = "127.0.0.1", viteOrigin = process.env.QA_VITE_ORIGIN || "http://127.0.0.1:5193" } = {}) {
  // Playwright already ships ws; reuse it without changing the project's dependencies/lockfile.
  const require = createRequire(import.meta.url);
  const playwrightEntry = require.resolve(process.env.PLAYWRIGHT_MODULE || "playwright");
  const coreRoot = resolve(dirname(playwrightEntry), "../playwright-core");
  const { wsServer } = require(resolve(coreRoot, "lib/utilsBundle.js"));
  const wss = new wsServer({ noServer: true });
  const connections = new Map();
  const fixtures = new Map();
  const fixture = (id) => {
    if (!fixtures.has(id)) fixtures.set(id, { sessions: [], commands: [], rejectedCommands: [], sockets: [], version: null, delayNextSession: 0,
      rejectNextSpeed: false, ignoreNextSpeed: false, failNextSession: false, holdFrames: false, next: 0,
      timeSource: 'container', containerClockRate: null, recordedClockRate: null, unsequencedACK: false,
      firstFrameOffsetSeconds: 0,
      recordedPrecision: 'millisecond', recordedEpochMs: Date.UTC(2026, 10, 16, 2) });
    return fixtures.get(id);
  };
  const json = (res, value, status = 200) => { res.writeHead(status, { "Content-Type": "application/json", "Cache-Control": "no-store" }); res.end(JSON.stringify(value)); };
  const readBody = async (req) => { let text = ""; for await (const part of req) text += part; return JSON.parse(text || "{}"); };
  const timers = new Set();
  const server = createServer(async (req, res) => {
    try {
      const url = new URL(req.url, "http://localhost");
      const f = fixture(url.searchParams.get("fixture") || req.headers["x-playback-fixture"] || "manual");
      if (url.pathname === `${base}__fixture/bootstrap.mjs`) {
        res.writeHead(200, { "Content-Type": "text/javascript; charset=utf-8", "Cache-Control": "no-store" });
        return res.end(bootstrap);
      }
      if (url.pathname === `${base}__fixture/state`) {
        return json(res, { ...f, sockets: f.sockets.map(s => ({ id: s.id, closed: s.closed, rate: s.rate,
          pts: s.pts, sourcePTS: s.pts, containerPTS: s.containerPTS, recordedAt: s.recordedAt, frames: s.frames })) });
      }
      if (url.pathname === `${base}__fixture/control` && req.method === "POST") {
        const control = await readBody(req);
        if ('timeSource' in control && !['container', 'recorded'].includes(control.timeSource)) return json(res, { error: 'Invalid fixture timeSource' }, 400);
        if ('recordedPrecision' in control && !['millisecond', 'second'].includes(control.recordedPrecision)) return json(res, { error: 'Invalid recorded precision' }, 400);
        if ('firstFrameOffsetSeconds' in control && !Number.isFinite(control.firstFrameOffsetSeconds)) return json(res, { error: 'Invalid first frame offset' }, 400);
        if ('containerClockRate' in control && control.containerClockRate !== null
          && (!Number.isFinite(control.containerClockRate) || control.containerClockRate <= 0)) return json(res, { error: 'Invalid container clock rate' }, 400);
        if ('recordedClockRate' in control && control.recordedClockRate !== null
          && (!Number.isFinite(control.recordedClockRate) || control.recordedClockRate <= 0)) return json(res, { error: 'Invalid recorded clock rate' }, 400);
        for (const key of ["delayNextSession", "rejectNextSpeed", "ignoreNextSpeed", "holdFrames", "failNextSession", "timeSource", "containerClockRate", "recordedClockRate", "recordedPrecision", "unsequencedACK", "firstFrameOffsetSeconds"]) if (key in control) f[key] = control[key];
        if ("version" in control) {
          f.version = control.version;
          for (const s of f.sockets) {
            const connection = connections.get(s);
            if (connection?.readyState === 1) connection.send(JSON.stringify({ version: f.version }));
          }
        }
        return json(res, { ok: true });
      }
      if (url.pathname.includes("/api/")) {
        if (url.pathname.endsWith("/cameras")) return json(res, { external_org_id: "synthetic", tenant_id: 0,
          store_name: "合成预览", cameras: [{ id: 1, name: "合成回放源", space_name: "本地预览", space_type: "other" }] });
        if (url.pathname.endsWith("/stream-session")) {
          const body = await readBody(req);
          const id = String(++f.next);
          const record = { id, ...body, requestedAt: Date.now(), respondedAt: null };
          f.sessions.push(record);
          if (f.failNextSession) { f.failNextSession = false; record.respondedAt = Date.now(); record.failed = true; return json(res, { error: "合成会话暂时失败", code: "FIXTURE_FAILURE" }, 503); }
          const delay = f.delayNextSession; f.delayNextSession = 0;
          if (delay > 0) await new Promise(r => { const t = setTimeout(() => { timers.delete(t); r(); }, delay); timers.add(t); });
          record.respondedAt = Date.now();
          const source = url.searchParams.get("fixture") || req.headers["x-playback-fixture"] || "manual";
          return json(res, { mode: body.mode, url: `ws://${req.headers.host}${base}__fixture/stream?fixture=${encodeURIComponent(source)}&id=${id}` });
        }
        if (url.pathname.endsWith("/screenshot-metadata")) return json(res, { watermark_enabled: false });
        // Never proxy an unhandled API to a real backend.
        return json(res, { error: "Unimplemented synthetic API", path: url.pathname }, 404);
      }
      if (url.pathname === previewPath || url.pathname === "/") {
        const html = '<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>合成回放源 | 本地预览</title><link rel="icon" href="data:,"></head><body><div id="root"></div><script type="module">import RefreshRuntime from "/erzhuang-project/@react-refresh"; RefreshRuntime.injectIntoGlobalHook(window); window.$RefreshReg$ = () => {}; window.$RefreshSig$ = () => type => type; window.__vite_plugin_react_preamble_installed__ = true; await import("/erzhuang-project/__fixture/bootstrap.mjs");</script></body></html>';
        res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" }); return res.end(html);
      }
      const upstream = proxyRequest(new URL(req.url, viteOrigin), { method: req.method, headers: { ...req.headers, host: new URL(viteOrigin).host } }, response => {
        res.writeHead(response.statusCode, response.headers); response.pipe(res);
      });
      upstream.on("error", error => json(res, { error: `Vite unavailable at ${viteOrigin}: ${error.message}` }, 502));
      req.pipe(upstream);
    } catch (error) { json(res, { error: error.message }, 500); }
  });
  server.on("upgrade", (req, socket, head) => {
    const url = new URL(req.url, "http://localhost");
    if (url.pathname !== `${base}__fixture/stream`) {
      // Accept Vite's heartbeat but isolate tests/preview state from concurrent HMR edits.
      // Reload the page to inspect the latest source; no second Vite is started.
      if (url.pathname === base && req.headers["sec-websocket-protocol"] === "vite-hmr") {
        wss.handleUpgrade(req, socket, head, () => {});
      } else socket.destroy();
      return;
    }
    wss.handleUpgrade(req, socket, head, (connection) => {
      const f = fixture(url.searchParams.get("fixture") || "manual");
      const s = { id: url.searchParams.get("id"), rate: 1, pts: 0, containerPTS: 0, recordedPTS: 0, recordedAt: null, frames: 0, closed: false };
      const session = f.sessions.find(session => session.id === s.id);
      const recordedEpochMs = (Number.isFinite(session?.start_time) ? session.start_time * 1000 : f.recordedEpochMs)
        + f.firstFrameOffsetSeconds * 1000;
      f.sockets.push(s);
      connections.set(s, connection);
      if (f.version !== null) connection.send(JSON.stringify({ version: f.version }));
      let last = performance.now();
      connection.on("message", (buffer, binary) => {
        const reject = (format) => {
          f.rejectedCommands.push({ id: s.id, format, bytes: buffer.length, at: Date.now() });
          connection.send(JSON.stringify({ status: 400, statusString: "JSON control command required" }));
        };
        // The SDK contract is JSON for every server version, including 0.1.
        if (binary) return reject("binary");
        let message;
        try { message = JSON.parse(buffer.toString()); } catch { return reject("invalid-json"); }
        if (!message || typeof message !== "object" || Array.isArray(message)
          || !Number.isSafeInteger(message.sequence) || message.sequence < 0
          || typeof message.cmd !== "string"
          || (message.cmd === "speed" && ![1, 2, 4, 8, 16].includes(message.rate))) return reject("invalid-command");
        f.commands.push({ ...message, format: "json", id: s.id, at: Date.now() });
        if (message.cmd !== "speed") return;
        if (f.rejectNextSpeed) { f.rejectNextSpeed = false; connection.send(JSON.stringify({ cmd: "speed", status: 500, sequence: message.sequence })); return; }
        if (f.ignoreNextSpeed) f.ignoreNextSpeed = false;
        else s.rate = message.rate;
        connection.send(JSON.stringify({ cmd: "speed", status: 200, ...(f.unsequencedACK ? {} : { sequence: message.sequence }) }));
      });
      const timer = setInterval(() => {
        const now = performance.now();
        if (!f.holdFrames) {
          s.pts += Math.round((now - last) * 1000 * s.rate);
          s.containerPTS += Math.round((now - last) * 1000 * (f.containerClockRate ?? s.rate));
          s.recordedPTS += Math.round((now - last) * 1000 * (f.recordedClockRate ?? s.rate));
          const precisionMs = f.recordedPrecision === 'second' ? 1000 : 1;
          s.recordedAt = recordedEpochMs + Math.floor(s.recordedPTS / (1000 * precisionMs)) * precisionMs;
          connection.send(Buffer.from(JSON.stringify({ type: "frame", pts: s.pts, containerPTS: s.containerPTS,
            recordedAt: s.recordedAt, timeSource: f.timeSource, rate: s.rate, frame: s.frames + 1 })));
          s.frames++;
        }
        last = now;
      }, 40);
      connection.on("close", () => { s.closed = true; connections.delete(s); clearInterval(timer); });
    });
  });
  await new Promise((resolve, reject) => { server.once("error", reject); server.listen(port, host, resolve); });
  const origin = `http://${host}:${server.address().port}`;
  return { origin, url: `${origin}${previewPath}?start_at=2026-11-16T10:00`, async close() {
    for (const timer of timers) clearTimeout(timer);
    for (const client of wss.clients) client.terminate();
    await new Promise(r => wss.close(r));
    server.closeAllConnections();
    await new Promise(r => server.close(r));
  } };
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const preview = await startPlaybackPreview({ port: Number(process.env.PORT || 5194) });
  console.log(`Synthetic media only; no real device/backend: ${preview.url}`);
  for (const signal of ["SIGINT", "SIGTERM"]) process.once(signal, async () => { await preview.close(); process.exit(0); });
}
