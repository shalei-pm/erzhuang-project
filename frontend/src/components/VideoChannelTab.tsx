import { useState } from "react";
import {
  ApiError,
  storeSpaceApi,
  type AreaType,
  type NonBusinessSceneType,
  type StoreDetail,
  type VideoChannel,
  type VideoRecorder,
} from "../api";
import { areaTypeLabels } from "../domain/areas";
import { formatDateTime } from "../domain/format";

const sceneLabels: Record<NonBusinessSceneType, string> = {
  front_desk: "前台",
  corridor: "走廊",
  passage: "通道",
  waiting_area: "候诊区",
  hall: "大厅",
  entrance: "门口",
  storage: "库房",
  pharmacy: "药房",
  machine_room: "机房",
  unknown: "未知",
};

type VideoChannelTabProps = {
  store: StoreDetail;
  onStoreUpdated: (store: StoreDetail) => void;
  onRecorderUpdated: (recorder: VideoRecorder) => void;
  onToast: (message: string) => void;
};

export function VideoChannelTab({ store, onStoreUpdated, onRecorderUpdated, onToast }: VideoChannelTabProps) {
  const [workingRecorderId, setWorkingRecorderId] = useState<number | null>(null);
  const [editingChannels, setEditingChannels] = useState<Record<number, Partial<VideoChannel>>>({});

  async function scanRecorder(recorder: VideoRecorder) {
    setWorkingRecorderId(recorder.id);
    try {
      const nextRecorder = await storeSpaceApi.scanRecorder(store.id, recorder.id);
      onRecorderUpdated(nextRecorder);
      onToast(`已扫描 ${recorder.deviceCode}，发现 ${nextRecorder.effectiveChannelCount} 个有效通道。`);
    } catch (error) {
      onToast(channelErrorMessage(error, "扫描失败，请稍后重试。"));
    } finally {
      setWorkingRecorderId(null);
    }
  }

  async function recognizeRecorder(recorder: VideoRecorder) {
    setWorkingRecorderId(recorder.id);
    try {
      const nextRecorder = await storeSpaceApi.recognizeRecorder(store.id, recorder.id);
      onRecorderUpdated(nextRecorder);
      onToast(`已完成 ${recorder.deviceCode} 的通道识别。`);
    } catch (error) {
      onToast(channelErrorMessage(error, "通道识别失败，请稍后重试。"));
    } finally {
      setWorkingRecorderId(null);
    }
  }

  async function confirmChannel(channel: VideoChannel) {
    const patch = editingChannels[channel.id] ?? {};
    const areaType = patch.areaType ?? channel.areaType;
    const areaNumber = patch.areaNumber ?? channel.areaNumber;
    const sceneType = patch.sceneType ?? channel.sceneType;
    if (areaType && !String(areaNumber).trim()) {
      onToast("确认为业务区域时，编号必填。");
      return;
    }
    try {
      const updated = await storeSpaceApi.confirmChannel(store.id, channel.id, {
        ...patch,
        areaType,
        areaNumber,
        sceneType,
      });
      setEditingChannels((current) => {
        const next = { ...current };
        delete next[channel.id];
        return next;
      });
      onStoreUpdated(updated);
      onToast("通道确认已保存。");
    } catch (error) {
      onToast(channelErrorMessage(error, "通道确认失败，请稍后重试。"));
    }
  }

  function updateChannelDraft(channelId: number, patch: Partial<VideoChannel>) {
    setEditingChannels((current) => ({
      ...current,
      [channelId]: {
        ...current[channelId],
        ...patch,
      },
    }));
  }

  return (
    <section className="channel-shell">
      <section className="recorder-panel">
        <div className="section-title-row">
          <div>
            <strong>录像机列表</strong>
            <span>{store.recorders.length} / 3 台</span>
          </div>
        </div>

        {store.recorders.length === 0 ? (
          <div className="manual-panel">
            <strong>暂无录像机</strong>
            <p>可以先在添加门店时录入设备编码，后续也会支持详情页补充。</p>
          </div>
        ) : (
          <table className="recorder-table">
            <thead>
              <tr>
                <th>录像机名称</th>
                <th>状态</th>
                <th>有效通道数</th>
                <th>上次扫描时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {store.recorders.map((recorder) => {
                const hasScanned = recorder.effectiveChannelCount > 0 || recorder.channels.length > 0 || Boolean(recorder.lastScannedAt);
                return (
                  <tr key={recorder.id}>
                    <td>
                      <strong>{recorder.deviceCode}</strong>
                    </td>
                    <td>
                      <span className={`status-pill recorder-${recorder.status}`}>{recorder.status === "online" ? "在线" : "离线"}</span>
                    </td>
                    <td>{recorder.effectiveChannelCount}</td>
                    <td>{formatDateTime(recorder.lastScannedAt)}</td>
                    <td>
                      <div className="row-actions recorder-actions">
                        <button disabled={workingRecorderId === recorder.id} onClick={() => void scanRecorder(recorder)}>
                          {hasScanned ? "再次扫描" : "扫描通道"}
                        </button>
                        {hasScanned ? (
                          <button disabled={workingRecorderId === recorder.id} onClick={() => void recognizeRecorder(recorder)}>
                            识别区域
                          </button>
                        ) : null}
                        <button className="danger-link" onClick={() => onToast("删除录像机需要后端级联删除接口，本阶段仅展示入口。")}>
                          删除
                        </button>
                      </div>
                      {workingRecorderId === recorder.id ? (
                        <div className="recognition-progress">
                          正在处理：录像机 {recorder.deviceCode}
                        </div>
                      ) : recorder.recognitionProgress ? (
                        <div className="recognition-progress">{recorder.recognitionProgress}</div>
                      ) : null}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </section>

      {store.recorders.map((recorder) => (
        <section className="channel-table-section" key={recorder.id}>
          <div className="section-title-row">
            <div>
              <strong>{recorder.deviceCode} 有效通道</strong>
              <span>请将白底黑字编号纸放在画面明显位置，例如：治疗室 1 / 面诊室 2 / 生美 3。</span>
            </div>
          </div>
          <table className="channel-table">
            <thead>
              <tr>
                <th>录像机</th>
                <th>通道号</th>
                <th>最近截图</th>
                <th>业务区域类型</th>
                <th>编号</th>
                <th>确认状态</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {recorder.channels.map((channel) => {
                const draft = editingChannels[channel.id] ?? {};
                const isEditable =
                  channel.status === "pending_confirmation" ||
                  channel.status === "pending_recognition" ||
                  channel.status === "recognition_failed" ||
                  Boolean(draft.status);
                return (
                  <tr key={channel.id}>
                    <td>{recorder.deviceCode}</td>
                    <td>{channel.channelNo}</td>
                    <td>
                      <div className="channel-thumb" aria-label="截图缩略图占位">
                        {channel.thumbnailUrl ? <img src={channel.thumbnailUrl} alt={`通道 ${channel.channelNo} 截图`} /> : <span />}
                      </div>
                    </td>
                    <td>
                      {isEditable ? (
                        <select
                          value={draft.areaType ?? channel.areaType}
                          onChange={(event) =>
                            updateChannelDraft(channel.id, {
                              areaType: event.target.value as AreaType | "",
                              sceneType: event.target.value ? (event.target.value as AreaType) : "unknown",
                            })
                          }
                        >
                          <option value="">非业务画面</option>
                          <option value="treatment">治疗室</option>
                          <option value="consultation">面诊室</option>
                          <option value="beauty">生美</option>
                        </select>
                      ) : (
                        channel.areaType ? areaTypeLabels[channel.areaType] : nonBusinessLabel(channel.sceneType)
                      )}
                    </td>
                    <td>
                      {isEditable ? (
                        <input
                          value={draft.areaNumber ?? channel.areaNumber}
                          inputMode="numeric"
                          onChange={(event) => updateChannelDraft(channel.id, { areaNumber: event.target.value })}
                          placeholder={draft.areaType || channel.areaType ? "必填" : "-"}
                        />
                      ) : (
                        channel.areaNumber || "-"
                      )}
                    </td>
                    <td>
                      <span className={`status-pill channel-${channel.status}`}>{channelStatusLabel(channel.status)}</span>
                    </td>
                    <td>
                      <div className="row-actions">
                        {isEditable ? (
                          <button onClick={() => void confirmChannel(channel)}>确认</button>
                        ) : (
                          <button onClick={() => updateChannelDraft(channel.id, { status: "pending_confirmation" })}>编辑</button>
                        )}
                        <button onClick={() => updateChannelDraft(channel.id, { status: "pending_confirmation" })}>重新识别</button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </section>
      ))}
    </section>
  );
}

function nonBusinessLabel(sceneType: VideoChannel["sceneType"]) {
  if (sceneType === "treatment" || sceneType === "consultation" || sceneType === "beauty") {
    return areaTypeLabels[sceneType];
  }
  return sceneLabels[sceneType] ?? "未知";
}

function channelStatusLabel(status: VideoChannel["status"]) {
  const labels: Record<VideoChannel["status"], string> = {
    pending_recognition: "待识别",
    pending_confirmation: "待确认",
    confirmed_business: "已确认-业务",
    confirmed_non_business: "已确认-非业务",
    recognition_failed: "识别失败",
    inactive: "已失效",
  };
  return labels[status];
}

function channelErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError && error.status === 404) {
    return "通道映射接口未就绪或资源不存在，请确认后端服务状态。";
  }
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return fallback;
}
