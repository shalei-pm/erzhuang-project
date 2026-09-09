import { useEffect, useState } from "react";
import { digitalTwinApi, type DigitalTwinSettings as Settings } from "../api-digital-twin";
import { parseInstitutionID } from "../domain/digital-twin";
import type { NVRMonitorStoreInfo } from "../domain/nvr-lab";
import "../pages/digital-twin.css";

export function DigitalTwinSettings() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [ids, setIds] = useState<string[]>([]);
  const [candidates, setCandidates] = useState<NVRMonitorStoreInfo[]>([]);
  const [input, setInput] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [reload, setReload] = useState(0);
  useEffect(() => {
    let cancelled = false;
    setSettings(null); setMessage("");
    void Promise.all([digitalTwinApi.settings(), digitalTwinApi.candidates()]).then(([next, directory]) => {
      if (cancelled) return;
      setSettings(next); setIds(next.store_ids); setCandidates(directory.cities.flatMap(group => group.stores));
    }).catch(error => { if (!cancelled) setMessage(error.message || "白名单加载失败"); });
    return () => { cancelled = true; };
  }, [reload]);
  function add() {
    const id = parseInstitutionID(input);
    if (!id) { setMessage("请输入有效的机构 ID"); return; }
    if (ids.includes(id)) { setMessage("该机构已在名单中"); return; }
    if (!candidates.some(store => store.external_org_id === id)) { setMessage("未找到该机构，或该机构尚未配置可用监控"); return; }
    if (ids.length >= 100) { setMessage("白名单最多支持 100 家机构"); return; }
    setIds(previous => [...previous, id]); setInput(""); setMessage("");
  }
  async function save() {
    if (!settings || busy) return;
    setBusy(true); setMessage("");
    try {
      const next = await digitalTwinApi.save({ version: settings.version, store_ids: ids });
      setSettings(next); setIds(next.store_ids); setMessage("已保存，全局生效");
    } catch (error) { setMessage(error instanceof Error ? error.message : "保存失败"); }
    finally { setBusy(false); }
  }
  const dirty = !!settings && [...ids].sort().join(",") !== [...settings.store_ids].sort().join(",");
  return <section className="twin-settings" aria-label="数字孪生白名单">
    <header><h2>数字孪生白名单</h2><span>已添加 {ids.length} 家机构</span></header>
    <form className="twin-settings-form" onSubmit={event => { event.preventDefault(); add(); }}>
      <label htmlFor="twin-institution-id">机构 ID</label>
      <input id="twin-institution-id" inputMode="numeric" autoComplete="off" placeholder="输入机构 ID" value={input} onChange={event => setInput(event.target.value)} disabled={!settings || busy} />
      <button className="plain-button" disabled={!settings || busy || !input.trim()}>添加机构</button>
    </form>
    <div className="twin-settings-table"><table><thead><tr><th>机构 ID</th><th>机构名称</th><th>城市</th><th>操作</th></tr></thead>
      <tbody>{ids.map(id => { const store = candidates.find(candidate => candidate.external_org_id === id); return <tr key={id}><td>{id}</td><td>{store?.store_name || "机构不可用或已移除"}</td><td>{store?.city || "—"}</td><td><button className="plain-button" disabled={busy} onClick={() => setIds(previous => previous.filter(value => value !== id))} aria-label={`移除机构 ${id}`}>移除</button></td></tr>; })}</tbody>
    </table>{settings && !ids.length ? <p className="twin-empty">白名单为空，数字孪生不开放任何机构</p> : null}</div>
    {!settings && !message ? <p role="status">正在加载白名单...</p> : null}
    {message ? <p role="status">{message}</p> : null}
    <footer><button className="plain-button" disabled={busy} onClick={() => setReload(value => value + 1)}>重新加载</button><button className="primary-button" disabled={!dirty || busy} onClick={() => void save()}>{busy ? "保存中..." : "保存白名单"}</button></footer>
  </section>;
}
