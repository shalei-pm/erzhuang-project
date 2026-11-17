/* eslint-disable */
/**
 * NVR Player SDK - H.265 视频播放器，支持直播和回放
 *
 * 协议支持：
 *   - 直播：RTP over WebSocket（H.265 FU-A 分片重组）
 *   - 回放：WASM（海康私有格式，URL 含 startTime/endTime 时自动启用）
 * 音频支持：G.711 A-law + Web Audio API（低通 + 高通双滤波器去噪）
 */

// 模块级常量，避免每帧重复分配
import { PcmStreamEngine } from './pcm-stream-engine.js';

const START_CODE = new Uint8Array([0, 0, 0, 1]);
const MAX_ACCESS_UNIT_BYTES = 4 * 1024 * 1024;
const playbackAudioSessions = new Map();
const PLAYBACK_RATES = new Set([1, 2, 4, 8, 16]);
const INITIALIZATION_PTS = -1;
const SPEED_TIMEOUT_MS = 6000;
const SPEED_SETTLE_MS = 500;
const SPEED_WINDOW_MS = 1000;
const SPEED_CADENCE_MS = 50;

function recordingTimestamp(info) {
    const { nYear: year, nMonth: month, nDay: day, nHour: hour,
        nMinute: minute, nSecond: second, nMilliSecond: millis = 0 } = info;
    if (![year, month, day, hour, minute, second, millis].every(Number.isInteger)
        || year < 1970 || year > 2100 || month < 1 || month > 12 || day < 1 || day > 31
        || hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59
        || millis < 0 || millis > 999) return null;
    // Calendar fields belong to the recording. UTC here only makes their delta independent of browser timezone.
    const stamp = Date.UTC(year, month - 1, day, hour, minute, second, millis);
    if (new Date(stamp).getUTCDate() !== day) return null;
    return stamp * 1000;
}

/**
 * Parse an RTP packet without assuming a fixed 12-byte header. Some NVRs add
 * CSRC, header extensions, or padding; treating those bytes as H.265 payload
 * corrupts the decoder input.
 */
export function parseRTPPacket(data) {
    const v = data instanceof Uint8Array ? data : new Uint8Array(data);
    if (v.length < 12 || (v[0] >>> 6) !== 2) return null;

    const hasPadding = (v[0] & 0x20) !== 0;
    const hasExtension = (v[0] & 0x10) !== 0;
    const csrcCount = v[0] & 0x0F;
    let payloadOffset = 12 + csrcCount * 4;
    if (payloadOffset > v.length) return null;

    if (hasExtension) {
        if (payloadOffset + 4 > v.length) return null;
        const extensionWords = (v[payloadOffset + 2] << 8) | v[payloadOffset + 3];
        payloadOffset += 4 + extensionWords * 4;
        if (payloadOffset > v.length) return null;
    }

    let payloadEnd = v.length;
    if (hasPadding) {
        const paddingLength = v[payloadEnd - 1];
        if (paddingLength === 0 || paddingLength > payloadEnd - payloadOffset) return null;
        payloadEnd -= paddingLength;
    }

    return {
        payloadType: v[1] & 0x7F,
        marker: (v[1] & 0x80) !== 0,
        sequenceNumber: (v[2] << 8) | v[3],
        timestamp: (((v[4] << 24) | (v[5] << 16) | (v[6] << 8) | v[7]) >>> 0),
        payload: v.slice(payloadOffset, payloadEnd)
    };
}

class NVRPlayer {
    /**
     * @param {HTMLCanvasElement|string} canvasOrId
     * @param {object} [options]
     * @param {boolean} [options.autoReconnect=true]
     * @param {number}  [options.reconnectDelay=3000]
     * @param {function} [options.onError]
     * @param {function} [options.onConnected]
     * @param {function} [options.onDisconnected]
     * @param {function} [options.onRecovered]
     * @param {function} [options.onFirstFrame]
     * @param {function} [options.onPlaybackTime] Presented relative media seconds (WASM playback only)
     * @param {boolean} [options.forceWasm=false] 强制使用 WASM 回放模式
     */
    constructor(canvasOrId, options = {}) {
        if (typeof canvasOrId === 'string') {
            this.canvas = document.getElementById(canvasOrId);
            if (!this.canvas) throw new Error(`NVRPlayer: 未找到 id="${canvasOrId}" 的 canvas`);
        } else if (canvasOrId instanceof HTMLCanvasElement) {
            this.canvas = canvasOrId;
        } else {
            throw new Error('NVRPlayer: 需传入 canvas 元素或 canvas ID 字符串');
        }
        this.ctx = this.canvas.getContext('2d');

        this.onError        = options.onError        || (() => {});
        this.onConnected    = options.onConnected    || (() => {});
        this.onDisconnected = options.onDisconnected || (() => {});
        this.onRecovered    = options.onRecovered    || (() => {});
        this.onFirstFrame   = options.onFirstFrame   || (() => {});
        this.onDiagnostics  = options.onDiagnostics  || (() => {});
        this.onPlaybackTime = options.onPlaybackTime || (() => {});
        this.onPlaybackControlDiagnostics = options.onPlaybackControlDiagnostics || (() => {});
        this._resetPlaybackControlDiagnostics(false);

        this.autoReconnect  = false;
        this.reconnectDelay = options.reconnectDelay || 3000;
        this.forceWasm      = options.forceWasm === true;
        this.wasmWorkerUrl  = options.wasmWorkerUrl || '/erzhuang-project/nvr-player/wasm/systemTransform-worker.js';
        this.audioWorkletUrl = options.audioWorkletUrl || '/erzhuang-project/nvr-player/pcm-stream-worklet.js';
        this._isRunning     = false;
        this._isPaused      = false;
        this._playbackGeneration = 0;
        this._playbackBasePTS = null;
        this._playbackLastPTS = null;
        this._playbackTime = null;
        this._playbackClockSource = null;
        this._playbackFrameSequence = 0;
        this._playbackFrameTimes = new Map();
        this._lastPresentedAt = null;
        this._speedRequiresSequence = false;
        this._pendingSpeed = null;
        this._speedAudioMuted = false;

        // WebSocket
        this.ws       = null;
        this.wsUrl    = null;
        this.sequence = 0;

        // 解码器
        this.decoder           = null;
        this.decoderConfigured = false;
        this.waitingForKeyFrame = true;
        this.needKeyFrame      = false;
        this.frameIndex        = 0;
        this.decodeErrorCount  = 0;
        this.lastOutputTime    = Date.now();
        this._hasRenderedFirstFrame = false;

        // H.265 RTP 相关
        this.fuBuffer          = null;
        this.accessUnit         = null;
        this.h265Params        = null;    // Map<nalType, Uint8Array>，使用 Map 防止重复累积
        this.h265ParamSets     = null;
        this.VIDEO_PAYLOAD_TYPE = 96;
        this.AUDIO_PAYLOAD_TYPE = 8;
        this.baseRtpTimestamp  = null;

        // H.264 WASM 相关
        this.h264DecoderConfigured = false;

        // 音频
        this.audioContext       = null;
        this.nextAudioTime      = 0;
        this.lastAudioSeq       = null;
        this.audioPacketLoss    = 0;
        this.currentVolume      = 100;
        this.gainNode           = null;
        this.highpassNode       = null;
        this.lowpassNode        = null;
        this.workletNode        = null;
        this._workletInitPromise = null;
        this._graphContext      = null;
        this._pcmEngine         = null;
        this._audioGestureReady = false;  // 用户手势触发后置为 true
        this._audioGestureHandler = null; // 手势监听引用，用于清理
        this._scheduledAudioSources = new Set();

        // WASM
        this.useWasm         = false;
        this.transformWorker = null;
        this.wasmRuntimeReady = false;
        this.wasmReady       = false;
        this.wasmHeaderSent  = false;
        this.wasmPendingHeader = null;
        this.diagnostics = null;
        this._lastDiagnosticsAt = 0;
        this._resetDiagnostics();

        this._monitorTimer   = null;
        this._reconnectTimer = null;
        this._startDecoderMonitor();
    }

