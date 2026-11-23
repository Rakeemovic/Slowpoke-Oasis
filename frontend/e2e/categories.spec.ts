import { expect, test, type Page } from "@playwright/test";

import { book, createHousehold, fillBooking, openLedger, register, transactionRow, uniqueUsername } from "./helpers";

/** Opens the categories screen from the ledger screen. */
async function openCategories(page: Page): Promise<void> {
  await page.getByRole("link", { name: "Categories", exact: true }).click();
  await expect(page.getByRole("link", { name: "New category" })).toBeVisible();
}

/** A category row on the categories screen; earlier screens stay hidden in the DOM. */
function categoryRow(page: Page, name: string) {
  return page.getByRole("link").filter({ has: page.getByText(name, { exact: true }), visible: true });
}

test("the owner renames, archives and deletes categories; bookings can be spontaneous", async ({ page }) => {
  await register(page, uniqueUsername("cat"));
  await openLedger(page, "Personal");

  // A spontaneous expense shows the mark in the list.
  await page.getByRole("link", { name: "Add transaction" }).click();
  await fillBooking(page, { kind: "Expense", amount: "12,50", category: "Leisure" });
  await page.getByRole("button", { name: "⚡ Spontaneous" }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(transactionRow(page, "Leisure ⚡")).toBeVisible();
  await book(page, { kind: "Expense", amount: "5", category: "Groceries" });

  // Default categories come with a purpose and count their bookings.
  await openCategories(page);
  await expect(categoryRow(page, "Leisure")).toContainText("Want · 1 booking");
  await expect(categoryRow(page, "Other")).toContainText("Catch-all");

  // Rename and change the purpose.
  await categoryRow(page, "Leisure").click();
  await page.getByLabel("Name", { exact: true }).fill("Fun");
  await page.getByRole("button", { name: "Need", exact: true }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(categoryRow(page, "Fun")).toContainText("Need · 1 booking");

  // Archived categories move to their own group and cannot be booked on.
  await categoryRow(page, "Rent").click();
  await page.getByRole("button", { name: "Archive category" }).click();
  await expect(page.getByText("Archived", { exact: true })).toBeVisible();

  // Deleting names the consequences, then moves the booking to "Other".
  await categoryRow(page, "Fun").click();
  await page.getByRole("button", { name: "Delete category" }).click();
  await expect(page.getByRole("alertdialog")).toContainText("Its 1 booking and planned amounts move to “Other”.");
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(categoryRow(page, "Fun")).toHaveCount(0);
  await expect(categoryRow(page, "Other")).toContainText("1 booking");

  // The owner may choose another category to take over the bookings.
  await categoryRow(page, "Groceries").click();
  await page.getByRole("button", { name: "Delete category" }).click();
  await page.getByRole("button", { name: "Household", exact: true }).click();
  await expect(page.getByRole("alertdialog")).toContainText("Its 1 booking and planned amounts move to “Household”.");
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(categoryRow(page, "Groceries")).toHaveCount(0);
  await expect(categoryRow(page, "Household")).toContainText("1 booking");

  // The catch-all category cannot be deleted.
  await categoryRow(page, "Other").click();
  await expect(page.getByText("This is the catch-all category", { exact: false })).toBeVisible();
  await expect(page.getByRole("button", { name: "Delete category" })).toHaveCount(0);
  await page.goBack();
  await page.goBack();

  await expect(transactionRow(page, "Other ⚡")).toBeVisible();
  await page.getByRole("link", { name: "Add transaction" }).click();
  await expect(page.getByRole("button", { name: "Household", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Rent", exact: true })).toHaveCount(0);
});

test("household members add categories, only the owner changes them", async ({ page: ownerPage, browser }) => {
  const memberContext = await browser.newContext();
  const memberPage = await memberContext.newPage();
  const member = uniqueUsername("cmem");
  await register(ownerPage, uniqueUsername("cown"));
  await register(memberPage, member);

  await createHousehold(ownerPage, "Flat");
  await openLedger(ownerPage, "Flat");
  await ownerPage.getByRole("link", { name: "Members" }).click();
  await ownerPage.getByLabel("Add member").fill(member);
  await ownerPage.getByRole("button", { name: "Add member" }).click();
  await expect(ownerPage.getByText(member)).toBeVisible();

  await memberPage.getByRole("button", { name: "Refresh" }).click();
  await openLedger(memberPage, "Flat");
  await openCategories(memberPage);
  await expect(memberPage.getByText("Only the owner of the household can change or delete categories.")).toBeVisible();
  // For members the rows are plain text, not links.
  await expect(memberPage.getByText("Groceries", { exact: true })).toBeVisible();
  await expect(categoryRow(memberPage, "Groceries")).toHaveCount(0);

  // A member may still add a category, with a purpose.
  await memberPage.getByRole("link", { name: "New category" }).click();
  await memberPage.getByLabel("Name", { exact: true }).fill("Plants");
  await memberPage.getByRole("button", { name: "Want", exact: true }).click();
  await memberPage.getByRole("button", { name: "Save" }).click();
  await expect(memberPage.getByText("Plants", { exact: true })).toBeVisible();

  // The member's spontaneous booking: the owner may edit it but not change the mark.
  await memberPage.goBack();
  await memberPage.getByRole("link", { name: "Add transaction" }).click();
  await fillBooking(memberPage, { kind: "Expense", amount: "8", category: "Plants" });
  await memberPage.getByRole("button", { name: "⚡ Spontaneous" }).click();
  await memberPage.getByRole("button", { name: "Save" }).click();
  await expect(memberPage.getByText("Estimated account balance", { exact: true })).toBeVisible();

  await ownerPage.goBack();
  await ownerPage.getByRole("button", { name: "Refresh" }).click();
  await transactionRow(ownerPage, "Plants ⚡").click();
  await expect(ownerPage.getByRole("button", { name: "⚡ Spontaneous" })).toHaveCount(0);
  await ownerPage.getByLabel("Amount (€)").fill("9");
  await ownerPage.getByRole("button", { name: "Save" }).click();
  await expect(transactionRow(ownerPage, "Plants ⚡")).toBeVisible();

  // The owner can open the member's category.
  await openCategories(ownerPage);
  await expect(categoryRow(ownerPage, "Plants")).toBeVisible();

  await memberContext.close();
});
