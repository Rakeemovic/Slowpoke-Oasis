import { isIsoDate } from "@/lib/dates";

/**
 * Checks the fields every booking has. Returns a message for the first invalid
 * one, or undefined if both are fine. cents comes from parseAmountToCents.
 */
export function bookingProblem(cents: number | null, bookedOn: string): string | undefined {
  if (cents === null) {
    return "Enter an amount greater than zero, with at most two decimal places.";
  }
  if (!isIsoDate(bookedOn)) {
    return "Enter the date as YYYY-MM-DD.";
  }
  return undefined;
}
