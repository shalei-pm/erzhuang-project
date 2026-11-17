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
const qa = window.__playbackQA = { players: [], events: [], renders: 0 };
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
  ctx.fillText('PTS ' + t.toFixed(3) + ' s  |  ' + packet.rate + 'x', 28, 510);
  this._renderFrame(new VideoFrame(paint, { timestamp: packet.pts }));
  feedSyntheticAudio(this);
  qa.renders++;
};
const originalPlay = NVRPlayer.prototype.play;
NVRPlayer.prototype.play = function (url) {
  registerPlayer(this);
  this.__qaPlayCount = (this.__qaPlayCount || 0) + 1;
  qa.events.push({ type: 'play', url, at: performance.now() });
  return originalPlay.call(this, url);
};
const originalStop = NVRPlayer.prototype.stop;
NVRPlayer.prototype.stop = function () {
  this.__qaStopCount = (this.__qaStopCount || 0) + 1;
  qa.events.push({ type: 'stop', url: this.wsUrl, mediaTime: this.getPlaybackTime(), at: performance.now() });
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
  const fixtures = new Map();
  const fixture = (id) => {
    if (!fixtures.has(id)) fixtures.set(id, { sessions: [], commands: [], sockets: [], delayNextSession: 0,
      rejectNextSpeed: false, ignoreNextSpeed: false, failNextSession: false, holdFrames: false, next: 0 });
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
        return json(res, { ...f, sockets: f.sockets.map(s => ({ id: s.id, closed: s.closed, rate: s.rate, pts: s.pts, frames: s.frames })) });
      }
      if (url.pathname === `${base}__fixture/control` && req.method === "POST") {
        const control = await readBody(req);
        for (const key of ["delayNextSession", "rejectNextSpeed", "ignoreNextSpeed", "holdFrames", "failNextSession"]) if (key in control) f[key] = control[key];
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
      const s = { id: url.searchParams.get("id"), rate: 1, pts: 0, frames: 0, closed: false };
      f.sockets.push(s);
      let last = performance.now();
      connection.on("message", (buffer, binary) => {
        let message;
        if (binary && buffer.length === 36) message = { cmd: "speed", rate: buffer.readUInt32BE(32), binary: true };
        else { try { message = JSON.parse(buffer.toString()); } catch { return; } }
        f.commands.push({ ...message, id: s.id, at: Date.now() });
        if (message.cmd !== "speed") return;
        if (f.rejectNextSpeed) { f.rejectNextSpeed = false; connection.send(JSON.stringify({ cmd: "speed", status: 500, sequence: message.sequence })); return; }
        if (f.ignoreNextSpeed) f.ignoreNextSpeed = false;
        else s.rate = message.rate;
        connection.send(JSON.stringify({ cmd: "speed", status: 200, sequence: message.sequence }));
      });
      const timer = setInterval(() => {
        const now = performance.now();
        if (!f.holdFrames) {
          s.pts += Math.round((now - last) * 1000 * s.rate);
          connection.send(Buffer.from(JSON.stringify({ type: "frame", pts: s.pts, rate: s.rate })));
          s.frames++;
        }
        last = now;
      }, 40);
      connection.on("close", () => { s.closed = true; clearInterval(timer); });
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
