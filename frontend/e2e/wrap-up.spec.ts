import { expect, test, type Page } from "@playwright/test";

import { fillBooking, openLedger, openSection, register, uniqueUsername } from "./helpers";

/** A day of the current month; paydays start on the 1st here, so it is also a day of the period. */
function dayOfThisMonth(day: number): Date {
  const now = new Date();
  return new Date(now.getFullYear(), now.getMonth(), day, 12);
}

const before = new Date(dayOfThisMonth(1).setMonth(new Date().getMonth() - 1));
const previousMonth = `${before.getFullYear()}-${String(before.getMonth() + 1).padStart(2, "0")}`;
const previousName = before.toLocaleDateString("de-DE", { month: "long" });

/** Books an income and an expense in the month before and returns to the ledger screen. */
async function bookLastMonth(page: Page): Promise<void> {
  for (const booking of [
    { kind: "Income" as const, amount: "100", category: "Salary" },
    { kind: "Expense" as const, amount: "130", category: "Groceries" },
  ]) {
    await page.getByRole("link", { name: "Add transaction" }).click();
    await fillBooking(page, booking);
    await page.getByLabel("Date (YYYY-MM-DD)").fill(`${previousMonth}-15`);
    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.getByText("Estimated account balance", { exact: true })).toBeVisible();
  }
}

test("a new period points to the finished one until dismissed", async ({ page }) => {
  // The app takes "today" from the browser, so the test chooses the day.
  await page.clock.setFixedTime(dayOfThisMonth(2));
  await register(page, uniqueUsername("wrapup"));
  await openLedger(page, "Personal");
  const headline = page.getByText(/ is over$/).filter({ visible: true });
  await expect(headline).toHaveCount(0);

  await bookLastMonth(page);
  await page.reload();
  await expect(headline).toHaveText(`${previousName} is over`);

  await page.getByRole("link", { name: "View insights" }).click();
  await expect(page.getByLabel(/^Left over: -30,00\s€$/)).toBeVisible();
  await page.goBack();

  await page.getByRole("button", { name: "Dismiss" }).click();
  await expect(headline).toHaveCount(0);
  await page.reload();
  await expect(page.getByText("Estimated account balance", { exact: true })).toBeVisible();
  await expect(headline).toHaveCount(0);
});

test("after the first week the finished period is not pointed out", async ({ page }) => {
  await page.clock.setFixedTime(dayOfThisMonth(8));
  await register(page, uniqueUsername("wrapup"));
  await openLedger(page, "Personal");
  await bookLastMonth(page);
  const report = page.waitForResponse((r) => r.url().includes("/purpose-report"));
  await page.reload();
  await report;
  await expect(page.getByText(/ is over$/)).toHaveCount(0);
});

test("the budget leads to the insights of the month it shows", async ({ page }) => {
  await register(page, uniqueUsername("wrapup"));
  await openLedger(page, "Personal");
  await bookLastMonth(page);

  await openSection(page, "Budget");
  await page.getByRole("button", { name: "Previous month" }).click();
  await page.getByRole("link", { name: new RegExp(`Needs, wants and savings in ${previousName}`) }).click();
  await expect(page.getByLabel(/^Left over: -30,00\s€$/)).toBeVisible();
});
