import { expect, test } from "@playwright/test";

import { PASSWORD, expectOverview, ledgerCard, register, signIn, uniqueUsername } from "./helpers";

test("a new account starts with an empty personal ledger", async ({ page }) => {
  await register(page, uniqueUsername("acc"));

  // Rule L6: registration creates the personal ledger right away.
  await expect(ledgerCard(page, "Personal")).toBeVisible();
  await expect(page.getByText("You are not in a household yet.", { exact: false })).toBeVisible();
});

test("the session survives a reload and ends on sign out", async ({ page }) => {
  const username = uniqueUsername("acc");
  await register(page, username);

  await page.reload();
  await expectOverview(page);

  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();

  // Signed out means signed out, also after a reload.
  await page.reload();
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();

  await signIn(page, username);
  await expectOverview(page);
});

test("a wrong password shows an error and keeps the user signed out", async ({ page }) => {
  const username = uniqueUsername("acc");
  await register(page, username);
  await page.getByRole("button", { name: "Sign out" }).click();

  await signIn(page, username, `${PASSWORD}!`);
  await expect(page.getByText("wrong username or password")).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
});
