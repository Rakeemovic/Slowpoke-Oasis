import { expect, test, type Page } from "@playwright/test";

import { balance, openLedger, register, uniqueUsername } from "./helpers";

// The test server delivers app and API under one origin; relative paths go to
// Playwright's baseURL and carry the session cookie of the page.
const API = "/api";
// Requests authenticated by cookie must carry this header (rule A8).
const HEADERS = { "X-Requested-With": "slowpoke-e2e" };

/** Books `count` small expenses through the API (far faster than the form) and returns the ledger and their ids. */
async function seedExpenses(page: Page, count: number): Promise<{ ledger: string; ids: string[] }> {
  const me = await (await page.request.get(`${API}/me`, { headers: HEADERS })).json();
  const ledger = me.personal_ledger_id as string;
  const category = await (
    await page.request.post(`${API}/ledgers/${ledger}/categories`, {
      headers: HEADERS,
      data: { name: "Snacks", is_income: false },
    })
  ).json();
  const ids: string[] = [];
  for (let i = 1; i <= count; i++) {
    const response = await page.request.post(`${API}/ledgers/${ledger}/transactions`, {
      headers: HEADERS,
      data: { type: "expense", category_id: category.id, amount_cents: 100, booked_on: "2026-01-01", note: `Snack ${i}` },
    });
    expect(response.ok()).toBeTruthy();
    ids.push((await response.json()).id);
  }
  return { ledger, ids };
}

test("a long list is shown in pages", async ({ page }) => {
  await register(page, uniqueUsername("page"));
  await seedExpenses(page, 105);
  await openLedger(page, "Personal");

  const rows = page.locator('a[href*="/transactions/"]');
  await expect(rows).toHaveCount(100);

  await page.getByRole("button", { name: "Load more" }).click();
  await expect(rows).toHaveCount(105);
  // The last page was not full, so there is nothing more to load.
  await expect(page.getByRole("button", { name: "Load more" })).toHaveCount(0);
  await expect(balance(page, "-105,00")).toBeVisible();

  // Returning from another screen reloads the list but keeps what was already shown.
  await rows.first().click();
  await expect(page.getByLabel("Amount (€)")).toBeVisible();
  await page.goBack();
  await expect(page.getByText("Estimated account balance", { exact: true })).toBeVisible();
  await expect(rows).toHaveCount(105);
});

test("the list of deleted transactions is shown in pages", async ({ page }) => {
  await register(page, uniqueUsername("page"));
  const { ledger, ids } = await seedExpenses(page, 105);
  for (const id of ids) {
    const response = await page.request.delete(`${API}/ledgers/${ledger}/transactions/${id}`, { headers: HEADERS });
    expect(response.ok()).toBeTruthy();
  }
  await openLedger(page, "Personal");
  await page.getByRole("link", { name: "Deleted transactions" }).click();

  const restoreButtons = page.getByRole("button", { name: /^Restore / });
  await expect(restoreButtons).toHaveCount(100);
  await page.getByRole("button", { name: "Load more" }).click();
  await expect(restoreButtons).toHaveCount(105);
  await expect(page.getByRole("button", { name: "Load more" })).toHaveCount(0);

  // Restoring reloads the list; the entries already shown stay, minus the restored one.
  await restoreButtons.first().click();
  await expect(restoreButtons).toHaveCount(104);
});
