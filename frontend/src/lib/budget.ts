import type { Budget, BudgetCategory, BudgetRecurringLine, BudgetReserveLine, Category, ReserveDue } from "@/api/types";
import { formatDate } from "@/lib/dates";
import { formatCents } from "@/lib/money";
import { type Progress, reachedProgress, spentProgress } from "@/lib/progress";

/** One category of the budget together with the payments its reserves pay this month. */
export interface BudgetGroup {
  line: BudgetCategory;
  dues: ReserveDue[];
}

/**
 * Puts every payment due from a reserve below its category. A category that
 * only has such a payment this month is missing from the budget's
 * categories, so it gets an empty line built from the category list.
 */
export function budgetGroups(budget: Budget, categories: Category[]): BudgetGroup[] {
  const groups = budget.categories.map((line) => ({ line, dues: [] as ReserveDue[] }));
  for (const due of budget.reserve_dues) {
    let group = groups.find((g) => g.line.category_id === due.category_id);
    if (!group) {
      group = { line: emptyLine(due.category_id, categories), dues: [] };
      groups.push(group);
    }
    group.dues.push(due);
  }
  return groups;
}

function emptyLine(categoryId: string, categories: Category[]): BudgetCategory {
  const category = categories.find((c) => c.id === categoryId);
  return {
    category_id: categoryId,
    name: category?.name ?? "",
    is_income: category?.is_income ?? false,
    planned_cents: 0,
    actual_cents: 0,
    difference_cents: 0,
    recurring: [],
    reserves: [],
  };
}

export interface BudgetTotals {
  plannedExpenses: number;
  spent: number;
  plannedIncome: number;
  received: number;
  hasIncome: boolean;
}

/**
 * Sums the whole month of the main pot: the free plan, the monthly payments
 * and the reserve rates count together, as they all weigh on the month.
 */
export function budgetTotals(budget: Budget): BudgetTotals {
  const totals: BudgetTotals = { plannedExpenses: 0, spent: 0, plannedIncome: 0, received: 0, hasIncome: false };
  for (const c of budget.categories) {
    if (c.is_income) {
      totals.hasIncome = true;
      totals.plannedIncome += c.planned_cents;
      totals.received += c.actual_cents;
      continue;
    }
    totals.plannedExpenses += c.planned_cents;
    totals.spent += c.actual_cents;
    for (const r of c.recurring) {
      totals.plannedExpenses += r.planned_cents;
      totals.spent += r.actual_cents;
    }
    for (const r of c.reserves) {
      totals.plannedExpenses += r.planned_cents ?? 0;
      totals.spent += r.actual_cents;
    }
  }
  return totals;
}

/** The two texts of a line below a category, and whether the status is a warning. */
export interface SubLineText {
  progress: string;
  status?: string;
  warn?: boolean;
  /** How far the line is; none when it has no amount to measure against. */
  bar?: Progress;
}

export function recurringText(line: BudgetRecurringLine): SubLineText {
  const due = `due ${formatDate(line.due_on)}`;
  if (line.skipped) {
    return { progress: due, status: "Skipped" };
  }
  // A booking in the main pot of a payment whose target lies elsewhere,
  // e.g. one that is normally paid from another pot.
  if (line.planned_cents === 0) {
    return { progress: `${formatCents(line.actual_cents)}, not planned` };
  }
  const progress = `${formatCents(line.actual_cents)} of ${formatCents(line.planned_cents)} · ${due}`;
  const bar = spentProgress(line.actual_cents, line.planned_cents);
  if (line.actual_cents === 0) {
    return { progress, status: "Open", bar };
  }
  if (line.difference_cents < 0) {
    return { progress, status: `${formatCents(-line.difference_cents)} over`, warn: true, bar };
  }
  if (line.difference_cents === 0) {
    return { progress, status: "Paid", bar };
  }
  return { progress, status: `${formatCents(line.difference_cents)} left`, bar };
}

export function reserveText(line: BudgetReserveLine): SubLineText {
  // Past months only know what was moved; their rates were never stored.
  if (line.planned_cents === null) {
    return { progress: `${formatCents(line.actual_cents)} saved` };
  }
  const forDue = line.due_on ? ` · for ${formatDate(line.due_on)}` : "";
  const progress = `${formatCents(line.actual_cents)} of ${formatCents(line.planned_cents)} saved${forDue}`;
  const open = line.planned_cents - line.actual_cents;
  return {
    progress,
    status: open > 0 ? `${formatCents(open)} to save` : "Saved",
    bar: reachedProgress(line.actual_cents, line.planned_cents),
  };
}

/** “120,00 € due 15.2.2027 from reserve “Car tax””, or that it was paid. */
export function reserveDueText(due: ReserveDue): string {
  const amount = formatCents(due.amount_cents);
  if (due.booked) {
    return `${amount} paid from reserve “${due.name}” (due ${formatDate(due.due_on)})`;
  }
  return `${amount} due ${formatDate(due.due_on)} from reserve “${due.name}” (covered: ${formatCents(due.covered_cents)})`;
}
