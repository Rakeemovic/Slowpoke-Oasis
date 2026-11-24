import { expect, test, type Locator, type Page } from "@playwright/test";

import { book, createPaymentDueToday, euros, openLedger, register, uniqueUsername } from "./helpers";

test.beforeEach(async ({ page }) => {
  await register(page, uniqueUsername("budget"));
  await openLedger(page, "Personal");
});

/** The top row of a category; it is a button while the budget can be changed. */
function planRow(page: Page, category: string): Locator {
  return page.getByRole("button", { name: new RegExp(`^${category}`) });
}

async function setAmount(page: Page, category: string, amount: string, save: string): Promise<void> {
  await page.getByLabel(`Planned for ${category} (€)`).fill(amount);
  await page.getByRole("button", { name: save, exact: true }).click();
  await expect(page.getByLabel(`Planned for ${category} (€)`)).toBeHidden();
}

test("the monthly plan changes this month or also the coming ones", async ({ page }) => {
  await book(page, { kind: "Expense", amount: "40", category: "Groceries" });
  await page.getByRole("link", { name: "Budget", exact: true }).click();

  // A booking without a plan still shows up.
  await expect(planRow(page, "Groceries")).toContainText(euros("40,00"));
  await expect(planRow(page, "Groceries")).toContainText("not planned");

  await planRow(page, "Groceries").click();
  await setAmount(page, "Groceries", "100", "This and coming months");
  await expect(planRow(page, "Groceries")).toContainText(euros("60,00"));
  await expect(page.getByText(/^Spent 40,00\s€ of 100,00\s€ planned$/)).toBeVisible();

  // Only this month: the coming month keeps the template.
  await planRow(page, "Groceries").click();
  await setAmount(page, "Groceries", "120", "Only this month");
  await expect(planRow(page, "Groceries")).toContainText(euros("80,00"));

  await page.getByRole("button", { name: "Next month" }).click();
  await expect(page.getByText(/^Preview from the budget/)).toBeVisible();
  // A preview cannot be changed, so its rows are no buttons.
  await expect(page.getByText(/^Spent 0,00\s€ of 100,00\s€ planned$/)).toBeVisible();
  await expect(planRow(page, "Groceries")).toHaveCount(0);

  await page.getByRole("button", { name: "Back to this month" }).click();
  await expect(planRow(page, "Groceries")).toContainText(euros("80,00"));

  // The month before the account existed has no plan.
  await page.getByRole("button", { name: "Previous month" }).click();
  await expect(page.getByText("No budget was made for this month.")).toBeVisible();
  await page.getByRole("button", { name: "Next month" }).click();

  // A category without bookings can be planned and removed again.
  await page.getByRole("button", { name: "Budget a category" }).click();
  await page.getByRole("button", { name: "Rent", exact: true }).click();
  await setAmount(page, "Rent", "500", "Only this month");
  await expect(planRow(page, "Rent")).toContainText(euros("500,00"));

  await planRow(page, "Rent").click();
  await page.getByRole("button", { name: "Remove from plan" }).click();
  await page.getByRole("button", { name: "Only this month", exact: true }).click();
  await expect(planRow(page, "Rent")).toHaveCount(0);
  await expect(page.getByText(/^Spent 40,00\s€ of 120,00\s€ planned$/)).toBeVisible();
});

/** A line below a category, found by its first text. */
function subRow(page: Page, title: string): Locator {
  return page.getByText(title, { exact: true }).locator("xpath=../..");
}

test("payments and reserve rates appear below their category", async ({ page }) => {
  await createPaymentDueToday(page, "Flat", "800", "Rent");
  await createPaymentDueToday(page, "Car", "600", "Insurance", true);
  await page.getByRole("link", { name: "Budget", exact: true }).click();

  // The monthly payment is a line of its own below the category.
  await expect(planRow(page, "Rent")).toContainText("Nothing else planned");
  await expect(subRow(page, "↻ Flat")).toContainText(/0,00\s€ of 800,00\s€ · due/);
  await expect(subRow(page, "↻ Flat")).toContainText("Open");

  // The yearly payment saves its rate and is paid from its reserve this month.
  await expect(subRow(page, "→ Save for Car")).toContainText(/600,00\s€ to save/);
  await expect(page.getByText(/^600,00\s€ due .* from reserve “Car” \(covered: 0,00\s€\)$/)).toBeVisible();

  // Plan, payments and rates count together; the payment paid from the
  // reserve does not.
  await expect(page.getByText(/^Spent 0,00\s€ of 1\.400,00\s€ planned$/)).toBeVisible();

  // Booking the monthly payment fills its line, not the free plan.
  await page.goBack();
  await page.getByRole("link", { name: "Book Flat" }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await page.getByRole("link", { name: "Budget", exact: true }).click();
  await expect(subRow(page, "↻ Flat")).toContainText("Paid");
  await expect(planRow(page, "Rent")).toContainText("Nothing else planned");
  await expect(page.getByText(/^Spent 800,00\s€ of 1\.400,00\s€ planned$/)).toBeVisible();
});
