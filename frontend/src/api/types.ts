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

/** What kind of spending an expense category stands for (rule K5). Income categories have none. */
export type Purpose = "need" | "want" | "saving";

export interface Category {
  id: string;
  name: string;
  is_income: boolean;
  /** Archived categories accept no new bookings (rule K8). */
  archived: boolean;
  purpose: Purpose | null;
  /** "Other" or "Other income": takes over the bookings of deleted categories and cannot be deleted (rule K6). */
  is_fallback: boolean;
  /** Bookings that are not deleted. */
  transaction_count: number;
}

export interface NewCategory {
  name: string;
  is_income: boolean;
  /** Only for expense categories; empty for none. */
  purpose?: Purpose | "";
}

/** Owner only. Fields left out stay unchanged; an empty purpose removes it. */
export interface CategoryChange {
  name?: string;
  purpose?: Purpose | "";
  archived?: boolean;
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
  /** An unplanned expense, marked by its author (rule X20). */
  is_spontaneous: boolean;
  /** The occurrence of a recurring payment this expense pays (rule R6). */
  recurring_payment_id?: string;
  recurring_due_on?: string;
  /** Only for transfers between ledgers (rule X17). */
  counterpart?: Counterpart;
  /** What the signed-in user may do with it (rule X19). The server checks again. */
  can_edit: boolean;
  can_delete: boolean;
}

/** A soft-deleted transaction. It has no can_* flags: the server checks the right when restoring. */
export interface DeletedTransaction extends Omit<Transaction, "can_edit" | "can_delete"> {
  deleted_at: string;
  /** Who deleted it; null for a member who has left or deleted their account (rule X18). */
  deleted_by_username: string | null;
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
  pot_id?: string;
  /** Always positive; the type decides the direction. */
  amount_cents: number;
  booked_on: string;
  note?: string;
  /** Only for expenses (rule X20). */
  is_spontaneous?: boolean;
  /** Books one occurrence of a recurring payment; both or neither (rule R6). */
  recurring_payment_id?: string;
  recurring_due_on?: string;
  /**
   * For an expense from a reserve: moves what the reserve lacks from the
   * default pot first, in the same step (rule R13).
   */
  top_up?: boolean;
}

/**
 * The fields of an income or expense that can be changed. Every field is sent,
 * so an empty note removes the old one.
 */
export interface TransactionChange {
  type: "expense" | "income";
  category_id: string;
  pot_id: string;
  /** Always positive; the type decides the direction. */
  amount_cents: number;
  booked_on: string;
  note: string;
  /**
   * Only the author may send it (rule X20); left out, the mark stays. The
   * form leaves it out for bookings of others, so an owner can still edit them.
   */
  is_spontaneous?: boolean;
  /**
   * Which bookings of the same recurring payment get the new category too
   * (rule R9). Left out, only this booking changes.
   */
  category_scope?: CategoryScope;
}

/** "this_and_later" and "all" also change the recurring payment; owner only. */
export type CategoryScope = "this" | "this_and_earlier" | "this_and_later" | "all";

