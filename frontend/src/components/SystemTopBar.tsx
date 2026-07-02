import type { ReactNode } from "react";
import { authUserDisplayName, type AuthState } from "../domain/auth";

export type SystemTopBarProps = {
  backAction?: { label: string; onClick: () => void };
  auth?: AuthState | null;
  loggingOut?: boolean;
  onLogout?: () => void | Promise<void>;
  rightExtra?: ReactNode;
};

export function SystemTopBar({ backAction, auth, loggingOut = false, onLogout, rightExtra }: SystemTopBarProps) {
  const showLogout = Boolean(auth?.authenticated && onLogout);
  const displayName = authUserDisplayName(auth?.user);

  return (
    <header className="system-topbar" aria-label="系统操作栏">
      <div className="system-topbar-left">
        {backAction ? (
          <button type="button" className="system-back-button" onClick={backAction.onClick} aria-label={backAction.label}>
            <span aria-hidden="true">←</span>
            <span>{backAction.label}</span>
          </button>
        ) : null}
      </div>
      <div className="system-topbar-right">
        {rightExtra}
        {showLogout ? (
          <div className="auth-user-chip" aria-label="当前登录用户">
            <span className="auth-user-name">{displayName}</span>
            <button type="button" className="plain-button auth-logout-button" onClick={() => void onLogout?.()} disabled={loggingOut}>
              {loggingOut ? "退出中..." : "退出登录"}
            </button>
          </div>
        ) : null}
      </div>
    </header>
  );
}
