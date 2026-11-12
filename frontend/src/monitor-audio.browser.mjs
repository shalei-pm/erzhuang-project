// Local component regression with synthetic audio; no company API is accessed.
import assert from "node:assert/strict";
import { mkdir } from "node:fs/promises";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const origin = process.env.QA_LOCAL_ORIGIN || "http://127.0.0.1:5193";
const browser = await chromium.launch({ channel: "chrome", headless: true });
const artifactDir = new URL("../../output/playwright/", import.meta.url);
await mkdir(artifactDir, { recursive: true });
let checked = 0;

const harness = `<!doctype html><html><head><meta name="viewport" content="width=device-width, initial-scale=1"></head><body>
<div id="root"></div><script type="module">
import RefreshRuntime from '/erzhuang-project/@react-refresh';
RefreshRuntime.injectIntoGlobalHook(window);
window.$RefreshReg$ = () => {};
window.$RefreshSig$ = () => (type) => type;
window.__vite_plugin_react_preamble_installed__ = true;
const reactModule = await import('/erzhuang-project/node_modules/.vite/deps/react.js');
const React = reactModule.default || reactModule;
const domModule = await import('/erzhuang-project/node_modules/.vite/deps/react-dom_client.js');
const { createRoot } = domModule.default || domModule;
const { default: NVRPlayer } = await import('/erzhuang-project/src/vendor/nvr-player/nvr-player.js');
const { NVRLabPlayer } = await import('/erzhuang-project/src/components/NVRLabPlayer.tsx');
await import('/erzhuang-project/src/styles.css');
NVRPlayer.prototype._createDecoder = function () {};
NVRPlayer.prototype._initWasmDecoder = async function () {};
NVRPlayer.prototype._connect = function () {
  window.__qaPlayer = this;
  this.onConnected();
  this.onFirstFrame();
};
const root = createRoot(document.getElementById('root'));
window.__qaMount = (mode) => root.render(React.createElement(NVRLabPlayer, {
  session: { mode, url: 'wss://unused.invalid/fixture', cameraId: 1 }, onRetry() {},
}));
window.__qaUnmount = () => root.unmount();
window.__qaMount('live');
</script></body></html>`;

try {
  for (const [width, height] of [[390, 844], [1440, 900]]) {
    const context = await browser.newContext({ viewport: { width, height } });
    await context.routeWebSocket((url) => url.hostname !== new URL(origin).hostname, (socket) => socket.close());
    const page = await context.newPage();
    page.setDefaultTimeout(8000);
    const errors = [];
    page.on("pageerror", (error) => { errors.push(error.message); console.error(error.message); });
    page.on("console", (message) => { if (message.type() === "error") errors.push(message.text()); });
    await context.route("**/*", (route) => {
      const url = new URL(route.request().url());
      if (url.origin !== origin || url.pathname.includes("/api/")) return route.abort();
      if (url.pathname.endsWith("/__qa-monitor-audio")) return route.fulfill({ contentType: "text/html", body: harness });
      return route.continue();
    });
    for (const mode of ["live", "playback"]) {
      await page.goto(`${origin}/erzhuang-project/__qa-monitor-audio`);
      await page.waitForFunction(() => typeof window.__qaMount === "function");
      if (mode === "playback") await page.evaluate(() => window.__qaMount("playback"));
      await page.getByRole("button", { name: "开启声音", exact: true }).waitFor();
      await page.locator("canvas").click({ position: { x: 10, y: 10 } });
      assert.equal(await page.evaluate(() => window.__qaPlayer.audioContext), null, "tapping video must not unmute audio");
      await page.locator("canvas").click({ position: { x: 10, y: 10 } });

      await page.getByRole("button", { name: "开启声音", exact: true }).click();
      await page.getByRole("button", { name: "关闭声音", exact: true }).waitFor();
      const signal = await page.evaluate(async (activeMode) => {
        const player = window.__qaPlayer;
        const audio = player.audioContext;
        const analyser = audio.createAnalyser();
        analyser.fftSize = 2048;
        player.gainNode.connect(analyser);
        const decoded = player._decodePCMA(Uint8Array.from({ length: 256 }, (_, i) => i));
        const encodeSample = (sample) => {
          let nearest = 0;
          for (let code = 1; code < 256; code++) {
            if (Math.abs(decoded[code] - sample) < Math.abs(decoded[nearest] - sample)) nearest = code;
          }
          return nearest;
        };
        let sequenceNumber = 0;
        const feed = () => {
          const base = sequenceNumber * 160;
          const packet = Uint8Array.from({ length: 160 }, (_, i) =>
            encodeSample(Math.sin(2 * Math.PI * 440 * (base + i) / 8000) * 18000)
          );
          if (activeMode === "live") player._decodeRTP({ payloadType: 8, sequenceNumber: sequenceNumber++, payload: packet });
          else {
            sequenceNumber++;
            player._handleH264Frame(packet.buffer, { nFrameType: 4 });
          }
        };
        const startAudioTime = audio.currentTime;
        for (let pre = 0; pre < 5; pre++) feed();
        const timer = setInterval(feed, 20);
        const deadline = Date.now() + 1500;
        while (audio.currentTime - startAudioTime < 0.18 && Date.now() < deadline) {
          await new Promise((resolve) => setTimeout(resolve, 20));
        }
        const samples = new Float32Array(2048);
        analyser.getFloatTimeDomainData(samples);
        clearInterval(timer);
        analyser.disconnect();
        let maxStep = 0;
        for (let i = 1; i < samples.length; i++) {
          maxStep = Math.max(maxStep, Math.abs(samples[i] - samples[i - 1]));
        }
        return {
          state: audio.state,
          hasWorklet: Boolean(player.workletNode),
          peak: Math.max(...samples.map(Math.abs)),
          maxStep,
        };
      }, mode);
      assert.equal(signal.state, "running");
      assert.equal(signal.hasWorklet, true, `expected AudioWorklet node to be active for ${mode}`);
      assert(signal.peak > 0.05, `expected an output signal for ${mode}: ${signal.peak}`);
      assert(signal.maxStep < 0.08, `expected smooth waveform without boundary clicks for ${mode}: ${signal.maxStep}`);

      await page.getByRole("button", { name: "关闭声音", exact: true }).click();
      await page.evaluate(() => {
        const audio = window.__qaPlayer.audioContext;
        Object.defineProperty(audio, "state", { value: "interrupted", configurable: true });
        audio.resume = async () => { throw new Error("Synthetic interruption"); };
      });
      await page.getByRole("button", { name: "开启声音", exact: true }).click();
      await page.getByText("声音暂未开启，请再次点击声音按钮").waitFor();
      assert.equal(await page.locator(".h5-player-error").count(), 0, "an audio failure must not block video");
      assert.equal(await page.getByRole("button", { name: "暂停", exact: true }).isEnabled(), true);
      assert.equal(await page.getByRole("button", { name: "开启声音", exact: true }).isEnabled(), true);
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
      await page.screenshot({ path: new URL(`monitor-audio-${width}-${mode}.png`, artifactDir).pathname });
      assert.deepEqual(errors, []);
      await page.evaluate(() => {
        delete window.__qaPlayer.audioContext.state;
        window.__qaUnmount();
      });
      checked++;
    }
    await context.close();
  }
  console.log(`PASS ${checked} original monitor audio browser scenarios (synthetic live/playback, mobile/desktop)`);
} finally {
  await browser.close();
}
