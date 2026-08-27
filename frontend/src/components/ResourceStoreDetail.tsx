import { useState } from "react";

import type { ResourceStoreDetail as ResourceStoreDetailType } from "../api";
import { formatDateTime } from "../domain/format";
import { cameraPlaceholderURL } from "../domain/camera-placeholder";
import {
  buildCameraBindingRows,
  cameraBindingSpaceTypeCounts,
  cameraBindingSpaceTypes,
  filterCameraBindingRows,
  type CameraBindingFilter,
  type CameraBindingPath,
} from "../domain/resource-view";

type ResourceStoreDetailProps = {
  store: ResourceStoreDetailType;
  onOpenMonitor: (url: string) => void;
};

export function ResourceStoreDetail({ store, onOpenMonitor }: ResourceStoreDetailProps) {
  const rows = buildCameraBindingRows(store);
  const [spaceTypeFilter, setSpaceTypeFilter] = useState<CameraBindingFilter>("all");
  const boundCameraCount = rows.filter((row) => row.isBound).length;
  const unboundCameraCount = rows.length - boundCameraCount;
  const spaceTypeCounts = cameraBindingSpaceTypeCounts(rows);
  const spaceTypes = cameraBindingSpaceTypes(rows);
  const activeSpaceTypeFilter = spaceTypeFilter === "all" || spaceTypeFilter === "unbound" || spaceTypes.includes(spaceTypeFilter) ? spaceTypeFilter : "all";
  const visibleRows = filterCameraBindingRows(rows, activeSpaceTypeFilter);

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

      <section className="resource-binding-section" aria-label="摄像头与空间绑定关系">
        <h2>摄像头列表</h2>
        {spaceTypes.length > 0 ? (
          <div className="resource-space-filter" aria-label="按空间类型筛选">
            <button
              className={activeSpaceTypeFilter === "all" ? "is-active" : undefined}
              onClick={() => setSpaceTypeFilter("all")}
              type="button"
            >
              全部
              <span>{rows.length}</span>
            </button>
            {spaceTypeCounts.map(({ spaceType, cameraCount }) => (
              <button
                className={activeSpaceTypeFilter === spaceType ? "is-active" : undefined}
                key={spaceType}
                onClick={() => setSpaceTypeFilter(spaceType)}
                type="button"
              >
                {spaceType}
                <span>{cameraCount}</span>
              </button>
            ))}
            <button
              className={activeSpaceTypeFilter === "unbound" ? "is-active" : undefined}
              onClick={() => setSpaceTypeFilter("unbound")}
              type="button"
            >
              未绑定
              <span>{unboundCameraCount}</span>
            </button>
          </div>
        ) : null}
        <div className="table-frame resource-binding-table-frame">
          <table className="resource-binding-table">
            <colgroup>
              <col className="resource-binding-col-camera" />
              <col className="resource-binding-col-recorder" />
              <col className="resource-binding-col-channel" />
              <col className="resource-binding-col-snapshot" />
              <col className="resource-binding-col-space-type" />
              <col className="resource-binding-col-space-name" />
              <col className="resource-binding-col-status" />
              <col className="resource-binding-col-action" />
            </colgroup>
            <thead>
              <tr>
                <th>摄像头 ID</th>
                <th>录像机编号</th>
                <th>通道号</th>
                <th>最近截图</th>
                <th>空间类型</th>
                <th>空间名称</th>
                <th>绑定状态</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {visibleRows.length === 0 ? (
                <tr>
                  <td className="empty-cell" colSpan={8}>
                    {activeSpaceTypeFilter === "all" ? "业务库暂无摄像头数据" : activeSpaceTypeFilter === "unbound" ? "暂无未绑定摄像头" : "该空间类型暂无已绑定摄像头"}
                  </td>
                </tr>
              ) : (
                visibleRows.map((row) => (
                  <tr key={row.camera.id}>
                    <td>{row.camera.id}</td>
                    <td className="resource-recorder-cell">{row.recorderIdentifier}</td>
                    <td>{row.channelNo ?? "-"}</td>
                    <td className="resource-snapshot-cell">
                      <SnapshotPreview src={row.camera.thumbnailUrl} thumbnailKind={row.camera.thumbnailKind} />
                    </td>
                    <BindingPathCells paths={row.bindingPaths} />
                    <td>
                      <span className={row.isBound ? "resource-binding-status is-bound" : "resource-binding-status is-unbound"}>
                        {row.isBound ? "已绑定" : "未绑定"}
                      </span>
                    </td>
                    <td>
                      <button className="resource-refresh-snapshot-button" disabled title="截图服务待接入">
                        刷新截图
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
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
    <td className={field === "spaceName" ? "resource-space-name-cell" : undefined}>
      <div className="resource-binding-path-values">
        {paths.map((path, index) => (
          <span key={`${field}-${index}`}>{path[field] || "-"}</span>
        ))}
      </div>
    </td>
  );
}

function SnapshotPreview({ src, thumbnailKind }: { src?: string; thumbnailKind?: string }) {
  const [failed, setFailed] = useState(false);
  const source = !failed && src ? src : cameraPlaceholderURL(thumbnailKind);
  return <img className="resource-snapshot-image" src={source} alt={!failed && src ? "摄像头最近截图" : "摄像头默认缩略图"} onError={() => setFailed(true)} />;
}
