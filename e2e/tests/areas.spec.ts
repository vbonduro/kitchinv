import { test, expect } from '../fixtures';
import { Page, request as playwrightRequest } from '@playwright/test';

async function addAreaViaDialog(page: Page, name: string) {
  await page.click('[data-testid="new-area-btn"]');
  await page.locator('#new-area-dialog').waitFor({ state: 'visible' });
  await page.fill('#new-area-dialog input[name="name"]', name);
  await page.click('#new-area-dialog button[type="submit"]');
}

/**
 * Create an area with a unique name on the areas list page.
 * Waits for the card to appear before returning.
 */
async function createArea(page: Page, name: string) {
  await page.goto('/areas');
  await addAreaViaDialog(page, name);
  await page.locator('.area-card-name', { hasText: name }).waitFor({ timeout: 10_000 });
}

async function createAreaWithItems(page: Page, appPort: number): Promise<string> {
  const name = `E2E ItemEdit ${Date.now()}`;
  await page.goto('/areas');
  await addAreaViaDialog(page, name);
  const card = page.locator('.area-card', { hasText: name });
  await card.waitFor({ timeout: 10_000 });
  const testid = await card.getAttribute('data-testid');
  const areaID = testid!.replace('area-card-', '');

  const ctx = await playwrightRequest.newContext({ baseURL: `http://localhost:${appPort}` });
  await ctx.post(`/areas/${areaID}/items`, { data: { name: 'Milk', quantity: '2', notes: 'top shelf' } });
  await ctx.dispose();

  await page.reload();
  await expect(card.locator('[data-testid="item-row"]')).toHaveCount(1, { timeout: 10_000 });
  return areaID;
}