    // ─── 解码器生命周期 ────────────────────────────────────────────────────────

    /**
     * 创建（或重建）VideoDecoder。
     * 所有新建解码器的入口统一走此方法，保证错误回调一致。
     */
    _createDecoder() {
        if (this.decoder) {
            try {
                if (this.decoder.state !== 'closed') this.decoder.close();
            } catch (_) {}
            this.decoder = null;
        }

        if (!('VideoDecoder' in window)) {
            const err = new Error('浏览器不支持 WebCodecs API');
            this.onError(err);
            throw err;
        }

        const generation = this._playbackGeneration;
        this.decoder = new VideoDecoder({
            output: (frame) => {
                if (generation !== this._playbackGeneration) { frame.close(); return; }
                this._renderFrame(frame);
            },
            error: (e) => {
                this._recordDecoderError(e);
                this._recoverDecoder();
            }
        });
    }

    /**
     * 重置所有解码相关的状态变量（不包含 decoder 对象本身）。
     * 重连、流重启、解码器错误时均调用此方法。
     */
    _resetDecoderState({ preserveParams = false } = {}) {
        this.decoderConfigured  = false;
        this.waitingForKeyFrame = true;
        this.needKeyFrame       = false;
        this.fuBuffer           = null;
        this.accessUnit         = null;
        if (!preserveParams) {
            this.h265Params     = null;
            this.h265ParamSets  = null;
        }
        this.baseRtpTimestamp   = null;
        this.frameIndex         = 0;
    }

    _recordDecoderError(error) {
        this.decodeErrorCount++;
        this.diagnostics.decoderErrors++;
        const message = error instanceof Error ? error.message : String(error || "unknown decoder error");
        this.diagnostics.lastDecoderError = message.slice(0, 160);
        this._publishDiagnostics(true);
    }

    /** Recover from a bad encoded frame without requiring a fresh stream URL. */
    _recoverDecoder() {
        if (!this._isRunning) return;
        this._resetDecoderState({ preserveParams: true });
        try {
            this._createDecoder();
            if (this.h265Params?.size >= 3) this._configureDecoderWithParams(this.h265Params);
        } catch (_) {
            // _createDecoder already reported unsupported browser failures.
        }
    }

    /**
     * 收到 VPS/SPS/PPS 后配置解码器。
     * 若解码器已关闭则先重建，configure 本身失败也会重建再试一次。
     */
    _configureDecoderWithParams(params) {
        const vps = params.get(32);
        const sps = params.get(33);
        const pps = params.get(34);
        if (!vps || !sps || !pps) return;

        this.h265ParamSets = { vps, sps, pps };

        // 确保解码器存在且未关闭
        if (!this.decoder || this.decoder.state === 'closed') {
            this._createDecoder();
        }

        const doConfigure = () => {
            this.decoder.configure({ codec: 'hev1.1.6.L93.B0', optimizeForLatency: true });
            this.decoderConfigured = true;
        };

        try {
            doConfigure();
        } catch (e) {
            // configure 失败时重建解码器再试一次
            try {
                this._createDecoder();
                doConfigure();
            } catch (e2) {
                this.onError(e2);
            }
        }
    }

    /**
     * 解码队列积压监控：超过 3s 无输出则 flush 解除卡死。
     */
    _startDecoderMonitor() {
        this._monitorTimer = setInterval(() => {
            if (!this._isRunning) return;
            if (this.decoder?.state === 'configured' && this.decoder.decodeQueueSize > 0) {
                if (Date.now() - this.lastOutputTime > 3000) {
                    this.decoder.flush().catch((error) => {
                        this._recordDecoderError(error);
                        this._recoverDecoder();
                    });
                    this.lastOutputTime = Date.now();
                }
            }
        }, 2000);
    }

    _resetDiagnostics() {
        this.diagnostics = {
            receivedPackets: 0,
            rtpPackets: 0,
            videoPayloadPackets: 0,
            audioPayloadPackets: 0,
            otherPayloadPackets: 0,
            vpsPackets: 0,
            spsPackets: 0,
            ppsPackets: 0,
            keyFrameNALUnits: 0,
            wasmRuntimeReady: false,
            wasmReady: false,
            wasmOutputInit: 0,
            wasmOutputFrames: 0,
            decoderInputFrames: 0,
            renderedFrames: 0,
            markerPackets: 0,
            accessUnits: 0,
            multiNALAccessUnits: 0,
            droppedAccessUnits: 0,
            malformedFuPackets: 0,
            decoderErrors: 0,
            lastDecoderError: "",
            closeCode: null
        };
        this._lastDiagnosticsAt = 0;
        this._publishDiagnostics(true);
    }

    _publishDiagnostics(force = false) {
        const now = Date.now();
        if (!force && now - this._lastDiagnosticsAt < 250) return;
        this._lastDiagnosticsAt = now;
        this.onDiagnostics({ ...this.diagnostics });
    }

    // ─── WASM 解码器（回放模式）────────────────────────────────────────────────

    async _createTransformWorker() {
        return new Worker(this.wasmWorkerUrl);
    }

    async _initWasmDecoder() {
        const generation = this._playbackGeneration;
        if (this.transformWorker) {
            this.transformWorker.terminate();
            this.transformWorker = null;
        }
        this.h264DecoderConfigured = false;
        this.wasmReady      = false;
        this.wasmRuntimeReady = false;
        this.wasmHeaderSent = false;
        this.wasmPendingHeader = null;

        const worker = await this._createTransformWorker();
        if (!this._isRunning || generation !== this._playbackGeneration) {
            worker.terminate();
            return;
        }
        this.transformWorker = worker;
        this.transformWorker.onmessage = ({ data: e }) => {
            if (!this._isRunning || generation !== this._playbackGeneration) return;
            if (e.type === 'loaded') {
                this.wasmRuntimeReady = true;
                this.diagnostics.wasmRuntimeReady = true;
                this._publishDiagnostics(true);
                this._sendPendingWasmHeader();
            } else if (e.type === 'created') {
                this.wasmReady = true;
                this.diagnostics.wasmReady = true;
                this._publishDiagnostics(true);
            } else if (e.type === 'outputData') {
                if (e.dType === 1) {
                    this.diagnostics.wasmOutputInit++;
                    this._publishDiagnostics();
                    this._handleH264Init(e.buf);
                } else if (e.dType === 2) {
                    this.diagnostics.wasmOutputFrames++;
                    this._publishDiagnostics();
                    this._handleH264Frame(e.buf, e.frameInfo);
                }
            }
        };
        this.transformWorker.onerror = (e) => {
            this.onError(new Error(`[NVRPlayer] WASM Worker 错误: ${e.message}`));
        };
    }

    _handleH264Init(buf) {
        const nalUnits = this._parseAVCCNALs(new Uint8Array(buf));

        // 检测是H.264还是H.265
        let isH265 = false;
        for (const nal of nalUnits) {
            const h265Type = (nal[0] >> 1) & 0x3F;
            if (h265Type === 32 || h265Type === 33 || h265Type === 34) {
                isH265 = true;
                break;
            }
        }

        if (isH265) {
            this._handleH265Init(nalUnits);
        } else {
            this._handleH264InitInternal(nalUnits);
        }
    }

