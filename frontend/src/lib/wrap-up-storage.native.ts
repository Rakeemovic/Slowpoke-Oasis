import * as SecureStore from "expo-secure-store";

// Which finished month was dismissed is remembered per device and ledger, with
// SecureStore as the app already uses it for the color scheme.
const key = (ledgerId: string) => `wrap-up-dismissed.${ledgerId}`;

export function readDismissedMonth(ledgerId: string): string | null {
  try {
    return SecureStore.getItem(key(ledgerId));
  } catch {
    return null;
  }
}

export function storeDismissedMonth(ledgerId: string, month: string): void {
  try {
    SecureStore.setItem(key(ledgerId), month);
  } catch {
    // Not remembered; the card stays hidden until the screen is opened again.
  }
}
