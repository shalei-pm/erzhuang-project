// Run against a local Vite server; every API and company navigation is intercepted.
// PLAYWRIGHT_MODULE may point to an installed Playwright package outside this repo.
import assert from "node:assert/strict";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const browser = await chromium.launch({ ...(process.env.QA_CHROMIUM_EXECUTABLE ? { executablePath: process.env.QA_CHROMIUM_EXECUTABLE } : { channel: "chrome" }), headless: true });
const localOrigin = process.env.QA_LOCAL_ORIGIN || "http://127.0.0.1:5178";
const origin = "http://lite.sy.soyoung.com";
const base = "/erzhuang-project";
const routes = ["/", "/h5/orgs/10001/monitor"];
const codes = ["session_idle_timeout", "session_absolute_timeout", "session_reauthentication_required"];
const user = { email: "test@example.invalid", username: "qa", display_name: "测试账号", role: "admin" };
const success = { enabled: true, authenticated: true, user };
let checked = 0;
let activeTest;

async function setup(path, options = {}) {
  const context = await browser.newContext({ viewport: path === "/" ? { width: 1440, height: 900 } : { width: 390, height: 844 } });
  await context.routeWebSocket("**/*", (socket) => socket.close());
  const page = await context.newPage();
  page.setDefaultTimeout(10_000);
  const requests = [];
  const navigations = [];
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await context.route("**/*", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.startsWith(`${base}/api/`)) {
      requests.push(url.pathname);
      const authPath = url.pathname.endsWith("/auth/me");
      if (authPath && options.networkFailure) return route.abort("failed");
      let status = authPath ? options.authStatus || 200 : 200;
      let body = authPath ? options.auth || success : {};
      if (url.pathname.endsWith("/auth/session-status")) {
        const response = options.statusResponse || { status: 401, body: { code: "session_idle_timeout" } };
        status = response.status;
        body = response.body;
      } else if (url.pathname.endsWith("/monitor-mode")) body = { mode: "nvr" };
      else if (url.pathname.endsWith("/cameras")) body = { store_name: "测试门店", cameras: [] };
      else if (url.pathname.endsWith("/stores")) body = { items: [], stores: [], cities: [], total: 0 };
      else if (url.pathname.endsWith("/users")) body = { users: [] };
      else if (url.pathname.endsWith("/audit-logs")) body = { items: [{ action: "auth.absolute_timeout", actor_display_name: "测试账号", created_at: "2026-09-07T08:00:00Z", result: "success" }], total: 1 };
      return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    }
    if (route.request().isNavigationRequest()) navigations.push(url.pathname + url.search);
    if (url.pathname === `${base}/logout` || url.pathname === `${base}/_/auth/callback`) {
      return route.fulfill({ contentType: "text/html", body: "<main>Intercepted auth navigation</main>" });
    }
    if (url.origin !== origin) return route.abort();
    const response = await route.fetch({ url: localOrigin + url.pathname + url.search });
    return route.fulfill({ response });
  });
  await page.goto(origin + base + path);
  activeTest = { page, context, requests, navigations, errors };
  return activeTest;
}

async function assertBlocked(page) {
  await page.locator(".auth-page").waitFor();
  assert.equal(await page.locator(".h5-camera-wall, .resource-store-list, .settings-tabs, .resource-store-detail").count(), 0);
}

try {
  for (const path of routes) {
    for (const networkFailure of [false, true]) {
      const test = await setup(path, { authStatus: 503, networkFailure });
      await assertBlocked(test.page);
      assert.match(await test.page.locator(".auth-copy").innerText(), /登录状态加载失败/);
      assert(test.requests.every((url) => url.endsWith("/auth/me")));
      assert.equal(test.navigations.length, 1);
      assert.deepEqual(test.errors, []);
      if (!networkFailure) await test.page.screenshot({ path: `/tmp/session-auth-${path === "/" ? "desktop" : "mobile"}.png` });
      await test.context.close();
      checked++;
    }

    for (const code of [...codes, "session_login_required"]) {
      const login_url = `${base}/_/auth/callback?return_to=h5`;
      const test = await setup(path, { authStatus: 401, auth: { code, login_url } });
      await test.page.waitForURL(code === "session_login_required" ? "**/_/auth/callback?*" : "**/logout?*");
      assert.equal(test.navigations.length, 2);
      if (code === "session_login_required") assert(test.page.url().endsWith(login_url));
      else assert(test.page.url().includes("logouttogether"));
      assert(test.requests.every((url) => url.endsWith("/auth/me")));
      assert.deepEqual(test.errors, []);
      await test.context.close();
      checked++;
    }

    for (const code of [...codes, "session_login_required"]) {
      const test = await setup(path);
      await test.page.getByRole("button", { name: "退出登录" }).waitFor();
      await test.page.evaluate(() => {
        sessionStorage.setItem("erzhuang:idle-session-timeout-redirected", "1");
        sessionStorage.setItem("erzhuang:session-login-redirected", "1");
      });
      await test.page.route("**/api/h5/nvr-monitor/orgs/10001/cameras", (route) => route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ code }) }));
      await test.page.evaluate(async () => {
        const { nvrLabApi } = await import("/erzhuang-project/src/api-nvr-lab.ts");
        await nvrLabApi.listCameras("10001").catch(() => {});
      });
      await assertBlocked(test.page);
      assert.equal(test.navigations.length, 1);
      await test.page.getByRole("button", { name: "使用公司 SSO 登录" }).click();
      await test.page.waitForURL(code === "session_login_required" ? "**/_/auth/callback" : "**/logout?*");
      assert.deepEqual(test.errors, []);
      await test.context.close();
      checked++;
    }

    for (const status of [401, 503]) {
      const test = await setup(path, {
        auth: { ...success, session: { idle_remaining_ms: 1200, absolute_remaining_ms: 2000 } },
        statusResponse: { status, body: status === 401 ? { code: "session_absolute_timeout" } : { error: "unavailable" } },
      });
      await test.page.getByRole("button", { name: "退出登录" }).waitFor();
      if (status === 401) await test.page.waitForURL("**/logout?*");
      else await assertBlocked(test.page);
      assert.equal(test.requests.filter((url) => url.endsWith("/session-status")).length, 1);
      if (status === 503) assert.equal(test.navigations.length, 1);
      assert.deepEqual(test.errors, []);
      await test.context.close();
      checked++;
    }
  }

  const audit = await setup("/");
  await audit.page.getByRole("button", { name: "系统设置" }).click();
  await audit.page.getByRole("button", { name: "操作日志", exact: true }).click();
  await audit.page.getByRole("cell", { name: "登录满8小时失效" }).waitFor();
  await audit.page.getByLabel("操作类型").selectOption("auth.absolute_timeout");
  await audit.page.screenshot({ path: "/tmp/session-auth-audit.png" });
  assert.deepEqual(audit.errors, []);
  await audit.context.close();
  checked++;
  console.log(`PASS: ${checked} browser cases (desktop + H5, callback/logout, global invalidation, deadlines, fail-closed, audit).`);
} catch (error) {
  if (activeTest && !activeTest.page.isClosed()) {
    console.error({ checked, url: activeTest.page.url(), requests: activeTest.requests, errors: activeTest.errors, body: await activeTest.page.locator("body").innerText() });
    await activeTest.page.screenshot({ path: "/tmp/session-auth-failure.png" });
  }
  throw error;
} finally {
  await browser.close();
}
