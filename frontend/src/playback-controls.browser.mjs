// Page-level regression against the actual React controls and SDK, using synthetic decoded frames.
import assert from "node:assert/strict";
import { mkdir, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { randomUUID } from "node:crypto";
import { startPlaybackPreview, base, previewPath } from "../../scripts/playback-controls-preview.mjs";

const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const runId = `${new Date().toISOString().replace(/[:.]/g, "-")}-${randomUUID().slice(0, 6)}`;
const artifacts = new URL(`../../output/playwright/playback-controls/${runId}/`, import.meta.url);
await mkdir(artifacts, { recursive: true });
const server = process.env.QA_LOCAL_ORIGIN ? null : await startPlaybackPreview({ port: 0 });
const origin = process.env.QA_LOCAL_ORIGIN || server.origin;
let browser;
try { browser = await chromium.launch({ channel: "chrome", headless: process.env.HEADED !== "1" }); }
catch (error) { await server?.close(); throw error; }
const results = [];
const pauseMs = 30000;
let activeContext;

async function open(viewport = { width: 1440, height: 900 }, component = false, { expectRender = true, noopForward = false } = {}) {
  const id = randomUUID();
  const context = activeContext = await browser.newContext({ viewport, extraHTTPHeaders: { "x-playback-fixture": id } });
  const page = await context.newPage();
  page.setDefaultTimeout(12000);
  const errors = [];
  page.on("pageerror", e => { errors.push(e.message); console.error(`PAGE: ${e.message}`); });
  page.on("console", message => { if (message.type() === "error") console.error(`CONSOLE: ${message.text()}`); });
  const blocked = [];
  await context.route("**/*", route => {
    const url = new URL(route.request().url());
    if (url.origin !== origin) { blocked.push(url.href); return route.abort(); }
    return route.continue();
  });
  await context.routeWebSocket(url => url.origin !== origin.replace("http", "ws"), socket => socket.close());
  await page.goto(`${origin}${previewPath}?fixture=${id}&start_at=2026-11-16T10:00${component ? "&component=1" : ""}${noopForward ? "&noop_forward=1" : ""}`);
  await page.waitForFunction(expected => window.__playbackQA?.snapshot().some(p => p.running && (!expected || p.renders > 2)), expectRender);
  const api = async (action, data) => {
    const response = await context.request.fetch(`${origin}${base}__fixture/${action}?fixture=${id}`, data ? { method: "POST", data } : {});
    assert(response.ok(), `fixture ${action}: ${response.status()}`);
    return response.json();
  };
  return { page, api, async close() { assert.deepEqual(errors, [], "page errors"); assert.deepEqual(blocked, [], "external HTTP requests"); await context.close(); activeContext = null; } };
}
const active = page => page.evaluate(() => window.__playbackQA.snapshot().findLast(p => p.running));
const cursor = page => page.getByRole("slider", { name: "录像进度" }).evaluate((element) => {
  const input = element;
  const value = Number(input.value);
  const minimum = Number(input.min);
  const maximum = Number(input.max);
  return maximum > 100000 ? value - minimum : value;
});
const button = (page, name) => page.getByRole("button", { name, exact: true });
async function playback(page) {
  await button(page, "录像").click();
  await button(page, "10:00 - 11:00").click();
  await button(page, "暂停").waitFor();
  await page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.time > 0.2);
}
async function selectRate(page, rate) {
  const started = Date.now();
  await button(page, "选择倍速").click();
  await button(page, `${rate}倍速`).click();
  // The SDK requires 500ms settle plus two 1000ms PTS windows; never fake its clock.
  await button(page, "恢复正常速度").waitFor();
  await page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.pendingRate === null);
  assert.equal((await active(page)).mutedBySpeed, true);
  assert.equal((await active(page)).volume, 0);
  assert.equal(await button(page, "恢复正常速度").innerText(), `${rate}×`);
  await page.waitForTimeout(Math.max(0, 3000 - (Date.now() - started)));
}
async function normalRate(page) {
  await button(page, "恢复正常速度").click();
  await button(page, "选择倍速").waitFor();
  await page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.pendingRate === null);
}
async function assertPresentedRate(page, rate) {
  const sample = () => page.evaluate(() => {
    const player = window.__playbackQA.players.findLast(p => p._isRunning);
    const slider = document.querySelector('input[aria-label="录像进度"]');
    const value = Number(slider.value);
    const pixels = player.ctx.getImageData(0, 0, player.canvas.width, player.canvas.height).data;
    let opaque = 0, varied = 0;
    for (let i = 0; i < pixels.length; i += 160) {
      if (pixels[i + 3]) opaque++;
      if (pixels[i] !== pixels[0]) varied++;
    }
    return { at: performance.now(), time: player.getPlaybackTime(), pts: player._playbackLastPTS,
      cursor: Number(slider.max) > 100000 ? value - Number(slider.min) : value,
      renders: player.diagnostics.renderedFrames, opaque, varied };
  });
  const before = await sample();
  await page.waitForTimeout(2000);
  const after = await sample();
  const elapsed = (after.at - before.at) / 1000;
  const mediaDelta = after.time - before.time;
  const cursorDelta = after.cursor - before.cursor;
  assert(after.renders > before.renders + 4, `${rate}x: frames must actually render`);
  assert(after.opaque > 100 && after.varied > 100, `${rate}x: canvas must contain visible media`);
  assert(Math.abs((after.pts - before.pts) / 1e6 - mediaDelta) < 0.001, `${rate}x: media time must follow presented PTS`);
  assert(Math.abs(mediaDelta / elapsed - rate) <= rate * 0.2, `${rate}x: rendered media measured ${mediaDelta / elapsed}x`);
  assert(Math.abs(cursorDelta - mediaDelta) < 1.2, `${rate}x: slider must follow presented media`);
  assert(Math.abs(cursorDelta / elapsed - rate) <= rate * 0.2 + 1.1 / elapsed, `${rate}x: slider measured ${cursorDelta / elapsed}x`);
  return { rate, before, after, mediaRate: mediaDelta / elapsed, sliderRate: cursorDelta / elapsed };
}
async function seek(page, offset) {
  const slider = page.getByRole("slider", { name: "录像进度" });
  await slider.focus();
  await slider.fill(String(offset));
  await slider.press("Enter");
  await slider.dispatchEvent("pointerup");
  await slider.blur();
}
async function pausePresented(h) {
  const state = await h.api("state");
  const sessions = state.sessions.filter(session => session.mode === "playback");
  assert(sessions.length > 0, "pause requires a signed playback session");
  await button(h.page, "暂停").click();
  const elapsed = await h.page.evaluate(() => window.__playbackQA.events.findLast(event =>
    event.type === "stop" && typeof event.mediaTime === "number")?.mediaTime);
  assert(Number.isFinite(elapsed), "pause must capture a presented frame timestamp");
  const position = Math.floor(sessions.at(-1).start_time + elapsed - sessions[0].start_time);
  await button(h.page, "播放").waitFor();
  // Wait for the final presented PTS to reach the controlled slider before checking its freeze.
  await h.page.waitForFunction(expected => Number(document.querySelector('input[aria-label="录像进度"]')?.value) === expected, position);
  return position;
}
async function check(name, run) {
  if (process.env.TEST_FILTER && !new RegExp(process.env.TEST_FILTER).test(name)) return;
  console.log(`RUN ${name}`);
  const started = Date.now();
  try { await run(); results.push({ name, status: "passed", ms: Date.now() - started }); console.log(`PASS ${name}`); }
  catch (error) {
    results.push({ name, status: "failed", error: error.stack, ms: Date.now() - started });
    console.error(`FAIL ${name}: ${error.message}`);
    const pages = activeContext?.pages();
    if (pages?.[0]) await pages[0].screenshot({ path: new URL(`failure-${results.length}.png`, artifacts).pathname }).catch(() => {});
    if (pages?.[0]) await saveSnapshot(pages[0], `failure-${results.length}`).catch(() => {});
  } finally { if (activeContext) { await activeContext.close(); activeContext = null; } }
}

