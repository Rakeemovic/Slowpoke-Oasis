import { CircleHelp, Gift, House, Sprout, type LucideIcon } from "lucide-react-native";

import type { PurposeKey, PurposeLine, PurposeReport } from "@/api/types";
import { monthName } from "@/lib/dates";
import { type Hue, NEUTRAL_HUE } from "@/lib/hues";
import { formatCents } from "@/lib/money";
import type { Progress } from "@/lib/progress";

/** How a purpose looks: its name, its icon, its colour and the colour of the bar behind its row. */
export interface PurposeLook {
  name: string;
  icon: LucideIcon;
  hue: Hue;
  /** The fill of the row's progress bar; complete class names, as for hues. */
  fill: string;
}

/**
 * The colours come from the app's own tokens, not from the hue palette:
 * needs wear the primary colour, wants the warning colour and savings the
 * positive one, so each purpose looks the same on every screen.
 */
export const purposeLook: Record<PurposeKey, PurposeLook> = {
  need: {
    name: "Needs",
    icon: House,
    hue: { tile: "bg-primary/15", icon: "text-primary", fill: "bg-primary" },
    fill: "bg-primary/15",
  },
  want: {
    name: "Wants",
    icon: Gift,
    hue: { tile: "bg-warning/15", icon: "text-warning", fill: "bg-warning" },
    fill: "bg-warning/15",
  },
  saving: {
    name: "Savings",
    icon: Sprout,
    hue: { tile: "bg-positive/15", icon: "text-positive", fill: "bg-positive" },
    fill: "bg-positive/15",
  },
  none: {
    name: "No purpose",
    icon: CircleHelp,
    hue: NEUTRAL_HUE,
    fill: "bg-muted-foreground/15",
  },
};

/** The line "46 % of income", or nothing when no income is booked (then there is no share). */
export function shareText(line: PurposeLine): string | undefined {
  return line.share_percent === null ? undefined : `${line.share_percent} % of income`;
}

/** The row's own bar: the share of the income, which can never fill more than the row. */
export function shareProgress(line: PurposeLine): Progress | undefined {
  return line.share_percent === null ? undefined : { fraction: Math.min(1, line.share_percent / 100), tone: "default" };
}

/** Which way a line moved compared with the period before. */
export interface Trend {
  direction: "up" | "down" | "same";
  text: string;
}

/**
 * Compares a line with the period before. Neutral on purpose: more spent on
 * needs is no bad news in itself, so there is only a direction and an amount.
 * Nothing in both periods needs no line.
 */
export function trendOf(line: PurposeLine, previousMonth: string): Trend | undefined {
  if (line.cents === 0 && line.previous_cents === 0) {
    return undefined;
  }
  const before = monthName(previousMonth);
  const difference = line.cents - line.previous_cents;
  if (difference === 0) {
    return { direction: "same", text: `Same as in ${before}` };
  }
  const amount = formatCents(Math.abs(difference));
  return difference > 0
    ? { direction: "up", text: `${amount} more than in ${before}` }
    : { direction: "down", text: `${amount} less than in ${before}` };
}

/** How much of the wants was spontaneous; shown for wants only, where it is worth a look. */
export function spontaneousText(line: PurposeLine): string | undefined {
  if (line.purpose !== "want" || line.spontaneous_cents === 0 || line.cents === 0) {
    return undefined;
  }
  const part = Math.round((line.spontaneous_cents * 100) / line.cents);
  return `${formatCents(line.spontaneous_cents)} spontaneous (${part} % of wants)`;
}

/** The part of the savings that moved into pots, which the category total alone would not show. */
export function transferText(line: PurposeLine): string | undefined {
  return line.transfer_cents > 0 ? `incl. ${formatCents(line.transfer_cents)} moved into pots` : undefined;
}

/** Whether the period has nothing to show: no income, no spending, no savings, here and before. */
export function isEmptyReport(report: PurposeReport): boolean {
  return report.income_cents === 0 && report.lines.every((l) => l.cents === 0 && l.previous_cents === 0);
}
