import { expect, test } from "@playwright/test";

import { PASSWORD, expectOverview, ledgerCard, recoveryCode, register, signIn, uniqueUsername } from "./helpers";

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

test("a forgotten password can be reset with the recovery code", async ({ page }) => {
  const username = uniqueUsername("acc");
  const code = await register(page, username);
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByRole("link", { name: "Forgot your password?" })).toBeVisible();

  // Opened directly, so the hidden sign-in screen does not add a second "Username" field.
  await page.goto("/recover");
  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Recovery code").fill("abacus zoom abacus zoom abacus zoom");
  await page.getByLabel("New password").fill("brand new password");
  await page.getByRole("button", { name: "Reset password" }).click();
  await expect(page.getByText("wrong username or recovery code")).toBeVisible();

  // Typed in capitals, as people might copy it from paper.
  await page.getByLabel("Recovery code").fill(code.toUpperCase());
  await page.getByRole("button", { name: "Reset password" }).click();
  const newCode = await recoveryCode(page);
  expect(newCode).not.toBe(code);
  await page.getByRole("button", { name: "I have saved it, sign in" }).click();
  await expectOverview(page);

  await page.getByRole("button", { name: "Sign out" }).click();
  await signIn(page, username, "brand new password");
  await expectOverview(page);
});

test("a signed-in user can change the password", async ({ page }) => {
  const username = uniqueUsername("acc");
  await register(page, username);

  await page.getByRole("link", { name: "Account" }).click();
  await page.getByLabel("Current password").fill(PASSWORD);
  await page.getByLabel("New password").fill("brand new password");
  await page.getByRole("button", { name: "Change password" }).click();
  await expect(page.getByText("Password changed.", { exact: false })).toBeVisible();

  await page.goto("/");
  await page.getByRole("button", { name: "Sign out" }).click();
  await signIn(page, username, "brand new password");
  await expectOverview(page);
});

test("the recovery code can be copied to the clipboard", async ({ page, context }) => {
  // Chromium allows clipboard access on localhost once the permissions are granted.
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("/register");
  await page.getByLabel("Username").fill(uniqueUsername("acc"));
  await page.getByLabel("Display name").fill("Copy");
  await page.getByLabel("Password").fill(PASSWORD);
  await page.getByRole("button", { name: "Create account" }).click();
  const code = await recoveryCode(page);

  await page.getByRole("button", { name: "Copy code" }).click();
  await expect(page.getByRole("button", { name: "Copied" })).toBeVisible();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(code);
});
