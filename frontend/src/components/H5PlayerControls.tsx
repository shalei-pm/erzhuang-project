import type { ReactNode } from "react";

export type H5PlayerControlState = {
  playing: boolean;
  muted: boolean;
  loading: boolean;
  failed: boolean;
  fullscreen: boolean;
  landscape: boolean;
};

export type H5PlayerControlsProps = {
  state: H5PlayerControlState;
  visible: boolean;
  center?: ReactNode;
  onTogglePlay: () => void;
  onToggleSound: () => void;
  onScreenshot: () => void;
  onToggleLandscape: () => void;
  onToggleFullscreen: () => void;
};

export function H5PlayerControls({
  state,
  visible,
  center,
  onTogglePlay,
  onToggleSound,
  onScreenshot,
  onToggleLandscape,
  onToggleFullscreen,
}: H5PlayerControlsProps) {
  const { playing, muted, loading, failed, fullscreen, landscape } = state;
  const pinned = loading || failed;

  return (
    <div
      className={`h5-player-controls ${visible || pinned ? "is-visible" : ""}`}
      onClick={(event) => event.stopPropagation()}
      onPointerDown={(event) => event.stopPropagation()}
    >
      <div className="h5-player-control-group left">
        <button
          type="button"
          onClick={onTogglePlay}
          disabled={loading || failed}
          aria-label={playing ? "暂停" : "播放"}
          aria-pressed={playing}
        >
          {playing ? <PauseIcon /> : <PlayIcon />}
        </button>
        <button
          type="button"
          onClick={onToggleSound}
          disabled={loading || failed}
          aria-label={muted ? "开启声音" : "关闭声音"}
          aria-pressed={!muted}
        >
          {muted ? <VolumeOffIcon /> : <VolumeOnIcon />}
        </button>
      </div>

      <div className="h5-player-control-center">{center || <span>实时视频</span>}</div>

      <div className="h5-player-control-group right">
        <button type="button" onClick={onScreenshot} disabled={loading || failed} aria-label="截图">
          <CameraIcon />
        </button>
        <button
          type="button"
          onClick={onToggleLandscape}
          aria-label={landscape ? "退出横屏" : "横屏查看"}
          aria-pressed={landscape}
        >
          <OrientationToggleIcon />
        </button>
        <button
          type="button"
          onClick={onToggleFullscreen}
          aria-label={fullscreen ? "退出全屏" : "全屏"}
          aria-pressed={fullscreen}
        >
          {fullscreen ? <MinimizeIcon /> : <MaximizeIcon />}
        </button>
      </div>
    </div>
  );
}

function PlayIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M8 5v14l11-7z" />
    </svg>
  );
}

function PauseIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M7 5h4v14H7zM13 5h4v14h-4z" />
    </svg>
  );
}

function VolumeOnIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M4 9v6h4l5 4V5L8 9H4z" />
      <path d="M16 8.5a5 5 0 0 1 0 7" />
      <path d="M18.5 6a8 8 0 0 1 0 12" />
    </svg>
  );
}

function VolumeOffIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M4 9v6h4l5 4V5L8 9H4z" />
      <path d="m17 9 4 4M21 9l-4 4" />
    </svg>
  );
}

function CameraIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M4 8h4l2-3h4l2 3h4v11H4z" />
      <circle cx="12" cy="13.5" r="3.5" />
    </svg>
  );
}

function OrientationToggleIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <rect x="5" y="3.5" width="8" height="13" rx="1.8" />
      <rect x="8" y="8" width="11" height="7.5" rx="1.8" />
    </svg>
  );
}

function MaximizeIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M8 4H4v4M16 4h4v4M4 16v4h4M20 16v4h-4" />
    </svg>
  );
}

function MinimizeIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M8 4v4H4M16 4v4h4M8 20v-4H4M16 20v-4h4" />
    </svg>
  );
}