async function saveSnapshot(page, name) {
  const snapshot = await page.evaluate(() => ({ players: window.__playbackQA.snapshot(), events: window.__playbackQA.events,
    controls: [...document.querySelectorAll('.h5-player-controls button')].map(button => ({
      label: button.ariaLabel, disabled: button.disabled, pressed: button.getAttribute('aria-pressed'),
    })),
  }));
  await writeFile(new URL(`${name}.json`, artifacts), JSON.stringify(snapshot, null, 2));
  return snapshot;
}

async function bounds(page, name) {
  await page.waitForTimeout(120);
  await page.evaluate(() => document.querySelector('.h5-viewer-player')?.scrollIntoView({ block: 'center', inline: 'nearest' }));
  const geometry = await page.evaluate(() => {
    const rect = e => { const r = e.getBoundingClientRect(); return { x: r.x, y: r.y, width: r.width, height: r.height, right: r.right, bottom: r.bottom }; };
    const visible = e => e.getClientRects().length && getComputedStyle(e).visibility !== 'hidden';
    const buttons = [...document.querySelectorAll('.h5-player-controls button')].filter(visible).map(e => ({ name: e.ariaLabel, ...rect(e) }));
    const slider = document.querySelector('input[type="range"]');
    const sheet = document.querySelector('[role="dialog"]');
    const canvas = document.querySelector('canvas');
    const pixels = canvas.getContext('2d').getImageData(0, 0, canvas.width, canvas.height).data;
    let opaque = 0, varied = 0;
    for (let i = 0; i < pixels.length; i += 160) { if (pixels[i + 3]) opaque++; if (pixels[i] !== pixels[0]) varied++; }
    return { viewport: { width: innerWidth, height: innerHeight }, overflow: document.documentElement.scrollWidth > innerWidth,
      buttons, slider: slider ? rect(slider) : null, sheet: sheet ? rect(sheet) : null, opaque, varied };
  });
  const { width, height } = geometry.viewport;
  await page.screenshot({ path: new URL(`${name}.png`, artifacts).pathname });
  await writeFile(new URL(`${name}.bounds.json`, artifacts), JSON.stringify(geometry, null, 2));
  const inside = (r, label) => {
    assert(r.width > 0 && r.height > 0, `${label}: empty bounds`);
    assert(r.x >= -1 && r.y >= -1 && r.right <= width + 1 && r.bottom <= height + 1, `${name} ${label}: outside viewport ${JSON.stringify(r)}`);
  };
  assert.equal(geometry.overflow, false, `${name}: horizontal page overflow`);
  assert(geometry.opaque > 100 && geometry.varied > 100, `${name}: canvas must contain rendered visual content`);
  for (const b of geometry.buttons) inside(b, b.name);
  if (geometry.slider) inside(geometry.slider, "slider");
  if (geometry.sheet) inside(geometry.sheet, "sheet");
  const rects = [...geometry.buttons, ...(geometry.slider ? [{ name: "slider", ...geometry.slider }] : [])];
  for (let i = 0; i < rects.length; i++) for (let j = i + 1; j < rects.length; j++) {
    const a = rects[i], b = rects[j];
    const overlap = Math.min(a.right, b.right) - Math.max(a.x, b.x) > 1 && Math.min(a.bottom, b.bottom) - Math.max(a.y, b.y) > 1;
    assert(!overlap, `${name}: ${a.name} overlaps ${b.name}`);
  }
}

