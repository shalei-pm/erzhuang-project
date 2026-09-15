(function(root){
  function normalizeCount(value,max){
    if(typeof value!=='number'||!Number.isFinite(value)||!Number.isInteger(value))throw new TypeError('人数须为有效整数');
    return Math.min(max,Math.max(0,value));
  }
  function slotDiff(previous,next){
    const keep=[],enter=[],exit=[];
    for(let i=0;i<Math.max(previous,next);i++){
      if(i<previous&&i<next)keep.push(i);else if(i<next)enter.push(i);else exit.push(i);
    }
    return {keep,enter,exit};
  }
  // This is a local demo guard, not a room/bed capacity constraint.
  const MAX_DEMO_COUNT=80;
  function crowdSlot(index){
    if(!Number.isSafeInteger(index)||index<0)throw new RangeError('无效活动位置');
    // Permutation spreads early arrivals across the surrounding activity area.
    const base=index%MAX_DEMO_COUNT;
    const shuffled=(base*37)%MAX_DEMO_COUNT;
    const side=shuffled%2,row=Math.floor(shuffled/2)%10,band=Math.floor(shuffled/20);
    const py=201+row*12+(band%2)*3;
    const offset=20+band*16;
    const px=side===0?68+(py-200)*(180/91)-offset:410-(py-208)*(162/83)+offset;
    // Additional blocks get deterministic sub-slot offsets; no 80-person clamp.
    const block=Math.floor(index/MAX_DEMO_COUNT),jitter=block?((block*.61803398875)%1-.5)*5:0;
    return {px:px+jitter,py:py+(block?((block*.41421356237)%1-.5)*4:0),roaming:true};
  }
  const api={normalizeCount,slotDiff,MAX_DEMO_COUNT,crowdSlot};
  if(typeof module!=='undefined')module.exports=api;else root.TwinModel=api;
})(typeof window!=='undefined'?window:globalThis);
