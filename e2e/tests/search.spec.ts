import { test, expect } from '../fixtures';
import { Page, request as playwrightRequest } from '@playwright/test';

async function createArea(page: Page, name: string): Promise<string> {
  await page.goto('/areas');
  await page.click('[data-testid="new-area-btn"]');
  await page.locator('#new-area-dialog').waitFor({ state: 'visible' });
  await page.fill('#new-area-dialog input[name="name"]', name);
  await page.click('#new-area-dialog button[type="submit"]');
  const card = page.locator('.area-card', { hasText: name });
  await card.waitFor({ timeout: 10_000 });
  const testid = await card.getAttribute('data-testid');
  return testid!.replace('area-card-', '');
}

/**
 * Create an area and seed it with the same 3 items the mock-ollama returns
 * (Milk, Butter, Orange Juice) via the REST API — no photo upload needed.
 * This is synchronous and race-free, avoiding the flakiness of waiting for
 * an async upload pipeline to complete under parallel worker load.
 */
async function setupAreaWithItems(page: Page, appPort: number): Promise<string> {
  const name = `E2E SearchArea ${Date.now()}`;
  const areaID = await createArea(page, name);

  const ctx = await playwrightRequest.newContext({ baseURL: `http://localhost:${appPort}` });
  await ctx.post(`/areas/${areaID}/items`, { data: { name: 'Milk',         quantity: '2 liters', notes: '' } });
  await ctx.post(`/areas/${areaID}/items`, { data: { name: 'Butter',       quantity: '1 block',  notes: '' } });
  await ctx.post(`/areas/${areaID}/items`, { data: { name: 'Orange Juice', quantity: '1 carton', notes: '' } });
  await ctx.dispose();

  await page.reload();

  const card = page.locator(`[data-testid="area-card-${areaID}"]`);
  await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 5_000 });

  return areaID;
}

test.describe('Search', () => {
  test.beforeEach(async ({ resetDB }) => { await resetDB(); });

  test('search filters cards by item name', async ({ page, appPort }) => {
    await setupAreaWithItems(page, appPort);

    // Use the header search bar to filter.
    await page.fill('[data-testid="search-input"]', 'Milk');

    // Card should still be visible (contains "Milk" item).
    await expect(page.locator('.area-card')).toBeVisible({ timeout: 5_000 });
  });

  test('search highlights matching text', async ({ page, appPort }) => {
    await setupAreaWithItems(page, appPort);

    await page.fill('[data-testid="search-input"]', 'Milk');

    // Highlighted text should appear in a <mark> element.
    await expect(page.locator('.area-card mark')).toBeVisible({ timeout: 5_000 });
  });

  test('search unknown term → no matches state', async ({ page, appPort }) => {
    await setupAreaWithItems(page, appPort);

    await page.fill('[data-testid="search-input"]', 'ZZZThisDoesNotExist999');

    // Card should be hidden.
    await expect(page.locator('.area-card')).toBeHidden({ timeout: 5_000 });

    // "No matches" indicator should appear.
    await expect(page.locator('#no-search-matches')).toBeVisible({ timeout: 5_000 });
  });

  test('search is case-insensitive', async ({ page, appPort }) => {
    await setupAreaWithItems(page, appPort);

    await page.fill('[data-testid="search-input"]', 'milk');

    // Card should still be visible (case-insensitive match on "Milk").
    await expect(page.locator('.area-card')).toBeVisible({ timeout: 5_000 });
  });

  test('clear search restores all cards', async ({ page, appPort }) => {
    await setupAreaWithItems(page, appPort);

    // Search for something that hides the card.
    await page.fill('[data-testid="search-input"]', 'ZZZNotFound');
    await expect(page.locator('.area-card')).toBeHidden({ timeout: 5_000 });

    // Click the clear button.
    await page.click('[data-testid="search-clear"]');

    // Card should reappear.
    await expect(page.locator('.area-card')).toBeVisible({ timeout: 5_000 });
  });

  // Regression tests for kitchinv-s91: search must filter individual item rows,
  // not just show/hide the whole card.

  test('search hides non-matching item rows within card', async ({ page, appPort }) => {
    const areaID = await setupAreaWithItems(page, appPort);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);

    // Filter to only "Milk" — Butter and Orange Juice rows must be hidden.
    await page.fill('[data-testid="search-input"]', 'Milk');

    await expect(card.locator('[data-testid="item-row"]').filter({ hasText: 'Milk' })).toBeVisible({ timeout: 5_000 });
    await expect(card.locator('[data-testid="item-row"]').filter({ hasText: 'Butter' })).toBeHidden({ timeout: 5_000 });
    await expect(card.locator('[data-testid="item-row"]').filter({ hasText: 'Orange Juice' })).toBeHidden({ timeout: 5_000 });
  });

  test('area with no matching items is hidden entirely', async ({ page, appPort }) => {
    // Create two areas with items; only one will match.
    const areaID1 = await setupAreaWithItems(page, appPort);
    const areaID2 = await setupAreaWithItems(page, appPort);

    // Both areas have the same mock items. Search for something that exists so
    // both are visible first, then search for something that won't match.
    await page.fill('[data-testid="search-input"]', 'ZZZNoMatch');

    await expect(page.locator(`[data-testid="area-card-${areaID1}"]`)).toBeHidden({ timeout: 5_000 });
    await expect(page.locator(`[data-testid="area-card-${areaID2}"]`)).toBeHidden({ timeout: 5_000 });
  });

  test('clear search restores all item rows', async ({ page, appPort }) => {
    const areaID = await setupAreaWithItems(page, appPort);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);

    await page.fill('[data-testid="search-input"]', 'Milk');
    await expect(card.locator('[data-testid="item-row"]').filter({ hasText: 'Butter' })).toBeHidden({ timeout: 5_000 });

    await page.click('[data-testid="search-clear"]');

    // All rows should be visible again.
    await expect(card.locator('[data-testid="item-row"]').filter({ hasText: 'Butter' })).toBeVisible({ timeout: 5_000 });
    await expect(card.locator('[data-testid="item-row"]').filter({ hasText: 'Orange Juice' })).toBeVisible({ timeout: 5_000 });
  });
});
