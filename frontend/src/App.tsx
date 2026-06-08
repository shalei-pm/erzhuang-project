import { useEffect, useMemo, useRef, useState } from "react";
import {
  type AreaBox,
  type AreaType,
  designPlanApi,
  type StoreArea,
  type StoreStatus,
  type StoreSummary,
} from "./api";

const PAGE_SIZE = 20;
const APP_VERSION = import.meta.env.VITE_APP_VERSION || "local-dev";

const areaTypeLabels: Record<AreaType, string> = {
  treatment: "治疗室",
  consultation: "面诊室",
  beauty: "生美",
};

const statusLabels: Record<StoreStatus, string> = {
  completed: "已完成",
  needs_review: "需确认",
  incomplete: "待完善",
};

type EditorMode = "create" | "edit";
type UploadStage = "initial" | "converting" | "recognizing" | "ready" | "failed";
type ResizeHandle = "nw" | "ne" | "sw" | "se";
type DragState = {
  areaId: string;
  mode: "move" | "resize";
  handle?: ResizeHandle;
  startX: number;
  startY: number;
  origin: AreaBox;
};
type ValidationResult = {
  fieldErrors: string[];
  areaErrors: Record<string, string[]>;
};
type EditorState = {
  id?: number;
  mode: EditorMode;
  storeName: string;
  fileName: string;
  uploadId?: string;
  originalPath: string;
  previewPath: string;
  thumbnailPath: string;
  pageCount: number;
  previewUrl: string;
  thumbnailUrl: string;
  recognitionResult?: unknown;
  uploadStage: UploadStage;
  areas: StoreArea[];
  selectedAreaId: string | null;
  dirty: boolean;
};

const emptyValidation: ValidationResult = { fieldErrors: [], areaErrors: {} };