/** Balance of a ledger on one day (rule L7). Planned transactions after as_of do not count. */
export interface Balance {
  as_of: string;
  /** The default pot: money that is not set aside in another pot. */
  available_cents: number;
  /** All pots together. */
  total_cents: number;
  /** Name of the default pot, usually "Main". */
  default_pot_name: string;
  /** All pots minus the unbooked payments: what the bank account most likely shows. */
  estimated_cents: number;
  /** Recurring payments due or overdue on as_of that are neither booked nor skipped. */
  unbooked_count: number;
  unbooked_cents: number;
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

export interface Pot {
  id: string;
  name: string;
  /** The default pot is "monthly"; pots created in the app are "one_time". */
  kind: "monthly" | "one_time";
  is_default: boolean;
  archived: boolean;
  /** Savings goal in cents; left out when the pot has none. */
  target_cents?: number;
  /** Goal date as YYYY-MM-DD; only present together with target_cents. */
  target_date?: string;
  /** Calculated by the server from all bookings, never stored. */
  balance_cents: number;
  /** The recurring payment this pot is the reserve of (rule R10), or null. */
  reserve_for: string | null;
}

/**
 * Which months a change to the plan of a monthly pot affects (rule P5):
 * only the current month, or also the template for the months to come.
 */
export type PlanScope = "this_month" | "from_now_on";

/**
 * Where the planned amounts of a summary come from: "pot" (one-time pot),
 * "month" (stored plan of the month), "preview" (future month, from the
 * template) or "none" (past month without a plan).
 */
export type PlanSource = "pot" | "month" | "preview" | "none";

/** Plan versus actual of one category. Only categories with a plan or a booking appear. */
export interface PlanLine {
  category_id: string;
  name: string;
  is_income: boolean;
  /** 0 when the category is not planned. */
  planned_cents: number;
  /** Spent (expenses) or received (income), always positive. */
  actual_cents: number;
  /** What is left of the plan: planned − spent for expenses, received − planned for income. */
  difference_cents: number;
}

export interface PotSummary {
  pot_id: string;
  /** YYYY-MM for monthly pots, empty for one-time pots. */
  month: string;
  plan_source: PlanSource;
  /** First and last day covered, YYYY-MM-DD; empty when not limited. */
  from: string;
  to: string;
  lines: PlanLine[];
  transfers_in_cents: number;
  transfers_out_cents: number;
}

/** What a new pot needs. A target of 0 or a missing one means no goal. */
export interface NewPot {
  name: string;
  target_cents?: number;
  target_date?: string;
}

/** Every field is sent. Sending target_cents: 0 removes the goal and its date. */
export interface PotChange {
  name: string;
  target_cents: number;
  target_date: string;
  archived: boolean;
}

/** Money moved from one pot to another pot of the same ledger. */
export interface NewPotTransfer {
  type: "transfer";
  /** The pot the money leaves. */
  pot_id: string;
  /** The pot the money arrives in. */
  counterpart_pot_id: string;
  /** Always positive. */
  amount_cents: number;
  booked_on: string;
  note?: string;
}

/** How often a recurring payment is due, in months (rule R3). */
export type IntervalMonths = 1 | 3 | 6 | 12;

/** A reminder of a fixed expense; it is never booked automatically. */
export interface RecurringPayment {
  id: string;
  name: string;
  /** Always positive. */
  amount_cents: number;
  category_id: string;
  pot_id: string;
  interval_months: IntervalMonths;
  /** 1–31; a month without that day uses its last day. */
  due_day: number;
  /** YYYY-MM */
  first_month: string;
  /** YYYY-MM, or null for no end. */
  last_month: string | null;
  archived: boolean;
  /** The next due date from today on, booked or not; null when it has ended. */
  next_due_on: string | null;
  /**
   * The reserve pot of a payment due every few months (rule R10). It stays
   * linked when the payment is archived or becomes monthly; null if none.
   */
  reserve_pot_id: string | null;
}

/** The fields of a recurring payment that the form edits. */
export interface RecurringPaymentInput {
  name: string;
  amount_cents: number;
  category_id: string;
  pot_id?: string;
  interval_months: IntervalMonths;
  due_day: number;
  first_month: string;
  /** Empty removes the end when editing. */
  last_month: string;
}

/** What the owner may change: the form fields and the archive flag. */
export type RecurringPaymentChange = Partial<RecurringPaymentInput> & { archived?: boolean };

/** One open due date of a recurring payment: neither booked nor skipped. */
export interface Occurrence {
  recurring_payment_id: string;
  name: string;
  amount_cents: number;
  category_id: string;
  pot_id: string;
  /** YYYY-MM-DD */
  due_on: string;
  status: "overdue" | "due" | "upcoming";
}

/** What a reserve should receive in the current payday period (rule R14). */
export interface ReserveRate {
  recurring_payment_id: string;
  name: string;
  /** The amount of the payment. */
  amount_cents: number;
  /** The reserve pot. */
  pot_id: string;
  /** The due date the reserve saves for, YYYY-MM-DD. */
  due_on: string;
  /** Payday periods until due_on, the current one included. */
  periods: number;
  balance_cents: number;
  rate_cents: number;
  /** Moved into the reserve in this period, as far as it is still there. */
  saved_cents: number;
  /** The part of the rate not saved yet; never below zero. */
  open_cents: number;
}
