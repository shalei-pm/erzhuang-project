/* eslint-disable */
/**
 * Continuous 8kHz -> AudioContext.sampleRate stream resampler with jitter
 * pre-buffering, packet-loss seam smoothing, and click-free fade ramps.
 */
class PcmStreamEngine {
    constructor(options = {}) {
        this.inputSampleRate = options.inputSampleRate || 8000;
        this.outputSampleRate = options.outputSampleRate || 48000;
        this.step = this.inputSampleRate / this.outputSampleRate;

        const minCapacity = options.capacity || 16384;
        let cap = 1024;
        while (cap < minCapacity) cap <<= 1;
        this.capacity = cap;
        this.mask = cap - 1;
        this.ring = new Float32Array(cap);

        this.minStartSamples = options.minStartSamples || Math.round(this.inputSampleRate * 0.08);
        this.maxBufferedSamples = options.maxBufferedSamples || Math.round(this.inputSampleRate * 0.36);
        this.targetTrimSamples = options.targetTrimSamples || Math.round(this.inputSampleRate * 0.12);
        this.fadeSamples = Math.max(1, Math.round(this.outputSampleRate * 0.005));
        this.seamSmoothSamples = Math.max(2, Math.round(this.inputSampleRate * 0.002));

        this.reset();
    }

    reset() {
        this.readIndex = 0;
        this.writeIndex = 0;
        this.readPhase = 0;
        this.mode = "buffering";
        this.fadeInPos = 0;
        this.fadeStartSample = 0;
        this.underrunActive = false;
        this.underrunPos = 0;
        this.underrunStartSample = 0;
        this.lastOutputSample = 0;
        this.hasPushedSample = false;
        this.lastPushedSample = 0;
    }

    get bufferedSamples() {
        return this.writeIndex - this.readIndex;
    }

    pushInt16(samples, options = {}) {
        if (!samples || samples.length === 0) return;
        const len = samples.length;
        const packetLoss = options.packetLoss || 0;

        if (this.bufferedSamples + len > this.capacity - 2) {
            this.readIndex = Math.max(this.readIndex, this.writeIndex + len - (this.capacity - 2));
            this.readPhase = 0;
        }

        const smoothCount = (packetLoss > 0 && this.hasPushedSample)
            ? Math.min(len, this.seamSmoothSamples)
            : 0;
        const prevSample = this.lastPushedSample;

        for (let i = 0; i < len; i++) {
            let s = samples[i] / 32768.0;
            if (s > 1) s = 1;
            else if (s < -1) s = -1;

            if (i < smoothCount) {
                const alpha = (i + 1) / (smoothCount + 1);
                s = prevSample * (1 - alpha) + s * alpha;
            }
            this.ring[this.writeIndex & this.mask] = s;
            this.writeIndex++;
        }

        this.lastPushedSample = this.ring[(this.writeIndex - 1) & this.mask];
        this.hasPushedSample = true;

        if (this.bufferedSamples > this.maxBufferedSamples) {
            const anchor = this._peekCurrentSample();
            this.readIndex = this.writeIndex - this.targetTrimSamples;
            this.readPhase = 0;
            const blend = Math.min(this.seamSmoothSamples, this.bufferedSamples);
            for (let i = 0; i < blend; i++) {
                const idx = (this.readIndex + i) & this.mask;
                const alpha = (i + 1) / (blend + 1);
                this.ring[idx] = anchor * (1 - alpha) + this.ring[idx] * alpha;
            }
        }
    }

    _peekCurrentSample() {
        if (this.bufferedSamples <= 0) return this.lastOutputSample;
        const s0 = this.ring[this.readIndex & this.mask];
        if (this.bufferedSamples < 2) return s0;
        const s1 = this.ring[(this.readIndex + 1) & this.mask];
        return s0 + (s1 - s0) * this.readPhase;
    }

    canScheduleFrames(outputFrames) {
        const neededInput = Math.ceil(outputFrames * this.step) + 2;
        if (this.mode === "playing") {
            return this.bufferedSamples >= neededInput;
        }
        return this.bufferedSamples >= Math.max(this.minStartSamples, neededInput);
    }

    pull(output) {
        const outLen = output.length;
        for (let i = 0; i < outLen; i++) {
            if (this.mode === "buffering") {
                if (this.bufferedSamples >= this.minStartSamples) {
                    this.mode = "playing";
                    this.fadeStartSample = this.lastOutputSample;
                    this.fadeInPos = 0;
                    this.underrunActive = false;
                } else {
                    output[i] = this._nextUnderrunSample();
                    continue;
                }
            }

            if (this.bufferedSamples < 2) {
                this.mode = "buffering";
                this.underrunActive = Math.abs(this.lastOutputSample) > 1e-6;
                this.underrunStartSample = this.lastOutputSample;
                this.underrunPos = 0;
                output[i] = this._nextUnderrunSample();
                continue;
            }

            const idx = this.readIndex;
            const s0 = this.ring[idx & this.mask];
            const s1 = this.ring[(idx + 1) & this.mask];
            let sample = s0 + (s1 - s0) * this.readPhase;

            if (this.fadeInPos < this.fadeSamples) {
                this.fadeInPos++;
                const t = this.fadeInPos / this.fadeSamples;
                sample = this.fadeStartSample * (1 - t) + sample * t;
            }

            output[i] = sample;
            this.lastOutputSample = sample;

            this.readPhase += this.step;
            const advance = Math.floor(this.readPhase);
            if (advance > 0) {
                this.readIndex += advance;
                this.readPhase -= advance;
            }
        }
        return output;
    }

    _nextUnderrunSample() {
        if (!this.underrunActive) {
            this.lastOutputSample = 0;
            return 0;
        }
        this.underrunPos++;
        if (this.underrunPos >= this.fadeSamples) {
            this.underrunActive = false;
            this.lastOutputSample = 0;
            return 0;
        }
        const t = this.underrunPos / this.fadeSamples;
        const sample = this.underrunStartSample * (1 - t);
        this.lastOutputSample = sample;
        return sample;
    }
}

class NvrPcmStreamProcessor extends AudioWorkletProcessor {
    constructor() {
        super();
        this.alive = true;
        this.engine = new PcmStreamEngine({ inputSampleRate: 8000, outputSampleRate: sampleRate });
        this.port.onmessage = (event) => {
            const msg = event.data;
            if (!msg) return;
            if (msg.type === "push") {
                this.engine.pushInt16(msg.samples, { packetLoss: msg.packetLoss || 0 });
            } else if (msg.type === "reset") {
                this.engine.reset();
            } else if (msg.type === "dispose") {
                this.engine.reset();
                this.alive = false;
            }
        };
    }

    process(_inputs, outputs) {
        if (!this.alive) return false;
        const out = outputs[0];
        if (!out || out.length === 0) return true;
        const ch0 = out[0];
        this.engine.pull(ch0);
        for (let c = 1; c < out.length; c++) {
            out[c].set(ch0);
        }
        return true;
    }
}

registerProcessor("nvr-pcm-stream-processor", NvrPcmStreamProcessor);
