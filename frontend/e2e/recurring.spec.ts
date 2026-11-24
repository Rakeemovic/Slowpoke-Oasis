import { expect, test, type Page } from "@playwright/test";

import { balance, euros, openLedger, register, transactionRow, uniqueUsername, openSection, backToOverview } from "./helpers";

test.beforeEach(async ({ page }) => {
  await register(page, uniqueUsername("rec"));
  await openLedger(page, "Personal");
});

/**
 * Creates a monthly payment from the ledger screen that is due today, so the
 * test does not depend on the date it runs on, and returns to the ledger.
 */
async function createDueToday(page: Page, name: string, amount: string, category: string): Promise<void> {
  await openSection(page, "Recurring");
  await page.getByRole("link", { name: "New recurring payment" }).click();
  await page.getByLabel("Name", { exact: true }).fill(name);
  await page.getByLabel("Amount (€)").fill(amount);
  await page.getByRole("button", { name: category, exact: true }).click();
  await page.getByLabel("Due day (1–31)").fill(String(new Date().getDate()));
  await page.getByRole("button", { name: "Save" }).click();
  await expect(paymentRow(page, name)).toContainText(`Monthly on day ${new Date().getDate()}`);
  await backToOverview(page);
}

/** A row on the recurring payments screen; earlier screens stay hidden in the DOM. */
function paymentRow(page: Page, name: string) {
  return page.getByRole("link").filter({ has: page.getByText(name, { exact: true }), visible: true });
}

/** A row of the "Due payments" card on the ledger screen. */
function dueRow(page: Page, name: string) {
  return page
    .getByText(name, { exact: true })
    .locator("xpath=ancestor::div[.//*[normalize-space()='Book' or normalize-space()='Undo skip']][1]")
    .filter({ visible: true });
}

test("a due payment is booked with one tap from prefilled values", async ({ page }) => {
  await createDueToday(page, "Flat", "800", "Rent");
  await expect(dueRow(page, "Flat")).toContainText("Due today");
  await expect(dueRow(page, "Flat")).toContainText(euros("800,00"));
  // The bank has most likely paid already, so the estimate counts it.
  await expect(balance(page, "-800,00")).toBeVisible();
  await expect(page.getByText(/^incl\. 1 unbooked payment \(-800,00\s€\)$/)).toBeVisible();

  await page.getByRole("link", { name: "Book Flat" }).click();
  // The form starts with the payment's values; the amount can still change.
  await expect(page.getByLabel("Amount (€)")).toHaveValue("800.00");
  await expect(page.getByRole("button", { name: "Income", exact: true })).toHaveCount(0);
  await page.getByLabel("Amount (€)").fill("810");
  await page.getByRole("button", { name: "Save" }).click();

  await expect(transactionRow(page, "Rent")).toContainText(euros("-810,00"));
  await expect(balance(page, "-810,00")).toBeVisible();
  await expect(page.getByText("Due payments")).toBeHidden();
});

test("a due payment can be skipped, the skip undone, and the payment archived", async ({ page }) => {
  await createDueToday(page, "Gym", "30", "Leisure");

  await expect(balance(page, "-30,00")).toBeVisible();
  await page.getByRole("button", { name: "Skip Gym" }).click();
  await expect(dueRow(page, "Gym")).toContainText("Skipped");
  await expect(balance(page, "0,00")).toBeVisible();
  await expect(page.getByRole("link", { name: "Book Gym" })).toHaveCount(0);

  await page.getByRole("button", { name: "Undo skip of Gym" }).click();
  await expect(dueRow(page, "Gym")).toContainText("Due today");
  await expect(balance(page, "-30,00")).toBeVisible();

  // An archived payment shows no due dates.
  await openSection(page, "Recurring");
  await paymentRow(page, "Gym").click();
  await page.getByRole("button", { name: "Archive payment" }).click();
  await expect(page.getByText("Archived", { exact: true })).toBeVisible();
  await backToOverview(page);
  await expect(page.getByText("Due payments")).toBeHidden();
});

test("a new category for a booking of a recurring payment asks how far it reaches", async ({ page }) => {
  await createDueToday(page, "Flat", "800", "Rent");
  await page.getByRole("link", { name: "Book Flat" }).click();
  await page.getByRole("button", { name: "Save" }).click();

  await transactionRow(page, "Rent").click();
  await page.getByRole("button", { name: "Utilities", exact: true }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText(/Which of its bookings move to “Utilities”\?/)).toBeVisible();

  // Cancel goes back to the form, which keeps the choice.
  await page.getByRole("button", { name: "Cancel" }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await page.getByRole("button", { name: "All of this series" }).click();
  await expect(transactionRow(page, "Utilities")).toBeVisible();

  // "All" also changes the recurring payment for future bookings.
  await openSection(page, "Recurring");
  await expect(paymentRow(page, "Flat")).toContainText("Utilities");
});
