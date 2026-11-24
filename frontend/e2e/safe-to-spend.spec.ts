import { expect, test, type Page } from "@playwright/test";

import { book, createPaymentDueToday, euros, openLedger, register, uniqueUsername, openSection } from "./helpers";

test.beforeEach(async ({ page }) => {
  await register(page, uniqueUsername("sts"));
  await openLedger(page, "Personal");
});

/** The pressable safe-to-spend block of the visible ledger screen. */
function safeToSpend(page: Page) {
  return page.getByRole("button", { name: /^Safe to spend: / }).filter({ visible: true });
}

/** "12,34" for 1234 cents, as the de-DE locale shows amounts below 1000 €. */
function plainEuros(cents: number): string {
  return `${Math.floor(cents / 100)},${String(cents % 100).padStart(2, "0")}`;
}

/** The days from today to the end of the month, today included; the default payday is the 1st. */
function daysLeftInMonth(): number {
  const now = new Date();
  const lastDay = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
  return lastDay - now.getDate() + 1;
}

test("safe to spend deducts open payments and the rest of the budget", async ({ page }) => {
  await book(page, { kind: "Income", amount: "1000", category: "Salary" });
  await book(page, { kind: "Expense", amount: "50", category: "Groceries" });
  await createPaymentDueToday(page, "Flat", "300", "Rent");

  // Budget 200 € for groceries, of which 50 € are spent.
  await openSection(page, "Budget");
  await page.getByRole("button", { name: /^Groceries/ }).click();
  await page.getByLabel("Planned for Groceries (€)").fill("200");
  await page.getByRole("button", { name: "This and coming months", exact: true }).click();
  await expect(page.getByLabel("Planned for Groceries (€)")).toBeHidden();
  await page.goBack();

  // 1000 − 50 spent − 300 rent − 150 budget left = 500 €.
  await expect(safeToSpend(page)).toContainText(euros("500,00"));
  await expect(safeToSpend(page)).toContainText(euros(plainEuros(Math.floor(50000 / daysLeftInMonth()))));
  await expect(safeToSpend(page)).toContainText("per day");

  await safeToSpend(page).click();
  const breakdown = page.getByLabel("Safe to spend breakdown").filter({ visible: true });
  await expect(breakdown).toContainText(euros("950,00"));
  await expect(breakdown).toContainText("Open payment (1)");
  await expect(breakdown).toContainText(euros("-300,00"));
  await expect(breakdown).toContainText(euros("-150,00"));

  // Booking the rent leaves the amount unchanged: it moves from open to spent.
  await page.getByRole("link", { name: "Book Flat" }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(safeToSpend(page)).toContainText(euros("500,00"));
});

test("an over-planned period is shown in red without an amount per day", async ({ page }) => {
  await createPaymentDueToday(page, "Flat", "300", "Rent");

  await expect(safeToSpend(page)).toContainText(/Over-planned by 300,00\s€/);
  await expect(safeToSpend(page)).not.toContainText("per day");
});
