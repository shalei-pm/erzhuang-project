/* Original, dependency-free SVG room drawings. No third-party models or assets. */
(function(root){
 const NS='http://www.w3.org/2000/svg';
 const p=(x,y,z=0)=>[230+(x-y)*.9,116+(x+y)*.46-z];
 const pt=(a)=>a.map(n=>Math.round(n*100)/100).join(',');
 const poly=(points,fill,stroke='none',sw=.6)=>`<polygon points="${points.map(pt).join(' ')}" fill="${fill}" stroke="${stroke}" stroke-width="${sw}" stroke-linejoin="round"/>`;
 const line=(a,b,color,width=1)=>`<line x1="${a[0]}" y1="${a[1]}" x2="${b[0]}" y2="${b[1]}" stroke="${color}" stroke-width="${width}" stroke-linecap="round"/>`;
 const plane=(x,y,w,d,z,fill,stroke)=>poly([p(x,y,z),p(x+w,y,z),p(x+w,y+d,z),p(x,y+d,z)],fill,stroke);
 function box(x,y,w,d,h,colors=['#edf2e9','#c6d8cf','#abbfb7'],z=0){
  return poly([p(x,y+d,z),p(x+w,y+d,z),p(x+w,y+d,z+h),p(x,y+d,z+h)],colors[1])+
   poly([p(x+w,y,z),p(x+w,y+d,z),p(x+w,y+d,z+h),p(x+w,y,z+h)],colors[2])+
   plane(x,y,w,d,z+h,colors[0]);
 }
 function pot(x,y,scale=1){
  const [a,b]=p(x,y,19);return box(x-7,y-7,14,14,17,['#ebeadd','#b9c8be','#95aca2'])+`<g transform="translate(${a} ${b}) scale(${scale})"><path d="M0 0 Q-3 -15 2 -36 M0 0 Q-10 -17 -16 -19 M0 0 Q7 -20 16 -25" stroke="#3e6d55" fill="none" stroke-width="2"/><ellipse cx="-10" cy="-23" rx="5" ry="13" transform="rotate(-45 -10 -23)" fill="#679b72"/><ellipse cx="12" cy="-28" rx="5" ry="14" transform="rotate(40 12 -28)" fill="#8bb48a"/><ellipse cx="1" cy="-35" rx="5" ry="13" fill="#7ca87b"/><ellipse cx="-9" cy="-8" rx="4" ry="10" transform="rotate(-55 -9 -8)" fill="#568b64"/></g>`;
 }
 function screen(x,y,w=24,h=19,z=34){
  const wall=(inset,fill)=>poly([p(x+inset,y,z+inset),p(x+w-inset,y,z+inset),p(x+w-inset,y,z+h-inset),p(x+inset,y,z+h-inset)],fill);
  return box(x+w/2-2,y-2,4,9,3,['#819b93','#526d65','#526d65'],z-8)+line(p(x+w/2,y,z-6),p(x+w/2,y,z),'#536f64',2)+wall(0,'#456158')+wall(2,'#203d39')+line(p(x+5,y,z+h-6),p(x+w-5,y,z+h-6),'#83c7b1',1.2)+line(p(x+5,y,z+h-10),p(x+w-10,y,z+h-10),'#527e6d',1);
 }
 function advertisement(x,y){
  let s=box(x-3,y-3,33,14,3,['#a9beb0','#718e7e','#6c8877']);
  s+=poly([p(x,y,4),p(x+26,y,4),p(x+26,y,73),p(x,y,73)],'#d6e2d1');
  s+=poly([p(x+2,y,8),p(x+24,y,8),p(x+24,y,70),p(x+2,y,70)],'#234c3c');
  const [a,b]=p(x+12,y,51);s+=`<ellipse cx="${a}" cy="${b}" rx="6" ry="11" fill="#92bca0"/><circle cx="${a+1}" cy="${b-6}" r="4" fill="#dce6cb"/>`;
  s+=line(p(x+5,y,27),p(x+21,y,27),'#e1edd8',1.2)+line(p(x+7,y,21),p(x+18,y,21),'#94b6a0',1);return s;
 }
 function chair(x,y,back=true){
  let s=box(x,y,20,20,14,['#a3bfb0','#77988a','#6b897d']);
  if(back)s+=box(x,y,20,4,19,['#c5d7c8','#8ead9b','#718e81'],14);
  s+=line(p(x+3,y+17,0),p(x+3,y+17,9),'#5c7568',1.5)+line(p(x+18,y+17,0),p(x+18,y+17,9),'#5c7568',1.5);return s;
 }
 function bed(x,y){
  return box(x+6,y+5,21,45,19,['#becfc7','#9bb3a8','#8fa99c'])+box(x,y,34,58,7,['#f5f4e9','#d5e0d3','#c0d0c2'],19)+box(x+2,y+2,30,12,4,['#fffdf0','#dbe1d2','#cbd7c6'],26)+plane(x+2,y+25,30,29,27,'#d0e0d3')+line(p(x+3,y+57,25),p(x+31,y+57,25),'#a5bcaf',1);
 }
 function recliner(x,y){
  let s='<g class="recliner">';
  s+=box(x+5,y+17,25,25,10,['#9eafa1','#7e9383','#6f8777']);
  // Angled backrest rises above the seat; the footrest slopes gently down.
  s+=poly([p(x,y,43),p(x+34,y,43),p(x+34,y+24,20),p(x,y+24,20)],'#ebdfc7','#c4b99e');
  s+=poly([p(x,y,43),p(x,y+24,20),p(x,y+28,16),p(x,y,37)],'#c2b49a');
  s+=box(x,y+24,34,19,6,['#e8dbc1','#cdbd9f','#b6a98e'],14);
  s+=poly([p(x,y+43,20),p(x+34,y+43,20),p(x+34,y+64,10),p(x,y+64,10)],'#e3d3b6','#b8aa8e');
  s+=box(x-4,y+19,5,27,6,['#f0e5d0','#cbbca1','#aa9e83'],22)+box(x+33,y+19,5,27,6,['#f0e5d0','#cbbca1','#aa9e83'],22);
  s+=poly([p(x+5,y+3,42),p(x+29,y+3,42),p(x+29,y+11,35),p(x+5,y+11,35)],'#f7f3e7');
  return s+'</g>';
 }
 function machine(x,y){
  return box(x,y,17,19,28,['#e5ede0','#b4ccc0','#9bb7aa'])+screen(x+1,y+4,15,13,30)+`<path d="M${pt(p(x+17,y+5,28))} Q${pt(p(x+33,y+5,20))} ${pt(p(x+25,y+20,7))}" stroke="#739489" fill="none" stroke-width="1.5"/>`;
 }
 function base(id){
  let s=`<defs><filter id="shadow-${id}" x="-40%" y="-50%" width="180%" height="210%"><feGaussianBlur stdDeviation="8"/></filter></defs>`;
  s+=plane(-4,-3,212,194,-9,'#020b0a');
  s+=`<g opacity=".25" filter="url(#shadow-${id})">${plane(-4,-3,212,194,-11,'#030b08')}</g>`;
  s+=box(-4,-4,208,188,7,['#d8e0d2','#a1b7a5','#859e8c'],-7);
  s+=plane(0,0,200,180,0,'#e3e8dc');
  for(let x=40;x<200;x+=40)s+=line(p(x,0),p(x,180),'#ccd7c9',.7);
  for(let y=36;y<180;y+=36)s+=line(p(0,y),p(200,y),'#ccd7c9',.7);
  if(id==='waiting')return s;
  s+='<g class="room-walls">';
  s+=box(0,0,200,4,92,['#ecf0e4','#cbd9cd','#b3c7b8']);
  s+=box(0,4,4,176,92,['#f0f2e8','#d9e1d3','#c1d1c0']);
  s+=line(p(4,4,4),p(200,4,4),'#afc1b1',2)+line(p(4,4,4),p(4,180,4),'#b5c4b4',2);
  // Recessed back-wall light; consistent room shell for all four functions.
  s+=line(p(17,4,83),p(186,4,83),'#f7faeb',2.8);
  s+='</g>';
  return s;
 }
 function wallSign(text){const [x,y]=p(78,4,55);return `<text transform="matrix(.9 .46 0 1 ${x} ${y})" font-size="10" letter-spacing="2" font-family="Arial,sans-serif" fill="#6a8b73">${text}</text>`;}
 function fixed(id){
  let s=base(id);
  if(id==='reception'){
   s+=wallSign('SOYOUNG');
   s+=poly([p(4,28,35),p(4,86,35),p(4,86,73),p(4,28,73)],'#bacdbb');
   s+=poly([p(4.1,31,38),p(4.1,83,38),p(4.1,83,70),p(4.1,31,70)],'#244f3c');
   s+=line(p(4.2,42,56),p(4.2,72,56),'#d8e4c7',2)+line(p(4.2,48,48),p(4.2,66,48),'#8bb997',1.4);
   s+=pot(174,20,.8);
   s+=box(43,65,111,34,38,['#ede7d6','#cfbf9f','#b3a88d']);
   for(let x=47;x<150;x+=6)s+=line(p(x,99,4),p(x,99,33),'#bbab8d',.7);
   s+=box(40,63,117,38,4,['#f8f6e9','#e2ddca','#cfc8b2'],38);
   s+=screen(71,77,25,19,42)+plane(70,86,26,8,43,'#849b8d');
   s+=box(129,81,14,9,1,['#819e86','#7c967c','#6e8972'],43);
   s+=advertisement(160,120)+pot(20,145,.9);
  }else if(id==='consultation'){
   s+=wallSign('CONSULT');
   s+=poly([p(4,22,29),p(4,72,29),p(4,72,74),p(4,22,74)],'#f1eee0');
   s+=poly([p(4.1,27,35),p(4.1,67,35),p(4.1,67,68),p(4.1,27,68)],'#b5c4aa');
   s+=line(p(4.2,46,37),p(4.2,46,65),'#e9ecdb',1.5);
   s+=box(138,12,45,18,25,['#edf0e0','#c5d3be','#a8bfab'])+pot(170,21,.55);
   s+=chair(78,40)+chair(39,101)+chair(77,123)+chair(134,111)+chair(141,67);
   s+=box(63,72,77,40,29,['#8d9f82','#9db297','#91a58a']);
   s+=box(60,68,84,46,4,['#eee3ca','#d6caac','#c3b596'],29);
   s+=screen(98,76,24,18,33)+plane(95,86,22,8,34,'#829a88');
   s+=plane(71,88,13,15,34,'#fcfcf2')+line(p(73,91,35),p(81,91,35),'#a3b9a5',.8);
   s+=pot(20,146,.95);
  }else if(id==='treatment'){
   s+=wallSign('TREATMENT');
   // Suspended monitor arm, medical console and preparation tables.
   s+=line(p(161,5,91),p(161,42,91),'#718f83',3)+line(p(161,42,91),p(161,42,68),'#718f83',2.5);
   s+=screen(145,42,29,19,52);
   s+=box(13,17,30,19,29,['#f3f3e7','#cbdace','#b2c7ba']);
   s+=box(17,20,8,6,6,['#b9d6d1','#8eaca0','#819e91'],29)+box(29,20,5,5,9,['#eee8d4','#bbbcaa','#aaa78e'],29);
   s+=machine(91,33)+machine(178,120);
   for(const [x,y] of [[45,28],[132,63],[35,112],[120,127]])s+=bed(x,y);
   s+=box(90,101,22,17,22,['#e9eee1','#a8c4b2','#91ac9b'])+plane(92,103,15,10,23,'#8aaf9c');
  }else if(id==='aftercare'){
   s+=wallSign('AFTERCARE');
   s+=box(20,12,73,19,25,['#eee9dc','#c9cfbc','#a8b9a5']);
   s+=box(27,18,13,8,4,['#f9f8ed','#d6dccd','#bfccba'],25);
   s+=box(46,18,7,7,7,['#bfd4bb','#9eb899','#87a483'],25);
   s+=box(60,18,7,7,9,['#f1e2c6','#cbbba1','#b7ac91'],25);
   s+=pot(175,20,.8);
   for(const [x,y] of [[38,37],[124,45],[32,117],[120,124]])s+=recliner(x,y);
   s+=box(90,91,20,19,23,['#f0e5d0','#c6bda6','#aea68d']);
   s+=plane(92,94,14,12,24,'#f8f8e9')+box(94,95,7,5,2,['#deebe0','#afc6b2','#9db39d'],24);
  }else{

   // Three seats on the back sofa and two at left, all available as stable actor slots.
   s+=box(30,24,107,31,15,['#99b7a1','#7c9e88','#668b75']);
   s+=box(30,24,107,6,24,['#b9cdb5','#90ae94','#74917b'],15);
   for(let x=35;x<130;x+=33)s+=box(x,32,29,20,4,['#b2c6ac','#9db79b','#8eaa8c'],15);
   s+=box(26,83,27,65,16,['#a8bfaa','#849f88','#75917a']);
   s+=box(26,83,6,65,21,['#bdcfb7','#98b096','#7f9d84'],16);
   s+=box(33,87,18,25,4,['#b9cdb2','#9fb797','#8aa583'],16)+box(33,117,18,26,4,['#b9cdb2','#9fb797','#8aa583'],16);
   s+=plane(65,73,82,76,.5,'#cbd4be');
   s+=box(77,83,51,35,20,['#d9c9a8','#b6a78a','#a3987c'])+box(75,81,55,39,3,['#f0e7d0','#d8cdaf','#cabe9e'],20);
   s+=plane(81,94,16,14,24,'#eeeede')+box(107,89,5,5,6,['#f2f4e7','#c5d1bd','#b2c3ab'],24);
   s+=pot(158,25,1.1)+advertisement(157,99)+pot(161,163,.65);
  }
  return s;
 }
 const guestSlots={
  reception:[{x:73,y:130},{x:108,y:142},{x:127,y:166},{x:75,y:166}],
  consultation:[{x:49,y:113,seated:true},{x:87,y:133,seated:true},{x:144,y:123,seated:true},{x:151,y:79,seated:true}],
  treatment:[{x:45,y:28,laying:true},{x:132,y:63,laying:true},{x:35,y:112,laying:true},{x:120,y:127,laying:true}],
  aftercare:[{x:38,y:37,reclining:true},{x:124,y:45,reclining:true},{x:32,y:117,reclining:true},{x:120,y:124,reclining:true}],
  waiting:[{x:49,y:46,seated:true},{x:83,y:46,seated:true},{x:116,y:46,seated:true},{x:45,y:104,seated:true},{x:45,y:135,seated:true},{x:139,y:152},{x:95,y:160},{x:80,y:136}]
 };
 const employees={reception:[{x:101,y:54}],consultation:[{x:88,y:53,seated:true}],treatment:[{x:110,y:86},{x:169,y:108}],waiting:[],aftercare:[]};
 function drawPerson(slot,employee=false,index=0){
  const g=document.createElementNS(NS,'g');g.classList.add('person',employee?'employee':'guest');
  if(slot.reclining){
   g.classList.add('reclining');const x=slot.x,y=slot.y;
   const head=p(x+17,y+9,44);
   g.innerHTML=`<g class="breath">${line(p(x+17,y+19,36),p(x+17,y+33,25),'#e5b58c',13)}${line(p(x+11,y+40,24),p(x+11,y+56,17),'#758985',4.5)}${line(p(x+23,y+40,24),p(x+23,y+56,17),'#758985',4.5)}${plane(x+3,y+31,28,13,25,'#d5dec9')}<g transform="translate(${head[0]} ${head[1]})"><ellipse class="guest-face" cx="0" cy="0" rx="6" ry="7" fill="#e7c0a0"/><path d="M-6 1 Q-8 -8 0 -8 Q7 -8 6 1 L3 -4 Q-1 -1 -6 -2Z" fill="#54524b"/></g><g class="care-status-icon" transform="translate(${head[0]} ${head[1]-20})"><g class="care-status-float" style="animation-delay:${-index*.65}s"><path d="M-6 -6 Q0 -9 6 -6 L5.5 0 Q4.5 5 0 8 Q-4.5 5 -5.5 0Z" fill="#71977f" stroke="#c5d8bd" stroke-width=".8"/><path d="M-1.3 -4 H1.3 V-1.3 H4 V1.3 H1.3 V4 H-1.3 V1.3 H-4 V-1.3 H-1.3Z" fill="#f1f0dc"/></g></g></g>`;
   return g;
  }
  if(slot.laying){
   g.classList.add('laying');const x=slot.x,y=slot.y;
   g.innerHTML=`<g class="breath">${line(p(x+16,y+22,34),p(x+16,y+41,31),'#e5b88e',13)}${line(p(x+12,y+41,31),p(x+12,y+52,30),'#647b7b',4)}${line(p(x+21,y+41,31),p(x+21,y+52,30),'#647b7b',4)}<circle cx="${p(x+16,y+11,34)[0]}" cy="${p(x+16,y+11,34)[1]}" r="5" fill="#e8c1a0"/><path d="M${pt(p(x+11,y+10,37))} Q${pt(p(x+15,y+4,40))} ${pt(p(x+22,y+9,35))}" fill="none" stroke="#4e5750" stroke-width="3" stroke-linecap="round"/></g>`;
   return g;
  }
  const [x,y]=slot.roaming?[slot.px,slot.py]:p(slot.x,slot.y),shirt=employee?'#7bc5b1':['#e5b58c','#d9a67e','#e6c39e'][index%3],pants=employee?'#52786f':'#61747a',skin='#e7c0a0';
  g.setAttribute('transform',`translate(${x} ${y})`);
  g.innerHTML=`<ellipse cx="1" cy="1" rx="10" ry="4" fill="#56765a" opacity=".14"/><g class="breath">${slot.seated?`<path d="M-4 -16 L-1 -10 L5 -6 L7 0 M3 -16 L8 -11 L11 -6 L13 0" stroke="${pants}" fill="none" stroke-width="4" stroke-linecap="round"/>`:`<path class="walk-leg walk-leg-left" d="M-3 -17 L-4 -1" stroke="${pants}" stroke-width="4.5" stroke-linecap="round"/><path class="walk-leg walk-leg-right" d="M3 -17 L5 -1" stroke="${pants}" stroke-width="4.5" stroke-linecap="round"/>`}<path d="M-5 -32 Q0 -35 5 -32 L7 -17 Q0 -13 -7 -17 Z" fill="${shirt}"/><path d="M-5 -29 L-9 -19 M5 -29 L9 -19" stroke="${shirt}" stroke-width="4.5" stroke-linecap="round"/><circle cx="-9" cy="-18" r="2" fill="${skin}"/><circle cx="9" cy="-18" r="2" fill="${skin}"/><rect x="-2" y="-36" width="4" height="4" rx="1" fill="${skin}"/><ellipse cx="0" cy="-40" rx="6" ry="7" fill="${skin}"/><path d="M-6 -39 Q-8 -48 0 -48 Q7 -48 6 -39 L3 -44 Q-1 -41 -6 -42Z" fill="${employee?'#496358':'#54524b'}"/>${employee?'<path d="M-2 -31 L0 -22 L3 -31" fill="none" stroke="#e6f3e8" stroke-width="1"/><rect x="1" y="-26" width="3" height="4" fill="#e7f2df"/>':''}</g>`;
  if(slot.roaming){
   g.classList.add('roaming');g.dataset.outsideFloor='true';
   g.innerHTML=`<g class="roam" style="--walk-time:${5+index%4}s;--walk-delay:${-(index*1.7)%13}s;--drift-x:${index%2?38:-38}px;--drift-y:${index%3?12:-12}px">${g.innerHTML}</g>`;
  }
  return g;
 }
 // Keep furniture and indoor anchors aligned, but do not shrink the roaming footprint.
 function person(slot,employee=false,index=0,region='waiting'){
  const g=drawPerson(slot,employee,index),figureScale=.82;
  const sceneScale=region==='waiting'||slot.roaming?1:.9;
  const anchor=slot.roaming?[slot.px,slot.py]:(slot.laying||slot.reclining?p(slot.x+17,slot.y+30,30):p(slot.x,slot.y));
  const target=[230+(anchor[0]-230)*sceneScale,200+(anchor[1]-200)*sceneScale];
  if(region==='consultation'&&index===0&&!slot.roaming){
   const bubble=document.createElementNS(NS,'g');bubble.classList.add('consultation-bubble');bubble.classList.add(employee?'speaker-staff':'speaker-guest');
   const bubbleTop=employee?-18:-9;
   const bubbleLines=employee?'<rect class="dialogue-line" x="-17" y="-10" width="34" height="2.5" rx="1.25"/><rect class="dialogue-line" x="-17" y="-2" width="24" height="2.5" rx="1.25"/>':'<rect class="dialogue-line" x="-17" y="-2" width="34" height="2.5" rx="1.25"/>';
   bubble.innerHTML=`<g transform="translate(0 -64)"><g class="consultation-bubble-motion"><path class="dialogue-outline" d="M-23 ${bubbleTop} H23 Q28 ${bubbleTop} 28 ${bubbleTop+5} V4 Q28 9 23 9 H3 L-3 14 V9 H-23 Q-28 9 -28 4 V${bubbleTop+5} Q-28 ${bubbleTop} -23 ${bubbleTop}Z" fill="${employee?'#71977f':'#f1f0dc'}" stroke="${employee?'#c5d8bd':'#71977f'}" stroke-width="1"/><g fill="${employee?'#f1f0dc':'#527860'}">${bubbleLines}</g></g></g>`;
   g.append(bubble);
  }
  g.dataset.figureScale=String(figureScale);
  if(slot.laying||slot.reclining)g.setAttribute('transform',`translate(${target[0]} ${target[1]}) scale(${figureScale}) translate(${-anchor[0]} ${-anchor[1]})`);
  else g.setAttribute('transform',`translate(${target[0]} ${target[1]}) scale(${figureScale})`);
  return g;
 }
 function guestSlot(id,index){
  const indoor=guestSlots[id];
  return index<indoor.length?indoor[index]:TwinModel.crowdSlot(index-indoor.length);
 }
 function render(id){return `<svg class="room-art" viewBox="${id==='waiting'?'-30 54 520 286':'-30 -8 520 348'}" aria-hidden="true"><g class="room-fixed" data-scene-scale="${id==='waiting'?1:.9}" transform="translate(230 200) scale(${id==='waiting'?1:.9}) translate(-230 -200)">${fixed(id)}</g><g class="employee-layer"></g><g class="guest-layer"></g></svg>`;}
 root.TwinScenes={render,person,guestSlot,guestSlots,employees};
})(window);
