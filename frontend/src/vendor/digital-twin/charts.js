/* Reusable, transport-independent SVG trend renderer. Inputs validated by TwinKitCore. */
(function(root){
 const X0=45,X1=548,Y0=14,Y1=111,WIDTH=560,HEIGHT=140;
 const esc=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 function render(container,data,{mode='external',specs=TwinChartData.chartSpecs}={}){
  container.replaceChildren();const length=data.length,last=length-1;
  const x=i=>length<=1?(X0+X1)/2:X0+(X1-X0)*i/last;
  for(const [chartIndex,spec]of specs.entries()){
   let maximum=0,hasValue=false;
   for(const d of data){let total=0;for(const s of spec.series){const v=d[s.key];if(v!==null){hasValue=true;maximum=Math.max(maximum,v);total+=v;}}if(spec.type==='stacked')maximum=Math.max(maximum,total);}
   const raw=Math.max(spec.max,maximum*1.08,1),power=10**Math.floor(Math.log10(raw));
   const max=maximum<=spec.max?spec.max:Math.ceil(raw/power*2)/2*power;
   const y=v=>Y1-v/max*(Y1-Y0),axis=v=>v>=1e6?`${+(v/1e6).toFixed(1)}M`:v>=1e4?`${+(v/1e3).toFixed(1)}k`:String(v);
   const card=document.createElement('section');card.className='chart-card';card.id=`chart-${spec.id}`;card.setAttribute('aria-labelledby',`chart-title-${spec.id}`);card.dataset.axisMax=max;
   const legend=spec.series.map(s=>`<span><i style="--series-color:${s.color}" class="${spec.type==='stacked'?'bar-key':'line-key'}"></i>${esc(s.label)}</span>`).join('');
   let shapes='';for(let i=0;i<3;i++){const value=max*i/2,cy=y(value);shapes+=`<line x1="${X0-4}" y1="${cy}" x2="${X1+6}" y2="${cy}" stroke="#1b3a30" stroke-width=".7" stroke-dasharray="3 5"/><text x="${X0-10}" y="${cy+4}" text-anchor="end" fill="#688b7e" font-size="11">${axis(value)}</text>`;}
   const ticks=[...new Set(Array.from({length:Math.min(7,length)},(_,i)=>Math.round(i*last/Math.max(1,Math.min(7,length)-1))))];
   for(const i of ticks)shapes+=`<text x="${x(i)}" y="132" text-anchor="middle" fill="#718f82" font-size="11">${esc(data[i].date.slice(5).replace('-','/'))}</text>`;
   if(spec.type==='stacked'){
    const barWidth=Math.min(9.4,(X1-X0)/Math.max(1,length)*.65);
    data.forEach((d,i)=>{let bottom=0;for(const s of spec.series){const v=d[s.key];if(v===null)continue;shapes+=`<rect class="data-bar" data-index="${i}" x="${x(i)-barWidth/2}" y="${y(bottom+v)}" width="${barWidth}" height="${v/max*(Y1-Y0)}" rx="1" fill="${s.color}" opacity=".88"/>`;bottom+=v;}});
   }else{
    for(const s of spec.series){let points=[],segments=[];const flush=()=>{if(points.length)segments.push(points);points=[];};data.forEach((d,i)=>{if(d[s.key]===null)flush();else points.push([x(i),y(d[s.key])]);});flush();
     shapes+=`<g class="data-series" data-key="${s.key}">`;
     for(const points of segments)shapes+=points.length===1?`<circle cx="${points[0][0]}" cy="${points[0][1]}" r="2.5" fill="${s.color}"/>`:`<polyline points="${points.map(p=>p.join(',')).join(' ')}" fill="none" stroke="${s.color}" stroke-width="${s.key.endsWith('All')?1.7:2.2}" stroke-linejoin="round" stroke-linecap="round"/>`;
     if(length&&data[last][s.key]!==null)shapes+=`<circle cx="${x(last)}" cy="${y(data[last][s.key])}" r="2.5" fill="${s.color}"/>`;shapes+='</g>';
    }
   }
   const source=mode==='demo'?'模拟数据':'输入数据';
   card.innerHTML=`<div class="chart-heading"><div><span class="chart-index">${String(chartIndex+1).padStart(2,'0')}</span><h3 id="chart-title-${spec.id}">${esc(spec.title)}</h3></div><span class="chart-unit">${esc(spec.unit)}</span></div><div class="chart-legend">${legend}<span class="chart-time">${length}天</span></div><div class="chart-canvas"><svg class="trend-svg" viewBox="0 0 ${WIDTH} ${HEIGHT}" preserveAspectRatio="none" role="img" tabindex="0" aria-label="${esc(spec.title)}，${length}天${source}；左右方向键查看每日数值"><title>${esc(spec.title)}：${source}</title>${shapes}<line class="chart-crosshair" x1="0" y1="${Y0}" x2="0" y2="${Y1}" stroke="#a4c9b2" stroke-width="1" stroke-dasharray="3 3" visibility="hidden"/></svg><div class="chart-tooltip" hidden></div>${hasValue?'':'<div class="chart-empty">暂无数据</div>'}</div>`;
   container.append(card);const svg=card.querySelector('svg'),tooltip=card.querySelector('.chart-tooltip'),crosshair=card.querySelector('.chart-crosshair');let current=Math.max(0,last);
   function show(index){if(!length)return;current=Math.max(0,Math.min(last,index));const d=data[current];tooltip.replaceChildren();const title=document.createElement('strong');title.textContent=`${d.date.replaceAll('-','.')} · ${mode==='demo'?'模拟':'输入数据'}`;tooltip.append(title);
    for(const s of spec.series){const row=document.createElement('span'),label=document.createElement('span'),value=document.createElement('b');label.textContent=s.label;label.style.color=s.color;value.textContent=d[s.key]===null?'—':`${d[s.key]} ${spec.unit}`;row.append(label,value);tooltip.append(row);}
    tooltip.hidden=false;tooltip.style.left=current>last/2?'8%':'auto';tooltip.style.right=current>last/2?'auto':'3%';crosshair.setAttribute('x1',x(current));crosshair.setAttribute('x2',x(current));crosshair.setAttribute('visibility','visible');svg.dataset.activeIndex=current;
   }
   function hide(){tooltip.hidden=true;crosshair.setAttribute('visibility','hidden');}
   svg.addEventListener('pointermove',e=>{const bounds=svg.getBoundingClientRect();show(Math.round(((e.clientX-bounds.left)/bounds.width*WIDTH-X0)/(X1-X0)*Math.max(0,last)));});
   svg.addEventListener('pointerleave',()=>{if(document.activeElement!==svg)hide();});svg.addEventListener('focus',()=>show(current));svg.addEventListener('blur',hide);svg.addEventListener('keydown',e=>{if(e.key==='ArrowLeft'||e.key==='ArrowRight'){e.preventDefault();show(current+(e.key==='ArrowRight'?1:-1));}else if(e.key==='Escape')hide();});
  }
 }
 root.TwinCharts={render};
})(window);