function App() {
  const [stores, setStores] = useState<StoreSummary[]>([]);
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [editor, setEditor] = useState<EditorState | null>(null);
  const [validation, setValidation] = useState<ValidationResult>(emptyValidation);
  const [saving, setSaving] = useState(false);
  const [toast, setToast] = useState("");
  const [dragState, setDragState] = useState<DragState | null>(null);
  const [planZoom, setPlanZoom] = useState(1);
  const planRef = useRef<HTMLDivElement | null>(null);
  const areaCardRefs = useRef<Record<string, HTMLElement | null>>({});
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const firstIndex = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const lastIndex = Math.min(total, page * PAGE_SIZE);

  useEffect(() => {
    void loadStores();
  }, [page, query]);

  useEffect(() => {
    if (!editor?.selectedAreaId) return;
    areaCardRefs.current[editor.selectedAreaId]?.scrollIntoView({
      block: "nearest",
      behavior: "smooth",
    });
  }, [editor?.selectedAreaId]);

  useEffect(() => {
    if (!editor) return;

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeEditor();
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [editor]);

  useEffect(() => {
    if (!dragState) return;

    const onPointerMove = (event: PointerEvent) => {
      const rect = planRef.current?.getBoundingClientRect();
      if (!rect) return;
      const dx = (event.clientX - dragState.startX) / rect.width;
      const dy = (event.clientY - dragState.startY) / rect.height;
      const nextBox =
        dragState.mode === "resize" && dragState.handle
          ? resizeBox(dragState.origin, dragState.handle, dx, dy)
          : {
              ...dragState.origin,
              x: dragState.origin.x + dx,
              y: dragState.origin.y + dy,
            };
      updateArea(dragState.areaId, {
        box: clampBox(nextBox),
      });
    };

    const onPointerUp = () => setDragState(null);

    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", onPointerUp, { once: true });
    return () => {
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", onPointerUp);
    };
  }, [dragState]);

  async function loadStores() {
    setLoading(true);
    const response = await designPlanApi.listStores(query, page, PAGE_SIZE);
    setStores(response.items);
    setTotal(response.total);
    setLoading(false);
  }

  function handleSearch(value: string) {
    setQuery(value);
    setPage(1);
  }

  function openCreateEditor() {
    setValidation(emptyValidation);
    setPlanZoom(1);
    setEditor({
      mode: "create",
      storeName: "",
      fileName: "",
      originalPath: "",
      previewPath: "",
      thumbnailPath: "",
      pageCount: 0,
      previewUrl: "",
      thumbnailUrl: "",
      uploadStage: "initial",
      areas: [],
      selectedAreaId: null,
      dirty: false,
    });
  }

  async function openEditEditor(storeId: number) {
    const detail = await designPlanApi.getStore(storeId);
    setValidation(emptyValidation);
    setPlanZoom(1);
    setEditor({
      id: detail.id,
      mode: "edit",
      storeName: detail.name,
      fileName: detail.fileName,
      originalPath: detail.originalPath,
      previewPath: detail.previewPath,
      thumbnailPath: detail.thumbnailPath,
      pageCount: detail.pageCount,
      previewUrl: detail.previewUrl,
      thumbnailUrl: detail.thumbnailUrl,
      recognitionResult: detail.recognitionResult,
      uploadStage: "ready",
      areas: detail.areas,
      selectedAreaId: detail.areas[0]?.id ?? null,
      dirty: false,
    });
  }

  function closeEditor() {
    if (!editor) return;
    if (editor.dirty && !window.confirm("有未保存修改，确认离开？")) {
      return;
    }
    setEditor(null);
    setValidation(emptyValidation);
  }

  function requestPdfUpload() {
    fileInputRef.current?.click();
  }

  function handlePdfSelected(fileList: FileList | null) {
    const file = fileList?.[0];
    if (!file) return;
    void uploadAndRecognize(file);
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  }

  async function uploadAndRecognize(file: File) {
    if (!editor) return;
    setValidation(emptyValidation);
    setEditor((current) =>
      current
        ? {
            ...current,
            uploadStage: "converting",
            fileName: file.name,
            dirty: true,
          }
        : current,
    );

    let upload;
    try {
      upload = await designPlanApi.uploadPdf(file);
    } catch (error) {
      setToast(errorMessage(error, "设计图解析失败，请重新上传 PDF。"));
      setEditor((current) =>
        current
          ? {
              ...current,
              uploadStage: "failed",
              areas: [],
              selectedAreaId: null,
              dirty: true,
            }
          : current,
      );
      return;
    }

    setEditor((current) =>
      current
        ? {
            ...current,
            uploadStage: "recognizing",
            uploadId: upload.uploadId,
            fileName: upload.fileName,
            originalPath: upload.originalPath,
            previewPath: upload.previewPath,
            thumbnailPath: upload.thumbnailPath,
            pageCount: upload.pageCount,
            previewUrl: upload.previewUrl,
            thumbnailUrl: upload.thumbnailUrl,
            dirty: true,
          }
        : current,
    );

    let recognition;
    try {
      recognition = await designPlanApi.recognizeUpload(upload.uploadId);
    } catch (error) {
      setToast(errorMessage(error, "AI 识别失败，可手动维护。"));
      setEditor((current) =>
        current
          ? {
              ...current,
              uploadStage: "failed",
              areas: [],
              selectedAreaId: null,
              dirty: true,
            }
          : current,
      );
      return;
    }

    setEditor((current) =>
      current
        ? {
            ...current,
            uploadStage: "ready",
            storeName: recognition.storeName.trim() || current.storeName,
            fileName: planFileNameForStore(recognition.storeName.trim() || current.storeName, current.fileName),
            areas: recognition.areas,
            selectedAreaId: recognition.areas[0]?.id ?? null,
            recognitionResult: recognition.rawResult ?? recognition,
            dirty: true,
          }
        : current,
    );
  }

  function updateEditor(patch: Partial<EditorState>) {
    setEditor((current) => (current ? { ...current, ...patch, dirty: true } : current));
  }

  function selectArea(areaId: string) {
    const activeElement = document.activeElement;
    const nextCard = areaCardRefs.current[areaId];
    setEditor((current) => {
      if (!current || current.selectedAreaId === areaId) return current;
      if (activeElement instanceof HTMLElement && !nextCard?.contains(activeElement)) {
        activeElement.blur();
      }
      return { ...current, selectedAreaId: areaId, dirty: true };
    });
  }

  function updateArea(areaId: string, patch: Partial<StoreArea>) {
    setEditor((current) =>
      current
        ? {
            ...current,
            areas: current.areas.map((item) => (item.id === areaId ? withGeneratedAreaFields({ ...item, ...patch }) : item)),
            dirty: true,
          }
        : current,
    );
  }

  function addArea() {
    if (!editor) return;
    const newArea: StoreArea = {
      id: `manual-${Date.now()}`,
      name: "",
      type: "",
      number: "",
      confidence: "high",
      needsReview: false,
      box: {
        x: 0.42,
        y: 0.4,
        width: 0.16,
        height: 0.12,
      },
    };
    setEditor({
      ...editor,
      areas: [...editor.areas, newArea],
      selectedAreaId: newArea.id,
      dirty: true,
    });
  }

  function deleteArea(areaId: string) {
    if (!editor) return;
    const nextAreas = editor.areas.filter((item) => item.id !== areaId);
    setEditor({
      ...editor,
      areas: nextAreas,
      selectedAreaId: editor.selectedAreaId === areaId ? (nextAreas[0]?.id ?? null) : editor.selectedAreaId,
      dirty: true,
    });
  }

  function moveArea(areaId: string, direction: -1 | 1) {
    if (!editor) return;
    const index = editor.areas.findIndex((item) => item.id === areaId);
    const targetIndex = index + direction;
    if (index < 0 || targetIndex < 0 || targetIndex >= editor.areas.length) return;
    const nextAreas = [...editor.areas];
    const [item] = nextAreas.splice(index, 1);
    nextAreas.splice(targetIndex, 0, item);
    updateEditor({ areas: nextAreas });
  }

  async function saveEditor() {
    if (!editor) return;
    const result = validateEditor(editor);
    setValidation(result);
    const invalidCount = Object.keys(result.areaErrors).length;

    if (result.fieldErrors.length > 0 || invalidCount > 0) {
      setToast(invalidCount > 0 ? `还有 ${invalidCount} 个区域未完善。` : result.fieldErrors[0]);
      return;
    }

    const normalizedAreas = editor.areas.map(normalizeAreaForSave);

    setSaving(true);
    const duplicate = await designPlanApi.checkDuplicate(editor.storeName, editor.id);
    if (duplicate.exactMatch && !window.confirm("系统检测到已存在同名门店。继续保存将覆盖该门店当前设计图和区域信息，覆盖后不可恢复。是否继续？")) {
      setSaving(false);
      return;
    }

    await designPlanApi.saveStore({
      id: editor.id,
      name: editor.storeName,
      fileName: editor.fileName,
      originalPath: editor.originalPath,
      previewPath: editor.previewPath,
      thumbnailPath: editor.thumbnailPath,
      pageCount: editor.pageCount,
      previewUrl: editor.previewUrl,
      thumbnailUrl: editor.thumbnailUrl || editor.previewUrl,
      uploadId: editor.uploadId,
      recognitionResult: editor.recognitionResult,
      areas: normalizedAreas,
    });
    setSaving(false);
    setEditor(null);
    setToast("保存成功，门店列表已刷新。");
    await loadStores();
  }

  async function deleteStore(store: StoreSummary) {
    if (!window.confirm("删除后无法恢复，是否确认删除该门店？")) return;
    await designPlanApi.deleteStore(store.id);
    setToast(`已删除：${store.name}`);
    await loadStores();
  }

  const selectedArea = useMemo(
    () => editor?.areas.find((item) => item.id === editor.selectedAreaId) ?? null,
    [editor],
  );
  const validationAreaErrorCount = Object.keys(validation.areaErrors).length;

  return (
    <main className="app-shell">
      <header className="page-header">
        <div>
          <p className="eyebrow">空间资源管理</p>
          <h1>设计图标记与诊室区域管理</h1>
        </div>
        <button className="primary-button" onClick={openCreateEditor}>
          <span aria-hidden="true">+</span>
          添加门店
        </button>
      </header>

      <section className="toolbar" aria-label="门店筛选">
        <label className="search-field">
          <span aria-hidden="true">⌕</span>
          <input
            value={query}
            onChange={(event) => handleSearch(event.target.value)}
            placeholder="搜索门店名称"
            aria-label="搜索门店名称"
          />
        </label>
        <div className="toolbar-meta">
          共 {total} 家门店
          {total > 0 ? `，当前 ${firstIndex}-${lastIndex}` : ""}
        </div>
      </section>

      {toast ? (
        <div className="toast" role="status">
          {toast}
          <button onClick={() => setToast("")} aria-label="关闭提示">
            ×
          </button>
        </div>
      ) : null}

      <section className="table-frame" aria-label="门店列表">
        <table>
          <thead>
            <tr>
              <th>序号</th>
              <th>门店名称</th>
              <th>缩略图</th>
              <th>治疗室</th>
              <th>面诊室</th>
              <th>生美</th>
              <th>区域总数</th>
              <th>配置状态</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={10} className="empty-cell">
                  正在加载门店列表
                </td>
              </tr>
            ) : stores.length === 0 ? (
              <tr>
                <td colSpan={10} className="empty-cell">
                  <div className="empty-state">
                    <div className="empty-illustration" aria-hidden="true">
                      <span />
                      <span />
                      <span />
                    </div>
                    <strong>暂无门店设计图</strong>
                    <p>添加门店后，可在这里查看图纸、区域数量和配置状态。</p>
                  </div>
                </td>
              </tr>
            ) : (
              stores.map((store, index) => (
                <tr key={store.id}>
                  <td>{(page - 1) * PAGE_SIZE + index + 1}</td>
                  <td className="store-name">{store.name}</td>
                  <td>
                    <div className="thumb-preview">
                      <img src={store.thumbnailUrl} alt={`${store.name} 缩略图`} />
                      <div className="thumb-popover" role="presentation">
                        <img src={store.thumbnailUrl} alt="" />
                      </div>
                    </div>
                  </td>
                  <td>{store.treatmentCount}</td>
                  <td>{store.consultationCount}</td>
                  <td>{store.beautyCount}</td>
                  <td>{store.areaCount}</td>
                  <td>
                    <span className={`status-pill status-${store.status}`}>{statusLabels[store.status]}</span>
                  </td>
                  <td>{formatDateTime(store.updatedAt)}</td>
                  <td>
                    <div className="row-actions">
                      <button onClick={() => void openEditEditor(store.id)}>编辑</button>
                      <button className="danger-link" onClick={() => void deleteStore(store)}>
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </section>

      <nav className="pagination" aria-label="分页">
        <button disabled={page <= 1} onClick={() => setPage((value) => Math.max(1, value - 1))}>
          上一页
        </button>
        <span>
          第 {page} / {pageCount} 页
        </span>
        <button disabled={page >= pageCount} onClick={() => setPage((value) => Math.min(pageCount, value + 1))}>
          下一页
        </button>
      </nav>

      <footer className="app-version" aria-label="当前版本">
        版本 {APP_VERSION}
      </footer>

      {editor ? (
        <section className="editor-overlay" aria-label={editor.mode === "create" ? "添加门店" : "编辑门店"}>
          <div className="editor-topbar">
            <label className="store-input">
              <span>门店名称</span>
              <input
                value={editor.storeName}
                onChange={(event) => updateEditor({ storeName: event.target.value })}
                placeholder="请输入门店名称"
              />
            </label>
            <div className="file-zone">
              <span className="file-name">{editor.fileName || "尚未上传 PDF"}</span>
              <input
                ref={fileInputRef}
                className="visually-hidden"
                type="file"
                accept="application/pdf"
                onChange={(event) => handlePdfSelected(event.target.files)}
              />
              <button onClick={requestPdfUpload}>
                {editor.previewUrl ? "重新上传 PDF" : "上传 PDF"}
              </button>
            </div>
            <div className="topbar-actions">
              <button onClick={closeEditor}>取消</button>
              <button className="primary-button" disabled={editor.uploadStage === "recognizing" || saving} onClick={() => void saveEditor()}>
                {saving ? "保存中" : "保存"}
              </button>
            </div>
          </div>

          {(validation.fieldErrors.length > 0 || validationAreaErrorCount > 0) && (
            <div className="editor-alert" role="alert">
              {validation.fieldErrors[0] || `还有 ${validationAreaErrorCount} 个区域未完善。`}
            </div>
          )}

          <div className="editor-body">
            <div className="plan-pane">
              <div className="plan-toolbar">
                <div>
                  <strong>图纸预览</strong>
                  <span>{stageText(editor.uploadStage)}</span>
                </div>
                <div className="view-actions" aria-label="图纸查看工具">
                  <button onClick={() => setPlanZoom((value) => Math.max(0.7, Number((value - 0.15).toFixed(2))))}>−</button>
                  <button onClick={() => setPlanZoom(1)}>适应</button>
                  <button onClick={() => setPlanZoom((value) => Math.min(1.8, Number((value + 0.15).toFixed(2))))}>+</button>
                </div>
              </div>

              <div className={`plan-canvas stage-${editor.uploadStage}`}>
                {editor.previewUrl ? (
                  <div className="plan-image-wrap" ref={planRef} style={{ width: `min(${92 * planZoom}%, ${980 * planZoom}px)` }}>
                    <img src={editor.previewUrl} alt="设计图预览" draggable={false} />
                    {editor.areas.map((areaItem) =>
                      areaItem.box ? (
                        <div
                          key={areaItem.id}
                          role="button"
                          tabIndex={0}
                          className={`area-box area-${areaItem.type || "unknown"} ${
                            areaItem.id === editor.selectedAreaId ? "is-selected" : ""
                          }`}
                          style={boxStyle(areaItem.box)}
                          onClick={() => selectArea(areaItem.id)}
                          onPointerDown={(event) => {
                            if (!areaItem.box) return;
                            event.preventDefault();
                            selectArea(areaItem.id);
                            setDragState({
                              areaId: areaItem.id,
                              mode: "move",
                              startX: event.clientX,
                              startY: event.clientY,
                              origin: areaItem.box,
                            });
                          }}
                        >
                          <span className="area-box-label">
                            <strong>{areaBoxPrimaryLabel(areaItem)}</strong>
                            <em>{areaBoxSecondaryLabel(areaItem)}</em>
                          </span>
                          {(["nw", "ne", "sw", "se"] as ResizeHandle[]).map((handle) => (
                            <span
                              aria-hidden="true"
                              className={`resize-handle resize-${handle}`}
                              key={handle}
                              onPointerDown={(event) => {
                                if (!areaItem.box) return;
                                event.preventDefault();
                                event.stopPropagation();
                                selectArea(areaItem.id);
                                setDragState({
                                  areaId: areaItem.id,
                                  mode: "resize",
                                  handle,
                                  startX: event.clientX,
                                  startY: event.clientY,
                                  origin: areaItem.box,
                                });
                              }}
                            />
                          ))}
                        </div>
                      ) : null,
                    )}
                    {editor.uploadStage === "recognizing" ? <div className="scan-line" aria-hidden="true" /> : null}
                  </div>
                ) : (
                  <div className="upload-placeholder">
                    <strong>等待上传设计图 PDF</strong>
                    <p>上传门店装修设计图后，系统会生成预览并辅助识别诊室区域。</p>
                    <button className="primary-button" onClick={requestPdfUpload}>
                      上传 PDF
                    </button>
                  </div>
                )}
              </div>
            </div>

            <aside className="area-pane">
              <div className="area-pane-header">
                <div>
                  <strong>区域卡片</strong>
                  <span>{editor.areas.length} 个区域</span>
                </div>
                <button onClick={addArea}>新增区域</button>
              </div>

              {editor.uploadStage === "recognizing" ? (
                <div className="recognizing-panel">
                  <span />
                  <span />
                  <span />
                  <p>正在识别门店与区域</p>
                </div>
              ) : editor.uploadStage === "failed" && editor.areas.length === 0 ? (
                <div className="manual-panel">
                  <strong>识别失败，可手动维护</strong>
                  <p>请填写门店名称，新增区域并调整左侧高亮框后保存。</p>
                </div>
              ) : null}

              <div className="area-list">
                {editor.areas.map((areaItem, index) => {
                  const errors = validation.areaErrors[areaItem.id] ?? [];
                  return (
                    <article
                      ref={(node) => {
                        if (node) {
                          areaCardRefs.current[areaItem.id] = node;
                        } else {
                          delete areaCardRefs.current[areaItem.id];
                        }
                      }}
                      className={`area-card area-card-${areaItem.type || "unknown"} ${
                        areaItem.id === selectedArea?.id ? "is-active" : ""
                      } ${
                        errors.length > 0 ? "has-error" : ""
                      }`}
                      key={areaItem.id}
                      onClick={() => selectArea(areaItem.id)}
                    >
                      <div className="area-card-head">
                        <span className={`type-dot area-${areaItem.type || "unknown"}`} aria-hidden="true" />
                        <strong>{areaDisplayName(areaItem) || `区域 ${index + 1}`}</strong>
                        <span className="area-card-subtitle">{areaSummary(areaItem)}</span>
                        {areaItem.needsReview ? <span className="review-tag">需确认</span> : null}
                      </div>
                      <div className="area-form-grid">
                        <label>
                          区域类型
                          <select
                            value={areaItem.type}
                            onChange={(event) => updateArea(areaItem.id, { type: event.target.value as AreaType | "" })}
                          >
                            <option value="">请选择</option>
                            <option value="treatment">治疗室</option>
                            <option value="consultation">面诊室</option>
                            <option value="beauty">生美</option>
                          </select>
                        </label>
                        <label>
                          编号
                          <input
                            value={areaItem.number}
                            onChange={(event) => updateArea(areaItem.id, { number: event.target.value })}
                            inputMode="numeric"
                            placeholder={areaItem.type === "beauty" ? "可选" : "必填"}
                          />
                        </label>
                      </div>
                      {errors.length > 0 ? <p className="area-error">{errors.join("；")}</p> : null}
                      <div className="area-card-actions">
                        <button disabled={index === 0} onClick={() => moveArea(areaItem.id, -1)}>
                          上移
                        </button>
                        <button disabled={index === editor.areas.length - 1} onClick={() => moveArea(areaItem.id, 1)}>
                          下移
                        </button>
                        <button className="danger-link" onClick={() => deleteArea(areaItem.id)}>
                          删除
                        </button>
                      </div>
                    </article>
                  );
                })}
              </div>
            </aside>
          </div>
        </section>
      ) : null}
    </main>
  );
}

function validateEditor(editor: EditorState): ValidationResult {
  const fieldErrors: string[] = [];
  const areaErrors: Record<string, string[]> = {};
  const seenNumbers = new Map<string, string>();

  if (!editor.storeName.trim()) {
    fieldErrors.push("门店名称不能为空");
  }
  if (!editor.previewUrl || editor.uploadStage === "initial" || editor.uploadStage === "converting") {
    fieldErrors.push("必须已上传并成功转换 PDF");
  }
  if (editor.areas.length < 1) {
    fieldErrors.push("至少需要 1 个区域");
  }

  editor.areas.forEach((rawAreaItem) => {
    const areaItem = withGeneratedAreaFields(rawAreaItem);
    const errors: string[] = [];
    if (!areaItem.name.trim()) errors.push("区域名称不能为空");
    if (!areaItem.type) errors.push("区域类型不能为空");
    if (!areaItem.box) errors.push("高亮框不能为空");
    if ((areaItem.type === "treatment" || areaItem.type === "consultation") && !areaItem.number.trim()) {
      errors.push(areaItem.type === "treatment" ? "治疗室编号不能为空" : "面诊室编号不能为空");
    }
    if (areaItem.number.trim() && !/^\d+$/.test(areaItem.number.trim())) {
      errors.push("编号只能填写数字");
    }
    if (areaItem.type && areaItem.number.trim() && /^\d+$/.test(areaItem.number.trim())) {
      const key = `${areaItem.type}:${Number(areaItem.number)}`;
      if (seenNumbers.has(key)) {
        errors.push("同类型下编号不能重复");
        const firstAreaId = seenNumbers.get(key);
        if (firstAreaId) {
          areaErrors[firstAreaId] = [...(areaErrors[firstAreaId] ?? []), "同类型下编号不能重复"];
        }
      } else {
        seenNumbers.set(key, areaItem.id);
      }
    }
    if (errors.length > 0) {
      areaErrors[areaItem.id] = [...(areaErrors[areaItem.id] ?? []), ...errors];
    }
  });

  return { fieldErrors, areaErrors };
}

function boxStyle(box: AreaBox) {
  return {
    left: `${box.x * 100}%`,
    top: `${box.y * 100}%`,
    width: `${box.width * 100}%`,
    height: `${box.height * 100}%`,
  };
}

function clampBox(box: AreaBox): AreaBox {
  const width = Math.min(0.8, Math.max(0.04, box.width));
  const height = Math.min(0.8, Math.max(0.04, box.height));
  return {
    width,
    height,
    x: Math.min(1 - width, Math.max(0, box.x)),
    y: Math.min(1 - height, Math.max(0, box.y)),
  };
}

function resizeBox(origin: AreaBox, handle: ResizeHandle, dx: number, dy: number): AreaBox {
  switch (handle) {
    case "nw":
      return {
        x: origin.x + dx,
        y: origin.y + dy,
        width: origin.width - dx,
        height: origin.height - dy,
      };
    case "ne":
      return {
        x: origin.x,
        y: origin.y + dy,
        width: origin.width + dx,
        height: origin.height - dy,
      };
    case "sw":
      return {
        x: origin.x + dx,
        y: origin.y,
        width: origin.width - dx,
        height: origin.height + dy,
      };
    case "se":
      return {
        x: origin.x,
        y: origin.y,
        width: origin.width + dx,
        height: origin.height + dy,
      };
  }
}

function areaDisplayName(area: StoreArea) {
  if (!area.type) return "";
  if (area.number.trim()) return `${areaTypeLabels[area.type]} ${area.number.trim()}`;
  return area.type === "beauty" ? areaTypeLabels[area.type] : "";
}

function areaBoxPrimaryLabel(area: StoreArea) {
  if (area.number.trim()) return area.number.trim();
  if (area.type) return areaTypeLabels[area.type];
  return "未分类";
}

function areaBoxSecondaryLabel(area: StoreArea) {
  if (!area.type || !area.number.trim()) return "";
  return areaTypeLabels[area.type];
}

function areaSummary(area: StoreArea) {
  if (!area.type) return "未选择类型";
  const label = areaTypeLabels[area.type];
  if (!area.number) return area.type === "beauty" ? label : `${label} · 未编号`;
  return `${label} · 编号 ${area.number}`;
}

function withGeneratedAreaFields(area: StoreArea): StoreArea {
  return {
    ...area,
    name: areaDisplayName(area),
  };
}

function normalizeAreaForSave(area: StoreArea): StoreArea {
  const generated = withGeneratedAreaFields(area);
  const isComplete =
    Boolean(generated.type) &&
    Boolean(generated.box) &&
    Boolean(generated.name.trim()) &&
    (generated.type === "beauty" || Boolean(generated.number.trim()));
  return {
    ...generated,
    confidence: isComplete ? "high" : generated.confidence,
    needsReview: isComplete ? false : generated.needsReview,
  };
}

function planFileNameForStore(storeName: string, currentFileName: string) {
  const baseName = storeName
    .trim()
    .replace(/\s+/g, "-")
    .replace(/[\\/:*?"<>|]/g, "")
    .replace(/^-+|-+$/g, "");
  if (baseName) {
    return `${baseName}-design-plan.pdf`;
  }
  return currentFileName;
}

function stageText(stage: UploadStage) {
  const stageMap: Record<UploadStage, string> = {
    initial: "待上传",
    converting: "解析图纸中",
    recognizing: "识别区域中",
    ready: "可编辑",
    failed: "识别失败，可手动维护",
  };
  return stageMap[stage];
}

function formatDateTime(value: string) {
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return fallback;
}

export default App;
