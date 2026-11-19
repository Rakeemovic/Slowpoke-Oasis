import { request } from "@/api/client";
import type { Balance, Category, Household, Me, Member, NewLedgerTransfer, NewTransaction, Transaction } from "@/api/types";

// One small function per API endpoint, so screens never build URLs themselves.

export const auth = {
  register: (input: { username: string; email: string; password: string; display_name: string }) =>
    request<{ id: string }>("POST", "/auth/register", input),

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

  createCategory: (ledgerId: string, name: string, isIncome: boolean) =>
    request<{ id: string }>("POST", `/ledgers/${enc(ledgerId)}/categories`, { name, is_income: isIncome }),

  transactions: (ledgerId: string) => request<Transaction[]>("GET", `/ledgers/${enc(ledgerId)}/transactions`),

  createTransaction: (ledgerId: string, tx: NewTransaction) =>
    request<{ id: string }>("POST", `/ledgers/${enc(ledgerId)}/transactions`, tx),

  /** asOf is the app's local today, since the server may be in another time zone. */
  balance: (ledgerId: string, asOf: string) =>
    request<Balance>("GET", `/ledgers/${enc(ledgerId)}/balance?date=${enc(asOf)}`),
};

export const transfers = {
  create: (transfer: NewLedgerTransfer) =>
    request<{ out_id: string; in_id: string }>("POST", "/transfers", transfer),
};

// IDs come from the API, but encoding keeps a malformed one from changing the path.
const enc = encodeURIComponent;
