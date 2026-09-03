import { useEffect, useMemo, useState } from "react";
import { storeSpaceApi, type ManagedUser, type ManagedUserPayload, type ManagedUserRole, type MonitorStoreScope } from "../api";
import { isIdleSessionTimeout } from "../domain/auth";
import { errorMessage, formatDateTime } from "../domain/format";

const roleLabels: Record<ManagedUserRole, string> = {
  admin: "管理员",
  editor: "编辑运维",
  viewer: "普通查看",
};

type UserManagementProps = {
  onToast: (message: string) => void;
  onAuthRequired?: (error?: unknown) => void;
};

type UserFormState = {
  id?: number;
  email: string;
  username: string;
  displayName: string;
  role: ManagedUserRole;
  enabled: boolean;
  monitorStoreScopeIds: number[];
};

const emptyForm: UserFormState = {
  email: "",
  username: "",
  displayName: "",
  role: "viewer",
  enabled: true,
  monitorStoreScopeIds: [],
};

export function UserManagement({ onToast, onAuthRequired }: UserManagementProps) {
  const [users, setUsers] = useState<ManagedUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState<UserFormState>(emptyForm);
  const [formOpen, setFormOpen] = useState(false);
  const [scopeCandidates, setScopeCandidates] = useState<MonitorStoreScope[]>([]);
  const [scopeLoading, setScopeLoading] = useState(false);
  const [scopeCity, setScopeCity] = useState("全部");
  const [scopeQuery, setScopeQuery] = useState("");

  useEffect(() => {
    void loadUsers();
  }, []);

  const sortedUsers = useMemo(
    () =>
      [...users].sort((a, b) => {
        if (a.enabled !== b.enabled) return a.enabled ? -1 : 1;
        const roleOrder = roleRank(a.role) - roleRank(b.role);
        if (roleOrder !== 0) return roleOrder;
        return a.email.localeCompare(b.email);
      }),
    [users],
  );

  const scopeCities = useMemo(() => ["全部", ...Array.from(new Set(scopeCandidates.map((store) => store.city || "未分城市")))], [scopeCandidates]);

  const filteredScopeCandidates = useMemo(() => {
    const queryValue = scopeQuery.trim().toLowerCase();
    const rows = scopeCandidates.filter((store) => {
      const city = store.city || "未分城市";
      if (scopeCity !== "全部" && city !== scopeCity) return false;
      if (!queryValue) return true;
      return `${city} ${store.name} ${store.externalOrgId}`.toLowerCase().includes(queryValue);
    });
    if (scopeCity !== "全部") return rows;
    const selected = new Set(form.monitorStoreScopeIds);
    return [...rows].sort((a, b) => Number(selected.has(b.storeId)) - Number(selected.has(a.storeId)));
  }, [form.monitorStoreScopeIds, scopeCandidates, scopeCity, scopeQuery]);

  async function loadUsers() {
    setLoading(true);
    try {
      setUsers(await storeSpaceApi.listUsers());
    } catch (error) {
      if (isIdleSessionTimeout(error) && onAuthRequired) {
        onAuthRequired(error);
        return;
      }
      onToast(errorMessage(error, "用户列表加载失败。"));
    } finally {
      setLoading(false);
    }
  }

  async function ensureScopeCandidatesLoaded() {
    if (scopeCandidates.length > 0 || scopeLoading) return;
    setScopeLoading(true);
    try {
      setScopeCandidates(await storeSpaceApi.listMonitorStoreScopeCandidates());
    } catch (error) {
      if (isIdleSessionTimeout(error) && onAuthRequired) {
        onAuthRequired(error);
        return;
      }
      onToast(errorMessage(error, "门店范围加载失败。"));
    } finally {
      setScopeLoading(false);
    }
  }

  function startCreate() {
    setForm(emptyForm);
    setScopeCity("全部");
    setScopeQuery("");
    setFormOpen(true);
    void ensureScopeCandidatesLoaded();
  }

  function startEdit(user: ManagedUser) {
    setForm({
      id: user.id,
      email: user.email,
      username: user.username,
      displayName: user.displayName,
      role: user.role,
      enabled: user.enabled,
      monitorStoreScopeIds: user.monitorStoreScopes.map((scope) => scope.storeId),
    });
    setScopeCity("全部");
    setScopeQuery("");
    setFormOpen(true);
    void ensureScopeCandidatesLoaded();
  }

  async function submitUser() {
    if (saving) return;
    const payload: ManagedUserPayload = {
      email: form.id ? undefined : form.email.trim(),
      username: form.username.trim(),
      displayName: form.displayName.trim(),
      role: form.role,
      enabled: form.enabled,
      monitorStoreScopeIds: form.monitorStoreScopeIds,
    };
    if (!form.id && !payload.email) {
      onToast("企业邮箱不能为空。");
      return;
    }
    setSaving(true);
    try {
      const saved = form.id ? await storeSpaceApi.updateUser(form.id, payload) : await storeSpaceApi.createUser(payload);
      setUsers((items) => {
        const exists = items.some((item) => item.id === saved.id);
        return exists ? items.map((item) => (item.id === saved.id ? saved : item)) : [...items, saved];
      });
      setFormOpen(false);
      onToast(form.id ? "用户权限已更新。" : "用户已添加。");
    } catch (error) {
      if (isIdleSessionTimeout(error) && onAuthRequired) {
        onAuthRequired(error);
        return;
      }
      onToast(errorMessage(error, "用户保存失败。"));
    } finally {
      setSaving(false);
    }
  }

  function toggleScopeStore(storeId: number, checked: boolean) {
    setForm((value) => {
      const next = new Set(value.monitorStoreScopeIds);
      if (checked) next.add(storeId);
      else next.delete(storeId);
      return { ...value, monitorStoreScopeIds: [...next] };
    });
  }

  function selectScopeStores(storeIds: number[]) {
    setForm((value) => {
      const next = new Set(value.monitorStoreScopeIds);
      for (const storeId of storeIds) next.add(storeId);
      return { ...value, monitorStoreScopeIds: [...next] };
    });
  }

  return (
    <section className="user-management-page" aria-label="用户管理">
      <header className="page-header compact-page-header">
        <div>
          <p className="eyebrow">系统设置</p>
          <h1>用户管理</h1>
        </div>
        <div className="page-header-actions">
          <button className="primary-button" onClick={startCreate}>
            <span aria-hidden="true">+</span>
            添加用户
          </button>
        </div>
      </header>

      <section className="table-frame" aria-label="用户列表">
        <table className="user-table">
          <thead>
            <tr>
              <th>企业邮箱</th>
              <th>显示名称</th>
              <th>角色</th>
              <th>监控范围</th>
              <th>状态</th>
              <th>最近登录</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={7} className="empty-cell">
                  正在加载用户列表
                </td>
              </tr>
            ) : sortedUsers.length === 0 ? (
              <tr>
                <td colSpan={7} className="empty-cell">
                  暂无用户
                </td>
              </tr>
            ) : (
              sortedUsers.map((user) => (
                <tr key={user.id}>
                  <td className="store-name">{user.email}</td>
                  <td>{user.displayName || user.username || "-"}</td>
                  <td>
                    <span className={`role-pill role-${user.role}`}>{roleLabels[user.role]}</span>
                  </td>
                  <td>{user.role === "viewer" ? `${user.monitorStoreScopeCount} 家` : "全部"}</td>
                  <td>
                    <span className={user.enabled ? "status-pill design-status-completed" : "status-pill design-status-not_uploaded"}>
                      {user.enabled ? "启用" : "停用"}
                    </span>
                  </td>
                  <td>{user.lastLoginAt ? formatDateTime(user.lastLoginAt) : "-"}</td>
                  <td>
                    <button onClick={() => startEdit(user)}>编辑</button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </section>

      {formOpen ? (
        <div className="modal-backdrop" role="presentation">
          <section className="create-modal user-modal" role="dialog" aria-modal="true" aria-label={form.id ? "编辑用户" : "添加用户"}>
            <div className="modal-head">
              <div>
                <strong>{form.id ? "编辑用户" : "添加用户"}</strong>
                <p>企业邮箱作为唯一登录身份，角色控制后台操作范围。</p>
              </div>
              <button className="modal-close-button icon-button" onClick={() => setFormOpen(false)} aria-label="关闭">
                ×
              </button>
            </div>
            <div className="user-form-grid">
              <label>
                <span>企业邮箱</span>
                <input
                  value={form.email}
                  disabled={Boolean(form.id)}
                  onChange={(event) => setForm((value) => ({ ...value, email: event.target.value }))}
                  placeholder="name@soyoung.com"
                />
              </label>
              <label>
                <span>用户名（可选）</span>
                <input value={form.username} onChange={(event) => setForm((value) => ({ ...value, username: event.target.value }))} placeholder="可选" />
              </label>
              <label>
                <span>显示名称（可选）</span>
                <input value={form.displayName} onChange={(event) => setForm((value) => ({ ...value, displayName: event.target.value }))} placeholder="可选" />
              </label>
              <label>
                <span>角色</span>
                <select value={form.role} onChange={(event) => setForm((value) => ({ ...value, role: event.target.value as ManagedUserRole }))}>
                  <option value="admin">管理员</option>
                  <option value="editor">编辑运维</option>
                  <option value="viewer">普通查看</option>
                </select>
              </label>
              <label className="user-switch-field">
                <span className="user-switch-copy">
                  <strong>登录状态</strong>
                  <em>{form.enabled ? "启用，允许通过 SSO 访问" : "停用，登录后提示暂无访问权限"}</em>
                </span>
                <button
                  type="button"
                  className={`switch-control ${form.enabled ? "is-on" : ""}`}
                  role="switch"
                  aria-checked={form.enabled}
                  onClick={() => setForm((value) => ({ ...value, enabled: !value.enabled }))}
                >
                  <span>{form.enabled ? "启用" : "停用"}</span>
                </button>
              </label>
            </div>
            {form.role === "viewer" ? (
              <section className="user-scope-panel">
                <div className="user-scope-head">
                  <div>
                    <strong>查看监控门店范围</strong>
                    <p>仅控制门店监控入口和 H5 Monitor 访问，不影响普通页面浏览。</p>
                  </div>
                  <span>已选 {form.monitorStoreScopeIds.length} 家</span>
                </div>
                <div className="user-scope-city-tabs">
                  {scopeCities.map((city) => (
                    <button key={city} type="button" className={scopeCity === city ? "active" : ""} onClick={() => setScopeCity(city)}>
                      {city}
                    </button>
                  ))}
                </div>
                <div className="user-scope-tools">
                  <input value={scopeQuery} onChange={(event) => setScopeQuery(event.target.value)} placeholder="搜索门店名 / 城市 / 机构 ID" />
                  <button type="button" onClick={() => selectScopeStores(filteredScopeCandidates.map((store) => store.storeId))}>
                    全选
                  </button>
                  <button type="button" onClick={() => setForm((value) => ({ ...value, monitorStoreScopeIds: [] }))}>
                    清空
                  </button>
                </div>
                <div className="user-scope-summary">当前筛选：{scopeCity}，共 {filteredScopeCandidates.length} 家</div>
                <div className="user-scope-list">
                  {scopeLoading ? <div className="empty-cell">正在加载门店范围</div> : null}
                  {!scopeLoading && filteredScopeCandidates.length === 0 ? <div className="empty-cell">没有匹配门店</div> : null}
                  {!scopeLoading
                    ? filteredScopeCandidates.map((store) => (
                        <label className="user-scope-row" key={store.storeId}>
                          <input
                            type="checkbox"
                            checked={form.monitorStoreScopeIds.includes(store.storeId)}
                            onChange={(event) => toggleScopeStore(store.storeId, event.target.checked)}
                          />
                          <span>{store.name}</span>
                          <em>{store.externalOrgId}</em>
                        </label>
                      ))
                    : null}
                </div>
              </section>
            ) : (
              <section className="user-scope-panel muted">
                <strong>查看监控门店范围</strong>
                <p>当前角色默认全量，不受门店范围限制。已有选择会保留，切回普通查看后恢复。</p>
              </section>
            )}
            <div className="modal-actions">
              <button onClick={() => setFormOpen(false)}>取消</button>
              <button className="primary-button" disabled={saving} onClick={() => void submitUser()}>
                {saving ? "保存中" : "保存"}
              </button>
            </div>
          </section>
        </div>
      ) : null}
    </section>
  );
}

function roleRank(role: ManagedUserRole) {
  if (role === "admin") return 1;
  if (role === "editor") return 2;
  return 3;
}
