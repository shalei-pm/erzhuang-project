import { useState } from "react";

import type { ResourceCamera, ResourceIssue, ResourceSpaceNode, ResourceStoreDetail as ResourceStoreDetailType } from "../api";
import { sortedResourceIssues } from "../domain/resource-view";

type ResourceDetailTab = "spaces" | "devices" | "issues";

type ResourceStoreDetailProps = {
  store: ResourceStoreDetailType;
  onOpenMonitor: (url: string) => void;
};

export function ResourceStoreDetail({ store, onOpenMonitor }: ResourceStoreDetailProps) {
  const [tab, setTab] = useState<ResourceDetailTab>("spaces");
  const issues = sortedResourceIssues(store.issues);

  return (
    <section className="detail-page resource-detail-page">
      <header className="detail-header">
        <div className="detail-header-main">
          <div>
            <p className="eyebrow">门店空间资源查看</p>
            <h1>{store.storeName || store.hospitalName || `机构 ${store.tenantId}`}</h1>
          </div>
          <div className="detail-metrics" aria-label="门店资源概览">
            <Metric label="机构 ID" value={store.tenantId} />
            <Metric label="工控机" value={store.summary.edgeCount} />
            <Metric label="NVR" value={store.summary.nvrCount} />
            <Metric label="摄像头" value={store.summary.cameraCount} />
            <Metric label="异常" value={store.summary.warningCount} />
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

      <nav className="tabs" aria-label="资源视角">
        <button className={tab === "spaces" ? "is-active" : ""} onClick={() => setTab("spaces")}>
          空间视角
        </button>
        <button className={tab === "devices" ? "is-active" : ""} onClick={() => setTab("devices")}>
          设备视角
        </button>
        <button className={tab === "issues" ? "is-active" : ""} onClick={() => setTab("issues")}>
          异常项
        </button>
      </nav>

      {tab === "spaces" ? <SpaceTree nodes={store.spaceTree} /> : null}
      {tab === "devices" ? <DeviceTree store={store} /> : null}
      {tab === "issues" ? <IssueList issues={issues} /> : null}
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

function SpaceTree({ nodes }: { nodes: ResourceSpaceNode[] }) {
  if (nodes.length === 0) {
    return <div className="resource-empty-panel">业务库暂无空间数据</div>;
  }

  return (
    <div className="resource-tree">
      {nodes.map((node) => (
        <SpaceNodeView key={node.id} node={node} />
      ))}
    </div>
  );
}

function SpaceNodeView({ node }: { node: ResourceSpaceNode }) {
  return (
    <article className="resource-node">
      <div className="resource-node-header">
        <div>
          <strong>{node.name || `空间 ${node.id}`}</strong>
          <span>Level {node.level}</span>
        </div>
        <span className={node.status === 1 ? "resource-status is-ok" : "resource-status is-muted"}>{node.statusText || `状态 ${node.status}`}</span>
      </div>
      <div className="resource-badges">
        <span>空间 ID {node.id}</span>
        {node.code ? <span>编码 {node.code}</span> : null}
        <span>绑定摄像头 {node.boundCameraCount}</span>
      </div>
      {node.boundCameras.length > 0 ? (
        <div className="resource-camera-list">
          {node.boundCameras.map((camera) => (
            <CameraBadge camera={camera} key={camera.id} />
          ))}
        </div>
      ) : null}
      {node.children.length > 0 ? (
        <div className="resource-node-children">
          {node.children.map((child) => (
            <SpaceNodeView key={child.id} node={child} />
          ))}
        </div>
      ) : null}
    </article>
  );
}

function DeviceTree({ store }: { store: ResourceStoreDetailType }) {
  return (
    <div className="resource-device-layout">
      <section className="resource-panel">
        <h2>工控机</h2>
        {store.deviceTree.edges.length === 0 ? <div className="resource-empty-panel">暂无工控机</div> : null}
        <div className="resource-device-grid">
          {store.deviceTree.edges.map((edge) => (
            <DeviceCard key={edge.id} name={edge.name || "工控机"} meta={edge.hardwareId || edge.sn || `设备 ${edge.id}`} status={edge.onlineText} />
          ))}
        </div>
      </section>
      <section className="resource-panel">
        <h2>NVR 与摄像头</h2>
        {store.deviceTree.nvrs.length === 0 ? <div className="resource-empty-panel">暂无 NVR</div> : null}
        <div className="resource-tree">
          {store.deviceTree.nvrs.map((nvr) => (
            <article className="resource-node" key={nvr.id}>
              <div className="resource-node-header">
                <div>
                  <strong>{nvr.name || `NVR ${nvr.id}`}</strong>
                  <span>{nvr.hardwareId || nvr.sn || `设备 ${nvr.id}`}</span>
                </div>
                <span className={nvr.onlineStatus === 1 ? "resource-status is-ok" : "resource-status is-warn"}>{nvr.onlineText}</span>
              </div>
              <div className="resource-camera-list">
                {nvr.cameras.length > 0 ? nvr.cameras.map((camera) => <CameraBadge camera={camera} key={camera.id} />) : <span>暂无摄像头</span>}
              </div>
            </article>
          ))}
        </div>
      </section>
    </div>
  );
}

function DeviceCard({ name, meta, status }: { name: string; meta: string; status: string }) {
  return (
    <article className="resource-device-card">
      <strong>{name}</strong>
      <span>{meta || "-"}</span>
      <em>{status || "未知"}</em>
    </article>
  );
}

function CameraBadge({ camera }: { camera: ResourceCamera }) {
  return (
    <span className="resource-camera-badge">
      {camera.channelNo ? `通道 ${camera.channelNo}` : `摄像头 ${camera.id}`}
      {camera.name ? ` · ${camera.name}` : ""}
    </span>
  );
}

function IssueList({ issues }: { issues: ResourceIssue[] }) {
  if (issues.length === 0) {
    return <div className="resource-empty-panel">暂未发现空间资源异常</div>;
  }

  return (
    <div className="resource-issues">
      {issues.map((issue) => (
        <article className={`resource-issue resource-issue-${issue.severity}`} key={`${issue.type}-${issue.entityType}-${issue.entityId}`}>
          <strong>{issue.message || issue.type}</strong>
          <span>
            {issue.entityType || "-"} #{issue.entityId || "-"}
          </span>
        </article>
      ))}
    </div>
  );
}
