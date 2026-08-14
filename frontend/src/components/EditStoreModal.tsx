import { useState } from "react";
import type { StoreSummary, UpdateStoreBasicInfoPayload } from "../api";
import { CITY_OPTIONS } from "../domain/cities";

type EditStoreModalProps = {
  store: StoreSummary;
  saving: boolean;
  onClose: () => void;
  onSubmit: (payload: UpdateStoreBasicInfoPayload) => Promise<void>;
};

export function EditStoreModal({ store, saving, onClose, onSubmit }: EditStoreModalProps) {
  const [city, setCity] = useState(store.city);
  const [name, setName] = useState(store.name);
  const [shortName, setShortName] = useState(store.shortName);
  const [externalOrgId, setExternalOrgId] = useState(store.externalOrgId);
  const [message, setMessage] = useState("");

  function validateAndSubmit() {
    if (!city.trim()) {
      setMessage("请选择城市");
      return;
    }
    if (!name.trim()) {
      setMessage("门店名称不能为空");
      return;
    }
    void onSubmit({
      id: store.id,
      city: city.trim(),
      name: name.trim(),
      shortName: shortName.trim(),
      externalOrgId: externalOrgId.trim(),
    });
  }

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="create-modal edit-store-modal" role="dialog" aria-modal="true" aria-label="编辑机构信息">
        <header className="modal-head">
          <div>
            <strong>编辑机构信息</strong>
            <p>仅修改基础资料，设计图和录像机请进入详情维护。</p>
          </div>
          <button className="icon-button modal-close-button" onClick={onClose} aria-label="关闭编辑机构信息" title="关闭">
            <span aria-hidden="true">×</span>
          </button>
        </header>

        <div className="create-form">
          <label>
            城市
            <select value={city} onChange={(event) => setCity(event.target.value)}>
              <option value="">请选择城市</option>
              {CITY_OPTIONS.map((option) => (
                <option value={option} key={option}>
                  {option}
                </option>
              ))}
            </select>
          </label>
          <label>
            门店名称
            <input value={name} onChange={(event) => setName(event.target.value)} placeholder="请输入门店名称" />
          </label>
          <label>
            机构简称
            <input value={shortName} onChange={(event) => setShortName(event.target.value)} placeholder="选填，例如 凯德晶萃" />
          </label>
          <label>
            新氧机构 ID
            <input value={externalOrgId} onChange={(event) => setExternalOrgId(event.target.value)} placeholder="选填" />
          </label>
        </div>

        {message ? (
          <div className="editor-status" role="status">
            {message}
          </div>
        ) : null}

        <footer className="modal-actions">
          <button onClick={onClose}>取消</button>
          <button className="primary-button" disabled={saving} onClick={validateAndSubmit}>
            {saving ? "保存中" : "保存修改"}
          </button>
        </footer>
      </section>
    </div>
  );
}
