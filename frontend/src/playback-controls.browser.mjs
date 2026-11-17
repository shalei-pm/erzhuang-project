// Page-level regression against the actual React controls and SDK, using synthetic decoded frames.
import assert from "node:assert/strict";
import { mkdir, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { randomUUID } from "node:crypto";
import { pathToFileURL } from "node:url";
import { resolve } from "node:path";
import { startPlaybackPreview, base, previewPath } from "../../scripts/playback-controls-preview.mjs";

const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const runId = `${new Date().toISOString().replace(/[:.]/g, "-")}-${randomUUID().slice(0, 6)}`;
const artifacts = process.env.QA_ARTIFACTS_DIR
  ? pathToFileURL(`${resolve(process.env.QA_ARTIFACTS_DIR, runId)}/`)
  : new URL(`../../output/playwright/playback-controls/${runId}/`, import.meta.url);
const omitRecordedUnix = process.env.QA_OMIT_RECORDED_UNIX === "1";
const recordingUTCOffsetSeconds = 480 * 60;
await mkdir(artifacts, { recursive: true });
const server = process.env.QA_LOCAL_ORIGIN ? null : await startPlaybackPreview({ port: 0 });
const origin = process.env.QA_LOCAL_ORIGIN || server.origin;
let browser;
try { browser = await chromium.launch({ channel: "chrome", headless: process.env.HEADED !== "1" }); }
catch (error) { await server?.close(); throw error; }
const results = [];
const pauseMs = 30000;
let activeContext;

async function open(viewport = { width: 1440, height: 900 }, component = false, { expectRender = true, noopForward = false, fixture = {} } = {}) {
  const id = randomUUID();
  const context = activeContext = await browser.newContext({ viewport, extraHTTPHeaders: { "x-playback-fixture": id } });
  const page = await context.newPage();
  if (omitRecordedUnix) await page.addInitScript(() => { window.__qaOmitRecordedUnix = true; });
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
  if (component) await context.route("**/__fixture/bootstrap.mjs", async route => {
    const response = await route.fetch();
    const source = await response.text();
    const target = "root.render(React.createElement(NVRLabPlayer, { session,";
    assert(source.includes(target), "standalone fixture must mount the actual player component");
    const wrapper = `function ControlledPlayer(props) {
      const [playbackRate, onPlaybackRateChange] = React.useState(1);
      return React.createElement(NVRLabPlayer, { ...props, playbackRate, onPlaybackRateChange });
    }
    `;
    await route.fulfill({ response, body: source.replace("  let revision = 0;", `${wrapper}  let revision = 0;`)
      .replace(target, "root.render(React.createElement(ControlledPlayer, { session,") });
  });
  const api = async (action, data) => {
    const response = await context.request.fetch(`${origin}${base}__fixture/${action}?fixture=${id}`, data ? { method: "POST", data } : {});
    assert(response.ok(), `fixture ${action}: ${response.status()}`);
    return response.json();
  };
  // Select the clock before the first frame; the SDK pins its clock source for a session.
  if (Object.keys(fixture).length) await api("control", fixture);
  await page.goto(`${origin}${previewPath}?fixture=${id}&start_at=2026-11-16T10:00${component ? "&component=1" : ""}${noopForward ? "&noop_forward=1" : ""}`);
  await page.waitForFunction(expected => window.__playbackQA?.snapshot().some(p => p.running && (!expected || p.renders > 2)), expectRender);
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
async function normalRate(h) {
  const { page } = h;
  const previous = await active(page);
  const before = await h.api("state");
  const component = new URL(page.url()).searchParams.has("component");
  await button(page, "恢复正常速度").click();
  await button(page, "选择倍速").waitFor();
  await page.waitForFunction(({ id, component }) => {
    const player = window.__playbackQA.snapshot().findLast(p => p.running);
    return player && (component ? player.id === id : player.id !== id)
      && player.pendingRate === null && player.time > 0.2 && player.renders > 4 && !player.mutedBySpeed;
  }, { id: previous.id, component });
  if (component) {
    const state = await assertSamePlayback(h, before, previous);
    assert.deepEqual(state.commands.slice(before.commands.length).map(command => command.rate), [1]);
    assert.equal(state.sockets.at(-1).rate, 1);
  } else await assertRestoredPlayback(h, before, previous);
}
async function assertSessionClock(h) {
  const sessions = (await h.api("state")).sessions.filter(session => session.mode === "playback");
  const frame = await presentedSample(h.page);
  const anchor = sessions.at(-1).start_time - sessions[0].start_time;
  if (frame.control.timeSource === "recorded") {
    assert.equal(frame.pts, frame.recordedPTS, "SDK must retain raw Beijing calendar PTS");
    assert.equal(frame.recordedPTS / 1e6 - recordingUTCOffsetSeconds, frame.paintedRecordedUnixMs / 1000,
      "raw Beijing calendar minus 480 minutes must equal the clock painted in the presented frame");
    const expected = Math.max(0, Math.min(sessions[0].end_time - sessions[0].start_time - 1,
      frame.paintedRecordedUnixMs / 1000 - sessions[0].start_time));
    assert(Math.abs(frame.cursor - expected) < 1.2,
      `slider must follow actual frame Unix relative to global window: cursor=${frame.cursor}, expected=${expected}`);
    assert.equal(frame.playbackCallback?.recordedUnixSeconds, frame.paintedRecordedUnixMs / 1000,
      "SDK callback must deliver actual Unix rather than raw pseudo-UTC or relative zero");
  } else assert(Math.abs(frame.cursor - (anchor + frame.time)) < 1.2,
    `container fallback must include the signed session anchor: cursor=${frame.cursor}, expected=${anchor + frame.time}`);
}
async function assertOffsetSession(h) {
  await assertSessionClock(h);
  const sessions = (await h.api("state")).sessions.filter(session => session.mode === "playback");
  const frame = await presentedSample(h.page);
  const frameUnix = frame.paintedRecordedUnixMs / 1000;
  assert(Math.abs(frameUnix - sessions.at(-1).start_time - 6 - frame.sourcePTS / 1e6) < 0.002,
    "every newly signed session must retain the six-second source offset");
  assert(frameUnix - sessions.at(-1).start_time - frame.time >= 5.99,
    "first presented frame must not be mistaken for the requested anchor or relative zero");
  return frame;
}
async function assertRestoredPlayback(h, before, previous) {
  const state = await h.api("state");
  const current = await active(h.page);
  assert.equal(state.sessions.length, before.sessions.length + 1, "explicit normal restore must re-sign exactly once");
  assert.equal(state.sockets.length, before.sockets.length + 1, "normal restore must replace the old socket exactly once");
  const oldSocket = state.sockets.find(socket => socket.id === before.sockets.at(-1).id);
  assert.equal(oldSocket.closed, true, "accelerated transport must close on normal restore");
  assert.equal(state.sockets.at(-1).closed, false);
  assert.equal(state.sockets.at(-1).rate, 1);
  assert.deepEqual(state.commands.slice(before.commands.length), [], "normal restore must not send speed1 or another acceleration command");
  assert.notEqual(current.id, previous.id, "normal restore must adopt a new SDK player");
  assert.notEqual(current.url, previous.url);
  assert.equal(new URL(current.url).searchParams.get("id"), state.sessions.at(-1).id);
  const stopped = await h.page.evaluate(id => {
    const old = window.__playbackQA.snapshot().find(player => player.id === id);
    const event = window.__playbackQA.events.findLast(event => event.type === "stop"
      && event.url === old.url && Number.isFinite(event.mediaTime));
    return { old, event };
  }, previous.id);
  assert(stopped.event, "restore must capture the last actually presented timestamp before stopping");
  assert(!stopped.old.running && !stopped.old.socket && !stopped.old.monitor && !stopped.old.audioGestureListener,
    "old SDK must release playback resources");
  assert(stopped.old.audioContexts.every(context => context.state === "closed"), "old audio contexts must close");
  assert.equal(state.sessions.at(-1).start_time,
    Math.floor(stopped.event.recordedUnixSeconds ?? (before.sessions.at(-1).start_time + stopped.event.mediaTime)), "normal session must start at the last presented cursor");
  assert.equal(state.sessions.at(-1).end_time, before.sessions.at(-1).end_time);
  assert.equal(current.control.phase, "idle", "new normal session must not inherit old speed diagnostics");
  assert.equal(current.control.requestedRate, 1);
  assert.equal(current.control.verifiedRate, 1);
  assert.equal(current.control.failure, null);
  assert.equal(current.control.commandSequence, null);
  assert.equal(current.pendingRate, null);
  assert.equal(current.mutedBySpeed, false);
  assert.equal(await h.page.locator(".h5-playback-control-failure").count(), 0);
  assert.deepEqual(await h.page.evaluate(() => window.__playbackQA.unhandledRejections), []);
  await assertSessionClock(h);
  return state;
}
async function requestPendingRate(page, rate) {
  await button(page, "选择倍速").click();
  await button(page, `${rate}倍速`).click();
  await page.waitForFunction(expected => window.__playbackQA.snapshot().findLast(p => p.running)?.pendingRate === expected, rate);
  const restore = button(page, "恢复正常速度");
  assert.equal(await restore.innerText(), `${rate}×`, "verification must display the requested rate immediately");
  assert.equal(await restore.isEnabled(), true, "restore must remain clickable during verification");
}
async function assertSamePlayback(h, before, player) {
  const state = await h.api("state");
  const current = await active(h.page);
  assert.equal(state.sessions.length, before.sessions.length, "speed handling must not re-sign the session");
  assert.equal(state.sockets.length, before.sockets.length, "speed handling must not replace the socket");
  assert.equal(state.sockets.at(-1).closed, false);
  assert.equal(current.id, player.id, "speed handling must retain the React SDK instance");
  assert.equal(current.url, player.url);
  assert.equal(current.stopCount, player.stopCount, "obsolete effects must not stop playback");
  const notice = h.page.locator(".h5-playback-control-failure");
  assert.equal(await notice.count(), current.control.phase === "unconfirmed" ? 1 : 0);
  if (current.control.phase === "unconfirmed") {
    assert.match(await notice.innerText(), /指令已接收，画面速率待确认/);
    assert.doesNotMatch(await notice.innerText(), /未能确认|请继续使用正常速度回放/);
  }
  assert.deepEqual(await h.page.evaluate(() => window.__playbackQA.unhandledRejections), []);
  return state;
}
const presentedSample = page => page.evaluate(() => {
    const player = window.__playbackQA.players.findLast(p => p._isRunning);
    const slider = document.querySelector('input[aria-label="录像进度"]');
    const value = slider ? Number(slider.value) : null;
    const pixels = player.ctx.getImageData(0, 0, player.canvas.width, player.canvas.height).data;
    let opaque = 0, varied = 0;
    for (let i = 0; i < pixels.length; i += 160) {
      if (pixels[i + 3]) opaque++;
      if (pixels[i] !== pixels[0]) varied++;
    }
    // Read the source millisecond stamp from the actual rendered canvas, independently of SDK PTS.
    let paintedSourceMs = 0;
    for (let bit = 0; bit < 32; bit++) {
      const pixel = player.ctx.getImageData(32 + bit * 8, 49, 1, 1).data;
      if (pixel[0] > 127) paintedSourceMs += 2 ** bit;
    }
    let paintedRecordedUnixMs = 0;
    for (let bit = 0; bit < 42; bit++) {
      const pixel = player.ctx.getImageData(32 + bit * 8, 61, 1, 1).data;
      if (pixel[0] > 127) paintedRecordedUnixMs += 2 ** bit;
    }
    return { at: performance.now(), time: player.getPlaybackTime(), pts: player._playbackLastPTS,
      cursor: slider && Number(slider.max) > 100000 ? value - Number(slider.min) : value,
      renders: player.diagnostics.renderedFrames, opaque, varied, paintedSourceMs, paintedRecordedUnixMs,
      playbackCallback: player.__qaPlaybackCallback,
      ...player.__qaPresented, control: player.getPlaybackControlDiagnostics() };
  });
async function assertSourceRate(page, rate, { containerRate } = {}) {
  const before = await presentedSample(page);
  await page.waitForTimeout(2000);
  const after = await presentedSample(page);
  const elapsed = (after.at - before.at) / 1000;
  assert(after.renders > before.renders + 4, `${rate}x: actual frames must keep rendering`);
  for (const frame of [before, after]) {
    assert(frame.opaque > 100 && frame.varied > 100, "canvas must contain visible source media");
    assert.equal(frame.paintedSourceMs, Math.floor(frame.sourcePTS / 1000), "painted source PTS must belong to the presented frame");
  }
  const sourceRate = (after.sourcePTS - before.sourcePTS) / (elapsed * 1e6);
  assert(Math.abs(sourceRate - rate) <= rate * 0.2, `real source measured ${sourceRate}x, expected ${rate}x`);
  if (containerRate !== undefined) {
    const observed = (after.containerPTS - before.containerPTS) / (elapsed * 1e6);
    assert(Math.abs(observed - containerRate) <= containerRate * 0.2, `retimed container measured ${observed}x`);
  }
  return { before, after, sourceRate };
}
async function assertPresentedRate(page, rate) {
  const sample = () => presentedSample(page);
  const before = await sample();
  await page.waitForTimeout(2000);
  const after = await sample();
  const elapsed = (after.at - before.at) / 1000;
  const mediaDelta = after.time - before.time;
  const cursorDelta = after.cursor - before.cursor;
  assert(Number.isFinite(before.cursor) && Number.isFinite(after.cursor), "presented slider rate requires a visible playback slider");
  assert(after.renders > before.renders + 4, `${rate}x: frames must actually render`);
  assert(after.opaque > 100 && after.varied > 100, `${rate}x: canvas must contain visible media`);
  assert(Math.abs((after.pts - before.pts) / 1e6 - mediaDelta) < 0.001, `${rate}x: media time must follow presented PTS`);
  assert(Math.abs(mediaDelta / elapsed - rate) <= rate * 0.2, `${rate}x: rendered media measured ${mediaDelta / elapsed}x`);
  assert(Math.abs(cursorDelta - mediaDelta) < 1.2, `${rate}x: slider must follow presented media`);
  assert.equal(after.paintedSourceMs, Math.floor(after.sourcePTS / 1000), "canvas source stamp must match presented metadata");
  assert(Math.abs((after.sourcePTS - before.sourcePTS) / 1e6 - mediaDelta) < 0.01, `${rate}x: SDK/slider must follow the real source rather than retimed container PTS`);
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
  const stopped = await h.page.evaluate(() => window.__playbackQA.events.findLast(event =>
    event.type === "stop" && typeof event.mediaTime === "number"));
  assert(Number.isFinite(stopped?.mediaTime), "pause must capture a presented frame timestamp");
  const position = Math.floor((stopped.recordedUnixSeconds ?? (sessions.at(-1).start_time + stopped.mediaTime)) - sessions[0].start_time);
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
    speedRequests: window.__playbackQA.speedRequests, unhandledRejections: window.__playbackQA.unhandledRejections,
    progress: [...document.querySelectorAll('input[aria-label="录像进度"]')].map(input => ({
      value: Number(input.value), min: Number(input.min), max: Number(input.max),
    })),
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
  for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
    await check(`Beijing calendar offset progress aligns with actual frame Unix / ${viewport.width}`, async () => {
      const h = await open(viewport, false, { fixture: { timeSource: "recorded", firstFrameOffsetSeconds: 6, containerClockRate: 0.167 } });
      await playback(h.page);
      const frame = await assertOffsetSession(h);
      assert(frame.cursor >= 6, "first session must expose the six-second offset in global progress");
      await bounds(h.page, `offset-${viewport.width}`);
      await writeFile(new URL(`offset-${viewport.width}.json`, artifacts), JSON.stringify(frame, null, 2));
      await h.close();
    });
  }
  await check("Beijing calendar offset pause resume twice uses the last actual picture", async () => {
    const h = await open(undefined, false, { fixture: { timeSource: "recorded", firstFrameOffsetSeconds: 6 } });
    await playback(h.page);
    const globalStart = (await h.api("state")).sessions.find(session => session.mode === "playback").start_time;
    for (let cycle = 0; cycle < 2; cycle++) {
      await assertOffsetSession(h);
      const before = await h.api("state");
      const position = await pausePresented(h);
      await h.page.waitForTimeout(350);
      assert.equal(await cursor(h.page), position, "paused picture and global progress must stay frozen");
      assert((await h.api("state")).sockets.every(socket => socket.closed));
      await button(h.page, "播放").click();
      await h.page.waitForFunction(() => window.__playbackQA.snapshot().some(p => p.running && p.time > 0.2));
      const after = await h.api("state");
      assert.equal(after.sessions.length, before.sessions.length + 1);
      assert.equal(after.sessions.at(-1).start_time, globalStart + position,
        "resume request must use actual paused frame Unix, including the preceding session offset");
      await assertOffsetSession(h);
    }
    await saveSnapshot(h.page, "offset-pause-resume");
    await h.close();
  });
  await check("Beijing calendar offset forward twice starts ten seconds after the actual picture", async () => {
    const h = await open(undefined, false, { fixture: { timeSource: "recorded", firstFrameOffsetSeconds: 6 } });
    await playback(h.page);
    for (let cycle = 0; cycle < 2; cycle++) {
      await assertOffsetSession(h);
      const previous = await active(h.page);
      const before = await h.api("state");
      await button(h.page, "快进10秒").click();
      await h.page.waitForFunction(url => window.__playbackQA.snapshot().some(p => p.running && p.url !== url && p.time > 0.2), previous.url);
      const stopped = await h.page.evaluate(url => window.__playbackQA.events.findLast(event => event.type === "stop" && event.url === url), previous.url);
      const after = await h.api("state");
      assert.equal(after.sessions.length, before.sessions.length + 1);
      assert.equal(after.sessions.at(-1).start_time, Math.floor(stopped.recordedUnixSeconds) + 10,
        "forward request must include real first-frame offset, not just session-relative elapsed time");
      await assertOffsetSession(h);
    }
    await saveSnapshot(h.page, "offset-forward");
    await h.close();
  });
  await check("Beijing calendar offset seek and restore normal speed retain the global window", async () => {
    const h = await open(undefined, false, { fixture: { timeSource: "recorded", firstFrameOffsetSeconds: 6 } });
    await playback(h.page);
    const globalStart = (await h.api("state")).sessions.find(session => session.mode === "playback").start_time;
    await seek(h.page, 120);
    await h.page.waitForFunction(() => window.__playbackQA.snapshot().some(p => p.running && p.time > 0.2));
    assert.equal((await h.api("state")).sessions.at(-1).start_time, globalStart + 120);
    assert((await assertOffsetSession(h)).cursor >= 126, "seek must display the new session offset in the original global window");
    await selectRate(h.page, 8);
    const previous = await active(h.page);
    const before = await h.api("state");
    await normalRate(h);
    await assertRestoredPlayback(h, before, previous);
    await assertOffsetSession(h);
    await h.close();
  });
  await check("Beijing calendar offset before the window clamps progress to zero", async () => {
    const h = await open(undefined, false, { fixture: { timeSource: "recorded", firstFrameOffsetSeconds: -6 } });
    await playback(h.page);
    const frame = await presentedSample(h.page);
    const start = (await h.api("state")).sessions.find(session => session.mode === "playback").start_time;
    assert(frame.paintedRecordedUnixMs / 1000 < start, "source picture must actually precede the global window");
    assert.equal(frame.cursor, 0);
    await assertSessionClock(h);
    await h.close();
  });
  await check("Beijing calendar offset beyond the window clamps progress and stops playback", async () => {
    const h = await open(undefined, false, { fixture: { timeSource: "recorded", firstFrameOffsetSeconds: 6 } });
    await playback(h.page);
    const end = (await h.api("state")).sessions.at(-1).end_time;
    await seek(h.page, 3597);
    await h.page.waitForFunction(end => window.__playbackQA.snapshot().some(p =>
      p.hasPlayed && p.recordedUnixSeconds > end && !p.running), end);
    await button(h.page, "播放").waitFor();
    assert.equal(await cursor(h.page), 3599, "exclusive playback end must clamp to the last valid second");
    const last = await h.page.evaluate(() => window.__playbackQA.snapshot().findLast(p => p.hasPlayed));
    assert(last.recordedUnixSeconds > end, "source must overshoot the end rather than suppressing the offset");
    assert.equal(last.running, false);
    await h.close();
  });
  await check("Beijing calendar offset without recorded metadata keeps relative fallback", async () => {
    const h = await open(undefined, false, { fixture: { timeSource: "container", firstFrameOffsetSeconds: 6 } });
    await playback(h.page);
    await seek(h.page, 120);
    await h.page.waitForFunction(() => window.__playbackQA.snapshot().some(p => p.running && p.time > 0.2));
    await assertSessionClock(h);
    const frame = await presentedSample(h.page);
    assert.equal(frame.playbackCallback.recordedUnixSeconds, null);
    assert.equal(frame.recordedPTS, null);
    assert(Math.abs(frame.cursor - 120 - frame.time) < 1.2);
    await h.close();
  });
  await check("live hides playback actions", async () => {
    const h = await open({ width: 1440, height: 900 }, false, { expectRender: false });
    assert.equal(await button(h.page, "选择倍速").count(), 0);
    assert.equal(await button(h.page, "快进10秒").count(), 0);
    await h.close();
  });
  await check("8x real SDK rate / return 1x / sound / cancel", async () => {
    const h = await open(); const { page } = h;
    await playback(page);
    await button(page, "开启声音").click();
    await button(page, "关闭声音").waitFor();
    assert.equal((await active(page)).audioState, "running");
    for (const rate of [8]) {
      const sessionCount = (await h.api("state")).sessions.length;
      await selectRate(page, rate);
      assert.equal((await h.api("state")).sockets.at(-1).rate, rate);
      assert.equal((await h.api("state")).sessions.length, sessionCount, "rate switch must reuse session");
      await assertPresentedRate(page, rate);
      await normalRate(h);
      await assertPresentedRate(page, 1);
      assert.equal((await active(page)).volume, 80);
      assert.equal((await active(page)).mutedBySpeed, false);
      await button(page, "关闭声音").waitFor();
      await button(page, "选择倍速").click();
      assert.deepEqual(await page.getByRole("dialog").getByRole("button").allTextContents(),
        ["8倍速", "取消"], "reopened speed menu must offer only 8x and cancel");
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
      for (const rate of [8, 1]) {
        if (rate === 8) await selectRate(page, rate);
        else await normalRate(h);
        const state = await h.api("state");
        assert.deepEqual(state.rejectedCommands, [], "fixture must never receive a binary or malformed control command");
        assert.equal(state.sessions.length, before.sessions.length + (rate === 1 ? 1 : 0), "only normal restore re-signs the session");
        assert.equal(state.sockets.length, before.sockets.length + (rate === 1 ? 1 : 0), "only normal restore replaces the WebSocket");
        assert.equal(state.sockets.at(-1).closed, false);
        assert.equal(state.sockets.at(-1).rate, rate);
        const commands = state.commands.slice(before.commands.length);
        assert.equal(commands.length, 1, "normal restore must not send a same-socket speed1 command");
        assert.deepEqual(commands.map(({ id, at, format, ...message }) => {
          assert.equal(id, before.sockets.at(-1).id, "commands must use the original playback socket");
          assert.equal(format, "json", "the actual WebSocket frame must be JSON text");
          assert(Number.isFinite(at));
          return message;
        }), [{ sequence: initialSequence, cmd: "speed", rate: 8 }]);
        const sdk = await active(page);
        if (rate === 8) {
          assert.equal(sdk.id, player.id, "acceleration must reuse the SDK player");
          assert.equal(sdk.url, player.url);
          assert.equal(sdk.stopCount, player.stopCount, "metadata and acceleration must not restart playback");
        } else assert.notEqual(sdk.id, player.id, "normal restore must adopt a new SDK player");
        assert.equal(sdk.mutedBySpeed, rate !== 1);
        const diagnostics = await page.evaluate(() => window.__playbackQA.players.findLast(p => p._isRunning).getPlaybackControlDiagnostics());
        assert.equal(diagnostics.protocol, "json");
        assert.equal(diagnostics.phase, rate === 8 ? "verified" : "idle");
        assert.equal(diagnostics.requestedRate, rate);
        assert.equal(diagnostics.verifiedRate, rate);
        assert.equal(diagnostics.commandSequence, rate === 8 ? commands.at(-1).sequence : null);
        assert.equal(diagnostics.ack, rate === 8 ? "accepted" : "none");
        assert.equal(diagnostics.failure, null);
        observations.push(await assertPresentedRate(page, rate));
        await assertSessionClock(h);
      }
      const after = await h.api("state");
      assert.equal(after.sessions.length, before.sessions.length + 1, "media sampling must not hide a duplicate normal restore");
      assert.equal(after.sockets.length, before.sockets.length + 1);
      assert.deepEqual(after.commands.slice(before.commands.length).map(command => command.rate), [8]);
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
      await selectRate(page, 8);
      const speedMuted = await recordAudio("8x output muted");
      assert.equal(speedMuted.gain, 0, "8x must mute the actual output gain");
      assert(speedMuted.audioSignal.peak < 0.0001, "8x must silence synthetic PCM output");
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
      await normalRate(h);
      await assertAudible("return 1x restores pre-speed sound intent", accelerated.url);
      await assertPresentedRate(page, 1);
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
    await selectRate(page, 8);
    const muted = button(page, "倍速播放静音");
    await muted.waitFor();
    assert.equal(await muted.isDisabled(), true);
    assert.equal(await muted.getAttribute("aria-pressed"), "false");
    await saveSnapshot(page, "speed-sound-disabled");
    await normalRate(h);
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
    const h = await open(); await playback(h.page); await selectRate(h.page, 8);
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
    assert.equal((await h.api("state")).sockets.at(-1).rate, 8);
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
    await button(h.page, "选择倍速").click(); await button(h.page, "8倍速").click();
    await h.page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.pendingRate === 8);
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
  await check("pending 8x restore 1x re-signs once without delayed React reconnect", async () => {
    const h = await open(undefined, false, { fixture: { containerClockRate: 0.167 } });
    const { page } = h;
    await playback(page);
    const before = await h.api("state"); const player = await active(page);
    await requestPendingRate(page, 8);
    const beforeRestore = await h.api("state");
    // Click the actual React control while the old setSpeed(8) promise is still pending.
    await normalRate(h);
    // Keep the real clock running past the stopped request's original six-second deadline.
    await page.waitForTimeout(6600);
    const state = await assertRestoredPlayback(h, beforeRestore, player);
    assert.deepEqual(state.commands.slice(before.commands.length).map(command => command.rate), [8], "no stale fallback/duplicate speed command");
    assert.equal(state.sockets.at(-1).rate, 1);
    const requests = await page.evaluate(() => window.__playbackQA.speedRequests);
    assert.equal(requests.at(-1).rate, 8, "new normal session must not call setSpeed(1)");
    assert.equal(requests.at(-1).status, "rejected", "stopping the old session must reject its SDK promise");
    assert.match(requests.at(-1).error, /stopped/i, "old request must reject because its session stopped");
    assert(requests.at(-1).settledAt - requests.at(-1).at < 6000, "restore must cancel the old verification timeout");
    assert.equal((await active(page)).pendingRate, null);
    assert.equal((await active(page)).control.requestedRate, 1);
    await button(page, "选择倍速").waitFor();
    await assertSourceRate(page, 1, { containerRate: 0.167 });
    await saveSnapshot(page, "pending-8x-restore-1x");
    await h.close();
  });
  await check("paused normal restore creates no unused audio player; resume restores 1x and sound", async () => {
    const h = await open(); const { page } = h;
    await playback(page);
    await button(page, "开启声音").click();
    await button(page, "关闭声音").waitFor();
    await selectRate(page, 8);
    const position = await pausePresented(h);
    await page.waitForFunction(() => !window.__playbackQA.snapshot().some(player => player.running));
    const before = await h.api("state");
    const playerCount = await page.evaluate(() => window.__playbackQA.snapshot().length);
    await button(page, "恢复正常速度").click();
    await button(page, "选择倍速").waitFor();
    await page.waitForTimeout(1200);
    const paused = await h.api("state");
    assert.equal(paused.sessions.length, before.sessions.length, "paused normal restore must not sign a session");
    assert.equal(paused.sockets.length, before.sockets.length);
    assert.deepEqual(paused.commands, before.commands, "paused restore must not send speed1");
    assert.equal(await cursor(page), position, "paused normal restore must retain the last presented cursor");
    const players = await page.evaluate(() => window.__playbackQA.snapshot());
    assert.equal(players.length, playerCount, "paused restore must not prepare an unused audio player");
    assert(players.every(player => !player.running && !player.monitor && !player.socket
      && player.audioContexts.every(context => context.state === "closed")), "paused players must release transport and audio");
    await button(page, "播放").click();
    await page.waitForFunction(() => {
      const player = window.__playbackQA.snapshot().findLast(player => player.running);
      return player?.time > 0.2 && player.audioState === "running" && player.volume === 80
        && !player.mutedBySpeed && player.gain > 0.7 && player.audioPackets > 2
        && player.audioSignal?.peak > 0.05 && player.audioSignal.rms > 0.02;
    });
    const resumed = await h.api("state");
    assert.equal(resumed.sessions.length, before.sessions.length + 1, "only explicit play may re-sign after paused normal restore");
    assert.equal(resumed.sessions.at(-1).start_time, before.sessions.find(session => session.mode === "playback").start_time + position);
    assert.equal(resumed.sockets.at(-1).rate, 1);
    assert.deepEqual(resumed.commands, before.commands, "resuming a normal session needs no speed command");
    const player = await active(page);
    assert(player.audioGestureReady && player.audioPreparations.some(preparation => preparation.gesture),
      "resume must adopt audio prepared in the trusted play gesture");
    await button(page, "关闭声音").waitFor();
    await assertPresentedRate(page, 1);
    await assertSessionClock(h);
    await h.close();
  });
  for (const pending of [false, true]) await check(`standalone component restores 1x on the same socket / ${pending ? "pending" : "verified"} 8x`, async () => {
    const h = await open(undefined, true, { fixture: pending ? { containerClockRate: 0.167 } : {} });
    const { page } = h;
    const before = await h.api("state"); const player = await active(page);
    if (pending) await requestPendingRate(page, 8);
    else {
      await selectRate(page, 8);
      await assertSourceRate(page, 8);
    }
    await normalRate(h);
    await page.waitForTimeout(6600);
    const state = await assertSamePlayback(h, before, player);
    assert.deepEqual(state.commands.map(command => command.rate), [8, 1], "standalone restore must call SDK setSpeed(1) exactly once");
    assert.deepEqual(state.commands.map(command => command.sequence), [0, 1]);
    const requests = await page.evaluate(() => window.__playbackQA.speedRequests);
    assert.equal(requests.at(-2).status, pending ? "rejected" : "resolved");
    if (pending) assert.match(requests.at(-2).error, /superseded/i);
    assert.equal(requests.at(-1).status, "resolved");
    assert.equal((await active(page)).control.requestedRate, 1);
    await assertSourceRate(page, 1, pending ? { containerRate: 0.167 } : {});
    await h.close();
  });
  await check("unsequenced ACK during pending restore rebuilds normal session without stale failure", async () => {
    const h = await open(undefined, false, { fixture: { containerClockRate: 0.167, unsequencedACK: true } });
    const { page } = h;
    await playback(page);
    const before = await h.api("state");
    const player = await active(page);
    await requestPendingRate(page, 8);
    const beforeRestore = await h.api("state");
    await normalRate(h);
    await page.waitForTimeout(6600);
    const state = await assertRestoredPlayback(h, beforeRestore, player);
    assert.equal(state.sessions.length, before.sessions.length + 1, "explicit restore with unsequenced ACK must re-sign exactly once");
    assert.deepEqual(state.commands.slice(before.commands.length).map(command => command.rate), [8]);
    assert.equal(state.sockets.at(-1).rate, 1);
    assert.equal(await page.locator(".h5-playback-control-failure").count(), 0);
    await assertSourceRate(page, 1, { containerRate: 0.167 });
    await h.close();
  });
  await check("retimed container resolves unconfirmed after 6s with real 8x frames and restores 1x", async () => {
    const h = await open(undefined, false, { fixture: { containerClockRate: 0.167 } });
    const { page } = h;
    await playback(page);
    const before = await h.api("state"); const player = await active(page);
    await requestPendingRate(page, 8);
    await page.waitForFunction(() => {
      const sdk = window.__playbackQA.snapshot().findLast(p => p.running);
      return sdk?.pendingRate === null && sdk.control.phase === 'unconfirmed';
    });
    const accelerated = await active(page);
    assert.equal(accelerated.control.timeSource, "container");
    assert.equal(accelerated.control.ack, "accepted");
    assert.equal(accelerated.control.requestedRate, 8);
    assert.notEqual(accelerated.control.verifiedRate, 8, "unconfirmed must not claim verified 8x");
    assert.equal(accelerated.control.failure, null);
    assert(accelerated.control.observedRate > 0.1 && accelerated.control.observedRate < 0.25);
    const request = await page.evaluate(() => window.__playbackQA.speedRequests.at(-1));
    assert.equal(request.status, "resolved");
    assert(request.settledAt - request.at >= 5900, "unconfirmed must wait for the real six-second ACK/frame deadline");
    assert.equal(await button(page, "恢复正常速度").innerText(), "8×");
    assert.equal(await button(page, "恢复正常速度").isEnabled(), true);
    const acceleratedSource = await assertSourceRate(page, 8, { containerRate: 0.167 });
    const state = await assertSamePlayback(h, before, player);
    assert.equal(state.sockets.at(-1).rate, 8);
    assert.deepEqual(state.commands.slice(before.commands.length).map(command => command.rate), [8]);
    const beforeRestore = await h.api("state");
    await normalRate(h);
    const normalSource = await assertSourceRate(page, 1, { containerRate: 0.167 });
    const restored = await assertRestoredPlayback(h, beforeRestore, player);
    assert.deepEqual(restored.commands.slice(before.commands.length).map(command => command.rate), [8]);
    assert.equal(restored.sockets.at(-1).rate, 1);
    assert.equal((await active(page)).control.phase, "idle");
    assert.equal((await active(page)).control.requestedRate, 1);
    await writeFile(new URL("retimed-container-source.json", artifacts), JSON.stringify({ acceleratedSource, normalSource }, null, 2));
    await saveSnapshot(page, "retimed-container-unconfirmed");
    await h.close();
  });
  await check("recorded frame metadata drives source PTS and slider through 8 to 1 and pause resume", async () => {
    const h = await open(undefined, false, { fixture: { timeSource: "recorded", containerClockRate: 0.167 } });
    const { page } = h;
    await playback(page);
    const before = await h.api("state"); const player = await active(page);
    assert.equal(player.control.timeSource, "recorded", "real _handleH264Frame must select recorded calendar metadata");
    await requestPendingRate(page, 8);
    await page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.pendingRate === null);
    const fast = await active(page);
    assert.equal(fast.control.phase, "verified");
    assert.equal(fast.control.verifiedRate, 8);
    assert.equal(fast.control.timeSource, "recorded");
    const accelerated = await assertPresentedRate(page, 8);
    assert.equal(accelerated.after.pts, accelerated.after.recordedPTS, "decoded frame must retain its own recorded metadata");
    assert.equal(accelerated.before.pts, accelerated.before.recordedPTS);
    assert(accelerated.after.sourcePTS > accelerated.after.containerPTS * 2, "fixture must retain genuinely different clocks");
    await assertSamePlayback(h, before, player);
    const beforeRestore = await h.api("state");
    await normalRate(h);
    const normal = await assertPresentedRate(page, 1);
    assert.equal((await active(page)).control.phase, "idle");
    assert.equal((await active(page)).control.verifiedRate, 1);
    await assertRestoredPlayback(h, beforeRestore, player);
    const position = await pausePresented(h);
    await page.waitForFunction(() => !window.__playbackQA.snapshot().some(p => p.running));
    await page.waitForTimeout(1200);
    assert.equal(await cursor(page), position, "pause must freeze the recorded-time slider");
    const paused = await h.api("state");
    assert.equal(paused.sessions.length, before.sessions.length + 1, "only the explicit normal restore re-signs before pause");
    assert(paused.sockets.every(socket => socket.closed), "pause closes recorded source transport");
    await button(page, "播放").click();
    await page.waitForFunction(() => window.__playbackQA.snapshot().some(p => p.running && p.time > 0.2 && p.control.timeSource === 'recorded'));
    const resumed = await h.api("state");
    assert.equal(resumed.sessions.length, before.sessions.length + 2, "normal restore and explicit resume each re-sign once");
    assert.equal(resumed.sessions.at(-1).start_time, before.sessions.at(-1).start_time + position);
    assert.equal(resumed.sockets.at(-1).rate, 1);
    const resumedSource = await assertPresentedRate(page, 1);
    assert.equal(resumedSource.after.pts, resumedSource.after.recordedPTS);
    await assertSessionClock(h);
    await writeFile(new URL("recorded-source-slider.json", artifacts), JSON.stringify({ accelerated, normal, position, resumedSource }, null, 2));
    await h.close();
  });
  await check("recorded second precision retains pictures with repeated metadata timestamps", async () => {
    const h = await open(undefined, false, { fixture: { timeSource: "recorded", recordedPrecision: "second", containerClockRate: 0.167 } });
    const { page } = h;
    await playback(page);
    const before = await active(page);
    await page.waitForFunction(count => window.__playbackQA.snapshot().findLast(p => p.running)?.repeatedRecordedFrames > count + 4, before.repeatedRecordedFrames);
    const repeated = await active(page);
    assert.equal(repeated.control.timeSource, "recorded");
    assert(repeated.renders > before.renders + 4, "repeated second timestamps must still draw their distinct source pictures");
    await selectRate(page, 8);
    const source = await assertSourceRate(page, 8, { containerRate: 0.167 });
    assert.equal(source.before.pts, source.before.recordedPTS);
    assert.equal(source.after.pts, source.after.recordedPTS);
    assert.equal(source.after.control.phase, "verified");
    const recordedDelta = (source.after.pts - source.before.pts) / 1e6;
    const sourceDelta = (source.after.sourcePTS - source.before.sourcePTS) / 1e6;
    assert(Math.abs(recordedDelta - sourceDelta) < 1.01, "second precision may quantize elapsed media by less than one second");
    assert(Math.abs(source.after.cursor - source.after.time) < 1.2, "slider must follow quantized recorded time");
    await saveSnapshot(page, "recorded-second-precision");
    await h.close();
  });
  await check("recorded 1.18x observations cannot reset real 8x pictures or re-sign the session", async () => {
    const h = await open(undefined, false, { fixture: { timeSource: "recorded", recordedClockRate: 1.18, containerClockRate: 0.167 } });
    const { page } = h;
    await playback(page);
    const before = await h.api("state"); const player = await active(page);
    await requestPendingRate(page, 8);
    await page.waitForFunction(() => {
      const sdk = window.__playbackQA.snapshot().findLast(p => p.running);
      return sdk?.pendingRate === null && sdk.control.phase === "unconfirmed";
    });
    await assertSourceRate(page, 8);
    await page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)
      ?.control.observations?.at(-1)?.elapsedMs > 26_000, null, { timeout: 35_000 });
    const current = await active(page);
    assert.equal(current.control.failure, null);
    assert.equal(current.control.requestedRate, 8);
    assert(Math.abs(current.control.observedRate - 1.18) < 0.15);
    assert(current.control.observations.length <= 8);
    await assertSamePlayback(h, before, player);
    await assertSourceRate(page, 8);
    await page.evaluate(() => Object.defineProperty(navigator, "clipboard", { configurable: true,
      value: { writeText: async report => { window.__playbackReport = report; } } }));
    await button(page, "复制倍速诊断信息").click();
    const report = JSON.parse(await page.evaluate(() => window.__playbackReport));
    assert(report.observations.at(-1).elapsedMs > 26_000, "copy must use current observations, not the six-second snapshot");
    assert(report.observations.every(sample => Math.abs(sample.observedRate - 1.18) < 0.15));
    let state = await h.api("state");
    assert.deepEqual(state.commands.slice(before.commands.length).map(command => command.rate), [8]);
    await h.api("control", { recordedClockRate: null });
    const beforeRestore = await h.api("state");
    await normalRate(h);
    await assertSourceRate(page, 1);
    state = await assertRestoredPlayback(h, beforeRestore, player);
    assert.deepEqual(state.commands.slice(before.commands.length).map(command => command.rate), [8]);
    await assertPresentedRate(page, 1);
    await saveSnapshot(page, "recorded-1.18-no-reset");
    await h.close();
  });
  for (const rate of [8]) await check(`accepted but ignored ${rate}x speed stays observable and can be restored manually`, async () => {
    const h = await open(undefined, false, { fixture: { timeSource: "recorded" } });
    const { page } = h;
    await playback(page);
    const before = await h.api("state"); const player = await active(page);
    await page.evaluate(() => {
      const sdk = window.__playbackQA.players.findLast(player => player._isRunning);
      const notify = sdk.onPlaybackControlDiagnostics;
      window.__playbackQA.controlHistory = [];
      sdk.onPlaybackControlDiagnostics = diagnostic => {
        window.__playbackQA.controlHistory.push(diagnostic);
        notify(diagnostic);
      };
    });
    await h.api("control", { ignoreNextSpeed: true });
    await requestPendingRate(page, rate);
    await page.waitForFunction(() => {
      const sdk = window.__playbackQA.snapshot().findLast(p => p.running);
      return sdk?.pendingRate === null && sdk.control.phase === "unconfirmed";
    });
    await assertSourceRate(page, 1);
    const state = await assertSamePlayback(h, before, player);
    assert.equal(state.sockets.at(-1).rate, 1, "fixture must demonstrate ignored upstream acceleration");
    const diagnostic = (await active(page)).control;
    assert.equal(diagnostic.phase, "unconfirmed");
    assert.equal(diagnostic.ack, "accepted");
    assert.equal(diagnostic.requestedRate, rate);
    assert.equal(diagnostic.verifiedRate, 1, `the UI must not claim measured ${rate}x`);
    assert(Math.abs(diagnostic.observedRate - 1) < 0.2, "accepted command must retain measured normal-speed evidence");
    const history = await page.evaluate(() => window.__playbackQA.controlHistory);
    assert(!history.some(diagnostic => diagnostic.phase === "verified" && diagnostic.requestedRate === rate),
      "an ignored command must never transiently publish verified status");
    assert.deepEqual(state.commands.slice(before.commands.length).map(command => command.rate), [rate]);
    await page.evaluate(() => Object.defineProperty(navigator, "clipboard", { configurable: true,
      value: { writeText: async report => { window.__playbackReport = report; } } }));
    await button(page, "复制倍速诊断信息").click();
    const report = JSON.parse(await page.evaluate(() => window.__playbackReport));
    assert.equal(report.phase, "unconfirmed");
    assert.equal(report.requestedRate, rate);
    assert.equal(report.verifiedRate, 1, "copied diagnostics must not turn accepted into measured acceleration");
    await writeFile(new URL(`accepted-ignored-${rate}x.json`, artifacts), JSON.stringify({ diagnostic, history, report }, null, 2));
    const beforeRestore = await h.api("state");
    await normalRate(h);
    await assertPresentedRate(page, 1);
    await assertRestoredPlayback(h, beforeRestore, player);
    await h.close();
  });
  await check("previously verified 8x becomes unconfirmed without copying stale verified8", async () => {
    const h = await open(undefined, false, { fixture: { timeSource: "recorded" } });
    const { page } = h;
    await playback(page);
    const before = await h.api("state"); const player = await active(page);
    await selectRate(page, 8);
    assert.equal((await active(page)).control.phase, "verified");
    assert.equal((await active(page)).control.verifiedRate, 8);
    await assertSourceRate(page, 8);
    await h.api("control", { recordedClockRate: 1 });
    await page.waitForFunction(() => window.__playbackQA.snapshot().findLast(player => player.running)
      ?.control.phase === "unconfirmed");
    await assertSamePlayback(h, before, player);
    assert.equal((await active(page)).control.verifiedRate, null,
      "diagnostic downgrade must immediately invalidate the previous verified8");
    // A mismatch can include buffered frames; wait for two settled windows before measuring 1x.
    await page.waitForFunction(() => {
      const diagnostic = window.__playbackQA.snapshot().findLast(player => player.running)?.control;
      const windows = diagnostic?.observations?.slice(-2) || [];
      return diagnostic?.phase === "unconfirmed" && windows.length === 2
        && windows.every(sample => Math.abs(sample.observedRate - 1) < 0.2);
    }, undefined, { timeout: 20_000 });
    const diagnostic = (await active(page)).control;
    assert.equal(diagnostic.requestedRate, 8);
    assert.equal(diagnostic.ack, "accepted");
    assert(Math.abs(diagnostic.observedRate - 1) < 0.2, "later windows must show the changed recorded clock");
    await page.evaluate(() => Object.defineProperty(navigator, "clipboard", { configurable: true,
      value: { writeText: async report => { window.__playbackReport = report; } } }));
    await button(page, "复制倍速诊断信息").click();
    const report = JSON.parse(await page.evaluate(() => window.__playbackReport));
    await writeFile(new URL("8x-unconfirmed-after-verification.json", artifacts), JSON.stringify({ diagnostic, report }, null, 2));
    assert.equal(report.phase, "unconfirmed");
    assert.equal(report.requestedRate, 8);
    assert.notEqual(report.verifiedRate, 8, "unconfirmed diagnostics must not present the previous verified8 as the current rate");
    assert.notEqual(diagnostic.verifiedRate, 8, "unconfirmed SDK diagnostics must not retain a stale verified8 claim");
    assert.equal(report.verifiedRate, null, "copied diagnostics must preserve unknown rather than invent normal speed");
    assert.equal(diagnostic.verifiedRate, null, "continuous mismatch must invalidate the current verified rate");
    await h.api("control", { recordedClockRate: null });
    await page.waitForFunction(() => {
      const diagnostic = window.__playbackQA.snapshot().findLast(player => player.running)?.control;
      return diagnostic?.phase === "verified" && diagnostic.verifiedRate === 8;
    });
    const recovered = await assertSamePlayback(h, before, player);
    assert.deepEqual(recovered.commands.slice(before.commands.length).map(command => command.rate), [8],
      "diagnostic downgrade and recovery must not resend speed or restore normal playback");
    const source = await assertSourceRate(page, 8);
    await writeFile(new URL("8x-reverified-after-unconfirmed.json", artifacts),
      JSON.stringify({ diagnostic: (await active(page)).control, source }, null, 2));
    await h.close();
  });
  for (const failure of ["rejectNextSpeed"]) await check(`${failure}: recover with fresh normal session`, async () => {
    const h = await open();
    await playback(h.page); const count = (await h.api("state")).sessions.length;
    await h.page.setViewportSize({ width: 390, height: 844 });
    await button(h.page, "全屏").click();
    await h.page.evaluate(() => Object.defineProperty(navigator, "clipboard", { configurable: true,
      value: { writeText: async report => { window.__playbackReport = report; } } }));
    await h.api("control", { [failure]: true });
    await button(h.page, "选择倍速").click(); await button(h.page, "8倍速").click();
    await h.page.getByText("录像倍速切换失败，正在重新连接正常速度回放", { exact: true }).waitFor();
    await button(h.page, "选择倍速").waitFor();
    await h.page.waitForFunction(() => window.__playbackQA.snapshot().findLast(p => p.running)?.time > 0);
    assert.equal((await h.api("state")).sessions.length, count + 1);
    assert.equal((await h.api("state")).sockets.at(-1).rate, 1);
    const notice = h.page.locator(".h5-player-shell .h5-playback-control-failure");
    await notice.getByText("未能确认8倍速生效，请继续使用正常速度回放", { exact: true }).waitFor();
    await button(h.page, "复制倍速诊断信息").click();
    await notice.getByText("诊断信息已复制", { exact: true }).waitFor();
    const reportText = await h.page.evaluate(() => window.__playbackReport);
    const report = JSON.parse(reportText);
    assert.equal(report.phase, "failed");
    assert.equal(report.requestedRate, 8);
    assert.equal(report.verifiedRate, 1);
    assert.equal(report.failure, "rejected");
    assert.equal(report.ack, "rejected");
    assert.equal(report.timeSource, "container");
    const failedRequest = await h.page.evaluate(() => window.__playbackQA.speedRequests.findLast(request => request.rate === 8));
    assert.equal(failedRequest.status, "rejected", "explicit rejection must reject the actual SDK promise");
    assert(!/wss?:|token|camera|fixture|private/.test(reportText), "report must exclude stream/user identifiers");
    const noticeBounds = await notice.boundingBox();
    assert(noticeBounds.x >= 0 && noticeBounds.y >= 0 && noticeBounds.x + noticeBounds.width <= 391
      && noticeBounds.y + noticeBounds.height <= 845, "fullscreen failure notice must fit the phone viewport");
    await h.page.screenshot({ path: new URL(`${failure}-notice.png`, artifacts).pathname });
    await h.page.evaluate(() => Object.defineProperty(navigator, "clipboard", { configurable: true,
      value: { writeText: async () => { throw new Error("Clipboard denied"); } } }));
    await button(h.page, "复制倍速诊断信息").click();
    const fallback = notice.getByRole("textbox", { name: "倍速诊断信息" });
    await fallback.waitFor();
    assert.deepEqual(JSON.parse(await fallback.inputValue()), report, "manual fallback must preserve the same safe report");
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
  await writeFile(new URL("results.json", artifacts), JSON.stringify({ source: "synthetic decoded frames; actual React + SDK + local HTTP/WS; no device validation",
    faultInjection: omitRecordedUnix ? "fixture omits onPlaybackTime second argument" : null, results }, null, 2));
}
console.log(`${results.filter(r => r.status === "passed").length}/${results.length} passed; artifacts: ${artifacts.pathname}`);
if (results.some(r => r.status === "failed")) process.exitCode = 1;
