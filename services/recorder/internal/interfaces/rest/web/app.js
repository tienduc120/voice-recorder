"use strict";

// The page polls; the recorder keeps running without it, so a reopened tab simply catches up.
const POLL_MS = 500;
const ERROR_SHOWN_MS = 8000;
const METER_CELLS = 10;

const $ = (id) => document.getElementById(id);

const STATE_LABELS = {
  "idle": "Idle",
  "recording": "● REC",
  "paused": "❚❚ Paused",
  "waiting-for-mic": "Waiting for a mic",
};

let current = null;
let micsShown = "";
let errorText = "";
let errorUntil = 0;
let poller = 0;

function noticeText(n) {
  switch (n.kind) {
    case "mic-connected":
      return `"${n.mic}" connected — choose it above to switch`;
    case "mic-fallback":
      return `"${n.previousMic}" disconnected — now recording from "${n.mic}"`;
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

function showError(text) {
  errorText = text;
  errorUntil = Date.now() + ERROR_SHOWN_MS;
  if (current) render(current);
}

// setUnlessEditing leaves a field the owner is typing in alone.
function setUnlessEditing(el, apply) {
  if (document.activeElement !== el) apply();
}

function renderMics(s) {
  const shown = JSON.stringify(s.mics);
  const select = $("mic");
  if (shown !== micsShown) {
    micsShown = shown;
    select.replaceChildren(...s.mics.map((m) => {
      const option = document.createElement("option");
      option.value = m.id;
      option.textContent = m.isDefault ? `${m.name} (Windows default)` : m.name;
      return option;
    }));
  }
  setUnlessEditing(select, () => { select.value = s.micId; });
}

function renderMeter(level) {
  const meter = $("level");
  if (meter.children.length !== METER_CELLS) {
    meter.replaceChildren(...Array.from({ length: METER_CELLS }, () => document.createElement("span")));
  }
  const lit = Math.round(level * METER_CELLS);
  Array.from(meter.children).forEach((cell, i) => { cell.className = i < lit ? "on" : ""; });
  meter.setAttribute("aria-valuenow", String(lit));
}

function render(s) {
  current = s;
  const idle = s.state === "idle";
  const active = !idle;

  $("state").textContent = STATE_LABELS[s.state] || s.state;
  $("state").className = s.state;
  $("timer").textContent = clock(s.recordedMs);
  renderMeter(s.level);
  renderMics(s);

  setUnlessEditing($("folder"), () => { $("folder").value = s.folder; });
  setUnlessEditing($("cleanup"), () => { $("cleanup").checked = s.noiseCleanup; });
  for (const id of ["folder", "browse", "title", "cleanup"]) $(id).disabled = active;

  $("start").disabled = !idle || s.folderProblem !== "";
  $("pause").disabled = idle;
  $("pause").textContent = s.state === "paused" ? "▶ Resume" : "❚❚ Pause";
  $("stop").disabled = idle;

  let notice = noticeText(s.notice);
  if (idle && s.folderProblem) notice = `Can't save to this folder: ${s.folderProblem}`;
  if (Date.now() < errorUntil) notice = errorText;
  $("notice").textContent = notice;

  $("last").textContent = s.file.name ? `Last: ${s.file.name} (${(s.file.size / 1e6).toFixed(1)} MB)` : "";
}

async function poll() {
  try {
    const res = await fetch("/api/status", { cache: "no-store" });
    if (res.ok) render(await res.json());
  } catch {
    $("notice").textContent = "Voice Recorder is not running.";
  }
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

async function quit() {
  const busy = current && current.state !== "idle";
  if (busy && !window.confirm("A recording is in progress. Stop it, save it and quit?")) return;
  const res = await fetch("/api/quit", { method: "POST" }).catch(() => null);
  if (res && res.status === 204) {
    window.clearInterval(poller);
    document.body.replaceChildren(document.createTextNode("Voice Recorder has quit. You can close this tab."));
  } else {
    showError("Could not quit.");
  }
}

$("folder").addEventListener("change", saveSettings);
$("cleanup").addEventListener("change", saveSettings);
$("mic").addEventListener("change", () => send("PUT", "/api/mic", { id: $("mic").value }));
$("browse").addEventListener("click", browse);
$("start").addEventListener("click", () => send("POST", "/api/start", { title: $("title").value }));
$("pause").addEventListener("click", () => send("POST", current && current.state === "paused" ? "/api/resume" : "/api/pause"));
$("stop").addEventListener("click", () => send("POST", "/api/stop"));
$("quit").addEventListener("click", quit);

poll();
poller = window.setInterval(poll, POLL_MS);