    _handleH265Init(nalUnits) {
        let vps, sps, pps, idrFrame;
        for (const nal of nalUnits) {
            const nalType = (nal[0] >> 1) & 0x3F;
            if (nalType === 32) vps = nal;
            else if (nalType === 33) sps = nal;
            else if (nalType === 34) pps = nal;
            else if (nalType >= 16 && nalType <= 21) idrFrame = nal;
        }

        if (vps && sps && pps && !this.h264DecoderConfigured) {
            this.h265ParamSets = { vps, sps, pps };
            const generation = this._playbackGeneration;
            this.decoder = new VideoDecoder({
                output: (frame) => {
                    if (generation !== this._playbackGeneration) { frame.close(); return; }
                    this._renderFrame(frame);
                },
                error: (e) => this.onError(e)
            });

            this.decoder.configure({
                codec: 'hev1.1.6.L93.B0',
                optimizeForLatency: true
            });

            this.h264DecoderConfigured = true;

            if (idrFrame) {
                const startCode = new Uint8Array([0, 0, 0, 1]);
                const totalLen = startCode.length * 4 + vps.length + sps.length + pps.length + idrFrame.length;
                const frameData = new Uint8Array(totalLen);
                let offset = 0;
                [vps, sps, pps, idrFrame].forEach(data => {
                    frameData.set(startCode, offset);
                    offset += 4;
                    frameData.set(data, offset);
                    offset += data.length;
                });

                try {
                    this.decoder.decode(new EncodedVideoChunk({
                        type: 'key',
                        timestamp: INITIALIZATION_PTS,
                        data: frameData
                    }));
                } catch (e) {}
            }
        }
    }

    _handleH264InitInternal(nalUnits) {
        let sps, pps, idrFrame;
        for (const nal of nalUnits) {
            const t = nal[0] & 0x1F;
            if (t === 7) sps = nal;
            else if (t === 8) pps = nal;
            else if (t === 5) idrFrame = nal;
        }
        if (!sps || !pps) return;

        try {
            this._createDecoder();
            this.decoder.configure({
                codec: 'avc1.640029',
                description: this._buildAvcCDescription(sps, pps),
                optimizeForLatency: true
            });
            this.h264DecoderConfigured = true;
        } catch (e) {
            this.onError(e);
            return;
        }

        if (idrFrame) {
            const len  = idrFrame.length;
            const data = new Uint8Array(4 + len);
            data[0] = (len >> 24) & 0xFF; data[1] = (len >> 16) & 0xFF;
            data[2] = (len >> 8) & 0xFF;  data[3] = len & 0xFF;
            data.set(idrFrame, 4);
            this._safeDecode('key', INITIALIZATION_PTS, data);
        }
    }

    _handleH264Frame(buf, frameInfo) {
        if (frameInfo.nFrameType === 4) {
            this._playAudioPCM(this._decodePCMA(new Uint8Array(buf)));
            return;
        }
        if (!this.h264DecoderConfigured || !this.decoder || this.decoder.state !== 'configured') return;

        // Preserve recording time with the exact decoded frame, even if container PTS repeats or resets.
        const timestamp = ++this._playbackFrameSequence;
        this._playbackFrameTimes.set(timestamp, {
            container: frameInfo.nTimeStamp * 1000,
            recorded: recordingTimestamp(frameInfo)
        });

        const data     = new Uint8Array(buf);
        const isKeyFrame    = frameInfo.nFrameType === 3;

        // H.265: AVCC转Annex-B
        if (this.h265ParamSets) {
            const nalUnits = this._parseAVCCNALs(data);
            const startCode = new Uint8Array([0, 0, 0, 1]);
            let frameData;

            if (isKeyFrame) {
                const { vps, sps, pps } = this.h265ParamSets;
                const totalLen = startCode.length * (nalUnits.length + 3) + vps.length + sps.length + pps.length + nalUnits.reduce((s, n) => s + n.length, 0);
                frameData = new Uint8Array(totalLen);
                let offset = 0;
                [vps, sps, pps, ...nalUnits].forEach(nal => {
                    frameData.set(startCode, offset);
                    offset += 4;
                    frameData.set(nal, offset);
                    offset += nal.length;
                });
            } else {
                const totalLen = startCode.length * nalUnits.length + nalUnits.reduce((s, n) => s + n.length, 0);
                frameData = new Uint8Array(totalLen);
                let offset = 0;
                for (const nal of nalUnits) {
                    frameData.set(startCode, offset);
                    offset += 4;
                    frameData.set(nal, offset);
                    offset += nal.length;
                }
            }

            try {
                this.diagnostics.decoderInputFrames++;
                this._publishDiagnostics();
                this.decoder.decode(new EncodedVideoChunk({
                    type: isKeyFrame ? 'key' : 'delta',
                    timestamp,
                    data: frameData
                }));
            } catch (e) {}
            return;
        }

        // H.264: 保持原有AVCC逻辑
        let frameData  = data;

        if (isKeyFrame) {
            const idrNals = this._parseAVCCNALs(data).filter(n => (n[0] & 0x1F) === 5);
            if (idrNals.length > 0) {
                const total = idrNals.reduce((s, n) => s + 4 + n.length, 0);
                frameData   = new Uint8Array(total);
                let offset  = 0;
                for (const nal of idrNals) {
                    const l = nal.length;
                    frameData[offset++] = (l >> 24) & 0xFF; frameData[offset++] = (l >> 16) & 0xFF;
                    frameData[offset++] = (l >> 8) & 0xFF;  frameData[offset++] = l & 0xFF;
                    frameData.set(nal, offset); offset += l;
                }
            }
        }
        this.diagnostics.decoderInputFrames++;
        this._publishDiagnostics();
        this._safeDecode(isKeyFrame ? 'key' : 'delta', timestamp, frameData);
    }

    // ─── 渲染 ─────────────────────────────────────────────────────────────────

    _renderFrame(frame) {
        try {
            if (!this._isRunning || this._isPaused) return;
            let pts = frame.timestamp;
            if (this.useWasm && this._playbackFrameSequence > 0) {
                const timing = this._playbackFrameTimes.get(pts);
                this._playbackFrameTimes.delete(pts);
                // Retire old metadata only after output advances, never before queued frames are presented.
                while (this._playbackFrameTimes.size && this._playbackFrameTimes.keys().next().value < pts - 256) {
                    this._playbackFrameTimes.delete(this._playbackFrameTimes.keys().next().value);
                }
                if (!timing) return;
                if (this._playbackClockSource === null) {
                    this._playbackClockSource = timing.recorded !== null ? 'recorded' : 'container';
                    this._publishPlaybackControlDiagnostics({ timeSource: this._playbackClockSource });
                }
                pts = timing[this._playbackClockSource];
            }
            // Initialization IDRs use a sentinel; real relative media PTS may be zero.
            const recorded = this._playbackClockSource === 'recorded';
            if (this.useWasm && pts !== null && (!Number.isSafeInteger(pts) || pts < 0
                || (this._playbackLastPTS != null && (recorded ? pts < this._playbackLastPTS : pts <= this._playbackLastPTS)))) return;
            this.lastOutputTime = Date.now();
            // 自动对齐 canvas 绘图缓冲区到视频帧实际分辨率，避免 CSS 拉伸模糊
            if (this.canvas.width !== frame.displayWidth || this.canvas.height !== frame.displayHeight) {
                this.canvas.width  = frame.displayWidth;
                this.canvas.height = frame.displayHeight;
            }
            this.ctx.drawImage(frame, 0, 0);
            this._lastPresentedAt = Date.now();
            if (this.useWasm && pts !== null) {
                if (this._playbackBasePTS == null) this._playbackBasePTS = pts;
                this._playbackLastPTS = pts;
                this._playbackTime = (pts - this._playbackBasePTS) / 1000000;
                this._observeSpeed(pts);
                this.onPlaybackTime(this._playbackTime);
            }
            this.diagnostics.renderedFrames++;
            this._publishDiagnostics();
            if (!this._hasRenderedFirstFrame) {
                this._hasRenderedFirstFrame = true;
                this.onFirstFrame();
            }
        } finally {
            frame.close();
        }
    }

