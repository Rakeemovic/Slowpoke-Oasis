import type { Pot } from "@/api/types";

/** Every ledger has exactly one default pot (rule L4). */
export function defaultPot(pots: Pot[]): Pot | undefined {
  return pots.find((p) => p.is_default);
}

/**
 * The pots a booking can go into (rule T5: archived pots accept nothing new).
 * An archived pot stays in the list while it is the one already chosen, so
 * editing an old booking does not silently move it.
 */
export function selectablePots(pots: Pot[], chosenId?: string): Pot[] {
  return pots.filter((p) => !p.archived || p.id === chosenId);
}

/** The pot name for pickers; an archived pot is marked as such. */
export function potLabel(pot: Pot): string {
  return pot.archived ? `${pot.name} (archived)` : pot.name;
}
