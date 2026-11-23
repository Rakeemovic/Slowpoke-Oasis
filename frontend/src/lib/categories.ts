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
 * The categories that can take over the bookings of a deleted one: the other
 * active categories of the same kind (rule K2). The fallback category comes
 * first, because it is the default (rule K6).
 */
export function moveTargets(categories: Category[], deleted: Category): Category[] {
  return categories
    .filter((c) => c.id !== deleted.id && !c.archived && c.is_income === deleted.is_income)
    .sort((a, b) => Number(b.is_fallback) - Number(a.is_fallback));
}

/**
 * The name the fallback category of a kind has, or gets: a ledger without one
 * receives it on its first deletion, under the default name (rule K6).
 */
export function fallbackName(categories: Category[], isIncome: boolean): string {
  const fallback = categories.find((c) => c.is_fallback && c.is_income === isIncome);
  return fallback?.name ?? (isIncome ? "Other income" : "Other");
}

/** "1 booking" or "3 bookings". */
export function bookingCount(n: number): string {
  return n === 1 ? "1 booking" : `${n} bookings`;
}
