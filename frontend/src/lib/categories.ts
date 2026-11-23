import type { Category, Purpose } from "@/api/types";

/** The labels the app shows for each purpose (rule K5). */
export const purposeLabels: Record<Purpose, string> = {
  need: "Need",
  want: "Want",
  saving: "Saving",
};

/** Choices for the purpose picker; "none" stands for no purpose. */
export const purposeChoices = [
  { id: "none", label: "None" },
  ...(Object.keys(purposeLabels) as Purpose[]).map((p) => ({ id: p, label: purposeLabels[p] })),
];

/** Turns a picker choice back into the value the API expects. */
export function purposeFromChoice(id: string): Purpose | "" {
  return id === "none" ? "" : (id as Purpose);
}

/**
 * The name of the category that takes over the bookings of a deleted one
 * (rule K6). A ledger without one gets it on the first deletion, under the
 * default name.
 */
export function fallbackName(categories: Category[], isIncome: boolean): string {
  const fallback = categories.find((c) => c.is_fallback && c.is_income === isIncome);
  return fallback?.name ?? (isIncome ? "Other income" : "Other");
}

/** "1 booking" or "3 bookings". */
export function bookingCount(n: number): string {
  return n === 1 ? "1 booking" : `${n} bookings`;
}
