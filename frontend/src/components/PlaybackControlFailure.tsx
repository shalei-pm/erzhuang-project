import { useState } from "react";
import { CopyOutlined } from "@ant-design/icons";
import type { PlaybackControlDiagnostics } from "../vendor/nvr-player/nvr-player.js";

export function serializePlaybackControlDiagnostic(diagnostic: PlaybackControlDiagnostics) {
  return JSON.stringify({
    schema: 1,
    appVersion: import.meta.env.VITE_APP_VERSION || "local-dev",
    phase: diagnostic.phase,
    requestedRate: diagnostic.requestedRate,
    verifiedRate: diagnostic.verifiedRate,
    protocol: diagnostic.protocol,
    commandSequence: diagnostic.commandSequence,
    ack: diagnostic.ack,
    observedRate: diagnostic.observedRate !== null && Number.isFinite(diagnostic.observedRate)
      ? Math.round(diagnostic.observedRate * 1000) / 1000 : null,
    timeSource: diagnostic.timeSource,
    failure: diagnostic.failure,
  }, null, 2);
}

export function PlaybackControlFailure({ diagnostic }: { diagnostic: PlaybackControlDiagnostics }) {
  const [copyState, setCopyState] = useState<"idle" | "copied" | "manual">("idle");
  const report = serializePlaybackControlDiagnostic(diagnostic);
  async function copyReport() {
    try {
      await navigator.clipboard.writeText(report);
      setCopyState("copied");
    } catch {
      setCopyState("manual");
    }
  }
  return (
    <div className="h5-playback-control-failure" role="status" onClick={(event) => event.stopPropagation()}>
      <span>{diagnostic.phase === "unconfirmed"
        ? diagnostic.requestedRate === 1 ? "正常速度指令已接收，画面速率待确认"
          : `${diagnostic.requestedRate}倍速指令已接收，画面速率待确认，可点击倍速恢复正常`
        : `未能确认${diagnostic.requestedRate}倍速生效，请继续使用正常速度回放`}</span>
      <button type="button" onClick={() => void copyReport()} aria-label="复制倍速诊断信息" title="复制倍速诊断信息">
        <CopyOutlined />
      </button>
      {copyState === "copied" ? <span className="h5-playback-copy-status">诊断信息已复制</span> : null}
      {copyState === "manual" ? <textarea aria-label="倍速诊断信息" readOnly value={report}
        onFocus={(event) => event.currentTarget.select()} /> : null}
    </div>
  );
}
