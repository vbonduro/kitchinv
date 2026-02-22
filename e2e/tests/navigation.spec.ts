import { test, expect, Page, request } from '@playwright/test';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

const OLLAMA_PORT = process.env.OLLAMA_PORT || '19434';

function createJpegFixture(): string {
  const buf = Buffer.alloc(512, 0);
  buf[0] = 0xff; buf[1] = 0xd8; buf[2] = 0xff; buf[3] = 0xe0;
  const tmpFile = path.join(os.tmpdir(), `e2e-nav-${Date.now()}.jpg`);
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

/** Navigate to an area's detail page and return its URL. */
async function openArea(page: Page, name: string): Promise<string> {
  await page.goto('/areas');
  await page.locator('.area-row-name a', { hasText: name }).click();
  await page.waitForURL(/\/areas\/\d+/);
  return page.url();
}

/** Enable slow mode on the mock Ollama server. */
async function setSlowMode(apiContext: ReturnType<typeof request.newContext> extends Promise<infer T> ? T : never, slow: boolean) {
  await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/${slow ? 'slow' : 'fast'}`);
}

test.describe('Navigation', () => {
  let jpegFixture: string;
  let apiContext: Awaited<ReturnType<typeof request.newContext>>;

  test.beforeAll(async ({ playwright }) => {
    jpegFixture = createJpegFixture();
    apiContext = await playwright.request.newContext();
  });

  test.afterAll(async () => {
    // Ensure slow mode is reset.
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/fast`);
    await apiContext.dispose();
    try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
  });

  test.afterEach(async () => {
    // Reset slow mode after each test.
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/fast`);
  });

  test('navigate away mid-stream then back: spinner shows, then items appear via polling', async ({ page }) => {
    const name = `E2E NavMidStream ${Date.now()}`;
    await createArea(page, name);
    const areaUrl = await openArea(page, name);

    // Enable slow mode before uploading.
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/slow`);

    // Start upload.
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

    // Navigate away — disconnects browser from SSE stream.
    await page.goto('/areas');

    // Navigate back.
    await page.goto(areaUrl);

    // Server-rendered page: photo exists, no items yet → JS sets spinner and polls.
    // The polling picks up items after server-side analysis completes.
    // Slow mock: 3 items × 500ms = 1.5s + 2s initial poll delay = ~3.5s minimum.
    await expect(page.locator('.item-row')).toHaveCount(3, { timeout: 20_000 });
  });

  test('areas list shows "Analysing…" card during analysis, not "No items"', async ({ page }) => {
    const name = `E2E NavAnalysingCard ${Date.now()}`;
    await createArea(page, name);
    await openArea(page, name);

    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/slow`);

    // Start upload.
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

    // Navigate to /areas list while analysis is in progress.
    await page.goto('/areas');

    // Find the card for our area.
    const card = page.locator('.area-row', {
      has: page.locator('.area-row-name a', { hasText: name }),
    });

    // Should show "Analysing…" text.
    await expect(card.locator('.area-row-analysing')).toBeVisible({ timeout: 5_000 });

    // Should NOT show "No items" text.
    await expect(card.locator('.area-row-no-items')).not.toBeVisible();
  });

  test('items persist after navigation (context.WithoutCancel)', async ({ page }) => {
    const name = `E2E NavPersist ${Date.now()}`;
    await createArea(page, name);
    const areaUrl = await openArea(page, name);

    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/slow`);

    // Start upload and immediately navigate away.
    const [fc] = await Promise.all([
      page.waitForEvent('filechooser'),
      page.click('#photo-input'),
    ]);
    await fc.setFiles(jpegFixture);
    await page.click('#upload-btn');

    // Navigate away right after submitting.
    await page.goto('/areas');

    // Wait long enough for slow mock to finish: 3 × 500ms = 1.5s → give 3s margin.
    await page.waitForTimeout(3_000);

    // Navigate back — items should be in DB now.
    await page.goto(areaUrl);
    await expect(page.locator('.item-row')).toHaveCount(3, { timeout: 10_000 });
  });
});
