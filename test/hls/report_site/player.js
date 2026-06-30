const params = new URLSearchParams(location.search);
const playlist = params.get('playlist') || params.get('url') || '';
const label = params.get('label') || playlist;

document.getElementById('title').textContent = label || 'HLS playback';

const video = document.getElementById('video');
const statusEl = document.getElementById('status');
const auditEl = document.getElementById('audit');

const audit = { jumps: [], errors: [] };
window.__playbackAudit = audit;

let lastT = 0;
let lastWall = performance.now();
let seeking = false;

function log(line) {
  auditEl.textContent += line + '\n';
  auditEl.scrollTop = auditEl.scrollHeight;
}

function samplePlayhead() {
  const now = performance.now();
  const t = video.currentTime;
  const wallDelta = (now - lastWall) / 1000;
  const playDelta = t - lastT;
  if (lastT > 0 && !video.paused && !seeking && !video.seeking) {
    const forwardJump = playDelta - wallDelta;
    if (playDelta < -0.05) {
      audit.jumps.push({ kind: 'backward', at: t, delta: playDelta });
      log(`BACKWARD ${t.toFixed(2)}s Δ${playDelta.toFixed(2)}s`);
    } else if (forwardJump > 0.35) {
      audit.jumps.push({ kind: 'forward', at: t, delta: forwardJump });
      log(`SKIP ${t.toFixed(2)}s +${forwardJump.toFixed(2)}s`);
    }
  }
  lastT = t;
  lastWall = now;
}

video.addEventListener('seeking', () => { seeking = true; });
video.addEventListener('seeked', () => { seeking = false; lastT = video.currentTime; lastWall = performance.now(); });

if (!playlist) {
  statusEl.textContent = 'Missing ?playlist=/media/…/playlist.m3u8';
} else if (!Hls.isSupported()) {
  statusEl.textContent = 'hls.js not supported in this browser';
} else {
  const hls = new Hls({ maxBufferLength: 12, maxMaxBufferLength: 12, maxBufferHole: 2.0, testBandwidth: false });
  hls.on(Hls.Events.ERROR, (_e, data) => {
    audit.errors.push(data);
    log(`ERROR ${data.type}/${data.details} fatal=${data.fatal}`);
  });
  hls.on(Hls.Events.FRAG_BUFFERED, (_e, data) => {
    log(`buf sn=${data.frag?.sn} start=${data.frag?.start?.toFixed(2)} ct=${video.currentTime.toFixed(2)}`);
  });
  hls.loadSource(playlist);
  hls.attachMedia(video);
  hls.on(Hls.Events.MANIFEST_PARSED, () => {
    statusEl.textContent = 'Playing — watch for skips; jumps logged below';
    void video.play();
    setInterval(samplePlayhead, 100);
  });
}
