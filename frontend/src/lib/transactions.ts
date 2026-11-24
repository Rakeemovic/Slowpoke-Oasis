import type { Category, Counterpart, Pot, Transaction } from "@/api/types";

/** The heading of a transaction: its category, or what kind of transfer it is. */
export function transactionTitle(
  tx: Pick<Transaction, "category_id" | "type" | "counterpart" | "round_up_of">,
  categories: Category[],
): string {
  if (tx.round_up_of) {
    return "Round-up";
  }
  return categories.find((c) => c.id === tx.category_id)?.name ?? describeType(tx.type, tx.counterpart);
}

/**
 * Which pot a transaction belongs to, for the list. A transfer between pots
 * reads "from → to" (money leaves pot_id and enters counterpart_pot_id).
 * With a single pot the name says nothing new, so there is none.
 */
export function potSummary(tx: Pick<Transaction, "type" | "pot_id" | "counterpart_pot_id">, pots: Pot[]): string | undefined {
  if (pots.length < 2) {
    return undefined;
  }
  const name = (id?: string) => pots.find((p) => p.id === id)?.name;
  const from = name(tx.pot_id);
  if (tx.type === "transfer") {
    const to = name(tx.counterpart_pot_id);
    return from && to ? `${from} → ${to}` : undefined;
  }
  return from;
}

/** Transfers between ledgers can only be deleted, never edited (rule X14). */
export function isLinkedTransfer(tx: Transaction): boolean {
  return tx.type === "transfer_out" || tx.type === "transfer_in";
}

function describeType(type: Transaction["type"], counterpart?: Counterpart): string {
  switch (type) {
    case "transfer":
      return "Transfer between pots";
    case "transfer_out":
      return `Transfer to ${describeLedger(counterpart)}`;
    case "transfer_in":
      return `Transfer from ${describeLedger(counterpart)}`;
    default:
      return "Uncategorised";
  }
}

/** The other ledger by name, or only its kind if the user cannot open it. */
function describeLedger(counterpart?: Counterpart): string {
  if (counterpart?.name) {
    return counterpart.name;
  }
  if (counterpart?.kind === "personal") {
    return "a personal ledger";
  }
  if (counterpart?.kind === "household") {
    return "a household";
  }
  // The other ledger was deleted (rule X16).
  return "a deleted ledger";
}
