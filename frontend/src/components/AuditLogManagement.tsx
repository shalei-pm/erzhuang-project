import { useEffect, useMemo, useRef, useState } from "react";
import { ApiError, storeSpaceApi, type AuditLogItem, type ManagedUser } from "../api";
import { OperatorSearchSelect } from "./OperatorSearchSelect";
import { buildOperatorNameCandidates, buildOperatorOptions, type AuditActorInput } from "../domain/audit-operators";
import { isIdleSessionTimeout } from "../domain/auth";
import { errorMessage, formatDateTime } from "../domain/format";

const actionLabels: Record<string, string> = {
  "auth.login": "登录",
  "auth.logout": "退出登录",
  "auth.idle_timeout": "登录空闲失效",
  "auth.absolute_timeout": "登录满8小时失效",
  "monitor.live_view": "查看直播",
  "monitor.playback_view": "查看回放",
  "monitor.screenshot": "监控截图",
  "snapshot.view": "查看截图",
  // Keep rendering records written before the explicit-view audit endpoint.
  "snapshot.download": "查看截图",
  "snapshot.refresh": "刷新截图",
  "user.create": "新增用户",
  "user.update": "更新用户权限",
  "system.digital_twin_whitelist.update": "更新数字孪生白名单",
};

const actionOptions = Object.entries(actionLabels).filter(([action]) => action !== "snapshot.download");

const ACTOR_SCAN_CHUNK_DAYS = 89;
const ACTOR_SCAN_MAX_PAGES = 60;

function localDate(offsetDays = 0) {
  const date = new Date();
  date.setDate(date.getDate() + offsetDays);
  return localDateString(date);
}

function localDateString(date: Date) {
  return date.toISOString().slice(0, 10);
}

