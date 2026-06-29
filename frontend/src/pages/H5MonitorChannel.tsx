import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { h5Api, H5ApiError } from "../api-h5";
import {
  H5FlvPlayer,
  type H5PlaybackState,
  type H5PlayerHandle,
  type H5PlayerStatus,
} from "../components/H5FlvPlayer";
import { H5PlayerControls, type H5PlayerControlState } from "../components/H5PlayerControls";
import { PlaybackSegmentSlider } from "../components/PlaybackSegmentSlider";
import {
  dataUrlToFile,
  estimatePlaybackUnixAt,
  nextRecordSegmentIndex,
  playbackUnixFromPlayerTime,
  shouldFallbackToInlineFullscreen,
  type PlaybackSession,
} from "../domain/h5-playback";
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
  onSnapshotRefreshed?: () => void;
}

type Mode = "live" | "playback";
type QuickDateKey = "today" | "yesterday" | "beforeYesterday";
type DateTimeParts = {
  date: string;
  hour: number;
  minute: number;
};

const LONG_PLAY_LIMIT_MS = 15 * 60 * 1000;
const SNAPSHOT_REFRESH_COOLDOWN_MS = 10 * 60 * 1000;
const snapshotRefreshCooldown = new Map<string, number>();

export function H5MonitorChannel({ externalOrgId, channelId, onBack, onSnapshotRefreshed }: H5MonitorChannelProps) {
  const [mode, setMode] = useState<Mode>("live");
  const [liveUrl, setLiveUrl] = useState<H5LiveURLResponse | null>(null);
  const [playbackUrl, setPlaybackUrl] = useState<H5PlaybackURLResponse | null>(null);
  const [segments, setSegments] = useState<H5RecordSegmentsResponse | null>(null);
  const [selectedDateTime, setSelectedDateTime] = useState(() => initialDateTimeValue());
  const [selectedSegment, setSelectedSegment] = useState<H5RecordSegment | null>(null);
  const [loading, setLoading] = useState(false);
  const [toast, setToast] = useState("");
  const [playerStatus, setPlayerStatus] = useState<H5PlayerStatus | null>(null);
  const [playbackState, setPlaybackState] = useState<H5PlaybackState>({
    playing: false,
    muted: true,
    loading: false,
    failed: false,
  });
  const [fullscreen, setFullscreen] = useState(false);
  const [landscape, setLandscape] = useState(false);
  const [screenshotNotice, setScreenshotNotice] = useState("");
  const [longPlayPromptOpen, setLongPlayPromptOpen] = useState(false);
  const [guardPausedAt, setGuardPausedAt] = useState<number | null>(null);
  const [controlsVisible, setControlsVisible] = useState(true);
  const [inlineFullscreen, setInlineFullscreen] = useState(false);
  const [frozenFrame, setFrozenFrame] = useState<string | null>(null);
  const [playbackCursorUnix, setPlaybackCursorUnix] = useState<number | null>(null);
  const [resumeCoverVisible, setResumeCoverVisible] = useState(false);

  const userId = useRef(`h5-user-${Date.now()}`);
  const playerRef = useRef<H5PlayerHandle | null>(null);
  const latestUrlIdsRef = useRef<{ live?: string; playback?: string }>({});
  const screenshotNoticeTimerRef = useRef<number | null>(null);
  const longPlayTimerRef = useRef<number | null>(null);
  const resumeCoverTimerRef = useRef<number | null>(null);
  const playbackTickTimerRef = useRef<number | null>(null);
  const playbackSessionRef = useRef<PlaybackSession | null>(null);
  const segmentsRef = useRef<H5RecordSegmentsResponse | null>(null);
  const selectedSegmentRef = useRef<H5RecordSegment | null>(null);
  const playbackUrlRef = useRef<H5PlaybackURLResponse | null>(null);
  const loadingRef = useRef(false);
  const guardPlaybackStartRef = useRef<number | null>(null);
  const playbackRequestSeqRef = useRef(0);
  const refreshedCurrentPlayerRef = useRef("");
  const isAdmin = false;

  const channelTitle = useMemo(() => {
    return sessionStorage.getItem("h5-monitor-active-channel-name") || `通道${channelId}`;
  }, [channelId]);

  const activeUrlId = mode === "live" ? liveUrl?.url_id : playbackUrl?.url_id;
  const currentPlayer = mode === "live" ? liveUrl : playbackUrl;
  const currentPlayerUrl = currentPlayer?.url;
  const selectedDate = selectedDateTime.slice(0, 10);
  const controlState: H5PlayerControlState = {
    ...playbackState,
    loading: loading || playbackState.loading,
    fullscreen: fullscreen || inlineFullscreen,
    landscape: landscape || inlineFullscreen,
  };

  const releaseUrl = useCallback(
    (urlId: string | undefined | null) => {
      if (!urlId) return Promise.resolve();
      return h5Api.disableUrl(externalOrgId, channelId, urlId, userId.current).catch(() => {});
    },
    [channelId, externalOrgId],
  );

  function nextPlaybackRequestSeq() {
    playbackRequestSeqRef.current += 1;
    return playbackRequestSeqRef.current;
  }

  function invalidatePlaybackRequest() {
    playbackRequestSeqRef.current += 1;
  }

  function isCurrentPlaybackRequest(seq: number) {
    return playbackRequestSeqRef.current === seq;
  }

  const handlePlayerError = useCallback((message: string) => {
    setToast(message);
  }, []);

  const handlePlayerStatus = useCallback((status: H5PlayerStatus) => {
    setPlayerStatus(status);
    if (status.stage === "first-frame-ready" || status.stage === "mock-ready") {
      maybeRefreshSnapshotAfterFirstFrame(status.stage);
      if (resumeCoverTimerRef.current !== null) {
        window.clearTimeout(resumeCoverTimerRef.current);
      }
      resumeCoverTimerRef.current = window.setTimeout(() => {
        setFrozenFrame(null);
        setResumeCoverVisible(false);
        resumeCoverTimerRef.current = null;
      }, 250);
    }
  }, [channelId, currentPlayerUrl, externalOrgId, mode, onSnapshotRefreshed]);

  useEffect(() => {
    latestUrlIdsRef.current = {
      live: liveUrl?.url_id,
      playback: playbackUrl?.url_id,
    };
    playbackUrlRef.current = playbackUrl;
  }, [liveUrl?.url_id, playbackUrl?.url_id]);

  useEffect(() => {
    segmentsRef.current = segments;
  }, [segments]);

  useEffect(() => {
    selectedSegmentRef.current = selectedSegment;
  }, [selectedSegment]);

  useEffect(() => {
    loadingRef.current = loading;
  }, [loading]);

  useEffect(() => {
    if (mode !== "live" || liveUrl) return;
    const previousLiveUrlId = latestUrlIdsRef.current.live;
    setLoading(true);
    setPlayerStatus({
      stage: "live-url-request",
      message: "正在获取直播播放地址",
      details: [`protocol=${preferredLiveProtocol()}`, `channel=${channelId}`],
      severity: "info",
    });
    h5Api
      .getLiveUrl(externalOrgId, channelId, userId.current, isAdmin, preferredLiveProtocol())
      .then(async (resp) => {
        await releaseUrl(previousLiveUrlId);
        setLiveUrl(resp);
        setToast("");
        setPlayerStatus({
          stage: "live-url-ready",
          message: "直播播放地址已返回，准备初始化播放器",
          details: [`protocol=${resp.protocol || "unknown"}`, `urlId=${resp.url_id || "-"}`],
          severity: "info",
        });
      })
      .catch((err) => {
        const message = errMessage(err, "直播地址获取失败");
        setToast(message);
        setPlayerStatus({
          stage: "live-url-error",
          message,
          details: [`channel=${channelId}`],
          severity: "error",
        });
      })
      .finally(() => setLoading(false));
  }, [mode, liveUrl, externalOrgId, channelId, releaseUrl]);

  useEffect(() => {
    return () => {
      invalidatePlaybackRequest();
      const { live, playback } = latestUrlIdsRef.current;
      void releaseUrl(live);
      if (playback && playback !== live) {
        void releaseUrl(playback);
      }
      if (screenshotNoticeTimerRef.current !== null) {
        window.clearTimeout(screenshotNoticeTimerRef.current);
      }
      if (longPlayTimerRef.current !== null) {
        window.clearTimeout(longPlayTimerRef.current);
      }
      if (resumeCoverTimerRef.current !== null) {
        window.clearTimeout(resumeCoverTimerRef.current);
      }
      if (playbackTickTimerRef.current !== null) {
        window.clearInterval(playbackTickTimerRef.current);
      }
    };
  }, [releaseUrl]);

  useEffect(() => {
    function handleFullscreenChange() {
      const isFullscreen = Boolean(document.fullscreenElement);
      setFullscreen(isFullscreen);
      if (!isFullscreen) {
        setInlineFullscreen(false);
      }
    }

    document.addEventListener("fullscreenchange", handleFullscreenChange);
    return () => document.removeEventListener("fullscreenchange", handleFullscreenChange);
  }, []);

  useEffect(() => {
    if (!currentPlayerUrl || !playbackState.playing || loading || longPlayPromptOpen) return;

    longPlayTimerRef.current = window.setTimeout(async () => {
      const pausedAt = Date.now();
      setGuardPausedAt(pausedAt);
      guardPlaybackStartRef.current = estimatePlaybackUnixAt(pausedAt, playbackSessionRef.current);
      setLongPlayPromptOpen(true);
      try {
        await playerRef.current?.pause();
      } catch {
        // The prompt still protects URL resources even if the player refuses pause.
      }
      setPlaybackState((current) => ({ ...current, playing: false }));
    }, LONG_PLAY_LIMIT_MS);

    return () => {
      if (longPlayTimerRef.current !== null) {
        window.clearTimeout(longPlayTimerRef.current);
        longPlayTimerRef.current = null;
      }
    };
  }, [currentPlayerUrl, playbackState.playing, loading, longPlayPromptOpen]);

  useEffect(() => {
    if (mode !== "playback" || !playbackState.playing || loading || !selectedSegment || !playbackSessionRef.current) {
      if (playbackTickTimerRef.current !== null) {
        window.clearInterval(playbackTickTimerRef.current);
        playbackTickTimerRef.current = null;
      }
      return;
    }

    playbackTickTimerRef.current = window.setInterval(() => {
      const nowUnix =
        playbackUnixFromPlayerTime(playerRef.current?.getCurrentTime() ?? null, playbackSessionRef.current) ??
        estimatePlaybackUnixAt(Date.now(), playbackSessionRef.current);
      if (nowUnix === null) return;
      setPlaybackCursorUnix(nowUnix);
      const currentSegment = selectedSegmentRef.current;
      if (currentSegment && nowUnix >= currentSegment.end_time - 1) {
        void advanceToNextSegment();
      }
    }, 1000);

    return () => {
      if (playbackTickTimerRef.current !== null) {
        window.clearInterval(playbackTickTimerRef.current);
        playbackTickTimerRef.current = null;
      }
    };
  }, [mode, playbackState.playing, loading, selectedSegment]);

  useEffect(() => {
    setControlsVisible(true);
    refreshedCurrentPlayerRef.current = "";
  }, [currentPlayerUrl, mode, loading, playbackState.failed]);

  function loadSegments(date: string) {
    invalidatePlaybackRequest();
    setSelectedDateTime((prev) => `${date}T${timePart(prev)}`);
    setSelectedSegment(null);
    setPlaybackCursorUnix(null);
    setResumeCoverVisible(false);
    void releaseUrl(playbackUrl?.url_id);
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

  function playRange(
    startTime: number,
    endTime: number,
    seg: H5RecordSegment | null,
    options: { previousUrlId?: string | null; preserveCurrentFrame?: boolean; reason?: "segment" | "slider" | "resume" | "guard" } = {},
  ) {
    const requestSeq = nextPlaybackRequestSeq();
    const previousUrlId = options.previousUrlId === undefined ? playbackUrl?.url_id : options.previousUrlId;
    setSelectedSegment(seg);
    setPlaybackCursorUnix(startTime);
    if (!options.preserveCurrentFrame) {
      setPlaybackUrl(null);
    }
    if (options.preserveCurrentFrame) {
      setResumeCoverVisible(true);
    }
    setLoading(true);
    setPlayerStatus({
      stage: "playback-url-request",
      message: "正在获取回放播放地址",
      details: [`start=${startTime}`, `end=${endTime}`, options.reason ? `reason=${options.reason}` : ""].filter(Boolean),
      severity: "info",
    });
    void (async () => {
      try {
        if (!options.preserveCurrentFrame) {
          void releaseUrl(previousUrlId);
        }
        if (!isCurrentPlaybackRequest(requestSeq)) {
          return;
        }
        const resp = await h5Api.getPlaybackUrl(externalOrgId, channelId, startTime, endTime, userId.current, isAdmin);
        if (!isCurrentPlaybackRequest(requestSeq)) {
          await releaseUrl(resp.url_id);
          return;
        }
        setPlaybackUrl(resp);
        playbackSessionRef.current = { startTime, endTime, startedAtMs: Date.now() };
        if (options.preserveCurrentFrame) {
          void releaseUrl(previousUrlId);
        } else {
          setFrozenFrame(null);
          setResumeCoverVisible(false);
        }
        setToast("");
        setPlayerStatus({
          stage: "playback-url-ready",
          message: "回放播放地址已返回，准备初始化播放器",
          details: [
            `protocol=${resp.protocol || "unknown"}`,
            `urlId=${resp.url_id || "-"}`,
            options.reason === "resume" ? `resumeFrom=${formatTime(startTime)}` : "",
          ].filter(Boolean),
          severity: "info",
        });
      } catch (err) {
        if (!isCurrentPlaybackRequest(requestSeq)) {
          return;
        }
        const message = errMessage(err, "回放地址获取失败");
        setToast(message);
        setPlayerStatus({
          stage: "playback-url-error",
          message,
          details: [`start=${startTime}`, `end=${endTime}`],
          severity: "error",
        });
      } finally {
        if (isCurrentPlaybackRequest(requestSeq)) {
          setLoading(false);
        }
      }
    })();
  }

  function playSegment(seg: H5RecordSegment) {
    setFrozenFrame(null);
    setResumeCoverVisible(false);
    playRange(seg.start_time, seg.end_time, seg, { reason: "segment" });
  }

  async function advanceToNextSegment() {
    const currentSegments = segmentsRef.current;
    const currentSegment = selectedSegmentRef.current;
    const currentPlaybackUrl = playbackUrlRef.current;
    if (!currentSegments || !currentSegment || loadingRef.current) return;
    if (playbackTickTimerRef.current !== null) {
      window.clearInterval(playbackTickTimerRef.current);
      playbackTickTimerRef.current = null;
    }
    const nextIndex = nextRecordSegmentIndex(currentSegments.segments, currentSegment);
    if (nextIndex === null) {
      invalidatePlaybackRequest();
      await releaseUrl(currentPlaybackUrl?.url_id);
      setPlaybackUrl(null);
      setPlaybackState((current) => ({ ...current, playing: false }));
      setPlaybackCursorUnix(null);
      playbackSessionRef.current = null;
      setToast("当前录像片段已播放完毕。");
      return;
    }
    const nextSegment = currentSegments.segments[nextIndex];
    await captureFrozenFrame();
    playRange(nextSegment.start_time, nextSegment.end_time, nextSegment, {
      previousUrlId: currentPlaybackUrl?.url_id,
      preserveCurrentFrame: true,
      reason: "segment",
    });
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
    invalidatePlaybackRequest();
    void releaseUrl(activeUrlId);
    setMode(next);
    setToast("");
    setPlayerStatus(null);
    setLongPlayPromptOpen(false);
    setPlaybackState((current) => ({ ...current, playing: false }));
    if (next === "live") {
      setPlaybackUrl(null);
      playbackSessionRef.current = null;
      setFrozenFrame(null);
      setPlaybackCursorUnix(null);
      setResumeCoverVisible(false);
      setLiveUrl(null);
      return;
    }
    setLiveUrl(null);
    setPlaybackUrl(null);
    setSelectedSegment(null);
    playbackSessionRef.current = null;
    setFrozenFrame(null);
    setPlaybackCursorUnix(null);
    setResumeCoverVisible(false);
    if (!segments) {
      loadSegments(selectedDate);
    }
  }

  async function handleTogglePlay() {
    try {
      if (playbackState.playing) {
        const pausedAtMs = Date.now();
        const pausedAtUnix =
          playbackUnixFromPlayerTime(playerRef.current?.getCurrentTime() ?? null, playbackSessionRef.current) ??
          estimatePlaybackUnixAt(pausedAtMs, playbackSessionRef.current);
        playbackSessionRef.current = playbackSessionRef.current
          ? { ...playbackSessionRef.current, pausedAtMs, pausedAtUnix: pausedAtUnix ?? undefined }
          : null;
        await captureFrozenFrame();
        await playerRef.current?.pause();
        setPlaybackState((current) => ({ ...current, playing: false }));
        return;
      }
      if (mode === "playback" && selectedSegment) {
        const pausedUnix = estimatePlaybackUnixAt(Date.now(), playbackSessionRef.current);
        if (pausedUnix !== null) {
          playRange(pausedUnix, selectedSegment.end_time, selectedSegment, {
            previousUrlId: playbackUrl?.url_id,
            preserveCurrentFrame: true,
            reason: "resume",
          });
          return;
        }
      }
      await playerRef.current?.play();
      setPlaybackState((current) => ({ ...current, playing: true }));
    } catch (err) {
      setToast(`播放控制失败 · ${errMessage(err, "播放器返回异常")}`);
    }
  }

  async function captureFrozenFrame() {
    try {
      const shot = await playerRef.current?.screenshot();
      if (shot?.dataUrl) {
        setFrozenFrame(shot.dataUrl);
      }
    } catch {
      // Screenshot is best-effort here; playback resume still uses the recorded timestamp.
    }
  }

  function maybeRefreshSnapshotAfterFirstFrame(stage: string) {
    if (!currentPlayerUrl) return;
    if (stage === "mock-ready") return;
    const playerKey = `${mode}:${currentPlayerUrl}`;
    if (refreshedCurrentPlayerRef.current === playerKey) return;
    const cooldownKey = `${externalOrgId}:${channelId}`;
    const now = Date.now();
    const refreshedAt = snapshotRefreshCooldown.get(cooldownKey) ?? 0;
    if (now - refreshedAt < SNAPSHOT_REFRESH_COOLDOWN_MS) return;
    refreshedCurrentPlayerRef.current = playerKey;
    snapshotRefreshCooldown.set(cooldownKey, now);
    void h5Api
      .refreshSnapshot(externalOrgId, channelId)
      .then(() => onSnapshotRefreshed?.())
      .catch(() => {
        snapshotRefreshCooldown.delete(cooldownKey);
      });
  }

  async function handleToggleSound() {
    try {
      if (playbackState.muted) {
        await playerRef.current?.openSound();
      } else {
        await playerRef.current?.closeSound();
      }
    } catch (err) {
      setToast(`当前浏览器暂无法切换声音 · ${errMessage(err, "播放器返回异常")}`);
    }
  }

  async function handleScreenshot() {
    try {
      if (!playerRef.current) {
        throw new Error("player handle is not ready");
      }
      const shot = await playerRef.current.screenshot();
      if (shot?.dataUrl) {
        const shared = await shareScreenshot(shot.dataUrl);
        if (!shared) {
          showScreenshotNotice("截图已取消");
          return;
        }
      }
      showScreenshotNotice(shot?.dataUrl ? "截图已准备" : "截图已触发");
    } catch (err) {
      setToast(`当前浏览器暂不支持截图 · ${errMessage(err, "播放器返回异常")}`);
    }
  }

  function handleToggleLandscape() {
    setLandscape((current) => !current);
    setControlsVisible(true);
  }

  function showScreenshotNotice(message: string) {
    setScreenshotNotice(message);
    if (screenshotNoticeTimerRef.current !== null) {
      window.clearTimeout(screenshotNoticeTimerRef.current);
    }
    screenshotNoticeTimerRef.current = window.setTimeout(() => {
      setScreenshotNotice("");
      screenshotNoticeTimerRef.current = null;
    }, 2600);
  }

  async function handleToggleFullscreen() {
    try {
      if (fullscreen || inlineFullscreen) {
        if (inlineFullscreen) {
          setLandscape(false);
        }
        setInlineFullscreen(false);
        await playerRef.current?.exitFullscreen();
        setFullscreen(false);
        return;
      }
      if (shouldFallbackToInlineFullscreen(document, navigator)) {
        setInlineFullscreen(true);
        setLandscape(true);
        setControlsVisible(true);
        return;
      }
      await playerRef.current?.enterFullscreen();
      setFullscreen(true);
    } catch (err) {
      void err;
      setInlineFullscreen(true);
      setLandscape(true);
      setControlsVisible(true);
      setToast("已切换为页面内全屏");
    }
  }

  function handlePlayerSurfaceClick() {
    if (loading || playbackState.failed) {
      setControlsVisible(true);
      return;
    }
    setControlsVisible((current) => !current);
  }

  async function continueAfterLongPlayPrompt() {
    setLongPlayPromptOpen(false);
    if (mode === "live") {
      await releaseUrl(liveUrl?.url_id);
      setPlaybackState((current) => ({ ...current, playing: false, loading: true }));
      setPlayerStatus({
        stage: "live-url-request",
        message: "正在重新获取直播播放地址",
        details: [`protocol=${preferredLiveProtocol()}`, `channel=${channelId}`],
        severity: "info",
      });
      setLiveUrl(null);
      return;
    }
    if (!selectedSegment) {
      setToast("未找到当前回放片段，请重新选择录像片段");
      return;
    }
    const fallbackElapsedSeconds = guardPausedAt ? Math.floor((Date.now() - guardPausedAt) / 1000) : 0;
    const fallbackStart = selectedSegment.start_time + fallbackElapsedSeconds;
    const nextStart = clampUnix(
      guardPlaybackStartRef.current ?? fallbackStart,
      selectedSegment.start_time,
      Math.max(selectedSegment.start_time, selectedSegment.end_time - 1),
    );
    const currentUrlId = playbackUrl?.url_id;
    await releaseUrl(currentUrlId);
    setPlaybackUrl(null);
    playRange(nextStart, selectedSegment.end_time, selectedSegment, { previousUrlId: null, reason: "guard" });
  }

  async function stopAfterLongPlayPrompt() {
    invalidatePlaybackRequest();
    setLongPlayPromptOpen(false);
    await releaseUrl(activeUrlId);
    setPlaybackState((current) => ({ ...current, playing: false }));
    if (mode === "live") {
      setLiveUrl(null);
    } else {
      setPlaybackUrl(null);
      playbackSessionRef.current = null;
      setPlaybackCursorUnix(null);
      setResumeCoverVisible(false);
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
            <div className={`h5-player-shell ${landscape || inlineFullscreen ? "is-landscape" : ""} ${inlineFullscreen ? "is-inline-fullscreen" : ""}`}>
              <div className="h5-player-rotator">
                <H5FlvPlayer
                  ref={playerRef}
                  url={currentPlayerUrl}
                  protocol={currentPlayer?.protocol}
                  isLive={mode === "live"}
                  onError={handlePlayerError}
                  onStatus={handlePlayerStatus}
                  onPlaybackStateChange={setPlaybackState}
                />
                {resumeCoverVisible && (
                  <div className={`h5-frozen-frame ${frozenFrame ? "" : "is-empty"}`} aria-label="正在从暂停位置恢复回放">
                    {frozenFrame ? <img src={frozenFrame} alt="" /> : null}
                    <span>正在从暂停位置恢复...</span>
                  </div>
                )}
                <button
                  type="button"
                  className="h5-player-surface-toggle"
                  onClick={handlePlayerSurfaceClick}
                  aria-label={controlsVisible ? "隐藏播放控件" : "显示播放控件"}
                />
                <H5PlayerControls
                  state={controlState}
                  visible={controlsVisible}
                  center={
                    mode === "playback" && selectedSegment ? (
                      <PlaybackSegmentSlider
                        segment={selectedSegment}
                        disabled={loading}
                        currentStartTime={playbackCursorUnix}
                        compactControls
                        onCommit={(startTime) => {
                          setFrozenFrame(null);
                          setResumeCoverVisible(false);
                          playRange(startTime, selectedSegment.end_time, selectedSegment, { reason: "slider" });
                        }}
                      />
                    ) : null
                  }
                  onTogglePlay={handleTogglePlay}
                  onToggleSound={handleToggleSound}
                  onScreenshot={handleScreenshot}
                  onToggleLandscape={handleToggleLandscape}
                  onToggleFullscreen={handleToggleFullscreen}
                />
                {screenshotNotice && <div className="h5-screenshot-notice">{screenshotNotice}</div>}
              </div>
            </div>
          ) : (
            <div className="h5-player-placeholder">
              <span>{loading ? "加载中..." : mode === "live" ? "正在获取直播画面" : "请选择录像片段"}</span>
            </div>
          )}
        </section>

        <PlayerStatusPanel status={playerStatus} loading={loading} mode={mode} channelId={channelId} />

        {toast && <div className="h5-toast">{toast}</div>}

        {longPlayPromptOpen && (
          <div className="h5-long-play-backdrop" role="dialog" aria-modal="true" aria-label="继续观看确认">
            <div className="h5-long-play-modal">
              <strong>已连续播放较长时间</strong>
              <p>为避免长时间占用播放资源，请确认是否继续观看。</p>
              <div>
                <button type="button" onClick={stopAfterLongPlayPrompt}>
                  停止观看
                </button>
                <button type="button" className="primary" onClick={continueAfterLongPlayPrompt}>
                  继续观看
                </button>
              </div>
            </div>
          </div>
        )}

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

function PlayerStatusPanel({
  status,
  loading,
  mode,
  channelId,
}: {
  status: H5PlayerStatus | null;
  loading: boolean;
  mode: Mode;
  channelId: number;
}) {
  const fallback: H5PlayerStatus = {
    stage: loading ? "requesting" : "idle",
    message: loading ? "正在准备播放资源" : mode === "live" ? "等待直播播放器状态" : "等待选择录像片段",
    details: [`channel=${channelId}`, `mode=${mode}`],
    severity: "info",
  };
  const current = status || fallback;

  return (
    <section className={`h5-player-status-panel ${current.severity}`} aria-label="播放器状态">
      <div>
        <strong>{current.stage}</strong>
        <span>{current.message}</span>
      </div>
      <div className="h5-player-status-details">
        {current.details.map((item) => (
          <code key={item}>{item}</code>
        ))}
      </div>
    </section>
  );
}

function BackIcon() {
  return (
    <svg className="h5-back-icon" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
      <path d="M10 3.5 5.5 8l4.5 4.5" />
    </svg>
  );
}

function preferredLiveProtocol(): "hls" | "flv" {
  return "flv";
}

async function shareScreenshot(dataUrl: string): Promise<boolean> {
  const file = await dataUrlToFile(dataUrl, `monitor-snapshot-${Date.now()}.png`);
  const shareData = {
    files: [file],
    title: "监控截图",
    text: "监控截图",
  };
  const maybeNavigator = navigator as Navigator & {
    canShare?: (data: ShareData) => boolean;
    share?: (data: ShareData) => Promise<void>;
  };
  if (typeof maybeNavigator.share === "function" && maybeNavigator.canShare?.(shareData)) {
    try {
      await maybeNavigator.share(shareData);
      return true;
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") {
        return false;
      }
      throw err;
    }
  }
  downloadDataUrl(dataUrl, file.name);
  return true;
}

function downloadDataUrl(dataUrl: string, filename: string) {
  const anchor = document.createElement("a");
  anchor.href = dataUrl;
  anchor.download = filename;
  anchor.rel = "noopener";
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
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
  }

  function selectCalendarDate(date: string) {
    onChange(formatDateTimeValue({ ...selectedParts, date }));
  }

  function selectTime(part: "hour" | "minute", nextValue: number) {
    onChange(formatDateTimeValue({ ...selectedParts, [part]: nextValue }));
  }

  function commitSelection() {
    setOpen(false);
    onConfirm(value);
  }

  function goMonth(offset: number) {
    setViewMonth(addMonths(viewMonth, offset));
  }

  const calendarDays = monthCalendarDays(viewMonth);
  const currentMonthLabel = `${viewMonth.getFullYear()}年${viewMonth.getMonth() + 1}月`;

  useEffect(() => {
    setViewMonth(startOfMonth(dateFromInput(selectedParts.date)));
  }, [selectedParts.date]);

  useEffect(() => {
    if (!open) return;

    function handlePointerDown(event: PointerEvent) {
      if (!pickerRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    }

    document.addEventListener("pointerdown", handlePointerDown);
    return () => document.removeEventListener("pointerdown", handlePointerDown);
  }, [open]);

  return (
    <div className="h5-date-picker" ref={pickerRef}>
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
      <div className="h5-date-time-field">
        <button
          type="button"
          className={`h5-date-time-trigger ${open ? "is-open" : ""}`}
          onClick={() => setOpen((current) => !current)}
          aria-expanded={open}
        >
          <span>回放时间</span>
          <strong>{formatDateTimeLabel(value)}</strong>
        </button>
        {open && (
          <div className="h5-date-popover" role="dialog" aria-label="选择回放时间">
            <div className="h5-date-popover-head">
              <button type="button" onClick={() => goMonth(-1)} aria-label="上个月">
                ‹
              </button>
              <strong>{currentMonthLabel}</strong>
              <button type="button" onClick={() => goMonth(1)} aria-label="下个月">
                ›
              </button>
            </div>
            <div className="h5-date-popover-body">
              <div className="h5-calendar-grid" aria-label="选择日期">
                {["一", "二", "三", "四", "五", "六", "日"].map((weekday) => (
                  <span key={weekday} className="h5-calendar-weekday">
                    {weekday}
                  </span>
                ))}
                {calendarDays.map((day) => {
                  const dateText = formatDateInput(day);
                  const inMonth = day.getMonth() === viewMonth.getMonth();
                  const selected = dateText === selectedParts.date;
                  const future = day > today;
                  return (
                    <button
                      key={dateText}
                      type="button"
                      className={`${inMonth ? "" : "is-muted"} ${selected ? "is-selected" : ""}`}
                      disabled={future}
                      onClick={() => selectCalendarDate(dateText)}
                    >
                      {day.getDate()}
                    </button>
                  );
                })}
              </div>
              <div className="h5-time-columns" aria-label="选择时间">
                <TimeColumn
                  label="时"
                  values={range(0, 23)}
                  active={selectedParts.hour}
                  onSelect={(next) => selectTime("hour", next)}
                />
                <TimeColumn
                  label="分"
                  values={range(0, 59)}
                  active={selectedParts.minute}
                  onSelect={(next) => selectTime("minute", next)}
                />
              </div>
            </div>
            <div className="h5-date-popover-actions">
              <button type="button" onClick={() => setOpen(false)}>
                取消
              </button>
              <button type="button" className="primary" onClick={commitSelection}>
                确定
              </button>
            </div>
          </div>
        )}
      </div>
      <button type="button" className="h5-date-confirm" onClick={() => onConfirm(value)}>
        定位回放
      </button>
    </div>
  );
}

function TimeColumn({
  label,
  values,
  active,
  onSelect,
}: {
  label: string;
  values: number[];
  active: number;
  onSelect: (value: number) => void;
}) {
  return (
    <div className="h5-time-column">
      <span>{label}</span>
      <div>
        {values.map((value) => (
          <button
            key={value}
            type="button"
            className={value === active ? "is-selected" : ""}
            onClick={() => onSelect(value)}
          >
            {pad2(value)}
          </button>
        ))}
      </div>
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

function addMonths(date: Date, months: number): Date {
  return new Date(date.getFullYear(), date.getMonth() + months, 1);
}

function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function formatDateInput(date: Date): string {
  const year = date.getFullYear();
  const month = pad2(date.getMonth() + 1);
  const day = pad2(date.getDate());
  return `${year}-${month}-${day}`;
}

function formatTimeInput(date: Date): string {
  const hour = pad2(date.getHours());
  const minute = pad2(date.getMinutes());
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

function parseDateTimeParts(value: string): DateTimeParts {
  const fallback = initialDateTimeValue();
  const text = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value) ? value : fallback;
  const [date, time] = text.split("T");
  const [hour, minute] = time.split(":").map((item) => Number.parseInt(item, 10));
  return { date, hour: clamp(hour, 0, 23), minute: clamp(minute, 0, 59) };
}

function dateFromInput(value: string): Date {
  const parsed = new Date(`${value}T00:00`);
  return Number.isNaN(parsed.getTime()) ? new Date() : parsed;
}

function formatDateTimeValue(parts: DateTimeParts): string {
  return `${parts.date}T${pad2(parts.hour)}:${pad2(parts.minute)}`;
}

function formatDateTimeLabel(value: string): string {
  const parts = parseDateTimeParts(value);
  return `${parts.date.replaceAll("-", "/")} ${pad2(parts.hour)}:${pad2(parts.minute)}`;
}

function monthCalendarDays(month: Date): Date[] {
  const first = startOfMonth(month);
  const mondayIndex = (first.getDay() + 6) % 7;
  const start = addDays(first, -mondayIndex);
  return Array.from({ length: 42 }, (_, index) => addDays(start, index));
}

function range(start: number, end: number): number[] {
  return Array.from({ length: end - start + 1 }, (_, index) => start + index);
}

function clamp(value: number, min: number, max: number): number {
  if (!Number.isFinite(value)) return min;
  return Math.min(max, Math.max(min, value));
}

function clampUnix(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function pad2(value: number): string {
  return `${value}`.padStart(2, "0");
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
