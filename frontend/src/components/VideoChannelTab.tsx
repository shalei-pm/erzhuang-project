import { useEffect, useRef, useState, type CSSProperties } from "react";
import {
  ApiError,
  storeSpaceApi,
  type EzvizAccount,
  type AreaType,
  type NonBusinessSceneType,
  type StoreDetail,
  type VideoChannel,
  type VideoRecorder,
} from "../api";
import { areaTypeLabels } from "../domain/areas";
import { displayAccountRegion, selectableRegionAccounts } from "../domain/ezviz";
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

type ChannelTypeFilter = "all" | AreaType;

const channelTypeFilters: { value: ChannelTypeFilter; label: string }[] = [
  { value: "all", label: "全部" },
  { value: "consultation", label: "面诊室" },
  { value: "treatment", label: "治疗室" },
  { value: "beauty", label: "生美" },
];

type VideoChannelTabProps = {
  store: StoreDetail;
  accounts: EzvizAccount[];
  onStoreUpdated: (update: StoreDetail | ((store: StoreDetail) => StoreDetail)) => void;
  onRecorderUpdated: (recorder: VideoRecorder) => void;
  onToast: (message: string) => void;
};

export function VideoChannelTab({ store, accounts, onStoreUpdated, onRecorderUpdated, onToast }: VideoChannelTabProps) {
  const [workingRecorderId, setWorkingRecorderId] = useState<number | null>(null);
  const [recognizingChannelIds, setRecognizingChannelIds] = useState<Set<number>>(() => new Set());
  const [recorderProgress, setRecorderProgress] = useState<Record<number, { done: number; total: number }>>({});
  const [completedRecorderProgressId, setCompletedRecorderProgressId] = useState<number | null>(null);
  const [previewChannel, setPreviewChannel] = useState<VideoChannel | null>(null);
  const [confirmingChannelIds, setConfirmingChannelIds] = useState<Set<number>>(() => new Set());
  const [channelError, setChannelError] = useState("");
  const [addingRecorder, setAddingRecorder] = useState(false);
  const [deletingRecorderIds, setDeletingRecorderIds] = useState<Set<number>>(() => new Set());
  const [deletingChannelIds, setDeletingChannelIds] = useState<Set<number>>(() => new Set());
  const [newRecorderCode, setNewRecorderCode] = useState("");
  const [newRecorderAccountId, setNewRecorderAccountId] = useState<number | "">("");
  const [channelTypeFilter, setChannelTypeFilter] = useState<ChannelTypeFilter>("all");
  const [editingChannels, setEditingChannels] = useState<Record<number, Partial<VideoChannel>>>({});
  const completionTimerRef = useRef<number | null>(null);
  const regionAccounts = selectableRegionAccounts(accounts);

  useEffect(() => {
    return () => {
      if (completionTimerRef.current !== null) {
        window.clearTimeout(completionTimerRef.current);
      }
    };
  }, []);

  async function scanRecorder(recorder: VideoRecorder) {
    setWorkingRecorderId(recorder.id);
    setChannelError("");
    try {
      const nextRecorder = await storeSpaceApi.scanRecorder(store.id, recorder.id);
      onRecorderUpdated(nextRecorder);
      onToast(`已扫描 ${recorder.deviceCode}，发现 ${nextRecorder.effectiveChannelCount} 个有效通道。`);
    } catch (error) {
      const message = channelErrorMessage(error, "扫描失败，请稍后重试。");
      setChannelError(`录像机 ${recorder.deviceCode} 扫描失败：${message}`);
      onToast(message);
    } finally {
      setWorkingRecorderId(null);
    }
  }

  async function recognizeRecorder(recorder: VideoRecorder) {
    const targetChannels = recorder.channels.filter((channel) => channel.status !== "inactive" && !isConfirmedChannel(channel));
    if (targetChannels.length === 0) {
      onToast("暂无可识别通道，已确认通道需先点击编辑后再重新识别。");
      return;
    }
    setWorkingRecorderId(recorder.id);
    setCompletedRecorderProgressId(null);
    setChannelError("");
    setRecorderProgress((current) => ({ ...current, [recorder.id]: { done: 0, total: targetChannels.length } }));
    setRecognizingChannelIds((current) => {
      const next = new Set(current);
      targetChannels.forEach((channel) => next.add(channel.id));
      return next;
    });
    try {
      for (let index = 0; index < targetChannels.length; index++) {
        const channel = targetChannels[index];
        const updatedChannel = await storeSpaceApi.recognizeChannel(store.id, channel.id);
        onStoreUpdated((currentStore) => replaceChannelInStore(currentStore, updatedChannel));
        setRecognizingChannelIds((current) => {
          const next = new Set(current);
          next.delete(channel.id);
          return next;
        });
        setRecorderProgress((current) => ({ ...current, [recorder.id]: { done: index + 1, total: targetChannels.length } }));
      }
      setCompletedRecorderProgressId(recorder.id);
      if (completionTimerRef.current !== null) {
        window.clearTimeout(completionTimerRef.current);
      }
      completionTimerRef.current = window.setTimeout(() => {
        setCompletedRecorderProgressId(null);
        completionTimerRef.current = null;
      }, 900);
      onToast(`已完成 ${recorder.deviceCode} 的通道识别。`);
    } catch (error) {
      const message = channelErrorMessage(error, "截图识别能力还在接入中，请稍后再试。");
      setChannelError(`录像机 ${recorder.deviceCode} 识别失败：${message}`);
      onToast(message);
    } finally {
      setWorkingRecorderId(null);
      setRecognizingChannelIds((current) => {
        const next = new Set(current);
        targetChannels.forEach((channel) => next.delete(channel.id));
        return next;
      });
    }
  }

  async function deleteRecorder(recorder: VideoRecorder) {
    const ok = window.confirm(`删除后将清除录像机 ${recorder.deviceCode} 及其通道映射，且无法恢复。是否确认删除？`);
    if (!ok) return;
    setWorkingRecorderId(recorder.id);
    setDeletingRecorderIds((current) => addIdToSet(current, recorder.id));
    try {
      const updated = await storeSpaceApi.deleteRecorder(store.id, recorder.id);
      onStoreUpdated(updated);
      onToast(`已删除录像机 ${recorder.deviceCode}。`);
    } catch (error) {
      onToast(channelErrorMessage(error, "删除录像机失败，请稍后重试。"));
    } finally {
      setWorkingRecorderId(null);
      setDeletingRecorderIds((current) => removeIdFromSet(current, recorder.id));
    }
  }

  async function addRecorder() {
    const deviceCode = newRecorderCode.trim();
    const regionAccountId = newRecorderAccountId ? Number(newRecorderAccountId) : 0;
    if (!deviceCode) {
      onToast("录像机设备编码不能为空。");
      return;
    }
    if (!regionAccountId || !regionAccounts.some((account) => account.id === regionAccountId)) {
      onToast("请选择区域。");
      return;
    }
    setAddingRecorder(true);
    try {
      const updated = await storeSpaceApi.addRecorder(store.id, {
        ezvizAccountId: regionAccountId,
        deviceCode,
      });
      setNewRecorderCode("");
      setNewRecorderAccountId("");
      onStoreUpdated(updated);
      onToast(`已添加录像机 ${deviceCode.toUpperCase()}。`);
    } catch (error) {
      onToast(channelErrorMessage(error, "添加录像机失败，请稍后重试。"));
    } finally {
      setAddingRecorder(false);
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
    setConfirmingChannelIds((current) => addIdToSet(current, channel.id));
    setChannelError("");
    try {
      const updated = await storeSpaceApi.confirmChannel(store.id, channel.id, {
        ...patch,
        areaType,
        areaNumber,
        areaNote: areaType ? "" : String(patch.areaNote ?? patch.areaNumber ?? channel.areaNote ?? ""),
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
      const message = channelErrorMessage(error, "通道确认失败，请稍后重试。");
      setChannelError(`通道 ${channel.channelNo} 确认失败：${message}`);
      onToast(message);
    } finally {
      setConfirmingChannelIds((current) => removeIdFromSet(current, channel.id));
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

  async function deleteChannel(recorder: VideoRecorder, channel: VideoChannel) {
    const ok = window.confirm(`删除后将移除录像机 ${recorder.deviceCode} 的通道 ${channel.channelNo} 映射。再次扫描如仍有效，会作为未确认通道重新出现。是否确认删除？`);
    if (!ok) return;
    setChannelError("");
    setDeletingChannelIds((current) => addIdToSet(current, channel.id));
    try {
      const updated = await storeSpaceApi.deleteChannel(store.id, channel.id);
      setEditingChannels((current) => {
        const next = { ...current };
        delete next[channel.id];
        return next;
      });
      onStoreUpdated(updated);
      onToast(`已删除通道 ${channel.channelNo}。`);
    } catch (error) {
      const message = channelErrorMessage(error, "删除通道失败，请稍后重试。");
      setChannelError(`通道 ${channel.channelNo} 删除失败：${message}`);
      onToast(message);
    } finally {
      setDeletingChannelIds((current) => removeIdFromSet(current, channel.id));
    }
  }

  async function recognizeChannel(recorder: VideoRecorder, channel: VideoChannel) {
    setRecognizingChannelIds((current) => new Set(current).add(channel.id));
    setChannelError("");
    try {
      const updatedChannel = await storeSpaceApi.recognizeChannel(store.id, channel.id);
      onStoreUpdated((currentStore) => replaceChannelInStore(currentStore, updatedChannel));
      onToast(`已重新识别录像机 ${recorder.deviceCode} 的通道 ${channel.channelNo}。`);
    } catch (error) {
      const message = channelErrorMessage(error, "截图识别能力还在接入中，请稍后再试。");
      setChannelError(`通道 ${channel.channelNo} 识别失败：${message}`);
      onToast(message);
    } finally {
      setRecognizingChannelIds((current) => {
        const next = new Set(current);
        next.delete(channel.id);
        return next;
      });
    }
  }

  return (
    <section className="channel-shell">
      <section className="recorder-panel">
        <div className="section-title-row">
          <div>
            <strong>录像机列表</strong>
            <span>最多 3 台，删除后可在这里重新补充。</span>
          </div>
          <div className="add-recorder-form" aria-label="添加录像机">
            <select
              value={newRecorderAccountId}
              disabled={addingRecorder}
              onChange={(event) => setNewRecorderAccountId(event.target.value ? Number(event.target.value) : "")}
              aria-label="选择区域"
            >
              <option value="">{regionAccounts.length === 0 ? "暂无区域" : "选择区域"}</option>
              {regionAccounts.map((account) => (
                <option value={account.id} key={account.id}>
                  {displayAccountRegion(account)}
                </option>
              ))}
            </select>
            <input
              value={newRecorderCode}
              disabled={addingRecorder || store.recorders.length >= 3}
              onChange={(event) => setNewRecorderCode(event.target.value)}
              placeholder="录像机设备编码"
            />
            <button disabled={addingRecorder || store.recorders.length >= 3} onClick={() => void addRecorder()}>
              添加录像机
            </button>
          </div>
        </div>
        {channelError ? <div className="inline-error">{channelError}</div> : null}

        {store.recorders.length === 0 ? (
          <div className="manual-panel">
            <strong>暂无录像机</strong>
            <p>可在上方填写设备编码并添加，添加后再扫描通道。</p>
          </div>
        ) : (
          <div className="recorder-table-wrap">
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
                  const isDeleting = deletingRecorderIds.has(recorder.id);
                  const isRecognizingRecorder = workingRecorderId === recorder.id;
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
                        <div className="recorder-operation-area">
                          <div className="row-actions recorder-actions">
                            <button disabled={isRecognizingRecorder || isDeleting} onClick={() => void scanRecorder(recorder)}>
                              {hasScanned ? "再次扫描" : "扫描通道"}
                            </button>
                            {hasScanned ? (
                              <button disabled={isRecognizingRecorder || isDeleting} onClick={() => void recognizeRecorder(recorder)}>
                                识别区域
                              </button>
                            ) : null}
                            <button className="danger-link" disabled={isRecognizingRecorder || isDeleting} onClick={() => void deleteRecorder(recorder)}>
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
                          {isRecognizingRecorder ? (
                            <span className="recorder-thinking-label">{recognitionProgressLabel(recorderProgress[recorder.id])}</span>
                          ) : recorder.recognitionProgress ? (
                            <span className="recorder-muted-label">{recorder.recognitionProgress}</span>
                          ) : null}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            <RecorderTableProgress
              progress={activeRecorderProgress(workingRecorderId, completedRecorderProgressId, recorderProgress)}
              complete={workingRecorderId === null && completedRecorderProgressId !== null}
            />
          </div>
        )}
      </section>

      <section className="channel-filter-bar" aria-label="通道筛选">
        <div>
          <strong>通道列表</strong>
          <span>按业务区域类型筛选当前通道映射。</span>
        </div>
        <div className="segmented-control" role="radiogroup" aria-label="业务区域类型筛选">
          {channelTypeFilters.map((filter) => (
            <button
              key={filter.value}
              type="button"
              role="radio"
              aria-checked={channelTypeFilter === filter.value}
              className={channelTypeFilter === filter.value ? "is-active" : ""}
              onClick={() => setChannelTypeFilter(filter.value)}
            >
              {filter.label}
            </button>
          ))}
        </div>
      </section>

      {store.recorders.map((recorder) => {
        const visibleChannels = recorder.channels.filter((channel) => channelMatchesTypeFilter(channel, channelTypeFilter, editingChannels[channel.id]));
        return (
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
                  <th>编号/备注</th>
                  <th>确认状态</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {visibleChannels.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="channel-empty-cell">
                      当前筛选下暂无通道
                    </td>
                  </tr>
                ) : (
                  visibleChannels.map((channel) => {
                const draft = editingChannels[channel.id] ?? {};
                const isEditable =
                  channel.status === "pending_confirmation" ||
                  channel.status === "pending_recognition" ||
                  channel.status === "recognition_failed" ||
                  Boolean(draft.status);
                const recognitionMessage = channelRecognitionMessage(channel);
                const isRecognizing = recognizingChannelIds.has(channel.id);
                const isConfirming = confirmingChannelIds.has(channel.id);
                const isDeleting = deletingChannelIds.has(channel.id);
                const isConfirmed = isConfirmedChannel(channel);
                const selectedAreaType = draft.areaType !== undefined ? draft.areaType : channel.areaType;
                const selectedAreaNumber = draft.areaNumber ?? (selectedAreaType ? channel.areaNumber : channel.areaNote || channel.areaNumber);
                return (
                  <tr key={channel.id}>
                    <td>{recorder.deviceCode}</td>
                    <td>{channel.channelNo}</td>
                    <td>
                      <button
                        className={`channel-thumb ${channel.thumbnailUrl ? "has-image" : ""}`}
                        type="button"
                        disabled={!channel.fullImageUrl && !channel.thumbnailUrl}
                        aria-label={channel.thumbnailUrl ? `查看通道 ${channel.channelNo} 截图` : "暂无截图"}
                        onClick={() => setPreviewChannel(channel)}
                      >
                        {channel.thumbnailUrl ? <img src={channel.thumbnailUrl} alt={`通道 ${channel.channelNo} 截图`} /> : <span />}
                      </button>
                      {recognitionMessage ? <div className="channel-row-note">{recognitionMessage}</div> : null}
                    </td>
                    <td>
                      {isEditable ? (
                        <select
                          value={selectedAreaType}
                          onChange={(event) =>
                            updateChannelDraft(channel.id, {
                              areaType: event.target.value as AreaType | "",
                              areaNote: event.target.value ? "" : channel.areaNote || draft.areaNumber || "",
                              sceneType: event.target.value ? (event.target.value as AreaType) : "unknown",
                            })
                          }
                        >
                          <option value="">其他区域</option>
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
                          value={selectedAreaNumber}
                          inputMode={selectedAreaType ? "numeric" : "text"}
                          onChange={(event) => {
                            if (selectedAreaType) {
                              updateChannelDraft(channel.id, { areaNumber: event.target.value, areaNote: "" });
                              return;
                            }
                            updateChannelDraft(channel.id, { areaNumber: event.target.value, areaNote: event.target.value });
                          }}
                          placeholder={selectedAreaType ? "必填" : "-"}
                        />
                      ) : (
                        channel.areaType ? channel.areaNumber || "-" : channel.areaNote || channel.areaNumber || "-"
                      )}
                    </td>
                    <td>
                      <span className={`status-pill channel-${channel.status}`}>{channelStatusLabel(channel.status)}</span>
                    </td>
                    <td>
                      <div className="row-actions">
                        {isEditable ? (
                          <button disabled={isConfirming || isDeleting} onClick={() => void confirmChannel(channel)}>
                            {isConfirming ? (
                              <>
                                <span className="button-spinner" aria-hidden="true" />
                                确认中
                              </>
                            ) : (
                              "确认"
                            )}
                          </button>
                        ) : (
                          <button disabled={isDeleting} onClick={() => updateChannelDraft(channel.id, { status: "pending_confirmation" })}>
                            编辑
                          </button>
                        )}
                        <button
                          disabled={isRecognizing || isDeleting || isConfirmed || workingRecorderId === recorder.id}
                          onClick={() => void recognizeChannel(recorder, channel)}
                        >
                          {isRecognizing ? (
                            <>
                              <span className="button-spinner" aria-hidden="true" />
                              识别中
                            </>
                          ) : (
                            "重新识别"
                          )}
                        </button>
                        <button
                          className="danger-link"
                          disabled={isRecognizing || isDeleting || isConfirming || workingRecorderId === recorder.id}
                          onClick={() => void deleteChannel(recorder, channel)}
                        >
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
      })}
      {previewChannel ? (
        <div className="snapshot-preview-backdrop" role="dialog" aria-modal="true" onClick={() => setPreviewChannel(null)}>
          <div className="snapshot-preview" onClick={(event) => event.stopPropagation()}>
            <div className="snapshot-preview-head">
              <div>
                <strong>通道 {previewChannel.channelNo} 最近截图</strong>
                <span>{previewChannel.fullImageExpiresAt ? `大图有效期至 ${formatDateTime(previewChannel.fullImageExpiresAt)}` : "来自萤石云抓图"}</span>
              </div>
              <button type="button" onClick={() => setPreviewChannel(null)} aria-label="关闭截图预览">
                ×
              </button>
            </div>
            <img src={previewChannel.fullImageUrl || previewChannel.thumbnailUrl} alt={`通道 ${previewChannel.channelNo} 最近截图`} />
          </div>
        </div>
      ) : null}
    </section>
  );
}

function channelMatchesTypeFilter(channel: VideoChannel, filter: ChannelTypeFilter, draft?: Partial<VideoChannel>) {
  if (filter === "all") return true;
  const areaType = draft?.areaType !== undefined ? draft.areaType : channel.areaType;
  return areaType === filter;
}

function addIdToSet(current: Set<number>, id: number) {
  return new Set(current).add(id);
}

function removeIdFromSet(current: Set<number>, id: number) {
  const next = new Set(current);
  next.delete(id);
  return next;
}

function channelRecognitionMessage(channel: VideoChannel) {
  const result = channel.recognitionResult;
  if (!result) return "";
  if (typeof result === "string") {
    try {
      return channelRecognitionMessageFromObject(JSON.parse(result));
    } catch {
      return result;
    }
  }
  if (typeof result === "object" && result) {
    return channelRecognitionMessageFromObject(result);
  }
  return "";
}

function channelRecognitionMessageFromObject(value: unknown) {
  if (!value || typeof value !== "object") return "";
  const result = value as {
    status?: string;
    message?: string;
    area_type?: AreaType | "";
    area_number?: string;
    confidence?: string;
    recognition_ms?: number;
    total_ms?: number;
    capture_ms?: number;
  };
  const timing = recognitionTimingLabel(result);
  if (result.status === "capture_failed" || result.status === "recognition_failed") {
    return ["失败", timing].filter(Boolean).join(" · ");
  }
  if (result.status === "recognized") {
    const confidence = result.confidence === "low" ? "低置信" : "";
    return [confidence, timing].filter(Boolean).join(" · ");
  }
  if (result.status === "captured") {
    return ["抓图", timing].filter(Boolean).join(" · ");
  }
  return result.message || timing;
}

function recognitionTimingLabel(result: { capture_ms?: number; recognition_ms?: number; total_ms?: number }) {
  const parts: string[] = [];
  if (typeof result.recognition_ms === "number" && result.recognition_ms > 0) {
    parts.push(`识别 ${formatDuration(result.recognition_ms)}`);
  }
  if (typeof result.total_ms === "number" && result.total_ms > 0) {
    parts.push(`总 ${formatDuration(result.total_ms)}`);
  }
  return parts.join(" / ");
}

function formatDuration(ms: number) {
  if (ms >= 1000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.max(1, Math.round(ms))}ms`;
}

function replaceChannelInStore(store: StoreDetail, updatedChannel: VideoChannel): StoreDetail {
  const recorders = store.recorders.map((recorder) => {
    if (!recorder.channels.some((channel) => channel.id === updatedChannel.id)) return recorder;
    const channels = recorder.channels.map((channel) => (channel.id === updatedChannel.id ? { ...updatedChannel, recorderCode: recorder.deviceCode } : channel));
    return {
      ...recorder,
      channels,
      effectiveChannelCount: channels.filter((channel) => channel.status !== "inactive").length,
    };
  });
  return {
    ...store,
    recorders,
    channelCount: recorders.reduce((total, recorder) => total + recorder.channels.filter((channel) => channel.status !== "inactive").length, 0),
    updatedAt: new Date().toISOString(),
  };
}

function isConfirmedChannel(channel: VideoChannel) {
  return channel.status === "confirmed_business" || channel.status === "confirmed_non_business";
}

function recognitionProgressLabel(progress?: { done: number; total: number }) {
  if (!progress || progress.total <= 0) return "正在准备识别";
  return `识别进度 ${progress.done}/${progress.total} · ${recognitionProgressPercent(progress)}%`;
}

function recognitionProgressPercent(progress?: { done: number; total: number }) {
  if (!progress || progress.total <= 0) return 0;
  return Math.min(100, Math.round((progress.done / progress.total) * 100));
}

function activeRecorderProgress(
  workingRecorderId: number | null,
  completedRecorderProgressId: number | null,
  recorderProgress: Record<number, { done: number; total: number }>,
) {
  if (workingRecorderId !== null) return recorderProgress[workingRecorderId];
  if (completedRecorderProgressId !== null) return recorderProgress[completedRecorderProgressId];
  return undefined;
}

function RecorderTableProgress({ progress, complete }: { progress?: { done: number; total: number }; complete: boolean }) {
  if (!progress) return null;
  return (
    <div className={`recorder-table-progress${complete ? " is-complete" : ""}`} aria-hidden="true">
      <i style={{ "--progress": `${recognitionProgressPercent(progress)}%` } as CSSProperties} />
    </div>
  );
}

function nonBusinessLabel(sceneType: VideoChannel["sceneType"]) {
  if (sceneType === "treatment" || sceneType === "consultation" || sceneType === "beauty") {
    return areaTypeLabels[sceneType];
  }
  return sceneLabels[sceneType] ?? "其他区域";
}

function channelStatusLabel(status: VideoChannel["status"]) {
  const labels: Record<VideoChannel["status"], string> = {
    pending_recognition: "待识别",
    pending_confirmation: "待确认",
    confirmed_business: "已确认-业务",
    confirmed_non_business: "已确认-其他",
    recognition_failed: "识别失败",
    inactive: "已失效",
  };
  return labels[status];
}

function channelErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError && error.status === 404) {
    return "通道映射接口未就绪或资源不存在，请确认后端服务状态。";
  }
  if (error instanceof ApiError && error.status === 501) {
    return "截图识别能力还在接入中，当前可以先完成真实通道扫描。";
  }
  if (error instanceof ApiError && Object.keys(error.fields).length > 0) {
    return Object.values(error.fields).join("；");
  }
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return fallback;
}
