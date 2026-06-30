import type { AreaType, StoreArea } from "../api";
import { areaDisplayName, areaSummary, isAreaNumberOptional } from "../domain/areas";

type AreaCardListProps = {
  areas: StoreArea[];
  selectedAreaId: string | null;
  areaErrors: Record<string, string[]>;
  areaCardRefs: React.MutableRefObject<Record<string, HTMLElement | null>>;
  onSelectArea: (areaId: string) => void;
  onUpdateArea: (areaId: string, patch: Partial<StoreArea>) => void;
  onMoveArea: (areaId: string, direction: -1 | 1) => void;
  onDeleteArea: (areaId: string) => void;
};

export function AreaCardList({
  areas,
  selectedAreaId,
  areaErrors,
  areaCardRefs,
  onSelectArea,
  onUpdateArea,
  onMoveArea,
  onDeleteArea,
}: AreaCardListProps) {
  return (
    <div className="area-list">
      {areas.map((areaItem, index) => {
        const errors = areaErrors[areaItem.id] ?? [];
        const lockedByChannel = areaItem.source === "video_channel" || areaItem.source === "multiple";
        const numberOptional = isAreaNumberOptional(areaItem.type);
        return (
          <article
            ref={(node) => {
              if (node) {
                areaCardRefs.current[areaItem.id] = node;
              } else {
                delete areaCardRefs.current[areaItem.id];
              }
            }}
            className={`area-card area-card-${areaItem.type || "unknown"} ${areaItem.id === selectedAreaId ? "is-active" : ""} ${
              errors.length > 0 ? "has-error" : ""
            }`}
            key={areaItem.id}
            onClick={() => onSelectArea(areaItem.id)}
          >
            <div className="area-card-head">
              <span className={`type-dot area-${areaItem.type || "unknown"}`} aria-hidden="true" />
              <strong>{areaDisplayName(areaItem) || `区域 ${index + 1}`}</strong>
              <span className="area-card-subtitle">{areaSummary(areaItem)}</span>
              {areaItem.needsReview ? <span className="review-tag">需确认</span> : null}
              {lockedByChannel ? <span className="review-tag is-confirmed">通道已确认</span> : null}
            </div>
            <div className="area-form-grid">
              <label>
                区域类型
                <select
                  value={areaItem.type}
                  disabled={lockedByChannel}
                  onChange={(event) => onUpdateArea(areaItem.id, { type: event.target.value as AreaType | "" })}
                >
                  <option value="">请选择</option>
                  <option value="treatment">治疗室</option>
                  <option value="vip_treatment">VIP治疗室</option>
                  <option value="consultation">面诊室</option>
                  <option value="beauty">美容室</option>
                </select>
              </label>
              <label>
                编号
                <input
                  value={areaItem.number}
                  disabled={lockedByChannel}
                  onChange={(event) => onUpdateArea(areaItem.id, { number: event.target.value })}
                  inputMode="numeric"
                  placeholder={numberOptional ? "-" : "必填"}
                />
              </label>
            </div>
            {lockedByChannel ? <p className="area-card-note">类型和编号由通道映射维护；这里仅补充或调整图纸标注框。</p> : null}
            {errors.length > 0 ? <p className="area-error">{errors.join("；")}</p> : null}
            <div className="area-card-actions">
              <button disabled={index === 0} onClick={() => onMoveArea(areaItem.id, -1)}>
                上移
              </button>
              <button disabled={index === areas.length - 1} onClick={() => onMoveArea(areaItem.id, 1)}>
                下移
              </button>
              <button className="danger-link" onClick={() => onDeleteArea(areaItem.id)}>
                删除
              </button>
            </div>
          </article>
        );
      })}
    </div>
  );
}