    _resetPlaybackControlDiagnostics(publish = true) {
        this._playbackControlDiagnostics = {
            phase: 'idle', requestedRate: 1, verifiedRate: 1, protocol: 'json',
            commandSequence: null, ack: 'none', observedRate: null, failure: null, timeSource: 'container'
        };
        if (publish) this._publishPlaybackControlDiagnostics({});
    }

    getPlaybackControlDiagnostics() {
        const snapshot = this._playbackControlDiagnostics;
        return {
            phase: snapshot.phase,
            requestedRate: snapshot.requestedRate,
            verifiedRate: snapshot.verifiedRate,
            protocol: snapshot.protocol,
            commandSequence: snapshot.commandSequence,
            ack: snapshot.ack,
            observedRate: snapshot.observedRate,
            failure: snapshot.failure,
            timeSource: snapshot.timeSource
        };
    }

    _publishPlaybackControlDiagnostics(changes) {
        Object.assign(this._playbackControlDiagnostics, changes);
        // Observer failures must not alter playback or expose transport contents.
        try { this.onPlaybackControlDiagnostics(this.getPlaybackControlDiagnostics()); } catch (_) {}
    }

    _recordPlaybackControlRequest(rate, phase, failure = null, commandSequence = null) {
        this._publishPlaybackControlDiagnostics({
            phase,
            requestedRate: Number.isFinite(rate) ? rate : 1,
            protocol: 'json',
            commandSequence,
            ack: 'none', observedRate: null, failure
        });
    }

    _observeSpeed(pts) {
        const pending = this._pendingSpeed;
        if (!pending) return;
        if (this.ws?.readyState !== WebSocket.OPEN) {
            this._finishSpeed(new Error('Playback connection closed'), 'connection');
            return;
        }
        const now = performance.now();
        if (now - pending.startedAt >= SPEED_TIMEOUT_MS) {
            this._expireSpeed();
            return;
        }
        if (now - pending.startedAt < SPEED_SETTLE_MS) return;
        if (!pending.window) {
            pending.window = {
                pts, at: now, lastAt: now, frames: 0, maxGap: 0,
                cadencePTS: pts, cadenceAt: now, cadences: 0, matchingCadences: 0
            };
            return;
        }
        const window = pending.window;
        if (now <= window.lastAt) return;
        window.frames++;
        window.maxGap = Math.max(window.maxGap, now - window.lastAt);
        window.lastAt = now;
        const cadenceElapsed = now - window.cadenceAt;
        if (cadenceElapsed >= SPEED_CADENCE_MS) {
            const cadenceRate = (pts - window.cadencePTS) / (cadenceElapsed * 1000);
            window.cadences++;
            if (Math.abs(cadenceRate - pending.rate) <= pending.rate * 0.35) window.matchingCadences++;
            window.cadencePTS = pts;
            window.cadenceAt = now;
        }
        const elapsed = now - window.at;
        const recorded = this._playbackClockSource === 'recorded';
        if (elapsed < (recorded ? 2000 : SPEED_WINDOW_MS)) return;
        const observed = (pts - window.pts) / (elapsed * 1000);
        // A matching average alone can hide periodic seek-like PTS jumps.
        // Require most short cadences to agree too; irregular sources fail closed.
        const cadenceMatches = recorded || (window.cadences >= 4 && window.matchingCadences / window.cadences >= 0.7);
        const matches = window.frames >= 4 && window.maxGap <= 500 && cadenceMatches
            && Math.abs(observed - pending.rate) <= pending.rate * (recorded ? 0.3 : 0.2);
        pending.matches = matches ? pending.matches + 1 : 0;
        const prior = this._playbackControlDiagnostics.observedRate;
        const mismatch = recorded && observed > 0 && window.frames >= 4 && window.maxGap <= 500
            && Math.abs(observed - pending.rate) > pending.rate * 0.3;
        pending.mismatches = mismatch
            ? (prior !== null && Math.abs(observed - prior) <= observed * 0.25 ? pending.mismatches + 1 : 1) : 0;
        pending.window = {
            pts, at: now, lastAt: now, frames: 0, maxGap: 0,
            cadencePTS: pts, cadenceAt: now, cadences: 0, matchingCadences: 0
        };
        this._publishPlaybackControlDiagnostics({
            phase: 'verifying', observedRate: Number.isFinite(observed) ? observed : null
        });
        if (pending.matches >= 2) this._finishSpeed();
    }

    _expireSpeed() {
        const pending = this._pendingSpeed;
        if (!pending) return;
        if (this.ws?.readyState !== WebSocket.OPEN) {
            this._finishSpeed(new Error('Playback connection closed'), 'connection');
            return;
        }
        const diagnostic = this._playbackControlDiagnostics;
        // A proxy ACK and continued pictures are not a failed command just because mux PTS is retimed.
        const inconclusive = diagnostic.timeSource === 'container' || pending.mismatches < 2;
        if (diagnostic.ack === 'accepted' && inconclusive
            && this.diagnostics.renderedFrames > pending.startedFrames + 3
            && this._lastPresentedAt !== null && Date.now() - this._lastPresentedAt < 1500) {
            this._finishSpeed(null, null, false);
        } else {
            this._finishSpeed(new Error('Playback speed verification timed out'), 'timeout');
        }
    }

    _finishSpeed(error, failure = null, verified = true) {
        const pending = this._pendingSpeed;
        if (!pending) return;
        this._pendingSpeed = null;
        clearTimeout(pending.timer);
        if (error) {
            this._publishPlaybackControlDiagnostics({ phase: 'failed', failure });
            // An unconfirmed upstream state stays silent until the host rebuilds the session.
            pending.reject(error);
            return;
        }
        this._speedAudioMuted = pending.rate !== 1;
        this._resetAudioStream();
        this._setSmoothGain(this.currentVolume / 100, true);
        if (!this._speedAudioMuted && this.currentVolume > 0 && this.audioContext && this._audioGestureReady) {
            this._claimPlaybackAudioSession();
        }
        this._publishPlaybackControlDiagnostics({
            phase: verified ? 'verified' : 'unconfirmed', failure: null,
            ...(verified ? { verifiedRate: pending.rate } : {})
        });
        pending.resolve();
    }

