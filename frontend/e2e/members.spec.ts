import { expect, test } from "@playwright/test";

import { book, createHousehold, ledgerCard, openLedger, register, uniqueUsername, openSection, backToOverview } from "./helpers";

test("members share a household, hand over ownership and leave", async ({ page: ownerPage, browser }) => {
  // Two separate browser contexts: two users with their own sessions at once.
  const memberContext = await browser.newContext();
  const memberPage = await memberContext.newPage();
  const owner = uniqueUsername("own");
  const member = uniqueUsername("mem");

  await register(ownerPage, owner);
  await register(memberPage, member);

  // The owner creates a household and adds the member by username.
  await createHousehold(ownerPage, "Oasis");
  await openLedger(ownerPage, "Oasis");
  await openSection(ownerPage, "Members");
  await ownerPage.getByLabel("Add member").fill(member);
  await ownerPage.getByRole("button", { name: "Add member" }).click();
  await expect(ownerPage.getByText(member)).toBeVisible();

  // The member sees the household after a refresh and books into it.
  await memberPage.getByRole("button", { name: "Refresh" }).click();
  await openLedger(memberPage, "Oasis");
  await book(memberPage, { kind: "Expense", amount: "25", category: "Snacks", newCategory: true });

  // The owner sees who booked it. It is not theirs, so the row opens nothing.
  await backToOverview(ownerPage);
  await ownerPage.getByRole("button", { name: "Refresh" }).click();
  await expect(ownerPage.getByText("Snacks")).toBeVisible();
  await expect(ownerPage.getByText(member, { exact: true })).toBeVisible();

  // Handing over ownership: the old owner stays as a plain member.
  await openSection(ownerPage, "Members");
  await ownerPage.getByRole("button", { name: "Make owner" }).click();
  await ownerPage.getByRole("alertdialog").getByRole("button", { name: "Make owner" }).click();
  await expect(ownerPage.getByRole("button", { name: "Make owner" })).toHaveCount(0);
  await expect(ownerPage.getByLabel("Add member")).toHaveCount(0);

  // Leaving returns to the overview, where the household is gone.
  await ownerPage.getByRole("button", { name: "Leave household" }).click();
  await ownerPage.getByRole("alertdialog").getByRole("button", { name: "Leave" }).click();
  await expect(ownerPage.getByText("No household yet", { exact: true })).toBeVisible();
  await expect(ledgerCard(ownerPage, "Oasis")).toHaveCount(0);

  // The member is now the only one left, and the owner.
  await openSection(memberPage, "Members");
  await expect(memberPage.getByText(owner)).toHaveCount(0);
  await expect(memberPage.getByText("Owner", { exact: true })).toBeVisible();

  await memberContext.close();
});