try {
  await check("live hides playback actions", async () => {
    const h = await open({ width: 1440, height: 900 }, false, { expectRender: false });
    assert.equal(await button(h.page, "选择倍速").count(), 0);
    assert.equal(await button(h.page, "快进10秒").count(), 0);
    await h.close();
  });
  await check("four real SDK rates / return 1x / sound / cancel", async () => {
    const h = await open(); const { page } = h;
    await playback(page);
    await button(page, "开启声音").click();
    await button(page, "关闭声音").waitFor();
    assert.equal((await active(page)).audioState, "running");
    for (const rate of [2, 4, 8, 16]) {
      const sessionCount = (await h.api("state")).sessions.length;
      await selectRate(page, rate);
      assert.equal((await h.api("state")).sockets.at(-1).rate, rate);
      assert.equal((await h.api("state")).sessions.length, sessionCount, "rate switch must reuse session");
      await normalRate(page);
      assert.equal((await active(page)).volume, 80);
      assert.equal((await active(page)).mutedBySpeed, false);
      await button(page, "关闭声音").waitFor();
      await button(page, "选择倍速").click();
      for (const r of [2, 4, 8, 16]) await button(page, `${r}倍速`).waitFor();
      await button(page, "取消").click();
      assert.equal(await page.getByRole("dialog").count(), 0);
    }
    await h.close();
  });
  for (const version of ["0.2", "0.1"]) for (const mobile of [false, true]) {
    await check(`JSON speed contract after version=${version} metadata / ${mobile ? "mobile" : "desktop"}`, async () => {
      const h = await open(); const { page } = h;
      await playback(page);
      // The phone view does not auto-start a session; resize only after actual playback.
      if (mobile) await page.setViewportSize({ width: 390, height: 844 });
      const before = await h.api("state");
      const player = await active(page);
      const initialSequence = await page.evaluate(() => {
        const sdk = window.__playbackQA.players.findLast(p => p._isRunning);
        window.__playbackQA.versionMetadata = [];
        sdk.ws.addEventListener("message", event => {
          if (typeof event.data !== "string") return;
          const message = JSON.parse(event.data);
          if (Object.hasOwn(message, "version")) window.__playbackQA.versionMetadata.push(message.version);
        });
        return sdk.sequence;
      });
      assert.equal(initialSequence, 0, "new playback starts with JSON command sequence zero");
      assert(player.time > 0.2 && player.renders > 2, "version metadata must arrive after media is presented");
      await h.api("control", { version });
      await page.waitForFunction(expected => window.__playbackQA.versionMetadata.includes(expected), version);

      const observations = [];
      for (const rate of [4, 1]) {
        if (rate === 4) await selectRate(page, rate);
        else await normalRate(page);
        const state = await h.api("state");
        assert.deepEqual(state.rejectedCommands, [], "fixture must never receive a binary or malformed control command");
        assert.equal(state.sessions.length, before.sessions.length, "rate changes must reuse the signed session");
        assert.equal(state.sockets.length, before.sockets.length, "rate changes must reuse the WebSocket");
        assert.equal(state.sockets.at(-1).closed, false);
        assert.equal(state.sockets.at(-1).rate, rate);
        const commands = state.commands.slice(before.commands.length);
        assert.equal(commands.length, rate === 4 ? 1 : 2, "each rate selection must send exactly one command");
        assert.deepEqual(commands.map(({ id, at, format, ...message }) => {
          assert.equal(id, before.sockets.at(-1).id, "commands must use the original playback socket");
          assert.equal(format, "json", "the actual WebSocket frame must be JSON text");
          assert(Number.isFinite(at));
          return message;
        }), [4, 1].slice(0, commands.length).map((rate, index) => ({ sequence: initialSequence + index, cmd: "speed", rate })));
        const sdk = await active(page);
        assert.equal(sdk.id, player.id, "rate changes must reuse the SDK player");
        assert.equal(sdk.url, player.url);
        assert.equal(sdk.stopCount, player.stopCount, "metadata and rate changes must not restart playback");
        assert.equal(sdk.mutedBySpeed, rate !== 1);
        const diagnostics = await page.evaluate(() => window.__playbackQA.players.findLast(p => p._isRunning).getPlaybackControlDiagnostics());
        assert.equal(diagnostics.protocol, "json");
        assert.equal(diagnostics.phase, "verified");
        assert.equal(diagnostics.requestedRate, rate);
        assert.equal(diagnostics.verifiedRate, rate);
        assert.equal(diagnostics.commandSequence, commands.at(-1).sequence);
        assert.equal(diagnostics.ack, "accepted");
        assert.equal(diagnostics.failure, null);
        observations.push(await assertPresentedRate(page, rate));
      }
      const after = await h.api("state");
      assert.equal(after.sessions.length, before.sessions.length, "media sampling must not hide delayed session recovery");
      assert.equal(after.sockets.length, before.sockets.length);
      assert.deepEqual(after.rejectedCommands, []);
      assert.equal(await page.locator(".h5-playback-control-failure").count(), 0);
      await writeFile(new URL(`json-version-${version}-${mobile ? "mobile" : "desktop"}.json`, artifacts), JSON.stringify({ version, observations }, null, 2));
      await h.close();
    });
  }
  await check("audio intent survives normal pause/resume seek forward and accelerated resume", async () => {
    const h = await open(); const { page } = h;
    const observations = [];
    const recordAudio = async (phase) => {
      const sdk = await active(page);
      observations.push({ phase, sdk });
      await writeFile(new URL("audio-intent.json", artifacts), JSON.stringify(observations, null, 2));
      return sdk;
    };
    try {
      await playback(page);
      await button(page, "开启声音").click();
      await button(page, "关闭声音").waitFor();
      const assertAudible = async (phase, previousUrl) => {
        const before = await recordAudio(`${phase}: before settling`);
        await page.waitForFunction(url => {
          const player = window.__playbackQA.snapshot().findLast(p => p.running);
          return player && (!url || player.url !== url) && player.time > 0.2
            && player.audioState === "running" && player.volume === 80 && !player.mutedBySpeed;
        }, previousUrl);
        const ready = await active(page);
        // Wait for actual audio render quanta and the SDK's 8ms gain ramp, not a wall-clock guess.
        await page.waitForFunction(({ url, audioTime }) => {
          const sdk = window.__playbackQA.snapshot().findLast(p => p.running);
          return sdk?.url === url && sdk.audioTime >= audioTime + 0.08
            && sdk.gain > 0.7 && sdk.audioPackets > 2 && sdk.audioSignal?.peak > 0.05
            && sdk.audioSignal.rms > 0.02;
        }, { url: ready.url, audioTime: ready.audioTime });
        const sdk = await recordAudio(phase);
        assert.equal(sdk.audioGestureReady, true, `${phase}: real SDK audio must be unlocked`);
        assert(sdk.audioPreparations.some(p => p.gesture), `${phase}: audio preparation must retain a trusted gesture`);
        assert(sdk.gain > 0.7, `${phase}: output gain must restore audio intent`);
        assert(sdk.audioSignal.peak > 0.05 && sdk.audioSignal.rms > 0.02, `${phase}: synthetic PCM must reach the output`);
        assert(sdk.audioTime > (before?.url === sdk.url ? before.audioTime : 0), `${phase}: audio clock must advance`);
        await button(page, "关闭声音").waitFor();
      };
      await assertAudible("initial sound");
      const initialUrl = (await active(page)).url;
      await pausePresented(h);
      await page.waitForFunction(() => !window.__playbackQA.snapshot().some(p => p.running));
      await button(page, "播放").click();
      await assertAudible("normal pause/resume", initialUrl);
      const beforeSeek = (await active(page)).url;
      const countBeforeSeek = (await h.api("state")).sessions.length;
      await seek(page, 180);
      await assertAudible("normal seek", beforeSeek);
      assert.equal((await h.api("state")).sessions.length, countBeforeSeek + 1, "seek pointerup/blur must not rebuild twice");
      const beforeForward = (await active(page)).url;
      await button(page, "快进10秒").click();
      await assertAudible("normal forward", beforeForward);
      await selectRate(page, 4);
      const speedMuted = await recordAudio("4x output muted");
      assert.equal(speedMuted.gain, 0, "4x must mute the actual output gain");
      assert(speedMuted.audioSignal.peak < 0.0001, "4x must silence synthetic PCM output");
      const acceleratedUrl = (await active(page)).url;
      const pausedOffset = await pausePresented(h);
      await page.waitForTimeout(250);
      assert.equal(await cursor(page), pausedOffset);
      await button(page, "播放").click();
      await page.waitForFunction(url => {
        const player = window.__playbackQA.snapshot().findLast(p => p.running);
        return player?.url !== url && player?.time > 0.2 && player.mutedBySpeed && player.pendingRate === null;
      }, acceleratedUrl);
      const accelerated = await recordAudio("accelerated pause/resume");
      assert.equal(accelerated.volume, 0, "accelerated resume must stay muted");
      assert.equal(accelerated.audioState, "running", "prepared audio context must survive accelerated resume");
      assert(accelerated.audioPreparations.some(p => p.gesture), "accelerated resume must prepare audio in trusted gesture");
      assert.equal(accelerated.gain, 0, "accelerated resume must keep output gain zero");
      assert(accelerated.audioSignal.peak < 0.0001, "accelerated resume must keep PCM output silent");
      await button(page, "倍速播放静音").waitFor();
      assert.equal(await button(page, "倍速播放静音").isDisabled(), true);
      await normalRate(page);
      await assertAudible("return 1x restores pre-speed sound intent");
      await writeFile(new URL("audio-intent.json", artifacts), JSON.stringify(observations, null, 2));
    } catch (error) {
      observations.push({ phase: "failure", error: error.stack, players: await page.evaluate(() => window.__playbackQA.snapshot()) });
      await writeFile(new URL("audio-intent.json", artifacts), JSON.stringify(observations, null, 2));
      throw error;
    }
    await h.close();
  });
  for (const action of ["seek", "forward"]) await check(`prepared no-op ${action} closes unused audio context without signing a session`, async () => {
    // The page pauses at the segment end; use an explicit false component callback for forward no-op.
    const h = await open(undefined, action === "forward", { noopForward: action === "forward" }); const { page } = h;
    if (action === "seek") await playback(page);
    await button(page, "开启声音").click();
    await button(page, "关闭声音").waitFor();
    await h.api("control", { holdFrames: true });
    await page.waitForTimeout(120);
    const before = await saveSnapshot(page, `prepared-noop-${action}-before`);
    const sessionCount = (await h.api("state")).sessions.length;
    const target = Math.floor(await cursor(page));
    if (action === "seek") {
      const slider = page.getByRole("slider", { name: "录像进度" });
      await slider.focus();
      // Return to the same integer second after a real edit so onCommit is exercised.
      await slider.fill(String(target + 1));
      await seek(page, target);
    } else await button(page, "快进10秒").click();
    await page.waitForTimeout(250);
    const after = await saveSnapshot(page, `prepared-noop-${action}-after`);
    const unused = after.players.filter(p => !before.players.some(old => old.id === p.id) && !p.hasPlayed);
    assert.equal((await h.api("state")).sessions.length, sessionCount, "no-op must not sign a new session");
    assert.equal((await active(page)).id, before.players.findLast(p => p.running).id, "no-op must keep the active player");
    assert.equal(await cursor(page), target, "no-op must keep rendered position");
    assert.equal(unused.length, 1, "a prepared instance must be observed even without play()");
    assert(unused.every(p => p.stopCount > 0 && !p.running && !p.monitor && !p.socket && !p.audioGestureListener
      && p.audioContexts.length > 0 && p.audioContexts.every(context => context.state === "closed")),
    "unused prepared player must be stopped with closed audio contexts and no monitor or listeners");
    if (action === "forward") assert.equal(await page.evaluate(() => window.__playbackQA.forwardNoops), 1, "forward must exercise the false callback");
    await button(page, "关闭声音").waitFor();
    await h.close();
  });
  await check("prepared unlock rejection mutes active and adopted players consistently", async () => {
    const h = await open(); const { page } = h;
    await playback(page);
    await button(page, "开启声音").click();
    await button(page, "关闭声音").waitFor();
    await page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.audioSignal?.peak > 0.05);
    const old = await active(page);
    await h.api("control", { delayNextSession: 800 });
    await page.evaluate(() => { window.__playbackQA.failNextPreparedUnlock = true; });
    await seek(page, 180);
    await page.waitForFunction(() => window.__playbackQA.snapshot().some(p => p.audioPreparations.some(a => a.injectedFailure && a.state === "rejected")));
    await button(page, "开启声音").waitFor();
    const failed = await saveSnapshot(page, "prepared-unlock-rejected");
    const previous = failed.players.find(p => p.id === old.id);
    if (previous.running) assert.equal(previous.volume, 0, "muted UI must also mute any still-active SDK");
    else assert(!previous.monitor && !previous.socket && !previous.audioGestureListener
      && previous.audioContexts.every(context => context.state === "closed"), "stopped old SDK must release its audio and transport resources");
    assert(failed.players.filter(p => !p.hasPlayed).every(p => p.volume === 0), "unlock failure must mute the pending prepared SDK");
    await page.waitForFunction(url => {
      const p = window.__playbackQA.snapshot().findLast(p => p.running);
      return p?.url !== url && p?.time > 0.2;
    }, old.url);
    const adopted = (await saveSnapshot(page, "prepared-unlock-adopted")).players.findLast(p => p.running);
    assert.equal(adopted.volume, 0, "adoption must retain mutedRef after unlock failure");
    assert.equal(adopted.gain, 0, "muted UI must never retain gain 0.8");
    await button(page, "开启声音").waitFor();
    await button(page, "快进10秒").click();
    await page.waitForFunction(url => {
      const p = window.__playbackQA.snapshot().findLast(p => p.running);
      return p?.url !== url && p?.time > 0.2;
    }, adopted.url);
    assert.equal((await active(page)).volume, 0, "the next session must retain muted intent");
    assert.equal(await page.evaluate(() => window.__playbackQA.snapshot().flatMap(p => p.audioPreparations).filter(a => a.injectedFailure).length), 1);
    await h.close();
  });
  await check("speed sound button is disabled and returns to the original sound intent at 1x", async () => {
    const h = await open(); const { page } = h;
    await playback(page); await button(page, "开启声音").click();
    await button(page, "关闭声音").waitFor();
    await selectRate(page, 4);
    const muted = button(page, "倍速播放静音");
    await muted.waitFor();
    assert.equal(await muted.isDisabled(), true);
    assert.equal(await muted.getAttribute("aria-pressed"), "false");
    await saveSnapshot(page, "speed-sound-disabled");
    await normalRate(page);
    await button(page, "关闭声音").waitFor();
    assert.equal(await button(page, "关闭声音").isEnabled(), true);
    assert.equal((await active(page)).volume, 80);
    await h.close();
  });
  await check("cursor follows presented PTS and freezes without frames", async () => {
    const h = await open(); await playback(h.page);
    await h.api("control", { holdFrames: true });
    await h.page.waitForTimeout(120);
    const before = await cursor(h.page);
    await h.page.waitForTimeout(1200);
    assert.equal(await cursor(h.page), before, "wall clock must not move cursor");
    const state = await h.api("state");
    const sdk = await active(h.page);
    assert(Math.abs(before - sdk.time) < 1.2, "visible cursor must track SDK-rendered PTS");
    assert(state.sockets.at(-1).frames > 0);
    await h.close();
  });
  await check("forward 10 seconds re-signs from rendered position", async () => {
    const h = await open(); await playback(h.page);
    await h.api("control", { holdFrames: true }); await h.page.waitForTimeout(120);
    const before = await active(h.page); const old = (await h.api("state")).sessions.at(-1);
    await button(h.page, "快进10秒").click();
    await h.page.waitForFunction(url => window.__playbackQA.snapshot().some(p => p.running && p.url !== url), before.url);
    const next = (await h.api("state")).sessions.at(-1);
    assert.equal(next.start_time, old.start_time + Math.floor(before.time) + 10);
    assert.equal(next.end_time, old.end_time);
    await h.close();
  });
  await check("pause 30 seconds stops socket; resume re-signs and restores rate", async () => {
    const h = await open(); await playback(h.page); await selectRate(h.page, 4);
    const position = await pausePresented(h);
    const state = await h.api("state");
    const count = state.sessions.length;
    await h.page.waitForFunction(() => !window.__playbackQA.snapshot().some(p => p.running));
    await h.page.waitForTimeout(pauseMs);
    assert.equal(await cursor(h.page), position);
    const paused = await h.api("state");
    assert.equal(paused.sessions.length, count);
    assert(paused.sockets.every(s => s.closed), "pause must close transport");
    await button(h.page, "播放").click();
    await h.page.waitForFunction(() => window.__playbackQA.snapshot().some(p => p.running && p.time > 0));
    const resumed = await h.api("state");
    assert.equal(resumed.sessions.at(-1).start_time, Math.floor(state.sessions.at(-1).start_time + position));
    assert.notEqual(resumed.sessions.at(-1).id, state.sessions.at(-1).id);
    await h.page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.mutedBySpeed && !window.__playbackQA.snapshot().findLast(p => p.running)?.pendingRate);
    assert.equal((await h.api("state")).sockets.at(-1).rate, 4);
    await h.close();
  });
  await check("paused seek / forward / rate do not create session until resume", async () => {
    const h = await open(); await playback(h.page); await pausePresented(h);
    const before = await h.api("state");
    await h.page.waitForFunction(() => !document.querySelector('button[aria-label="快进10秒"]')?.disabled);
    const pausedCursor = await cursor(h.page);
    await button(h.page, "快进10秒").click();
    await h.page.waitForFunction(value => Number(document.querySelector('input[aria-label="录像进度"]')?.value) >= value + 10, pausedCursor);
    assert.equal(await cursor(h.page), pausedCursor + 10);
    await seek(h.page, 120); assert.equal(await cursor(h.page), 120);
    await button(h.page, "选择倍速").click(); await button(h.page, "8倍速").click();
    assert.equal((await h.api("state")).sessions.length, before.sessions.length);
    await button(h.page, "播放").click();
    await h.page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.time > 0);
    assert.equal((await h.api("state")).sessions.at(-1).start_time, before.sessions.at(-1).start_time + 120);
    await h.page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.mutedBySpeed && !window.__playbackQA.snapshot().findLast(p => p.running)?.pendingRate);
    assert.equal((await h.api("state")).sockets.at(-1).rate, 8);
    await h.close();
  });
  await check("late session response cannot replace newer selection", async () => {
    const h = await open(); await button(h.page, "录像").click();
    await h.api("control", { delayNextSession: 900 });
    await button(h.page, "09:00 - 10:00").click();
    await button(h.page, "10:00 - 11:00").click();
    await button(h.page, "暂停").waitFor(); await h.page.waitForTimeout(1100);
    const state = await h.api("state");
    assert.equal(new URL((await active(h.page)).url).searchParams.get("id"), state.sessions.at(-1).id);
    assert(state.sessions.at(-2).respondedAt > state.sessions.at(-1).respondedAt);
    await h.close();
  });
  await check("seek commits once across pointerup and blur; focus and arrow keep following PTS", async () => {
    const h = await open(); await playback(h.page);
    const slider = h.page.getByRole("slider", { name: "录像进度" });
    await slider.focus(); const before = await cursor(h.page);
    await h.page.waitForTimeout(1400);
    assert(await cursor(h.page) > before, "focus alone must not freeze PTS tracking");
    const count = (await h.api("state")).sessions.length;
    await seek(h.page, 120);
    await h.page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.time > 0.2);
    assert.equal((await h.api("state")).sessions.length, count + 1, "Enter/pointerup/blur must consume one dirty commit");
    await slider.focus(); await slider.press("ArrowRight"); await slider.press("Enter");
    await h.page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.time > 0.2);
    const afterSeek = await cursor(h.page); await h.page.waitForTimeout(1400);
    assert(await cursor(h.page) > afterSeek, "keyboard seek must resume following PTS without blur");
    assert.equal((await h.api("state")).sessions.length, count + 2);
    await h.close();
  });
  await check("resume API failure lets next play immediately request a fresh session", async () => {
    const h = await open(); await playback(h.page); await pausePresented(h);
    const count = (await h.api("state")).sessions.length;
    await h.api("control", { failNextSession: true }); await button(h.page, "播放").click();
    await h.page.getByText("合成会话暂时失败（FIXTURE_FAILURE）", { exact: true }).waitFor();
    await button(h.page, "播放").click();
    await button(h.page, "暂停").waitFor();
    assert.equal((await h.api("state")).sessions.length, count + 2);
    await h.close();
  });
  await check("old pending speed rejection and old rendered frame cannot replace a newer range", async () => {
    const h = await open(); await playback(h.page);
    await h.api("control", { ignoreNextSpeed: true });
    await button(h.page, "选择倍速").click(); await button(h.page, "4倍速").click();
    await h.page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.pendingRate === 4);
    const oldUrl = (await active(h.page)).url;
    const count = (await h.api("state")).sessions.length;
    await h.api("control", { delayNextSession: 500 });
    await button(h.page, "11:00 - 12:00").click();
    // A decoder output arriving after stop must go through the actual SDK render guard.
    await h.page.evaluate(url => {
      const old = window.__playbackQA.players.find(p => p.wsUrl === url);
      const canvas = document.createElement("canvas"); canvas.width = 64; canvas.height = 36;
      old._renderFrame(new VideoFrame(canvas, { timestamp: 3500e6 }));
    }, oldUrl);
    await h.page.waitForFunction(url => window.__playbackQA.snapshot().findLast(p => p.running)?.url !== url && window.__playbackQA.snapshot().findLast(p => p.running)?.time > 0.2, oldUrl);
    await h.page.waitForTimeout(3200);
    const state = await h.api("state");
    assert.equal(state.sessions.length, count + 1, "old speed cancellation must not re-sign current range");
    assert.equal(state.sessions.at(-1).start_time, Math.floor(new Date("2026-11-16T11:00:00").getTime() / 1000));
    assert(await cursor(h.page) < 120, "old PTS must not leak into current slider");
    await h.close();
  });
  for (const failure of ["rejectNextSpeed", "ignoreNextSpeed"]) await check(`${failure}: recover with fresh normal session`, async () => {
    const h = await open(); await playback(h.page); const count = (await h.api("state")).sessions.length;
    await h.page.setViewportSize({ width: 390, height: 844 });
    await button(h.page, "全屏").click();
    await h.page.evaluate(() => Object.defineProperty(navigator, "clipboard", { configurable: true,
      value: { writeText: async report => { window.__playbackReport = report; } } }));
    await h.api("control", { [failure]: true });
    await button(h.page, "选择倍速").click(); await button(h.page, "4倍速").click();
    await h.page.getByText("未能确认录像倍速生效，已重新连接正常速度回放", { exact: true }).waitFor();
    await button(h.page, "选择倍速").waitFor();
    await h.page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.time > 0);
    assert.equal((await h.api("state")).sessions.length, count + 1);
    assert.equal((await h.api("state")).sockets.at(-1).rate, 1);
    const notice = h.page.locator(".h5-player-shell .h5-playback-control-failure");
    await notice.getByText("未能确认4倍速生效，请继续使用正常速度回放", { exact: true }).waitFor();
    await button(h.page, "复制倍速诊断信息").click();
    await notice.getByText("诊断信息已复制", { exact: true }).waitFor();
    const reportText = await h.page.evaluate(() => window.__playbackReport);
    const report = JSON.parse(reportText);
    assert.equal(report.phase, "failed");
    assert.equal(report.requestedRate, 4);
    assert.equal(report.verifiedRate, 1);
    assert.equal(report.failure, failure === "rejectNextSpeed" ? "rejected" : "timeout");
    assert.equal(report.ack, failure === "rejectNextSpeed" ? "rejected" : "accepted");
    if (failure === "ignoreNextSpeed") assert(report.observedRate > 0.75 && report.observedRate < 1.25,
      `an ACK with ignored speed must preserve measured 1x, got ${report.observedRate}`);
    assert(!/wss?:|token|camera|fixture|private/.test(reportText), "report must exclude stream/user identifiers");
    const noticeBounds = await notice.boundingBox();
    assert(noticeBounds.x >= 0 && noticeBounds.y >= 0 && noticeBounds.x + noticeBounds.width <= 391
      && noticeBounds.y + noticeBounds.height <= 845, "fullscreen failure notice must fit the phone viewport");
    await h.page.screenshot({ path: new URL(`${failure}-notice.png`, artifacts).pathname });
    if (failure === "ignoreNextSpeed") {
      await h.page.evaluate(() => Object.defineProperty(navigator, "clipboard", { configurable: true,
        value: { writeText: async () => { throw new Error("Clipboard denied"); } } }));
      await button(h.page, "复制倍速诊断信息").click();
      const fallback = notice.getByRole("textbox", { name: "倍速诊断信息" });
      await fallback.waitFor();
      assert.deepEqual(JSON.parse(await fallback.inputValue()), report, "manual fallback must preserve the same safe report");
    }
    await h.close();
  });
  await check("new callback identities do not rebuild SDK player", async () => {
    const h = await open(undefined, true);
    const before = await active(h.page);
    await h.page.evaluate(() => window.__playbackQA.rerenderCallbacks());
    await h.page.waitForFunction(() => window.__playbackQA.callbackRevision === 2);
    const after = await active(h.page);
    assert.equal(after.url, before.url);
    assert.equal(await h.page.evaluate(() => window.__playbackQA.players.length), 1);
    assert.equal((await h.api("state")).sockets.length, 1);
    await h.close();
  });
  for (const [width, height] of [[1440, 900], [390, 844], [320, 740], [844, 390]]) await check(`layout ${width}x${height} / panel / landscape / fullscreen`, async () => {
    const h = await open({ width, height }); const { page } = h;
    await playback(page); await bounds(page, `${width}-normal`);
    await button(page, "选择倍速").click(); await bounds(page, `${width}-panel`);
    await button(page, "取消").click();
    await button(page, "横屏查看").click(); await bounds(page, `${width}-landscape`);
    await button(page, "选择倍速").click(); await bounds(page, `${width}-landscape-panel`);
    await button(page, "取消").click(); await button(page, "退出横屏").click();
    await button(page, "全屏").click();
    await page.waitForFunction(() => Boolean(document.fullscreenElement));
    await bounds(page, `${width}-fullscreen`);
    await button(page, "选择倍速").click(); await bounds(page, `${width}-fullscreen-panel`);
    await button(page, "取消").click(); await page.evaluate(() => document.exitFullscreen());
    await h.close();
  });
} finally {
  await browser.close(); await server?.close();
  await writeFile(new URL("results.json", artifacts), JSON.stringify({ source: "synthetic decoded frames; actual React + SDK + local HTTP/WS; no device validation", results }, null, 2));
}
console.log(`${results.filter(r => r.status === "passed").length}/${results.length} passed; artifacts: ${artifacts.pathname}`);
if (results.some(r => r.status === "failed")) process.exitCode = 1;
