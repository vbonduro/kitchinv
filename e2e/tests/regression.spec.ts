import { test, expect, devices, Page, request } from '@playwright/test';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

const OLLAMA_PORT = process.env.OLLAMA_PORT || '19434';

function createJpegFixture(): string {
  const buf = Buffer.alloc(512, 0);
  buf[0] = 0xff; buf[1] = 0xd8; buf[2] = 0xff; buf[3] = 0xe0;
  const tmpFile = path.join(os.tmpdir(), `e2e-reg-${Date.now()}.jpg`);
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

async function openArea(page: Page, name: string): Promise<string> {
  await page.goto('/areas');
  await page.locator('.area-row-name a', { hasText: name }).click();
  await page.waitForURL(/\/areas\/\d+/);
  return page.url();
}

test.describe('Regression', () => {
  let jpegFixture: string;
  let apiContext: Awaited<ReturnType<typeof request.newContext>>;

  test.beforeAll(async ({ playwright }) => {
    jpegFixture = createJpegFixture();
    apiContext = await playwright.request.newContext();
  });

  test.afterAll(async () => {
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/fast`);
    await apiContext.dispose();
    try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
  });

  test.afterEach(async () => {
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/fast`);
  });

  test('Bug 1: #photo-input has no capture attribute', async ({ page }) => {
    const name = `E2E RegCapture ${Date.now()}`;
    await createArea(page, name);
    await openArea(page, name);

    const attr = await page.locator('#photo-input').getAttribute('capture');
    expect(attr).toBeNull();
  });

  test('Bug 2: mobile file picker opens on tap (iPhone 14)', async ({ browser }) => {
    // Use iPhone 14 device for this specific test.
    const context = await browser.newContext({ ...devices['iPhone 14'] });
    const page = await context.newPage();

    try {
      const name = `E2E RegMobilePicker ${Date.now()}`;
      await createArea(page, name);
      await openArea(page, name);

      const [fc] = await Promise.all([
        page.waitForEvent('filechooser', { timeout: 5_000 }),
        page.locator('#photo-input').tap(),
      ]);
      expect(fc).toBeTruthy();
    } finally {
      await context.close();
    }
  });

  test('Bug 3: spinner is server-rendered (no sessionStorage), not JS-only', async ({ page }) => {
    const name = `E2E RegServerSpinner ${Date.now()}`;
    await createArea(page, name);
    const areaUrl = await openArea(page, name);

    // Enable slow mode so analysis is still in-progress when we open a fresh page.
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/slow`);

    // Start upload.
    const [fc] = await Promise.all([
      page.waitForEvent('filechooser'),
      page.click('#photo-input'),
    ]);
    await fc.setFiles(jpegFixture);

    // Wait for the stream response to begin before navigating. The server only
    // sends a response after the photo is committed to the DB, so this guarantees
    // the photo record exists when the fresh page loads.
    const streamResponsePromise = page.waitForResponse(/photos\/stream/, { timeout: 10_000 });
    await page.click('#upload-btn');
    await streamResponsePromise;

    // Navigate away (disconnects this tab from the SSE stream).
    await page.goto('/areas');

    // Open a fresh page in the same browser context.
    // The area has a photo but no items yet (server-side analysis still in progress).
    const freshPage = await page.context().newPage();
    await freshPage.goto(areaUrl);

    // The fresh page must show the scanning spinner (server-rendered: hasPhoto && !hasItems).
    await expect(freshPage.locator('.analyse-scanning')).toBeVisible({ timeout: 10_000 });

    // Verify sessionStorage is empty (spinner comes from server-rendered JS, not sessionStorage).
    const ssLength = await freshPage.evaluate(() => Object.keys(sessionStorage).length);
    expect(ssLength).toBe(0);

    await freshPage.close();
  });

  test('Bug 4: "Analysing…" shown on areas list (not "No items") during analysis', async ({ page }) => {
    const name = `E2E RegAnalysingCard ${Date.now()}`;
    await createArea(page, name);
    await openArea(page, name);

    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/slow`);

    const [fc] = await Promise.all([
      page.waitForEvent('filechooser'),
      page.click('#photo-input'),
    ]);
    await fc.setFiles(jpegFixture);

    // Wait for the stream response to begin before navigating. The server only
    // sends a response after the photo is committed to the DB, so this guarantees
    // the photo record exists when we arrive at /areas.
    const streamResponsePromise = page.waitForResponse(/photos\/stream/, { timeout: 10_000 });
    await page.click('#upload-btn');
    await streamResponsePromise;

    // Navigate to areas list while analysis is in-progress.
    await page.goto('/areas');

    const card = page.locator('.area-row', {
      has: page.locator('.area-row-name a', { hasText: name }),
    });

    // Positive: "Analysing…" text is present.
    await expect(card.locator('.area-row-analysing')).toBeVisible({ timeout: 5_000 });

    // Negative: "No items" text is absent.
    await expect(card.locator('.area-row-no-items')).not.toBeVisible();
  });
});
