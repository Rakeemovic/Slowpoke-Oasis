import { ApiError } from "@/api/client";
import { pots } from "@/api/endpoints";
import type { NewTransaction, Pot, ReserveRate } from "@/api/types";
import { formatDate } from "@/lib/dates";
import { formatCents } from "@/lib/money";

/** A reserve belongs to a recurring payment due every few months (rule R10). */
export function isReserve(pot: Pot): boolean {
  return pot.reserve_for !== null;
}

/** "For 01.01.2027 · 3 periods left · 200,00 € of 400,00 € saved" */
export function describeRate(rate: ReserveRate): string {
  const periods = rate.periods === 1 ? "last period" : `${rate.periods} periods left`;
  return `For ${formatDate(rate.due_on)} · ${periods} · ${formatCents(rate.saved_cents)} of ${formatCents(rate.rate_cents)} saved`;
}

/**
 * After a booking failed, finds out whether it failed because the reserve it
 * comes from holds too little (rule R13). Returns the missing amount then, so
 * the app can offer to top the reserve up; otherwise undefined.
 *
 * The status alone is not enough: other conflicts also answer 409. The pot
 * list tells whether the pot is a reserve and how much it lacks.
 */
export async function reserveShortfall(ledgerId: string, tx: NewTransaction, error: unknown): Promise<number | undefined> {
  if (!(error instanceof ApiError) || error.status !== 409 || tx.type !== "expense" || !tx.pot_id) {
    return undefined;
  }
  const pot = (await pots.list(ledgerId)).find((p) => p.id === tx.pot_id);
  if (!pot || !isReserve(pot) || pot.archived) {
    return undefined;
  }
  const missing = tx.amount_cents - pot.balance_cents;
  return missing > 0 ? missing : undefined;
}