    _handleControlMessage(data) {
        let message;
        try { message = JSON.parse(data); } catch (_) { return; }
        if (!message || typeof message !== 'object' || Array.isArray(message)) return;
        // The company proxy uses JSON controls; version metadata is not negotiation.
        const pending = this._pendingSpeed;
        if (!pending) return;
        // After preemption, a status without a sequence cannot establish the new rate.
        // Ask the host for a fresh normal-speed session rather than wait or trust an old ACK.
        if (this._speedRequiresSequence && message.sequence === undefined) {
            if ((message.cmd === undefined || message.cmd === 'speed')
                && (message.status !== undefined || message.res !== undefined || message.statusString !== undefined)) {
                this._finishSpeed(new Error('Normal playback requires a fresh session for unsequenced replies'), 'uncorrelated');
            }
            return;
        }
        if (message.sequence !== undefined && message.sequence !== pending.sequence) return;
        if (message.cmd !== undefined && message.cmd !== 'speed') return;
        // Official replies may omit sequence/cmd. Contradictory statuses fail closed.
        const failed = (message.statusString !== undefined
                && (typeof message.statusString !== 'string' || message.statusString.toLowerCase() !== 'ok'))
            || (message.status !== undefined && parseInt(message.status, 10) !== 200)
            || (message.res !== undefined && parseInt(message.res, 10) !== 0);
        const hasStatus = message.statusString !== undefined || message.status !== undefined || message.res !== undefined;
        if (hasStatus) this._publishPlaybackControlDiagnostics({ ack: failed ? 'rejected' : 'accepted' });
        if (failed) this._finishSpeed(new Error('Playback speed command was rejected'), 'rejected');
        // Successful ACKs do not prove the presented media rate.
    }

    // ─── RTP 解析 ─────────────────────────────────────────────────────────────

    _decodeH265(data) {
        if (this.useWasm) { this._decodeH265Wasm(data); return; }
        if (!this.decoder) return;
        const rtp = this._parseRTP(data);
        if (!rtp) return;
        this.diagnostics.rtpPackets++;
        this._publishDiagnostics();
        this._decodeRTP(rtp);
    }

    _decodeH265Wasm(data) {
        if (!this.transformWorker) return;
        const view = new Uint8Array(data);
        if (!this.wasmHeaderSent) {
            this.wasmPendingHeader = view.slice();
            this._sendPendingWasmHeader();
            return;
        }
        if (!this.wasmReady) return;
        this.transformWorker.postMessage({ type: 'inputData', buf: view.buffer, len: view.length });
    }

    _sendPendingWasmHeader() {
        if (!this.transformWorker || !this.wasmRuntimeReady || this.wasmHeaderSent || !this.wasmPendingHeader) return;
        const header = this.wasmPendingHeader;
        this.wasmPendingHeader = null;
        this.wasmHeaderSent = true;
        this.transformWorker.postMessage({ type: 'create', buf: header.buffer, len: header.length, packType: 11 });
    }

    _decodeRTP(rtp) {

        // 音频分支
        if (rtp.payloadType === this.AUDIO_PAYLOAD_TYPE) {
            this.diagnostics.audioPayloadPackets++;
            this._publishDiagnostics();
            let lost = 0;
            if (this.lastAudioSeq !== null) {
                lost = (rtp.sequenceNumber - this.lastAudioSeq - 1 + 65536) % 65536;
                if (lost > 0) this.audioPacketLoss += lost;
            }
            this.lastAudioSeq = rtp.sequenceNumber;
            this._playAudioPCM(this._decodePCMA(rtp.payload), { packetLoss: lost });
            return;
        }

        if (rtp.payloadType !== this.VIDEO_PAYLOAD_TYPE) {
            this.diagnostics.otherPayloadPackets++;
            this._publishDiagnostics();
            return;
        }
        this.diagnostics.videoPayloadPackets++;
        this._publishDiagnostics();

        const nalUnits = this._parseH265Payload(rtp.payload, rtp);
        if (!nalUnits) return;

        for (const nal of nalUnits) this._collectH265NAL(nal, rtp.timestamp);
        if (rtp.marker) {
            this.diagnostics.markerPackets++;
            this._publishDiagnostics();
            this._flushAccessUnit(rtp.timestamp);
        }
    }

    _collectH265NAL(nal, rtpTimestamp) {
        const nalType = (nal[0] >> 1) & 0x3F;

        // 参数集（VPS=32, SPS=33, PPS=34）
        if (nalType === 32 || nalType === 33 || nalType === 34) {
            if (nalType === 32) this.diagnostics.vpsPackets++;
            if (nalType === 33) this.diagnostics.spsPackets++;
            if (nalType === 34) this.diagnostics.ppsPackets++;
            this._publishDiagnostics();
            if (!this.h265Params) this.h265Params = new Map();

            const existing = this.h265Params.get(nalType);
            // 参数集内容变化 → 流已重启，重置状态重新收集
            if (existing && !this._arrayEqual(existing, nal)) {
                this._resetDecoderState();
                this.h265Params = new Map();
            }

            this.h265Params.set(nalType, nal);

            // 凑齐 VPS+SPS+PPS 后完成解码器配置
            if (!this.decoderConfigured && this.h265Params.size >= 3) {
                this._configureDecoderWithParams(this.h265Params);
            }
            return;
        }

        if (!this.decoderConfigured) return;

        const isKey = nalType >= 16 && nalType <= 21;
        if (isKey) {
            this.diagnostics.keyFrameNALUnits++;
            this._publishDiagnostics();
        }

        if (this.accessUnit && this.accessUnit.timestamp !== rtpTimestamp) {
            // A marker packet was lost. Do not send a partial frame to WebCodecs.
            this.diagnostics.droppedAccessUnits++;
            this.accessUnit = null;
        }

        if (!this.accessUnit) this.accessUnit = { timestamp: rtpTimestamp, nals: [], bytes: 0, isKey: false };
        if (this.accessUnit.bytes + nal.length > MAX_ACCESS_UNIT_BYTES) {
            this.diagnostics.droppedAccessUnits++;
            this.accessUnit = null;
            this._publishDiagnostics(true);
            return;
        }
        this.accessUnit.nals.push(nal);
        this.accessUnit.bytes += nal.length;
        this.accessUnit.isKey = this.accessUnit.isKey || isKey;
    }

    _flushAccessUnit(markerTimestamp) {
        const accessUnit = this.accessUnit;
        this.accessUnit = null;
        if (!accessUnit || accessUnit.timestamp !== markerTimestamp || accessUnit.nals.length === 0) return;

        this.diagnostics.accessUnits++;
        if (accessUnit.nals.length > 1) this.diagnostics.multiNALAccessUnits++;
        this._publishDiagnostics();

        if (this.waitingForKeyFrame || this.needKeyFrame) {
            if (!accessUnit.isKey) {
                this.diagnostics.droppedAccessUnits++;
                this._publishDiagnostics();
                return;
            }
            this.waitingForKeyFrame = false;
            this.needKeyFrame = false;
            this.frameIndex = 0;
            this.decodeErrorCount = 0;
        }

        this._decodeAccessUnit(accessUnit);
    }

    _decodeAccessUnit(accessUnit) {
        if (!this.decoder || this.decoder.state !== 'configured') return;

        let parts = accessUnit.nals;
        if (accessUnit.isKey && this.h265ParamSets) {
            const { vps, sps, pps } = this.h265ParamSets;
            parts = [vps, sps, pps, ...parts];
        }
        const total = parts.reduce((sum, part) => sum + START_CODE.length + part.length, 0);
        const frameData = new Uint8Array(total);
        let offset = 0;
        for (const part of parts) {
            frameData.set(START_CODE, offset); offset += START_CODE.length;
            frameData.set(part, offset); offset += part.length;
        }

        if (this.baseRtpTimestamp === null) this.baseRtpTimestamp = accessUnit.timestamp;
        const elapsedTicks = (accessUnit.timestamp - this.baseRtpTimestamp) >>> 0;
        const ts = Math.floor(elapsedTicks * 1000 / 90);

        this.diagnostics.decoderInputFrames++;
        this._publishDiagnostics();
        this._safeDecode(accessUnit.isKey ? 'key' : 'delta', ts, frameData);
        this.frameIndex++;
    }

