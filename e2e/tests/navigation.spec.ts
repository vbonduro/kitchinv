import { test, expect } from '../fixtures';
import { Page, request } from '@playwright/test';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

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

test.describe('Navigation', () => {
  test('navigate away during upload then back: items appear after analysis completes', async ({ page, ollamaPort }) => {
    const jpegFixture = createJpegFixture();
    const apiContext = await request.newContext({ baseURL: `http://localhost:${ollamaPort}` });

    try {
      const name = `E2E NavMidStream ${Date.now()}`;
      const areaID = await createArea(page, name);

      // Slow mode so analysis takes time — gives us room to navigate away.
      await apiContext.post('/control/slow');

      const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
      await fileInput.setInputFiles(jpegFixture);

      // Wait for overlay to confirm upload started, then navigate away.
      await expect(page.locator(`[data-testid="analyzing-indicator-${areaID}"]`)).toBeVisible({ timeout: 5_000 });
      await page.goto('about:blank');

      // Wait long enough for slow mock to finish: 2s delay → give 6s margin for CI.
      await page.waitForTimeout(6_000);

      // Navigate back — items should be in DB now (server completed via context.WithoutCancel).
      await page.goto('/areas');
      const card = page.locator(`[data-testid="area-card-${areaID}"]`);
      await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 15_000 });
    } finally {
      await apiContext.post('/control/fast');
      await apiContext.dispose();
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });

  test('items persist after navigation (context.WithoutCancel)', async ({ page, ollamaPort }) => {
    const jpegFixture = createJpegFixture();
    const apiContext = await request.newContext({ baseURL: `http://localhost:${ollamaPort}` });

    try {
      const name = `E2E NavPersist ${Date.now()}`;
      const areaID = await createArea(page, name);

      await apiContext.post('/control/slow');

      const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
      await fileInput.setInputFiles(jpegFixture);

      // Navigate away right after submitting.
      await page.goto('about:blank');

      // Wait long enough for slow mock to finish: 2s delay → give 6s margin for CI.
      await page.waitForTimeout(6_000);

      // Navigate back — items should be in DB now.
      await page.goto('/areas');
      const card = page.locator(`[data-testid="area-card-${areaID}"]`);
      await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 15_000 });
    } finally {
      await apiContext.post('/control/fast');
      await apiContext.dispose();
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });
});
