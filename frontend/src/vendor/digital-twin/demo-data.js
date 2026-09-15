/* Example fixtures only. Production hosts pass their own data to TwinDashboard.mount. */
(function(root){
 const regions=[
  {id:'reception',name:'前台',en:'RECEPTION',state:'当前接待',initial:2,cumulativeLabel:'已到访',cumulative:12,caption:'接待登记 · 服务起点',cameras:['前台 1','前台 2']},
  {id:'consultation',name:'咨询室',en:'CONSULTATION',state:'当前咨询',initial:1,cumulativeLabel:'已接待',cumulative:5,caption:'一对一咨询 · 沟通需求',occupiedCameras:[true,false,false],cameras:['咨询室 1','咨询室 2','咨询室 3']},
  {id:'treatment',name:'治疗室',en:'TREATMENT',state:'当前治疗',initial:10,cumulativeLabel:'已服务',cumulative:10,caption:'专业诊疗 · 安心服务',occupiedCameras:Array.from({length:20},(_,index)=>index<2),cameras:Array.from({length:20},(_,index)=>`治疗室 ${index+1}`)},
  {id:'aftercare',name:'术后护理',en:'AFTERCARE',state:'当前护理',initial:2,caption:'半躺休息 · 敷膜护理',cameras:['术后护理 1','术后护理 2']},
  {id:'waiting',name:'等候区',en:'WAITING LOUNGE',state:'当前等候',initial:30,caption:'中央开放等候 · 舒适休息',cameras:['等候区 1','等候区 2']}
 ];
 function create(){
  const s=TwinKitCore.emptyData();s.mode='demo';s.store={id:'beijing-poly',name:'北京保利总部店',experimentStoreCount:1,userName:'总部用户'};
  s.overview={expected:60,arrived:42,receptionists:2,consultants:3,nurses:4,doctors:2};s.permissions.canViewCameras=true;
  for(const r of regions)s.regions[r.id]={current:r.initial,cumulative:r.cumulative??null,noConsultation:null,consultationRequired:null,staff:TwinScenes.employees[r.id].length,staleFields:['reception','treatment','waiting'].includes(r.id)?['noConsultation','consultationRequired']:[],cameras:r.cameras.map((name,i)=>({id:`${r.id}-${i+1}`,name,occupied:r.occupiedCameras?r.occupiedCameras[i]:null,canView:true}))};
  s.trends=TwinChartData.buildDemoData();return s;
 }
 root.TwinDemo={create};
})(window);
