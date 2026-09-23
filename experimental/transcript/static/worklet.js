// Captures mic audio and hands it to the page as 100 ms chunks of signed
// 16-bit PCM. The AudioContext is created at 16 kHz, so there is no
// resampling to do here - just float32 -> int16 and a level for the meter.
const CHUNK = 1600; // 100 ms at 16 kHz

class PCMProcessor extends AudioWorkletProcessor {
  constructor() {
    super();
    this.buf = new Float32Array(CHUNK);
    this.n = 0;
  }

  process(inputs) {
    const input = inputs[0];
    if (!input || !input[0]) return true;
    const ch = input[0];

    for (let i = 0; i < ch.length; i++) {
      this.buf[this.n++] = ch[i];
      if (this.n < CHUNK) continue;

      const pcm = new Int16Array(CHUNK);
      let sum = 0;
      for (let j = 0; j < CHUNK; j++) {
        const s = Math.max(-1, Math.min(1, this.buf[j]));
        pcm[j] = s < 0 ? s * 0x8000 : s * 0x7fff;
        sum += s * s;
      }
      this.port.postMessage(
        { pcm: pcm.buffer, level: Math.sqrt(sum / CHUNK) },
        [pcm.buffer]
      );
      this.n = 0;
    }
    return true;
  }
}

registerProcessor('pcm-processor', PCMProcessor);
