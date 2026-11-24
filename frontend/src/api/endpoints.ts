import { request } from "@/api/client";
import type {
  Balance,
  Budget,
  Category,
  CategoryChange,
  DeletedTransaction,
  Household,
  Me,
  Member,
  ReconcileInput,
  ReconcileResult,
  NewCategory,
  NewLedgerTransfer,
  NewPot,
  NewPotTransfer,
  NewTransaction,
  Occurrence,
  PlanScope,
  Pot,
  PotChange,
  RecurringPayment,
  RecurringPaymentChange,
  RecurringPaymentInput,
  ReserveRate,
  SafeToSpend,
  SavingsGoal,
  Transaction,
  TransactionChange,
} from "@/api/types";

// One small function per API endpoint, so screens never build URLs themselves.

export const auth = {
  /** The answer carries the recovery code. The server shows it only this once. */
  register: (input: { username: string; password: string; display_name: string }) =>
    request<{ id: string; recovery_code: string }>("POST", "/auth/register", input),

  /** Sets a new password with the recovery code. The code is used up; the answer carries a new one. */
  recover: (input: { username: string; recovery_code: string; new_password: string }) =>
    request<{ recovery_code: string }>("POST", "/auth/recover", input),

  /** Other devices are signed out; this one stays signed in. */
  changePassword: (currentPassword: string, newPassword: string) =>
    request<void>("PUT", "/auth/password", { current_password: currentPassword, new_password: newPassword }),

  /** Native apps receive a token. The web app asks for a cookie and gets no body. */
  login: (username: string, password: string, cookie: boolean) =>
    request<{ token: string } | undefined>("POST", "/auth/login", { username, password, cookie }),

  logout: () => request<void>("POST", "/auth/logout"),

  me: () => request<Me>("GET", "/me"),
};

export const households = {
  list: () => request<Household[]>("GET", "/households"),
  create: (name: string) => request<{ id: string; ledger_id: string }>("POST", "/households", { name }),

  members: (householdId: string) => request<Member[]>("GET", `/households/${enc(householdId)}/members`),

  /** Owner only. */
  addMember: (householdId: string, username: string) =>
    request<void>("POST", `/households/${enc(householdId)}/members`, { username }),

  /** The owner removes someone else, or any member removes themselves to leave. */
  removeMember: (householdId: string, userId: string) =>
    request<void>("DELETE", `/households/${enc(householdId)}/members/${enc(userId)}`),

  /** Owner only. The current owner becomes a regular member. */
  transferOwnership: (householdId: string, userId: string) =>
    request<void>("PUT", `/households/${enc(householdId)}/owner/${enc(userId)}`),
};

export const ledgers = {
  categories: (ledgerId: string) => request<Category[]>("GET", `/ledgers/${enc(ledgerId)}/categories`),

  /** Every member may create categories; a ledger has at most 50 (rule K7). */
  createCategory: (ledgerId: string, category: NewCategory) =>
    request<{ id: string }>("POST", `/ledgers/${enc(ledgerId)}/categories`, category),

  /** Owner only (rule K9). */
  updateCategory: (ledgerId: string, categoryId: string, change: CategoryChange) =>
    request<void>("PATCH", `/ledgers/${enc(ledgerId)}/categories/${enc(categoryId)}`, change),

  /**
   * Owner only. The bookings move to moveTo, another active category of the
   * same kind, or without it to the fallback category (rules K2, K6).
   */
  deleteCategory: (ledgerId: string, categoryId: string, moveTo?: string) =>
    request<{ moved_to: string; moved_transactions: number }>(
      "DELETE",
      `/ledgers/${enc(ledgerId)}/categories/${enc(categoryId)}${moveTo ? `?move_to=${enc(moveTo)}` : ""}`,
    ),

  /** Newest first. The server returns at most 100 by default and 500 at most; page with limit and offset. */
  transactions: (ledgerId: string, page: { limit: number; offset: number }) =>
    request<Transaction[]>("GET", `/ledgers/${enc(ledgerId)}/transactions?limit=${page.limit}&offset=${page.offset}`),

  createTransaction: (ledgerId: string, tx: NewTransaction | NewPotTransfer) =>
    request<{ id: string; topped_up_cents?: number }>("POST", `/ledgers/${enc(ledgerId)}/transactions`, tx),

  transaction: (ledgerId: string, id: string) => request<Transaction>("GET", `/ledgers/${enc(ledgerId)}/transactions/${enc(id)}`),

  /** Incomes, expenses and transfers between pots; a transfer between ledgers can only be deleted. */
  updateTransaction: (ledgerId: string, id: string, change: TransactionChange | NewPotTransfer) =>
    request<void>("PATCH", `/ledgers/${enc(ledgerId)}/transactions/${enc(id)}`, change),

  /** Soft delete; a transfer between ledgers loses both legs. */
  deleteTransaction: (ledgerId: string, id: string) =>
    request<void>("DELETE", `/ledgers/${enc(ledgerId)}/transactions/${enc(id)}`),

  /** Soft-deleted transactions that can still be restored (they are removed for good after 30 days). */
  deletedTransactions: (ledgerId: string, page: { limit: number; offset: number }) =>
    request<DeletedTransaction[]>(
      "GET",
      `/ledgers/${enc(ledgerId)}/transactions/deleted?limit=${page.limit}&offset=${page.offset}`,
    ),

  /** A transfer between ledgers comes back with both legs. */
  restoreTransaction: (ledgerId: string, id: string) =>
    request<void>("POST", `/ledgers/${enc(ledgerId)}/transactions/${enc(id)}/restore`),

  /** asOf is the app's local today, since the server may be in another time zone. */
  balance: (ledgerId: string, asOf: string) => request<Balance>("GET", `/ledgers/${enc(ledgerId)}/balance?date=${enc(asOf)}`),

  /** Compares the real balance with the ledger on date, the app's local today; owner only (rule L9). */
  reconcile: (ledgerId: string, input: ReconcileInput, date: string) =>
    request<ReconcileResult>("POST", `/ledgers/${enc(ledgerId)}/reconciliations?date=${enc(date)}`, input),

  /** Pots that save a rate (target date or fixed rate), with this period's rate; date is the app's local today. */
  savingsGoals: (ledgerId: string, date: string) =>
    request<SavingsGoal[]>("GET", `/ledgers/${enc(ledgerId)}/savings-goals?date=${enc(date)}`),

  /** What is safe to spend until the end of the payday period; date is the app's local today. */
  safeToSpend: (ledgerId: string, date: string) =>
    request<SafeToSpend>("GET", `/ledgers/${enc(ledgerId)}/safe-to-spend?date=${enc(date)}`),

  /**
   * The budget of the main pot for one month, by default the current one.
   * date is the app's local today and decides which month is current.
   */
  budget: (ledgerId: string, date: string, month?: string) =>
    request<Budget>("GET", `/ledgers/${enc(ledgerId)}/budget?date=${enc(date)}${month ? `&month=${enc(month)}` : ""}`),
};

