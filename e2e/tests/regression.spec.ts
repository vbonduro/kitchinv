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

  test('Bug 3: analyzing overlay is server-rendered, not JS-only', async ({ page }) => {
    const name = `E2E RegServerSpinner ${Date.now()}`;
    const areaID = await createArea(page, name);

    // Close the gate so the stream blocks after the photo is committed to DB.
    await apiContext.post(`http://localhost:${ollamaPort}/control/gate/close`);

    // Start upload.
    const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
    await fileInput.setInputFiles(jpegFixture);

    // Poll until the stream is blocked at the gate: photo is in DB, no items yet.
    await waitForGate(apiContext, ollamaPort);

    // Open a fresh page — server sees hasPhoto && !hasItems and renders the analyzing overlay.
    const freshPage = await page.context().newPage();
    await freshPage.goto('/areas');

    // The fresh page must show the analyzing indicator (server-rendered: hasPhoto && !hasItems).
    await expect(freshPage.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).toBeVisible({ timeout: 5_000 });

    await freshPage.close();
  });

  test('Bug 4: analyzing overlay shown (not "no items") during analysis', async ({ page }) => {
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

    // Positive: analyzing indicator is present.
    await expect(card.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).toBeVisible({ timeout: 5_000 });

    // Negative: "no items" text is absent.
    await expect(card.locator('.no-items-text')).not.toBeVisible();
  });

  // Regression test for kitchinv-ywq: when a page is loaded with photo+no items
  // (analysis was interrupted), a remove button must be visible so the user can
  // escape the stuck analysing state.
  test('Bug 5: stuck analysing overlay has a visible remove-photo button', async ({ page }) => {
    const name = `E2E RegStuckOverlay ${Date.now()}`;
    const areaID = await createArea(page, name);

    // Close the gate to hold the stream open (photo in DB, no items).
    await apiContext.post(`http://localhost:${ollamaPort}/control/gate/close`);

    const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
    await fileInput.setInputFiles(jpegFixture);
    await waitForGate(apiContext, ollamaPort);

    // Reload to get the server-rendered stuck state.
    await page.goto('/areas');

    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    await expect(card.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).toBeVisible({ timeout: 5_000 });

    // The remove button must be immediately visible (no hover required).
    const removeBtn = card.locator('button[aria-label="Remove photo"]');
    await expect(removeBtn).toBeVisible({ timeout: 3_000 });
  });
});
