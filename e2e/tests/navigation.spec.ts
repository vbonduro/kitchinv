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

      // Poll until items appear: navigate back repeatedly until the card shows 3 rows
      // or we exceed the deadline. This avoids a fixed sleep that can flake on slow CI.
      const deadline = Date.now() + 30_000;
      let card = page.locator(`[data-testid="area-card-${areaID}"]`);
      while (Date.now() < deadline) {
        await page.goto('/areas');
        card = page.locator(`[data-testid="area-card-${areaID}"]`);
        const count = await card.locator('[data-testid="item-row"]').count();
        if (count === 3) break;
        await page.waitForTimeout(1_000);
      }
      await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 5_000 });
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

      // Poll until items appear: navigate back repeatedly until the card shows 3 rows
      // or we exceed the deadline. This avoids a fixed sleep that can flake on slow CI.
      const deadline = Date.now() + 30_000;
      let card = page.locator(`[data-testid="area-card-${areaID}"]`);
      while (Date.now() < deadline) {
        await page.goto('/areas');
        card = page.locator(`[data-testid="area-card-${areaID}"]`);
        const count = await card.locator('[data-testid="item-row"]').count();
        if (count === 3) break;
        await page.waitForTimeout(1_000);
      }
      await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 5_000 });
    } finally {
      await apiContext.post('/control/fast');
      await apiContext.dispose();
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });
});
