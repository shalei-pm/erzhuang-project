import { useEffect, useMemo, useRef, useState } from "react";
import { storeSpaceApi, type AreaType, type StoreArea, type StoreDetail } from "../api";
import { createManualArea, mergeRecognizedAreas, normalizeAreaForSave, withGeneratedAreaFields } from "../domain/areas";
import { clampBox, type DragState, planFileNameForStore, resizeBox, stageText, type UploadStage } from "../domain/designPlan";
import { errorMessage } from "../domain/format";
import { AreaCardList } from "./AreaCardList";
import { FloorPlanCanvas } from "./FloorPlanCanvas";

const MAX_PDF_BYTES = 5 * 1024 * 1024;

type ValidationResult = {
  fieldErrors: string[];
  areaErrors: Record<string, string[]>;
};

const emptyValidation: ValidationResult = { fieldErrors: [], areaErrors: {} };

type DesignPlanTabProps = {
  store: StoreDetail;
  saving: boolean;
  onStoreUpdated: (store: StoreDetail) => void;
  onToast: (message: string) => void;
};

export function DesignPlanTab({ store, saving, onStoreUpdated, onToast }: DesignPlanTabProps) {
  const [fileName, setFileName] = useState(store.fileName);
  const [uploadId, setUploadId] = useState<string | undefined>();
  const [originalPath, setOriginalPath] = useState(store.originalPath);
  const [previewPath, setPreviewPath] = useState(store.previewPath);
  const [thumbnailPath, setThumbnailPath] = useState(store.thumbnailPath);
  const [pageCount, setPageCount] = useState(store.pageCount);
  const [previewUrl, setPreviewUrl] = useState(store.previewUrl);
  const [pendingPreviewUrl, setPendingPreviewUrl] = useState("");
  const [thumbnailUrl, setThumbnailUrl] = useState(store.thumbnailUrl);
  const [recognitionResult, setRecognitionResult] = useState<unknown>(store.recognitionResult);
  const [uploadStage, setUploadStage] = useState<UploadStage>(store.previewUrl ? "ready" : "initial");
  const [uploadMessage, setUploadMessage] = useState("");
  const [areas, setAreas] = useState<StoreArea[]>(store.areas);
  const [selectedAreaId, setSelectedAreaId] = useState<string | null>(store.areas[0]?.id ?? null);
  const [validation, setValidation] = useState<ValidationResult>(emptyValidation);
  const [dragState, setDragState] = useState<DragState | null>(null);
  const [planZoom, setPlanZoom] = useState(1);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const planRef = useRef<HTMLDivElement | null>(null);
  const areaPaneRef = useRef<HTMLElement | null>(null);
  const areaCardRefs = useRef<Record<string, HTMLElement | null>>({});
  const previousPreviewRef = useRef<{ previewUrl: string; areas: StoreArea[] } | null>(null);

  const validationAreaErrorCount = Object.keys(validation.areaErrors).length;
  const designStatus = useMemo(() => {
    if (!previewUrl) return "未上传";
    if (areas.length === 0) return "待识别";
    if (areas.some((area) => area.needsReview || !area.box)) return "待标注";
    return "已完成";
  }, [areas, previewUrl]);

  useEffect(() => {
    setFileName(store.fileName);
    setOriginalPath(store.originalPath);
    setPreviewPath(store.previewPath);
    setThumbnailPath(store.thumbnailPath);
    setPageCount(store.pageCount);
    setPreviewUrl(store.previewUrl);
    setPendingPreviewUrl("");
    setThumbnailUrl(store.thumbnailUrl);
    setRecognitionResult(store.recognitionResult);
    setUploadStage(store.previewUrl ? "ready" : "initial");
    setAreas(store.areas);
    selectArea(store.areas[0]?.id ?? null);
  }, [store]);

  useEffect(() => {
    if (!selectedAreaId) return;
    scrollToAreaCard(selectedAreaId);
  }, [selectedAreaId]);

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

  function requestPdfUpload() {
    fileInputRef.current?.click();
  }

  function commitPendingPreview() {
    if (!pendingPreviewUrl) return;
    setPreviewUrl(pendingPreviewUrl);
    setPendingPreviewUrl("");
    previousPreviewRef.current = null;
    setUploadMessage("图纸预览已生成，请手动点击识别或直接维护区域。");
  }

  function rollbackPendingPreview() {
    const previous = previousPreviewRef.current;
    if (previous) {
      setPreviewUrl(previous.previewUrl);
      setAreas(previous.areas);
      selectArea(previous.areas[0]?.id ?? null);
    }
    previousPreviewRef.current = null;
    setPendingPreviewUrl("");
    setUploadStage(previous?.previewUrl ? "ready" : "failed");
    const message = "新图纸加载失败，已恢复原设计图。";
    setUploadMessage(message);
    onToast(message);
  }

  async function handlePdfSelected(fileList: FileList | null) {
    const file = fileList?.[0];
    if (!file) return;
    if (file.type !== "application/pdf" && !file.name.toLowerCase().endsWith(".pdf")) {
      onToast("仅支持上传 PDF 文件。");
      return;
    }
    if (file.size > MAX_PDF_BYTES) {
      onToast("文件过大，请上传 5MB 以内的 PDF。");
      return;
    }
    setValidation(emptyValidation);
    setUploadStage("converting");
    setUploadMessage("正在解析 PDF 并生成图纸预览。");
    const previousPreviewUrl = previewUrl;
    const previousAreas = areas;
    previousPreviewRef.current = { previewUrl: previousPreviewUrl, areas: previousAreas };
    setPendingPreviewUrl("");
    setAreas([]);
    setSelectedAreaId(null);
    try {
      const upload = await storeSpaceApi.uploadPdf(file);
      setUploadId(upload.uploadId);
      setFileName(upload.fileName);
      setOriginalPath(upload.originalPath);
      setPreviewPath(upload.previewPath);
      setThumbnailPath(upload.thumbnailPath);
      setPageCount(upload.pageCount);
      setThumbnailUrl(upload.thumbnailUrl);
      setPendingPreviewUrl(upload.previewUrl);
      setUploadStage("ready");
      setUploadMessage("图纸预览已生成，正在加载新图纸。");
    } catch (error) {
      const message = errorMessage(error, "设计图解析失败，请重新上传 PDF。");
      setPreviewUrl(previousPreviewUrl);
      setPendingPreviewUrl("");
      setAreas(previousAreas);
      selectArea(previousAreas[0]?.id ?? null);
      previousPreviewRef.current = null;
      setUploadStage("failed");
      setUploadMessage(message);
      onToast(message);
    }
  }

  async function recognizeDesignPlan() {
    if (!uploadId && !previewUrl) {
      onToast("请先上传设计图 PDF。");
      return;
    }
    setUploadStage("recognizing");
    setUploadMessage("正在识别图纸区域，可切换页面，完成后返回查看结果。");
    try {
      const recognition = await storeSpaceApi.recognizeUpload(uploadId ?? `store-${store.id}`);
      const mergedAreas = mergeRecognizedAreas(areas, recognition.areas);
      setAreas(mergedAreas);
      selectArea(mergedAreas[0]?.id ?? null);
      setRecognitionResult(recognition.rawResult ?? recognition);
      setUploadStage("ready");
      setUploadMessage(
        recognition.areas.length > 0 ? `AI 识别完成：识别到 ${recognition.areas.length} 个目标区域。` : "AI 未识别到目标区域，可手动新增区域。",
      );
      if (recognition.storeName.trim()) {
        setFileName(planFileNameForStore(recognition.storeName, fileName));
      }
    } catch (error) {
      const message = errorMessage(error, "AI 识别失败，可手动维护。");
      setUploadStage("failed");
      setUploadMessage(message);
      onToast(message);
    }
  }

  function updateArea(areaId: string, patch: Partial<StoreArea>) {
    setAreas((items) => items.map((item) => (item.id === areaId ? withGeneratedAreaFields({ ...item, ...patch }) : item)));
  }

  function selectArea(areaId: string | null) {
    setSelectedAreaId(areaId);
    if (areaId) {
      scrollToAreaCard(areaId);
    }
  }

  function scrollToAreaCard(areaId: string) {
    window.requestAnimationFrame(() => {
      scrollAreaCardIntoPane(areaPaneRef.current, areaCardRefs.current[areaId]);
    });
  }

  function addArea() {
    const area = createManualArea();
    setAreas((items) => [...items, area]);
    selectArea(area.id);
  }

  function deleteArea(areaId: string) {
    setAreas((items) => items.filter((item) => item.id !== areaId));
    setSelectedAreaId((current) => (current === areaId ? null : current));
  }

  function moveArea(areaId: string, direction: -1 | 1) {
    const index = areas.findIndex((item) => item.id === areaId);
    const targetIndex = index + direction;
    if (index < 0 || targetIndex < 0 || targetIndex >= areas.length) return;
    const nextAreas = [...areas];
    const [item] = nextAreas.splice(index, 1);
    nextAreas.splice(targetIndex, 0, item);
    setAreas(nextAreas);
  }

  async function saveAnnotations() {
    const result = validateAreas(areas, Boolean(previewUrl));
    setValidation(result);
    if (result.fieldErrors.length > 0 || Object.keys(result.areaErrors).length > 0) {
      onToast(result.fieldErrors[0] || `还有 ${Object.keys(result.areaErrors).length} 个区域未完善。`);
      return;
    }
    const saved = await storeSpaceApi.saveStore({
      id: store.id,
      city: store.city,
      name: store.name,
      externalOrgId: store.externalOrgId,
      fileName,
      originalPath,
      previewPath,
      thumbnailPath,
      pageCount,
      previewUrl,
      thumbnailUrl: thumbnailUrl || previewUrl,
      uploadId,
      recognitionResult,
      areas: areas.map(normalizeAreaForSave),
      recorders: store.recorders,
    });
    onStoreUpdated(saved);
    onToast("设计图标注已保存。");
  }

  return (
    <section className="detail-tab-panel">
      {(validation.fieldErrors.length > 0 || validationAreaErrorCount > 0) && (
        <div className="editor-alert" role="alert">
          {validation.fieldErrors[0] || `还有 ${validationAreaErrorCount} 个区域未完善。`}
        </div>
      )}

      {uploadMessage ? (
        <div className={`editor-status editor-status-${uploadStage}`} role={uploadStage === "failed" ? "alert" : "status"}>
          {uploadMessage}
        </div>
      ) : null}

      <div className="editor-body detail-editor-body">
        <div className="plan-pane">
          <div className="plan-toolbar">
            <div>
              <strong>图纸预览</strong>
              <span>
                {stageText(uploadStage)} · {designStatus}
              </span>
            </div>
            <div className="view-actions" aria-label="图纸查看工具">
              <input
                ref={fileInputRef}
                className="visually-hidden"
                type="file"
                accept="application/pdf"
                onChange={(event) => void handlePdfSelected(event.target.files)}
              />
              <button onClick={requestPdfUpload} disabled={uploadStage === "converting"}>
                {previewUrl || pendingPreviewUrl ? "更换 PDF" : "上传 PDF"}
              </button>
              <button disabled={(!previewUrl && !pendingPreviewUrl) || uploadStage === "recognizing" || uploadStage === "converting"} onClick={() => void recognizeDesignPlan()}>
                识别图纸区域
              </button>
              <button onClick={() => setPlanZoom((value) => Math.max(0.7, Number((value - 0.15).toFixed(2))))}>-</button>
              <button onClick={() => setPlanZoom(1)}>适应</button>
              <button onClick={() => setPlanZoom((value) => Math.min(1.8, Number((value + 0.15).toFixed(2))))}>+</button>
            </div>
          </div>

          <FloorPlanCanvas
            previewUrl={previewUrl}
            pendingPreviewUrl={pendingPreviewUrl}
            uploadStage={uploadStage}
            areas={areas}
            selectedAreaId={selectedAreaId}
            planZoom={planZoom}
            planRef={planRef}
            onRequestUpload={requestPdfUpload}
            onSelectArea={selectArea}
            onStartDrag={setDragState}
            onPendingPreviewLoaded={commitPendingPreview}
            onPendingPreviewError={rollbackPendingPreview}
          />
        </div>

        <aside className="area-pane" ref={areaPaneRef}>
          <div className="area-pane-header">
            <div>
              <strong>区域卡片</strong>
              <span>{areas.length} 个区域</span>
            </div>
            <div className="row-actions">
              <button onClick={addArea}>新增区域</button>
              <button disabled={saving || uploadStage === "recognizing"} onClick={() => void saveAnnotations()}>
                保存标注
              </button>
            </div>
          </div>

          {uploadStage === "recognizing" ? (
            <div className="recognizing-panel">
              <span />
              <span />
              <span />
              <p>正在识别门店与区域</p>
            </div>
          ) : uploadStage === "failed" && areas.length === 0 ? (
            <div className="manual-panel">
              <strong>识别失败，可手动维护</strong>
              <p>新增区域后，可在左侧拖动矩形框到正确位置。</p>
            </div>
          ) : null}

          <AreaCardList
            areas={areas}
            selectedAreaId={selectedAreaId}
            areaErrors={validation.areaErrors}
            areaCardRefs={areaCardRefs}
            onSelectArea={selectArea}
            onUpdateArea={updateArea}
            onMoveArea={moveArea}
            onDeleteArea={deleteArea}
          />
        </aside>
      </div>
    </section>
  );
}

