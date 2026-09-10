/** Browser globals. Load kit scripts before calling TwinDashboard.mount. */
type TwinRegionId = 'reception' | 'consultation' | 'treatment' | 'aftercare' | 'waiting';
type TwinChartId = 'visits' | 'stay' | 'wait' | 'upgrade' | 'redemption' | 'service-points';
type TwinMeasurement = number | null;
interface TwinCamera { id: string; name: string; occupied: boolean | null; canView: boolean; }
type TwinRegionMeasurementKey = 'current' | 'cumulative' | 'noConsultation' | 'consultationRequired' | 'staff';
interface TwinRegion { current: TwinMeasurement; cumulative: TwinMeasurement; noConsultation: TwinMeasurement; consultationRequired: TwinMeasurement; staff: TwinMeasurement; staleFields: TwinRegionMeasurementKey[]; cameras: TwinCamera[]; }
interface TwinStore { id: string; name: string; experimentStoreCount: TwinMeasurement; userName: string; }
interface TwinOverview { expected: TwinMeasurement; arrived: TwinMeasurement; receptionists: TwinMeasurement; consultants: TwinMeasurement; nurses: TwinMeasurement; doctors: TwinMeasurement; }
interface TwinTrend {
 date: string;
 noConsult: TwinMeasurement; consult: TwinMeasurement;
 stayAll: TwinMeasurement; stayNo: TwinMeasurement; stayConsult: TwinMeasurement;
 waitAll: TwinMeasurement; waitNo: TwinMeasurement; waitConsult: TwinMeasurement;
 upgradeAll: TwinMeasurement; upgradeNo: TwinMeasurement; upgradeConsult: TwinMeasurement;
 redemptionAll: TwinMeasurement; redemptionNo: TwinMeasurement; redemptionConsult: TwinMeasurement;
 servicePointAll: TwinMeasurement; servicePointNo: TwinMeasurement; servicePointConsult: TwinMeasurement;
}
interface TwinSnapshot {
 mode: 'demo' | 'external'; revision: number; updatedAt: string | null;
 store: TwinStore; overview: TwinOverview; staleOverview: (keyof TwinOverview)[]; permissions: { canViewCameras: boolean };
 regions: Record<TwinRegionId, TwinRegion>; trends: TwinTrend[];
}
interface TwinPatch {
 mode?: TwinSnapshot['mode']; revision?: number; updatedAt?: string | null;
 store?: Partial<TwinStore>; overview?: Partial<TwinOverview>; staleOverview?: (keyof TwinOverview)[]; permissions?: {canViewCameras?:boolean};
 regions?: Partial<Record<TwinRegionId, Partial<TwinRegion>>>;
 trends?: (Pick<TwinTrend,'date'> & Partial<Omit<TwinTrend,'date'>>)[];
}
interface TwinDiagnostics {
 limits: {perRegion:number;total:number}; mode: TwinSnapshot['mode']; revision:number;
 regions: Record<TwinRegionId,{rendered:number;reason:null|'unknown'|'region-limit'|'total-limit'}>;
}
interface TwinInstance {
 update(patch:TwinPatch): {applied:true;revision:number;diagnostics:TwinDiagnostics} | {applied:false;reason:'stale';revision:number};
 /** Replace clears omitted fields instead of retaining the previous store's values. */
 replace(snapshot:TwinPatch): {applied:true;revision:number;diagnostics:TwinDiagnostics};
 getState():TwinSnapshot; getDiagnostics():TwinDiagnostics; destroy():void;
}
interface TwinStoreEntry {id:string;name:string;city:string;disabled?:boolean;}
interface TwinMountOptions {
 externalCameraDialog?:boolean;
 storeDirectory?:TwinStoreEntry[];
 /** Host calls replace(fullSnapshot); selection never relabels existing data. */
 onStoreSelect?:(store:TwinStoreEntry)=>void|Promise<void>;
 data?:TwinPatch;
 config?:{regions?:Partial<Record<TwinRegionId,{name?:string;en?:string;caption?:string}>>;chartOrder?:TwinChartId[];chartTitles?:Partial<Record<TwinChartId,string>>};
 limits?:{perRegion?:number;total?:number};
 onCameraOpen?:(context:{store:TwinStore;regionId:TwinRegionId;camera:TwinCamera;container:HTMLDivElement;signal:AbortSignal})=>void|(()=>void)|Promise<void|(()=>void)>;
}
declare const TwinDashboard:{version:string;regionIds:TwinRegionId[];createEmptyData():TwinSnapshot;mount(root:Document|HTMLElement,options?:TwinMountOptions):TwinInstance};
interface Window {clinicDashboard:TwinInstance;TwinDashboard:typeof TwinDashboard;}
