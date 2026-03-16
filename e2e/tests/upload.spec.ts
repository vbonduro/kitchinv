import { test, expect } from '../fixtures';
import { Page, request } from '@playwright/test';
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

async function uploadPhoto(page: Page, areaID: string, jpegFixture: string) {
  const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
  await fileInput.setInputFiles(jpegFixture);
}

test.describe('Upload & Analysis', () => {
  test('analyzing overlay appears immediately on file select', async ({ page, ollamaPort }) => {
    const jpegFixture = createJpegFixture();
    const apiContext = await request.newContext({ baseURL: `http://localhost:${ollamaPort}` });

    try {
      const name = `E2E UploadBanner ${Date.now()}`;
      const areaID = await createArea(page, name);

      // Slow mode delays the mock response so the overlay stays visible long enough to assert.
      await apiContext.post('/control/slow');

      // Select the file — JS immediately shows the overlay before the fetch completes.
      await uploadPhoto(page, areaID, jpegFixture);

      // Overlay must be visible showing "Analyzing your space...".
      const overlay = page.locator(`[data-testid="analyzing-indicator-${areaID}"]`);
      await expect(overlay).toBeVisible({ timeout: 5_000 });
      await expect(overlay.locator('.area-analysing-text')).toHaveText('Analyzing your space...');
    } finally {
      await apiContext.post('/control/fast');
      await apiContext.dispose();
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });

  test('upload shows analyzing indicator then 3 items appear', async ({ page, ollamaPort }) => {
    const jpegFixture = createJpegFixture();
    const apiContext = await request.newContext({ baseURL: `http://localhost:${ollamaPort}` });

    try {
      const name = `E2E UploadStream ${Date.now()}`;
      const areaID = await createArea(page, name);

      // Slow mode so overlay is visible long enough to assert before items load.
      await apiContext.post('/control/slow');

      await uploadPhoto(page, areaID, jpegFixture);

      // Analyzing indicator should be visible while upload is in progress.
      await expect(page.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).toBeVisible({ timeout: 5_000 });

      // 3 item rows should appear after upload completes.
      const card = page.locator(`[data-testid="area-card-${areaID}"]`);
      await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 15_000 });
    } finally {
      await apiContext.post('/control/fast');
      await apiContext.dispose();
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });

  test('file input is disabled during upload and re-enabled after', async ({ page, ollamaPort }) => {
    const jpegFixture = createJpegFixture();
    const apiContext = await request.newContext({ baseURL: `http://localhost:${ollamaPort}` });

    try {
      const name = `E2E UploadBtnState ${Date.now()}`;
      const areaID = await createArea(page, name);

      // Slow mode keeps the upload in progress long enough to assert disabled state.
      await apiContext.post('/control/slow');

      await uploadPhoto(page, areaID, jpegFixture);

      const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
      // File input should be disabled while uploading.
      await expect(fileInput).toBeDisabled({ timeout: 5_000 });

      // After upload completes, file input re-enabled.
      await expect(fileInput).toBeEnabled({ timeout: 15_000 });
    } finally {
      await apiContext.post('/control/fast');
      await apiContext.dispose();
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });

  // Regression test for kitchinv-uh7: after a successful upload, if a
  // subsequent upload fails, the area must show the previous photo and items
  // both immediately (no refresh) and after a page refresh.
  test('failed re-upload restores previous photo and items', async ({ page, ollamaPort }) => {
    const jpegFixture = createJpegFixture();
    const apiContext = await request.newContext({ baseURL: `http://localhost:${ollamaPort}` });

    try {
      const name = `E2E UploadFailRestore ${Date.now()}`;
      const areaID = await createArea(page, name);
      const card = page.locator(`[data-testid="area-card-${areaID}"]`);

      // First upload succeeds — 3 items appear.
      await uploadPhoto(page, areaID, jpegFixture);
      await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 15_000 });

      // Make the next upload fail at the vision API level.
      await apiContext.post('/control/fail');

      // Attempt second upload — should fail.
      await uploadPhoto(page, areaID, jpegFixture);

      // Error toast must appear.
      await expect(page.locator('.toast')).toBeVisible({ timeout: 5_000 });

      // Card must restore to previous state: photo present, 3 items, no analysing banner.
      await expect(card.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).not.toBeVisible({ timeout: 5_000 });
      await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 5_000 });

      // After page refresh the area must still show previous photo and items.
      await page.goto('/areas');
      const cardAfterRefresh = page.locator(`[data-testid="area-card-${areaID}"]`);
      await expect(cardAfterRefresh.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).not.toBeVisible({ timeout: 5_000 });
      await expect(cardAfterRefresh.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 5_000 });
    } finally {
      await apiContext.post('/control/fail/reset');
      await apiContext.post('/control/fast');
      await apiContext.dispose();
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });

  // Regression test for kitchinv-c0o: after upload completes, item controls
  // (delete, edit) must be interactive immediately without a page refresh.
  test('item controls are interactive after upload without refresh', async ({ page, ollamaPort }) => {
    const jpegFixture = createJpegFixture();
    const apiContext = await request.newContext({ baseURL: `http://localhost:${ollamaPort}` });

    try {
      const name = `E2E UploadInteractive ${Date.now()}`;
      const areaID = await createArea(page, name);
      const card = page.locator(`[data-testid="area-card-${areaID}"]`);

      await uploadPhoto(page, areaID, jpegFixture);
      await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 15_000 });

      // Delete the first item without refreshing — must work immediately.
      const firstRow = card.locator('[data-testid="item-row"]').first();
      await firstRow.locator('button[aria-label="Delete item"]').click();

      // Item count should drop to 2.
      await expect(card.locator('[data-testid="item-row"]')).toHaveCount(2, { timeout: 5_000 });

      // Add-item input must be present and functional without a refresh.
      const addInput = card.locator('.add-item-name');
      await expect(addInput).toBeVisible({ timeout: 2_000 });
      await addInput.fill('manually added item');
      await addInput.press('Enter');

      // Item count should rise to 3.
      await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 5_000 });
    } finally {
      await apiContext.dispose();
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });

  test('second upload replaces items (still 3 item-rows)', async ({ page, ollamaPort }) => {
    const jpegFixture = createJpegFixture();
    const apiContext = await request.newContext({ baseURL: `http://localhost:${ollamaPort}` });

    try {
      const name = `E2E UploadReplace ${Date.now()}`;
      const areaID = await createArea(page, name);
      const card = page.locator(`[data-testid="area-card-${areaID}"]`);

      // First upload.
      await uploadPhoto(page, areaID, jpegFixture);
      await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 15_000 });

      // Second upload (replace photo).
      await uploadPhoto(page, areaID, jpegFixture);
      await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 15_000 });
    } finally {
      await apiContext.dispose();
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });

  // Regression: after a second upload the photo <img> src must include a new
  // cache-busting parameter so the browser fetches the new image without a
  // page refresh. Without this fix the src URL is identical after replacement
  // and the browser shows the stale cached image.
  test('photo src changes after second upload (cache-bust)', async ({ page }) => {
    const jpegFixture = createJpegFixture();

    try {
      const name = `E2E PhotoCacheBust ${Date.now()}`;
      const areaID = await createArea(page, name);
      const card = page.locator(`[data-testid="area-card-${areaID}"]`);
      const img = card.locator('.area-photo-img');

      await uploadPhoto(page, areaID, jpegFixture);
      await expect(img).toHaveAttribute('src', /\?v=/, { timeout: 5_000 });
      const srcAfterFirst = await img.getAttribute('src');

      await uploadPhoto(page, areaID, jpegFixture);
      // Wait for the card reload to complete — src must change to a new cache-buster value.
      await expect(img).not.toHaveAttribute('src', srcAfterFirst!, { timeout: 5_000 });
      const srcAfterSecond = await img.getAttribute('src');

      expect(srcAfterSecond).not.toBe(srcAfterFirst);
    } finally {
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });
});
