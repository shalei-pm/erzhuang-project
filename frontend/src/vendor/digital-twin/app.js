/* Five-region dashboard template. Hosts own data fetching, authentication and camera playback. */
(function(root){
 const K=TwinKitCore,instances=new WeakMap();
 const definitions=[
  {id:'reception',name:'前台',en:'RECEPTION',state:'当前接待',cumulativeLabel:'已到访',segmentMetrics:[['noConsultation','无需咨询人数'],['consultationRequired','需要咨询人数']],caption:'接待登记 · 服务起点'},
  {id:'consultation',name:'咨询室',en:'CONSULTATION',state:'当前咨询',cumulativeLabel:'已接待',caption:'一对一咨询 · 沟通需求'},
  {id:'treatment',name:'治疗室',en:'TREATMENT',state:'当前治疗',cumulativeLabel:'已服务',segmentMetrics:[['noConsultation','无需咨询人数'],['consultationRequired','需要咨询人数']],caption:'专业诊疗 · 安心服务'},
  {id:'aftercare',name:'术后护理',en:'AFTERCARE',state:'当前护理',caption:'半躺休息 · 敷膜护理'},
  {id:'waiting',name:'等候区',en:'WAITING LOUNGE',state:'当前等候',segmentMetrics:[['noConsultation','无需咨询人数'],['consultationRequired','需要咨询人数']],caption:'中央开放等候 · 舒适休息'}
 ];
 function configuration(input={}){
  if(!input||typeof input!=='object'||Array.isArray(input))throw new TypeError('config must be object');
  for(const key of Object.keys(input))if(!['regions','chartOrder','chartTitles'].includes(key))throw new TypeError(`unknown config.${key}`);
  const regions=definitions.map(x=>({...x}));
  if(input.regions){for(const [id,settings]of Object.entries(input.regions)){const region=regions.find(r=>r.id===id);if(!region||!settings||typeof settings!=='object')throw new TypeError('unknown region config');for(const [key,value]of Object.entries(settings)){if(!['name','en','caption'].includes(key)||typeof value!=='string'||value.length>100)throw new TypeError('invalid region text');region[key]=value;}}}
  const order=input.chartOrder||TwinChartData.chartSpecs.map(s=>s.id);if(!Array.isArray(order)||!order.length||new Set(order).size!==order.length)throw new TypeError('chartOrder must be nonempty unique ids');
  const specs=order.map(id=>{const s=TwinChartData.chartSpecs.find(s=>s.id===id);if(!s)throw new TypeError('unknown chart id');return {...s};});
  if(input.chartTitles)for(const [id,title]of Object.entries(input.chartTitles)){const spec=specs.find(s=>s.id===id);if(!spec||typeof title!=='string'||title.length>100)throw new TypeError('invalid chart title');spec.title=title;}
  return {regions,specs};
 }
 function mount(host,options={}){
  if(!host?.querySelector)throw new TypeError('mount requires a dashboard template root');
  const {regions,specs}=configuration(options.config),limits={perRegion:500,total:1500,...options.limits};
  for(const [key,value]of Object.entries(limits))if(!['perRegion','total'].includes(key)||!Number.isSafeInteger(value)||value<1||value>10000)throw new TypeError('limits: positive integers <=10000');
  let state=K.merge(K.emptyData(),options.data||{});const initial=K.clone(state);
  if(options.onCameraOpen!==undefined&&typeof options.onCameraOpen!=='function')throw new TypeError('onCameraOpen must be function');
  const directory=options.storeDirectory===undefined?[]:K.clone(options.storeDirectory);
  if(!Array.isArray(directory)||directory.length>500||directory.some(x=>!x||['id','name','city'].some(k=>typeof x[k]!=='string'||!x[k].trim()||x[k].length>100)||(x.disabled!==undefined&&typeof x.disabled!=='boolean'))||new Set(directory.map(x=>x.id)).size!==directory.length)throw new TypeError('invalid storeDirectory');
  if(options.onStoreSelect!==undefined&&typeof options.onStoreSelect!=='function')throw new TypeError('onStoreSelect must be function');
  const $=s=>host.querySelector(s),all=s=>[...host.querySelectorAll(s)];
  for(const id of ['room-grid','count-controls','camera-dialog','bi-charts','treatment-camera-dock','store-selector'])if(!$(`#${id}`))throw new Error(`missing template #${id}`);
  instances.get(host)?.destroy();
  const abort=new AbortController(),on=(node,event,handler)=>node.addEventListener(event,handler,{signal:abort.signal});
  const timers=new Set(),later=(fn,delay)=>{const t=setTimeout(()=>{timers.delete(t);fn();},delay);timers.add(t);return t;};
  const storeTrigger=$('#store-selector'),storeMenu=$('#store-menu');let storeMenuKey='';
  function placeStoreMenu(){const r=storeTrigger.getBoundingClientRect();storeMenu.style.left=`${Math.max(12,Math.min(r.left,window.innerWidth-storeMenu.offsetWidth-12))}px`;storeMenu.style.top=`${r.bottom+12}px`;storeMenu.style.maxHeight=`${Math.min(480,Math.max(100,window.innerHeight-r.bottom-28))}px`;}
  function toggleStores(open,focus=false){storeMenu.hidden=!open;storeTrigger.setAttribute('aria-expanded',String(open));if(open){placeStoreMenu();if(focus)(storeMenu.querySelector('[aria-current="true"]')||storeMenu.querySelector('button:not(:disabled)'))?.focus();}else if(focus)storeTrigger.focus();}
  function renderStores(){
   $('.store-selected-name').textContent=state.store.name;storeTrigger.title=state.store.name;storeTrigger.setAttribute('aria-label',state.store.name);
   const key=JSON.stringify([state.store.id,state.store.name]);if(key===storeMenuKey)return;storeMenuKey=key;
   const entries=directory.map(x=>({...x}));const current=entries.find(x=>x.id===state.store.id);
   if(current)current.name=state.store.name;else entries.unshift({id:state.store.id,name:state.store.name,city:'当前门店'});
   const groups=new Map();for(const entry of entries){if(!groups.has(entry.city))groups.set(entry.city,[]);groups.get(entry.city).push(entry);}
   const target=$('.store-menu-groups');target.replaceChildren();
   for(const [city,stores]of groups){const row=document.createElement('div');row.className='store-city-row';const label=document.createElement('strong');label.className='store-city';label.textContent=city;const list=document.createElement('div');list.className='store-city-options';
    for(const entry of stores){const button=document.createElement('button');button.type='button';button.className='store-option';button.dataset.storeId=entry.id;button.textContent=entry.name;button.title=entry.name;const active=entry.id===state.store.id;button.setAttribute('aria-current',String(active));button.disabled=!active&&(entry.disabled||!options.onStoreSelect);list.append(button);}row.append(label,list);target.append(row);
   }
   $('.store-menu-note').textContent=directory.some(x=>x.disabled)?'演示目录 · 灰色门店待接入，暂不可切换':'仅展示已配置门店 · 数据由宿主系统提供';
   if(!storeMenu.hidden)placeStoreMenu();
  }
  on(storeTrigger,'click',()=>toggleStores(storeMenu.hidden));
  on(storeTrigger,'keydown',e=>{if(e.key==='ArrowDown'){e.preventDefault();toggleStores(true,true);}});
  on(storeMenu,'click',e=>{const button=e.target.closest('.store-option');if(!button||button.disabled)return;toggleStores(false,true);if(button.dataset.storeId!==state.store.id){try{const result=options.onStoreSelect({...directory.find(x=>x.id===button.dataset.storeId)});Promise.resolve(result).catch(()=>{if(!destroyed)notify('门店切换失败，请重试');});}catch(error){notify('门店切换失败，请重试');}}});
  on(document,'pointerdown',e=>{if(!storeMenu.hidden&&!storeMenu.contains(e.target)&&!storeTrigger.contains(e.target))toggleStores(false);});
  on(document,'focusin',e=>{if(!storeMenu.hidden&&!storeMenu.contains(e.target)&&!storeTrigger.contains(e.target))toggleStores(false);});
  on(host,'keydown',e=>{if(e.key==='Escape'&&!storeMenu.hidden){e.preventDefault();toggleStores(false,true);}});
  on(window,'resize',()=>{if(!storeMenu.hidden)placeStoreMenu();});
  const actors={},staffCounts={},removals={},rendered={},observers=new Map();
  let destroyed=false,toastTimer,selected=null,playerCleanup=null,playerAbort=null,playerGeneration=0,diagnostics={},cameraCache={},lastTrends='',chartCleanup=null;
  const grid=$('#room-grid'),controls=$('#count-controls'),dialog=$('#camera-dialog'),permission=$('#permission'),debug=$('#debug-panel'),debugToggle=$('#debug-toggle'),dock=$('#treatment-camera-dock');
  const placeholder=$('.camera-viewport').innerHTML;
  grid.replaceChildren();controls.replaceChildren();dock.replaceChildren();$('#bi-charts').style.setProperty('--chart-count',TwinCharts.slotCount(specs));
  let notice=$('#kit-notice');if(!notice){notice=document.createElement('div');notice.id='kit-notice';notice.setAttribute('role','status');grid.before(notice);}notice.hidden=true;
  const value=x=>x===null?'—':String(x);
  for(const metric of all('.store-metric')){metric.querySelector('dd>span')?.classList.add('metric-unit');if(!metric.querySelector('.metric-stale-dot')){const dot=document.createElement('i');dot.className='metric-stale-dot';dot.setAttribute('aria-hidden','true');metric.querySelector('dd')?.append(dot);}}
  function setMeasurement(strong,x,stale){const metric=strong.closest('.room-metric,.store-metric');strong.textContent=value(x);metric.classList.toggle('is-missing',x===null);metric.classList.toggle('is-stale',stale);metric.title=stale?(x===null?'本次未获取到数据':'本次未获取到数据，暂时显示上次结果'):'';}
  function alive(){if(destroyed)throw new Error('dashboard instance has been destroyed');}
  function notify(message){const toast=$('#toast');toast.textContent=message;toast.classList.add('visible');clearTimeout(toastTimer);toastTimer=later(()=>toast.classList.remove('visible'),3000);}
  function setDebug(open){debug.hidden=!open;debugToggle.setAttribute('aria-expanded',String(open));if(!open)debugToggle.focus();}
  function clearPlayer(){playerGeneration++;playerAbort?.abort();playerAbort=null;if(playerCleanup){try{playerCleanup();}catch(e){console.error('camera cleanup failed',e);}playerCleanup=null;}$('.camera-viewport').replaceChildren();}
  function closeCamera(){clearPlayer();if(dialog.open)dialog.close();const old=selected;selected=null;if(old?.trigger?.isConnected&&!old.trigger.disabled)old.trigger.focus();delete dialog.dataset.area;delete dialog.dataset.camera;}
  function openCamera(id,cameraId,trigger){
   const camera=state.regions[id].cameras.find(c=>c.id===cameraId);if(!camera||!camera.canView||!state.permissions.canViewCameras)return;
   closeCamera();selected={id,cameraId,trigger};$('#camera-title').textContent=`${state.store.name} · ${camera.name}`;dialog.dataset.area=id;dialog.dataset.camera=camera.id;
   const container=$('.camera-viewport');container.setAttribute('aria-label','直播画面占位，未接入摄像头');container.innerHTML=placeholder;if(!options.externalCameraDialog)dialog.showModal();
   if(options.onCameraOpen){container.replaceChildren();container.removeAttribute('aria-label');const generation=playerGeneration;playerAbort=new AbortController();const surface=document.createElement('div');surface.className='camera-player-surface';container.append(surface);
    try{Promise.resolve(options.onCameraOpen({store:K.clone(state.store),regionId:id,camera:K.clone(camera),container:surface,signal:playerAbort.signal})).then(cleanup=>{
     if(cleanup!==undefined&&typeof cleanup!=='function')throw new TypeError('onCameraOpen must return cleanup function or undefined');
     if(destroyed||generation!==playerGeneration){if(cleanup)cleanup();return;}playerCleanup=cleanup||null;
    }).catch(error=>{if(generation===playerGeneration&&!destroyed){container.textContent='摄像头加载失败，请重试';console.error(error);}});}catch(error){container.textContent='摄像头加载失败，请重试';console.error(error);}
   }
  }
  for(const r of regions){
   actors[r.id]=new Map();removals[r.id]=new Map();rendered[r.id]=0;
   const card=document.createElement('section');card.id=`room-${r.id}`;card.className='room-card';
   const segmentMarkup=(r.segmentMetrics||[]).map(([key,label])=>`<span class="room-metric room-segment" data-metric="${key}">${label} <strong id="${key}-${r.id}">—</strong> <span class="metric-unit">人</span><i class="metric-stale-dot" aria-hidden="true"></i></span>`).join('');
   card.innerHTML=`<div class="room-title-row"><span class="room-heading-copy"><span class="room-name"></span><span class="room-en"></span></span><span class="room-metrics"><span class="room-metric room-count">${r.state} <strong id="number-${r.id}">—</strong> <span class="metric-unit">人</span><i class="metric-stale-dot" aria-hidden="true"></i></span>${r.cumulativeLabel?`<span class="room-metric room-cumulative">${r.cumulativeLabel} <strong id="cumulative-${r.id}">—</strong> <span class="metric-unit">人</span><i class="metric-stale-dot" aria-hidden="true"></i></span>`:''}${segmentMarkup}</span></div>${TwinScenes.render(r.id)}<div class="room-bottom"><span></span></div>`;
   card.querySelector('.room-name').textContent=r.name;card.querySelector('.room-en').textContent=r.en;card.querySelector('.room-bottom span').textContent=r.caption;grid.append(card);
   const row=document.createElement('div');row.className='count-row';row.innerHTML=`<span class="count-row-label"></span><div class="stepper"><button id="minus-${r.id}" type="button">−</button><output id="output-${r.id}" aria-live="polite">—</output><button id="plus-${r.id}" type="button">+</button><button class="plus-ten" id="plus-ten-${r.id}" type="button">+10</button></div>`;row.querySelector('.count-row-label').textContent=r.name;controls.append(row);
   for(const [prefix,delta]of [['minus',-1],['plus',1],['plus-ten',10]]){const node=$(`#${prefix}-${r.id}`);node.setAttribute('aria-label',`${r.name}${delta<0?'减少':'增加'}${Math.abs(delta)}人`);on(node,'click',()=>{if(state.mode==='demo')update({regions:{[r.id]:{current:TwinModel.normalizeCount((state.regions[r.id].current||0)+delta,TwinModel.MAX_DEMO_COUNT)}}});});}
  }
  function clearActors(id){for(const t of removals[id].values()){clearTimeout(t);timers.delete(t);}removals[id].clear();actors[id].clear();$(`#room-${id} .guest-layer`).replaceChildren();rendered[id]=0;}
  function drawGuests(id,next,instant=false){
   const previous=rendered[id],diff=TwinModel.slotDiff(previous,next);rendered[id]=next;
   for(const i of diff.exit){const node=actors[id].get(i);if(!node)continue;node.classList.add('leaving');const timer=later(()=>{if(i>=rendered[id]){node.remove();actors[id].delete(i);}removals[id].delete(i);},650);removals[id].set(i,timer);}
   const fragment=document.createDocumentFragment();for(const i of diff.enter){if(removals[id].has(i)){const timer=removals[id].get(i);clearTimeout(timer);timers.delete(timer);removals[id].delete(i);actors[id].get(i).classList.remove('leaving');continue;}
    const node=TwinScenes.person(TwinScenes.guestSlot(id,i),false,i,id);node.dataset.slot=i;if(!instant)node.classList.add('entering');actors[id].set(i,node);fragment.append(node);if(!instant)later(()=>node.classList.remove('entering'),32);
   }$(`#room-${id} .guest-layer`).append(fragment);
  }
  function drawStaff(id,count){const n=count!==null&&count<=100?count:0;if(staffCounts[id]===n)return;staffCounts[id]=n;const layer=$(`#room-${id} .employee-layer`);layer.replaceChildren();const slots=TwinScenes.employees[id];for(let i=0;i<n;i++){const base=slots[i%Math.max(1,slots.length)]||TwinScenes.guestSlot(id,0);const node=TwinScenes.person({...base},true,i,id);layer.append(node);}}
  function cameraPaging(id,list,nav,items){
   let page=0,capacity=20,lastWidth=-1,active=true;
   const prev=document.createElement('button'),next=document.createElement('button'),label=document.createElement('span');prev.type=next.type='button';prev.textContent='‹';next.textContent='›';prev.setAttribute('aria-label',`${regions.find(r=>r.id===id).name}摄像头上一页`);next.setAttribute('aria-label',`${regions.find(r=>r.id===id).name}摄像头下一页`);label.setAttribute('aria-live','polite');nav.append(prev,label,next);
   function render(){const pages=Math.max(1,Math.ceil(items.length/capacity));page=Math.max(0,Math.min(page,pages-1));items.forEach((item,i)=>item.hidden=i<page*capacity||i>=(page+1)*capacity);nav.hidden=pages<=1;prev.disabled=page===0;next.disabled=page===pages-1;label.textContent=`${page+1} / ${pages}`;}
   function layout(){if(!active||destroyed)return;const width=list.clientWidth;if(width===lastWidth)return;lastWidth=width;items.forEach(item=>item.hidden=false);const maxWidth=Math.max(76,...items.map(item=>item.getBoundingClientRect().width));const columns=Math.max(1,Math.floor((width+6)/(maxWidth+6)));capacity=columns*2;list.style.gridTemplateColumns=`repeat(${columns},max-content)`;render();}
   prev.addEventListener('click',()=>{page--;render();});next.addEventListener('click',()=>{page++;render();});const observer=new ResizeObserver(layout);observer.observe(list);layout();document.fonts.ready.then(()=>{if(active){lastWidth=-1;layout();}});observers.set(id,()=>{active=false;observer.disconnect();});
  }
  function drawCameras(r){
   const data=state.regions[r.id].cameras,cache=JSON.stringify([data,state.permissions.canViewCameras]);if(cameraCache[r.id]===cache)return;cameraCache[r.id]=cache;
   if(selected?.id===r.id)closeCamera();observers.get(r.id)?.();observers.delete(r.id);
   const parent=r.id==='treatment'?dock:$(`#room-${r.id}`);parent.querySelector('.region-cameras')?.remove();parent.querySelector('.dock-pagination')?.remove();
   const list=document.createElement('div');list.className='region-cameras';list.setAttribute('aria-label',`${r.name}摄像头`);list.style.setProperty('--camera-rows',Math.max(1,Math.min(6,data.length)));list.hidden=!data.length;
   for(const c of data){const button=document.createElement('button');button.type='button';button.className='camera-trigger';button.dataset.regionId=r.id;button.dataset.cameraId=c.id;button.disabled=!state.permissions.canViewCameras||!c.canView;button.setAttribute('aria-haspopup','dialog');button.setAttribute('aria-controls','camera-dialog');const icon=document.createElement('span');icon.className='camera-symbol';icon.setAttribute('aria-hidden','true');icon.textContent=button.disabled?'🔒':'▣';const label=document.createElement('span');label.className='camera-label';label.textContent=c.name;button.append(icon,label);
    const status=c.occupied===null?'占用状态未知':c.occupied?'占用中':'空闲';if(c.occupied!==null){button.dataset.occupancy=c.occupied?'occupied':'idle';button.classList.toggle('is-occupied',c.occupied);}button.setAttribute('aria-label',`${c.name}，${status}`);button.title=`${c.name} · ${status}${button.disabled?' · 暂无监控查看权限':''}`;list.append(button);
   }
   if(r.id==='treatment'){dock.append(list);dock.hidden=!data.length;const nav=document.createElement('div');nav.className='dock-pagination';nav.hidden=true;dock.prepend(nav);cameraPaging(r.id,list,nav,[...list.children]);}
   else {parent.querySelector('.room-art').before(list);if(data.length>6){list.classList.add('camera-list-overflow');list.style.gridTemplateRows='repeat(6,24px)';}}
  }
  function render(instant=false){
   const plan=K.renderPlan(state,limits);diagnostics={limits:K.clone(limits),regions:K.clone(plan),mode:state.mode,revision:state.revision};
   const warnings=[];
   for(const r of regions){const d=state.regions[r.id],stale=new Set(d.staleFields);setMeasurement($(`#number-${r.id}`),d.current,stale.has('current'));if(r.cumulativeLabel)setMeasurement($(`#cumulative-${r.id}`),d.cumulative,stale.has('cumulative'));for(const [key]of(r.segmentMetrics||[]))setMeasurement($(`#${key}-${r.id}`),d[key],stale.has(key));$(`#output-${r.id}`).textContent=value(d.current);$(`#room-${r.id}`).setAttribute('aria-label',`${r.name}，${r.state}${value(d.current)}人`);
    $(`#minus-${r.id}`).disabled=state.mode!=='demo'||!d.current;$(`#plus-${r.id}`).disabled=$(`#plus-ten-${r.id}`).disabled=state.mode!=='demo'||(d.current||0)>=TwinModel.MAX_DEMO_COUNT;
    if(plan[r.id].reason){clearActors(r.id);if(d.current!==null)warnings.push(`${r.name}${d.current}人：超出人物渲染保护范围，已暂停该区人物图示`);}else drawGuests(r.id,plan[r.id].rendered,instant);
    drawStaff(r.id,d.staff);if(d.staff>100)warnings.push(`${r.name}员工${d.staff}人：超出100人保护范围，暂停员工图示`);drawCameras(r);
   }
   $('#room-consultation').classList.toggle('is-consulting',plan.consultation.rendered>0);
   notice.textContent=warnings.join('；');notice.hidden=!warnings.length;
   const hasUnknown=regions.some(r=>state.regions[r.id].current===null);$('#total-count').textContent=hasUnknown?'—':regions.reduce((n,r)=>n+BigInt(state.regions[r.id].current),0n).toString();
   renderStores();
   $('#store-selector-help').textContent='由宿主通过replace输入完整门店快照切换门店。';$('#experiment-store-count').textContent=value(state.store.experimentStoreCount);$('.account-name').textContent=state.store.userName;
   for(const [i,k]of K.overviewKeys.entries())setMeasurement(all('.store-metric strong')[i],state.overview[k],state.staleOverview.includes(k));
   const demo=state.mode==='demo';$('.account-copy small').textContent=demo?'示例账号 · 未接入登录':'宿主提供 · 身份未校验';all('.experiment-stores small,.store-picker .sample-tag').forEach(n=>n.textContent=demo?'示例':'输入');
   $('.scene-footer span').textContent=demo?'实时场景为模拟数据 · 人物代表人数，不代表真实位置':'输入数据 · 人物代表区域人数，不代表真实位置';
   $('.bi-heading .sample-tag').textContent=demo?'模拟数据':'输入数据';$('.bi-heading>span').textContent=state.trends.length?`${state.trends.length}天 / ${state.trends[0].date} — ${state.trends.at(-1).date}`:'暂无趋势数据';
   $('.bi-disclaimer').textContent=`${demo?'合成样例 · 业务口径待确认':'指标直接展示输入值 · 来源由宿主负责'}${state.updatedAt?' · 数据时间 '+state.updatedAt:''}`;
   $('#permission').checked=state.permissions.canViewCameras;$('#permission').disabled=!demo;
   for(const id of ['peak-demo','dense-demo'])$(`#${id}`).disabled=!demo;
   $('#reset').disabled=!demo;$('#reset').textContent=`示例人数 · ${regions.reduce((n,r)=>n+BigInt(initial.regions[r.id].current||0),0n)}`;
   if(options.externalCameraDialog){$('.account-copy small').textContent='已登录二壮';all('.experiment-stores small,.store-picker .sample-tag').forEach(n=>n.textContent='已开放');$('.scene-footer span').textContent='人数与运营图表为演示数据 · 摄像头来自真实门店';$('#store-selector-help').textContent='选择已开放且有权限的机构';}
   const serialized=JSON.stringify([state.trends,state.mode]);if(lastTrends!==serialized){chartCleanup?.();chartCleanup=TwinCharts.render($('#bi-charts'),state.trends,{mode:state.mode,specs,rotationMs:7000});lastTrends=serialized;}
  }
  function commit(next,instant=false){const previous=state;state=next;try{render(instant);}catch(error){state=previous;throw error;}return {applied:true,revision:state.revision,diagnostics:K.clone(diagnostics)};}
  function update(patch){alive();const next=K.merge(state,patch);if(next.store.id!==state.store.id)throw new Error('Switch stores with replace(fullSnapshot), not update');if(Object.hasOwn(patch,'revision')&&next.revision<=state.revision)return {applied:false,reason:'stale',revision:state.revision};return commit(next);}
  function replace(snapshot){alive();const next=K.merge(K.emptyData(),snapshot);closeCamera();for(const r of regions)clearActors(r.id);return commit(next,true);}
  on(debugToggle,'click',()=>setDebug(debug.hidden));on($('#debug-close'),'click',()=>setDebug(false));
  on(host,'keydown',e=>{if(e.key==='Escape'&&!dialog.open&&!debug.hidden)setDebug(false);});
  on($('#close-dialog'),'click',closeCamera);on(dialog,'cancel',e=>{e.preventDefault();closeCamera();});on(dialog,'close',()=>{if(selected&&!dialog.open)closeCamera();});
  on(dialog,'click',e=>{if(e.target===dialog){const r=dialog.getBoundingClientRect();if(e.clientX<r.left||e.clientX>r.right||e.clientY<r.top||e.clientY>r.bottom)closeCamera();}});
  for(const parent of [grid,dock])on(parent,'click',e=>{const button=e.target.closest('.camera-trigger');if(button&&!button.disabled)openCamera(button.dataset.regionId,button.dataset.cameraId,button);});
  on(permission,'change',()=>{if(state.mode==='demo'){closeCamera();update({permissions:{canViewCameras:permission.checked}});}});
  on($('#reset'),'click',()=>{if(state.mode==='demo'){update({regions:Object.fromEntries(regions.map(r=>[r.id,{current:initial.regions[r.id].current}]))});notify('已恢复初始演示人数');}});
  on($('#dense-demo'),'click',()=>{if(state.mode==='demo'){update({regions:Object.fromEntries(regions.map(r=>[r.id,{current:60}]))});notify('密集演示：每区60人');}});
  on($('#peak-demo'),'click',()=>{if(state.mode==='demo'){const peak=[18,12,16,14,30];update({regions:Object.fromEntries(regions.map((r,i)=>[r.id,{current:peak[i]}]))});notify('高峰演示：90人');}});
  function destroy(){if(destroyed)return;destroyed=true;toggleStores(false);closeCamera();chartCleanup?.();chartCleanup=null;abort.abort();for(const t of timers)clearTimeout(t);timers.clear();for(const stop of observers.values())stop();observers.clear();grid.replaceChildren();controls.replaceChildren();dock.replaceChildren();$('#bi-charts').replaceChildren();notice.hidden=true;instances.delete(host);}
  const api={update,replace,getState:()=>K.clone(state),getDiagnostics:()=>K.clone(diagnostics),destroy};render(true);instances.set(host,api);return api;
 }
 root.TwinDashboard={mount,createEmptyData:TwinKitCore.emptyData,version:'1.2.0',regionIds:[...TwinKitCore.regionIds]};
})(window);
