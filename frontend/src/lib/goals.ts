import type { BudgetSavingsLine, Pot, SavingsGoal } from "@/api/types";
import { formatDate } from "@/lib/dates";
import { formatCents } from "@/lib/money";
import { type Progress, reachedProgress } from "@/lib/progress";
import { isReserve } from "@/lib/reserves";

/**
 * A pot with a savings goal: a target amount, a target date or a fixed
 * monthly rate. The default pot and reserves never are one (rule T7).
 */
export function isGoal(pot: Pot): boolean {
  return !pot.is_default && !isReserve(pot) && ((pot.target_cents ?? 0) > 0 || (pot.monthly_rate_cents ?? 0) > 0);
}

/** How far the pot is on its way to the target; none without a target. */
export function potProgress(pot: Pot): Progress | undefined {
  return reachedProgress(pot.balance_cents, pot.target_cents ?? null);
}

/** "Goal 500,00 € by 10.02.2027", "Goal 500,00 € · 200,00 € per month" or "Saves 200,00 € per month". */
export function describeGoal(pot: Pot): string {
  const parts: string[] = [];
  if (pot.target_cents) {
    parts.push(`Goal ${formatCents(pot.target_cents)}${pot.target_date ? ` by ${formatDate(pot.target_date)}` : ""}`);
  }
  if (pot.monthly_rate_cents) {
    const rate = `${formatCents(pot.monthly_rate_cents)} per month`;
    parts.push(pot.target_cents ? rate : `Saves ${rate}`);
  }
  return parts.join(" · ");
}

/** "By 10.02.2027 · 3 periods left · 100,00 € of 300,00 € saved": the rate of this period. */
export function describeRate(goal: SavingsGoal): string {
  const parts: string[] = [];
  if (goal.date_passed) {
    parts.push("Target date passed, the rest is due now");
  } else if (goal.target_date) {
    const periods = goal.periods === 1 ? "last period" : `${goal.periods} periods left`;
    parts.push(`By ${formatDate(goal.target_date)}`, periods);
  } else {
    parts.push("Monthly amount");
  }
  parts.push(`${formatCents(goal.saved_cents)} of ${formatCents(goal.rate_cents)} saved`);
  return parts.join(" · ");
}

/** What a savings line of the budget says next to the name, and how far it is. */
export function savingsLine(line: BudgetSavingsLine): { progress: string; status?: string; bar?: Progress } {
  // Past months only know what was moved; their rates were never stored.
  if (line.planned_cents === null) {
    return { progress: `${formatCents(line.actual_cents)} saved` };
  }
  if (line.preview) {
    return { progress: `${formatCents(line.planned_cents)} planned` };
  }
  const open = line.planned_cents - line.actual_cents;
  return {
    progress: `${formatCents(line.actual_cents)} of ${formatCents(line.planned_cents)} saved`,
    status: open > 0 ? `${formatCents(open)} to save` : "Saved",
    bar: reachedProgress(line.actual_cents, line.planned_cents),
  };
}
