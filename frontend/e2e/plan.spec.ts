import { expect, test, type Locator, type Page } from "@playwright/test";

import { book, euros, openLedger, register, uniqueUsername } from "./helpers";

test.beforeEach(async ({ page }) => {
  await register(page, uniqueUsername("plan"));
  await openLedger(page, "Personal");
});

/** A row of the plan; it is a button while the plan can be changed. */
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

test("a one-time pot has a single plan without months", async ({ page }) => {
  await page.getByRole("link", { name: "Pots", exact: true }).click();
  await page.getByRole("link", { name: "New pot" }).click();
  await page.getByLabel("Name", { exact: true }).fill("Holiday");
  await page.getByRole("button", { name: "Save" }).click();
  await page.getByRole("link", { name: /Holiday/ }).click();
  await page.getByRole("link", { name: "Budget", exact: true }).click();

  await expect(page.getByRole("button", { name: "Next month" })).toHaveCount(0);
  await page.getByRole("button", { name: "Budget a category" }).click();
  await page.getByRole("button", { name: "Leisure", exact: true }).click();
  await setAmount(page, "Leisure", "200", "Save");
  await expect(planRow(page, "Leisure")).toContainText(euros("200,00"));
  await expect(page.getByText(/^Spent 0,00\s€ of 200,00\s€ planned$/)).toBeVisible();
});
