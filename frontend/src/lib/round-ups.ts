import type { Pot } from "@/api/types";
import { isReserve } from "@/lib/reserves";

/** The multipliers the server accepts; 0 switches the round-up off (rule T10). */
export const roundUpMultipliers = [0, 1, 2, 3, 5] as const;

/**
 * Whether a pot may collect round-ups (rule T10): only in the personal
 * ledger, never the main pot or a reserve, and not while archived.
 */
export function canCollectRoundUps(pot: Pot, isPersonalLedger: boolean): boolean {
  return isPersonalLedger && !pot.is_default && !isReserve(pot) && !pot.archived;
}

/** The pot that collects round-ups now, if any; at most one per ledger. */
export function collectingPot(pots: Pot[]): Pot | undefined {
  return pots.find((p) => p.round_up_multiplier > 0);
}

/** The label of a multiplier button. */
export function multiplierLabel(multiplier: number): string {
  return multiplier === 0 ? "Off" : `x${multiplier}`;
}

/** The line in the pot list under a pot that collects, such as "Collects round-ups (x2)". */
export function describeRoundUp(pot: Pot): string | undefined {
  return pot.round_up_multiplier > 0 ? `Collects round-ups (x${pot.round_up_multiplier})` : undefined;
}