test.describe('Areas', () => {
  test('empty state is visible on a fresh load', async ({ page }) => {
    await page.goto('/areas');
    await expect(page.locator('[data-testid="empty-state"]')).toBeVisible();
  });

  test('add area → appears in the list', async ({ page }) => {
    const name = `E2E AddArea ${Date.now()}`;
    await createArea(page, name);
    await expect(page.locator('.area-card-name', { hasText: name })).toBeVisible();
  });

  test('delete area → card removed from list', async ({ page }) => {
    const name = `E2E DeleteArea ${Date.now()}`;
    await createArea(page, name);

    // Accept the confirm dialog.
    page.on('dialog', (d) => d.accept());
    await page.locator('[data-testid="delete-area-btn"]').click();

    // Card should be gone.
    await expect(page.locator('.area-card-name', { hasText: name })).toHaveCount(0);
  });

  test('two areas → two area cards present', async ({ page }) => {
    const name1 = `E2E TwoAreas A ${Date.now()}`;
    const name2 = `E2E TwoAreas B ${Date.now()}`;
    await createArea(page, name1);
    await createArea(page, name2);

    await page.goto('/areas');
    await expect(page.locator('.area-card-name', { hasText: name1 })).toBeVisible();
    await expect(page.locator('.area-card-name', { hasText: name2 })).toBeVisible();
  });

  // Regression test for kitchinv-oct: add area button must remain visible after
  // the first area is created, without requiring a page refresh.
  test('add area button visible after first area created (no refresh)', async ({ page }) => {
    await page.goto('/areas');

    const name = `E2E FirstAreaBtn ${Date.now()}`;
    await addAreaViaDialog(page, name);
    await page.locator('.area-card-name', { hasText: name }).waitFor({ timeout: 5_000 });

    // The add area button must still be visible without a page refresh.
    await expect(page.locator('[data-testid="new-area-btn"]')).toBeVisible({ timeout: 3_000 });
  });

  // Regression test for kitchinv-1wd: errors in create area dialog must be shown
  // inline (not as toasts) because toasts are hidden behind the <dialog> top-layer.
  test('create area dialog shows inline error on duplicate name', async ({ page }) => {
    const name = `E2E DupDialog ${Date.now()}`;
    await createArea(page, name);
    await page.click('[data-testid="new-area-btn"]');
    await page.locator('#new-area-dialog').waitFor({ state: 'visible' });
    await page.fill('#new-area-dialog input[name="name"]', name);
    await page.click('#new-area-dialog button[type="submit"]');
    const dialogError = page.locator('[data-testid="dialog-error"]');
    await expect(dialogError).toBeVisible({ timeout: 3_000 });
    await expect(dialogError).toContainText('already exists');
    await expect(page.locator('#new-area-dialog')).toBeVisible();
  });

  test('empty state removed when first area added', async ({ page }) => {
    await page.goto('/areas');

    const emptyState = page.locator('[data-testid="empty-state"]');
    await expect(emptyState).toBeVisible();

    const name = `E2E FirstArea ${Date.now()}`;
    await addAreaViaDialog(page, name);

    // New card should appear dynamically.
    await expect(page.locator('.area-card-name', { hasText: name })).toBeVisible({ timeout: 5_000 });

    // Empty state must be gone.
    await expect(emptyState).toHaveCount(0);
  });

  // Regression test for kitchinv-zec: deleting all areas must show exactly one
  // "Add Area" button (the empty state), not two.
  // kitchinv-sw4: photo section defaults to unanchored; pin toggle makes it sticky.
  test('area photo section is not sticky by default', async ({ page }) => {
    const name = `E2E Unanchored ${Date.now()}`;
    await createArea(page, name);

    const stickyWrapper = page.locator('.area-card', { hasText: name })
      .locator('.area-sticky');
    await stickyWrapper.waitFor();

    const position = await stickyWrapper.evaluate((el) =>
      window.getComputedStyle(el).position
    );
    expect(position).not.toBe('sticky');
  });

  test('pin toggle makes photo section sticky', async ({ page }) => {
    const name = `E2E PinToggle ${Date.now()}`;
    await createArea(page, name);

    const card = page.locator('.area-card', { hasText: name });
    const stickyWrapper = card.locator('.area-sticky');
    await stickyWrapper.waitFor();

    // Click the pin button to enable sticky.
    await card.locator('[data-testid="pin-photo-btn"]').click();

    const position = await stickyWrapper.evaluate((el) =>
      window.getComputedStyle(el).position
    );
    expect(position).toBe('sticky');
  });

  test('pin toggle can be undone to unanchor the photo', async ({ page }) => {
    const name = `E2E UnpinToggle ${Date.now()}`;
    await createArea(page, name);

    const card = page.locator('.area-card', { hasText: name });
    const stickyWrapper = card.locator('.area-sticky');
    await stickyWrapper.waitFor();

    // Pin then unpin.
    await card.locator('[data-testid="pin-photo-btn"]').click();
    await card.locator('[data-testid="pin-photo-btn"]').click();

    const position = await stickyWrapper.evaluate((el) =>
      window.getComputedStyle(el).position
    );
    expect(position).not.toBe('sticky');
  });

  test('deleting all areas shows exactly one add area button', async ({ page }) => {
    const name1 = `E2E DelAll A ${Date.now()}`;
    const name2 = `E2E DelAll B ${Date.now()}`;
    await createArea(page, name1);
    await createArea(page, name2);

    await page.goto('/areas');
    page.on('dialog', (d) => d.accept());

    // Delete first area and wait for its card to disappear.
    const cards = page.locator('.area-card');
    await expect(cards).toHaveCount(2);
    await page.locator('[data-testid="delete-area-btn"]').first().click();
    await expect(cards).toHaveCount(1, { timeout: 5_000 });

    // Delete second area.
    await page.locator('[data-testid="delete-area-btn"]').first().click();

    // Wait for empty state.
    await expect(page.locator('[data-testid="empty-state"]')).toBeVisible({ timeout: 5_000 });

    // Exactly one "Add Area" button must be visible.
    await expect(page.locator('[data-testid="new-area-btn"]')).toHaveCount(1);
  });

  // Regression tests for kitchinv-13c: clicking any cell in an item row must
  // start inline editing, not just the name cell.
  test('clicking qty cell starts inline edit', async ({ page, appPort }) => {
    const areaID = await createAreaWithItems(page, appPort);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    const row = card.locator('[data-testid="item-row"]').first();

    // Click the qty cell (second td).
    await row.locator('td').nth(1).click();

    // Inline edit inputs must appear.
    await expect(row.locator('input[data-field="name"]')).toBeVisible({ timeout: 3_000 });
    await expect(row.locator('input[data-field="qty"]')).toBeVisible({ timeout: 3_000 });
  });

  test('clicking notes cell starts inline edit', async ({ page, appPort }) => {
    const areaID = await createAreaWithItems(page, appPort);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    const row = card.locator('[data-testid="item-row"]').first();

    // Click the notes cell (third td).
    await row.locator('td').nth(2).click();

    // Inline edit inputs must appear.
    await expect(row.locator('input[data-field="name"]')).toBeVisible({ timeout: 3_000 });
    await expect(row.locator('input[data-field="qty"]')).toBeVisible({ timeout: 3_000 });
  });

  test('qty inline edit input is type=number', async ({ page, appPort }) => {
    const areaID = await createAreaWithItems(page, appPort);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    const row = card.locator('[data-testid="item-row"]').first();

    await row.locator('td').first().click();

    const qtyInput = row.locator('input[data-field="qty"]');
    await expect(qtyInput).toBeVisible({ timeout: 3_000 });
    await expect(qtyInput).toHaveAttribute('type', 'number');
  });
});
