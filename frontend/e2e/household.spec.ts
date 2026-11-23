import { expect, test } from "@playwright/test";

import {
  balance,
  book,
  createHousehold,
  euros,
  ledgerCard,
  openLedger,
  register,
  transactionRow,
  uniqueUsername,
} from "./helpers";

test("a transfer moves money into a household and deleting it undoes both sides", async ({ page }) => {
  await register(page, uniqueUsername("hh"));
  await createHousehold(page, "Oasis");
  await openLedger(page, "Personal");
  await book(page, { kind: "Income", amount: "100", category: "Salary" });

  await page.getByRole("link", { name: "Transfer" }).click();
  await page.getByRole("button", { name: "Oasis", exact: true }).click();
  await page.getByLabel("Amount (€)").fill("40");
  await page.getByLabel("Note (optional)").fill("Rent share");
  await page.getByRole("button", { name: "Transfer", exact: true }).click();

  // The sending side names where the money went.
  await expect(transactionRow(page, "Transfer to Oasis")).toContainText(euros("40,00"));
  await expect(balance(page, "60,00")).toBeVisible();

  await page.goto("/");
  await expect(balance(ledgerCard(page, "Personal"), "60,00")).toBeVisible();
  await expect(balance(ledgerCard(page, "Oasis"), "40,00")).toBeVisible();

  // The receiving side names where it came from; the owner may see their own ledger's name.
  await openLedger(page, "Oasis");
  await transactionRow(page, "Transfer from Personal").click();
  // exact: the list underneath stays in the page (hidden) and contains the note too.
  await expect(page.getByText("Rent share", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("alertdialog")).toContainText("The other side of the transfer is deleted too.");
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(balance(page, "0,00")).toBeVisible();

  // Rule X14: both bookings of the transfer are gone.
  await page.goto("/");
  await expect(balance(ledgerCard(page, "Personal"), "100,00")).toBeVisible();
  await expect(balance(ledgerCard(page, "Oasis"), "0,00")).toBeVisible();
});
