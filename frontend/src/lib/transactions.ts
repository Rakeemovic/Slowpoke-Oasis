import type { Category, Counterpart, Transaction } from "@/api/types";

/** The heading of a transaction: its category, or what kind of transfer it is. */
export function transactionTitle(tx: Transaction, categories: Category[]): string {
  return categories.find((c) => c.id === tx.category_id)?.name ?? describeType(tx.type, tx.counterpart);
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