function scrollAreaCardIntoPane(container: HTMLElement | null, target: HTMLElement | null) {
  if (!container || !target) return;

  const containerRect = container.getBoundingClientRect();
  const targetRect = target.getBoundingClientRect();
  const targetTop = targetRect.top - containerRect.top + container.scrollTop;
  const centeredTop = targetTop - container.clientHeight / 2 + targetRect.height / 2;
  container.scrollTo({ top: Math.max(0, centeredTop), behavior: "auto" });
}

function validateAreas(areas: StoreArea[], hasPreview: boolean): ValidationResult {
  const fieldErrors: string[] = [];
  const areaErrors: Record<string, string[]> = {};
  const seenNumbers = new Map<string, string>();

  if (!hasPreview) {
    fieldErrors.push("必须已上传并成功转换 PDF");
  }

  areas.forEach((rawAreaItem) => {
    const areaItem = withGeneratedAreaFields(rawAreaItem);
    const errors: string[] = [];
    if (!areaItem.name.trim()) errors.push("区域名称不能为空");
    if (!areaItem.type) errors.push("区域类型不能为空");
    if (!areaItem.box) errors.push("高亮框不能为空");
    if (areaItem.type && !areaItem.number.trim()) {
      errors.push(`${areaTypeLabel(areaItem.type)}编号不能为空`);
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

function areaTypeLabel(type: AreaType) {
  if (type === "treatment") return "治疗室";
  if (type === "consultation") return "面诊室";
  return "生美";
}
