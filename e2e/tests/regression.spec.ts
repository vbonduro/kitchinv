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
  await page.click('[data-testid="new-area-btn"]');
  await page.locator('#new-area-dialog').waitFor({ state: 'visible' });
  await page.fill('#new-area-dialog input[name="name"]', name);
  await page.click('#new-area-dialog button[type="submit"]');
  const card = page.locator('.area-card', { hasText: name });
  await card.waitFor({ timeout: 10_000 });
  const testid = await card.getAttribute('data-testid');
  return testid!.replace('area-card-', '');
}

/** Poll until at least one stream is blocked at the gate (photo committed, no items yet). */
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

// Gate-based tests mutate shared mock state — run this suite serially.
test.describe.configure({ mode: 'serial' });

test.describe('Regression', () => {
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
  });

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

  // Bug 3 (updated): on page load with photo+no items (mid-stream on another connection),
  // the card must show the photo with controls — not a stuck analysing overlay.
  // The analysing overlay is JS-only (shown only on the tab that initiated the upload).
  test('Bug 3: page load with photo+no items shows photo with controls, not stuck overlay', async ({ page }) => {
    const name = `E2E RegServerSpinner ${Date.now()}`;
    const areaID = await createArea(page, name);

    // Close the gate so the stream blocks after the photo is committed to DB.
    await apiContext.post(`http://localhost:${ollamaPort}/control/gate/close`);

    // Start upload.
    const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
    await fileInput.setInputFiles(jpegFixture);

    // Poll until the stream is blocked at the gate: photo is in DB, no items yet.
    await waitForGate(apiContext, ollamaPort);

    // Open a fresh page — should show photo with controls, not the stuck analysing overlay.
    const freshPage = await page.context().newPage();
    await freshPage.goto('/areas');

    const freshCard = freshPage.locator(`[data-testid="area-card-${areaID}"]`);
    // Photo is visible.
    await expect(freshCard.locator('.area-photo-img')).toBeVisible({ timeout: 5_000 });
    // No stuck analysing overlay on a fresh page load.
    await expect(freshCard.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).not.toBeVisible();

    await freshPage.close();
  });

  // Bug 4 (updated): on page load with photo+no items, items section is visible
  // and the add-item row is accessible.
  test('Bug 4: page load with photo+no items shows empty items section', async ({ page }) => {
    const name = `E2E RegAnalysingCard ${Date.now()}`;
    const areaID = await createArea(page, name);

    // Close the gate so the stream blocks after the photo is committed to DB.
    await apiContext.post(`http://localhost:${ollamaPort}/control/gate/close`);

    const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
    await fileInput.setInputFiles(jpegFixture);

    // Poll until the stream is blocked at the gate: photo is in DB, no items yet.
    await waitForGate(apiContext, ollamaPort);

    // Reload the areas list.
    await page.goto('/areas');

    const card = page.locator(`[data-testid="area-card-${areaID}"]`);

    // No stuck analysing overlay.
    await expect(card.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).not.toBeVisible();

    // Items section and add-item row must be visible.
    await expect(card.locator('.items-section')).toBeVisible();
    await expect(card.locator('.add-item-name')).toBeVisible();
  });
});
