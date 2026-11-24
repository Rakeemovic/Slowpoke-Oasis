import { expect, test, type Page } from "@playwright/test";

import { bar, euros, openLedger, register, uniqueUsername } from "./helpers";

test.beforeEach(async ({ page }) => {
  await register(page, uniqueUsername("goal"));
  await openLedger(page, "Personal");
});

/** Creates a pot with a goal: a target, a monthly amount, or both. */
async function createGoal(
  page: Page,
  name: string,
  goal: { kind: "Target" | "Monthly amount"; amount?: string; target?: string; date?: string },
): Promise<void> {
  await page.getByRole("link", { name: "Pots", exact: true }).click();
  await page.getByRole("link", { name: "New pot" }).click();
  await page.getByLabel("Name", { exact: true }).fill(name);
  await page.getByRole("button", { name: goal.kind, exact: true }).click();
  if (goal.amount) {
    await page.getByLabel("Save per month (€)").fill(goal.amount);
  }
  if (goal.target) {
    await page.getByLabel(/^Target amount \(€/).fill(goal.target);
  }
  if (goal.date) {
    await page.getByLabel("Goal date (YYYY-MM-DD, optional)").fill(goal.date);
  }
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("link", { name: "New pot" })).toBeVisible();
}

/** A row of the "Due payments" card whose first line is title. */
function cardRow(page: Page, title: string) {
  return page
    .getByText(title, { exact: true })
    .locator("xpath=ancestor::div[.//*[normalize-space()='Book' or normalize-space()='Transfer' or normalize-space()='Skip']][1]")
    .filter({ visible: true });
}

test("a goal with a monthly amount shows its progress and asks for its rate everywhere", async ({ page }) => {
  await createGoal(page, "Bike", { kind: "Monthly amount", amount: "200", target: "500" });

  // The pot list groups it under Goals; its row is the progress bar.
  await expect(page.getByText("Goals", { exact: true })).toBeVisible();
  const row = page.getByRole("link", { name: /Bike/ });
  await expect(row).toContainText(euros("Goal 500,00"));
  await expect(row).toContainText(/200,00\s€ per month/);
  await expect(bar(page, "Bike")).toHaveAttribute("aria-valuenow", "0");
  await page.goBack();

  // The rate of this period is due, with a Transfer into the pot.
  await expect(cardRow(page, "Save for Bike")).toContainText(euros("200,00"));
  await page.getByRole("link", { name: "Transfer to Bike" }).click();
  await expect(page.getByLabel("Amount (€)")).toHaveValue("200.00");
  await page.getByLabel("Amount (€)").fill("50");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(cardRow(page, "Save for Bike")).toContainText(euros("150,00"));

  // Safe to spend holds back what is still to be saved.
  await page.getByRole("button", { name: /^Safe to spend: / }).filter({ visible: true }).click();
  const breakdown = page.getByLabel("Safe to spend breakdown").filter({ visible: true });
  await expect(breakdown).toContainText("Savings goals to save");
  await expect(breakdown).toContainText(euros("-150,00"));

  // The budget has a section of its own, filled by what was saved.
  await page.getByRole("link", { name: "Budget", exact: true }).click();
  await expect(page.getByText("Savings", { exact: true })).toBeVisible();
  await expect(page.getByText(/^50,00\s€ of 200,00\s€ saved$/)).toBeVisible();
  await expect(page.getByText(/^150,00\s€ to save$/)).toBeVisible();
  await expect(bar(page, "Bike")).toHaveAttribute("aria-valuenow", "25");
  await page.goBack();

  // The pot shows how far it is: 50 of 500 €.
  await page.getByRole("link", { name: "Pots", exact: true }).click();
  await expect(bar(page, "Bike")).toHaveAttribute("aria-valuenow", "10");
  await page.getByRole("link", { name: /Bike/ }).click();
  await expect(page.getByText(/^50,00\s€ of 500,00\s€$/)).toBeVisible();
  await expect(page.getByText(/This period: Monthly amount · 50,00\s€ of 200,00\s€ saved/)).toBeVisible();
});

test("a passed target date makes the rest due now, and removing the goal ends the rate", async ({ page }) => {
  await createGoal(page, "Old plan", { kind: "Target", target: "300", date: "2020-01-15" });
  await page.goBack();

  await expect(cardRow(page, "Save for Old plan")).toContainText(euros("300,00"));
  await expect(cardRow(page, "Save for Old plan")).toContainText("Target date passed");

  // Without a goal the pot asks for nothing, and the card disappears.
  await page.getByRole("link", { name: "Pots", exact: true }).click();
  await page.getByRole("link", { name: /Old plan/ }).click();
  await page.getByRole("button", { name: "No goal", exact: true }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("link", { name: "New pot" })).toBeVisible();
  await page.goBack();
  await expect(page.getByText("Estimated account balance", { exact: true })).toBeVisible();
  await expect(page.getByText("Due payments", { exact: true })).toBeHidden();
});

test("the form only offers the fields of the chosen kind", async ({ page }) => {
  await page.getByRole("link", { name: "Pots", exact: true }).click();
  await page.getByRole("link", { name: "New pot" }).click();

  await expect(page.getByLabel(/^Target amount/)).toHaveCount(0);
  await page.getByRole("button", { name: "Target", exact: true }).click();
  await expect(page.getByLabel("Goal date (YYYY-MM-DD, optional)")).toBeVisible();
  await expect(page.getByLabel("Save per month (€)")).toHaveCount(0);

  // A fixed amount and a date exclude each other, so there is no date field.
  await page.getByRole("button", { name: "Monthly amount", exact: true }).click();
  await expect(page.getByLabel("Save per month (€)")).toBeVisible();
  await expect(page.getByLabel("Goal date (YYYY-MM-DD, optional)")).toHaveCount(0);

  await page.getByLabel("Name", { exact: true }).fill("Broken");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText(/Enter the monthly amount greater than zero/)).toBeVisible();
});
