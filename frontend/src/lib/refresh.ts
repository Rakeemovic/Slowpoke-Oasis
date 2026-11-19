// A signal that asks the visible screen to load its data again, so changes by
// other household members show up without reloading the whole app.

type Listener = () => void;

const listeners = new Set<Listener>();

// Switching back to a browser tab fires both "visible" and "focus"; one reload is enough.
const MIN_INTERVAL_MS = 1000;
let lastRequest = 0;

/** Registers a listener and returns the function that removes it again. */
export function onRefreshRequest(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/** Asks every listening loader to reload. Calls in quick succession count once. */
export function requestRefresh() {
  const now = Date.now();
  if (now - lastRequest < MIN_INTERVAL_MS) {
    return;
  }
  lastRequest = now;
  listeners.forEach((listener) => listener());
}
