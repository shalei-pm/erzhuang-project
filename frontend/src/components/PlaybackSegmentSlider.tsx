import { useEffect, useState } from "react";
import type { H5RecordSegment } from "../domain/h5-types";
import { clampSegmentOffset, segmentDurationSeconds, segmentOffsetToUnix, shouldShowSegmentSlider } from "../domain/h5-playback";

export function PlaybackSegmentSlider({
  segment,
  disabled,
  currentStartTime,
  overlay = false,
  compactControls = false,
  visible = true,
  onCommit,
}: {
  segment: H5RecordSegment;
  disabled: boolean;
  currentStartTime?: number | null;
  overlay?: boolean;
  compactControls?: boolean;
  visible?: boolean;
  onCommit: (startTime: number) => void;
}) {
  const duration = segmentDurationSeconds(segment.start_time, segment.end_time);
  const initialOffset = currentStartTime ? currentStartTime - segment.start_time : 0;
  const [offset, setOffset] = useState(() => clampSegmentOffset(initialOffset, segment.start_time, segment.end_time));
  const [interacting, setInteracting] = useState(false);
  const currentUnix = segmentOffsetToUnix(segment.start_time, segment.end_time, offset);

  useEffect(() => {
    if (interacting) return;
    setOffset(clampSegmentOffset(currentStartTime ? currentStartTime - segment.start_time : 0, segment.start_time, segment.end_time));
  }, [currentStartTime, interacting, segment.start_time, segment.end_time]);

  if (!shouldShowSegmentSlider(segment.start_time, segment.end_time)) {
    return (
      <div
        className={`h5-playback-slider compact ${compactControls ? "is-control-center" : ""} ${overlay ? "is-overlay" : ""} ${visible ? "is-visible" : ""}`}
        onClick={(event) => event.stopPropagation()}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <span>{formatDateTime(currentUnix)}</span>
      </div>
    );
  }

  function commit() {
    setInteracting(false);
    onCommit(currentUnix);
  }

  return (
    <div
      className={`h5-playback-slider ${compactControls ? "is-control-center" : ""} ${overlay ? "is-overlay" : ""} ${visible ? "is-visible" : ""}`}
      aria-label="片段内定位"
      onClick={(event) => event.stopPropagation()}
      onPointerDown={(event) => event.stopPropagation()}
    >
      {!compactControls && (
        <div className="h5-playback-slider-head">
          <span>{formatTime(segment.start_time)}</span>
          <strong>当前：{formatDateTime(currentUnix)}</strong>
          <span>{formatTime(segment.end_time)}</span>
        </div>
      )}
      <input
        type="range"
        min={0}
        max={duration}
        step={1}
        value={offset}
        disabled={disabled}
        onPointerDown={() => setInteracting(true)}
        onFocus={() => setInteracting(true)}
        onChange={(event) => {
          setInteracting(true);
          setOffset(Number(event.target.value));
        }}
        onPointerUp={commit}
        onBlur={commit}
        onKeyUp={(event) => {
          if (event.key === "Enter" || event.key === " ") commit();
        }}
      />
    </div>
  );
}

function formatTime(unix: number): string {
  return new Date(unix * 1000).toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function formatDateTime(unix: number): string {
  return new Date(unix * 1000).toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}
