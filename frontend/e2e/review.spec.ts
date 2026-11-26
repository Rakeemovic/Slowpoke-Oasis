import { expect, test, type Page } from "@playwright/test";

import { fillBooking, openLedger, register, uniqueUsername } from "./helpers";

/** Yesterday as YYYY-MM-DD in local time: a review covers the days up to yesterday. */
function yesterday(): string {
  const d = new Date();
  d.setDate(d.getDate() - 1);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

async function bookYesterday(page: Page, category: string, amount: string): Promise<void> {
  await page.getByRole("link", { name: "Add transaction" }).click();
  await fillBooking(page, { kind: "Expense", amount, category });
  await page.getByLabel("Date (YYYY-MM-DD)").fill(yesterday());
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Estimated account balance", { exact: true })).toBeVisible();
}

test("the weekly review looks back, marks wants and can be finished", async ({ page }) => {
  await register(page, uniqueUsername("review"));
  await openLedger(page, "Personal");
  const card = page.getByRole("link", { name: /^Weekly review / });
  // Nothing booked yet, so there is nothing to review.
  await expect(page.getByText("No transactions yet")).toBeVisible();
  await expect(card).toHaveCount(0);

  await bookYesterday(page, "Eating out", "18");
  await bookYesterday(page, "Groceries", "30");
  await card.click();

  await expect(page.getByText("Your week", { exact: true })).toBeVisible();
  await expect(page.getByText(/^18,00\s€ more than in the days before$/)).toBeVisible();
  await expect(page.getByText("Safe to spend", { exact: true }).filter({ visible: true })).toBeVisible();

  // Only the want can be marked, and the mark shows up in the wants line.
  const marks = page.getByRole("switch", { name: /^Spontaneous: / });
  await expect(marks).toHaveCount(1);
  await marks.click();
  await expect(page.getByText(/^18,00\s€ spontaneous \(100 % of wants\)$/)).toBeVisible();
  await expect(marks).toBeChecked();

  await page.getByRole("button", { name: "Finish review" }).click();
  await expect(page.getByText("Weekly review done")).toBeVisible();
  await expect(page.getByText("Estimated account balance", { exact: true }).filter({ visible: true })).toBeVisible();
  await expect(card).toHaveCount(0);
});

test("the review names categories over plan and leads to the budget", async ({ page }) => {
  await register(page, uniqueUsername("review"));
  await openLedger(page, "Personal");
  await page.getByRole("tab", { name: "Budget", exact: true }).click();
  await page.getByRole("button", { name: "Budget a category" }).click();
  await page.getByRole("button", { name: "Groceries", exact: true }).click();
  await page.getByLabel("Planned for Groceries (€)").fill("20");
  await page.getByRole("button", { name: "This and coming months", exact: true }).click();
  await expect(page.getByLabel("Planned for Groceries (€)")).toBeHidden();
  await page.getByRole("tab", { name: "Overview", exact: true }).click();

  // Booked today for the budget of this month; yesterday's booking opens the review.
  await page.getByRole("link", { name: "Add transaction" }).click();
  await fillBooking(page, { kind: "Expense", amount: "30", category: "Groceries" });
  await page.getByRole("button", { name: "Save" }).click();
  await bookYesterday(page, "Eating out", "5");

  await page.getByRole("link", { name: /^Weekly review / }).click();
  await expect(page.getByText(/^10,00\s€ over plan$/)).toBeVisible();
  await page.getByRole("link", { name: /^Open budget/ }).click();
  await expect(page.getByRole("button", { name: "Budget a category" })).toBeVisible();
});

test("the review day is chosen under account", async ({ page }) => {
  await register(page, uniqueUsername("review"));
  await page.getByRole("link", { name: "Account", exact: true }).click();
  const friday = page.getByRole("button", { name: "Friday", exact: true });
  await expect(page.getByRole("button", { name: "Monday", exact: true })).toHaveAttribute("aria-selected", "true");
  await friday.click();
  await expect(page.getByText("Review day saved")).toBeVisible();
  await page.reload();
  await expect(friday).toHaveAttribute("aria-selected", "true");
});
