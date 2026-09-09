import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { digitalTwinApi } from "../api-digital-twin";
import { nvrLabApi } from "../api-nvr-lab";
import { composeMonitorScreenshot } from "../domain/screenshot-watermark";
import { NVRLabPlayer } from "../components/NVRLabPlayer";
import { cameraRegion, chooseTwinStore, regionCameras } from "../domain/digital-twin";
import type { NVRLabCamera, NVRLabCameraListResponse, NVRLabStreamSession, NVRMonitorStoreInfo } from "../domain/nvr-lab";
import { digitalTwinDocument } from "./digital-twin-document";
import "./digital-twin.css";

type ActiveCamera = { storeID: string; storeName: string; camera: NVRLabCamera };
type KitWindow = Window & { TwinDemo: {create: () => TwinSnapshot} };

export function DigitalTwin({ displayName, onLogout, loggingOut }: { displayName: string; onLogout: () => void; loggingOut: boolean }) {
  const [stores, setStores] = useState<NVRMonitorStoreInfo[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [data, setData] = useState<NVRLabCameraListResponse | null>(null);
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(true);
  const [reload, setReload] = useState(0);
  const [frameDocument, setFrameDocument] = useState<Document | null>(null);
  const [activeCamera, setActiveCamera] = useState<ActiveCamera | null>(null);
  const frame = useRef<HTMLIFrameElement>(null);
  const srcDoc = useMemo(digitalTwinDocument, []);
  const directory = useMemo(() => stores.map(store => ({ id: store.external_org_id, name: store.store_name, city: store.city || "其他" })), [stores]);
  const closeCamera = useCallback(() => setActiveCamera(null), []);
  useEffect(() => {
    const button = frameDocument?.querySelector<HTMLButtonElement>("#twin-logout");
    if (!button) return;
    button.disabled = loggingOut;
    button.textContent = loggingOut ? "正在登出..." : "登出";
    button.addEventListener("click", onLogout);
    return () => button.removeEventListener("click", onLogout);
  }, [frameDocument, onLogout, loggingOut]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true); setData(null); setFrameDocument(null); setActiveCamera(null); setMessage("");
    void digitalTwinApi.stores().then(result => {
      if (cancelled) return;
      const next = (result.cities || []).flatMap(group => group.stores);
      setStores(next);
      const requested = new URLSearchParams(location.search).get("store");
      const id = chooseTwinStore(next.map(store => store.external_org_id), requested);
      setSelected(id);
      if (!id) { setMessage(requested ? "该机构未开放数字孪生，或你暂无访问权限" : "暂无已开放且在你授权范围内的机构"); setLoading(false); }
    }).catch(error => { if (!cancelled) { setSelected(null); setStores([]); setMessage(error.message || "机构列表加载失败"); setLoading(false); } });
    return () => { cancelled = true; };
  }, [reload]);

  useEffect(() => {
    const onPopState = () => setReload(value => value + 1);
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  useEffect(() => {
    if (!selected) return;
    const controller = new AbortController();
    setLoading(true); setData(null); setFrameDocument(null); setActiveCamera(null); setMessage("");
    void digitalTwinApi.cameras(selected, controller.signal).then(result => {
      if (!controller.signal.aborted) setData(result);
    }).catch(error => { if (!controller.signal.aborted) setMessage(error.message || "摄像头加载失败"); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [selected, reload]);

  useEffect(() => {
    if (!data || !frameDocument) return;
    const win = frameDocument.defaultView as KitWindow | null;
    if (!win?.TwinDashboard || !win.TwinDemo) { setMessage("数字孪生组件加载失败，请刷新重试"); return; }
    const snapshot = win.TwinDemo.create();
    snapshot.store = { id: data.external_org_id, name: data.store_name, experimentStoreCount: stores.length, userName: displayName };
    for (const region of win.TwinDashboard.regionIds) {
      snapshot.regions[region].cameras = regionCameras(data.cameras || [], region).map(camera => ({ id: String(camera.id), name: camera.space_name || camera.name || `摄像头 ${camera.id}`, occupied: null, canView: true }));
    }
    const instance = win.TwinDashboard.mount(frameDocument, {
      data: snapshot, storeDirectory: directory, externalCameraDialog: true,
      onStoreSelect: store => {
        const url = new URL(location.href); url.searchParams.set("store", store.id); history.pushState({}, "", url);
        setSelected(store.id);
      },
      onCameraOpen: ({camera, signal}) => {
        const actual = data.cameras.find(item => String(item.id) === camera.id);
        if (!actual || !cameraRegion(actual) || signal.aborted) return;
        const context = { storeID: data.external_org_id, storeName: data.store_name, camera: actual };
        setActiveCamera(context);
        return () => setActiveCamera(current => current === context ? null : current);
      },
    });
    frameDocument.querySelector(".account-copy small")!.textContent = "已登录二壮";
    frameDocument.querySelectorAll(".experiment-stores small,.store-picker .sample-tag").forEach(node => { node.textContent = "已开放"; });
    frameDocument.querySelector(".scene-footer span")!.textContent = "人数与运营图表为演示数据 · 摄像头来自真实门店";
    frameDocument.querySelector(".store-menu-note")!.textContent = "仅展示白名单内且已授权的机构";
    frameDocument.querySelector("#store-selector-help")!.textContent = "选择已开放且有权限的机构";
    const element = frame.current;
    const resize = () => {
      if (!element) return;
      element.style.height = element.clientWidth < 1100 ? `${Math.ceil(frameDocument.body.getBoundingClientRect().height)}px` : "";
    };
    const observer = new ResizeObserver(resize);
    observer.observe(frameDocument.body);
    window.addEventListener("resize", resize);
    resize();
    return () => { observer.disconnect(); window.removeEventListener("resize", resize); instance.destroy(); };
  }, [data, frameDocument, directory, displayName, stores.length]);

  return <div className="digital-twin-module">
    {loading ? <div className="twin-empty" role="status">正在加载数字孪生...</div> : null}
    {message ? <div className="twin-empty" role="alert"><p>{message}</p><button className="plain-button" onClick={() => setReload(value => value + 1)}>重新加载</button></div> : null}
    {data && !loading && !message ? <>
      <iframe ref={frame} className="twin-frame" title="门店数字孪生看板" srcDoc={srcDoc} onLoad={() => setFrameDocument(frame.current?.contentDocument || null)} />
    </> : null}
    {activeCamera ? <TwinCamera key={`${activeCamera.storeID}:${activeCamera.camera.id}`} active={activeCamera} onClose={closeCamera} /> : null}
  </div>;
}

function TwinCamera({ active, onClose }: { active: ActiveCamera; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [session, setSession] = useState<NVRLabStreamSession | null>(null);
  const [message, setMessage] = useState("");
  const [retry, setRetry] = useState(0);
  const [screenshotMessage, setScreenshotMessage] = useState("");
  const handleScreenshot = async (dataUrl: string) => {
    try {
      const metadata = await nvrLabApi.getScreenshotMetadata(active.storeID, active.camera.id);
      const result = await composeMonitorScreenshot(dataUrl, metadata);
      const anchor = document.createElement("a");
      anchor.href = result;
      anchor.download = `monitor-snapshot-${Date.now()}.png`;
      anchor.rel = "noopener";
      document.body.append(anchor);
      anchor.click();
      anchor.remove();
      setScreenshotMessage("");
    } catch (error) {
      setScreenshotMessage(error instanceof Error ? `截图失败：${error.message}` : "截图失败，请稍后重试");
    }
  };
  useEffect(() => {
    const element = dialog.current!; element.showModal();
    return () => element.close();
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    setSession(null); setMessage("");
    void digitalTwinApi.stream(active.storeID, active.camera.id, controller.signal).then(value => { if (!controller.signal.aborted) setSession(value); })
      .catch(error => { if (!controller.signal.aborted) setMessage(error.message || "摄像头取流失败"); });
    return () => controller.abort();
  }, [active.storeID, active.camera.id, retry]);
  return <dialog ref={dialog} className="twin-camera-dialog" aria-labelledby="twin-camera-title" onCancel={onClose} onClick={event => { if (event.target === dialog.current) onClose(); }}>
    <header><h2 id="twin-camera-title">{active.storeName} · {active.camera.space_name || active.camera.name} <small>#{active.camera.id}</small></h2><button className="plain-button" aria-label="关闭摄像头" onClick={onClose}>×</button></header>
    {message ? <p role="alert">{message}<button className="plain-button" onClick={() => setRetry(value => value + 1)}>重新连接</button></p> : <NVRLabPlayer session={session} onScreenshot={handleScreenshot} onRetry={() => setRetry(value => value + 1)} />}
    {screenshotMessage ? <p role="alert">{screenshotMessage}</p> : null}
  </dialog>;
}
