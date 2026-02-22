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

  test.afterAll(async () => {
    // Ensure slow mode is reset.
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/fast`);
    await apiContext.dispose();
    try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
  });

  test.afterEach(async () => {
    // Reset slow mode and open the gate in case a test left it closed.
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/fast`);
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/gate/open`);
  });

  test('navigate away mid-stream then back: spinner shows, then items appear via polling', async ({ page }) => {
    const name = `E2E NavMidStream ${Date.now()}`;
    await createArea(page, name);
    const areaUrl = await openArea(page, name);

    // Close the gate so the stream blocks after the photo is committed to DB.
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/gate/close`);

    // Start upload.
    const [fc] = await Promise.all([
      page.waitForEvent('filechooser'),
      page.click('#photo-input'),
    ]);
    await fc.setFiles(jpegFixture);
    await page.click('#upload-btn');

    // Poll until the stream is blocked at the gate: photo is in DB, no items yet.
    await waitForGate(apiContext);

    // Navigate away while the gate is still closed (stream mid-flight).
    await page.goto('/areas');

    // Navigate back — gate still closed, so server renders spinner (hasPhoto && !hasItems).
    await page.goto(areaUrl);

    // Open the gate — server-side goroutine resumes writing items to DB.
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/gate/open`);

    // JS polls /items every 2s and replaces the list when items arrive.
    await expect(page.locator('.item-row')).toHaveCount(3, { timeout: 20_000 });
  });

  test('areas list shows "Analysing…" card during analysis, not "No items"', async ({ page }) => {
    const name = `E2E NavAnalysingCard ${Date.now()}`;
    await createArea(page, name);
    await openArea(page, name);

    // Close the gate so the stream blocks after the photo is committed to DB.
    await apiContext.post(`http://localhost:${OLLAMA_PORT}/control/gate/close`);

    // Start upload.
    const [fc] = await Promise.all([
      page.waitForEvent('filechooser'),
      page.click('#photo-input'),
    ]);
    await fc.setFiles(jpegFixture);

    await page.click('#upload-btn');

    // Poll until the stream is blocked at the gate: photo is in DB, no items yet.
    await waitForGate(apiContext);

    // Gate is still closed: photo exists in DB, no items yet. Navigate to list.
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
