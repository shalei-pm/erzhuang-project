import { useEffect, useMemo, useRef, useState } from "react";
import { DatePicker } from "antd";
import dayjs, { type Dayjs } from "dayjs";
import { h5Api, H5ApiError } from "../api-h5";
import { H5FlvPlayer } from "../components/H5FlvPlayer";
import type {
  H5LiveURLResponse,
  H5PlaybackURLResponse,
  H5RecordSegment,
  H5RecordSegmentsResponse,
} from "../domain/h5-types";

interface H5MonitorChannelProps {
  externalOrgId: string;
  channelId: number;
  onBack: () => void;
}

type Mode = "live" | "playback";
type QuickDateKey = "today" | "yesterday" | "beforeYesterday";

export function H5MonitorChannel({ externalOrgId, channelId, onBack }: H5MonitorChannelProps) {
  const [mode, setMode] = useState<Mode>("live");
  const [liveUrl, setLiveUrl] = useState<H5LiveURLResponse | null>(null);
  const [playbackUrl, setPlaybackUrl] = useState<H5PlaybackURLResponse | null>(null);
  const [segments, setSegments] = useState<H5RecordSegmentsResponse | null>(null);
  const [selectedDateTime, setSelectedDateTime] = useState(() => initialDateTimeValue());
  const [selectedSegment, setSelectedSegment] = useState<H5RecordSegment | null>(null);
  const [loading, setLoading] = useState(false);
  const [toast, setToast] = useState("");

  const userId = useRef(`h5-user-${Date.now()}`);
  const isAdmin = false;

  const channelTitle = useMemo(() => {
    return sessionStorage.getItem("h5-monitor-active-channel-name") || `通道${channelId}`;
  }, [channelId]);

  const activeUrlId = mode === "live" ? liveUrl?.url_id : playbackUrl?.url_id;
  const currentPlayerUrl = mode === "live" ? liveUrl?.url : playbackUrl?.url;
  const selectedDate = selectedDateTime.slice(0, 10);

  useEffect(() => {
    if (mode !== "live" || liveUrl) return;
    setLoading(true);
    h5Api
      .getLiveUrl(externalOrgId, channelId, userId.current, isAdmin)
      .then((resp) => {
        setLiveUrl(resp);
        setToast("");
      })
      .catch((err) => setToast(errMessage(err, "直播地址获取失败")))
      .finally(() => setLoading(false));
  }, [mode, liveUrl, externalOrgId, channelId]);

  useEffect(() => {
    return () => {
      const urlId = liveUrl?.url_id || playbackUrl?.url_id;
      if (urlId) {
        h5Api.disableUrl(externalOrgId, channelId, urlId, userId.current).catch(() => {});
      }
    };
  }, [channelId, externalOrgId, liveUrl?.url_id, playbackUrl?.url_id]);

  function loadSegments(date: string) {
    setSelectedDateTime((prev) => `${date}T${timePart(prev)}`);
    setSelectedSegment(null);
    setPlaybackUrl(null);
    setLoading(true);
    h5Api
      .getRecordSegments(externalOrgId, channelId, date)
      .then((resp) => {
        setSegments(resp);
        setToast("");
      })
      .catch((err) => setToast(errMessage(err, "录像片段查询失败")))
      .finally(() => setLoading(false));
  }

  function playRange(startTime: number, endTime: number, seg: H5RecordSegment | null) {
    setSelectedSegment(seg);
    setPlaybackUrl(null);
    setLoading(true);
    h5Api
      .getPlaybackUrl(externalOrgId, channelId, startTime, endTime, userId.current, isAdmin)
      .then((resp) => {
        setPlaybackUrl(resp);
        setToast("");
      })
      .catch((err) => setToast(errMessage(err, "回放地址获取失败")))
      .finally(() => setLoading(false));
  }

  function playSegment(seg: H5RecordSegment) {
    playRange(seg.start_time, seg.end_time, seg);
  }

  function handleDateTimeChange(nextValue: string) {
    const nextDate = nextValue.slice(0, 10);
    const prevDate = selectedDateTime.slice(0, 10);
    setSelectedDateTime(nextValue);
    if (nextDate !== prevDate) {
      loadSegments(nextDate);
    }
  }

  function playFromDateTime(value: string) {
    const target = parseLocalDateTime(value);
    if (!target) {
      setToast("回放时间格式不正确");
      return;
    }
    if (!segments || segments.date !== value.slice(0, 10)) {
      setToast("正在查询该日期录像片段，请稍后再试");
      return;
    }
    const targetUnix = Math.floor(target.getTime() / 1000);
    const matched =
      segments.segments.find((seg) => targetUnix >= seg.start_time && targetUnix < seg.end_time) ||
      segments.segments.find((seg) => targetUnix < seg.start_time);
    if (!matched) {
      setToast("该时间点之后暂无可播放录像片段");
      return;
    }
    playRange(Math.max(targetUnix, matched.start_time), matched.end_time, matched);
  }

  function switchMode(next: Mode) {
    if (next === mode) return;
    if (activeUrlId) {
      h5Api.disableUrl(externalOrgId, channelId, activeUrlId, userId.current).catch(() => {});
    }
    setMode(next);
    setToast("");
    if (next === "live") {
      setLiveUrl(null);
      return;
    }
    setPlaybackUrl(null);
    setSelectedSegment(null);
    if (!segments) {
      loadSegments(selectedDate);
    }
  }

  return (
    <div className="h5-page h5-channel-page">
      <main className="h5-viewer">
        <header className="h5-viewer-header">
          <button className="h5-back-btn" onClick={onBack} aria-label="返回">
            <BackIcon />
          </button>
          <div>
            <h1>{channelTitle}</h1>
            <span>{mode === "live" ? "实时视频" : "录像回放"}</span>
          </div>
        </header>

        <section className="h5-viewer-player" aria-label="监控画面">
          {currentPlayerUrl ? (
            <H5FlvPlayer url={currentPlayerUrl} isLive={mode === "live"} onError={setToast} />
          ) : (
            <div className="h5-player-placeholder">
              <span>{loading ? "加载中..." : mode === "live" ? "正在获取直播画面" : "请选择录像片段"}</span>
            </div>
          )}
        </section>

        {toast && <div className="h5-toast">{toast}</div>}

        {mode === "playback" && (
          <section className="h5-playback-panel">
            <PlaybackDatePicker
              value={selectedDateTime}
              onChange={handleDateTimeChange}
              onConfirm={playFromDateTime}
            />

            {segments && segments.segments.length > 0 && (
              <div className="h5-segment-list" aria-label="录像片段">
                {segments.segments.map((seg, i) => (
                  <button
                    key={`${seg.start_time}-${i}`}
                    className={`h5-segment-item ${selectedSegment === seg ? "active" : ""}`}
                    onClick={() => playSegment(seg)}
                  >
                    <span className="h5-segment-time">
                      {formatTime(seg.start_time)} - {formatTime(seg.end_time)}
                    </span>
                    <span className="h5-segment-type">{seg.type_label || "录像"}</span>
                  </button>
                ))}
              </div>
            )}

            {segments && segments.segments.length === 0 && <div className="h5-empty">当天无录像片段。</div>}
          </section>
        )}
      </main>

      <nav className="h5-bottom-tabs" aria-label="播放模式">
        <button className={mode === "live" ? "active" : ""} onClick={() => switchMode("live")}>
          实时视频
        </button>
        <button className={mode === "playback" ? "active" : ""} onClick={() => switchMode("playback")}>
          录像
        </button>
      </nav>
    </div>
  );
}

