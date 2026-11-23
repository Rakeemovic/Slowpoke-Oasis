import { request } from "@/api/client";
import type {
  Balance,
  Category,
  CategoryChange,
  DeletedTransaction,
  Household,
  Me,
  Member,
  NewCategory,
  NewLedgerTransfer,
  NewPot,
  NewPotTransfer,
  NewTransaction,
  Pot,
  PotChange,
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

  /** Owner only. The bookings move to the fallback category of the same kind (rules K2, K6). */
  deleteCategory: (ledgerId: string, categoryId: string) =>
    request<{ moved_to: string; moved_transactions: number }>(
      "DELETE",
      `/ledgers/${enc(ledgerId)}/categories/${enc(categoryId)}`,
    ),

  /** Newest first. The server returns at most 100 by default and 500 at most; page with limit and offset. */
  transactions: (ledgerId: string, page: { limit: number; offset: number }) =>
    request<Transaction[]>("GET", `/ledgers/${enc(ledgerId)}/transactions?limit=${page.limit}&offset=${page.offset}`),

  createTransaction: (ledgerId: string, tx: NewTransaction | NewPotTransfer) =>
    request<{ id: string }>("POST", `/ledgers/${enc(ledgerId)}/transactions`, tx),

  transaction: (ledgerId: string, id: string) =>
    request<Transaction>("GET", `/ledgers/${enc(ledgerId)}/transactions/${enc(id)}`),

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
  balance: (ledgerId: string, asOf: string) =>
    request<Balance>("GET", `/ledgers/${enc(ledgerId)}/balance?date=${enc(asOf)}`),
};

export const pots = {
  /** The default pot comes first. Archived pots are included. */
  list: (ledgerId: string) => request<Pot[]>("GET", `/ledgers/${enc(ledgerId)}/pots`),

  create: (ledgerId: string, pot: NewPot) => request<{ id: string }>("POST", `/ledgers/${enc(ledgerId)}/pots`, pot),

  update: (ledgerId: string, potId: string, change: PotChange) =>
    request<void>("PATCH", `/ledgers/${enc(ledgerId)}/pots/${enc(potId)}`, change),
};

export const transfers = {
  create: (transfer: NewLedgerTransfer) =>
    request<{ out_id: string; in_id: string }>("POST", "/transfers", transfer),
};

// IDs come from the API, but encoding keeps a malformed one from changing the path.
const enc = encodeURIComponent;
