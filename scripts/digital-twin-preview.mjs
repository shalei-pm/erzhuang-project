// Local-only synthetic preview. Never proxies API requests to company services.
import http from "node:http";
import { pathToFileURL } from "node:url";

export const fixtureStores = [
  {external_org_id:"10001",store_name:"北京保利总部店",city:"北京",available_camera_count:3},
  {external_org_id:"10042",store_name:"联调示例机构",city:"上海",available_camera_count:1},
];
export function previewState() { return { store_ids:["10001"], version:"preview:0", requests:[] }; }
export function fixtureAPI(path, method, body, state) {
  state.requests.push({path,method,body});
  if (path.endsWith("/auth/me")) return [200,{enabled:true,authenticated:true,user:{display_name:"本地联调账号",email:"preview@example.invalid",role:"admin",permissions:["store.read","store.write","user.manage","audit.view"]}}];
  if (path.endsWith("/auth/session-status")) return [200,{authenticated:true}];
  if (path.endsWith("/admin/digital-twin-settings")) {
    if (method === "PUT") {
      if (body.version !== state.version) return [409,{error:"白名单已被其他管理员更新，请重新加载后修改"}];
      if (body.store_ids.some(id => !fixtureStores.some(store => store.external_org_id===id))) return [400,{error:"机构不存在"}];
      state.store_ids = body.store_ids; state.version = `preview:${Number(state.version.split(":")[1])+1}`;
    }
    return [200,{store_ids:state.store_ids,version:state.version}];
  }
  if (path.endsWith("/admin/digital-twin-candidates")) return [200,{cities:[{city:"北京",stores:fixtureStores}]}];
  if (path.endsWith("/digitaltwin/stores")) return [200,{cities:[{city:"北京",stores:fixtureStores.filter(store=>state.store_ids.includes(store.external_org_id))}]}];
  const match = path.match(/\/digitaltwin\/orgs\/(\d+)\/cameras(?:\/(\d+)\/stream-session)?$/);
  if (match) {
    if (!state.store_ids.includes(match[1])) return [403,{error:"暂无该机构数字孪生访问权限"}];
    if (match[2]) return [503,{error:"本地联调不连接真实摄像头",code:"preview_only"}];
    return [200,{external_org_id:match[1],tenant_id:Number(match[1]),store_name:fixtureStores.find(store=>store.external_org_id===match[1]).store_name,cameras:[
      {id:111,name:"联调摄像头一",space_type:"治疗室",space_name:"治疗室4"},
      {id:72,name:"联调摄像头二",space_type:"面诊室",space_name:"面诊室1"},
      {id:73,name:"未绑定联调摄像头"},
      {id:74,name:"护理联调",space_type:"术后护理"},
      {id:75,name:"护士联调",space_type:"公共区域",space_name:"护士站"},
      {id:76,name:"前台联调",space_type:"前台",space_name:"前台1"},
      {id:77,name:"等候联调",space_type:"等候区",space_name:"等候区1"},
      {id:78,name:"等待联调",space_type:"等待区",space_name:"等待区1"},
    ]}];
  }
  if (path.endsWith("/store-space-resource-view/stores")) return [200,{items:[],total:0,cities:[],summary:{storeCount:0,edgeCount:0,nvrCount:0,cameraCount:0,spaceCount:0,boundCameraCount:0,unboundCameraCount:0,offlineDeviceCount:0,warningCount:0}}];
  if (path.endsWith("/users")) return [200,{users:[]}];
  return [404,{error:"本地联调未配置该接口"}];
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  const state = previewState();
  const server = http.createServer(async (request,response) => {
    try {
      const url = new URL(request.url,"http://127.0.0.1:5189");
      if (url.pathname.startsWith("/erzhuang-project/api/")) {
        let raw="";
        for await (const chunk of request) { raw+=chunk; if(raw.length>16384) { response.writeHead(413).end(); return; } }
        const [status,body]=fixtureAPI(url.pathname,request.method,raw?JSON.parse(raw):{},state);
        response.writeHead(status,{"Content-Type":"application/json","Cache-Control":"no-store"}).end(JSON.stringify(body));return;
      }
      const upstream=await fetch(`http://127.0.0.1:5179${url.pathname}${url.search}`);
      response.writeHead(upstream.status,{"Content-Type":upstream.headers.get("content-type")||"text/plain","Cache-Control":"no-store"}).end(Buffer.from(await upstream.arrayBuffer()));
    } catch { response.writeHead(502).end("Local preview unavailable"); }
  });
  server.listen(5189,"127.0.0.1",()=>console.log("Synthetic preview: http://127.0.0.1:5189/erzhuang-project/digitaltwin/"));
}
