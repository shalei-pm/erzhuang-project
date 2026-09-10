/* Data contract only: no DOM, transport, credentials, or business-metric calculations. */
(function(root){
 const regionIds=['reception','consultation','treatment','aftercare','waiting'];
 const overviewKeys=['expected','arrived','receptionists','consultants','nurses','doctors'];
 const trendKeys=['noConsult','consult','stayAll','stayNo','stayConsult','waitAll','waitNo','waitConsult','upgradeAll','upgradeNo','upgradeConsult','redemptionAll','redemptionNo','redemptionConsult','servicePointAll','servicePointNo','servicePointConsult'];
 const regionMeasurementKeys=['current','cumulative','noConsultation','consultationRequired','staff'];
 const clone=x=>structuredClone(x);
 function object(x,path){if(!x||typeof x!=='object'||Array.isArray(x)||Object.prototype.toString.call(x)!=='[object Object]'||(Object.getPrototypeOf(x)!==null&&Object.getPrototypeOf(x)?.constructor?.name!=='Object'))throw new TypeError(`${path}: expected plain object`);}
 function keys(x,allowed,path){object(x,path);for(const k of Object.keys(x))if(!allowed.includes(k))throw new TypeError(`${path}.${k}: unknown field`);}
 function count(x,path){if(x!==null&&(!Number.isSafeInteger(x)||x<0))throw new TypeError(`${path}: expected non-negative safe integer or null`);}
 function text(x,path,max=120){if(typeof x!=='string'||x.length>max)throw new TypeError(`${path}: expected string <= ${max} characters`);}
 function bool(x,path){if(typeof x!=='boolean')throw new TypeError(`${path}: expected boolean`);}
 function fieldList(x,allowed,path){if(!Array.isArray(x)||new Set(x).size!==x.length||x.some(k=>!allowed.includes(k)))throw new TypeError(`${path}: expected unique known fields`);}
 function emptyData(){return {mode:'external',revision:0,updatedAt:null,store:{id:'',name:'未选择门店',experimentStoreCount:null,userName:'未接入用户'},overview:Object.fromEntries(overviewKeys.map(k=>[k,null])),staleOverview:[],permissions:{canViewCameras:false},regions:Object.fromEntries(regionIds.map(id=>[id,{current:null,cumulative:null,noConsultation:null,consultationRequired:null,staff:null,staleFields:[],cameras:[]}])),trends:[]};}
 function validate(s){
  if(!['demo','external'].includes(s.mode))throw new TypeError('mode: demo or external');count(s.revision,'revision');if(s.revision===null)throw new TypeError('revision required');
  if(s.updatedAt!==null&&(typeof s.updatedAt!=='string'||!Number.isFinite(Date.parse(s.updatedAt))))throw new TypeError('updatedAt: ISO timestamp or null');
  keys(s.store,['id','name','experimentStoreCount','userName'],'store');for(const k of ['id','name','userName'])text(s.store[k],`store.${k}`);count(s.store.experimentStoreCount,'store.experimentStoreCount');
  keys(s.overview,overviewKeys,'overview');for(const k of overviewKeys)count(s.overview[k],`overview.${k}`);
  fieldList(s.staleOverview,overviewKeys,'staleOverview');
  keys(s.permissions,['canViewCameras'],'permissions');bool(s.permissions.canViewCameras,'permissions.canViewCameras');
  for(const id of regionIds){const r=s.regions[id];keys(r,[...regionMeasurementKeys,'staleFields','cameras'],`regions.${id}`);for(const k of regionMeasurementKeys)count(r[k],`${id}.${k}`);fieldList(r.staleFields,regionMeasurementKeys,`${id}.staleFields`);
   if(!Array.isArray(r.cameras)||r.cameras.length>1000)throw new TypeError(`${id}.cameras: array, maximum 1000`);
   const ids=new Set();for(const c of r.cameras){keys(c,['id','name','occupied','canView'],`${id}.camera`);text(c.id,'camera.id');text(c.name,'camera.name',80);if(!c.id||ids.has(c.id))throw new TypeError('camera.id must be unique and nonempty within region');ids.add(c.id);if(c.occupied!==null)bool(c.occupied,'camera.occupied');bool(c.canView,'camera.canView');}
  }
  if(!Array.isArray(s.trends)||s.trends.length>3660)throw new TypeError('trends: array, maximum 3660 dates');
  let prev='';for(const d of s.trends){keys(d,['date',...trendKeys],'trends');if(typeof d.date!=='string'||!/^\d{4}-\d{2}-\d{2}$/.test(d.date)||!Number.isFinite(Date.parse(d.date))||new Date(d.date).toISOString().slice(0,10)!==d.date||d.date<=prev)throw new TypeError('trends.date: valid, unique, ascending YYYY-MM-DD');prev=d.date;
   for(const k of trendKeys){const v=d[k];if(v!==null&&(typeof v!=='number'||!Number.isFinite(v)||v<0||v>1e12||(k.startsWith('upgrade')&&v>100)))throw new TypeError(`trends.${k}: invalid value`);if(['noConsult','consult'].includes(k)&&v!==null&&!Number.isSafeInteger(v))throw new TypeError(`trends.${k}: integer required`);}
  }return s;
 }
 function merge(current,patch){
  keys(patch,['mode','revision','updatedAt','store','overview','staleOverview','permissions','regions','trends'],'data');const next=clone(current);
  for(const k of ['mode','revision','updatedAt'])if(Object.hasOwn(patch,k))next[k]=patch[k];
  for(const k of ['store','overview','permissions'])if(Object.hasOwn(patch,k)){keys(patch[k],Object.keys(next[k]),k);Object.assign(next[k],clone(patch[k]));}
  if(Object.hasOwn(patch,'staleOverview'))next.staleOverview=clone(patch.staleOverview);
  if(Object.hasOwn(patch,'regions')){keys(patch.regions,regionIds,'regions');for(const [id,value]of Object.entries(patch.regions)){keys(value,[...regionMeasurementKeys,'staleFields','cameras'],`regions.${id}`);Object.assign(next.regions[id],clone(value));}}
  if(Object.hasOwn(patch,'trends')){if(!Array.isArray(patch.trends))throw new TypeError('trends must be an array');next.trends=patch.trends.map(d=>{object(d,'trend');return {...Object.fromEntries(trendKeys.map(k=>[k,null])),...clone(d)};});}
  return validate(next);
 }
 function renderPlan(s,limits){
  const sum=regionIds.reduce((n,id)=>n+BigInt(s.regions[id].current||0),0n),totalExceeded=sum>BigInt(limits.total);
  return Object.fromEntries(regionIds.map(id=>{const n=s.regions[id].current;const reason=n===null?'unknown':n>limits.perRegion?'region-limit':totalExceeded?'total-limit':null;return [id,{rendered:reason?0:n,reason}];}));
 }
 const api={regionIds,overviewKeys,trendKeys,emptyData,merge,validate,renderPlan,clone};if(typeof module!=='undefined')module.exports=api;else root.TwinKitCore=api;
})(typeof window==='undefined'?globalThis:window);
