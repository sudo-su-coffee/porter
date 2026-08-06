import { ref } from "vue";

// Live-update events the Control API emits over SSE. Any relevant event
// triggers a re-fetch of the current view — simple and correct for a
// minimal dashboard; a fuller build would patch state in place.
const EVENT_NAMES = [
  "vm.state",
  "replica.health",
  "project.progress",
  "pool.updated",
  "domain.status",
  "traffic.request",
];

// A "down" stream is expected (e.g. nothing is deploying, or the server
// briefly restarts). Reconnect automatically with exponential backoff so
// the dashboard heals itself and comes back live the moment events flow.
const BASE_DELAY = 800; // ms
const MAX_DELAY = 5000; // ms cap
const MAX_RETRIES = 6; // hard ceiling after which we keep the cap

export const connectionLive = ref(false);

let source = null;
let onEvent = () => {};
let retryAttempt = 0;
let retryTimer = null;

function delayFor(attempt) {
  const exp = BASE_DELAY * 2 ** Math.min(attempt, MAX_RETRIES);
  return Math.min(exp, MAX_DELAY);
}

function open() {
  try {
    source = new EventSource("/events");
  } catch (_) {
    scheduleReconnect();
    return;
  }
  source.onopen = () => {
    connectionLive.value = true;
    retryAttempt = 0;
  };
  source.onerror = () => {
    connectionLive.value = false;
    // The EventSource is now unusable; close it and schedule a fresh one.
    try {
      source?.close();
    } catch (_) {}
    scheduleReconnect();
  };
  for (const name of EVENT_NAMES) {
    source.addEventListener(name, () => onEvent(name));
  }
}

function scheduleReconnect() {
  if (retryTimer) return; // already waiting
  const delay = delayFor(retryAttempt);
  retryAttempt += 1;
  retryTimer = setTimeout(() => {
    retryTimer = null;
    // Explicitly disconnected (or an earlier connect already succeeded) —
    // nothing to do.
    if (source === null || connectionLive.value) return;
    open();
  }, delay);
}

export function connectEvents(cb) {
  disconnectEvents();
  onEvent = cb;
  open();
  return source;
}

export function disconnectEvents() {
  if (retryTimer) {
    clearTimeout(retryTimer);
    retryTimer = null;
  }
  retryAttempt = 0;
  if (source) {
    try {
      source.close();
    } catch (_) {}
    source = null;
  }
  connectionLive.value = false;
}
