// 诊断脚本：数字孪生摄像头弹窗是否会被后台刷新意外关闭。
// 用 addInitScript 在主页面与 srcdoc iframe 中都安装埋点，记录：
//   - TwinDashboard.mount 调用次数（React 是否重建了看板实例）
//   - AbortController.abort（看板 clearPlayer 拆播放器的真实入口）
//   - HTMLDialogElement.close / showModal
//   - 关键接口的请求与返回时刻
// 运行：PLAYWRIGHT_MODULE=<playwright> [WATCH_MS=100000] [T1_DELAY_MS=2500] node frontend/src/digital-twin-camera-dialog.browser.mjs
import { createRequire } from "node:module";
import { fixtureAPI, previewState } from "../../scripts/digital-twin-preview.mjs";

const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");

const base = `${process.env.QA_LOCAL_ORIGIN || "http://127.0.0.1:5189"}/erzhuang-project`;
const watchMS = Number(process.env.WATCH_MS || 100_000);
const t1DelayMS = Number(process.env.T1_DELAY_MS || 2500);

const browser = await chromium.launch({ channel: "chrome", headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await context.newPage();
const state = previewState();
const started = Date.now();
const stamp = () => `+${((Date.now() - started) / 1000).toFixed(2)}s`;
const timeline = [];
const mark = message => timeline.push(`${stamp()} ${message}`);

page.on("pageerror", error => mark(`pageerror ${error.message}`));
page.on("console", message => {
  if (message.type() === "error") mark(`console ${message.text()}`);
});
await page.addInitScript(() => {
  window.__diag = window.__diag || { events: [], mounts: 0, aborts: [], closes: [], showModals: [] };
  const diag = window.__diag;
  diag.at = () => Math.round(performance.now());
  diag.hookMount = () => {
    if (!window.TwinDashboard || diag.mountHooked) return;
    diag.mountHooked = true;
    const original = window.TwinDashboard.mount;
    window.TwinDashboard.mount = function (...args) {
      diag.mounts += 1;
      diag.events.push({ t: diag.at(), kind: "TwinDashboard.mount", n: diag.mounts, stack: new Error("mount").stack });
      return original.apply(this, args);
    };
  };
  const abortOriginal = AbortController.prototype.abort;
  AbortController.prototype.abort = function (...args) {
    diag.aborts.push({ t: diag.at(), stack: new Error("abort").stack });
    diag.events.push({ t: diag.at(), kind: "AbortController.abort" });
    return abortOriginal.apply(this, args);
  };
  if (window.HTMLDialogElement) {
    const closeOriginal = HTMLDialogElement.prototype.close;
    HTMLDialogElement.prototype.close = function (...args) {
      diag.closes.push({ t: diag.at(), stack: new Error("close").stack });
      diag.events.push({ t: diag.at(), kind: "dialog.close" });
      return closeOriginal.apply(this, args);
    };
    const showOriginal = HTMLDialogElement.prototype.showModal;
    HTMLDialogElement.prototype.showModal = function (...args) {
      diag.showModals.push({ t: diag.at(), className: this.className });
      diag.events.push({ t: diag.at(), kind: "dialog.showModal", className: this.className });
      return showOriginal.apply(this, args);
    };
  }
  diag.hookMount();
  document.addEventListener("DOMContentLoaded", diag.hookMount);
});

await context.route("**/api/**", async route => {
  const request = route.request();
  const path = new URL(request.url()).pathname;
  const [status, body] = fixtureAPI(path, request.method(), request.postDataJSON?.() || {}, state);
  const kind = path.split("/").pop();
  mark(`api <- ${request.method()} .../${kind}`);
  if (path.endsWith("/t1-bi") && t1DelayMS) await new Promise(resolve => setTimeout(resolve, t1DelayMS));
  await route.fulfill({ status, json: body });
  mark(`api -> ${status} ${kind}`);
});
await context.routeWebSocket("**/nvrapi/**", socket => socket.close());

const parentDialog = page.locator("dialog.twin-camera-dialog");

try {
  await page.goto(`${base}/digitaltwin/`);
  const kit = page.frameLocator("iframe");
  const trigger = kit.locator("#treatment-camera-dock .camera-trigger").first();
  await trigger.waitFor({ timeout: 20_000 });
  mark("dashboard ready (camera trigger visible)");

  await page.evaluate(() => {
    window.__parentDiag = { removedAt: null, addedAt: null };
    const observer = new MutationObserver(() => {
      const node = document.querySelector("dialog.twin-camera-dialog");
      if (node && window.__parentDiag.addedAt === null) window.__parentDiag.addedAt = Math.round(performance.now());
      if (!node && window.__parentDiag.addedAt !== null && window.__parentDiag.removedAt === null) window.__parentDiag.removedAt = Math.round(performance.now());
    });
    observer.observe(document.body, { childList: true, subtree: true });
  });

  await trigger.click();
  await parentDialog.waitFor({ state: "visible", timeout: 15_000 });
  mark("camera dialog opened");

  const samples = [];
  let closedAt = null;
  for (let second = 1; second <= Math.ceil(watchMS / 1000); second += 1) {
    await page.waitForTimeout(1000);
    const open = (await parentDialog.count()) > 0;
    samples.push(second + (open ? "+" : "-"));
    if (!open && closedAt === null) {
      closedAt = second;
      mark(`parent dialog REMOVED (sampled at ${second}s)`);
    }
  }

  const parentDiag = await page.evaluate(() => window.__parentDiag);
  const frames = page.frames();
  const iframeDiag = await frames[1].evaluate(() => ({
    mounts: window.__diag?.mounts ?? -1,
    events: window.__diag?.events ?? [],
    aborts: window.__diag?.aborts ?? [],
    closes: window.__diag?.closes ?? [],
  }));
  console.log(`watch=${watchMS / 1000}s t1Delay=${t1DelayMS}ms closedAt=${closedAt}s`);
  console.log("parent: dialog addedAt/removedAt(ms since page start):", JSON.stringify(parentDiag));
  console.log("iframe mounts:", iframeDiag.mounts);
  console.log("timeline:\n" + timeline.join("\n"));
  console.log("iframe aborts:\n" + JSON.stringify(iframeDiag.aborts, null, 2));
  console.log("iframe closes:\n" + JSON.stringify(iframeDiag.closes, null, 2));
  console.log("iframe events:\n" + JSON.stringify(iframeDiag.events, null, 2));
  console.log("samples(+open/-closed): " + samples.join(" "));
} finally {
  await browser.close();
}
