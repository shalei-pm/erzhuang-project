import type { ResourceStoreDetail as ResourceStoreDetailType } from "../api";
import { formatDateTime } from "../domain/format";
import { buildCameraBindingRows, type CameraBindingPath } from "../domain/resource-view";

type ResourceStoreDetailProps = {
  store: ResourceStoreDetailType;
  onOpenMonitor: (url: string) => void;
};

export function ResourceStoreDetail({ store, onOpenMonitor }: ResourceStoreDetailProps) {
  const rows = buildCameraBindingRows(store);
  const boundCameraCount = rows.filter((row) => row.isBound).length;
  const unboundCameraCount = rows.length - boundCameraCount;

  return (
    <section className="detail-page resource-detail-page">
      <header className="detail-header">
        <div className="detail-header-main">
          <div>
            <p className="eyebrow">门店空间资源查看</p>
            <h1>{store.storeName || store.hospitalName || `机构 ${store.tenantId}`}</h1>
          </div>
          <div className="detail-metrics resource-detail-metrics" aria-label="摄像头绑定概览">
            <Metric label="机构 ID" value={store.tenantId} />
            <Metric label="摄像头" value={rows.length} />
            <Metric label="已绑定" value={boundCameraCount} />
            <Metric label="未绑定" value={unboundCameraCount} />
            <Metric label="最近映射更新" value={formatDateTime(store.updatedAt)} />
            <Metric label="状态" value={store.camerasFullyBound ? "已确认" : "待确认"} />
          </div>
        </div>
        {store.canViewMonitor && store.monitorUrl ? (
          <div className="detail-header-side">
            <button className="secondary-action-button" onClick={() => onOpenMonitor(store.monitorUrl!)}>
              查看监控
            </button>
          </div>
        ) : null}
      </header>

      <section className="table-frame resource-binding-table-frame" aria-label="摄像头与空间绑定关系">
        <table className="resource-binding-table">
          <thead>
            <tr>
              <th>录像机编号</th>
              <th>通道号</th>
              <th>摄像头 ID</th>
              <th>最近截图</th>
              <th>绑定状态</th>
              <th>空间类型</th>
              <th>空间名称</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td className="empty-cell" colSpan={7}>
                  业务库暂无摄像头数据
                </td>
              </tr>
            ) : (
              rows.map((row) => (
                <tr key={row.camera.id}>
                  <td className="resource-recorder-cell">{row.recorderIdentifier}</td>
                  <td>{row.channelNo ?? "-"}</td>
                  <td>{row.camera.id}</td>
                  <td className="resource-snapshot-cell">-</td>
                  <td>
                    <span className={row.isBound ? "resource-binding-status is-bound" : "resource-binding-status is-unbound"}>
                      {row.isBound ? "已绑定" : "未绑定"}
                    </span>
                  </td>
                  <BindingPathCells paths={row.bindingPaths} />
                </tr>
              ))
            )}
          </tbody>
        </table>
      </section>
    </section>
  );
}

function Metric({ label, value }: { label: string; value: number | string }) {
  return (
    <div>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function BindingPathCells({ paths }: { paths: CameraBindingPath[] }) {
  if (paths.length === 0) {
    return <>
      <EmptyPathCell />
      <EmptyPathCell />
    </>;
  }

  return <>
    <PathCell paths={paths} field="spaceType" />
    <PathCell paths={paths} field="spaceName" />
  </>;
}

function EmptyPathCell() {
  return <td className="resource-empty-value">-</td>;
}

function PathCell({ paths, field }: { paths: CameraBindingPath[]; field: keyof CameraBindingPath }) {
  return (
    <td>
      <div className="resource-binding-path-values">
        {paths.map((path, index) => (
          <span key={`${field}-${index}`}>{path[field] || "-"}</span>
        ))}
      </div>
    </td>
  );
}
