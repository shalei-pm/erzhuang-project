import template from "../vendor/digital-twin/index.html?raw";
import styles from "../vendor/digital-twin/styles.css?url";
import model from "../vendor/digital-twin/model.js?url";
import scenes from "../vendor/digital-twin/scenes.js?url";
import core from "../vendor/digital-twin/kit-core.js?url";
import chartData from "../vendor/digital-twin/chart-data.js?url";
import charts from "../vendor/digital-twin/charts.js?url";
import app from "../vendor/digital-twin/app.js?url";
import demo from "../vendor/digital-twin/demo-data.js?url";

export function digitalTwinDocument() {
  const doc = new DOMParser().parseFromString(template, "text/html");
  doc.querySelectorAll("script, link").forEach(node => node.remove());
  const sheet = doc.createElement("link"); sheet.rel = "stylesheet"; sheet.href = new URL(styles, location.origin).href; doc.head.append(sheet);
  const override = doc.createElement("style");
  override.textContent = ".scene-modebar,#debug-toggle,#debug-panel{display:none!important}.brand{pointer-events:none}.scene-footer{flex-wrap:wrap}.header-meta{gap:12px}body{margin:0}.product-name>span{font-size:10px;letter-spacing:0;color:#b6c9bd}@media(max-width:1099px){body.dashboard{min-height:0}}";
  doc.head.append(override);
  const brand = doc.querySelector(".brand"); brand?.removeAttribute("href");
  doc.querySelector(".product-name h1")!.textContent = "门店数字孪生看板";
  doc.querySelector(".product-name>span")!.textContent = "演示人数与趋势 · 真实摄像头";
  for (const url of [model, scenes, core, chartData, charts, app, demo]) {
    const script = doc.createElement("script"); script.src = new URL(url, location.origin).href; doc.body.append(script);
  }
  return `<!doctype html>${doc.documentElement.outerHTML}`;
}
