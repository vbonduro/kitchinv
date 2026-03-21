import { test, expect } from '../fixtures';
import { Page, request as playwrightRequest } from '@playwright/test';

async function enableEditMode(page: Page) {
  const body = page.locator('body');
  const isEdit = await body.evaluate((el) => el.hasAttribute('data-edit-mode'));
  if (!isEdit) await page.locator('[data-testid="edit-mode-btn"]').click();
}

/**
 * Creates an area with one item via the API. Returns the areaID.
 */
async function createAreaWithItem(page: Page, appPort: number, areaName: string, itemName: string): Promise<string> {
  await page.goto('/areas');
  await enableEditMode(page);
  await page.click('[data-testid="new-area-btn"]');
  await page.locator('#new-area-dialog').waitFor({ state: 'visible' });
  await page.fill('#new-area-dialog input[name="name"]', areaName);
  await page.click('#new-area-dialog button[type="submit"]');
  const card = page.locator('.area-card', { hasText: areaName });
  await card.waitFor({ timeout: 10_000 });
  const testid = await card.getAttribute('data-testid');
  const areaID = testid!.replace('area-card-', '');

  const ctx = await playwrightRequest.newContext({ baseURL: `http://localhost:${appPort}` });
  await ctx.post(`/areas/${areaID}/items`, { data: { name: itemName, quantity: '1' } });
  await ctx.dispose();

  await page.reload();
  await card.locator('[data-testid="item-row"]').waitFor({ timeout: 10_000 });
  return areaID;
}

/**
 * Renames an item inline via click → edit → blur.
 */
async function renameItem(page: Page, areaID: string, newName: string) {
  const card = page.locator(`[data-testid="area-card-${areaID}"]`);
  const row = card.locator('[data-testid="item-row"]').first();
  await row.click();
  const nameInput = row.locator('input[data-field="name"]');
  await nameInput.waitFor({ state: 'visible', timeout: 5_000 });
  await nameInput.click({ clickCount: 3 });
  await nameInput.fill(newName);
  // Submit by pressing Enter.
  await nameInput.press('Enter');
  // Wait for the row to leave edit mode.
  await expect(nameInput).toBeHidden({ timeout: 5_000 });
}

test.describe('Override rules — auto-creation', () => {
  test('renaming an item creates an override rule on the overrides page', async ({ page, appPort }) => {
    const areaName = `E2E OverrideCreate ${Date.now()}`;
    const areaID = await createAreaWithItem(page, appPort, areaName, 'Tropicana OJ');

    await renameItem(page, areaID, 'Orange Juice');

    await page.goto('/overrides');
    await expect(page.locator('table')).toBeVisible({ timeout: 5_000 });
    await expect(page.locator('td').filter({ hasText: 'Tropicana OJ' }).first()).toBeVisible();
    await expect(page.locator('td').filter({ hasText: 'Orange Juice' }).first()).toBeVisible();
  });

  test('auto-created rule is area-scoped', async ({ page, appPort }) => {
    const areaName = `E2E OverrideScoped ${Date.now()}`;
    const areaID = await createAreaWithItem(page, appPort, areaName, 'Milk');

    await renameItem(page, areaID, 'Whole Milk');

    await page.goto('/overrides');
    // Scope badge should show the area name, not "Global".
    const row = page.locator('tr', { hasText: 'Milk' });
    await expect(row).toBeVisible({ timeout: 5_000 });
    await expect(row.locator('.scope-area')).toBeVisible();
    await expect(row.locator('.scope-area')).toContainText(areaName);
  });

  test('renaming same item again does not duplicate the rule', async ({ page, appPort }) => {
    const areaName = `E2E OverrideNoDupe ${Date.now()}`;
    const areaID = await createAreaWithItem(page, appPort, areaName, 'OJ');

    await renameItem(page, areaID, 'Orange Juice');
    await renameItem(page, areaID, 'OJ Premium');

    await page.goto('/overrides');
    // Two renames → two rules (OJ→OrangeJuice, OrangeJuice→OJPremium), not three.
    const rows = page.locator('#override-rules-body tr');
    const count = await rows.count();
    // Verify no duplicate "OJ" pattern exists.
    const ojRows = await page.locator('td code', { hasText: /^OJ$/ }).count();
    expect(ojRows).toBeLessThanOrEqual(1);
    expect(count).toBeGreaterThan(0);
  });

  test('deleting an area removes its override rules', async ({ page, appPort }) => {
    const areaName = `E2E OverrideDeleteArea ${Date.now()}`;
    const areaID = await createAreaWithItem(page, appPort, areaName, 'Butter');

    await renameItem(page, areaID, 'Salted Butter');

    // Verify rule exists.
    await page.goto('/overrides');
    await expect(page.locator('td').filter({ hasText: 'Butter' }).first()).toBeVisible({ timeout: 5_000 });

    // Delete the area.
    await page.goto('/areas');
    await enableEditMode(page);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    page.on('dialog', (d) => d.accept());
    await card.locator('[data-testid="delete-area-btn"]').click();
    await expect(card).toHaveCount(0, { timeout: 5_000 });

    // Rule should be gone.
    await page.goto('/overrides');
    const butterCells = page.locator('td code', { hasText: 'Butter' });
    await expect(butterCells).toHaveCount(0, { timeout: 5_000 });
  });

  test('overrides page loads without errors', async ({ page }) => {
    await page.goto('/overrides');
    await expect(page.locator('h1, .section-label')).toContainText('Override Rules');
    await expect(page.locator('body')).not.toContainText('template error');
  });
});
