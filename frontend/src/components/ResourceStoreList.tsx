import type { ResourceStoreSummary } from "../api";

type ResourceStoreListProps = {
  stores: ResourceStoreSummary[];
  loading: boolean;
  page: number;
  pageSize: number;
  openingStoreIds: Set<number>;
  onOpenStore: (tenantId: number) => void;
  onOpenMonitor: (store: ResourceStoreSummary) => void;
};

export function ResourceStoreList({
  stores,
  loading,
  page,
  pageSize,
  openingStoreIds,
  onOpenStore,
  onOpenMonitor,
}: ResourceStoreListProps) {
  return (
    <section className="table-frame" aria-label="门店空间资源查看列表">
      <table className="store-table resource-store-table">
        <thead>
          <tr>
            <th>序号</th>
            <th>城市</th>
            <th>门店名称</th>
            <th>机构 ID</th>
            <th>工控机</th>
            <th>NVR</th>
            <th>摄像头</th>
            <th>空间</th>
            <th>已绑定</th>
            <th>未绑定</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {loading ? (
            <tr>
              <td colSpan={11} className="empty-cell">
                正在加载门店空间资源
              </td>
            </tr>
          ) : stores.length === 0 ? (
            <tr>
              <td colSpan={11} className="empty-cell">
                <div className="empty-state">
                  <strong>暂无已部署工控机的门店</strong>
                  <p>请确认业务库已配置工控机、空间和摄像头绑定关系。</p>
                </div>
              </td>
            </tr>
          ) : (
            stores.map((store, index) => {
              const isOpening = openingStoreIds.has(store.tenantId);
              return (
                <tr key={store.tenantId}>
                  <td className="store-index-cell">
                    <span className="store-index-number">{(page - 1) * pageSize + index + 1}</span>
                  </td>
                  <td>{store.cityName || `城市 ${store.cityId || "-"}`}</td>
                  <td className="store-name">{store.storeName || store.hospitalName || "-"}</td>
                  <td>{store.tenantId || "-"}</td>
                  <td>{store.edgeCount}</td>
                  <td>{store.nvrCount}</td>
                  <td>{store.cameraCount}</td>
                  <td>{store.spaceCount}</td>
                  <td>{store.boundCameraCount}</td>
                  <td>{store.unboundCameraCount}</td>
                  <td>
                    <div className="row-actions">
                      <button disabled={isOpening} onClick={() => onOpenStore(store.tenantId)}>
                        {isOpening ? (
                          <>
                            <span className="button-spinner" aria-hidden="true" />
                            进入中
                          </>
                        ) : (
                          "详情"
                        )}
                      </button>
                      {store.canViewMonitor && store.monitorUrl ? (
                        <button className="secondary-action-button" onClick={() => onOpenMonitor(store)}>
                          查看监控
                        </button>
                      ) : null}
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
