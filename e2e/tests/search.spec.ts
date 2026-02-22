import { test, expect, Page } from '@playwright/test';
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

async function createArea(page: Page, name: string) {
  await page.goto('/areas');
  await page.click('#menu-btn');
  await page.fill('input[name="name"]', name);
  await page.click('button[type="submit"]');
  await page.locator('.area-row-name a', { hasText: name }).waitFor({ timeout: 10_000 });
}

/** Create an area, upload a photo, and wait for all 3 items to appear. */
async function createAreaWithItems(page: Page, name: string, jpegFixture: string) {
  await createArea(page, name);

  // Navigate to area detail.
  await page.goto('/areas');
  await page.locator('.area-row-name a', { hasText: name }).click();
  await page.waitForURL(/\/areas\/\d+/);

  // Upload.
  const [fc] = await Promise.all([
    page.waitForEvent('filechooser'),
    page.click('#photo-input'),
  ]);
  await fc.setFiles(jpegFixture);
  await page.click('#upload-btn');

  // Wait for items to stream in.
  await expect(page.locator('.item-row')).toHaveCount(3, { timeout: 15_000 });

  // Return the current URL (area detail URL) for reference.
  return page.url();
}

test.describe('Search', () => {
  let jpegFixture: string;
  let areaName: string;
  let areaUrl: string;

  test.beforeAll(async ({ browser }) => {
    jpegFixture = createJpegFixture();
    // Create one area with items shared across all search tests.
    areaName = `E2E SearchArea ${Date.now()}`;
    const page = await browser.newPage();
    areaUrl = await createAreaWithItems(page, areaName, jpegFixture);
    await page.close();
  });

  test.afterAll(() => {
    try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
  });

  test('search "Milk" → result card with Milk visible', async ({ page }) => {
    await page.goto('/search');
    await page.fill('input[type="search"]', 'Milk');
    // HTMX triggers on input; wait for results.
    await expect(page.locator('.result-card', { hasText: 'Milk' }).first()).toBeVisible({ timeout: 5_000 });
  });

  test('search result has "View area" link', async ({ page }) => {
    await page.goto('/search');
    await page.fill('input[type="search"]', 'Milk');
    await expect(page.locator('.result-card .result-area-link').first()).toBeVisible({ timeout: 5_000 });
  });

  test('click "View area" → navigates to area detail page', async ({ page }) => {
    await page.goto('/search');
    await page.fill('input[type="search"]', 'Milk');

    // Wait for results.
    await page.locator('.result-card').first().waitFor({ timeout: 5_000 });

    // Click first "View area" link.
    await page.locator('.result-area-link').first().click();

    // Should land on an area detail page.
    await page.waitForURL(/\/areas\/\d+/);
    await expect(page.locator('.detail-title')).toBeVisible();
  });

  test('search unknown term → empty state visible', async ({ page }) => {
    await page.goto('/search');
    await page.fill('input[type="search"]', 'ZZZThisItemDoesNotExist999');
    await expect(page.locator('.empty-state')).toBeVisible({ timeout: 5_000 });
  });

  test('search lowercase "milk" finds "Milk" (case-insensitive)', async ({ page }) => {
    await page.goto('/search');
    await page.fill('input[type="search"]', 'milk');
    await expect(page.locator('.result-card', { hasText: 'Milk' }).first()).toBeVisible({ timeout: 5_000 });
  });

  test('search by area name finds items from that area', async ({ page }) => {
    // Navigate directly to the search page and search for our unique area's items.
    await page.goto('/search');
    await page.fill('input[type="search"]', 'Orange Juice');
    await expect(page.locator('.result-card', { hasText: 'Orange Juice' }).first()).toBeVisible({ timeout: 5_000 });

    // Click "View area" and verify we land on an area that has the area name.
    // Find the card that links to our specific area.
    const cards = page.locator('.result-card', { hasText: 'Orange Juice' });
    const count = await cards.count();
    let found = false;
    for (let i = 0; i < count; i++) {
      const link = cards.nth(i).locator('.result-area-link');
      const href = await link.getAttribute('href');
      if (href && href === areaUrl.replace(/^https?:\/\/[^/]+/, '')) {
        found = true;
        break;
      }
    }
    expect(found).toBe(true);
  });
});
