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
  override.textContent = ".scene-modebar,.permission-control,.kit-input-panel{display:none!important}.brand{pointer-events:none}.scene-footer{flex-wrap:wrap}.header-meta{gap:12px}body{margin:0}.product-name{display:grid;grid-template-columns:auto auto;align-items:center;gap:0 12px}.product-name>span{grid-column:1/-1;font-size:10px;letter-spacing:0;color:#b6c9bd}.product-name h1{letter-spacing:0}.debug-toggle{grid-column:2;grid-row:1;white-space:nowrap}@media(max-width:1099px){body.dashboard{min-height:0}}@media(max-width:650px){.product-name{gap:0 6px}.debug-toggle{padding:5px}}";
  doc.head.append(override);
  doc.querySelector(".product-name")!.append(doc.querySelector("#debug-toggle")!);
  doc.querySelector("#debug-toggle")!.setAttribute("aria-label", "演示设置");
  doc.querySelector(".rail-description")!.innerHTML = '当前场景 <strong id="total-count">0</strong> 人。调节各区域演示人数，查看人物增减效果。仅当前页面生效，不修改业务数据。';
  doc.querySelector(".control-footer")!.textContent = "人数与运营趋势均为演示数据";
  doc.querySelector(".limit-note")!.textContent = "加减按钮用于0–80人演示；刷新页面或切换机构后恢复默认演示人数。";
  const logout = doc.createElement("button");
  logout.id = "twin-logout";
  logout.type = "button";
  logout.textContent = "登出";
  doc.querySelector(".account-summary")!.append(logout);
  override.textContent += "#twin-logout{margin-left:12px;padding:6px 8px;background:transparent;border:0;color:#b6c9bd;font:inherit;font-size:12px;cursor:pointer;white-space:nowrap}#twin-logout:hover{color:#fff}#twin-logout:focus-visible{outline:2px solid #00d7a0;outline-offset:2px}#twin-logout:disabled{opacity:.5;cursor:wait}";
  const brand = doc.querySelector(".brand"); brand?.removeAttribute("href");
  doc.querySelector(".product-name h1")!.textContent = "门店数字孪生看板";
  doc.querySelector(".product-name>span")!.textContent = "演示人数与趋势 · 真实摄像头";
  for (const url of [model, scenes, core, chartData, charts, app, demo]) {
    const script = doc.createElement("script"); script.src = new URL(url, location.origin).href; doc.body.append(script);
  }
  return `<!doctype html>${doc.documentElement.outerHTML}`;
}
