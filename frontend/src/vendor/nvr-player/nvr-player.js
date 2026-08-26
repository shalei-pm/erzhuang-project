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
const START_CODE = new Uint8Array([0, 0, 0, 1]);

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

        this.autoReconnect  = false;
        this.reconnectDelay = options.reconnectDelay || 3000;
        this.forceWasm      = options.forceWasm === true;
        this.wasmWorkerUrl  = options.wasmWorkerUrl || '/erzhuang-project/nvr-player/wasm/systemTransform-worker.js';
        this._isRunning     = false;
        this._isPaused      = false;

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
        this._audioGestureReady = false;  // 用户手势触发后置为 true
        this._audioGestureHandler = null; // 手势监听引用，用于清理

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

        this.decoder = new VideoDecoder({
            output: (frame) => this._renderFrame(frame),
            error: (e) => {
                this.decodeErrorCount++;
                // 重置所有解码状态，重建解码器，等待下一个关键帧恢复
                this._resetDecoderState();
                try { this._createDecoder(); } catch (_) {}
            }
        });
    }

    /**
     * 重置所有解码相关的状态变量（不包含 decoder 对象本身）。
     * 重连、流重启、解码器错误时均调用此方法。
     */
    _resetDecoderState() {
        this.decoderConfigured  = false;
        this.waitingForKeyFrame = true;
        this.needKeyFrame       = false;
        this.fuBuffer           = null;
        this.h265Params         = null;
        this.h265ParamSets      = null;
        this.baseRtpTimestamp   = null;
        this.frameIndex         = 0;
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
                    this.decoder.flush().catch(() => {});
                    this.lastOutputTime = Date.now();
                }
            }
        }, 2000);
    }

    _resetDiagnostics() {
        this.diagnostics = {
            receivedPackets: 0,
            wasmRuntimeReady: false,
            wasmReady: false,
            wasmOutputInit: 0,
            wasmOutputFrames: 0,
            decoderInputFrames: 0,
            renderedFrames: 0,
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
        if (this.transformWorker) {
            this.transformWorker.terminate();
            this.transformWorker = null;
        }
        this.h264DecoderConfigured = false;
        this.wasmReady      = false;
        this.wasmRuntimeReady = false;
        this.wasmHeaderSent = false;
        this.wasmPendingHeader = null;

        this.transformWorker = await this._createTransformWorker();
        this.transformWorker.onmessage = ({ data: e }) => {
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
            this.decoder = new VideoDecoder({
                output: (frame) => this._renderFrame(frame),
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
                        timestamp: 0,
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
            this._safeDecode('key', 0, data);
        }
    }

    _handleH264Frame(buf, frameInfo) {
        if (frameInfo.nFrameType === 4) {
            this._playAudioPCM(this._decodePCMA(new Uint8Array(buf)));
            return;
        }
        if (!this.h264DecoderConfigured || !this.decoder || this.decoder.state !== 'configured') return;

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
                    timestamp: frameInfo.nTimeStamp * 1000,
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
        this._safeDecode(isKeyFrame ? 'key' : 'delta', frameInfo.nTimeStamp * 1000, frameData);
    }

    // ─── 渲染 ─────────────────────────────────────────────────────────────────

    _renderFrame(frame) {
        if (!this._isRunning) { frame.close(); return; }
        if (this._isPaused) { frame.close(); return; }
        this.lastOutputTime = Date.now();
        // 自动对齐 canvas 绘图缓冲区到视频帧实际分辨率，避免 CSS 拉伸模糊
        if (this.canvas.width !== frame.displayWidth || this.canvas.height !== frame.displayHeight) {
            this.canvas.width  = frame.displayWidth;
            this.canvas.height = frame.displayHeight;
        }
        this.ctx.drawImage(frame, 0, 0);
        this.diagnostics.renderedFrames++;
        this._publishDiagnostics();
        if (!this._hasRenderedFirstFrame) {
            this._hasRenderedFirstFrame = true;
            this.onFirstFrame();
        }
        frame.close();
    }

    // ─── RTP 解析 ─────────────────────────────────────────────────────────────

    _decodeH265(data) {
        if (this.useWasm) { this._decodeH265Wasm(data); return; }
        if (!this.decoder) return;
        const view = new Uint8Array(data);
        if (this._isRTP(view)) this._decodeRTP(data);
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

    _decodeRTP(data) {
        const rtp = this._parseRTP(data);
        if (!rtp) return;

        // 音频分支
        if (rtp.payloadType === this.AUDIO_PAYLOAD_TYPE) {
            if (this.lastAudioSeq !== null) {
                const lost = (rtp.sequenceNumber - this.lastAudioSeq - 1 + 65536) % 65536;
                if (lost > 0) this.audioPacketLoss += lost;
            }
            this.lastAudioSeq = rtp.sequenceNumber;
            this._playAudioPCM(this._decodePCMA(rtp.payload));
            return;
        }

        if (rtp.payloadType !== this.VIDEO_PAYLOAD_TYPE) return;

        const nal = this._parseFU(rtp.payload);
        if (!nal) return;

        const nalType = (nal[0] >> 1) & 0x3F;

        // 参数集（VPS=32, SPS=33, PPS=34）
        if (nalType === 32 || nalType === 33 || nalType === 34) {
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

        const isKey = nalType >= 16 && nalType <= 20;

        // 等待关键帧
        if (this.waitingForKeyFrame || this.needKeyFrame) {
            if (!isKey) return;
            this.waitingForKeyFrame = false;
            this.needKeyFrame       = false;
            this.frameIndex         = 0;
            this.decodeErrorCount   = 0;
        }

        if (isKey || (nalType >= 1 && nalType <= 9)) {
            this._decodeNAL(nal, isKey, rtp.timestamp);
        }
    }

    _decodeNAL(nal, isKey, rtpTimestamp) {
        if (!this.decoder || this.decoder.state !== 'configured') return;

        let frameData;
        if (isKey && this.h265ParamSets) {
            const { vps, sps, pps } = this.h265ParamSets;
            const total = 4 * 4 + vps.length + sps.length + pps.length + nal.length;
            frameData   = new Uint8Array(total);
            let offset  = 0;
            for (const part of [vps, sps, pps, nal]) {
                frameData.set(START_CODE, offset); offset += 4;
                frameData.set(part, offset);       offset += part.length;
            }
        } else {
            frameData = new Uint8Array(4 + nal.length);
            frameData.set(START_CODE, 0);
            frameData.set(nal, 4);
        }

        if (this.baseRtpTimestamp === null) this.baseRtpTimestamp = rtpTimestamp;
        const ts = Math.floor((rtpTimestamp - this.baseRtpTimestamp) * 1000 / 90);

        this._safeDecode(isKey ? 'key' : 'delta', ts, frameData);
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
            this.onError(e);
            this._resetDecoderState();
            try { this._createDecoder(); } catch (_) {}
        }
    }

    // ─── RTP / FU 解析 ────────────────────────────────────────────────────────

    _parseRTP(data) {
        const v = new Uint8Array(data);
        if (v.length < 12) return null;
        return {
            payloadType:    v[1] & 0x7F,
            sequenceNumber: (v[2] << 8) | v[3],
            timestamp:      (v[4] << 24) | (v[5] << 16) | (v[6] << 8) | v[7],
            payload:        v.slice(12)
        };
    }

    _parseFU(payload) {
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
                // 新分片序列开始，丢弃旧残留缓冲
                const header = new Uint8Array([(fuType << 1) | (payload[0] & 0x81), payload[1]]);
                this.fuBuffer = [header, payload.slice(3)];
                return null;
            }

            // 未收到起始包就来了中间/结束包（丢包场景），等待下一轮
            if (!this.fuBuffer) return null;

            this.fuBuffer.push(payload.slice(3));

            if (E === 1) {
                const total = this.fuBuffer.reduce((s, a) => s + a.length, 0);
                const nal   = new Uint8Array(total);
                let offset  = 0;
                for (const chunk of this.fuBuffer) { nal.set(chunk, offset); offset += chunk.length; }
                this.fuBuffer = null;
                return nal;
            }
            return null;
        }

        // 非分片 NAL：清理残留 FU 缓存（防止后续 FU 序列拼接到错误缓冲）
        this.fuBuffer = null;
        return payload;
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
        // Chrome 自动播放策略：AudioContext 必须在用户手势后创建，否则会被锁定并持续报警告。
        // 此处不立即创建，而是注册一次性手势监听；用户首次交互时才真正创建 AudioContext。
        // 交互前到来的音频帧会在 _playAudioPCM 中被静默丢弃，不影响视频播放。
        if (this._audioGestureReady || this._audioGestureHandler) return;

        this._audioGestureHandler = () => {
            this._audioGestureReady = true;
            if (!this.audioContext) {
                this.audioContext = new (window.AudioContext || window.webkitAudioContext)();
                this.nextAudioTime = this.audioContext.currentTime;
                this.gainNode = this.audioContext.createGain();
                this.gainNode.gain.value = this.currentVolume / 100;
                this.gainNode.connect(this.audioContext.destination);
            }
            document.removeEventListener('click', this._audioGestureHandler);
            document.removeEventListener('keydown', this._audioGestureHandler);
            document.removeEventListener('touchstart', this._audioGestureHandler);
            this._audioGestureHandler = null;
        };
        document.addEventListener('click', this._audioGestureHandler);
        document.addEventListener('keydown', this._audioGestureHandler);
        document.addEventListener('touchstart', this._audioGestureHandler);
    }

    // Must be invoked directly from an application's unmute gesture. Browser
    // autoplay policies do not reliably treat a document-level listener as the
    // original user action once a control stops propagation.
    enableAudio() {
        this._audioGestureReady = true;
        if (!this.audioContext) {
            this.audioContext = new (window.AudioContext || window.webkitAudioContext)();
            this.nextAudioTime = this.audioContext.currentTime;
            this.gainNode = this.audioContext.createGain();
            this.gainNode.connect(this.audioContext.destination);
        }
        this.gainNode.gain.value = this.currentVolume / 100;
        if (this.audioContext.state === 'suspended') {
            return this.audioContext.resume();
        }
        return Promise.resolve();
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

    _playAudioPCM(pcmData) {
        // 用户尚未交互，AudioContext 还未创建，静默丢弃音频帧
        if (!this.audioContext || !this.gainNode) return;
        const buf = this.audioContext.createBuffer(1, pcmData.length, 8000);
        const ch  = buf.getChannelData(0);
        for (let i = 0; i < pcmData.length; i++) ch[i] = pcmData[i] / 32768.0;

        const src = this.audioContext.createBufferSource();
        src.buffer = buf;

        const lpf = this.audioContext.createBiquadFilter();
        lpf.type = 'lowpass'; lpf.frequency.value = 2000; lpf.Q.value = 1.0;
        const hpf = this.audioContext.createBiquadFilter();
        hpf.type = 'highpass'; hpf.frequency.value = 200; hpf.Q.value = 1.0;

        src.connect(hpf); hpf.connect(lpf); lpf.connect(this.gainNode);

        if (this.nextAudioTime < this.audioContext.currentTime) {
            this.nextAudioTime = this.audioContext.currentTime;
        }
        src.start(this.nextAudioTime);
        this.nextAudioTime += buf.duration;
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

        this.ws.onopen = () => {
            this._publishDiagnostics(true);
            this.onConnected();
        };

        this.ws.onmessage = ({ data }) => {
            if (data instanceof ArrayBuffer) {
                this.diagnostics.receivedPackets++;
                this._publishDiagnostics();
                this._decodeH265(data);
            }
        };

        this.ws.onclose = (e) => {
            this.diagnostics.closeCode = Number.isFinite(e?.code) ? e.code : null;
            this._publishDiagnostics(true);
            this.onDisconnected();
            // 断线后重置解码器状态，确保重连后以干净状态处理新流
            this._resetDecoderState();
            this._scheduleReconnect();
        };

        this.ws.onerror = () => {
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
        if (this.ws?.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({ sequence: this.sequence++, cmd, ...params }));
        }
    }

    /**
     * 开始播放
     * @param {string} url - WebSocket URL；回放由 forceWasm 显式开启
     */
    async play(url) {
        this._isRunning = true;
        this._isPaused = false;
        this._hasRenderedFirstFrame = false;
        this._resetDiagnostics();
        this.wsUrl   = url;
        this.useWasm = this.forceWasm;

        if (this.useWasm) {
            try {
                await this._initWasmDecoder();
            } catch (e) {
                this.onError(e);
                return;
            }
        } else {
            // 提前创建解码器（unconfigured 状态），减少首帧延迟
            this._createDecoder();
        }
        this._initAudioContext();
        this._connect();
    }

    /** A signed session must be re-issued by the host application. */
    start() {
        this.onError(new Error('需要重新创建播放会话'));
    }

    stop() {
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

        // 清理未触发的手势监听，防止播放器销毁后事件泄漏
        if (this._audioGestureHandler) {
            document.removeEventListener('click', this._audioGestureHandler);
            document.removeEventListener('keydown', this._audioGestureHandler);
            document.removeEventListener('touchstart', this._audioGestureHandler);
            this._audioGestureHandler = null;
        }

        if (this.audioContext?.state !== 'closed') {
            try { this.audioContext.close(); } catch (_) {}
        }
        this.audioContext = null;
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
        if (this.gainNode) this.gainNode.gain.value = volume / 100;
    }

    setSpeed(rate) { this._sendCommand('speed',  { rate }); }
    // Live streams do not guarantee upstream pause support. Keep receiving and
    // decoding the signed session but gate local canvas output deterministically.
    pause()        { this._isPaused = true; }
    resume()       { this._isPaused = false; }
}

export default NVRPlayer;
