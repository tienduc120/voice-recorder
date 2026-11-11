"use strict";

// The page polls; the recorder keeps running without it, so a reopened tab simply catches up.
const STATUS_MS = 500;
// The meter has its own, faster loop: the server holds each peak and lets it fall, so 20 readings a second are
// enough for the animation below to look continuous.
const LEVEL_MS = 50;
const ERROR_SHOWN_MS = 8000;
const CELLS = 32;
// Where the ladder turns amber and red, as fractions of the 60 dB scale (about -24 and -9 dBFS).
const WARM_FROM = 0.6;
const HOT_FROM = 0.85;
// Display ballistics: rise almost at once, settle smoothly; the peak tick holds, then drifts down.
const RISE = 0.6;
const FALL = 0.2;
const PEAK_HOLD_MS = 900;
const PEAK_DRIFT = 0.008;

const $ = (id) => document.getElementById(id);

const STATE_LABELS = {
  "idle": "Standby",
  "recording": "Recording",
  "paused": "Paused",
  "waiting-for-mic": "Waiting for a mic",
};

let current = null;
let micsShown = "";
let errorText = "";
let errorUntil = 0;

const meter = { target: 0, shown: 0, peak: 0, peakAt: 0, lit: -1, cells: [] };

function noticeText(n) {
  switch (n.kind) {
    case "mic-connected":
      return `“${n.mic}” connected — choose it below to switch`;
    case "mic-fallback":
      return `“${n.previousMic}” disconnected — now recording from “${n.mic}”`;
    case "no-mic":
      return "No microphone — recording paused until one is connected";
    case "disk-full":
      return "Disk full — recording stopped and saved";
    case "write-failed":
      return "Could not write the recording — stopped and saved what was written";
    default:
      return "";
  }
}

function clock(ms) {
  const total = Math.floor(ms / 1000);
  const pad = (v) => String(v).padStart(2, "0");
  return `${pad(Math.floor(total / 3600))}:${pad(Math.floor(total / 60) % 60)}:${pad(total % 60)}`;
}

function megabytes(bytes) {
  return `${(bytes / 1e6).toFixed(1)} MB`;
}

function showError(text) {
  errorText = text || "Something went wrong.";
  errorUntil = Date.now() + ERROR_SHOWN_MS;
  if (current) render(current);
}

// setUnlessEditing leaves a field the owner is typing in alone.
function setUnlessEditing(el, apply) {
  if (document.activeElement !== el) apply();
}

function buildMeter() {
  const holder = $("level").querySelector(".meter-cells");
  meter.cells = Array.from({ length: CELLS }, (_, i) => {
    const cell = document.createElement("span");
    const at = (i + 1) / CELLS;
    if (at > HOT_FROM) cell.className = "hot";
    else if (at > WARM_FROM) cell.className = "warm";
    return cell;
  });
  holder.replaceChildren(...meter.cells);
}

function drawMeter(now) {
  const m = meter;
  m.shown += (m.target - m.shown) * (m.target > m.shown ? RISE : FALL);
  if (m.shown < 0.002) m.shown = 0;
  if (m.shown >= m.peak) {
    m.peak = m.shown;
    m.peakAt = now;
  } else if (now - m.peakAt > PEAK_HOLD_MS) {
    m.peak = Math.max(m.shown, m.peak - PEAK_DRIFT);
  }

  const lit = Math.round(m.shown * CELLS);
  if (lit !== m.lit) {
    m.cells.forEach((cell, i) => cell.classList.toggle("on", i < lit));
    m.lit = lit;
    $("level").setAttribute("aria-valuenow", String(Math.round(m.shown * 100)));
  }
  const el = $("level");
  el.style.setProperty("--peak", String(Math.min(m.peak, 0.995)));
  el.style.setProperty("--peak-shown", m.peak > 0.02 ? "1" : "0");
  window.requestAnimationFrame(drawMeter);
}

function renderMics(s) {
  const shown = JSON.stringify(s.mics);
  const select = $("mic");
  if (shown !== micsShown) {
    micsShown = shown;
    const options = s.mics.map((m) => {
      const option = document.createElement("option");
      option.value = m.id;
      option.textContent = m.isDefault ? `${m.name}  · Windows default` : m.name;
      return option;
    });
    if (options.length === 0) {
      const none = document.createElement("option");
      none.textContent = "No microphone connected";
      none.value = "";
      options.push(none);
    }
    select.replaceChildren(...options);
  }
  setUnlessEditing(select, () => { select.value = s.micId; });
}