    /**
     * 统一的解码入口，捕获同步异常并自动重建解码器。
     */
    _safeDecode(type, timestamp, data) {
        if (!this.decoder || this.decoder.state !== 'configured') return;
        try {
            this.decoder.decode(new EncodedVideoChunk({ type, timestamp, data }));
        } catch (e) {
            this._recordDecoderError(e);
            this._recoverDecoder();
        }
    }

    // ─── RTP / FU 解析 ────────────────────────────────────────────────────────

    _parseRTP(data) {
        return parseRTPPacket(data);
    }

    _parseH265Payload(payload, rtp) {
        if (payload.length < 2) return null;
        const nalType = (payload[0] >> 1) & 0x3F;
        if (nalType === 48) return this._parseAggregationPacket(payload);
        if (nalType === 49) {
            const nal = this._parseFU(payload, rtp);
            return nal ? [nal] : null;
        }
        this.fuBuffer = null;
        return [payload];
    }

    _parseAggregationPacket(payload) {
        const nals = [];
        let offset = 2;
        while (offset + 2 <= payload.length) {
            const length = (payload[offset] << 8) | payload[offset + 1];
            offset += 2;
            if (length === 0 || offset + length > payload.length) {
                this.diagnostics.malformedFuPackets++;
                this._publishDiagnostics(true);
                return null;
            }
            nals.push(payload.slice(offset, offset + length));
            offset += length;
        }
        if (offset !== payload.length || nals.length === 0) {
            this.diagnostics.malformedFuPackets++;
            this._publishDiagnostics(true);
            return null;
        }
        return nals;
    }

    _parseFU(payload, rtp) {
        if (payload.length < 2) return null;
        const nalType = (payload[0] >> 1) & 0x3F;

        // FU 分片（nalType === 49）
        if (nalType === 49) {
            if (payload.length < 4) return null;
            const fuHeader = payload[2];
            const S      = (fuHeader >> 7) & 0x01;
            const E      = (fuHeader >> 6) & 0x01;
            const fuType = fuHeader & 0x3F;

            if (S === 1) {
                const header = new Uint8Array([(fuType << 1) | (payload[0] & 0x81), payload[1]]);
                this.fuBuffer = {
                    timestamp: rtp.timestamp,
                    nextSequenceNumber: (rtp.sequenceNumber + 1) & 0xFFFF,
                    chunks: [header, payload.slice(3)]
                };
                if (E === 1) return this._completeFU();
                return null;
            }

            if (!this.fuBuffer || this.fuBuffer.timestamp !== rtp.timestamp || this.fuBuffer.nextSequenceNumber !== rtp.sequenceNumber) {
                this.fuBuffer = null;
                this.diagnostics.malformedFuPackets++;
                this._publishDiagnostics(true);
                return null;
            }

            this.fuBuffer.chunks.push(payload.slice(3));
            this.fuBuffer.nextSequenceNumber = (rtp.sequenceNumber + 1) & 0xFFFF;

            if (E === 1) return this._completeFU();
            return null;
        }

        return null;
    }

    _completeFU() {
        if (!this.fuBuffer) return null;
        const chunks = this.fuBuffer.chunks;
        const total = chunks.reduce((sum, chunk) => sum + chunk.length, 0);
        const nal = new Uint8Array(total);
        let offset = 0;
        for (const chunk of chunks) { nal.set(chunk, offset); offset += chunk.length; }
        this.fuBuffer = null;
        return nal;
    }

    _isRTP(data) {
        return (data[0] & 0xC0) === 0x80;
    }

    // ─── AVCC 工具 ────────────────────────────────────────────────────────────

    _parseAVCCNALs(data) {
        const nals = [];
        let offset = 0;
        while (offset + 4 <= data.length) {
            const len = (data[offset] << 24) | (data[offset+1] << 16) | (data[offset+2] << 8) | data[offset+3];
            offset += 4;
            if (len > 0 && offset + len <= data.length) {
                nals.push(data.slice(offset, offset + len));
                offset += len;
            } else break;
        }
        return nals;
    }

    _buildAvcCDescription(sps, pps) {
        const buf = new Uint8Array(11 + sps.length + 3 + pps.length);
        let i = 0;
        buf[i++] = 1; buf[i++] = sps[1]; buf[i++] = sps[2]; buf[i++] = sps[3];
        buf[i++] = 0xFF; buf[i++] = 0xE1;
        buf[i++] = (sps.length >> 8) & 0xFF; buf[i++] = sps.length & 0xFF;
        buf.set(sps, i); i += sps.length;
        buf[i++] = 1;
        buf[i++] = (pps.length >> 8) & 0xFF; buf[i++] = pps.length & 0xFF;
        buf.set(pps, i);
        return buf.buffer;
    }

    // ─── 音频 ─────────────────────────────────────────────────────────────────

    _initAudioContext() {
        if (this._audioGestureHandler) return;

        this._audioGestureHandler = () => {
            if (document.visibilityState === 'hidden' || this.currentVolume <= 0) return;
            if (this.audioContext?.state === 'running') return;
            // Safari may interrupt audio on a tab switch or screen lock. Keep
            // the gesture listener so returning users can resume the same stream.
            void this.enableAudio().catch(() => {});
        };
        document.addEventListener('click', this._audioGestureHandler);
        document.addEventListener('keydown', this._audioGestureHandler);
        document.addEventListener('touchend', this._audioGestureHandler);
        document.addEventListener('visibilitychange', this._audioGestureHandler);
    }

    _claimPlaybackAudioSession() {
        if (this._speedAudioMuted) return;
        if (this._playbackAudioSession) return;
        try {
            const session = navigator.audioSession;
            if (!session) return;
            let lease = playbackAudioSessions.get(session);
            if (!lease) {
                lease = { previousType: session.type, players: new Set() };
                session.type = 'playback';
                playbackAudioSessions.set(session, lease);
            }
            lease.players.add(this);
            this._playbackAudioSession = session;
        } catch (_) {
            // Audio Session is optional; unsupported browsers still use Web Audio.
        }
    }

    _releasePlaybackAudioSession() {
        const session = this._playbackAudioSession;
        this._playbackAudioSession = null;
        const lease = playbackAudioSessions.get(session);
        if (!lease) return;
        lease.players.delete(this);
        if (lease.players.size > 0) return;
        playbackAudioSessions.delete(session);
        try {
            if (session.type === 'playback') session.type = lease.previousType;
        } catch (_) {}
    }

    // Must be invoked directly from an application's unmute gesture. Browser
    // autoplay policies do not reliably treat a document-level listener as the
    // original user action once a control stops propagation.
    _ensureAudioGraph(context) {
        if (this._graphContext === context && this.gainNode && this.highpassNode && this.lowpassNode && this._pcmEngine) {
            return;
        }
        const hpf = context.createBiquadFilter();
        hpf.type = 'highpass';
        hpf.frequency.value = 180;
        hpf.Q.value = Math.SQRT1_2;

        const lpf = context.createBiquadFilter();
        lpf.type = 'lowpass';
        lpf.frequency.value = 2800;
        lpf.Q.value = Math.SQRT1_2;

        const gain = context.createGain();
        hpf.connect(lpf);
        lpf.connect(gain);
        gain.connect(context.destination);

        this.highpassNode = hpf;
        this.lowpassNode = lpf;
        this.gainNode = gain;
        this._graphContext = context;
        this._pcmEngine = new PcmStreamEngine({
            inputSampleRate: 8000,
            outputSampleRate: context.sampleRate || 48000
        });
        this.workletNode = null;
        this._workletInitPromise = null;
        this.nextAudioTime = 0;
    }

