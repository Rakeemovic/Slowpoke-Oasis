// Amounts are handled as whole cents, like in the backend, to avoid the
// rounding errors of floating-point numbers (0.1 + 0.2 !== 0.3).

const AMOUNT_PATTERN = /^(\d+)(?:[.,](\d{1,2}))?$/;

/**
 * Reads an amount typed by the user, such as "12", "12.5" or "12,50", and
 * returns it in cents. Returns null for anything else, including zero.
 */
export function parseAmountToCents(input: string): number | null {
  const match = AMOUNT_PATTERN.exec(input.trim());
  if (!match) {
    return null;
  }
  const euros = Number(match[1]);
  const cents = Number((match[2] ?? "").padEnd(2, "0"));
  const total = euros * 100 + cents;
  return total > 0 && Number.isSafeInteger(total) ? total : null;
}

// The MVP only supports EUR (rule L3).
const currencyFormat = new Intl.NumberFormat(undefined, { style: "currency", currency: "EUR" });

export function formatCents(cents: number): string {
  return currencyFormat.format(cents / 100);
}

/** Like formatCents, but positive amounts get a "+" so income stands out. */
export function formatSignedCents(cents: number): string {
  return cents > 0 ? `+${formatCents(cents)}` : formatCents(cents);
}
