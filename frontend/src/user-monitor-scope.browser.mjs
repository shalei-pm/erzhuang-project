import assert from "node:assert/strict";
import { mkdir } from "node:fs/promises";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const origin = process.env.QA_LOCAL_ORIGIN || "http://127.0.0.1:5193";
const artifacts = new URL("../../output/playwright/", import.meta.url);
await mkdir(artifacts, { recursive: true });
const browser = await chromium.launch({ channel: "chrome", headless: true });
const html = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"></head><body>
<div id="root"></div><script type="module">
import RefreshRuntime from '/erzhuang-project/@react-refresh';
RefreshRuntime.injectIntoGlobalHook(window);
window.$RefreshReg$ = () => {};
window.$RefreshSig$ = () => (type) => type;
window.__vite_plugin_react_preamble_installed__ = true;
const React = (await import('/erzhuang-project/node_modules/.vite/deps/react.js')).default;
const domModule = await import('/erzhuang-project/node_modules/.vite/deps/react-dom_client.js');
const { createRoot } = domModule.default || domModule;
const { storeSpaceApi } = await import('/erzhuang-project/src/api.ts');
const { UserManagement } = await import('/erzhuang-project/src/components/UserManagement.tsx');
await import('/erzhuang-project/src/styles.css');
const state = window.__scopeQA = { requests: 0, payload: null, candidates: [
 { institutionId: 10053, city: '南京', name: '南京金鹰世界店', externalOrgId: '10053' },
 { institutionId: 10030, city: '南京', name: '数字化测试门店', externalOrgId: '10030' },
] };
let user = { id: 7, email: 'viewer@example.test', username: 'viewer', displayName: '验收用户', role: 'viewer', enabled: true, monitorScopeRevision: 'revision-1', monitorStoreScopeCount: 1, monitorStoreScopes: [{ institutionId: 10099, city: '上海', name: '上海无有效工控机机构', externalOrgId: '10099' }] };
storeSpaceApi.listUsers = async () => [user];
storeSpaceApi.listMonitorStoreScopeCandidates = async () => { state.requests++; if (state.failCandidates) throw new Error('机构目录暂时不可用'); return structuredClone(state.candidates); };
storeSpaceApi.updateUser = async (_, payload) => {
 state.payload = payload;
 const all = new Map([...user.monitorStoreScopes, ...state.candidates].map(s => [s.institutionId, s]));
 const scopes = [...all.values()].filter(s => payload.monitorInstitutionIds.includes(s.institutionId));
 user = { ...user, ...payload, email: user.email, monitorStoreScopes: scopes, monitorStoreScopeCount: scopes.length };
 return user;
};
createRoot(document.getElementById('root')).render(React.createElement(UserManagement, { onToast() {} }));
</script></body></html>`;

try {
  for (const width of [1440, 1024, 390]) {
    const context = await browser.newContext({ viewport: { width, height: 900 } });
    await context.route("**/*", route => {
      const url = new URL(route.request().url());
      if (url.origin !== origin || url.pathname.includes("/api/")) return route.abort();
      if (url.pathname.endsWith("/__qa-user-scope")) return route.fulfill({ contentType: "text/html", body: html });
      return route.continue();
    });
    const page = await context.newPage();
    page.setDefaultTimeout(8000);
    const errors = [];
    page.on("pageerror", error => errors.push(error.message));
    page.on("console", message => { if (message.type() === "error") console.error("fixture console:", message.text()); });
    await page.goto(`${origin}/erzhuang-project/__qa-user-scope`);
    try {
      await page.getByRole("button", { name: "编辑", exact: true }).click({ timeout: 10000 });
    } catch (error) {
      console.error("fixture startup errors:", errors);
      throw error;
    }
    try {
      await page.getByRole("button", { name: "南京", exact: true }).click();
    } catch (error) {
      console.error("fixture state:", await page.evaluate(() => ({ state: window.__scopeQA, text: document.body.innerText })));
      throw error;
    }
    await page.locator(".user-scope-row").filter({ hasText: "10030" }).waitFor();
    await page.getByRole("button", { name: "取消", exact: true }).click();
    await page.evaluate(() => window.__scopeQA.candidates.push({ institutionId: 10086, city: "南京", name: "南京金陵中环店", externalOrgId: "10086" }));
    await page.getByRole("button", { name: "编辑", exact: true }).click();
    await page.getByRole("button", { name: "南京", exact: true }).click();
    await page.locator(".user-scope-row").filter({ hasText: "10086" }).waitFor({ timeout: 3000 });
    assert.equal(await page.locator(".user-scope-row").count(), 3);
    for (const id of ["10030", "10086"]) await page.locator(".user-scope-row").filter({ hasText: id }).getByRole("checkbox").check();
    await page.getByRole("button", { name: "保存", exact: true }).click();
    assert.deepEqual(await page.evaluate(() => window.__scopeQA.payload.monitorInstitutionIds.slice().sort((a,b) => a-b)), [10030,10086,10099]);
    assert.equal(await page.evaluate(() => window.__scopeQA.payload.monitorScopeRevision), 'revision-1');
    await page.getByRole("button", { name: "编辑", exact: true }).click();
    await page.getByRole("button", { name: "南京", exact: true }).click();
    for (const id of ["10030", "10086"]) assert.equal(await page.locator(".user-scope-row").filter({ hasText: id }).getByRole("checkbox").isChecked(), true);
    await page.getByRole('button', { name: '全部', exact: true }).click();
    const unavailable = page.locator('.user-scope-row').filter({ hasText: '10099' });
    assert.equal(await unavailable.getByRole('checkbox').isChecked(), true);
    assert.match(await unavailable.innerText(), /不在当前可选范围/);
    assert.match(await page.locator('.user-scope-summary').innerText(), /可选 3 家.*保留授权 1 家/);
    await page.getByRole('button', { name: '上海', exact: true }).click();
    assert.match(await page.locator('.user-scope-summary').innerText(), /可选 0 家.*保留授权 1 家/);
    await unavailable.scrollIntoViewIfNeeded();
    assert.equal(await unavailable.locator('span').evaluate(element => getComputedStyle(element).whiteSpace), 'normal');
    assert.equal(await unavailable.locator('span').evaluate(element => element.scrollWidth > element.clientWidth), false);
    await page.screenshot({ path: new URL(`retained-permission-${width}.png`, artifacts).pathname });
    await unavailable.getByRole('checkbox').uncheck();
    await page.getByRole('button', { name: '保存', exact: true }).click();
    assert.deepEqual(await page.evaluate(() => window.__scopeQA.payload.monitorInstitutionIds.slice().sort((a,b) => a-b)), [10030,10086]);
    await page.evaluate(() => { window.__scopeQA.failCandidates = true; });
    await page.getByRole('button', { name: '编辑', exact: true }).click();
    await page.getByRole('alert').waitFor();
    assert.equal(await page.getByRole('button', { name: '保存', exact: true }).isDisabled(), true);
    await page.evaluate(() => { window.__scopeQA.failCandidates = false; });
    await page.getByRole('button', { name: '重试', exact: true }).click();
    await page.waitForFunction(() => !document.querySelector('.modal-actions .primary-button')?.disabled);
    for (const label of ['取消', '保存']) {
      const bounds = await page.getByRole('button', { name: label, exact: true }).boundingBox();
      assert.ok(bounds && bounds.x >= 0 && bounds.y >= 0 && bounds.x + bounds.width <= width && bounds.y + bounds.height <= 900, `${label} must be fully inside viewport at ${width}px`);
    }
    assert.equal(await page.locator('.user-modal-body').evaluate(element => element.scrollWidth > element.clientWidth), false);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    assert.deepEqual(errors, []);
    await page.screenshot({ path: new URL(`nanjing-permission-${width}.png`, artifacts).pathname });
    await context.close();
  }
  console.log("PASS CRM permission browser scenarios: fresh candidates, CRM IDs, saved selection, out-of-scope grants counted separately, revoke, source failure and retry");
} finally {
  await browser.close();
}
