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
import { PlaybackDatePicker, initialPlaybackDateTimeValue } from "../components/PlaybackDatePicker";
import { SystemTopBar } from "../components/SystemTopBar";
import { readSessionStorage, type AuthState } from "../domain/auth";
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
  H5StreamQuality,
} from "../domain/h5-types";

interface H5MonitorChannelProps {
  externalOrgId: string;
  channelId: number;
  auth?: AuthState | null;
  loggingOut?: boolean;
  authMessage?: string;
  onBack: () => void;
  onAuthRequired?: (error?: unknown) => void;
  onLogout?: () => void | Promise<void>;
}

type Mode = "live" | "playback";
type PlayerDiagnosticEntry = H5PlayerStatus & {
  occurredAt: string;
};

const LONG_PLAY_LIMIT_MS = 15 * 60 * 1000;
const MAX_PLAYER_DIAGNOSTIC_ENTRIES = 24;

export function H5MonitorChannel({
  externalOrgId,
  channelId,
  auth,
  loggingOut = false,
  authMessage = "",
  onBack,
  onAuthRequired,
  onLogout,
}: H5MonitorChannelProps) {
  const [mode, setMode] = useState<Mode>("live");
  const [liveUrl, setLiveUrl] = useState<H5LiveURLResponse | null>(null);
  const [playbackUrl, setPlaybackUrl] = useState<H5PlaybackURLResponse | null>(null);
  const [segments, setSegments] = useState<H5RecordSegmentsResponse | null>(null);
  const [selectedDateTime, setSelectedDateTime] = useState(() => initialPlaybackDateTimeValue());
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
  const [streamQuality, setStreamQuality] = useState<H5StreamQuality>("sd");
  const [diagnosticsOpen, setDiagnosticsOpen] = useState(false);
  const [diagnosticEntries, setDiagnosticEntries] = useState<PlayerDiagnosticEntry[]>([]);
  const [diagnosticCopyNotice, setDiagnosticCopyNotice] = useState("");

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
  const liveRequestSeqRef = useRef(0);
  const playbackRequestSeqRef = useRef(0);
  const pendingLiveReleaseUrlIdRef = useRef<string | null>(null);
  const isAdmin = false;

  const channelTitle = useMemo(() => {
    return readSessionStorage("h5-monitor-active-channel-name") || `通道${channelId}`;
  }, [channelId]);

  const activeUrlId = mode === "live" ? liveUrl?.url_id : playbackUrl?.url_id;
  const currentPlayer = mode === "live" ? liveUrl : playbackUrl;
  const currentPlayerUrl = currentPlayer?.url;
  const selectedDate = selectedDateTime.slice(0, 10);
  const nextQuality: H5StreamQuality = streamQuality === "sd" ? "hd" : "sd";
  const nextQualityLabel = streamQuality === "sd" ? "切为高清" : "切为标清";
  const controlsPinned = loading || playbackState.failed;
  const controlsActuallyVisible = controlsVisible || controlsPinned;
  const controlState: H5PlayerControlState = {
    ...playbackState,
    loading: controlsPinned || playbackState.loading,
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

  function nextLiveRequestSeq() {
    liveRequestSeqRef.current += 1;
    return liveRequestSeqRef.current;
  }

  function invalidateLiveRequest() {
    liveRequestSeqRef.current += 1;
  }

  function isCurrentLiveRequest(seq: number) {
    return liveRequestSeqRef.current === seq;
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

  const recordPlayerStatus = useCallback((status: H5PlayerStatus) => {
    setPlayerStatus(status);
    setDiagnosticEntries((current) => {
      const latest = current[0];
      if (
        latest &&
        latest.stage === status.stage &&
        latest.message === status.message &&
        latest.severity === status.severity &&
        latest.details.join("\n") === status.details.join("\n")
      ) {
        return current;
      }
      return [
        { ...status, occurredAt: new Date().toLocaleString("zh-CN", { hour12: false }) },
        ...current,
      ].slice(0, MAX_PLAYER_DIAGNOSTIC_ENTRIES);
    });
    if (status.stage === "first-frame-ready" || status.stage === "mock-ready") {
      if (resumeCoverTimerRef.current !== null) {
        window.clearTimeout(resumeCoverTimerRef.current);
      }
      resumeCoverTimerRef.current = window.setTimeout(() => {
        setFrozenFrame(null);
        setResumeCoverVisible(false);
        resumeCoverTimerRef.current = null;
      }, 250);
    }
  }, []);

  const setStatusAndRecord = useCallback(
    (status: H5PlayerStatus | null) => {
      if (!status) {
        setPlayerStatus(null);
        return;
      }
      recordPlayerStatus(status);
    },
    [recordPlayerStatus],
  );

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
    const requestSeq = nextLiveRequestSeq();
    const previousLiveUrlId = pendingLiveReleaseUrlIdRef.current || latestUrlIdsRef.current.live;
    setLoading(true);
    setStatusAndRecord({
      stage: "live-url-request",
      message: "正在获取直播播放地址",
      details: [`protocol=${preferredLiveProtocol()}`, `quality=${streamQuality}`, `channel=${channelId}`],
      severity: "info",
    });
    h5Api
      .getLiveUrl(externalOrgId, channelId, userId.current, isAdmin, preferredLiveProtocol(), streamQuality)
      .then(async (resp) => {
        if (!isCurrentLiveRequest(requestSeq)) {
          await releaseUrl(resp.url_id);
          return;
        }
        await releaseUrl(previousLiveUrlId);
        if (pendingLiveReleaseUrlIdRef.current === previousLiveUrlId) {
          pendingLiveReleaseUrlIdRef.current = null;
        }
        setLiveUrl(resp);
        setToast("");
        setStatusAndRecord({
          stage: "live-url-ready",
          message: "直播播放地址已返回，准备初始化播放器",
          details: [`protocol=${resp.protocol || "unknown"}`, `quality=${streamQuality}`, `urlId=${resp.url_id || "-"}`],
          severity: "info",
        });
      })
      .catch((err) => {
        if (!isCurrentLiveRequest(requestSeq)) return;
        if (isUnauthorizedError(err, onAuthRequired)) return;
        const message = errMessage(err, "直播地址获取失败");
        setToast(message);
        setStatusAndRecord({
          stage: "live-url-error",
          message,
          details: [`quality=${streamQuality}`, `channel=${channelId}`],
          severity: "error",
        });
      })
      .finally(() => {
        if (isCurrentLiveRequest(requestSeq)) setLoading(false);
      });

    return () => {
      if (isCurrentLiveRequest(requestSeq)) {
        invalidateLiveRequest();
      }
    };
  }, [mode, liveUrl, externalOrgId, channelId, releaseUrl, streamQuality, setStatusAndRecord, onAuthRequired]);

  useEffect(() => {
    return () => {
      invalidateLiveRequest();
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
      .catch((err) => {
        if (isUnauthorizedError(err, onAuthRequired)) return;
        setToast(errMessage(err, "录像片段查询失败"));
      })
      .finally(() => setLoading(false));
  }

  function playRange(
    startTime: number,
    endTime: number,
    seg: H5RecordSegment | null,
    options: {
      previousUrlId?: string | null;
      preserveCurrentFrame?: boolean;
      reason?: "segment" | "slider" | "resume" | "guard";
    } = {},
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
    setStatusAndRecord({
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
        setStatusAndRecord({
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
        if (isUnauthorizedError(err, onAuthRequired)) {
          return;
        }
        const message = errMessage(err, "回放地址获取失败");
        setToast(message);
        setStatusAndRecord({
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
    setStatusAndRecord(null);
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

  async function handleToggleQuality() {
    const targetQuality = nextQuality;
    const currentUrlId = activeUrlId;
    setStreamQuality(targetQuality);
    setControlsVisible(true);
    if (mode === "live") {
      pendingLiveReleaseUrlIdRef.current = currentUrlId || null;
      setLiveUrl(null);
      setPlaybackState((current) => ({ ...current, playing: false, loading: true }));
      setStatusAndRecord({
        stage: "live-url-request",
        message: "正在切换直播清晰度",
        details: [`quality=${targetQuality}`, `channel=${channelId}`],
        severity: "info",
      });
      return;
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
      setStatusAndRecord({
        stage: "live-url-request",
        message: "正在重新获取直播播放地址",
        details: [`protocol=${preferredLiveProtocol()}`, `quality=${streamQuality}`, `channel=${channelId}`],
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

  async function copyDiagnostics() {
    const text = formatPlayerDiagnosticsForCopy({
      status: playerStatus,
      entries: diagnosticEntries,
      context: {
        externalOrgId,
        channelId,
        mode,
        loading,
        playing: playbackState.playing,
        muted: playbackState.muted,
        failed: playbackState.failed,
        streamQuality,
        urlId: activeUrlId || "-",
        selectedSegment: selectedSegment
          ? `${selectedSegment.start_time}-${selectedSegment.end_time}`
          : "-",
      },
    });
    try {
      await navigator.clipboard.writeText(text);
      setDiagnosticCopyNotice("已复制");
    } catch {
      setDiagnosticCopyNotice("复制失败，请手动选择文本");
    }
    window.setTimeout(() => setDiagnosticCopyNotice(""), 1800);
  }

  return (
    <div className="h5-page h5-channel-page">
      <SystemTopBar
        backAction={{ label: "返回", onClick: onBack }}
        auth={auth}
        loggingOut={loggingOut}
        onLogout={onLogout}
      />
      {authMessage ? <div className="h5-auth-message">{authMessage}</div> : null}
      <main className="h5-viewer">
        <header className="h5-viewer-header">
          <div className="h5-viewer-title">
            <h1>{channelTitle}</h1>
            <span>{mode === "live" ? "实时视频" : "录像回放"}</span>
          </div>
          <button
            type="button"
            className={`h5-diagnostics-btn ${diagnosticsOpen ? "active" : ""}`}
            onClick={() => setDiagnosticsOpen((current) => !current)}
            aria-label={diagnosticsOpen ? "收起播放器日志" : "查看播放器日志"}
            aria-expanded={diagnosticsOpen}
            title="播放器日志"
          >
            <InfoIcon />
          </button>
        </header>

        {diagnosticsOpen && (
          <PlayerDiagnosticsPanel
            status={playerStatus}
            entries={diagnosticEntries}
            loading={loading}
            mode={mode}
            channelId={channelId}
            copyNotice={diagnosticCopyNotice}
            onCopy={copyDiagnostics}
            onClose={() => setDiagnosticsOpen(false)}
          />
        )}

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
                  onStatus={recordPlayerStatus}
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
                {mode === "live" && controlsActuallyVisible && (
                  <button
                    type="button"
                    className="h5-quality-toggle"
                    onClick={handleToggleQuality}
                    disabled={loading || playbackState.failed}
                    aria-label={nextQualityLabel}
                  >
                    {nextQualityLabel}
                  </button>
                )}
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

function PlayerDiagnosticsPanel({
  status,
  entries,
  loading,
  mode,
  channelId,
  copyNotice,
  onCopy,
  onClose,
}: {
  status: H5PlayerStatus | null;
  entries: PlayerDiagnosticEntry[];
  loading: boolean;
  mode: Mode;
  channelId: number;
  copyNotice: string;
  onCopy: () => void;
  onClose: () => void;
}) {
  const fallback: H5PlayerStatus = {
    stage: loading ? "requesting" : "idle",
    message: loading ? "正在准备播放资源" : mode === "live" ? "等待直播播放器状态" : "等待选择录像片段",
    details: [`channel=${channelId}`, `mode=${mode}`],
    severity: "info",
  };
  const current = status || fallback;
  const visibleEntries = entries.length > 0 ? entries : [{ ...fallback, occurredAt: "-" }];

  return (
    <section className={`h5-player-status-panel ${current.severity}`} aria-label="播放器日志">
      <div className="h5-player-status-head">
        <div>
          <strong>播放器日志</strong>
          <span>{current.stage} · {current.message}</span>
        </div>
        <div className="h5-player-status-actions">
          {copyNotice && <span>{copyNotice}</span>}
          <button type="button" onClick={onCopy}>
            复制
          </button>
          <button type="button" onClick={onClose} aria-label="关闭播放器日志">
            关闭
          </button>
        </div>
      </div>
      <div className="h5-player-status-details">
        {current.details.map((item) => (
          <code key={item}>{item}</code>
        ))}
      </div>
      <div className="h5-player-status-list">
        {visibleEntries.map((entry, index) => (
          <div key={`${entry.occurredAt}-${entry.stage}-${index}`}>
            <span>{entry.occurredAt}</span>
            <strong>{entry.stage}</strong>
            <p>{entry.message}</p>
            {entry.details.length > 0 && (
              <div>
                {entry.details.map((item) => (
                  <code key={item}>{item}</code>
                ))}
              </div>
            )}
          </div>
        ))}
      </div>
    </section>
  );
}

function InfoIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 10.8v5.1" />
      <path d="M12 7.6h.01" />
    </svg>
  );
}

function formatPlayerDiagnosticsForCopy({
  status,
  entries,
  context,
}: {
  status: H5PlayerStatus | null;
  entries: PlayerDiagnosticEntry[];
  context: Record<string, string | number | boolean>;
}) {
  const lines = [
    "H5 Monitor 播放器诊断",
    `copiedAt=${new Date().toLocaleString("zh-CN", { hour12: false })}`,
    "",
    "[context]",
    ...Object.entries(context).map(([key, value]) => `${key}=${value}`),
    "",
    "[current]",
    status ? `${status.stage} · ${status.message}` : "empty",
    ...(status?.details ?? []).map((item) => `- ${item}`),
    "",
    "[recent]",
  ];
  for (const entry of entries) {
    lines.push(`${entry.occurredAt} · ${entry.severity} · ${entry.stage} · ${entry.message}`);
    for (const detail of entry.details) {
      lines.push(`  - ${detail}`);
    }
  }
  return lines.join("\n");
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

function formatTime(unix: number): string {
  const d = new Date(unix * 1000);
  return d.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
}

function parseLocalDateTime(value: string): Date | null {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

function timePart(value: string): string {
  const time = value.split("T")[1] || "00:00";
  return /^\d{2}:\d{2}$/.test(time) ? time : "00:00";
}

function clampUnix(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function errMessage(err: unknown, fallback: string): string {
  if (err instanceof H5ApiError) {
    if (err.status === 403) return "暂无访问权限";
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

function isUnauthorizedError(err: unknown, onAuthRequired: ((error?: unknown) => void) | undefined): boolean {
  if (err instanceof H5ApiError && err.status === 401) {
    onAuthRequired?.(err);
    return true;
  }
  return false;
}
