import { ledgers } from "@/api/endpoints";
import { shiftDay } from "@/lib/dates";
import { hasActivity } from "@/lib/purposes";

/** For how many days of a new period the one before is pointed out. */
const VISIBLE_DAYS = 7;

/**
 * The month of the period that has just ended, if it is worth pointing out: the
 * current period is still young and something happened in the one before.
 */
export async function finishedMonth(ledgerId: string, today: string): Promise<string | undefined> {
  const current = await ledgers.purposeReport(ledgerId, today);
  // ISO dates sort like the days they name.
  if (today > shiftDay(current.from, VISIBLE_DAYS - 1)) {
    return undefined;
  }
  const finished = await ledgers.purposeReport(ledgerId, today, current.previous_month);
  return hasActivity(finished) ? finished.month : undefined;
}
