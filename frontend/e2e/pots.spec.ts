import { expect, test, type Locator, type Page } from "@playwright/test";

import {
  balance,
  euros,
  fillBooking,
  openLedger,
  register,
  transactionRow,
  uniqueUsername,
  openSection,
  backToOverview,
} from "./helpers";

test.beforeEach(async ({ page }) => {
  await register(page, uniqueUsername("pot"));
  await openLedger(page, "Personal");
});

/** The money in the default pot on the ledger screen; earlier screens stay hidden in the DOM. */
function inMain(page: Page, amount: string): Locator {
  return page
    .getByText(new RegExp(`^${amount}\\s€ in Main$`))
    .filter({ visible: true })
    .first();
}

/** Opens the pots screen from the ledger screen. */
async function openPots(page: Page): Promise<void> {
  await openSection(page, "Pots");
  await expect(page.getByRole("link", { name: "New pot" })).toBeVisible();
}

async function createPot(page: Page, name: string, goal?: string): Promise<void> {
  await page.getByRole("link", { name: "New pot" }).click();
  await page.getByLabel("Name", { exact: true }).fill(name);
  if (goal) {
    await page.getByRole("button", { name: "Target", exact: true }).click();
    await page.getByLabel("Target amount (€)").fill(goal);
  }
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("link", { name: "New pot" })).toBeVisible();
}

test("a booking goes into a chosen pot and money moves between pots", async ({ page }) => {
  await openPots(page);
  await createPot(page, "Holiday", "500");
  await expect(page.getByRole("link", { name: /Holiday/ })).toContainText(euros("Goal 500,00"));
  await page.goBack();

  // The picker only appears once there is more than the default pot.
  await page.getByRole("link", { name: "Add transaction" }).click();
  await fillBooking(page, {
    kind: "Income",
    amount: "100",
    category: "Salary",
  });
  await page.getByRole("button", { name: "Holiday", exact: true }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Estimated account balance", { exact: true })).toBeVisible();

  // The money sits in the savings pot, so nothing is available yet.
  await expect(balance(page, "100,00")).toBeVisible();
  await expect(inMain(page, "0,00")).toBeVisible();
  // With more than one pot the list names the pot of each booking.
  await expect(transactionRow(page, "Salary")).toContainText("Holiday");

  // Move 30 € out of Holiday into the default pot.
  await openPots(page);
  await openSection(page, "Move between pots");
  await page.getByRole("button", { name: "Holiday", exact: true }).first().click();
  await page.getByRole("button", { name: "Main", exact: true }).last().click();
  await page.getByLabel("Amount (€)").fill("30");
  await page.getByRole("button", { name: "Save" }).click();
  await openSection(page, "Pots");
  await expect(page.getByRole("link", { name: "New pot" })).toBeVisible();
  await expect(page.getByRole("link", { name: /Holiday/ })).toContainText(euros("70,00"));
  await page.goBack();
  await expect(inMain(page, "30,00")).toBeVisible();
  await expect(balance(page, "100,00")).toBeVisible();
  await expect(transactionRow(page, "Transfer between pots")).toContainText("Holiday → Main");

  // The move can be edited: change the amount.
  await transactionRow(page, "Transfer between pots").click();
  await page.getByLabel("Amount (€)").fill("40");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(inMain(page, "40,00")).toBeVisible();
});

test("a pot can be renamed, archived and restored", async ({ page }) => {
  await openPots(page);
  await createPot(page, "Holiday");
  await createPot(page, "Spare");
  await page.getByRole("link", { name: /Holiday/ }).click();
  await page.getByLabel("Name", { exact: true }).fill("Trip");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("link", { name: /Trip/ })).toBeVisible();

  await page.getByRole("link", { name: /Trip/ }).click();
  await page.getByRole("button", { name: "Archive pot" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Archive" }).click();
  await expect(page.getByText("Archived", { exact: true })).toBeVisible();

  // An archived pot is no longer offered when booking (rule T5). The picker only
  // shows once the pots are loaded, so waiting for "Spare" makes the check meaningful.
  await page.goBack();
  await page.getByRole("link", { name: "Add transaction" }).click();
  await expect(page.getByRole("button", { name: "Spare", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Trip", exact: true })).toHaveCount(0);
  await backToOverview(page);

  await openPots(page);
  await page.getByRole("link", { name: /Trip/ }).click();
  await page.getByRole("button", { name: "Restore pot" }).click();
  await expect(page.getByText("Archived", { exact: true })).toHaveCount(0);
});
