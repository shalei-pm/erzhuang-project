(function(root){
 const colors={all:'#e3e9d9',no:'#00dca0',consult:'#ff913d'};
 const series=(label,key,color)=>({label,key,color});
 const chartSpecs=[
  {id:'visits',title:'到访人数趋势',en:'DAILY VISITS',type:'stacked',unit:'人',max:100,series:[series('无需咨询','noConsult',colors.no),series('需要面诊','consult',colors.consult)]},
  {id:'stay',title:'平均在店时长',en:'TIME IN STORE',type:'line',unit:'分钟',max:120,series:[series('全部顾客','stayAll',colors.all),series('无需咨询','stayNo',colors.no),series('需要面诊','stayConsult',colors.consult)]},
  {id:'wait',title:'平均等待时长',en:'WAITING TIME',type:'line',unit:'分钟',max:30,series:[series('全部顾客','waitAll',colors.all),series('无需咨询','waitNo',colors.no),series('需要面诊','waitConsult',colors.consult)]},
  {id:'upgrade',title:'升单率趋势',en:'UPGRADE RATE',type:'line',unit:'%',max:50,series:[series('全部顾客','upgradeAll',colors.all),series('无需咨询','upgradeNo',colors.no),series('需要面诊','upgradeConsult',colors.consult)]},
  {id:'redemption',rotationGroup:'redemption',title:'人均核销金额',en:'REDEMPTION PER GUEST',type:'line',unit:'元/人',max:2000,series:[series('全部顾客','redemptionAll',colors.all),series('无需咨询','redemptionNo',colors.no),series('需要咨询','redemptionConsult',colors.consult)]},
  {id:'service-points',rotationGroup:'redemption',title:'人均核销服务点',en:'SERVICES PER GUEST',type:'line',unit:'点/人',max:10,series:[series('全部顾客','servicePointAll',colors.all),series('无需咨询','servicePointNo',colors.no),series('需要咨询','servicePointConsult',colors.consult)]}
 ];
 function buildDemoData(){
  const round=n=>Math.round(n*10)/10;
  // Seeded, irregular daily examples; never smooth or rewrite real observations.
  const noise=(i,salt)=>{let n=Math.imul(i+1,374761393)^Math.imul(salt,668265263);n=Math.imul(n^(n>>>13),1274126177);return ((n^(n>>>16))>>>0)/4294967295*2-1;};
  return Array.from({length:30},(_,i)=>{
   const noConsult=Math.round(34+i*.53+Math.sin(i*.84)*8+(i%7===5?10:0));
   const consult=Math.round(17+i*.19+Math.cos(i*.62)*5+(i%7===5?5:0));
   const stayNo=round(53+noise(i,1)*12-i*.14),stayConsult=round(88+noise(i,2)*17-i*.19);
   const waitNo=round(12+noise(i,3)*4-i*.06),waitConsult=round(20+noise(i,4)*6-i*.08);
   const upgradeNo=round(15+i*.22+noise(i,5)*6),upgradeConsult=round(29+i*.25+noise(i,6)*8);
   // Visual-only weighted example; production upgrade eligibility/denominators remain undecided.
   const upgradeAll=round((upgradeNo*noConsult+upgradeConsult*consult)/(noConsult+consult));
   // Amounts are synthetic presentation samples, not finalized redemption accounting.
   const redemptionNo=round(650+i*5+noise(i,7)*150),redemptionConsult=round(1250+i*8+noise(i,8)*230);
   const redemptionAll=round((redemptionNo*noConsult+redemptionConsult*consult)/(noConsult+consult));
   const servicePointNo=round(3.2+i*.03+noise(i,9)*.7),servicePointConsult=round(6.4+i*.04+noise(i,10)*1.1);
   const servicePointAll=round((servicePointNo*noConsult+servicePointConsult*consult)/(noConsult+consult));
   return {date:new Date(Date.UTC(2026,7,10+i)).toISOString().slice(0,10),noConsult,consult,stayNo,stayConsult,stayAll:round((stayNo*noConsult+stayConsult*consult)/(noConsult+consult)),waitNo,waitConsult,waitAll:round((waitNo*noConsult+waitConsult*consult)/(noConsult+consult)),upgradeNo,upgradeConsult,upgradeAll,redemptionNo,redemptionConsult,redemptionAll,servicePointNo,servicePointConsult,servicePointAll};
  });
 }
 const api={chartSpecs,buildDemoData};
 if(typeof module!=='undefined')module.exports=api;else root.TwinChartData=api;
})(typeof window!=='undefined'?window:globalThis);
