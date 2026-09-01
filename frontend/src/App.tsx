import { Suspense, lazy, useEffect, useRef, useState } from "react";
import {
  ApiError,
  storeSpaceResourceViewApi,
  storeSpaceApi,
  type ResourceCityOption,
  type ResourceStoreDetail as ResourceStoreDetailType,
  type ResourceStoreListSummary,
  type ResourceStoreSummary,
} from "./api";
import { EzvizLiveDemo } from "./components/EzvizLiveDemo";
import { ResourceStoreDetail } from "./components/ResourceStoreDetail";
import { ResourceStoreList } from "./components/ResourceStoreList";
import { SystemTopBar } from "./components/SystemTopBar";
import { UserManagement } from "./components/UserManagement";
import { AuditLogManagement } from "./components/AuditLogManagement";
import {
  authCompanyEntryPath,
  claimIdleSessionTimeoutRedirect,
  authLoginPath,
  authLogoutPath,
  canManageUsers,
  idleSessionTimeoutRedirectKey,
  isIdleSessionTimeout,
  shouldBlockBusinessData,
  shouldShowForbiddenAccess,
  shouldShowLoginWelcome,
  shouldShowLogoutEntry,
  shouldSkipLocalLogoutBeforeRedirect,
  type AuthState,
} from "./domain/auth";
import { errorMessage } from "./domain/format";
import {
  h5MonitorTabSearch,
  readH5MonitorActiveTabFromSearch,
  type H5MonitorTabKey,
} from "./domain/h5-monitor-active-tab";
import { nvrLabApi } from "./api-nvr-lab";
import { parseNVRLabRoute, nvrLabRoutePath, nvrMonitorRoutePath } from "./domain/nvr-lab";

const PAGE_SIZE = 20;
const APP_VERSION = import.meta.env.VITE_APP_VERSION || "local-dev";
const EMPTY_RESOURCE_LIST_SUMMARY: ResourceStoreListSummary = {
  storeCount: 0,
  edgeCount: 0,
  nvrCount: 0,
  cameraCount: 0,
  spaceCount: 0,
  consultationCameraCount: 0,
  treatmentCameraCount: 0,
  boundCameraCount: 0,
  unboundCameraCount: 0,
  offlineDeviceCount: 0,
  warningCount: 0,
};

const H5MonitorPage = lazy(() => import("./pages/H5Monitor").then((module) => ({ default: module.H5Monitor })));
const H5MonitorChannelPage = lazy(() =>
  import("./pages/H5MonitorChannel").then((module) => ({ default: module.H5MonitorChannel })),
);
const NVRLabMonitorPage = lazy(() => import("./pages/NVRLabMonitor").then((module) => ({ default: module.NVRLabMonitor })));
const NVRLabCameraPage = lazy(() => import("./pages/NVRLabCamera").then((module) => ({ default: module.NVRLabCamera })));

type H5Route =
  | { name: "home"; externalOrgId: string; tab?: H5MonitorTabKey }
  | { name: "channel"; externalOrgId: string; channelId: number; tab?: H5MonitorTabKey }
	| { name: "nvr-camera"; externalOrgId: string; cameraId: number }
  | { name: "nvr-lab-home" }
  | { name: "nvr-lab-camera"; cameraId: number }
  | null;

function App() {
  const h5Route = parseH5Route();
  if (h5Route) {
    return <H5RouteShell initialRoute={h5Route} />;
  }

  return <AdminApp />;
}

