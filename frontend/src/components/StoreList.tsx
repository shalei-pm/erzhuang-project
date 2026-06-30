import type { DesignPlanStatus, StoreSummary } from "../api";
import { formatDateTime } from "../domain/format";

const designPlanStatusLabels: Record<DesignPlanStatus, string> = {
  not_uploaded: "未上传",
  pending_recognition: "待识别",
  pending_annotation: "待标注",
  completed: "已完成",
};

type StoreListProps = {
  stores: StoreSummary[];
  loading: boolean;
  page: number;
  pageSize: number;
  deletingStoreIds: Set<number>;
  openingStoreIds: Set<number>;
  onOpenStore: (storeId: number) => void;
  onEditStore: (store: StoreSummary) => void;
  onDeleteStore: (store: StoreSummary) => void;
};

export function StoreList({ stores, loading, page, pageSize, deletingStoreIds, openingStoreIds, onOpenStore, onEditStore, onDeleteStore }: StoreListProps) {
  return (
    <section className="table-frame" aria-label="门店列表">
      <table className="store-table">
        <thead>
          <tr>
            <th>序号</th>
            <th>城市</th>
            <th>门店名称</th>
            <th>新氧机构 ID</th>
            <th>设计图状态</th>
            <th>录像机</th>
            <th>通道</th>
            <th>面诊室</th>
            <th>治疗室</th>
            <th>美容室</th>
            <th>更新时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {loading ? (
            <tr>
              <td colSpan={12} className="empty-cell">
                正在加载门店列表
              </td>
            </tr>
          ) : stores.length === 0 ? (
            <tr>
              <td colSpan={12} className="empty-cell">
                <div className="empty-state">
                  <div className="empty-illustration" aria-hidden="true">
                    <span />
                    <span />
                    <span />
                  </div>
                  <strong>暂无门店空间资源档案</strong>
                  <p>添加门店后，可维护设计图标注、录像机和通道映射。</p>
                </div>
              </td>
            </tr>
          ) : (
            stores.map((store, index) => {
              const isDeleting = deletingStoreIds.has(store.id);
              const isOpening = openingStoreIds.has(store.id);
              return (
                <tr className={store.channelsFullyConfirmed ? "is-channels-confirmed" : ""} key={store.id}>
                  <td className="store-index-cell">
                    <span className="store-index-number">{(page - 1) * pageSize + index + 1}</span>
                    {store.channelsFullyConfirmed ? (
                      <span className="store-confirmed-watermark" aria-hidden="true">
                        已确认
                      </span>
                    ) : null}
                  </td>
                  <td>{store.city || "未设置"}</td>
                  <td className="store-name">{store.name}</td>
                  <td>{store.externalOrgId || "-"}</td>
                  <td>
                    <span className={`status-pill design-status-${store.designPlanStatus}`}>
                      {designPlanStatusLabels[store.designPlanStatus]}
                    </span>
                  </td>
                  <td>{store.recorderCount}</td>
                  <td>{store.channelCount}</td>
                  <td>{store.consultationCount}</td>
                  <td>{store.treatmentCount}</td>
                  <td>{store.beautyCount}</td>
                  <td>{formatDateTime(store.updatedAt)}</td>
                  <td>
                    <div className="row-actions">
                      <button disabled={isDeleting || isOpening} onClick={() => onOpenStore(store.id)}>
                        {isOpening ? (
                          <>
                            <span className="button-spinner" aria-hidden="true" />
                            进入中
                          </>
                        ) : (
                          "详情"
                        )}
                      </button>
                      <button disabled={isDeleting || isOpening} onClick={() => onEditStore(store)}>
                        编辑
                      </button>
                      <button className="danger-link" disabled={isDeleting || isOpening} onClick={() => onDeleteStore(store)}>
                        {isDeleting ? (
                          <>
                            <span className="button-spinner" aria-hidden="true" />
                            删除中
                          </>
                        ) : (
                          "删除"
                        )}
                      </button>
                    </div>
                  </td>
                </tr>
              );
            })
          )}
        </tbody>
      </table>
    </section>
  );
}