export function AuditLogManagement({ onToast, onAuthRequired }: { onToast: (message: string) => void; onAuthRequired?: (error?: unknown) => void }) {
  const [users, setUsers] = useState<ManagedUser[]>([]);
  const [items, setItems] = useState<AuditLogItem[]>([]);
  const [startDate, setStartDate] = useState(() => localDate(-6));
  const [endDate, setEndDate] = useState(() => localDate());
  const [userId, setUserId] = useState("");
  const [action, setAction] = useState("");
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [logActors, setLogActors] = useState<AuditActorInput[]>([]);
  const scanGeneration = useRef(0);

  const pageCount = Math.max(1, Math.ceil(total / 20));
  const operatorOptions = useMemo(() => buildOperatorOptions(users, [...logActors, ...items]), [users, logActors, items]);
  const operatorNameCandidates = useMemo(() => buildOperatorNameCandidates([...logActors, ...items], users), [logActors, items, users]);

  useEffect(() => {
    void storeSpaceApi
      .listUsers()
      .then(setUsers)
      .catch((error) => {
        if (isIdleSessionTimeout(error) && onAuthRequired) {
          onAuthRequired(error);
          return;
        }
        onToast(errorMessage(error, "操作人列表加载失败。"));
      });
    void loadLogs(1);
    void scanLogActors();
    // The initial request intentionally uses the default seven-day range.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Historical logs may carry actor names without user_id; scan the retained
  // range in 3-month chunks so those operators stay searchable.
  async function scanLogActors() {
    const generation = ++scanGeneration.current;
    const collected: AuditActorInput[] = [];
    const today = new Date();
    for (let chunk = 0; chunk < 8; chunk += 1) {
      const end = new Date(today);
      end.setDate(end.getDate() - chunk * ACTOR_SCAN_CHUNK_DAYS);
      const start = new Date(end);
      start.setDate(start.getDate() - ACTOR_SCAN_CHUNK_DAYS + 1);
      let reachedData = false;
      for (let page = 1; page <= ACTOR_SCAN_MAX_PAGES; page += 1) {
        let response;
        try {
          response = await storeSpaceApi.listAuditLogs(localDateString(start), localDateString(end), "", "", page);
        } catch (error) {
          if (error instanceof ApiError && error.status === 400) break;
          return;
        }
        if (generation !== scanGeneration.current) return;
        collected.push(...response.items.map((item) => ({ userId: item.userId, actorDisplayName: item.actorDisplayName, userEmail: item.userEmail })));
        if (response.items.length > 0) reachedData = true;
        if (page * 20 >= response.total || response.items.length === 0) break;
      }
      setLogActors([...collected]);
      if (!reachedData && chunk >= 1) break;
    }
  }

  async function loadLogs(nextPage = page, filters = { startDate, endDate, userId, action }) {
    const { startDate: queryStartDate, endDate: queryEndDate, userId: queryUserId, action: queryAction } = filters;
    if (!queryStartDate || !queryEndDate || queryStartDate > queryEndDate) {
      onToast("请选择有效的时间范围。");
      return;
    }
    const start = new Date(`${queryStartDate}T00:00:00+08:00`);
    const end = new Date(`${queryEndDate}T00:00:00+08:00`);
    const maxEnd = new Date(start);
    maxEnd.setMonth(maxEnd.getMonth() + 3);
    if (end > maxEnd) {
      onToast("一次最多查询 3 个月的操作日志。");
      return;
    }
    setLoading(true);
    try {
      const response = queryUserId.startsWith("name:") || queryUserId.startsWith("email:")
        ? await loadLogsByActorName(queryStartDate, queryEndDate, queryUserId, nextPage)
        : await storeSpaceApi.listAuditLogs(queryStartDate, queryEndDate, queryUserId, queryAction, nextPage);
      setItems(response.items);
      setPage(response.page);
      setTotal(response.total);
    } catch (error) {
      if (isIdleSessionTimeout(error) && onAuthRequired) {
        onAuthRequired(error);
        return;
      }
      onToast(errorMessage(error, "操作日志加载失败。"));
    } finally {
      setLoading(false);
    }
  }

  // Name candidates cover historical rows without user_id; the backend has no
  // actor filter, so fetch the range and match the recorded name client-side.
  async function loadLogsByActorName(startDateValue: string, endDateValue: string, key: string, targetPage: number) {
    const candidate = operatorNameCandidates.find((entry) => entry.key === key);
    const name = (candidate?.displayName || "").trim();
    const email = (candidate?.email || "").trim().toLowerCase();
    const matched: AuditLogItem[] = [];
    for (let page = 1; page <= ACTOR_SCAN_MAX_PAGES; page += 1) {
      const response = await storeSpaceApi.listAuditLogs(startDateValue, endDateValue, "", "", page);
      for (const item of response.items) {
        const itemEmail = (item.userEmail || "").trim().toLowerCase();
        const hit = email ? itemEmail === email : (item.actorDisplayName || "").trim() === name;
        if (hit) matched.push(item);
      }
      if (page * 20 >= response.total || response.items.length === 0) break;
    }
    const filtered = action ? matched.filter((item) => item.action === action) : matched;
    const offset = (targetPage - 1) * 20;
    return { items: filtered.slice(offset, offset + 20), page: targetPage, pageSize: 20, total: filtered.length };
  }

  function query() {
    void loadLogs(1);
  }

  function reset() {
    const nextStartDate = localDate(-6);
    const nextEndDate = localDate();
    setUserId("");
    setAction("");
    setStartDate(nextStartDate);
    setEndDate(nextEndDate);
    setPage(1);
    void loadLogs(1, { startDate: nextStartDate, endDate: nextEndDate, userId: "", action: "" });
  }

  return (
    <section className="audit-log-page" aria-label="操作日志">
      <header className="page-header compact-page-header">
        <div>
          <p className="eyebrow">系统设置</p>
          <h1>操作日志</h1>
        </div>
      </header>

      <section className="audit-log-toolbar" aria-label="操作日志筛选">
        <label>
          <span>开始日期</span>
          <input type="date" value={startDate} onChange={(event) => setStartDate(event.target.value)} />
        </label>
        <label>
          <span>结束日期</span>
          <input type="date" value={endDate} onChange={(event) => setEndDate(event.target.value)} />
        </label>
        <div className="audit-field">
          <span>操作人</span>
          <OperatorSearchSelect users={operatorOptions} nameCandidates={operatorNameCandidates} value={userId} onChange={setUserId} />
        </div>
        <label>
          <span>操作类型</span>
          <select value={action} onChange={(event) => setAction(event.target.value)}>
            <option value="">全部类型</option>
            {actionOptions.map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </label>
        <div className="audit-log-toolbar-actions">
          <button className="primary-button" type="button" onClick={query} disabled={loading}>
            查询
          </button>
          <button type="button" onClick={reset} disabled={loading}>
            重置
          </button>
        </div>
      </section>

      <section className="table-frame" aria-label="操作日志列表">
        <table className="audit-log-table">
          <thead>
            <tr>
              <th>操作人</th>
              <th>邮箱地址</th>
              <th>操作时间</th>
              <th>操作类型</th>
              <th>操作内容</th>
              <th>结果</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={6} className="empty-cell">正在加载操作日志</td></tr>
            ) : items.length === 0 ? (
              <tr><td colSpan={6} className="empty-cell">暂无操作日志</td></tr>
            ) : (
              items.map((item, index) => (
                <tr key={`${item.createdAt}-${item.action}-${index}`}>
                  <td>{item.actorDisplayName || "-"}</td>
                  <td>{item.userEmail || "-"}</td>
                  <td>{item.createdAt ? formatDateTime(item.createdAt) : "-"}</td>
                  <td>{actionLabels[item.action] || item.action || "-"}</td>
                  <td>{item.summary || "-"}</td>
                  <td><span className={`audit-result audit-result-${item.result}`}>{resultLabel(item.result)}</span></td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </section>

      <nav className="pagination" aria-label="操作日志分页">
        <button type="button" disabled={loading || page <= 1} onClick={() => void loadLogs(page - 1)}>上一页</button>
        <span>第 {page} / {pageCount} 页</span>
        <button type="button" disabled={loading || page >= pageCount} onClick={() => void loadLogs(page + 1)}>下一页</button>
      </nav>
    </section>
  );
}

function resultLabel(result: string) {
  if (result === "success") return "成功";
  if (result === "denied") return "拒绝";
  if (result === "failed") return "失败";
  return result || "-";
}
