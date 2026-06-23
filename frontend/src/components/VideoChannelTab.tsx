import { useEffect, useRef, useState, type CSSProperties } from "react";
import {
  ApiError,
  storeSpaceApi,
  type EzvizAccount,
  type AreaType,
  type StoreDetail,
  type VideoChannel,
  type VideoRecorder,
} from "../api";
import { areaTypeLabels } from "../domain/areas";
import { channelSceneLabel } from "../domain/channel-labels";
import { displayAccountRegion, selectableRegionAccounts } from "../domain/ezviz";
import { fallbackProbeChannelNumbers, fallbackProbeMaxChannelNo, shouldStopFallbackProbe } from "../domain/fallback-probe";
import { formatDateTime } from "../domain/format";

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
  const [fallbackProbeProgress, setFallbackProbeProgress] = useState<Record<number, { checked: number; active: number }>>({});
  const [completedRecorderProgressId, setCompletedRecorderProgressId] = useState<number | null>(null);
  const [previewChannel, setPreviewChannel] = useState<VideoChannel | null>(null);
  const [confirmingChannelIds, setConfirmingChannelIds] = useState<Set<number>>(() => new Set());
  const [unlockingChannelIds, setUnlockingChannelIds] = useState<Set<number>>(() => new Set());
  const [channelError, setChannelError] = useState("");
  const [addingRecorder, setAddingRecorder] = useState(false);
  const [deletingRecorderIds, setDeletingRecorderIds] = useState<Set<number>>(() => new Set());
  const [deletingChannelIds, setDeletingChannelIds] = useState<Set<number>>(() => new Set());
  const [newRecorderCode, setNewRecorderCode] = useState("");
  const [newRecorderAccountId, setNewRecorderAccountId] = useState<number | "">("");
  const [channelTypeFilter, setChannelTypeFilter] = useState<ChannelTypeFilter>("all");
  const [exportingChannels, setExportingChannels] = useState(false);
  const [expiredSnapshotIds, setExpiredSnapshotIds] = useState<Set<number>>(() => new Set());
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
      if (shouldUseFallbackProbe(message)) {
        await runFallbackProbeRecognition(recorder);
        return;
      }
      setChannelError(`录像机 ${recorder.deviceCode} 扫描失败：${message}`);
      onToast(message);
    } finally {
      setWorkingRecorderId(null);
    }
  }

  async function runFallbackProbeRecognition(recorder: VideoRecorder) {
    let activeCount = 0;
    let consecutiveFailures = 0;
    const channelNumbers = fallbackProbeChannelNumbers();
    setChannelError("");
    setFallbackProbeProgress((current) => ({ ...current, [recorder.id]: { checked: 0, active: 0 } }));
    setRecorderProgress((current) => ({ ...current, [recorder.id]: { done: 0, total: fallbackProbeMaxChannelNo } }));
    onToast(`无法直接获取 ${recorder.deviceCode} 的通道列表，正在通过抓图识别有效通道。`);
    for (const channelNo of channelNumbers) {
      try {
        const result = await storeSpaceApi.probeRecognizeChannel(store.id, recorder, channelNo);
        if (result.active && result.channel) {
          consecutiveFailures = 0;
          activeCount += 1;
          onStoreUpdated((currentStore) => upsertChannelInStore(currentStore, recorder.id, result.channel as VideoChannel));
          setExpiredSnapshotIds((current) => removeIdFromSet(current, result.channel!.id));
        } else {
          consecutiveFailures += 1;
        }
      } catch (error) {
        consecutiveFailures += 1;
      }
      setFallbackProbeProgress((current) => ({ ...current, [recorder.id]: { checked: channelNo, active: activeCount } }));
      setRecorderProgress((current) => ({ ...current, [recorder.id]: { done: channelNo, total: fallbackProbeMaxChannelNo } }));
      if (shouldStopFallbackProbe(channelNo, consecutiveFailures)) {
        break;
      }
    }
    setCompletedRecorderProgressId(recorder.id);
    if (completionTimerRef.current !== null) {
      window.clearTimeout(completionTimerRef.current);
    }
    completionTimerRef.current = window.setTimeout(() => {
      setCompletedRecorderProgressId(null);
      setFallbackProbeProgress((current) => {
        const next = { ...current };
        delete next[recorder.id];
        return next;
      });
      completionTimerRef.current = null;
    }, 900);
    onToast(`已完成 ${recorder.deviceCode} 的抓图识别，发现 ${activeCount} 个有效通道。`);
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
        setExpiredSnapshotIds((current) => removeIdFromSet(current, channel.id));
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
    const previousStore = store;
    setWorkingRecorderId(recorder.id);
    setDeletingRecorderIds((current) => addIdToSet(current, recorder.id));
    onStoreUpdated((currentStore) => removeRecorderFromStore(currentStore, recorder.id));
    try {
      const updated = await storeSpaceApi.deleteRecorder(store.id, recorder.id);
      onStoreUpdated(updated);
      onToast(`已删除录像机 ${recorder.deviceCode}。`);
    } catch (error) {
      onStoreUpdated(previousStore);
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
    const previousChannel = channel;
    const optimisticChannel = confirmedChannelDraft(channel, areaType, areaNumber, patch.areaNote, sceneType);
    setConfirmingChannelIds((current) => addIdToSet(current, channel.id));
    setChannelError("");
    setEditingChannels((current) => {
      const next = { ...current };
      delete next[channel.id];
      return next;
    });
    onStoreUpdated((currentStore) => replaceChannelInStore(currentStore, optimisticChannel));
    try {
      const updated = await storeSpaceApi.confirmChannel(store.id, channel.id, {
        ...patch,
        areaType,
        areaNumber,
        areaNote: areaType ? "" : String(patch.areaNote ?? patch.areaNumber ?? channel.areaNote ?? ""),
        sceneType,
      });
      onStoreUpdated(updated);
      onToast("通道确认已保存。");
    } catch (error) {
      onStoreUpdated((currentStore) => replaceChannelInStore(currentStore, previousChannel));
      setEditingChannels((current) => ({ ...current, [channel.id]: patch }));
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

  async function unlockChannelForEdit(channel: VideoChannel) {
    const previousChannel = channel;
    const optimisticChannel = { ...channel, status: "pending_confirmation" as const, confirmedAt: undefined };
    setUnlockingChannelIds((current) => addIdToSet(current, channel.id));
    setChannelError("");
    setEditingChannels((current) => ({
      ...current,
      [channel.id]: channelDraftFromChannel(channel),
    }));
    onStoreUpdated((currentStore) => replaceChannelInStore(currentStore, optimisticChannel));
    try {
      const updatedChannel = await storeSpaceApi.unlockChannelForEdit(store.id, channel.id);
      onStoreUpdated((currentStore) => replaceChannelInStore(currentStore, updatedChannel));
    } catch (error) {
      onStoreUpdated((currentStore) => replaceChannelInStore(currentStore, previousChannel));
      setEditingChannels((current) => {
        const next = { ...current };
        delete next[channel.id];
        return next;
      });
      const message = channelErrorMessage(error, "通道编辑状态切换失败，请稍后重试。");
      setChannelError(`通道 ${channel.channelNo} 编辑失败：${message}`);
      onToast(message);
    } finally {
      setUnlockingChannelIds((current) => removeIdFromSet(current, channel.id));
    }
  }

  async function deleteChannel(recorder: VideoRecorder, channel: VideoChannel) {
    const ok = window.confirm(`删除后将移除录像机 ${recorder.deviceCode} 的通道 ${channel.channelNo} 映射。再次扫描如仍有效，会作为未确认通道重新出现。是否确认删除？`);
    if (!ok) return;
    const previousStore = store;
    setChannelError("");
    setDeletingChannelIds((current) => addIdToSet(current, channel.id));
    setEditingChannels((current) => {
      const next = { ...current };
      delete next[channel.id];
      return next;
    });
    onStoreUpdated((currentStore) => removeChannelFromStore(currentStore, channel.id));
    try {
      const updated = await storeSpaceApi.deleteChannel(store.id, channel.id);
      onStoreUpdated(updated);
      onToast(`已删除通道 ${channel.channelNo}。`);
    } catch (error) {
      onStoreUpdated(previousStore);
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
      setExpiredSnapshotIds((current) => removeIdFromSet(current, channel.id));
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

  async function refreshChannelSnapshot(recorder: VideoRecorder, channel: VideoChannel) {
    setRecognizingChannelIds((current) => new Set(current).add(channel.id));
    setChannelError("");
    try {
      const updatedChannel = await storeSpaceApi.refreshChannelSnapshot(store.id, channel.id);
      onStoreUpdated((currentStore) => replaceChannelInStore(currentStore, updatedChannel));
      setExpiredSnapshotIds((current) => removeIdFromSet(current, channel.id));
      onToast(`已刷新录像机 ${recorder.deviceCode} 的通道 ${channel.channelNo} 截图。`);
    } catch (error) {
      const message = channelErrorMessage(error, "刷新截图失败，请稍后重试。");
      setChannelError(`通道 ${channel.channelNo} 刷新截图失败：${message}`);
      onToast(message);
    } finally {
      setRecognizingChannelIds((current) => {
        const next = new Set(current);
        next.delete(channel.id);
        return next;
      });
    }
  }

  async function exportChannelMappings() {
    setExportingChannels(true);
    setChannelError("");
    try {
      await storeSpaceApi.exportChannelMappings(store.id);
      onToast("通道映射表已开始下载。");
    } catch (error) {
      const message = channelErrorMessage(error, "导出失败，请稍后重试。");
      setChannelError(message);
      onToast(message);
    } finally {
      setExportingChannels(false);
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
                            <span className="recorder-thinking-label">
                              {fallbackProbeProgress[recorder.id]
                                ? fallbackProbeProgressLabel(fallbackProbeProgress[recorder.id])
                                : recognitionProgressLabel(recorderProgress[recorder.id])}
                            </span>
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
        <div className="channel-filter-actions">
          <button className="secondary-action-button" type="button" disabled={exportingChannels} onClick={() => void exportChannelMappings()}>
            {exportingChannels ? (
              <>
                <span className="button-spinner" aria-hidden="true" />
                导出中
              </>
            ) : (
              "导出 Excel"
            )}
          </button>
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
                const isUnlocking = unlockingChannelIds.has(channel.id);
                const isDeleting = deletingChannelIds.has(channel.id);
                const isConfirmed = isConfirmedChannel(channel);
                const isSnapshotExpired = hasExpiredSnapshot(channel);
                const canPreviewSnapshot = Boolean(channel.thumbnailUrl && !expiredSnapshotIds.has(channel.id) && !isSnapshotExpired);
                const selectedAreaType = draft.areaType !== undefined ? draft.areaType : channel.areaType;
                const selectedAreaNumber = draft.areaNumber ?? (selectedAreaType ? channel.areaNumber : channel.areaNote || channel.areaNumber);
                return (
                  <tr key={channel.id}>
                    <td>{recorder.deviceCode}</td>
                    <td>{channel.channelNo}</td>
                    <td>
                      <button
                        className={`channel-thumb ${canPreviewSnapshot ? "has-image" : ""}`}
                        type="button"
                        disabled={!canPreviewSnapshot}
                        aria-label={canPreviewSnapshot ? `查看通道 ${channel.channelNo} 截图` : "暂无截图"}
                        onClick={() => setPreviewChannel(channel)}
                      >
                        {canPreviewSnapshot ? (
                          <img
                            src={channel.thumbnailUrl}
                            alt={`通道 ${channel.channelNo} 截图`}
                            onError={() => {
                              setExpiredSnapshotIds((current) => new Set(current).add(channel.id));
                            }}
                          />
                        ) : expiredSnapshotIds.has(channel.id) || isSnapshotExpired ? (
                          <span className="channel-thumb-expired">{isSnapshotExpired ? "已过期" : "加载失败"}</span>
                        ) : (
                          <span />
                        )}
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
                          <button disabled={isDeleting || isUnlocking} onClick={() => void unlockChannelForEdit(channel)}>
                            {isUnlocking ? (
                              <>
                                <span className="button-spinner" aria-hidden="true" />
                                解锁中
                              </>
                            ) : (
                              "编辑"
                            )}
                          </button>
                        )}
                        <button
                          disabled={isRecognizing || isDeleting || workingRecorderId === recorder.id}
                          onClick={() => void (isConfirmed ? refreshChannelSnapshot(recorder, channel) : recognizeChannel(recorder, channel))}
                        >
                          {isRecognizing ? (
                            <>
                              <span className="button-spinner" aria-hidden="true" />
                              {isConfirmed ? "刷新中" : "识别中"}
                            </>
                          ) : (
                            isConfirmed ? "刷新截图" : "重新识别"
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
                <span>{previewChannel.fullImageExpiresAt ? `原始抓图有效期至 ${formatDateTime(previewChannel.fullImageExpiresAt)}` : "已保存到系统截图库"}</span>
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

function hasExpiredSnapshot(channel: VideoChannel) {
  if (!channel.fullImageExpiresAt) {
    return false;
  }
  return new Date(channel.fullImageExpiresAt).getTime() <= Date.now();
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

function upsertChannelInStore(store: StoreDetail, recorderId: number, updatedChannel: VideoChannel): StoreDetail {
  const recorders = store.recorders.map((recorder) => {
    if (recorder.id !== recorderId) return recorder;
    const channel = { ...updatedChannel, recorderId, recorderCode: recorder.deviceCode };
    const existing = recorder.channels.some((item) => item.id === channel.id || item.channelNo === channel.channelNo);
    const channels = (existing
      ? recorder.channels.map((item) => (item.id === channel.id || item.channelNo === channel.channelNo ? channel : item))
      : [...recorder.channels, channel]
    ).sort((a, b) => a.channelNo - b.channelNo);
    return {
      ...recorder,
      status: "online" as const,
      channels,
      effectiveChannelCount: channels.filter((item) => item.status !== "inactive").length,
      lastScannedAt: new Date().toISOString(),
    };
  });
  return recalculateStoreVideoMetrics({
    ...store,
    recorders,
    updatedAt: new Date().toISOString(),
  });
}

function removeRecorderFromStore(store: StoreDetail, recorderId: number): StoreDetail {
  const recorders = store.recorders.filter((recorder) => recorder.id !== recorderId);
  return recalculateStoreVideoMetrics({
    ...store,
    recorders,
  });
}

function removeChannelFromStore(store: StoreDetail, channelId: number): StoreDetail {
  const recorders = store.recorders.map((recorder) => {
    const channels = recorder.channels.filter((channel) => channel.id !== channelId);
    return {
      ...recorder,
      channels,
      effectiveChannelCount: channels.filter((channel) => channel.status !== "inactive").length,
      status: channels.some((channel) => channel.status !== "inactive") ? recorder.status : "offline",
    };
  });
  return recalculateStoreVideoMetrics({
    ...store,
    recorders,
  });
}

function recalculateStoreVideoMetrics(store: StoreDetail): StoreDetail {
  return {
    ...store,
    recorderCount: store.recorders.length,
    channelCount: store.recorders.reduce((total, recorder) => total + recorder.channels.filter((channel) => channel.status !== "inactive").length, 0),
    updatedAt: new Date().toISOString(),
  };
}

function confirmedChannelDraft(
  channel: VideoChannel,
  areaType: AreaType | "",
  areaNumber: string,
  areaNote: unknown,
  sceneType: VideoChannel["sceneType"],
): VideoChannel {
  const isBusiness = Boolean(areaType);
  return {
    ...channel,
    areaType,
    areaNumber: isBusiness ? String(areaNumber).trim() : "",
    areaNote: isBusiness ? "" : String(areaNote ?? areaNumber ?? channel.areaNote ?? ""),
    sceneType: isBusiness ? (areaType as AreaType) : sceneType,
    status: isBusiness ? "confirmed_business" : "confirmed_non_business",
    confirmedAt: new Date().toISOString(),
  };
}

function channelDraftFromChannel(channel: VideoChannel): Partial<VideoChannel> {
  return {
    areaType: channel.areaType,
    areaNumber: channel.areaType ? channel.areaNumber : channel.areaNote || channel.areaNumber,
    areaNote: channel.areaNote,
    sceneType: channel.sceneType,
    status: "pending_confirmation",
  };
}

function isConfirmedChannel(channel: VideoChannel) {
  return channel.status === "confirmed_business" || channel.status === "confirmed_non_business";
}

function recognitionProgressLabel(progress?: { done: number; total: number }) {
  if (!progress || progress.total <= 0) return "正在准备识别";
  return `识别进度 ${progress.done}/${progress.total} · ${recognitionProgressPercent(progress)}%`;
}

function fallbackProbeProgressLabel(progress: { checked: number; active: number }) {
  return `抓图识别中 · 已检测 ${progress.checked} 个，有效 ${progress.active} 个`;
}

function shouldUseFallbackProbe(message: string) {
  return message.includes("10026") || message.includes("设备数量超出个人版限制");
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
  return channelSceneLabel(sceneType);
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
