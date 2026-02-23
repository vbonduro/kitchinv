import { test, expect } from '../fixtures';
import { Page } from '@playwright/test';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

/** Minimal valid JPEG: 512-byte buffer starting with JPEG magic bytes. */
function createJpegFixture(): string {
  const buf = Buffer.alloc(512, 0);
  buf[0] = 0xff;
  buf[1] = 0xd8;
  buf[2] = 0xff;
  buf[3] = 0xe0;
  const tmpFile = path.join(os.tmpdir(), `e2e-fixture-${Date.now()}.jpg`);
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

async function navigateToArea(page: Page, name: string) {
  await page.goto('/areas');
  await page.locator('.area-row-name a', { hasText: name }).click();
  await page.waitForURL(/\/areas\/\d+/);
}

test.describe('Upload & Analysis', () => {
  let jpegFixture: string;

  test.beforeAll(() => {
    jpegFixture = createJpegFixture();
  });

  test.beforeEach(async ({ resetDB }) => { await resetDB(); });

  test.afterAll(() => {
    try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
  });

  test('upload triggers file chooser', async ({ page }) => {
    const name = `E2E UploadFC ${Date.now()}`;
    await createArea(page, name);
    await navigateToArea(page, name);

    const [fc] = await Promise.all([
      page.waitForEvent('filechooser'),
      page.click('#photo-input'),
    ]);
    expect(fc).toBeTruthy();
  });

  test('upload shows scanning indicator then 3 items stream in', async ({ page }) => {
    const name = `E2E UploadStream ${Date.now()}`;
    await createArea(page, name);
    await navigateToArea(page, name);

    // Attach file and submit.
    const [fc] = await Promise.all([
      page.waitForEvent('filechooser'),
      page.click('#photo-input'),
    ]);
    await fc.setFiles(jpegFixture);

    await page.click('#upload-btn');

    // Scanning indicator should appear.
    await expect(page.locator('.analyse-scanning')).toBeVisible({ timeout: 5_000 });

    // 3 item rows should appear after stream completes.
    await expect(page.locator('.item-row')).toHaveCount(3, { timeout: 15_000 });
  });

  test('upload button is disabled during upload and re-enabled after', async ({ page }) => {
    const name = `E2E UploadBtnState ${Date.now()}`;
    await createArea(page, name);
    await navigateToArea(page, name);

    const [fc] = await Promise.all([
      page.waitForEvent('filechooser'),
      page.click('#photo-input'),
    ]);
    await fc.setFiles(jpegFixture);

    await page.click('#upload-btn');

    // Button should be disabled while uploading.
    await expect(page.locator('#upload-btn')).toBeDisabled({ timeout: 5_000 });

    // After stream completes, button re-enabled.
    await expect(page.locator('#upload-btn')).toBeEnabled({ timeout: 15_000 });
  });

  test('second upload replaces items (still 3 item-rows)', async ({ page }) => {
    const name = `E2E UploadReplace ${Date.now()}`;
    await createArea(page, name);
    await navigateToArea(page, name);

    async function doUpload() {
      const [fc] = await Promise.all([
        page.waitForEvent('filechooser'),
        page.click('#photo-input'),
      ]);
      await fc.setFiles(jpegFixture);
      await page.click('#upload-btn');
      await expect(page.locator('.item-row')).toHaveCount(3, { timeout: 15_000 });
    }

    // First upload.
    await doUpload();
    // Second upload.
    await doUpload();

    // Still exactly 3 items.
    await expect(page.locator('.item-row')).toHaveCount(3);
  });
});