    _setSmoothGain(targetLinear, immediate = false) {
        if (!this.gainNode) return;
        if (this._speedAudioMuted) { targetLinear = 0; immediate = true; }
        if (!immediate && this.audioContext && typeof this.gainNode.gain.setTargetAtTime === 'function') {
            this.gainNode.gain.setTargetAtTime(targetLinear, this.audioContext.currentTime || 0, 0.008);
        } else {
            this.gainNode.gain.value = targetLinear;
        }
    }

    _resetAudioStream() {
        this.nextAudioTime = 0;
        if (this._scheduledAudioSources) {
            for (const source of this._scheduledAudioSources) {
                source.onended = null;
                try { source.stop(); } catch (_) {}
                try { source.disconnect(); } catch (_) {}
            }
            this._scheduledAudioSources.clear();
        }
        if (this._pcmEngine) this._pcmEngine.reset();
        if (this.workletNode?.port) {
            try { this.workletNode.port.postMessage({ type: 'reset' }); } catch (_) {}
        }
    }

    _ensureAudioWorklet(context) {
        if (this.workletNode && this._graphContext === context) {
            return Promise.resolve(true);
        }
        if (this._workletInitPromise && this._graphContext === context) {
            return this._workletInitPromise;
        }
        if (!context?.audioWorklet || typeof context.audioWorklet.addModule !== 'function' || typeof AudioWorkletNode === 'undefined') {
            return Promise.resolve(false);
        }
        const url = this.audioWorkletUrl || '/erzhuang-project/nvr-player/pcm-stream-worklet.js';
        this._workletInitPromise = (async () => {
            try {
                await context.audioWorklet.addModule(url, { credentials: 'same-origin' });
                if (this.audioContext !== context || context.state === 'closed' || !this.highpassNode) {
                    return false;
                }
                const node = new AudioWorkletNode(context, 'nvr-pcm-stream-processor', {
                    numberOfInputs: 0,
                    numberOfOutputs: 1,
                    outputChannelCount: [1]
                });
                node.connect(this.highpassNode);
                this.workletNode = node;
                return true;
            } catch (_) {
                this._workletInitPromise = null;
                return false;
            }
        })();
        return this._workletInitPromise;
    }

    async enableAudio() {
        this._audioGestureReady = true;
        if (this._speedAudioMuted) {
            this._resetAudioStream();
            this._releasePlaybackAudioSession();
        }
        this._claimPlaybackAudioSession();
        try {
            if (!this.audioContext || this.audioContext.state === 'closed') {
                this.audioContext = new (window.AudioContext || window.webkitAudioContext)();
            }
            const context = this.audioContext;
            this._ensureAudioGraph(context);
            this._setSmoothGain(this.currentVolume / 100, true);
            // Call resume synchronously in the user's gesture for both Safari's
            // interrupted state and the ordinary autoplay suspended state.
            const resumePromise = context.state !== 'running' ? context.resume() : null;
            const workletPromise = this._ensureAudioWorklet(context);
            if (resumePromise) await resumePromise;
            if (this.audioContext !== context || context.state !== 'running') {
                throw new Error('Audio output is not running');
            }
            await workletPromise;
            this.nextAudioTime = context.currentTime;
        } catch (error) {
            this._releasePlaybackAudioSession();
            throw error;
        }
    }

    _decodePCMA(pcmaData) {
        const pcm = new Int16Array(pcmaData.length);
        for (let i = 0; i < pcmaData.length; i++) {
            let alaw = pcmaData[i] ^ 0x55;
            const sign = (alaw & 0x80) ? -1 : 1;
            const exp  = (alaw & 0x70) >> 4;
            let val    = (alaw & 0x0F) << 4;
            val += 8;
            if (exp !== 0) val += 0x100;
            if (exp > 1)   val <<= (exp - 1);
            pcm[i] = sign * val;
        }
        return pcm;
    }

    _playAudioPCM(pcmData, options = {}) {
        if (!this.audioContext || this.audioContext.state !== 'running'
            || this.currentVolume <= 0 || this._isPaused || this._speedAudioMuted) {
            this._resetAudioStream();
            return;
        }
        this._ensureAudioGraph(this.audioContext);
        if (this.workletNode) {
            this.workletNode.port.postMessage({
                type: 'push',
                samples: pcmData,
                packetLoss: options.packetLoss || 0
            });
            return;
        }

        this._pcmEngine.pushInt16(pcmData, options);
        const rate = this.audioContext.sampleRate || 48000;
        const chunkFrames = Math.max(128, Math.round(rate * 0.04));
        while (this._pcmEngine.canScheduleFrames(chunkFrames)) {
            const buf = this.audioContext.createBuffer(1, chunkFrames, rate);
            const ch = buf.getChannelData(0);
            this._pcmEngine.pull(ch);

            const src = this.audioContext.createBufferSource();
            src.buffer = buf;
            src.connect(this.highpassNode);
            if (!this._scheduledAudioSources) this._scheduledAudioSources = new Set();
            this._scheduledAudioSources.add(src);
            src.onended = () => {
                this._scheduledAudioSources.delete(src);
                src.disconnect();
            };

            const minStart = (this.audioContext.currentTime || 0) + 0.01;
            if (this.nextAudioTime < minStart) {
                this.nextAudioTime = minStart;
            }
            src.start(this.nextAudioTime);
            this.nextAudioTime += buf.duration;
        }
    }

    // ─── WebSocket ────────────────────────────────────────────────────────────

    _connect() {
        if (!this._isRunning) return;

        try {
            this.ws = new WebSocket(this.wsUrl);
        } catch (err) {
            this.onError(err);
            this._scheduleReconnect();
            return;
        }
        this.ws.binaryType = 'arraybuffer';
        const socket = this.ws;

        this.ws.onopen = () => {
            if (this.ws !== socket || !this._isRunning) return;
            this._publishDiagnostics(true);
            this.onConnected();
        };

        this.ws.onmessage = ({ data }) => {
            if (this.ws !== socket || !this._isRunning) return;
            if (typeof data === 'string') {
                this._handleControlMessage(data);
                return;
            }
            if (data instanceof ArrayBuffer) {
                this.diagnostics.receivedPackets++;
                this._publishDiagnostics();
                this._decodeH265(data);
            }
        };

        this.ws.onclose = (e) => {
            if (this.ws !== socket) return;
            this._finishSpeed(new Error('Playback connection closed'), 'connection');
            this._isRunning = false;
            this.diagnostics.closeCode = Number.isFinite(e?.code) ? e.code : null;
            this._publishDiagnostics(true);
            this.onDisconnected({
                code: this.diagnostics.closeCode,
                wasClean: e?.wasClean === true
            });
            // 断线后重置解码器状态，确保重连后以干净状态处理新流
            this._resetDecoderState();
            this._scheduleReconnect();
        };

        this.ws.onerror = () => {
            if (this.ws !== socket) return;
            this._finishSpeed(new Error('Playback connection failed'), 'connection');
            // onerror 之后必然触发 onclose，无需在此重连，只上报错误
            this.onError(new Error('[NVRPlayer] WebSocket 连接错误'));
        };
    }

    _scheduleReconnect() {
        // Signed stream URLs must be refreshed by the application before reconnecting.
    }

    // ─── 工具 ─────────────────────────────────────────────────────────────────

    _arrayEqual(a, b) {
        if (a.length !== b.length) return false;
        for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return false;
        return true;
    }

    // ─── 公共 API ─────────────────────────────────────────────────────────────

