import { expect, type Locator, type Page } from "@playwright/test";

export const PASSWORD = "correct horse battery";

/**
 * A username no other test uses, so tests can run in parallel against the
 * same database. Usernames allow 3–32 lowercase letters, digits or "_".
 */
export function uniqueUsername(prefix: string): string {
  return `${prefix}_${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

/**
 * Matches an amount as the app shows it in the de-DE locale, e.g. "12,50 €".
 * Intl puts a non-breaking space before "€", hence \s instead of " ".
 */
export function euros(amount: string): RegExp {
  return new RegExp(`${escape(amount)}\\s€`);
}

/** The balance line that every ledger card and the ledger screen show. */
export function balance(scope: Page | Locator, amount: string): Locator {
  return scope.getByText(new RegExp(`^${escape(amount)}\\s€ in all pots$`));
}

/** Creates an account, confirms the recovery code and returns that code. */
export async function register(page: Page, username: string): Promise<string> {
  await page.goto("/register");
  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Display name").fill(username);
  await page.getByLabel("Password").fill(PASSWORD);
  await page.getByRole("button", { name: "Create account" }).click();
  const code = await recoveryCode(page);
  await page.getByRole("button", { name: "I have saved it, continue" }).click();
  await expectOverview(page);
  return code;
}

/** The recovery code the app shows after registration or a reset: six words. */
export async function recoveryCode(page: Page): Promise<string> {
  const shown = page.getByLabel("Recovery code", { exact: true });
  await expect(shown).toHaveText(/^\s*\S+( \S+){5}\s*$/);
  return (await shown.innerText()).trim();
}

export async function signIn(page: Page, username: string, password = PASSWORD): Promise<void> {
  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
}

export async function expectOverview(page: Page): Promise<void> {
  await expect(page.getByText("Households", { exact: true })).toBeVisible();
}

/** The card of a ledger on the overview screen. */
export function ledgerCard(page: Page, name: string): Locator {
  return page.getByRole("link").filter({ hasText: name });
}

export async function openLedger(page: Page, name: string): Promise<void> {
  await ledgerCard(page, name).click();
  await expect(page.getByText("Available today")).toBeVisible();
}

export async function createHousehold(page: Page, name: string): Promise<void> {
  await page.getByLabel("New household").fill(name);
  await page.getByRole("button", { name: "Create household" }).click();
  await expect(ledgerCard(page, name)).toBeVisible();
}

export interface Booking {
  kind: "Expense" | "Income";
  amount: string;
  category: string;
  /** Creates the category in the form instead of picking an existing one. */
  newCategory?: boolean;
  note?: string;
}

/**
 * Fills the booking form. The caller says whether the category is new,
 * because checking for it on screen would race with the categories loading.
 */
export async function fillBooking(page: Page, booking: Booking): Promise<void> {
  await page.getByRole("button", { name: booking.kind, exact: true }).click();
  await page.getByLabel("Amount (€)").fill(booking.amount);
  const category = page.getByRole("button", { name: booking.category, exact: true });
  if (booking.newCategory) {
    await page.getByLabel(`New ${booking.kind.toLowerCase()} category`).fill(booking.category);
    await page.getByRole("button", { name: "Add", exact: true }).click();
    await expect(category).toBeVisible();
  } else {
    await category.click();
  }
  await page.getByLabel("Note (optional)").fill(booking.note ?? "");
}

/** Books an income or expense from the ledger screen and returns to it. */
export async function book(page: Page, booking: Booking): Promise<void> {
  // Buttons wrapped in a router Link are links for the browser, not buttons.
  await page.getByRole("link", { name: "Add transaction" }).click();
  await fillBooking(page, booking);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Available today")).toBeVisible();
}

/**
 * A row in the transaction list of the open ledger. Matched by its target, so
 * a row like "Transfer to …" is not confused with the "Transfer" button.
 * Rows the user may not change are no links and need page.getByText instead.
 */
export function transactionRow(page: Page, title: string): Locator {
  return page.locator('a[href*="/transactions/"]').filter({ hasText: title });
}

function escape(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
