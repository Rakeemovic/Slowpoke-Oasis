import { expect, test } from "@playwright/test";

import { balance, book, createHousehold, ledgerCard, openLedger, register, transactionRow, uniqueUsername } from "./helpers";

test("a difference to the bank is booked as a correction that can only be deleted", async ({ page }) => {
  await register(page, uniqueUsername("rec"));
  await openLedger(page, "Personal");
  await book(page, { kind: "Income", amount: "100", category: "Salary" });

  await page.getByRole("link", { name: "Reconcile", exact: true }).click();
  await expect(page.getByLabel(/^Expected: 100,00\s€$/)).toBeVisible();
  await page.getByLabel("Balance at your bank (€)").fill("90");
  await expect(page.getByLabel(/^Difference: -10,00\s€$/)).toBeVisible();
  await expect(page.getByText("Your bank shows less than the app.")).toBeVisible();
  await page.getByRole("button", { name: "Book correction and save" }).click();

  // Rule L9: the correction brings the ledger in line with the bank.
  await expect(balance(page, "90,00")).toBeVisible();
  await expect(page.getByText(/^Reconciled on /)).toBeVisible();
  const correction = transactionRow(page, "Balance correction");
  await expect(correction).toContainText(/-10,00\s€/);

  // Rule X22: a correction cannot be edited, only deleted.
  await correction.click();
  await expect(page.getByRole("button", { name: "Delete" })).toBeVisible();
  await expect(page.getByLabel("Amount (€)")).toHaveCount(0);
});

test("a matching balance is saved without a correction, and members cannot reconcile", async ({ browser }) => {
  const ownerPage = await (await browser.newContext()).newPage();
  const memberPage = await (await browser.newContext()).newPage();
  const member = uniqueUsername("recm");
  await register(memberPage, member);
  await register(ownerPage, uniqueUsername("reco"));
  await createHousehold(ownerPage, "Flat");
  await ledgerCard(ownerPage, "Flat").click();

  await ownerPage.getByRole("link", { name: "Reconcile", exact: true }).click();
  await ownerPage.getByLabel("Balance at your bank (€)").fill("0");
  await expect(ownerPage.getByText("Bank and app agree")).toBeVisible();
  await expect(ownerPage.getByRole("button", { name: "Book correction and save" })).toHaveCount(0);
  await ownerPage.getByRole("button", { name: "Save", exact: true }).click();
  await expect(ownerPage.getByText(/^Reconciled on /)).toBeVisible();
  await expect(ownerPage.getByText("No transactions yet", { exact: true })).toBeVisible();

  // Only the owner reconciles (rule L9), so members get no way in.
  await ownerPage.getByRole("tab", { name: "More" }).click();
  await ownerPage.getByRole("link", { name: /^Members/ }).click();
  await ownerPage.getByLabel("Add member").fill(member);
  await ownerPage.getByRole("button", { name: "Add member" }).click();
  await expect(ownerPage.getByText(member, { exact: true })).toBeVisible();
  await memberPage.getByRole("button", { name: "Refresh" }).click();
  await openLedger(memberPage, "Flat");
  await expect(memberPage.getByRole("link", { name: "Transfer", exact: true })).toBeVisible();
  await expect(memberPage.getByRole("link", { name: "Reconcile", exact: true })).toHaveCount(0);
});
