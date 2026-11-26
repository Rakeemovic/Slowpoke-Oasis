import { expect, test, type Page } from "@playwright/test";

import { bar, book, euros, fillBooking, openLedger, openSection, register, uniqueUsername } from "./helpers";

/** The month before this one as YYYY-MM; paydays start on the 1st here, so it is the period before. */
function previousMonth(): string {
  const d = new Date();
  d.setDate(1);
  d.setMonth(d.getMonth() - 1);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

/** Books an expense from the ledger screen with the form open, then returns to the ledger screen. */
async function saveAndReturn(page: Page): Promise<void> {
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Estimated account balance", { exact: true })).toBeVisible();
}

/** Creates a pot from the pots screen and moves money into it from the main pot. */
async function saveInto(page: Page, pot: string, amount: string): Promise<void> {
  await openSection(page, "Pots");
  await page.getByRole("link", { name: "New pot" }).click();
  await page.getByLabel("Name", { exact: true }).fill(pot);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("link", { name: new RegExp(pot) })).toBeVisible();

  await openSection(page, "Move between pots");
  await page.getByRole("button", { name: "Main", exact: true }).first().click();
  await page.getByRole("button", { name: pot, exact: true }).last().click();
  await page.getByLabel("Amount (€)").fill(amount);
  await page.getByRole("button", { name: "Save" }).click();
}

test("insights split a period by purpose and compare it with the one before", async ({ page }) => {
  await register(page, uniqueUsername("insight"));
  await openLedger(page, "Personal");

  // This period: income, a need, a spontaneous want and 300 € moved into a pot.
  await book(page, { kind: "Income", amount: "2000", category: "Salary" });
  await book(page, { kind: "Expense", amount: "100", category: "Groceries" });
  await page.getByRole("link", { name: "Add transaction" }).click();
  await fillBooking(page, { kind: "Expense", amount: "40", category: "Eating out" });
  await page.getByRole("switch", { name: "Spontaneous" }).click();
  await saveAndReturn(page);
  // The period before: 50 € for groceries.
  await page.getByRole("link", { name: "Add transaction" }).click();
  await fillBooking(page, { kind: "Expense", amount: "50", category: "Groceries" });
  await page.getByLabel("Date (YYYY-MM-DD)").fill(`${previousMonth()}-15`);
  await saveAndReturn(page);
  await saveInto(page, "Holiday", "300");

  await openSection(page, "Insights");
  await expect(page.getByText("Income this period", { exact: true })).toBeVisible();
  await expect(page.getByText(euros("2.000,00")).filter({ visible: true })).toBeVisible();

  // Each row is its own bar: its fill is the share of the income.
  await expect(page.getByText("5 % of income", { exact: true })).toBeVisible();
  await expect(bar(page, "Needs")).toHaveAttribute("aria-valuenow", "5");
  await expect(page.getByText("2 % of income", { exact: true })).toBeVisible();
  await expect(bar(page, "Wants")).toHaveAttribute("aria-valuenow", "2");
  await expect(page.getByText("15 % of income", { exact: true })).toBeVisible();
  await expect(bar(page, "Savings")).toHaveAttribute("aria-valuenow", "15");

  // Against the period before, and the details that belong to a purpose.
  await expect(page.getByText(/^50,00\s€ more than in /)).toBeVisible();
  await expect(page.getByText(/^40,00\s€ more than in /)).toBeVisible();
  await expect(page.getByText(/^300,00\s€ more than in /)).toBeVisible();
  await expect(page.getByText(/^40,00\s€ spontaneous \(100 % of wants\)$/)).toBeVisible();
  await expect(page.getByText(/^incl\. 300,00\s€ moved into pots$/)).toBeVisible();

  // The period before has no income, so no shares, and its own amounts.
  await page.getByRole("button", { name: "Previous month" }).click();
  await expect(page.getByRole("button", { name: "Back to this month" })).toBeVisible();
  await expect(page.getByText("No income booked, so there are no shares")).toBeVisible();
  await expect(page.getByText(/^50,00\s€$/).filter({ visible: true })).toBeVisible();
  await expect(page.getByText(/ of income$/)).toHaveCount(0);

  await page.getByRole("button", { name: "Back to this month" }).click();
  await expect(page.getByText("5 % of income", { exact: true })).toBeVisible();
});

test("insights without bookings say so", async ({ page }) => {
  await register(page, uniqueUsername("insight"));
  await openLedger(page, "Personal");
  await openSection(page, "Insights");
  await expect(page.getByText("Nothing to show for this period", { exact: true })).toBeVisible();
});
