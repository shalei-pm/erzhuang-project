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
  onTogglePlay: () => void;
  onToggleSound: () => void;
  onScreenshot: () => void;
  onToggleLandscape: () => void;
  onToggleFullscreen: () => void;
};

export function H5PlayerControls({
  state,
  visible,
  onTogglePlay,
  onToggleSound,
  onScreenshot,
  onToggleLandscape,
  onToggleFullscreen,
}: H5PlayerControlsProps) {
  const { playing, muted, loading, failed, fullscreen, landscape } = state;
  const pinned = loading || failed || !playing;

  return (
    <div
      className={`h5-player-controls ${visible || pinned ? "is-visible" : ""}`}
      onClick={(event) => event.stopPropagation()}
      onPointerDown={(event) => event.stopPropagation()}
    >
      <button
        type="button"
        onClick={onTogglePlay}
        disabled={loading || failed}
        aria-label={playing ? "暂停" : "播放"}
        aria-pressed={playing}
      >
        {playing ? "暂停" : "播放"}
      </button>
      <button
        type="button"
        onClick={onToggleSound}
        disabled={loading || failed}
        aria-label={muted ? "开启声音" : "关闭声音"}
        aria-pressed={!muted}
      >
        {muted ? "开声音" : "静音"}
      </button>
      <button type="button" onClick={onScreenshot} disabled={loading || failed} aria-label="截图">
        截图
      </button>
      <button
        type="button"
        onClick={onToggleLandscape}
        aria-label={landscape ? "退出横屏" : "横屏查看"}
        aria-pressed={landscape}
      >
        {landscape ? "竖屏" : "横屏"}
      </button>
      <button
        type="button"
        onClick={onToggleFullscreen}
        aria-label={fullscreen ? "退出全屏" : "全屏"}
        aria-pressed={fullscreen}
      >
        {fullscreen ? "退出全屏" : "全屏"}
      </button>
    </div>
  );
}
