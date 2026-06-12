import { useRef, useState } from "react";
import type { CreateStoreSpacePayload, EzvizAccount, RecorderDraft, UploadResult } from "../api";

const MAX_PDF_BYTES = 5 * 1024 * 1024;
const MAX_RECORDERS = 3;

type CreateStoreModalProps = {
  accounts: EzvizAccount[];
  uploading: boolean;
  saving: boolean;
  onUploadPdf: (file: File) => Promise<UploadResult>;
  onClose: () => void;
  onSubmit: (payload: CreateStoreSpacePayload) => Promise<void>;
};

export function CreateStoreModal({ accounts, uploading, saving, onUploadPdf, onClose, onSubmit }: CreateStoreModalProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [name, setName] = useState("");
  const [externalOrgId, setExternalOrgId] = useState("");
  const [designPlan, setDesignPlan] = useState<UploadResult | null>(null);
  const [recorders, setRecorders] = useState<RecorderDraft[]>([createRecorderDraft()]);
  const [message, setMessage] = useState("");

  async function handlePdfSelected(fileList: FileList | null) {
    const file = fileList?.[0];
    if (!file) return;
    if (file.type !== "application/pdf" && !file.name.toLowerCase().endsWith(".pdf")) {
      setMessage("仅支持上传 PDF 文件。");
      return;
    }
    if (file.size > MAX_PDF_BYTES) {
      setMessage("文件过大，请上传 5MB 以内的 PDF。");
      return;
    }
    const upload = await onUploadPdf(file);
    setDesignPlan(upload);
    setMessage(`已选择设计图：${upload.fileName}`);
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  }

  function updateRecorder(id: string, patch: Partial<RecorderDraft>) {
    setRecorders((items) => items.map((item) => (item.id === id ? { ...item, ...patch } : item)));
  }

  function validateAndSubmit() {
    const cleanRecorders = recorders
      .map((item) => ({ ...item, deviceCode: item.deviceCode.trim() }))
      .filter((item) => item.deviceCode);
    const repeatedDeviceCode = cleanRecorders.find(
      (item, index) => cleanRecorders.findIndex((other) => other.deviceCode === item.deviceCode) !== index,
    );
    const missingAccount = cleanRecorders.some(
      (item) => !item.ezvizAccountId || !accounts.some((account) => account.id === item.ezvizAccountId),
    );

    if (!name.trim()) {
      setMessage("门店名称不能为空");
      return;
    }
    if (!designPlan && cleanRecorders.length === 0) {
      setMessage("请至少上传设计图或填写一个录像机设备编码");
      return;
    }
    if (repeatedDeviceCode) {
      setMessage("同一门店内录像机设备编码不允许重复");
      return;
    }
    if (missingAccount) {
      setMessage("请选择萤石云账号");
      return;
    }

    void onSubmit({
      name: name.trim(),
      externalOrgId: externalOrgId.trim(),
      designPlan,
      recorders: cleanRecorders,
    });
  }

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="create-modal" role="dialog" aria-modal="true" aria-label="添加门店">
        <header className="modal-head">
          <div>
            <strong>添加门店</strong>
            <p>设计图和录像机至少提供一个，创建后进入对应详情页继续维护。</p>
          </div>
          <button className="plain-button" onClick={onClose} aria-label="关闭添加门店">
            x
          </button>
        </header>

        <div className="create-form">
          <label>
            门店名称
            <input value={name} onChange={(event) => setName(event.target.value)} placeholder="请输入门店名称" />
          </label>
          <label>
            新氧机构 ID
            <input value={externalOrgId} onChange={(event) => setExternalOrgId(event.target.value)} placeholder="选填" />
          </label>

          <div className="upload-row">
            <div>
              <strong>设计图 PDF</strong>
              <span>{designPlan?.fileName ?? "选填，可后补"}</span>
            </div>
            <input
              ref={fileInputRef}
              className="visually-hidden"
              type="file"
              accept="application/pdf"
              onChange={(event) => void handlePdfSelected(event.target.files)}
            />
            <button disabled={uploading} onClick={() => fileInputRef.current?.click()}>
              {uploading ? "解析中" : designPlan ? "更换 PDF" : "上传 PDF"}
            </button>
          </div>

          <section className="recorder-drafts" aria-label="录像机设备编码">
            <div className="section-title-row">
              <div>
                <strong>录像机</strong>
                <span>选填，最多 3 台</span>
              </div>
              <button
                disabled={recorders.length >= MAX_RECORDERS}
                onClick={() => setRecorders((items) => [...items, createRecorderDraft()])}
              >
                增加录像机
              </button>
            </div>

            {recorders.map((recorder, index) => (
              <div className="recorder-draft-row" key={recorder.id}>
                <label>
                  萤石云账号
                  <select
                    value={recorder.ezvizAccountId}
                    disabled={accounts.length === 0}
                    onChange={(event) =>
                      updateRecorder(recorder.id, { ezvizAccountId: event.target.value ? Number(event.target.value) : "" })
                    }
                  >
                    <option value="">{accounts.length === 0 ? "暂无可选账号" : "请选择账号"}</option>
                    {accounts.map((account) => (
                      <option value={account.id} key={account.id}>
                        {account.accountName}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  设备编码
                  <input
                    value={recorder.deviceCode}
                    onChange={(event) => updateRecorder(recorder.id, { deviceCode: event.target.value })}
                    placeholder="例如 D12345678"
                  />
                </label>
                <button
                  className="danger-link"
                  disabled={recorders.length === 1 && index === 0}
                  onClick={() => setRecorders((items) => items.filter((item) => item.id !== recorder.id))}
                >
                  删除
                </button>
              </div>
            ))}
          </section>
        </div>

        {message ? (
          <div className="editor-status" role="status">
            {message}
          </div>
        ) : null}

        <footer className="modal-actions">
          <button onClick={onClose}>取消</button>
          <button className="primary-button" disabled={saving || uploading} onClick={validateAndSubmit}>
            {saving ? "创建中" : "创建门店"}
          </button>
        </footer>
      </section>
    </div>
  );
}

function createRecorderDraft(): RecorderDraft {
  return {
    id: `recorder-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    ezvizAccountId: "",
    deviceCode: "",
  };
}
