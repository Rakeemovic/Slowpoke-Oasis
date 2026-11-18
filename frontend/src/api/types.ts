// Shapes of the JSON the API sends and receives. Field names match the backend.

export type Role = "owner" | "member";

export interface Household {
  id: string;
  name: string;
  role: Role;
  ledger_id: string;
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
  type: TransactionType;
  category_id?: string;
  pot_id: string;
  counterpart_pot_id?: string;
  /** Signed: income is positive, expenses are negative. */
  amount_cents: number;
  /** Date as YYYY-MM-DD. */
  booked_on: string;
  note?: string;
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
