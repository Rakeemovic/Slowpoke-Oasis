import { auth, households } from "@/api/endpoints";

/** A ledger the user can open, as shown in lists. */
export interface LedgerSummary {
  id: string;
  name: string;
  kind: "personal" | "household";
  /** A short line under the name, such as the user's role in the household. */
  description: string;
}

/** The user's personal ledger first, then the ledgers of all their households. */
export async function loadMyLedgers(): Promise<LedgerSummary[]> {
  const [me, list] = await Promise.all([auth.me(), households.list()]);
  const personal: LedgerSummary[] = me.personal_ledger_id
    ? [{ id: me.personal_ledger_id, name: "Personal", kind: "personal", description: "Only you can see this ledger" }]
    : [];
  return [
    ...personal,
    ...list.map((h) => ({
      id: h.ledger_id,
      name: h.name,
      kind: "household" as const,
      description: h.role === "owner" ? "Owner" : "Member",
    })),
  ];
}
