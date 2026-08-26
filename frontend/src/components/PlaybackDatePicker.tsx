import { useEffect, useRef, useState } from "react";

type QuickDateKey = "today" | "yesterday" | "beforeYesterday";
type DateTimeParts = {
  date: string;
  hour: number;
  minute: number;
};

export type PlaybackDatePickerProps = {
  value: string;
  onChange: (dateTime: string) => void;
  onConfirm?: (dateTime: string) => void;
  showTime?: boolean;
  showConfirm?: boolean;
};

export function PlaybackDatePicker({ value, onChange, onConfirm, showTime = true, showConfirm = true }: PlaybackDatePickerProps) {
  const [open, setOpen] = useState(false);
  const pickerRef = useRef<HTMLDivElement | null>(null);
  const selectedParts = parseDateTimeParts(value);
  const [viewMonth, setViewMonth] = useState(() => startOfMonth(dateFromInput(selectedParts.date)));
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
    setViewMonth(startOfMonth(dateFromInput(date)));
    if (!showTime) setOpen(false);
  }

  function selectCalendarDate(date: string) {
    onChange(formatDateTimeValue({ ...selectedParts, date }));
    if (!showTime) setOpen(false);
  }

  function selectTime(part: "hour" | "minute", nextValue: number) {
    onChange(formatDateTimeValue({ ...selectedParts, [part]: nextValue }));
  }

  function commitSelection() {
    setOpen(false);
    onConfirm?.(value);
  }

  const calendarDays = monthCalendarDays(viewMonth);
  const currentMonthLabel = `${viewMonth.getFullYear()}年${viewMonth.getMonth() + 1}月`;

  useEffect(() => {
    setViewMonth(startOfMonth(dateFromInput(selectedParts.date)));
  }, [selectedParts.date]);

  useEffect(() => {
    if (!open) return;
    function handlePointerDown(event: PointerEvent) {
      if (!pickerRef.current?.contains(event.target as Node)) setOpen(false);
    }
    document.addEventListener("pointerdown", handlePointerDown);
    return () => document.removeEventListener("pointerdown", handlePointerDown);
  }, [open]);

  return (
    <div className="h5-date-picker" ref={pickerRef}>
      <div className="h5-date-quick-row">
        {quickDates.map((item) => (
          <button key={item.key} className={activeQuick === item.key ? "active" : ""} onClick={() => selectDate(item.date)} type="button">
            {item.label}
          </button>
        ))}
      </div>
      <div className="h5-date-time-field">
        <button type="button" className={`h5-date-time-trigger ${open ? "is-open" : ""}`} onClick={() => setOpen((current) => !current)} aria-expanded={open}>
          <span>{showTime ? "回放时间" : "回放日期"}</span>
          <strong>{showTime ? formatDateTimeLabel(value) : selectedParts.date.replaceAll("-", "/")}</strong>
        </button>
        {open ? (
          <div className="h5-date-popover" role="dialog" aria-label={showTime ? "选择回放时间" : "选择回放日期"}>
            <div className="h5-date-popover-head">
              <button type="button" onClick={() => setViewMonth((month) => addMonths(month, -1))} aria-label="上个月">‹</button>
              <strong>{currentMonthLabel}</strong>
              <button type="button" onClick={() => setViewMonth((month) => addMonths(month, 1))} aria-label="下个月">›</button>
            </div>
            <div className={`h5-date-popover-body ${showTime ? "" : "is-date-only"}`}>
              <div className="h5-calendar-grid" aria-label="选择日期">
                {["一", "二", "三", "四", "五", "六", "日"].map((weekday) => <span key={weekday} className="h5-calendar-weekday">{weekday}</span>)}
                {calendarDays.map((day) => {
                  const dateText = formatDateInput(day);
                  const inMonth = day.getMonth() === viewMonth.getMonth();
                  const selected = dateText === selectedParts.date;
                  return <button key={dateText} type="button" className={`${inMonth ? "" : "is-muted"} ${selected ? "is-selected" : ""}`} disabled={day > today} onClick={() => selectCalendarDate(dateText)}>{day.getDate()}</button>;
                })}
              </div>
              {showTime ? (
                <div className="h5-time-columns" aria-label="选择时间">
                  <TimeColumn label="时" values={range(0, 23)} active={selectedParts.hour} onSelect={(next) => selectTime("hour", next)} />
                  <TimeColumn label="分" values={range(0, 59)} active={selectedParts.minute} onSelect={(next) => selectTime("minute", next)} />
                </div>
              ) : null}
            </div>
            {showConfirm ? (
              <div className="h5-date-popover-actions">
                <button type="button" onClick={() => setOpen(false)}>取消</button>
                <button type="button" className="primary" onClick={commitSelection}>确定</button>
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
      {showConfirm ? <button type="button" className="h5-date-confirm" onClick={() => onConfirm?.(value)}>定位回放</button> : null}
    </div>
  );
}

function TimeColumn({ label, values, active, onSelect }: { label: string; values: number[]; active: number; onSelect: (value: number) => void }) {
  return <div className="h5-time-column"><span>{label}</span><div>{values.map((value) => <button key={value} type="button" className={value === active ? "is-selected" : ""} onClick={() => onSelect(value)}>{pad2(value)}</button>)}</div></div>;
}

export function initialPlaybackDateTimeValue(now = new Date()): string {
  return `${formatDateInput(now)}T${formatTimeInput(now)}`;
}

function startOfToday(): Date { const date = new Date(); date.setHours(0, 0, 0, 0); return date; }
function addDays(date: Date, days: number): Date { const next = new Date(date); next.setDate(next.getDate() + days); return next; }
function addMonths(date: Date, months: number): Date { return new Date(date.getFullYear(), date.getMonth() + months, 1); }
function startOfMonth(date: Date): Date { return new Date(date.getFullYear(), date.getMonth(), 1); }
function formatDateInput(date: Date): string { return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`; }
function formatTimeInput(date: Date): string { return `${pad2(date.getHours())}:${pad2(date.getMinutes())}`; }
function timePart(value: string): string { const valuePart = value.split("T")[1] || "00:00"; return /^\d{2}:\d{2}$/.test(valuePart) ? valuePart : "00:00"; }
function parseDateTimeParts(value: string): DateTimeParts { const text = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value) ? value : initialPlaybackDateTimeValue(); const [date, time] = text.split("T"); const [hour, minute] = time.split(":").map((item) => Number.parseInt(item, 10)); return { date, hour: clamp(hour, 0, 23), minute: clamp(minute, 0, 59) }; }
function dateFromInput(value: string): Date { const parsed = new Date(`${value}T00:00`); return Number.isNaN(parsed.getTime()) ? new Date() : parsed; }
function formatDateTimeValue(parts: DateTimeParts): string { return `${parts.date}T${pad2(parts.hour)}:${pad2(parts.minute)}`; }
function formatDateTimeLabel(value: string): string { const parts = parseDateTimeParts(value); return `${parts.date.replaceAll("-", "/")} ${pad2(parts.hour)}:${pad2(parts.minute)}`; }
function monthCalendarDays(month: Date): Date[] { const first = startOfMonth(month); const mondayIndex = (first.getDay() + 6) % 7; const start = addDays(first, -mondayIndex); return Array.from({ length: 42 }, (_, index) => addDays(start, index)); }
function range(start: number, end: number): number[] { return Array.from({ length: end - start + 1 }, (_, index) => start + index); }
function clamp(value: number, min: number, max: number): number { return Number.isFinite(value) ? Math.min(max, Math.max(min, value)) : min; }
function pad2(value: number): string { return `${value}`.padStart(2, "0"); }