    _sendCommand(cmd, params = {}) {
        if (this.ws?.readyState !== WebSocket.OPEN) return false;
        this.ws.send(JSON.stringify({ sequence: this.sequence++, cmd, ...params }));
        return true;
    }

    /**
     * 开始播放
     * @param {string} url - WebSocket URL；回放由 forceWasm 显式开启
     */
    async play(url) {
        if (this.ws || this.decoder || this.transformWorker) this.stop();
        this._finishSpeed(new Error('Playback restarted'), 'stopped');
        this._playbackGeneration = (this._playbackGeneration || 0) + 1;
        this._playbackBasePTS = null;
        this._playbackLastPTS = null;
        this._playbackTime = null;
        this._playbackClockSource = null;
        this._playbackFrameSequence = 0;
        this._playbackFrameTimes.clear();
        this._lastPresentedAt = null;
        this._speedRequiresSequence = false;
        this._resetPlaybackControlDiagnostics();
        this._speedAudioMuted = false;
        if (!this._monitorTimer) this._startDecoderMonitor();
        this._isRunning = true;
        this._isPaused = false;
        this._hasRenderedFirstFrame = false;
        this._resetDiagnostics();
        this.wsUrl   = url;
        this.useWasm = this.forceWasm;
        const generation = this._playbackGeneration;

        if (this.useWasm) {
            try {
                await this._initWasmDecoder();
            } catch (e) {
                if (this._isRunning && generation === this._playbackGeneration) this.onError(e);
                return;
            }
        } else {
            // 提前创建解码器（unconfigured 状态），减少首帧延迟
            this._createDecoder();
        }
        if (!this._isRunning || generation !== this._playbackGeneration) return;
        this._initAudioContext();
        this._connect();
    }

    /** A signed session must be re-issued by the host application. */
    start() {
        this.onError(new Error('需要重新创建播放会话'));
    }

    stop() {
        this._finishSpeed(new Error('Playback stopped'), 'stopped');
        this._playbackGeneration = (this._playbackGeneration || 0) + 1;
        this._playbackBasePTS = null;
        this._playbackLastPTS = null;
        this._playbackTime = null;
        this._playbackClockSource = null;
        this._playbackFrameSequence = 0;
        this._playbackFrameTimes.clear();
        this._lastPresentedAt = null;
        this._speedRequiresSequence = false;
        this._speedAudioMuted = false;
        this._isRunning = false;
        this._isPaused = false;
        this._hasRenderedFirstFrame = false;

        // 先清除重连定时器，再关闭 ws，防止 onclose 触发新的重连
        clearTimeout(this._reconnectTimer);
        this._reconnectTimer = null;

        if (this.ws) {
            this.ws.onclose = null;   // 屏蔽 onclose，避免 stop() 后触发自动重连
            this.ws.onerror = null;
            try { this.ws.close(); } catch (_) {}
            this.ws = null;
        }

        if (this.decoder) {
            try { if (this.decoder.state !== 'closed') this.decoder.close(); } catch (_) {}
            this.decoder = null;
        }

        // 清理音频恢复监听，防止播放器销毁后事件泄漏
        if (this._audioGestureHandler) {
            document.removeEventListener('click', this._audioGestureHandler);
            document.removeEventListener('keydown', this._audioGestureHandler);
            document.removeEventListener('touchend', this._audioGestureHandler);
            document.removeEventListener('visibilitychange', this._audioGestureHandler);
            this._audioGestureHandler = null;
        }

        this._resetAudioStream();
        this._releasePlaybackAudioSession();
        if (this.workletNode) {
            try {
                this.workletNode.port.postMessage({ type: 'dispose' });
                this.workletNode.disconnect();
            } catch (_) {}
            this.workletNode = null;
        }
        if (this.audioContext?.state !== 'closed') {
            try { this.audioContext.close(); } catch (_) {}
        }
        this.audioContext = null;
        this._graphContext = null;
        this.highpassNode = null;
        this.lowpassNode = null;
        this._workletInitPromise = null;
        this._pcmEngine = null;
        this._audioGestureReady = false;

        if (this.transformWorker) {
            this.transformWorker.terminate();
            this.transformWorker = null;
        }

        if (this._monitorTimer) {
            clearInterval(this._monitorTimer);
            this._monitorTimer = null;
        }

        this._resetDecoderState();
        this.h264DecoderConfigured = false;
        this.nextAudioTime  = 0;
        this.lastAudioSeq   = null;
        this.gainNode       = null;
        this.wasmRuntimeReady = false;
        this.wasmReady      = false;
        this.wasmHeaderSent = false;
        this.wasmPendingHeader = null;
        this.sequence       = 0;
    }

    setVolume(volume) {
        if (volume < 0 || volume > 100) throw new Error('音量范围必须在 0-100 之间');
        this.currentVolume = volume;
        this._setSmoothGain(volume / 100);
        if (volume === 0 || this._speedAudioMuted) {
            this._resetAudioStream();
            this._releasePlaybackAudioSession();
        } else if (this.audioContext && this._audioGestureReady) {
            this._claimPlaybackAudioSession();
        }
    }

    getPlaybackTime() { return this.useWasm ? this._playbackTime ?? null : null; }

    setSpeed(rate) {
        const rejectSpeed = (message, failure) => {
            if (!this._pendingSpeed) this._recordPlaybackControlRequest(rate, 'failed', failure);
            return Promise.reject(new Error(message));
        };
        if (!PLAYBACK_RATES.has(rate)) return rejectSpeed('Unsupported playback speed', 'unsupported');
        if (!this.useWasm || !this._isRunning || this._isPaused) {
            return rejectSpeed('Playback speed requires active WASM playback', this._isRunning && !this._isPaused ? 'unsupported' : 'stopped');
        }
        if (this.ws?.readyState !== WebSocket.OPEN) return rejectSpeed('Playback connection is not open', 'connection');
        if (this._pendingSpeed) {
            if (rate !== 1) return Promise.reject(new Error('Playback speed verification is already pending'));
            this._speedRequiresSequence = true;
            this._finishSpeed(new Error('Playback speed request superseded by normal speed'), 'superseded');
        }
        return new Promise((resolve, reject) => {
            const pending = {
                rate, resolve, reject, startedAt: performance.now(), sequence: this.sequence,
                window: null, matches: 0, mismatches: 0, timer: null, startedFrames: this.diagnostics.renderedFrames
            };
            this._pendingSpeed = pending;
            pending.timer = setTimeout(() => {
                this._expireSpeed();
            }, SPEED_TIMEOUT_MS);
            if (rate !== 1) {
                this._speedAudioMuted = true;
                this._setSmoothGain(0, true);
                this._resetAudioStream();
                this._releasePlaybackAudioSession();
            }
            try {
                if (!this._sendCommand('speed', { rate })) throw new Error('Playback speed command was not sent');
                this._recordPlaybackControlRequest(rate, 'sent', null, pending.sequence);
            } catch (error) {
                this._recordPlaybackControlRequest(rate, 'sent');
                this._finishSpeed(error, 'send_failed');
            }
        });
    }
    // Live streams do not guarantee upstream pause support. Keep receiving and
    // decoding the signed session but gate local canvas output deterministically.
    pause() {
        this._isPaused = true;
        this._finishSpeed(new Error('Playback paused'), 'stopped');
        this._resetAudioStream();
    }
    resume() {
        this._isPaused = false;
        if (this.currentVolume > 0 && this.audioContext && this.audioContext.state !== 'running') {
            void this.enableAudio().catch(() => {});
        }
    }
}

export default NVRPlayer;
