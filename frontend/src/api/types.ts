// Shapes of the JSON the API sends and receives. Field names match the backend.

export type Role = "owner" | "member";

/** The signed-in user (GET /me). The email address is never sent. */
export interface Me {
  user_id: string;
  username: string;
  display_name: string;
  /** Created at registration; null only for accounts older than that rule. */
  personal_ledger_id: string | null;
}

export interface Household {
  id: string;
  name: string;
  role: Role;
  ledger_id: string;
}

/** A household member. Other members only ever see the username, never the email. */
export interface Member {
  user_id: string;
  username: string;
  role: Role;
}

export interface Category {
  id: string;
  name: string;
  is_income: boolean;
  archived: boolean;
}

export type TransactionType = "expense" | "income" | "transfer" | "transfer_out" | "transfer_in";

export interface Transaction {
  id: string;
  ledger_id: string;
  user_id: string;
  /** Who booked it; null for a member who has left or deleted their account (rule X18). */
  username: string | null;
  type: TransactionType;
  category_id?: string;
  pot_id: string;
  counterpart_pot_id?: string;
  /** Signed: income is positive, expenses are negative. */
  amount_cents: number;
  /** Date as YYYY-MM-DD. */
  booked_on: string;
  note?: string;
  /** Only for transfers between ledgers (rule X17). */
  counterpart?: Counterpart;
}

/**
 * The other ledger of a transfer. ledger_id and name are missing when the user
 * cannot open that ledger, e.g. another member's personal ledger.
 */
export interface Counterpart {
  kind: "personal" | "household";
  ledger_id?: string;
  name?: string;
}

/** A new income or expense. Without pot_id it goes into the default pot. */
export interface NewTransaction {
  type: "expense" | "income";
  category_id: string;
  /** Always positive; the type decides the direction. */
  amount_cents: number;
  booked_on: string;
  note?: string;
}

/** Balance of a ledger on one day (rule L7). Planned transactions after as_of do not count. */
export interface Balance {
  as_of: string;
  /** The default pot: money that is not set aside in another pot. */
  available_cents: number;
  /** All pots together. */
  total_cents: number;
}

/** A transfer between two ledgers. Without pot IDs, both sides use the default pot. */
export interface NewLedgerTransfer {
  from_ledger_id: string;
  to_ledger_id: string;
  /** Always positive. */
  amount_cents: number;
  booked_on: string;
  note?: string;
}
