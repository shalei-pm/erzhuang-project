import assert from "node:assert/strict";
import {createRequire} from "node:module";
import {fixtureAPI,previewState} from "../../scripts/digital-twin-preview.mjs";
const require=createRequire(import.meta.url);
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||"playwright");
const browser=await chromium.launch({channel:"chrome",headless:true});
const base="http://127.0.0.1:5179/erzhuang-project";
let checked=0;
async function setup(width=1440,height=900,options={}) {
  const context=await browser.newContext({viewport:{width,height}}),page=await context.newPage(),state=previewState(),errors=[];
  if(options.ids) state.store_ids=options.ids;
  page.on("pageerror",error=>errors.push(error.message));
  await context.route("**/api/**",route=>{
    const req=route.request(),path=new URL(req.url()).pathname;
    if(options.unauthenticated && path.endsWith("/auth/me")) return route.fulfill({status:401,json:{code:"auth_check_failed",error:"需要登录"}});
    const [status,body]=fixtureAPI(path,req.method(),req.postDataJSON()||{},state);
    return route.fulfill({status,json:body});
  });
  await context.routeWebSocket("**/nvrapi/**",socket=>socket.close());
  return {context,page,state,errors};
}
try {
  for(const [width,height] of [[1440,900],[1366,768],[390,844]]) {
    const t=await setup(width,height); await t.page.goto(base+"/digitaltwin/");
    const kit=t.page.frameLocator("iframe"); await kit.locator("#room-treatment .room-name").waitFor();
    const logo=kit.getByRole('img',{name:'新氧青春诊所 SOYOUNG CLINIC',exact:true});
    assert(await logo.evaluate(image=>image.complete && image.naturalWidth===627));
    assert.equal(await t.page.locator('.system-topbar').count(),0);
    assert.equal(await kit.getByRole('button',{name:'登出',exact:true}).count(),1);
    await kit.getByRole('button',{name:'演示设置',exact:true}).click();
    assert.equal(await kit.locator('#debug-panel').isVisible(),true);
    assert.equal(await kit.locator('.permission-control').isVisible(),false);
    assert.equal(await kit.locator('.kit-input-panel').isVisible(),false);
    const beforeCount=Number(await kit.locator('#output-treatment').innerText());
    await kit.locator('#plus-treatment').click();
    assert.equal(Number(await kit.locator('#output-treatment').innerText()),beforeCount+1);
    assert.equal(await kit.locator('.account-copy small').innerText(),'已登录二壮');
    await kit.getByRole('button',{name:'关闭演示设置',exact:true}).click();
    assert.equal(await kit.locator(".camera-trigger").count(),6);
    assert.deepEqual(await kit.locator('#room-reception .camera-trigger').evaluateAll(nodes=>nodes.map(node=>node.dataset.cameraId)),["76","75"]);
    assert.equal(await kit.locator('.camera-trigger[data-camera-id="74"]').count(),0);
    assert.equal(t.state.requests.filter(r=>r.path.endsWith("stream-session")).length,0);
    assert.equal(await t.page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
    await t.page.screenshot({path:`/tmp/digital-twin-${width}.png`,fullPage:true});
    await kit.locator('.camera-trigger[data-camera-id="111"]').click();
    await t.page.getByRole("dialog").waitFor();
    assert.equal(await t.page.getByRole('dialog').evaluate(node=>getComputedStyle(node).backgroundColor),'rgb(7, 29, 20)');
    await t.page.getByText("本地联调不连接真实摄像头",{exact:false}).waitFor();
    await t.page.screenshot({path:`/tmp/digital-twin-dialog-${width}.png`,fullPage:true});
    assert(t.state.requests.some(r=>r.path.endsWith("/10001/cameras/111/stream-session")));
    assert(t.state.requests.filter(r=>r.path.endsWith("stream-session")).every(r=>r.body.mode==="live"));
    assert.equal(await t.page.getByRole("dialog").getByText("录像",{exact:true}).count(),0);
    await t.page.getByRole("button",{name:"关闭摄像头",exact:true}).click();
    assert.equal(await t.page.getByRole("dialog").count(),0);
    assert.equal(await t.page.locator(".twin-unmapped").count(),0);
    assert.equal(await kit.locator('.camera-trigger[data-camera-id="73"]').count(),0);
    assert(!t.state.requests.some(r=>r.path.endsWith("/10001/cameras/73/stream-session")));
    assert.deepEqual(t.errors,[]); await t.context.close(); checked++;
  }
  const s=await setup(); await s.page.goto(base+"/");
  await s.page.getByRole("button",{name:"系统设置",exact:true}).click();
  await s.page.getByRole("button",{name:"数字孪生白名单",exact:true}).click();
  await s.page.getByRole("cell",{name:"北京保利总部店",exact:true}).waitFor();
  await s.page.getByLabel("机构 ID",{exact:true}).fill("10042");
  await s.page.getByRole("button",{name:"添加机构",exact:true}).click();
  await s.page.getByRole("button",{name:"保存白名单",exact:true}).click();
  await s.page.getByText("已保存，全局生效").waitFor(); assert.deepEqual(s.state.store_ids,["10001","10042"]);
  await s.page.screenshot({path:"/tmp/digital-twin-settings.png",fullPage:true});
  await s.page.goto(base+"/digitaltwin/");
  const kit=s.page.frameLocator("iframe"); await kit.getByRole("button",{name:"北京保利总部店",exact:false}).click();
  await kit.getByRole("button",{name:"联调示例机构",exact:true}).click();
  try { await kit.locator(".store-selected-name").filter({hasText:"联调示例机构"}).waitFor({timeout:5000}); }
  catch(error) { console.log({errors:s.errors,requests:s.state.requests.slice(-6),body:await s.page.locator("body").innerText()});await s.page.screenshot({path:"/tmp/digital-twin-switch-failure.png",fullPage:true});throw error; }
  assert(s.page.url().includes("store=10042")); assert.deepEqual(s.errors,[]); await s.context.close(); checked++;
  for(const options of [{ids:[]},{ids:["10042"]},{unauthenticated:true}]) {
    const t=await setup(1440,900,options); await t.page.goto(base+"/digitaltwin/?store=10001");
    await t.page.locator(options.unauthenticated?".auth-page":".twin-empty[role=alert]").waitFor();
    assert.equal(await t.page.locator("iframe").count(),0); assert(!t.state.requests.some(r=>r.path.endsWith("stream-session")));
    assert.deepEqual(t.errors,[]);await t.context.close();checked++;
  }
  console.log(`digital twin browser checks: ${checked} passed`);
} finally { await browser.close(); }