export const pots = {
  /** The default pot comes first. Archived pots are included. */
  list: (ledgerId: string) => request<Pot[]>("GET", `/ledgers/${enc(ledgerId)}/pots`),

  create: (ledgerId: string, pot: NewPot) => request<{ id: string }>("POST", `/ledgers/${enc(ledgerId)}/pots`, pot),

  /**
   * Only the fields sent are changed. date is the app's local today: a
   * round-up switched on starts with it (rule T10).
   */
  update: (ledgerId: string, potId: string, change: Partial<PotChange>, date: string) =>
    request<void>("PATCH", `/ledgers/${enc(ledgerId)}/pots/${enc(potId)}?date=${enc(date)}`, change),

  /** Monthly pots need a scope (rule P5); one-time pots take none. */
  setPlanned: (ledgerId: string, potId: string, categoryId: string, amountCents: number, date: string, scope?: PlanScope) =>
    request<void>("PUT", `${planLinePath(ledgerId, potId, categoryId)}?date=${enc(date)}`, {
      amount_cents: amountCents,
      scope,
    }),

  removePlanned: (ledgerId: string, potId: string, categoryId: string, date: string, scope?: PlanScope) =>
    request<void>(
      "DELETE",
      `${planLinePath(ledgerId, potId, categoryId)}?date=${enc(date)}${scope ? `&scope=${enc(scope)}` : ""}`,
    ),
};

function planLinePath(ledgerId: string, potId: string, categoryId: string): string {
  return `/ledgers/${enc(ledgerId)}/pots/${enc(potId)}/lines/${enc(categoryId)}`;
}

// date is the app's local today: the server may run in another time zone.
export const recurring = {
  /** Archived payments are included. */
  list: (ledgerId: string, date: string) => request<RecurringPayment[]>("GET", `${recurringPath(ledgerId)}?date=${enc(date)}`),

  /** Every member may create one (rule R8). Earlier due dates never become open (rule R5). */
  create: (ledgerId: string, payment: RecurringPaymentInput, date: string) =>
    request<{ id: string }>("POST", `${recurringPath(ledgerId)}?date=${enc(date)}`, payment),

  /** Owner only (rule R8). */
  update: (ledgerId: string, paymentId: string, change: RecurringPaymentChange, date: string) =>
    request<void>("PATCH", `${recurringPath(ledgerId)}/${enc(paymentId)}?date=${enc(date)}`, change),

  /** Open occurrences up to the end of the current payday period, oldest first. */
  due: (ledgerId: string, date: string) => request<Occurrence[]>("GET", `${recurringPath(ledgerId)}/due?date=${enc(date)}`),

  /** The rates of the current payday period for the active reserves, by due date. */
  reserveRates: (ledgerId: string, date: string) =>
    request<ReserveRate[]>("GET", `${recurringPath(ledgerId)}/reserves?date=${enc(date)}`),

  /** Every member may skip an occurrence and undo it (rule R7). */
  skip: (ledgerId: string, paymentId: string, dueOn: string) =>
    request<void>("PUT", `${recurringPath(ledgerId)}/${enc(paymentId)}/skips/${enc(dueOn)}`),

  undoSkip: (ledgerId: string, paymentId: string, dueOn: string) =>
    request<void>("DELETE", `${recurringPath(ledgerId)}/${enc(paymentId)}/skips/${enc(dueOn)}`),
};

function recurringPath(ledgerId: string): string {
  return `/ledgers/${enc(ledgerId)}/recurring-payments`;
}

export const transfers = {
  create: (transfer: NewLedgerTransfer) => request<{ out_id: string; in_id: string }>("POST", "/transfers", transfer),
};

// IDs come from the API, but encoding keeps a malformed one from changing the path.
const enc = encodeURIComponent;
