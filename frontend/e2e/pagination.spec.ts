import { expect, test, type Page } from "@playwright/test";

import { balance, openLedger, register, uniqueUsername } from "./helpers";

const API = "http://localhost:9090";
// Requests authenticated by cookie must carry this header (rule A8).
const HEADERS = { "X-Requested-With": "slowpoke-e2e" };

/** Books `count` small expenses through the API: far faster than the form. */
async function seedExpenses(page: Page, count: number): Promise<void> {
  const me = await (await page.request.get(`${API}/me`, { headers: HEADERS })).json();
  const ledger = me.personal_ledger_id as string;
  const category = await (
    await page.request.post(`${API}/ledgers/${ledger}/categories`, {
      headers: HEADERS,
      data: { name: "Snacks", is_income: false },
    })
  ).json();
  for (let i = 1; i <= count; i++) {
    const response = await page.request.post(`${API}/ledgers/${ledger}/transactions`, {
      headers: HEADERS,
      data: { type: "expense", category_id: category.id, amount_cents: 100, booked_on: "2026-01-01", note: `Snack ${i}` },
    });
    expect(response.ok()).toBeTruthy();
  }
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
  await expect(page.getByText("Available today")).toBeVisible();
  await expect(rows).toHaveCount(105);
});
