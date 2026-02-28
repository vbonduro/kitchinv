import { test, expect } from '../fixtures';
import { Page } from '@playwright/test';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

function createJpegFixture(): string {
  const buf = Buffer.alloc(512, 0);
  buf[0] = 0xff; buf[1] = 0xd8; buf[2] = 0xff; buf[3] = 0xe0;
  const tmpFile = path.join(os.tmpdir(), `e2e-search-${Date.now()}.jpg`);
  fs.writeFileSync(tmpFile, buf);
  return tmpFile;
}

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

/** Create an area, upload a photo, wait for items. Returns area ID. */
async function setupAreaWithItems(page: Page, jpegFixture: string): Promise<string> {
  const name = `E2E SearchArea ${Date.now()}`;
  const areaID = await createArea(page, name);

  // Upload photo via file input.
  const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
  await fileInput.setInputFiles(jpegFixture);

  // Wait for items to stream in.
  const card = page.locator(`[data-testid="area-card-${areaID}"]`);
  await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 15_000 });

  return areaID;
}

test.describe('Search', () => {
  let jpegFixture: string;

  test.beforeAll(() => {
    jpegFixture = createJpegFixture();
  });

  test.beforeEach(async ({ resetDB }) => { await resetDB(); });

  test.afterAll(() => {
    try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
  });

  test('search filters cards by item name', async ({ page }) => {
    await setupAreaWithItems(page, jpegFixture);

    // Use the header search bar to filter.
    await page.fill('[data-testid="search-input"]', 'Milk');

    // Card should still be visible (contains "Milk" item).
    await expect(page.locator('.area-card')).toBeVisible({ timeout: 5_000 });
  });

  test('search highlights matching text', async ({ page }) => {
    await setupAreaWithItems(page, jpegFixture);

    await page.fill('[data-testid="search-input"]', 'Milk');

    // Highlighted text should appear in a <mark> element.
    await expect(page.locator('.area-card mark')).toBeVisible({ timeout: 5_000 });
  });

  test('search unknown term → no matches state', async ({ page }) => {
    await setupAreaWithItems(page, jpegFixture);

    await page.fill('[data-testid="search-input"]', 'ZZZThisDoesNotExist999');

    // Card should be hidden.
    await expect(page.locator('.area-card')).toBeHidden({ timeout: 5_000 });

    // "No matches" indicator should appear.
    await expect(page.locator('#no-search-matches')).toBeVisible({ timeout: 5_000 });
  });

  test('search is case-insensitive', async ({ page }) => {
    await setupAreaWithItems(page, jpegFixture);

    await page.fill('[data-testid="search-input"]', 'milk');

    // Card should still be visible (case-insensitive match on "Milk").
    await expect(page.locator('.area-card')).toBeVisible({ timeout: 5_000 });
  });

  test('clear search restores all cards', async ({ page }) => {
    await setupAreaWithItems(page, jpegFixture);

    // Search for something that hides the card.
    await page.fill('[data-testid="search-input"]', 'ZZZNotFound');
    await expect(page.locator('.area-card')).toBeHidden({ timeout: 5_000 });

    // Click the clear button.
    await page.click('[data-testid="search-clear"]');

    // Card should reappear.
    await expect(page.locator('.area-card')).toBeVisible({ timeout: 5_000 });
  });
});
