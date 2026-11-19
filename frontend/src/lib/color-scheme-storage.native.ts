import * as SecureStore from "expo-secure-store";

// On Android the chosen color scheme is kept with SecureStore, which the app
// already uses for the login token. Its synchronous API lets the saved scheme
// apply before the first screen is drawn, so the app does not flash.
const KEY = "color-scheme";

export function readStoredScheme(): string | null {
  try {
    return SecureStore.getItem(KEY);
  } catch {
    return null;
  }
}

export function storeScheme(value: string): void {
  try {
    SecureStore.setItem(KEY, value);
  } catch {
    // Not remembered; the choice still applies until the app restarts.
  }
}
