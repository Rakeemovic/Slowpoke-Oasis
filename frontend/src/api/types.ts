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

/** "correction" aligns the ledger with the bank after a reconciliation (rule L9); it can only be deleted. */
export type TransactionType = "expense" | "income" | "transfer" | "transfer_out" | "transfer_in" | "correction";

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
  /** Set on the transfer that rounds up this expense (rule X21); it only changes together with the expense. */
  round_up_of?: string;
  /** On an expense: the amount of its round-up, which is deleted with it (rule X21). */
  round_up_cents?: number;
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
  default_pot_id: string;
  /** The last reconciliation of the bank account, or null. */
  reconciled: Reconciliation | null;
  /** All pots minus the unbooked payments: what the bank account most likely shows. */
  estimated_cents: number;
  /** Recurring payments due or overdue on as_of that are neither booked nor skipped. */
  unbooked_count: number;
  unbooked_cents: number;
  /** Pots that are real sub-accounts at the bank (rule T9); their money is not in estimated_cents. */
  separate_accounts: { pot_id: string; name: string; balance_cents: number; reconciled: Reconciliation | null }[];
  separate_cents: number;
}

/** The last time the user compared an account with the bank (rule L9). */
export interface Reconciliation {
  reconciled_on: string;
  actual_cents: number;
}

/** Compares the real balance of the bank account (no pot_id) or of a separate account. */
export interface ReconcileInput {
  actual_cents: number;
  pot_id?: string;
  /** Books the difference as a correction. */
  correct: boolean;
}

export interface ReconcileResult {
  pot_id: string;
  expected_cents: number;
  actual_cents: number;
  difference_cents: number;
  correction_id?: string;
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
  /** Fixed amount the pot saves per payday period (rule T6); never together with target_date. */
  monthly_rate_cents?: number;
  /** A real sub-account at the bank (rule T9): its money is not part of the estimated account balance. */
  is_separate_account: boolean;
  /** 0 = off, else 1, 2, 3 or 5: the pot collects the round-ups of the main pot's expenses (rule T10). */
  round_up_multiplier: number;
  /** The first day that is rounded up, as YYYY-MM-DD; only while the pot collects. */
  round_up_from?: string;
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

/**
 * What the main pot can spend freely until the end of the payday period
 * (rule L8): main_pot_cents minus the three deductions.
 */
export interface SafeToSpend {
  as_of: string;
  period_end: string;
  /** Days from as_of to period_end, both included. */
  days_left: number;
  main_pot_cents: number;
  /** Open monthly payments of the main pot up to period_end, overdue ones included. */
  recurring_cents: number;
  recurring_count: number;
  /** What the reserves still need to receive this period. */
  reserve_rates_cents: number;
  /** What the savings goals still need to receive this period. */
  savings_cents: number;
  /** Rest of the budget of planned expense categories. */
  budget_left_cents: number;
  /** Negative when the period is over-planned. */
  amount_cents: number;
  /** Rounded down; 0 when nothing is left. */
  per_day_cents: number;
}

/** A monthly recurring payment of the main pot, below its category (rule P9). */
export interface BudgetRecurringLine {
  payment_id: string;
  name: string;
  due_on: string;
  /** 0 when the occurrence is skipped or the booking belongs to another payment. */
  planned_cents: number;
  actual_cents: number;
  difference_cents: number;
  skipped: boolean;
}

/** The rate a reserve saves in this month (rule P10). */
export interface BudgetReserveLine {
  payment_id: string;
  name: string;
  pot_id: string;
  /** The due date the rate saves for; missing in past months. */
  due_on?: string;
  /** null in past months, whose rates were never stored. */
  planned_cents: number | null;
  /** Net transfers into the reserve in this month. */
  actual_cents: number;
  preview: boolean;
}

export interface BudgetCategory extends PlanLine {
  recurring: BudgetRecurringLine[];
  reserves: BudgetReserveLine[];
}

/** The rate of a savings goal in a month of the budget (rule P11). */
export interface BudgetSavingsLine {
  pot_id: string;
  name: string;
  /** null in past months, whose rates were never stored. */
  planned_cents: number | null;
  /** Net transfers into the pot in this month. */
  actual_cents: number;
  preview: boolean;
}

/** An occurrence paid from its reserve, so it does not weigh on the month. */
export interface ReserveDue {
  payment_id: string;
  name: string;
  category_id: string;
  pot_id: string;
  due_on: string;
  amount_cents: number;
  /** What the reserve holds today; the full amount once booked. */
  covered_cents: number;
  booked: boolean;
}

/** The budget of the main pot for one month. */
export interface Budget {
  pot_id: string;
  month: string;
  current_month: string;
  plan_source: PlanSource;
  from: string;
  to: string;
  categories: BudgetCategory[];
  reserve_dues: ReserveDue[];
  savings: BudgetSavingsLine[];
}

/** What a new pot needs. A target of 0 or a missing one means no goal. */
export interface NewPot {
  name: string;
  target_cents?: number;
  target_date?: string;
  monthly_rate_cents?: number;
  is_separate_account?: boolean;
  /** 1, 2, 3 or 5 makes the new pot collect round-ups from today on (rule T10). */
  round_up_multiplier?: number;
}

/**
 * Only the fields sent are changed. Sending target_cents: 0 removes the goal
 * and its date, monthly_rate_cents: 0 the fixed rate.
 */
export interface PotChange {
  name: string;
  target_cents: number;
  target_date: string;
  monthly_rate_cents: number;
  is_separate_account: boolean;
  /** 0 switches the round-up off; switching it on moves it from the pot that collected before (rule T10). */
  round_up_multiplier: number;
  archived: boolean;
}

/** What a pot with a savings goal should receive in the current period (rule T8). */
export interface SavingsGoal {
  pot_id: string;
  name: string;
  target_cents: number;
  target_date?: string;
  monthly_rate_cents: number;
  balance_cents: number;
  /** Payday periods until the target date, the current one included; 0 for a fixed rate. */
  periods: number;
  /** Moved into the pot in this period. */
  saved_cents: number;
  rate_cents: number;
  /** The part of the rate not saved yet. */
  open_cents: number;
  reached: boolean;
  /** The target date lies in the past: the rest is due now. */
  date_passed: boolean;
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

/** What a purpose line of the report stands for; "none" is expenses of categories without one (rule L10). */
export type PurposeKey = "need" | "want" | "saving" | "none";

export interface PurposeLine {
  purpose: PurposeKey;
  /** Always positive. For saving it includes transfer_cents. */
  cents: number;
  /** The part of cents booked as spontaneous (rule X20). */
  spontaneous_cents: number;
  /** For saving: the part that moved into savings pots, net of what came back. */
  transfer_cents: number;
  /** cents of the period before. */
  previous_cents: number;
  /** Share of the income of the period, whole percent; null when no income is booked. */
  share_percent: number | null;
}

/** What a ledger spent in one payday period, by purpose (rule L10). */
export interface PurposeReport {
  month: string;
  current_month: string;
  previous_month: string;
  from: string;
  to: string;
  income_cents: number;
  /** need, want, saving and none, in this order. */
  lines: PurposeLine[];
}
