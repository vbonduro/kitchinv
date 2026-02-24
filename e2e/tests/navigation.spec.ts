import { test, expect } from '../fixtures';
import { Page, request } from '@playwright/test';
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
async function waitForGate(apiContext: Awaited<ReturnType<typeof request.newContext>>, timeoutMs = 10_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const resp = await apiContext.get(`http://localhost:${OLLAMA_PORT}/control/gate/waiting`);
    const { waiting } = await resp.json();
    if (waiting >= 1) return;
    await new Promise(r => setTimeout(r, 50));
  }
  throw new Error('Timed out waiting for stream to reach gate');
}

// Gate-based tests mutate shared mock state — run this suite serially.
test.describe.configure({ mode: 'serial' });

test.describe('Navigation', () => {
  let jpegFixture: string;
  let apiContext: Awaited<ReturnType<typeof request.newContext>>;

  test.beforeAll(async ({ playwright }) => {
    jpegFixture = createJpegFixture();
    apiContext = await playwright.request.newContext();
  });

  test.beforeEach(async ({ resetDB }) => { await resetDB(); });

  test.afterAll(async () => {
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/fast`);
    await apiContext.dispose();
    try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
  });

  test.afterEach(async () => {
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/fast`);
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/gate/open`);
  });

  test('navigate away mid-stream then back: analyzing overlay shows, then items appear', async ({ page }) => {
    const name = `E2E NavMidStream ${Date.now()}`;
    const areaID = await createArea(page, name);

    // Close the gate so the stream blocks after the photo is committed to DB.
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/gate/close`);

    // Start upload via file input.
    const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
    await fileInput.setInputFiles(jpegFixture);

    // Poll until the stream is blocked at the gate: photo is in DB, no items yet.
    await waitForGate(apiContext);

    // Navigate away while the gate is still closed (stream mid-flight).
    await page.goto('about:blank');

    // Navigate back — server renders card with photo but no items (analyzing state).
    await page.goto('/areas');

    // The card should show the analyzing overlay.
    await expect(page.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).toBeVisible({ timeout: 5_000 });

    // Open the gate — server-side goroutine resumes writing items to DB.
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/gate/open`);

    // Reload the page to see items.
    // The analyzing overlay goes away and items render server-side.
    await page.waitForTimeout(2_000);
    await page.goto('/areas');

    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 20_000 });
  });

  test('areas list shows analyzing overlay during analysis, not empty items text', async ({ page }) => {
    const name = `E2E NavAnalysing ${Date.now()}`;
    const areaID = await createArea(page, name);

    // Close the gate so the stream blocks after the photo is committed to DB.
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/gate/close`);

    // Start upload.
    const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
    await fileInput.setInputFiles(jpegFixture);

    // Poll until the stream is blocked at the gate: photo is in DB, no items yet.
    await waitForGate(apiContext);

    // Reload the areas list — server should render the analyzing state.
    await page.goto('/areas');

    const card = page.locator(`[data-testid="area-card-${areaID}"]`);

    // Should show the analyzing indicator.
    await expect(card.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).toBeVisible({ timeout: 5_000 });

    // Should NOT show "no items" text.
    await expect(card.locator('.no-items-text')).not.toBeVisible();
  });

  test('items persist after navigation (context.WithoutCancel)', async ({ page }) => {
    const name = `E2E NavPersist ${Date.now()}`;
    const areaID = await createArea(page, name);

    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/slow`);

    // Start upload and immediately navigate away.
    const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
    await fileInput.setInputFiles(jpegFixture);

    // Navigate away right after submitting.
    await page.goto('about:blank');

    // Wait long enough for slow mock to finish: 3 x 500ms = 1.5s → give 3s margin.
    await page.waitForTimeout(3_000);

    // Navigate back — items should be in DB now.
    await page.goto('/areas');
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 10_000 });
  });
});
