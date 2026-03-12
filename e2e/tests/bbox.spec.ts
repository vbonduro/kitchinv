import { test, expect } from '../fixtures';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { Page } from '@playwright/test';

/** Minimal valid JPEG: 512-byte buffer starting with JPEG magic bytes. */
function createJpegFixture(): string {
  const buf = Buffer.alloc(512, 0);
  buf[0] = 0xff;
  buf[1] = 0xd8;
  buf[2] = 0xff;
  buf[3] = 0xe0;
  const tmpFile = path.join(os.tmpdir(), `e2e-bbox-${Date.now()}.jpg`);
  fs.writeFileSync(tmpFile, buf);
  return tmpFile;
}

async function createAreaWithPhoto(page: Page): Promise<string> {
  await page.goto('/areas');

  // Enable edit mode.
  const body = page.locator('body');
  if (!await body.evaluate((el) => el.hasAttribute('data-edit-mode'))) {
    await page.locator('[data-testid="edit-mode-btn"]').click();
  }

  // Create area.
  await page.click('[data-testid="new-area-btn"]');
  await page.locator('#new-area-dialog').waitFor({ state: 'visible' });
  await page.fill('#new-area-dialog input[name="name"]', `BBox Test ${Date.now()}`);
  await page.click('#new-area-dialog button[type="submit"]');

  const card = page.locator('.area-card').last();
  await card.waitFor({ timeout: 10_000 });
  const testid = await card.getAttribute('data-testid');
  const areaID = testid!.replace('area-card-', '');

  // Upload photo and wait for items to appear.
  const jpegFixture = createJpegFixture();
  try {
    const fileInput = page.locator(`[data-testid="photo-input-${areaID}"]`);
    await fileInput.setInputFiles(jpegFixture);
    await expect(card.locator('[data-testid="item-row"]')).toHaveCount(3, { timeout: 15_000 });
  } finally {
    try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
  }

  return areaID;
}

test.describe('BBox overlay', () => {
  test('SVG overlay is present after photo upload with items', async ({ page }) => {
    const areaID = await createAreaWithPhoto(page);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);

    // The photo-wrapper must contain an SVG bbox-overlay.
    const svg = card.locator('.photo-wrapper .bbox-overlay');
    await expect(svg).toBeAttached();

    // All 3 items from the mock have bbox — expect 3 rects.
    const rects = svg.locator('.bbox-rect');
    await expect(rects).toHaveCount(3);
  });

  test('hovering an item row activates its bbox rect and dims other rows', async ({ page }) => {
    const areaID = await createAreaWithPhoto(page);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    const rows = card.locator('[data-testid="item-row"]');
    const svg = card.locator('.photo-wrapper .bbox-overlay');

    // Get item IDs from the rows.
    const firstRow = rows.nth(0);
    const itemID = await firstRow.getAttribute('data-item-id');
    expect(itemID).toBeTruthy();

    // Hover the first row.
    await firstRow.hover();

    // The matching rect must have class bbox-active.
    const activeRect = svg.locator(`.bbox-rect[data-item-id="${itemID}"]`);
    await expect(activeRect).toHaveClass(/bbox-active/);

    // The tbody must have class bbox-hover (dims other rows).
    const tbody = card.locator('.items-tbody');
    await expect(tbody).toHaveClass(/bbox-hover/);

    // The hovered row itself must have bbox-row-active.
    await expect(firstRow).toHaveClass(/bbox-row-active/);

    // The other rows must NOT have bbox-row-active.
    const secondRow = rows.nth(1);
    await expect(secondRow).not.toHaveClass(/bbox-row-active/);
  });

  test('moving mouse off item row clears highlight', async ({ page }) => {
    const areaID = await createAreaWithPhoto(page);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    const rows = card.locator('[data-testid="item-row"]');
    const tbody = card.locator('.items-tbody');

    // Hover then move away.
    await rows.nth(0).hover();
    await expect(tbody).toHaveClass(/bbox-hover/);

    // Move mouse to the card header (outside the table).
    await card.locator('.area-card-header').hover();

    // bbox-hover class must be removed.
    await expect(tbody).not.toHaveClass(/bbox-hover/);
  });
});