function renderPause(state) {
  const key = $("pause");
  const paused = state === "paused";
  key.querySelector(".glyph").className = `glyph ${paused ? "glyph-play" : "glyph-pause"}`;
  key.querySelector(".key-label").textContent = paused ? "Resume" : "Pause";
  key.disabled = state === "idle";
}

function render(s) {
  current = s;
  const idle = s.state === "idle";
  document.body.dataset.state = s.state;

  const state = $("state");
  state.textContent = STATE_LABELS[s.state] || s.state;
  state.className = `state ${s.state}`;
  $("timer").textContent = clock(s.recordedMs);
  if (idle) meter.target = 0;

  renderMics(s);
  setUnlessEditing($("folder"), () => { $("folder").value = s.folder; });
  setUnlessEditing($("cleanup"), () => { $("cleanup").checked = s.noiseCleanup; });
  $("title").placeholder = s.defaultName;
  for (const id of ["folder", "browse", "title", "cleanup"]) $(id).disabled = !idle;

  $("start").disabled = !idle || s.folderProblem !== "";
  renderPause(s.state);
  $("stop").disabled = idle;

  let notice = noticeText(s.notice);
  if (idle && s.folderProblem) notice = `Can't save to this folder: ${s.folderProblem}`;
  const showingError = Date.now() < errorUntil;
  if (showingError) notice = errorText;
  $("notice").textContent = notice;
  $("notice").classList.toggle("error", showingError || (idle && s.folderProblem !== ""));

  $("last").textContent = s.file.name ? `${idle ? "Last" : "Now"} · ${s.file.name} · ${megabytes(s.file.size)}` : "";
}

async function pollStatus() {
  try {
    const res = await fetch("/api/status", { cache: "no-store" });
    if (res.ok) render(await res.json());
  } catch {
    $("notice").textContent = "Voice Recorder is not running.";
  }
  window.setTimeout(pollStatus, STATUS_MS);
}

async function pollLevel() {
  if (current && current.state !== "idle") {
    try {
      const res = await fetch("/api/level", { cache: "no-store" });
      if (res.ok) meter.target = (await res.json()).level;
    } catch {
      meter.target = 0;
    }
  }
  window.setTimeout(pollLevel, LEVEL_MS);
}

// send runs a command; the reply is the new status, or the reason it was refused.
async function send(method, path, body) {
  try {
    const res = await fetch(path, {
      method,
      headers: { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (!res.ok) {
      showError((await res.text()).trim());
      return null;
    }
    if (res.status === 204) return null;
    const data = await res.json();
    if (data.state !== undefined) render(data);
    return data;
  } catch {
    showError("Voice Recorder is not running.");
    return null;
  }
}

function saveSettings() {
  return send("PUT", "/api/settings", { folder: $("folder").value, noiseCleanup: $("cleanup").checked });
}

async function browse() {
  const picked = await send("POST", "/api/browse");
  if (picked && picked.chosen) {
    $("folder").value = picked.folder;
    await saveSettings();
  }
}

$("folder").addEventListener("change", saveSettings);
$("cleanup").addEventListener("change", saveSettings);
$("mic").addEventListener("change", () => send("PUT", "/api/mic", { id: $("mic").value }));
$("browse").addEventListener("click", browse);
$("open-folder").addEventListener("click", () => send("POST", "/api/open-folder"));
$("start").addEventListener("click", () => send("POST", "/api/start", { title: $("title").value }));
$("pause").addEventListener("click", () => send("POST", current && current.state === "paused" ? "/api/resume" : "/api/pause"));
$("stop").addEventListener("click", () => send("POST", "/api/stop"));
document.addEventListener("keydown", (e) => {
  // Space toggles recording when no field has focus: the one control used most in a meeting.
  if (e.code !== "Space" || e.target !== document.body || !current) return;
  e.preventDefault();
  if (current.state === "idle") $("start").click();
  else $("pause").click();
});

buildMeter();
window.requestAnimationFrame(drawMeter);
pollStatus();
pollLevel();
