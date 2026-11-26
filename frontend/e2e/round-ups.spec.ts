import { expect, test, type Page } from "@playwright/test";

import {
  book,
  createHousehold,
  ledgerCard,
  openLedger,
  openPot,
  register,
  transactionRow,
  uniqueUsername,
  openSection,
} from "./helpers";

/** Creates a pot from the pots screen and stays there. */
async function createPot(page: Page, name: string): Promise<void> {
  await page.getByRole("link", { name: "New pot" }).click();
  await page.getByLabel("Name", { exact: true }).fill(name);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("link", { name: new RegExp(name) })).toBeVisible();
}

/** Opens a pot from the pots screen, sets its round-up and saves. */
async function setRoundUp(page: Page, pot: string, multiplier: string): Promise<void> {
  await openPot(page, new RegExp(pot));
  await page.getByRole("button", { name: multiplier, exact: true }).click();
  await page.getByRole("button", { name: "Save" }).click();
}

test("an expense is rounded up into the collecting pot and goes with it", async ({ page }) => {
  await register(page, uniqueUsername("round"));
  await openLedger(page, "Personal");
  await openSection(page, "Pots");
  await createPot(page, "Change jar");
  await setRoundUp(page, "Change jar", "x2");
  await expect(page.getByRole("link", { name: /Change jar/ })).toContainText("Collects round-ups (x2)");
  await page.goBack();

  // 12.30 € → 0.70 € to the next euro, times two.
  await book(page, { kind: "Expense", amount: "12.30", category: "Groceries" });
  const roundUp = page.getByText("Round-up", { exact: true }).filter({ visible: true });
  await expect(roundUp).toBeVisible();
  await expect(page.getByText(/Main → Change jar/).filter({ visible: true })).toBeVisible();
  await expect(page.getByText(/^1,40\s€$/).filter({ visible: true })).toBeVisible();
  // It only changes together with the expense, so it opens nothing.
  await expect(transactionRow(page, "Round-up")).toHaveCount(0);

  // Deleting the expense takes the round-up with it, and the dialog says so.
  await transactionRow(page, "Groceries").click();
  await page.getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("alertdialog")).toContainText(/Its round-up of 1,40\s€ is deleted too\./);
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByText("Estimated account balance", { exact: true })).toBeVisible();
  await expect(roundUp).toHaveCount(0);
});

test("only one pot collects, and the form says which one it replaces", async ({ page }) => {
  await register(page, uniqueUsername("round"));
  await openLedger(page, "Personal");
  await openSection(page, "Pots");
  await createPot(page, "Jar A");
  await createPot(page, "Jar B");
  await setRoundUp(page, "Jar A", "x1");
  await expect(page.getByRole("link", { name: /Jar A/ })).toContainText("Collects round-ups (x1)");

  await openPot(page, /Jar B/);
  await page.getByRole("button", { name: "x3", exact: true }).click();
  await expect(page.getByText("This replaces “Jar A”, which collects the round-ups now.")).toBeVisible();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("link", { name: /Jar B/ })).toContainText("Collects round-ups (x3)");
  await expect(page.getByRole("link", { name: /Jar A/ })).not.toContainText("Collects round-ups");

  // The main pot never collects.
  await openPot(page, /Main/);
  await expect(page.getByText("Round-ups", { exact: true }).filter({ visible: true })).toHaveCount(0);
});

test("a new pot can collect round-ups right away and replaces the old one", async ({ page }) => {
  await register(page, uniqueUsername("round"));
  await openLedger(page, "Personal");
  await openSection(page, "Pots");
  await createPot(page, "Jar A");
  await setRoundUp(page, "Jar A", "x1");
  await expect(page.getByRole("link", { name: /Jar A/ })).toContainText("Collects round-ups (x1)");

  await page.getByRole("link", { name: "New pot" }).click();
  await page.getByLabel("Name", { exact: true }).fill("Jar B");
  await page.getByRole("button", { name: "x2", exact: true }).click();
  await expect(page.getByText("This replaces “Jar A”, which collects the round-ups now.")).toBeVisible();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("link", { name: /Jar B/ })).toContainText("Collects round-ups (x2)");
  await expect(page.getByRole("link", { name: /Jar A/ })).not.toContainText("Collects round-ups");
});

test("a household has no round-ups", async ({ page }) => {
  await register(page, uniqueUsername("round"));
  await createHousehold(page, "Flat");
  await ledgerCard(page, "Flat").click();
  await openSection(page, "Pots");
  await page.getByRole("link", { name: "New pot" }).click();
  await expect(page.getByLabel("Name", { exact: true })).toBeVisible();
  await expect(page.getByText("Round-ups", { exact: true }).filter({ visible: true })).toHaveCount(0);
  await page.goBack();
  await createPot(page, "Jar");
  await openPot(page, /Jar/);
  await expect(page.getByText("Round-ups", { exact: true }).filter({ visible: true })).toHaveCount(0);
});