function BackIcon() {
  return (
    <svg className="h5-back-icon" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
      <path d="M10 3.5 5.5 8l4.5 4.5" />
    </svg>
  );
}

function PlaybackDatePicker({
  value,
  onChange,
  onConfirm,
}: {
  value: string;
  onChange: (dateTime: string) => void;
  onConfirm: (dateTime: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const today = startOfToday();
  const quickDates: Array<{ key: QuickDateKey; label: string; date: string }> = [
    { key: "today", label: "今天", date: formatDateInput(today) },
    { key: "yesterday", label: "昨天", date: formatDateInput(addDays(today, -1)) },
    { key: "beforeYesterday", label: "前天", date: formatDateInput(addDays(today, -2)) },
  ];
  const activeQuick = quickDates.find((item) => item.date === value.slice(0, 10))?.key;

  function selectDate(date: string) {
    const next = `${date}T${timePart(value)}`;
    onChange(next);
  }

  return (
    <div className="h5-date-picker">
      <div className="h5-date-quick-row">
        {quickDates.map((item) => (
          <button
            key={item.key}
            className={activeQuick === item.key ? "active" : ""}
            onClick={() => selectDate(item.date)}
          >
            {item.label}
          </button>
        ))}
      </div>
      <DatePicker
        open={open}
        showTime={{ format: "HH:mm" }}
        format="YYYY-MM-DD HH:mm"
        value={dayjs(value)}
        popupClassName="h5-antd-date-popup"
        className="h5-antd-date-picker"
        placeholder="选择具体日期"
        inputReadOnly
        onOpenChange={setOpen}
        onChange={(next) => {
          if (next) onChange(formatDayjsValue(next));
        }}
        onOk={(next) => {
          const formatted = formatDayjsValue(next);
          onChange(formatted);
          onConfirm(formatted);
          setOpen(false);
        }}
      />
    </div>
  );
}

function formatTime(unix: number): string {
  const d = new Date(unix * 1000);
  return d.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
}

function initialDateTimeValue(): string {
  const now = new Date();
  return `${formatDateInput(now)}T${formatTimeInput(now)}`;
}

function startOfToday(): Date {
  const d = new Date();
  d.setHours(0, 0, 0, 0);
  return d;
}

function addDays(date: Date, days: number): Date {
  const next = new Date(date);
  next.setDate(next.getDate() + days);
  return next;
}

function formatDateInput(date: Date): string {
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, "0");
  const day = `${date.getDate()}`.padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function formatTimeInput(date: Date): string {
  const hour = `${date.getHours()}`.padStart(2, "0");
  const minute = `${date.getMinutes()}`.padStart(2, "0");
  return `${hour}:${minute}`;
}

function timePart(value: string): string {
  const fallback = "00:00";
  const time = value.split("T")[1] || fallback;
  return /^\d{2}:\d{2}$/.test(time) ? time : fallback;
}

function parseLocalDateTime(value: string): Date | null {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

function errMessage(err: unknown, fallback: string): string {
  if (err instanceof H5ApiError) {
    const fieldMsgs = Object.values(err.fields).filter(Boolean);
    return [
      fallback,
      `HTTP ${err.status}`,
      err.code ? `code=${err.code}` : "",
      err.message,
      fieldMsgs.length > 0 ? `fields=${fieldMsgs.join("；")}` : "",
    ]
      .filter(Boolean)
      .join(" · ");
  }
  if (err instanceof Error && err.message.trim()) return `${fallback} · ${err.name}: ${err.message}`;
  return fallback;
}

function formatDayjsValue(value: Dayjs): string {
  return value.format("YYYY-MM-DDTHH:mm");
}
