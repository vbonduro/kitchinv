import { test, expect } from '../fixtures';
import { devices, Page, request } from '@playwright/test';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

function createJpegFixture(): string {
  const buf = Buffer.alloc(512, 0);
  buf[0] = 0xff; buf[1] = 0xd8; buf[2] = 0xff; buf[3] = 0xe0;
  const tmpFile = path.join(os.tmpdir(), `e2e-reg-${Date.now()}.jpg`);
  fs.writeFileSync(tmpFile, buf);
  return tmpFile;
}

async function createArea(page: Page, name: string): Promise<string> {
  await page.goto('/areas');
  const body = page.locator('body');
  if (!await body.evaluate((el) => el.hasAttribute('data-edit-mode'))) {
    await page.locator('[data-testid="edit-mode-btn"]').click();
  }
  await page.click('[data-testid="new-area-btn"]');
  await page.locator('#new-area-dialog').waitFor({ state: 'visible' });
  await page.fill('#new-area-dialog input[name="name"]', name);
  await page.click('#new-area-dialog button[type="submit"]');
  const card = page.locator('.area-card', { hasText: name });
  await card.waitFor({ timeout: 10_000 });
  const testid = await card.getAttribute('data-testid');
  return testid!.replace('area-card-', '');
}

test.describe('Regression', () => {
  test('Bug 1: photo-input has no capture attribute', async ({ page }) => {
    const name = `E2E RegCapture ${Date.now()}`;
    const areaID = await createArea(page, name);

    const attr = await page.locator(`[data-testid="photo-input-${areaID}"]`).getAttribute('capture');
    expect(attr).toBeNull();
  });

  test('Bug 2: mobile file picker opens on tap (iPhone 14)', async ({ browser, baseURL }) => {
    const context = await browser.newContext({ ...devices['iPhone 14'], baseURL: baseURL! });
    const page = await context.newPage();

    try {
      const name = `E2E RegMobilePicker ${Date.now()}`;
      const areaID = await createArea(page, name);

      // Tap the upload zone (the visible element) — it triggers the hidden file input.
      const [fc] = await Promise.all([
        page.waitForEvent('filechooser', { timeout: 5_000 }),
        page.locator(`[data-testid="upload-zone-${areaID}"]`).tap(),
      ]);
      expect(fc).toBeTruthy();
    } finally {
      await context.close();
    }
  });

  test('Bug 3: analyzing overlay appears immediately on file select (JS-driven)', async ({ page, ollamaPort }) => {
    const jpegFixture = createJpegFixture();
    const apiContext = await request.newContext({ baseURL: `http://localhost:${ollamaPort}` });

    try {
      const name = `E2E RegClientSpinner ${Date.now()}`;
      const areaID = await createArea(page, name);

      // Slow mode keeps the upload in progress long enough to assert overlay visibility.
      await apiContext.post('/control/slow');

      const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
      await fileInput.setInputFiles(jpegFixture);

      // Overlay must appear immediately on the client side while upload is in progress.
      await expect(page.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).toBeVisible({ timeout: 5_000 });
    } finally {
      await apiContext.post('/control/fast');
      await apiContext.dispose();
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });

  test('Bug 4: analyzing overlay shown (not "no items") during analysis', async ({ page, ollamaPort }) => {
    const jpegFixture = createJpegFixture();
    const apiContext = await request.newContext({ baseURL: `http://localhost:${ollamaPort}` });

    try {
      const name = `E2E RegAnalysingCard ${Date.now()}`;
      const areaID = await createArea(page, name);

      // Slow mode keeps the upload in progress long enough to assert overlay visibility.
      await apiContext.post('/control/slow');

      const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
      await fileInput.setInputFiles(jpegFixture);

      const card = page.locator(`[data-testid="area-card-${areaID}"]`);

      // Positive: analyzing indicator is present.
      await expect(card.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).toBeVisible({ timeout: 5_000 });

      // Negative: "no items" text is absent.
      await expect(card.locator('.no-items-text')).not.toBeVisible();
    } finally {
      await apiContext.post('/control/fast');
      await apiContext.dispose();
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });
});
