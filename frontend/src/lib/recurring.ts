import type { IntervalMonths, Occurrence, RecurringPayment } from "@/api/types";
import { formatDate, formatMonth, today } from "@/lib/dates";

/** The intervals a recurring payment can have (rule R3), with their labels. */
export const intervalLabels: Record<IntervalMonths, string> = {
  1: "Monthly",
  3: "Every 3 months",
  6: "Every 6 months",
  12: "Yearly",
};

export const intervalChoices = (Object.keys(intervalLabels) as `${IntervalMonths}`[]).map((months) => ({
  id: months,
  label: intervalLabels[Number(months) as IntervalMonths],
}));

/** The current calendar month as YYYY-MM, the default first month of a new payment. */
export function currentMonth(): string {
  return today().slice(0, 7);
}

export function isMonth(value: string): boolean {
  return /^\d{4}-(0[1-9]|1[0-2])$/.test(value);
}

/** "Monthly on day 3 · until March 2027" */
export function describeSchedule(p: RecurringPayment): string {
  const parts = [`${intervalLabels[p.interval_months]} on day ${p.due_day}`];
  if (p.last_month) {
    parts.push(`until ${formatMonth(p.last_month)}`);
  }
  return parts.join(" · ");
}

/** "Overdue since 20.11.2026", "Due today" or "Due 25.11.2026". */
export function describeOccurrence(o: Occurrence): string {
  switch (o.status) {
    case "overdue":
      return `Overdue since ${formatDate(o.due_on)}`;
    case "due":
      return "Due today";
    default:
      return `Due ${formatDate(o.due_on)}`;
  }
}
