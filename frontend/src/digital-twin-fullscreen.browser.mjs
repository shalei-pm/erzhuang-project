import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const css = await Promise.all(["./styles.css", "./pages/digital-twin.css"].map(path => readFile(new URL(path, import.meta.url), "utf8")));
const browser = await chromium.launch({ channel: "chrome", headless: true });
try {
  for (const [width, height] of [[2560,1440],[3840,2160],[3440,1440],[1440,900]]) {
    const page = await browser.newPage({ viewport: { width, height } });
    // Mirror NVRLabPlayer's nested surfaces and fullscreen target, without a real camera connection.
    await page.setContent(`<style>${css.join("\n")}</style><dialog class="twin-camera-dialog"><header>合成画面布局验收</header><div class="h5-player-shell"><div class="h5-player-rotator"><div class="h5-player-wrapper"><div class="h5-player-container"><canvas width="1920" height="1080"></canvas><button id="fullscreen" style="position:absolute;top:0;left:0">全屏</button></div></div></div></div></dialog>`);
    await page.evaluate(() => {
      document.querySelector("dialog").showModal();
      const canvas = document.querySelector("canvas"), ctx = canvas.getContext("2d");
      ctx.fillStyle = "#28988b"; ctx.fillRect(0,0,1920,1080);
      ctx.strokeStyle = "white"; ctx.lineWidth = 10; ctx.strokeRect(5,5,1910,1070);
      document.querySelector("#fullscreen").onclick = () => document.querySelector(".h5-player-shell").requestFullscreen();
    });
    const normal = await page.locator('.h5-player-container').boundingBox();
    assert(normal.height <= height * .72 + 1);
    await page.locator("#fullscreen").click();
    await page.waitForFunction(() => !!document.fullscreenElement);
    const sizes = await page.evaluate(() => {
      const rect = selector => { const r=document.querySelector(selector).getBoundingClientRect(); return {x:r.x,y:r.y,width:r.width,height:r.height}; };
      return {shell:rect('.h5-player-shell'),surface:rect('.h5-player-container'),canvas:rect('canvas'),fit:getComputedStyle(document.querySelector('canvas')).objectFit};
    });
    console.log({viewport:[width,height],...sizes});
    await page.screenshot({path:`/tmp/twin-fullscreen-${width}.png`});
    assert(Math.abs(sizes.surface.height - sizes.shell.height) <= 1, 'fullscreen surface must not retain the dialog 72vh cap');
    assert(Math.abs(sizes.surface.width - sizes.shell.width) <= 1);
    assert.equal(sizes.fit, 'contain');
    await page.evaluate(() => document.exitFullscreen());
    await page.waitForFunction(() => !document.fullscreenElement);
    const restored = await page.locator('.h5-player-container').boundingBox();
    assert(Math.abs(restored.height - normal.height) <= 1);
    await page.close();
  }
  console.log('fullscreen layout: 4 viewports passed');
} finally { await browser.close(); }
