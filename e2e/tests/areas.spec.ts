import { test, expect } from '../fixtures';
import { Page } from '@playwright/test';

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

test.describe('Areas', () => {
  test.beforeEach(async ({ resetDB }) => { await resetDB(); });

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
});
