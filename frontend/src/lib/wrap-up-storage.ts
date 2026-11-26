// Which finished month was dismissed is remembered per device and ledger. In
// the browser that is localStorage; if it is blocked, the card just comes back.
const key = (ledgerId: string) => `wrap-up-dismissed.${ledgerId}`;

export function readDismissedMonth(ledgerId: string): string | null {
  try {
    return globalThis.localStorage?.getItem(key(ledgerId)) ?? null;
  } catch {
    return null;
  }
}

export function storeDismissedMonth(ledgerId: string, month: string): void {
  try {
    globalThis.localStorage?.setItem(key(ledgerId), month);
  } catch {
    // Not remembered; the card stays hidden until the screen is opened again.
  }
}
