import { expect, test } from "@playwright/test";

import { balance, book, fillBooking, openLedger, openPot, register, uniqueUsername, openSection } from "./helpers";

test.beforeEach(async ({ page }) => {
  await register(page, uniqueUsername("sep"));
  await openLedger(page, "Personal");
});

test("a real sub-account is left out of the estimated balance and listed on its own", async ({ page }) => {
  await book(page, { kind: "Income", amount: "1000", category: "Salary" });
  await expect(balance(page, "1.000,00")).toBeVisible();

  // A pot that is a separate account at the bank.
  await openSection(page, "Pots");
  await page.getByRole("link", { name: "New pot" }).click();
  await page.getByLabel("Name", { exact: true }).fill("Savings");
  await page.getByRole("switch", { name: "Separate account at my bank" }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("link", { name: /Savings/ })).toContainText("Separate account at the bank");
  await page.goBack();

  // 300 € arrive in the sub-account: they are not on the main account.
  await page.getByRole("link", { name: "Add transaction" }).click();
  await fillBooking(page, { kind: "Income", amount: "300", category: "Salary" });
  await page.getByRole("button", { name: "Savings", exact: true }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Estimated account balance", { exact: true })).toBeVisible();
  await expect(balance(page, "1.000,00")).toBeVisible();
  const accounts = page.getByLabel("Separate accounts").filter({ visible: true });
  await expect(accounts).toContainText("Separate accounts, not included above");
  await expect(accounts).toContainText("Savings");
  await expect(accounts).toContainText(/300,00\s€/);

  // Switching the flag off counts the money as part of the account again.
  await openSection(page, "Pots");
  await openPot(page, /Savings/);
  await page.getByRole("switch", { name: "Separate account at my bank" }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("link", { name: /Savings/ })).not.toContainText("Separate account at the bank");
  await page.goBack();
  await expect(balance(page, "1.300,00")).toBeVisible();
  await expect(page.getByLabel("Separate accounts")).toHaveCount(0);
});

test("the main pot cannot be a separate account", async ({ page }) => {
  await openSection(page, "Pots");
  await openPot(page, /Main/);
  await expect(page.getByRole("switch", { name: "Separate account at my bank" })).toHaveCount(0);
});
