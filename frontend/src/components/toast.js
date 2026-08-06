import { reactive } from "vue";

let nextId = 1;
export const toasts = reactive([]);

export function toast(message) {
  const id = nextId++;
  toasts.push({ id, message });
  setTimeout(() => {
    const idx = toasts.findIndex((t) => t.id === id);
    if (idx !== -1) toasts.splice(idx, 1);
  }, 3500);
}
