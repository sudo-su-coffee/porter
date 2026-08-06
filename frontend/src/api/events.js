import { ref } from "vue";

// Live-update events the Control API emits over SSE (see ARCHITECTURE.md
// §2, Events). Any relevant event triggers a re-fetch of the current
// view — simple and correct for a minimal dashboard; a fuller build
// would patch state in place instead of refetching.
const EVENT_NAMES = [
  "vm.state",
  "replica.health",
  "project.progress",
  "pool.updated",
  "domain.status",
  "traffic.request",
];

export const connectionLive = ref(false);

let source = null;

export function connectEvents(onEvent) {
  disconnectEvents();
  source = new EventSource("/events");
  source.onopen = () => (connectionLive.value = true);
  source.onerror = () => (connectionLive.value = false);

  for (const name of EVENT_NAMES) {
    source.addEventListener(name, () => onEvent(name));
  }
  return source;
}

export function disconnectEvents() {
  if (source) {
    source.close();
    source = null;
  }
  connectionLive.value = false;
}
