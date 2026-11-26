import { expect, test, type Page } from "@playwright/test";

import { balance, euros, openLedger, register, transactionRow, uniqueUsername, openSection, backToOverview } from "./helpers";

test.beforeEach(async ({ page }) => {
  await register(page, uniqueUsername("res"));
  await openLedger(page, "Personal");
});

/**
 * Creates a yearly payment due today, so its reserve's rate is the whole
 * amount in the current period, whatever date the test runs on.
 */
async function createYearlyDueToday(page: Page, name: string, amount: string): Promise<void> {
  await openSection(page, "Recurring");
  await page.getByRole("link", { name: "New recurring payment" }).click();
  await page.getByLabel("Name", { exact: true }).fill(name);
  await page.getByLabel("Amount (€)").fill(amount);
  await page.getByRole("button", { name: "Insurance", exact: true }).click();
  await page.getByRole("button", { name: "Yearly", exact: true }).click();
  // No pot choice: the payment gets its own reserve.
  await expect(page.getByText("It is paid from its own reserve pot.", { exact: false })).toBeVisible();
  await page.getByLabel("Due day (1–31)").fill(String(new Date().getDate()));
  await page.getByRole("button", { name: "Save" }).click();
  // The row of the new payment; earlier payments show the same schedule.
  await expect(page.getByRole("link", { name: new RegExp(`^${name} Yearly on day`) })).toBeVisible();
  await backToOverview(page);
}

/** A row of the "Due payments" card whose first line is title. */
function cardRow(page: Page, title: string) {
  return page
    .getByText(title, { exact: true })
    .locator("xpath=ancestor::div[.//*[normalize-space()='Book' or normalize-space()='Transfer' or normalize-space()='Skip']][1]")
    .filter({ visible: true });
}

test("a reserve is filled by its rate and topped up when booking", async ({ page }) => {
  await createYearlyDueToday(page, "Car", "600");
  await expect(cardRow(page, "Save for Car")).toContainText(euros("600,00"));

  // The reserve shows up as its own group of pots.
  await openSection(page, "Pots");
  await expect(page.getByText("Reserves", { exact: true })).toBeVisible();
  await page.goBack();

  // Transfer starts prefilled from the default pot; a part of the rate counts too.
  await page.getByRole("link", { name: "Transfer to Car" }).click();
  await expect(page.getByLabel("Amount (€)")).toHaveValue("600.00");
  await page.getByLabel("Amount (€)").fill("200");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(cardRow(page, "Save for Car")).toContainText(euros("400,00"));

  // The reserve holds 200 €; booking 600 € from it offers to top it up.
  await page.getByRole("link", { name: "Book Car" }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText(/The reserve lacks 400,00\s€/)).toBeVisible();
  await page.getByRole("button", { name: /Top up 400,00\s€ and book/ }).click();

  await expect(transactionRow(page, "Insurance")).toContainText(euros("-600,00"));
  await expect(balance(page, "-600,00")).toBeVisible();
  await expect(page.getByRole("link", { name: "Book Car" })).toHaveCount(0);
});

test("archiving a payment offers to archive its reserve", async ({ page }) => {
  await createYearlyDueToday(page, "Bike", "120");
  await page.getByRole("link", { name: "Transfer to Bike" }).click();
  await page.getByLabel("Amount (€)").fill("50");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(cardRow(page, "Save for Bike")).toContainText(euros("70,00"));

  await openSection(page, "Recurring");
  await linkRow(page, "Bike").click();
  await page.getByRole("button", { name: "Archive payment" }).click();
  await page.getByRole("button", { name: /Move 50,00\s€ and archive/ }).click();
  // The app goes back to the list by itself once the move is saved; going
  // back before that would race with it.
  await expect(page).toHaveURL(/\/recurring$/);

  // The money is back in the default pot; the reserve is archived and empty.
  await backToOverview(page);
  await expect(page.getByText("Due payments", { exact: true })).toBeHidden();
  await openSection(page, "Pots");
  await expect(page.getByText("Reserves", { exact: true })).toBeHidden();
  await expect(linkRow(page, "Bike")).toContainText(euros("0,00"));
  await expect(linkRow(page, "Main")).toContainText(euros("0,00"));
});

test("renaming a payment renames its reserve, unless the user named it", async ({ page }) => {
  await createYearlyDueToday(page, "Car", "600");
  await createYearlyDueToday(page, "Bike", "120");

  // The user names the reserve of the bike themselves.
  await openSection(page, "Pots");
  await linkRow(page, "Bike").click();
  await page.getByLabel("Name", { exact: true }).fill("Bike cushion");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(linkRow(page, "Bike cushion")).toBeVisible();

  // Both payments get new names; only the reserve with its automatic name follows.
  for (const [from, to] of [["Car", "Family car"], ["Bike", "City bike"]]) {
    await openSection(page, "Recurring");
    await linkRow(page, from).click();
    await page.getByLabel("Name", { exact: true }).fill(to);
    await page.getByRole("button", { name: "Save" }).click();
    // The app goes back to the list by itself once the change is saved.
    await expect(page).toHaveURL(/\/recurring$/);
    await backToOverview(page);
  }
  await openSection(page, "Pots");
  await expect(linkRow(page, "Family car")).toBeVisible();
  await expect(linkRow(page, "Bike cushion")).toBeVisible();
  await expect(linkRow(page, "City bike")).toHaveCount(0);
});

/** A row on a list of payments or pots; earlier screens stay hidden in the DOM. */
function linkRow(page: Page, name: string) {
  return page.getByRole("link").filter({ has: page.getByText(name, { exact: true }), visible: true });
}
