// In the browser the chosen color scheme lives in localStorage. Storage can be
// blocked (private mode, disabled site data); then the app simply starts dark.
const KEY = "color-scheme";

export function readStoredScheme(): string | null {
  try {
    return globalThis.localStorage?.getItem(KEY) ?? null;
  } catch {
    return null;
  }
}

export function storeScheme(value: string): void {
  try {
    globalThis.localStorage?.setItem(KEY, value);
  } catch {
    // Not remembered; the choice still applies until the page is reloaded.
  }
}
