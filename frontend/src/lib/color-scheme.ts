import { colorScheme } from "nativewind";

import { readStoredScheme, storeScheme } from "@/lib/color-scheme-storage";

export type Scheme = "light" | "dark";

/** The app is dark unless the user switched to light; it does not follow the system. */
export function savedScheme(): Scheme {
  return readStoredScheme() === "light" ? "light" : "dark";
}

/** Applies the saved scheme. Called once when the app starts. */
export function applySavedScheme(): void {
  colorScheme.set(savedScheme());
}

/** Switches the scheme and remembers the choice for the next start. */
export function chooseScheme(scheme: Scheme): void {
  colorScheme.set(scheme);
  storeScheme(scheme);
}
