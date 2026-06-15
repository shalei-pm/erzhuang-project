import type { StoreArea } from "../api";
import { areaBoxPrimaryLabel, areaBoxSecondaryLabel, boxStyle } from "../domain/areas";
import type { DragState, ResizeHandle, UploadStage } from "../domain/designPlan";

type FloorPlanCanvasProps = {
  previewUrl: string;
  pendingPreviewUrl?: string;
  uploadStage: UploadStage;
  areas: StoreArea[];
  selectedAreaId: string | null;
  planZoom: number;
  planRef: React.RefObject<HTMLDivElement | null>;
  onRequestUpload: () => void;
  onSelectArea: (areaId: string) => void;
  onStartDrag: (drag: DragState) => void;
  onPendingPreviewLoaded?: () => void;
  onPendingPreviewError?: () => void;
};

export function FloorPlanCanvas({
  previewUrl,
  pendingPreviewUrl = "",
  uploadStage,
  areas,
  selectedAreaId,
  planZoom,
  planRef,
  onRequestUpload,
  onSelectArea,
  onStartDrag,
  onPendingPreviewLoaded,
  onPendingPreviewError,
}: FloorPlanCanvasProps) {
  const visiblePreviewUrl = pendingPreviewUrl || previewUrl;
  const isLoadingPreview = uploadStage === "converting" && !pendingPreviewUrl;

  return (
    <div className={`plan-canvas stage-${uploadStage}`}>
      {isLoadingPreview ? (
        <div className="plan-loading-state" role="status" aria-live="polite">
          <span aria-hidden="true" />
          <strong>正在解析设计图</strong>
          <p>PDF 正在转换为可标注图片，请稍候。</p>
        </div>
      ) : visiblePreviewUrl ? (
        <div className="plan-image-wrap" ref={planRef} style={{ width: `min(${92 * planZoom}%, ${980 * planZoom}px)` }}>
          <img
            src={visiblePreviewUrl}
            alt="设计图预览"
            draggable={false}
            onLoad={() => {
              if (pendingPreviewUrl) {
                onPendingPreviewLoaded?.();
              }
            }}
            onError={() => {
              if (pendingPreviewUrl) {
                onPendingPreviewError?.();
              }
            }}
          />
          {areas.map((areaItem) =>
            areaItem.box ? (
              <div
                key={areaItem.id}
                role="button"
                tabIndex={0}
                className={`area-box area-${areaItem.type || "unknown"} ${
                  areaItem.id === selectedAreaId ? "is-selected" : ""
                } ${areaItem.needsReview ? "is-pending" : ""}`}
                style={boxStyle(areaItem.box)}
                onClick={() => onSelectArea(areaItem.id)}
                onPointerDown={(event) => {
                  if (!areaItem.box) return;
                  event.preventDefault();
                  onSelectArea(areaItem.id);
                  onStartDrag({
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
                  <em>{areaItem.needsReview ? "待标注" : areaBoxSecondaryLabel(areaItem)}</em>
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
                      onSelectArea(areaItem.id);
                      onStartDrag({
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
          {uploadStage === "recognizing" ? <div className="scan-line" aria-hidden="true" /> : null}
          {pendingPreviewUrl ? (
            <div className="plan-loading-overlay" role="status" aria-live="polite">
              <span aria-hidden="true" />
              <strong>正在加载新图纸</strong>
            </div>
          ) : null}
        </div>
      ) : (
        <div className="upload-placeholder">
          <strong>等待上传设计图 PDF</strong>
          <p>上传门店装修设计图后，可以在这里维护业务区域和图纸矩形框。</p>
          <button className="primary-button" onClick={onRequestUpload}>
            上传 PDF
          </button>
        </div>
      )}
    </div>
  );
}
