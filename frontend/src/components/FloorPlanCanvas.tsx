import type { StoreArea } from "../api";
import { areaBoxPrimaryLabel, areaBoxSecondaryLabel, boxStyle } from "../domain/areas";
import type { DragState, ResizeHandle, UploadStage } from "../domain/designPlan";

type FloorPlanCanvasProps = {
  previewUrl: string;
  uploadStage: UploadStage;
  areas: StoreArea[];
  selectedAreaId: string | null;
  planZoom: number;
  planRef: React.RefObject<HTMLDivElement | null>;
  onRequestUpload: () => void;
  onSelectArea: (areaId: string) => void;
  onStartDrag: (drag: DragState) => void;
};

export function FloorPlanCanvas({
  previewUrl,
  uploadStage,
  areas,
  selectedAreaId,
  planZoom,
  planRef,
  onRequestUpload,
  onSelectArea,
  onStartDrag,
}: FloorPlanCanvasProps) {
  return (
    <div className={`plan-canvas stage-${uploadStage}`}>
      {previewUrl ? (
        <div className="plan-image-wrap" ref={planRef} style={{ width: `min(${92 * planZoom}%, ${980 * planZoom}px)` }}>
          <img src={previewUrl} alt="设计图预览" draggable={false} />
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
