import { useEffect, useMemo, useState } from "react";
import { storeSpaceApi, type AuditLogItem, type ManagedUser } from "../api";
import { isIdleSessionTimeout } from "../domain/auth";
import { errorMessage, formatDateTime } from "../domain/format";

const actionLabels: Record<string, string> = {
  "auth.login": "登录",
  "auth.logout": "退出登录",
  "monitor.live_view": "查看直播",
  "monitor.playback_view": "查看回放",
  "snapshot.view": "查看截图",
  // Keep rendering records written before the explicit-view audit endpoint.
  "snapshot.download": "查看截图",
  "snapshot.refresh": "刷新截图",
  "user.create": "新增用户",
  "user.update": "更新用户权限",
};

const actionOptions = Object.entries(actionLabels).filter(([action]) => action !== "snapshot.download");

function localDate(offsetDays = 0) {
  const date = new Date();
  date.setDate(date.getDate() + offsetDays);
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

  const pageCount = Math.max(1, Math.ceil(total / 20));
  const sortedUsers = useMemo(() => [...users].sort((a, b) => a.email.localeCompare(b.email)), [users]);

  useEffect(() => {
    void Promise.all([storeSpaceApi.listUsers(), loadLogs(1)]).then(([nextUsers]) => setUsers(nextUsers)).catch((error) => {
      if (isIdleSessionTimeout(error) && onAuthRequired) {
        onAuthRequired(error);
        return;
      }
      onToast(errorMessage(error, "操作日志加载失败。"));
    });
    // The initial request intentionally uses the default seven-day range.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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
      const response = await storeSpaceApi.listAuditLogs(queryStartDate, queryEndDate, queryUserId, queryAction, nextPage);
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
        <label>
          <span>操作人</span>
          <select value={userId} onChange={(event) => setUserId(event.target.value)}>
            <option value="">全部操作人</option>
            {sortedUsers.map((user) => (
              <option key={user.id} value={user.id}>
                {user.displayName || user.username || user.email}
              </option>
            ))}
          </select>
        </label>
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
