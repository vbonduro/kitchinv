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

/** Poll until at least one stream is blocked at the gate. */
async function waitForGate(apiContext: Awaited<ReturnType<typeof request.newContext>>, ollamaPort: number, timeoutMs = 10_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const resp = await apiContext.get(`http://localhost:${ollamaPort}/control/gate/waiting`);
    const { waiting } = await resp.json();
    if (waiting >= 1) return;
    await new Promise(r => setTimeout(r, 50));
  }
  throw new Error('Timed out waiting for stream to reach gate');
}

// Gate-based tests mutate shared mock state — run serially.
test.describe.configure({ mode: 'serial' });

test.describe('Upload & Analysis', () => {
  let jpegFixture: string;
  let apiContext: Awaited<ReturnType<typeof request.newContext>>;
  let ollamaPort: number;

  test.beforeAll(async ({ playwright, ollamaPort: port }) => {
    jpegFixture = createJpegFixture();
    apiContext = await playwright.request.newContext();
    ollamaPort = port;
  });

  test.beforeEach(async ({ resetDB }) => { await resetDB(); });

  test.afterAll(async () => {
    await apiContext.post(`http://localhost:${ollamaPort}/control/fast`);
    await apiContext.dispose();
    try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
  });

  test.afterEach(async () => {
    await apiContext.post(`http://localhost:${ollamaPort}/control/fast`);
    await apiContext.post(`http://localhost:${ollamaPort}/control/gate/open`);
    await apiContext.post(`http://localhost:${ollamaPort}/control/fail/reset`);
  });

  test('upload shows analyzing indicator then 3 items stream in', async ({ page }) => {
    const name = `E2E UploadStream ${Date.now()}`;
    const areaID = await createArea(page, name);

    // Close the gate so analyzing indicator is visible long enough to assert.
    await apiContext.post(`http://localhost:${ollamaPort}/control/gate/close`);

    await uploadPhoto(page, areaID, jpegFixture);

    // Wait for the stream to reach the gate (photo committed).
    await waitForGate(apiContext, ollamaPort);

    // Analyzing indicator should be visible.
    await expect(page.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).toBeVisible({ timeout: 5_000 });

    // Open the gate to let items stream through.
    await apiContext.post(`http://localhost:${ollamaPort}/control/gate/open`);

    // 3 item rows should appear after stream completes.
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 15_000 });
  });

  test('file input is disabled during upload and re-enabled after', async ({ page }) => {
    const name = `E2E UploadBtnState ${Date.now()}`;
    const areaID = await createArea(page, name);

    // Close the gate so the upload stays in progress long enough to assert.
    await apiContext.post(`http://localhost:${ollamaPort}/control/gate/close`);

    await uploadPhoto(page, areaID, jpegFixture);

    await waitForGate(apiContext, ollamaPort);

    const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
    // File input should be disabled while uploading.
    await expect(fileInput).toBeDisabled({ timeout: 5_000 });

    // Open the gate.
    await apiContext.post(`http://localhost:${ollamaPort}/control/gate/open`);

    // After stream completes, file input re-enabled.
    await expect(fileInput).toBeEnabled({ timeout: 15_000 });
  });

  // Regression test for kitchinv-uh7: after a successful upload, if a
  // subsequent upload fails, the area must show the previous photo and items
  // both immediately (no refresh) and after a page refresh.
  test('failed re-upload restores previous photo and items', async ({ page }) => {
    const name = `E2E UploadFailRestore ${Date.now()}`;
    const areaID = await createArea(page, name);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);

    // First upload succeeds — 3 items appear.
    await uploadPhoto(page, areaID, jpegFixture);
    await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 15_000 });

    // Make the next upload fail at the vision API level.
    await apiContext.post(`http://localhost:${ollamaPort}/control/fail`);

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
  });

  // Regression test for kitchinv-c0o: after upload completes, item controls
  // (delete, edit) must be interactive immediately without a page refresh.
  test('item controls are interactive after upload without refresh', async ({ page }) => {
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
  });

  test('second upload replaces items (still 3 item-rows)', async ({ page }) => {
    const name = `E2E UploadReplace ${Date.now()}`;
    const areaID = await createArea(page, name);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);

    // First upload.
    await uploadPhoto(page, areaID, jpegFixture);
    await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 15_000 });

    // Second upload (replace photo).
    await uploadPhoto(page, areaID, jpegFixture);
    await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 15_000 });
  });
});
