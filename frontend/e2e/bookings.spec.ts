import { expect, test } from "@playwright/test";

import { balance, book, euros, fillBooking, openLedger, register, transactionRow, uniqueUsername } from "./helpers";

test.beforeEach(async ({ page }) => {
  await register(page, uniqueUsername("book"));
  await openLedger(page, "Personal");
});

test("income and expenses change the balance", async ({ page }) => {
  await expect(page.getByText("No transactions yet.", { exact: false })).toBeVisible();

  await book(page, { kind: "Income", amount: "1000", category: "Salary" });
  await expect(transactionRow(page, "Salary")).toContainText(euros("+1.000,00"));
  await expect(balance(page, "1.000,00")).toBeVisible();

  await book(page, { kind: "Expense", amount: "12,50", category: "Groceries", note: "Market" });
  const groceries = transactionRow(page, "Groceries");
  await expect(groceries).toContainText(euros("-12,50"));
  await expect(groceries).toContainText("Market");
  await expect(balance(page, "987,50")).toBeVisible();
});

test("a booking can be edited and deleted", async ({ page }) => {
  await book(page, { kind: "Income", amount: "100", category: "Salary" });
  await book(page, { kind: "Expense", amount: "30", category: "Groceries" });
  await expect(balance(page, "70,00")).toBeVisible();

  // Edit: the form opens prefilled; change the amount and pick an existing category.
  await transactionRow(page, "Groceries").click();
  await expect(page.getByLabel("Amount (€)")).toHaveValue("30.00");
  await fillBooking(page, { kind: "Expense", amount: "45", category: "Groceries", note: "Corrected" });
  await page.getByRole("button", { name: "Save" }).click();
  await expect(transactionRow(page, "Groceries")).toContainText("Corrected");
  await expect(balance(page, "55,00")).toBeVisible();

  // Delete asks for confirmation first; Cancel keeps the booking.
  await transactionRow(page, "Salary").click();
  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("button", { name: "Cancel" }).click();
  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();

  await expect(transactionRow(page, "Salary")).toHaveCount(0);
  await expect(balance(page, "-45,00")).toBeVisible();
});

test("a deleted booking can be restored", async ({ page }) => {
  await book(page, { kind: "Income", amount: "100", category: "Salary" });
  await transactionRow(page, "Salary").click();
  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(transactionRow(page, "Salary")).toHaveCount(0);

  await page.getByRole("link", { name: "Deleted transactions" }).click();
  await expect(page.getByText("Salary", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Restore Salary" }).click();
  await expect(page.getByText("Nothing to restore.")).toBeVisible();

  await page.goBack();
  await expect(transactionRow(page, "Salary")).toBeVisible();
  await expect(balance(page, "100,00").last()).toBeVisible();
});
