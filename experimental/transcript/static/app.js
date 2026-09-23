const $ = (id) => document.getElementById(id);
const els = {
  toggle: $('toggle'), model: $('model'), grammar: $('grammar'),
  meter: $('meter').firstElementChild, clear: $('clear'), copy: $('copy'),
  transcript: $('transcript'), status: $('status'), stats: $('stats'),
  banner: $('banner'),
};

// Transitions are slow enough to click through - the microphone prompt alone
// can sit there for seconds - so the button is a state machine rather than a
// boolean. Clicking during a transition is ignored instead of starting a
// second session on top of the first.
let state = 'idle'; // idle | starting | running | stopping

let ws = null, ctx = null, stream = null, node = null;
let live = null;    // the <p> holding the running partial
const finals = [];
let latencies = [];

init();

async function init() {
  els.toggle.onclick = () => (state === 'running' ? stop() : start());
  els.clear.onclick = clearAll;
  els.copy.onclick = () => navigator.clipboard.writeText(finals.join('\n'));
  window.addEventListener('beforeunload', () => teardown());

  try {
    const r = await fetch('/api/models');
    const { models } = await r.json();
    for (const m of models || []) {
      const opt = document.createElement('option');
      opt.value = opt.textContent = m;
      els.model.appendChild(opt);
    }
    if (!els.model.options.length) fail('The server reported no models. Run setup.ps1.');
  } catch (e) {
    fail('Could not reach the server: ' + e.message);
  }
}

async function start() {
  if (state !== 'idle') return;
  setState('starting');
  clearBanner();

  try {
    if (!els.model.value) throw new Error('no model selected');
    if (!navigator.mediaDevices?.getUserMedia) {
      throw new Error('this browser exposes no microphone API - open the page over http://127.0.0.1, not a LAN address');
    }

    setStatus('waiting for microphone permission');
    stream = await navigator.mediaDevices.getUserMedia({
      audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true },
    });

    // Asking for 16 kHz here means the worklet has no resampling to do.
    setStatus('starting audio');
    ctx = new AudioContext({ sampleRate: 16000 });
    await ctx.audioWorklet.addModule('worklet.js');
    if (ctx.state === 'suspended') await ctx.resume();

    setStatus('connecting');
    ws = await openSocket();

    node = new AudioWorkletNode(ctx, 'pcm-processor');
    node.port.onmessage = ({ data }) => {
      els.meter.style.width = Math.min(100, data.level * 320) + '%';
      if (ws && ws.readyState === WebSocket.OPEN) ws.send(data.pcm);
    };
    // A muted gain node keeps the graph pulling audio without echoing it back.
    const mute = ctx.createGain();
    mute.gain.value = 0;
    ctx.createMediaStreamSource(stream).connect(node).connect(mute).connect(ctx.destination);

    document.querySelector('.hint')?.remove();
    setState('running');
    setStatus('listening on ' + els.model.value);
  } catch (e) {
    // Any failure half-way through would otherwise leave the microphone open.
    await teardown();
    setState('idle');
    setStatus('idle');
    fail(describe(e));
  }
}

async function stop() {
  if (state !== 'running') return;
  setState('stopping');
  if (ws && ws.readyState === WebSocket.OPEN) ws.send('eof');
  await teardown();
  commitLive();
  setState('idle');
  setStatus('idle');
}

// openSocket resolves only once the upgrade succeeds. The old version awaited
// onopen alone, so a refused upgrade hung start() forever and the button never
// came back.
function openSocket() {
  // Follow the page's own scheme. The server only speaks plain HTTP, but it is
  // often reached through something that terminates TLS - VS Code port
  // forwarding, a dev tunnel, a reverse proxy - and a page loaded over HTTPS is
  // forbidden from opening a ws:// socket.
  const url = new URL('/ws', location.href);
  url.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  url.search = new URLSearchParams({
    model: els.model.value,
    grammar: els.grammar.value.trim(),
  }).toString();

  const sock = new WebSocket(url);
  sock.binaryType = 'arraybuffer';

  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      sock.close();
      reject(new Error('the server did not accept the websocket within 10s'));
    }, 10000);

    sock.onopen = () => {
      clearTimeout(timer);
      sock.onmessage = (ev) => handle(JSON.parse(ev.data));
      sock.onerror = null;
      sock.onclose = () => { if (state === 'running') stop(); };
      resolve(sock);
    };
    sock.onerror = () => {
      clearTimeout(timer);
      reject(new Error('websocket refused - is a model loaded?'));
    };
    sock.onclose = () => {
      clearTimeout(timer);
      reject(new Error('websocket closed during connect'));
    };
  });
}

// teardown is safe to call at any point, however far start() got.
async function teardown() {
  if (ws) {
    ws.onclose = ws.onerror = ws.onmessage = null;
    ws.close();
    ws = null;
  }
  if (node) {
    node.port.onmessage = null;
    node.port.close();
    node.disconnect();
    node = null;
  }
  stream?.getTracks().forEach((t) => t.stop());
  stream = null;
  if (ctx) {
    try { await ctx.close(); } catch {}
    ctx = null;
  }
  els.meter.style.width = '0';
}

function setState(s) {
  state = s;
  const busy = s === 'starting' || s === 'stopping';
  els.toggle.disabled = busy;
  els.toggle.classList.toggle('recording', s === 'running');
  els.toggle.textContent =
    { idle: 'Start', starting: 'Starting...', running: 'Stop', stopping: 'Stopping...' }[s];
  els.model.disabled = els.grammar.disabled = s !== 'idle';
}

function handle(m) {
  if (m.type === 'error') return fail(m.text);
  if (m.type === 'info') return;

  if (m.ms) {
    latencies.push(m.ms);
    if (latencies.length > 50) latencies.shift();
  }
  if (m.type === 'partial') {
    if (!live) {
      live = document.createElement('p');
      live.className = 'live';
      els.transcript.appendChild(live);
    }
    live.textContent = m.text;
  } else if (m.type === 'final') {
    finals.push(m.text);
    if (live) {
      live.className = '';
      live.textContent = m.text;
      live = null;
    } else {
      const p = document.createElement('p');
      p.textContent = m.text;
      els.transcript.appendChild(p);
    }
  }
  els.transcript.scrollTop = els.transcript.scrollHeight;
  render();
}

// A partial left on screen when recording stops is still the best guess we
// have, so keep it rather than dropping it.
function commitLive() {
  if (!live) return;
  if (live.textContent.trim()) {
    live.className = '';
    finals.push(live.textContent);
  } else {
    live.remove();
  }
  live = null;
  render();
}

function clearAll() {
  finals.length = 0;
  latencies = [];
  live = null;
  els.transcript.textContent = '';
  clearBanner();
  render();
}

function render() {
  const words = finals.join(' ').split(/\s+/).filter(Boolean).length;
  const worst = latencies.length ? Math.max(...latencies) : 0;
  els.stats.textContent =
    finals.length + ' utterances · ' + words + ' words · recognizer peak ' + worst + ' ms';
}

// Failures used to land in the footer, which is easy to miss when the button
// simply does not change - so they get a banner.
function describe(e) {
  if (e.name === 'NotAllowedError') return 'Microphone permission was denied. Allow it in the address bar, then press Start again.';
  if (e.name === 'NotFoundError') return 'No microphone was found.';
  if (e.name === 'NotReadableError') return 'The microphone is in use by another application.';
  return e.message || String(e);
}

function fail(text) {
  els.banner.textContent = text;
  els.banner.hidden = false;
}

function clearBanner() { els.banner.hidden = true; }

function setStatus(text) { els.status.textContent = text; }
