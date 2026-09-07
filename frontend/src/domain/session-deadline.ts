import type { SessionRemaining } from "./auth";

const sessionStatusTimeoutMS = 10_000;

// Only status checks run here: they must never refresh the backend session.
export function watchSessionDeadline(
  initial: SessionRemaining,
  checkStatus: () => Promise<SessionRemaining>,
  onError: (error: unknown) => void,
): () => void {
  let stopped = false;
  let checking = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let statusTimeout: ReturnType<typeof setTimeout> | undefined;

  function stop() {
    stopped = true;
    clearTimeout(timer);
    clearTimeout(statusTimeout);
    document.removeEventListener("visibilitychange", onVisibilityChange);
  }

  function arm(session: SessionRemaining, allowExpired = false) {
    if (!session || !Number.isFinite(session.idle_remaining_ms) || !Number.isFinite(session.absolute_remaining_ms)) {
      throw new Error("Invalid session deadline");
    }
    const remaining = Math.min(session.idle_remaining_ms, session.absolute_remaining_ms);
    if (remaining <= 0 && !allowExpired) throw new Error("Session deadline already expired");
    clearTimeout(timer);
    timer = setTimeout(() => { void check(); }, Math.max(0, Math.min(remaining, 2_147_483_647)));
  }

  async function check() {
    if (stopped || checking) return;
    checking = true;
    clearTimeout(timer);
    try {
      const session = await Promise.race([
        checkStatus(),
        new Promise<never>((_, reject) => {
          statusTimeout = setTimeout(() => reject(new Error("Session status request timed out")), sessionStatusTimeoutMS);
        }),
      ]);
      if (!stopped) arm(session);
    } catch (error) {
      if (!stopped) {
        stop();
        onError(error);
      }
    } finally {
      clearTimeout(statusTimeout);
      checking = false;
    }
  }

  function onVisibilityChange() {
    if (document.visibilityState === "visible") void check();
  }

  document.addEventListener("visibilitychange", onVisibilityChange);
  try {
    arm(initial, true);
  } catch (error) {
    stop();
    onError(error);
  }
  return stop;
}