function AdminApp() {
  const [auth, setAuth] = useState<AuthState | null>(null);
  const [authLoading, setAuthLoading] = useState(true);
  const [stores, setStores] = useState<ResourceStoreSummary[]>([]);
  const [query, setQuery] = useState("");
  const [cityFilter, setCityFilter] = useState<number | "all">("all");
  const [cityOptions, setCityOptions] = useState<ResourceCityOption[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [listSummary, setListSummary] = useState<ResourceStoreListSummary>(EMPTY_RESOURCE_LIST_SUMMARY);
  const [loading, setLoading] = useState(true);
  const [loggingOut, setLoggingOut] = useState(false);
  const [toast, setToast] = useState("");
  const [activeStore, setActiveStore] = useState<ResourceStoreDetailType | null>(null);
  const [openingStoreIds, setOpeningStoreIds] = useState<Set<number>>(() => new Set());
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [settingsSection, setSettingsSection] = useState<"users" | "audit">("users");
  const listRequestIdRef = useRef(0);
  const detailRequestIdRef = useRef(0);
  const authRedirectingRef = useRef(false);

  if (new URLSearchParams(window.location.search).get("tool") === "ezviz-live-demo") {
    return <EzvizLiveDemo appVersion={APP_VERSION} />;
  }

  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const visibleStores = stores;
  const visibleSummary = listSummary;
  const visibleFirstIndex = visibleStores.length === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const visibleLastIndex = visibleStores.length === 0 ? 0 : visibleFirstIndex + visibleStores.length - 1;
  const showLogoutEntry = shouldShowLogoutEntry(auth);
  const canManageSystemUsers = canManageUsers(auth);
  const systemSettingsAction = canManageSystemUsers ? (
    <button type="button" className="topbar-settings-button" onClick={() => { setSettingsSection("users"); setSettingsOpen(true); }}>
      系统设置
    </button>
  ) : null;

  function handleAuthRequired(error?: unknown) {
    const idleTimeout = isIdleSessionTimeout(error);
    const logoutPath = authLogoutPath();
    setAuth({
      enabled: true,
      authenticated: false,
      code: idleTimeout ? "session_idle_timeout" : undefined,
      login_url: idleTimeout ? logoutPath : authLoginPath(),
    });
    if (idleTimeout && !authRedirectingRef.current) {
      authRedirectingRef.current = true;
      if (claimIdleSessionTimeoutRedirect(window.sessionStorage)) {
        window.location.assign(logoutPath);
      }
    }
  }

  useEffect(() => {
    void storeSpaceApi
      .getAuthMe()
      .then(setAuth)
      .catch((error) => {
        if (error instanceof Error && "status" in error && (error as { status?: number }).status === 401) {
          handleAuthRequired(error);
          return;
        }
        if (error instanceof Error && "status" in error && (error as { status?: number }).status === 403) {
          setAuth({ enabled: true, authenticated: false, forbidden: true });
          return;
        }
        setToast(errorMessage(error, "登录状态加载失败。"));
        setAuth({ enabled: false, authenticated: true });
      })
      .finally(() => setAuthLoading(false));
  }, []);

  useEffect(() => {
    if (settingsOpen || authLoading || shouldBlockBusinessData(auth)) return;
    void loadStores(query, cityFilter, page);
  }, [page, query, cityFilter, authLoading, auth, settingsOpen]);

  useEffect(() => {
    if (auth?.authenticated) {
      window.sessionStorage.removeItem("erzhuang:sso-entry-redirected");
      window.sessionStorage.removeItem(idleSessionTimeoutRedirectKey);
    }
  }, [auth]);

  useEffect(() => {
    if (!shouldShowLoginWelcome(auth)) return;
    const companyEntryPath = authCompanyEntryPath();
    if (!companyEntryPath) return;
    const redirectKey = "erzhuang:sso-entry-redirected";
    if (window.sessionStorage.getItem(redirectKey) === "1") return;
    window.sessionStorage.setItem(redirectKey, "1");
    window.location.replace(companyEntryPath);
  }, [auth]);

  async function loadStores(nextQuery = query, nextCityFilter = cityFilter, nextPage = page) {
    const requestId = listRequestIdRef.current + 1;
    listRequestIdRef.current = requestId;
    setLoading(true);
    try {
      const response = await storeSpaceResourceViewApi.listStores(nextQuery.trim(), nextPage, PAGE_SIZE, nextCityFilter);
      if (listRequestIdRef.current !== requestId) return;
      setStores(response.items);
      setTotal(response.total);
      setListSummary(response.summary);
      if (nextCityFilter === "all") {
        setCityOptions(response.cities);
      }
    } catch (error) {
      if (listRequestIdRef.current !== requestId) return;
      if (isIdleSessionTimeout(error)) {
        handleAuthRequired(error);
        return;
      }
      setStores([]);
      setTotal(0);
      setListSummary(EMPTY_RESOURCE_LIST_SUMMARY);
      if (nextCityFilter === "all") {
        setCityOptions([]);
      }
      setToast(storeListLoadErrorMessage(error));
    } finally {
      if (listRequestIdRef.current === requestId) {
        setLoading(false);
      }
    }
  }

  function handleSearch(value: string) {
    setQuery(value);
    setCityFilter("all");
    setPage(1);
  }

  function handleCityFilter(value: number | "all") {
    setCityFilter(value);
    setPage(1);
  }

  async function openStore(tenantId: number) {
    if (openingStoreIds.has(tenantId)) return;
    const requestId = detailRequestIdRef.current + 1;
    detailRequestIdRef.current = requestId;
    setOpeningStoreIds((current) => new Set(current).add(tenantId));
    try {
      const startedAt = performance.now();
      const detail = await storeSpaceResourceViewApi.getStore(tenantId);
      if (detailRequestIdRef.current !== requestId) return;
      console.info(`[resource-view] loaded ${tenantId} in ${Math.round(performance.now() - startedAt)}ms`);
      setActiveStore(detail);
    } catch (error) {
      if (detailRequestIdRef.current !== requestId) return;
      if (isIdleSessionTimeout(error)) {
        handleAuthRequired(error);
        return;
      }
      setActiveStore(null);
      setToast(errorMessage(error, "门店详情加载失败。"));
    } finally {
      setOpeningStoreIds((current) => removeIdFromSet(current, tenantId));
    }
  }

  async function logout() {
    const logoutPath = authLogoutPath();
    setLoggingOut(true);
    if (shouldSkipLocalLogoutBeforeRedirect()) {
      window.location.assign(logoutPath);
      return;
    }
    try {
      await storeSpaceApi.logout();
      setAuth({ enabled: true, authenticated: false, login_url: logoutPath });
      window.location.assign(logoutPath);
    } catch (error) {
      setToast(errorMessage(error, "本地退出状态清理失败，正在尝试退出 SSO。"));
      window.location.assign(logoutPath);
    } finally {
      setLoggingOut(false);
    }
  }

  function returnToStoreList() {
    detailRequestIdRef.current += 1;
    setActiveStore(null);
    setSettingsOpen(false);
    setSettingsSection("users");
    void loadStores();
  }

  if (activeStore) {
    return (
      <main className="app-shell">
        <SystemTopBar
          backAction={{ label: "返回列表", onClick: returnToStoreList }}
          auth={showLogoutEntry ? auth : null}
          loggingOut={loggingOut}
          onLogout={logout}
          rightExtra={systemSettingsAction}
        />
        {toast ? <Toast message={toast} onClose={() => setToast("")} /> : null}
        <ResourceStoreDetail
          store={activeStore}
          onOpenMonitor={(url) => window.location.assign(url)}
        />
        <footer className="app-version" aria-label="当前版本">
          版本 {APP_VERSION}
        </footer>
      </main>
    );
  }

  if (authLoading) {
    return (
      <main className="app-shell">
        <div className="auth-loading">正在确认登录状态...</div>
      </main>
    );
  }

  if (shouldShowLoginWelcome(auth) && authCompanyEntryPath()) {
    return (
      <main className="app-shell">
        <div className="auth-loading">正在进入公司 SSO 登录...</div>
      </main>
    );
  }

  if (shouldShowLoginWelcome(auth)) {
    return <LoginWelcome auth={auth} appVersion={APP_VERSION} />;
  }

  if (shouldShowForbiddenAccess(auth)) {
    return <ForbiddenAccess appVersion={APP_VERSION} />;
  }

  if (settingsOpen && canManageSystemUsers) {
    return (
      <main className="app-shell">
        <SystemTopBar
          backAction={{ label: "返回列表", onClick: returnToStoreList }}
          auth={showLogoutEntry ? auth : null}
          loggingOut={loggingOut}
          onLogout={logout}
        />
        {toast ? <Toast message={toast} onClose={() => setToast("")} /> : null}
        <nav className="settings-tabs" aria-label="系统设置菜单">
          <button type="button" className={settingsSection === "users" ? "is-active" : ""} onClick={() => setSettingsSection("users")}>
            用户管理
          </button>
          <button type="button" className={settingsSection === "audit" ? "is-active" : ""} onClick={() => setSettingsSection("audit")}>
            操作日志
          </button>
        </nav>
        {settingsSection === "audit" ? <AuditLogManagement onToast={setToast} /> : <UserManagement onToast={setToast} />}
        <footer className="app-version" aria-label="当前版本">
          版本 {APP_VERSION}
        </footer>
      </main>
    );
  }

  return (
    <main className="app-shell">
      <SystemTopBar auth={showLogoutEntry ? auth : null} loggingOut={loggingOut} onLogout={logout} rightExtra={systemSettingsAction} />
      <header className="page-header">
        <div>
          <p className="eyebrow">门店空间资源查看</p>
          <h1>门店空间资源查看</h1>
        </div>
      </header>

      <section className="toolbar" aria-label="门店筛选">
        <div className="toolbar-main">
          <label className="search-field">
            <span aria-hidden="true">⌕</span>
            <input value={query} onChange={(event) => handleSearch(event.target.value)} placeholder="搜索门店名称" aria-label="搜索门店名称" />
          </label>
          <div className="toolbar-meta">
            <span>
              共 {visibleSummary.storeCount} 家门店
              {visibleSummary.storeCount > 0 ? `，当前 ${visibleFirstIndex}-${visibleLastIndex}` : ""}
            </span>
            <span>工控机 {visibleSummary.edgeCount}</span>
            <span>录像机 {visibleSummary.nvrCount}</span>
            <span>摄像头 {visibleSummary.cameraCount}</span>
            <span>已绑定 {visibleSummary.boundCameraCount}</span>
          </div>
        </div>
        <div className="city-filter" role="radiogroup" aria-label="城市筛选">
          <button type="button" role="radio" aria-checked={cityFilter === "all"} className={cityFilter === "all" ? "is-active" : ""} onClick={() => handleCityFilter("all")}>
            全部
          </button>
          {cityOptions.map((city) => (
            <button
              type="button"
              role="radio"
              aria-checked={cityFilter === city.cityId}
              className={cityFilter === city.cityId ? "is-active" : ""}
              key={city.cityId}
              onClick={() => handleCityFilter(city.cityId)}
            >
              {city.name || `城市 ${city.cityId}`}
            </button>
          ))}
        </div>
      </section>

      {toast ? <Toast message={toast} onClose={() => setToast("")} /> : null}

      <ResourceStoreList
        stores={visibleStores}
        loading={loading}
        page={page}
        pageSize={PAGE_SIZE}
        openingStoreIds={openingStoreIds}
        onOpenStore={openStore}
        onOpenMonitor={(store) => {
          if (store.monitorUrl) window.location.assign(store.monitorUrl);
        }}
      />

      <nav className="pagination" aria-label="分页">
        <button disabled={page <= 1} onClick={() => setPage((value) => Math.max(1, value - 1))}>
          上一页
        </button>
        <span>
          第 {page} / {pageCount} 页
        </span>
        <button disabled={page >= pageCount} onClick={() => setPage((value) => Math.min(pageCount, value + 1))}>
          下一页
        </button>
      </nav>

      <footer className="app-version" aria-label="当前版本">
        版本 {APP_VERSION}
      </footer>

    </main>
  );
}

function storeListLoadErrorMessage(error: unknown) {
  const message = errorMessage(error, "门店列表加载失败，请稍后重试。");
  if (error instanceof ApiError && (error.code || error.stage || error.detail)) {
    let diagnostics = "";
    if (error.code) diagnostics += ` code=${error.code}`;
    if (error.stage) diagnostics += ` stage=${error.stage}`;
    if (error.detail) diagnostics += ` detail=${error.detail}`;
    return `${message} ·${diagnostics}`;
  }
  return message;
}

function LoginWelcome({ auth, appVersion }: { auth: AuthState | null; appVersion: string }) {
  const companyEntryPath = authCompanyEntryPath();
  const loginPath = companyEntryPath || authLoginPath(auth?.login_url);

  return (
    <main className="auth-page">
      <section className="auth-panel" aria-label="登录">
        <p className="eyebrow">新氧青春</p>
        <h1>门店空间资源管理系统</h1>
        <p className="auth-copy">请通过公司 APISIX-SSO 网关登录后继续访问后台。</p>
        <button className="primary-button auth-login-button" onClick={() => window.location.assign(loginPath)}>
          使用公司 SSO 登录
        </button>
        <p className="auth-note">登录由公司网关统一处理；权限范围由项目用户表控制。</p>
      </section>
      <footer className="app-version" aria-label="当前版本">
        版本 {appVersion}
      </footer>
    </main>
  );
}

function ForbiddenAccess({ appVersion }: { appVersion: string }) {
  return (
    <main className="auth-page">
      <section className="auth-panel" aria-label="暂无访问权限">
        <p className="eyebrow">访问受限</p>
        <h1>暂无访问权限</h1>
        <p className="auth-copy">当前公司账号尚未被授权访问二壮系统，请联系项目负责人开通权限，或更换账号登录。</p>
        <button className="primary-button auth-login-button" onClick={() => window.location.assign(authLogoutPath())}>
          重新登录
        </button>
        <p className="auth-note">点击后会先退出公司 SSO，再通过项目首页重新唤起登录。</p>
      </section>
      <footer className="app-version" aria-label="当前版本">
        版本 {appVersion}
      </footer>
    </main>
  );
}

function H5RouteShell({ initialRoute }: { initialRoute: H5Route }) {
  const [route, setRoute] = useState<H5Route>(initialRoute);
  const [auth, setAuth] = useState<AuthState | null>(null);
  const [authLoading, setAuthLoading] = useState(true);
  const [loggingOut, setLoggingOut] = useState(false);
  const [authMessage, setAuthMessage] = useState("");
  const authRedirectingRef = useRef(false);
  const [monitorMode, setMonitorMode] = useState<"legacy" | "nvr">("legacy");
  const [monitorModeLoading, setMonitorModeLoading] = useState(true);
  const showLogoutEntry = shouldShowLogoutEntry(auth);

  function handleAuthRequired(error?: unknown) {
    const idleTimeout = isIdleSessionTimeout(error);
    const logoutPath = authLogoutPath();
    setAuth({
      enabled: true,
      authenticated: false,
      code: idleTimeout ? "session_idle_timeout" : undefined,
      login_url: idleTimeout ? logoutPath : authLoginPath(),
    });
    setAuthMessage(idleTimeout ? "登录已因长时间未操作失效，请重新扫码登录。" : "");
    if (idleTimeout && !authRedirectingRef.current) {
      authRedirectingRef.current = true;
      if (claimIdleSessionTimeoutRedirect(window.sessionStorage)) {
        window.location.assign(logoutPath);
      }
    }
  }

  useEffect(() => {
    const onPopState = () => setRoute(parseH5Route());
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  useEffect(() => {
    void storeSpaceApi
      .getAuthMe()
      .then((nextAuth) => {
        setAuth(nextAuth);
        setAuthMessage("");
      })
      .catch((error) => {
        if (error instanceof Error && "status" in error && (error as { status?: number }).status === 401) {
          handleAuthRequired(error);
          return;
        }
        if (error instanceof Error && "status" in error && (error as { status?: number }).status === 403) {
          setAuth({ enabled: true, authenticated: false, forbidden: true });
          return;
        }
        setAuth({ enabled: false, authenticated: false });
        setAuthMessage(errorMessage(error, "登录状态加载失败，请稍后重试。"));
      })
      .finally(() => setAuthLoading(false));
  }, []);

  useEffect(() => {
    if (auth?.authenticated) {
      window.sessionStorage.removeItem("erzhuang:h5-sso-entry-redirected");
      window.sessionStorage.removeItem(idleSessionTimeoutRedirectKey);
    }
  }, [auth]);

  useEffect(() => {
    if (!auth?.authenticated) {
      setMonitorMode("legacy");
      setMonitorModeLoading(false);
      return;
    }
    setMonitorModeLoading(true);
    let cancelled = false;
    nvrLabApi.getMonitorMode().then((response) => {
      if (!cancelled) setMonitorMode(response.mode === "nvr" ? "nvr" : "legacy");
    }).catch((error) => {
      if (cancelled) return;
      if (isIdleSessionTimeout(error)) {
        handleAuthRequired(error);
        return;
      }
      setMonitorMode("legacy");
    }).finally(() => {
      if (!cancelled) setMonitorModeLoading(false);
    });
    return () => { cancelled = true; };
  }, [auth?.authenticated]);

  useEffect(() => {
    if (!shouldShowLoginWelcome(auth)) return;
    const companyEntryPath = authCompanyEntryPath();
    if (!companyEntryPath) return;
    const redirectKey = "erzhuang:h5-sso-entry-redirected";
    if (window.sessionStorage.getItem(redirectKey) === "1") return;
    window.sessionStorage.setItem(redirectKey, "1");
    window.location.replace(companyEntryPath);
  }, [auth]);

  async function logout() {
    const logoutPath = authLogoutPath();
    setLoggingOut(true);
    if (shouldSkipLocalLogoutBeforeRedirect()) {
      window.location.assign(logoutPath);
      return;
    }
    try {
      await storeSpaceApi.logout();
      setAuth({ enabled: true, authenticated: false, login_url: logoutPath });
      window.location.assign(logoutPath);
    } catch (error) {
      setAuthMessage(errorMessage(error, "本地退出状态清理失败，正在尝试退出 SSO。"));
      window.location.assign(logoutPath);
    } finally {
      setLoggingOut(false);
    }
  }

  if (!route) {
    return (
      <div className="h5-not-found">
        <p>页面不存在。请通过门店监控二维码进入。</p>
      </div>
    );
  }

  if (authLoading) {
    return (
      <main className="app-shell">
        <div className="auth-loading">正在确认登录状态...</div>
      </main>
    );
  }

  if (shouldShowForbiddenAccess(auth)) {
    return <ForbiddenAccess appVersion={APP_VERSION} />;
  }

  if (shouldShowLoginWelcome(auth) && authCompanyEntryPath()) {
    return (
      <main className="app-shell">
        <div className="auth-loading">正在进入公司 SSO 登录...</div>
      </main>
    );
  }

  if (shouldShowLoginWelcome(auth)) {
    return <LoginWelcome auth={auth} appVersion={APP_VERSION} />;
  }

	if (monitorModeLoading) {
		return <main className="app-shell"><div className="auth-loading">正在加载监控配置...</div></main>;
	}

	if (monitorMode === "nvr" && route.name === "home") {
		return <Suspense fallback={<div className="h5-loading">加载中...</div>}><NVRLabMonitorPage
			externalOrgId={route.externalOrgId}
			auth={showLogoutEntry ? auth : null}
			loggingOut={loggingOut}
			authMessage={authMessage}
			onLogout={logout}
			onAuthRequired={handleAuthRequired}
			onOpenCamera={(cameraId) => {
				window.history.pushState({}, "", nvrMonitorRoutePath({ name: "camera", externalOrgId: route.externalOrgId, cameraId }));
				setRoute({ name: "nvr-camera", externalOrgId: route.externalOrgId, cameraId });
			}}
			onSelectStore={(externalOrgId) => {
				window.history.pushState({}, "", nvrMonitorRoutePath({ name: "home", externalOrgId }));
				setRoute({ name: "home", externalOrgId });
			}}
		/></Suspense>;
	}

	if (monitorMode === "nvr" && route.name === "nvr-camera") {
		return <Suspense fallback={<div className="h5-loading">加载中...</div>}><NVRLabCameraPage
			externalOrgId={route.externalOrgId}
			cameraId={route.cameraId}
			auth={showLogoutEntry ? auth : null}
			loggingOut={loggingOut}
			authMessage={authMessage}
			onLogout={logout}
			onAuthRequired={handleAuthRequired}
			onBack={() => {
				window.history.replaceState({}, "", nvrMonitorRoutePath({ name: "home", externalOrgId: route.externalOrgId }));
				setRoute({ name: "home", externalOrgId: route.externalOrgId });
			}}
		/></Suspense>;
	}

	if (monitorMode === "nvr" && route.name === "channel") {
		return <main className="h5-page h5-monitor-page"><SystemTopBar auth={showLogoutEntry ? auth : null} loggingOut={loggingOut} onLogout={logout} /><div className="h5-empty">监控地址已更新，请返回摄像头列表继续查看。<button type="button" onClick={() => { window.history.replaceState({}, "", nvrMonitorRoutePath({ name: "home", externalOrgId: route.externalOrgId })); setRoute({ name: "home", externalOrgId: route.externalOrgId }); }}>返回摄像头列表</button></div></main>;
	}

	if (route.name === "nvr-camera") {
		return <main className="h5-page h5-monitor-page"><div className="h5-empty">当前监控模式不支持该摄像头地址。</div></main>;
	}

  if (route.name === "home") {
    return (
      <Suspense fallback={<div className="h5-loading">加载中...</div>}>
        <H5MonitorPage
          externalOrgId={route.externalOrgId}
          auth={showLogoutEntry ? auth : null}
          loggingOut={loggingOut}
          authMessage={authMessage}
          activeTabFromRoute={route.tab ?? null}
          onLogout={logout}
          onAuthRequired={handleAuthRequired}
          onOpenChannel={(channelId, activeTab) => {
            const url = h5ChannelRoutePath(route.externalOrgId, channelId, activeTab);
            window.history.pushState({}, "", url);
            setRoute({ name: "channel", externalOrgId: route.externalOrgId, channelId, tab: normalizeH5RouteTab(activeTab) });
          }}
          onSelectTab={(activeTab) => {
            const tab = normalizeH5RouteTab(activeTab);
            const url = h5MonitorRoutePath(route.externalOrgId, tab);
            window.history.replaceState({}, "", url);
            setRoute((currentRoute) => {
              if (!currentRoute || currentRoute.name !== "home" || currentRoute.externalOrgId !== route.externalOrgId) {
                return currentRoute;
              }
              return { ...currentRoute, tab };
            });
          }}
          onSelectStore={(externalOrgId) => {
            const url = h5MonitorRoutePath(externalOrgId);
            window.history.pushState({}, "", url);
            setRoute({ name: "home", externalOrgId });
          }}
        />
      </Suspense>
    );
  }

  if (route.name === "nvr-lab-home") {
    return (
      <Suspense fallback={<div className="h5-loading">加载中...</div>}>
        <NVRLabMonitorPage
		  externalOrgId="10001"
          auth={showLogoutEntry ? auth : null}
          loggingOut={loggingOut}
          authMessage={authMessage}
          onLogout={logout}
          onAuthRequired={handleAuthRequired}
          onOpenCamera={(cameraId) => {
            const nextRoute: H5Route = { name: "nvr-lab-camera", cameraId };
            window.history.pushState({}, "", nvrLabRoutePath({ name: "camera", cameraId }));
            setRoute(nextRoute);
          }}
		  onSelectStore={() => undefined}
        />
      </Suspense>
    );
  }

  if (route.name === "nvr-lab-camera") {
    return (
      <Suspense fallback={<div className="h5-loading">加载中...</div>}>
        <NVRLabCameraPage
		  externalOrgId="10001"
          cameraId={route.cameraId}
          auth={showLogoutEntry ? auth : null}
          loggingOut={loggingOut}
          authMessage={authMessage}
          onLogout={logout}
          onAuthRequired={handleAuthRequired}
          onBack={() => {
            window.history.replaceState({}, "", nvrLabRoutePath({ name: "home" }));
            setRoute({ name: "nvr-lab-home" });
          }}
        />
      </Suspense>
    );
  }

  return (
    <Suspense fallback={<div className="h5-loading">加载中...</div>}>
      <H5MonitorChannelPage
        externalOrgId={route.externalOrgId}
        channelId={route.channelId}
        auth={showLogoutEntry ? auth : null}
        loggingOut={loggingOut}
        authMessage={authMessage}
        onLogout={logout}
        onAuthRequired={handleAuthRequired}
        onBack={() => {
          const url = h5MonitorRoutePath(route.externalOrgId, route.tab);
          window.history.replaceState({}, "", url);
          setRoute({ name: "home", externalOrgId: route.externalOrgId, tab: route.tab });
        }}
      />
    </Suspense>
  );
}

function parseH5Route(): H5Route {
  const path = window.location.pathname;
  const nvrLabRoute = parseNVRLabRoute(path);
  if (nvrLabRoute?.name === "home") return { name: "nvr-lab-home" };
  if (nvrLabRoute?.name === "camera") return { name: "nvr-lab-camera", cameraId: nvrLabRoute.cameraId };
  const h5Path = stripKnownBasePrefix(path);
	const nvrCameraMatch = h5Path.match(/^\/h5\/orgs\/([^/]+)\/monitor\/cameras\/([^/]+)$/);
	if (nvrCameraMatch) {
		const cameraId = Number.parseInt(nvrCameraMatch[2], 10);
		if (Number.isInteger(cameraId) && cameraId > 0) {
			return { name: "nvr-camera", externalOrgId: decodeURIComponent(nvrCameraMatch[1]), cameraId };
		}
		return null;
	}
  const tab = normalizeH5RouteTab(readH5MonitorActiveTabFromSearch(window.location.search));

  const channelMatch = h5Path.match(/^\/h5\/orgs\/([^/]+)\/monitor\/channels\/([^/]+)$/);
  if (channelMatch) {
    const channelId = Number.parseInt(channelMatch[2], 10);
    if (Number.isInteger(channelId) && channelId > 0) {
      return { name: "channel", externalOrgId: decodeURIComponent(channelMatch[1]), channelId, tab };
    }
    return null;
  }

  const homeMatch = h5Path.match(/^\/h5\/orgs\/([^/]+)\/monitor$/);
  if (homeMatch) {
    return { name: "home", externalOrgId: decodeURIComponent(homeMatch[1]), tab };
  }

  return null;
}

function h5MonitorRoutePath(externalOrgId: string, tab?: H5MonitorTabKey) {
  return `${h5RoutePrefix()}/h5/orgs/${encodeURIComponent(externalOrgId)}/monitor${h5MonitorTabSearch(tab)}`;
}

function h5ChannelRoutePath(externalOrgId: string, channelId: number, tab?: H5MonitorTabKey) {
  return `${h5RoutePrefix()}/h5/orgs/${encodeURIComponent(externalOrgId)}/monitor/channels/${channelId}${h5MonitorTabSearch(tab)}`;
}

function normalizeH5RouteTab(tab: H5MonitorTabKey | null | undefined) {
  return tab && tab !== "all" ? tab : undefined;
}

function stripKnownBasePrefix(path: string) {
  for (const prefix of h5BasePrefixes()) {
    if (prefix && path === prefix) return "/";
    if (prefix && path.startsWith(`${prefix}/`)) {
      return path.slice(prefix.length) || "/";
    }
  }
  return path;
}

function h5RoutePrefix() {
  const currentPrefix = h5BasePrefixes().find((prefix) => prefix && window.location.pathname.startsWith(`${prefix}/h5/`));
  return currentPrefix || normalizeBasePrefix(import.meta.env.BASE_URL || "/erzhuang-project/");
}

function h5BasePrefixes() {
  return Array.from(new Set([normalizeBasePrefix(import.meta.env.BASE_URL || "/erzhuang-project/"), "/erzhuang-project", "/erzhuang"].filter(Boolean)));
}

function normalizeBasePrefix(value: string) {
  const normalized = value.startsWith("/") ? value : `/${value}`;
  const firstSegment = normalized.split("/").filter(Boolean)[0];
  return firstSegment ? `/${firstSegment}` : "";
}

function removeIdFromSet(current: Set<number>, id: number) {
  return removeFromSet(current, id);
}

function removeFromSet<T>(current: Set<T>, value: T) {
  const next = new Set(current);
  next.delete(value);
  return next;
}

function Toast({ message, onClose }: { message: string; onClose: () => void }) {
  return (
    <div className="toast" role="status">
      {message}
      <button onClick={onClose} aria-label="关闭提示">
        x
      </button>
    </div>
  );
}

export default App;
