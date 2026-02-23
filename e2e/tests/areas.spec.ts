import { test, expect } from '../fixtures';
import { Page } from '@playwright/test';

async function addAreaViaMenu(page: Page, name: string) {
  await page.click('#menu-btn');
  await page.fill('input[name="name"]', name);
  await page.click('button[type="submit"]');
}

/**
 * Create an area with a unique name and return to the areas list.
 * Must navigate to /areas first because the HTMX form targets .area-list,
 * which only exists on that page.
 */
async function createArea(page: Page, name: string) {
  await page.goto('/areas');
  await addAreaViaMenu(page, name);
  // Wait for the new card to appear.
  await page.locator('.area-row-name a', { hasText: name }).waitFor({ timeout: 10_000 });
}

test.describe('Areas', () => {
  test.beforeEach(async ({ resetDB }) => { await resetDB(); });

  test('empty state is visible on a fresh load', async ({ page }) => {
    await page.goto('/areas');
    await expect(page.locator('#area-list .empty-state')).toBeVisible();
  });

  test('add area → appears in the list', async ({ page }) => {
    const name = `E2E AddArea ${Date.now()}`;
    await createArea(page, name);
    await expect(page.locator('.area-row-name a', { hasText: name })).toBeVisible();
  });

  test('click area link → navigates to detail page with correct title', async ({ page }) => {
    const name = `E2E DetailNav ${Date.now()}`;
    await createArea(page, name);
    await page.locator('.area-row-name a', { hasText: name }).click();
    await expect(page.locator('.detail-title')).toHaveText(name);
  });

  test('delete area → redirects to /areas, card gone', async ({ page }) => {
    const name = `E2E DeleteArea ${Date.now()}`;
    await createArea(page, name);

    // Navigate to the area detail page.
    await page.locator('.area-row-name a', { hasText: name }).click();
    await page.waitForURL(/\/areas\/\d+/);

    // Accept the confirm dialog that HTMX shows for hx-confirm.
    page.on('dialog', (d) => d.accept());
    await page.locator('button.btn-danger').click();

    // Should redirect to /areas.
    await page.waitForURL(/\/areas$/);
    await expect(page.locator('.area-row-name a', { hasText: name })).toHaveCount(0);
  });

  test('two areas → two .area-row elements present', async ({ page }) => {
    const name1 = `E2E TwoAreas A ${Date.now()}`;
    const name2 = `E2E TwoAreas B ${Date.now()}`;
    await createArea(page, name1);
    await createArea(page, name2);

    await page.goto('/areas');
    // Both areas exist somewhere in the list.
    await expect(page.locator('.area-row-name a', { hasText: name1 })).toBeVisible();
    await expect(page.locator('.area-row-name a', { hasText: name2 })).toBeVisible();
  });

  test('Bug kitchinv-49a: empty state removed when first area added', async ({ page }) => {
    await page.goto('/areas');

    // Clean DB — empty state must be visible.
    const emptyState = page.locator('#area-list .empty-state');
    await expect(emptyState).toBeVisible();

    // Add the first area without navigating away.
    const name = `E2E FirstArea ${Date.now()}`;
    await addAreaViaMenu(page, name);

    // New card should appear dynamically.
    await expect(page.locator('.area-row-name a', { hasText: name })).toBeVisible({ timeout: 5_000 });

    // Empty state must be gone.
    await expect(emptyState).toHaveCount(0);
  });

  test('Bug kitchinv-49a: add area from detail page navigates to /areas with new card', async ({ page }) => {
    // Create an initial area so we can navigate to its detail page.
    const initial = `E2E InitialArea ${Date.now()}`;
    await createArea(page, initial);

    // Navigate to the detail page (no #area-list exists here).
    await page.locator('.area-row-name a', { hasText: initial }).click();
    await page.waitForURL(/\/areas\/\d+/);

    // Add a new area from the detail page via the banner menu.
    const name = `E2E FromDetail ${Date.now()}`;
    await addAreaViaMenu(page, name);

    // Should end up on /areas with the new card visible.
    await page.waitForURL(/\/areas$/, { timeout: 5_000 });
    await expect(page.locator('.area-row-name a', { hasText: name })).toBeVisible({ timeout: 5_000 });
  });
});
