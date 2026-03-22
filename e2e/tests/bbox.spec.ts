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
  // Mock returns 4 entries (Milk×2, Butter, OJ) which merge to 3 items.
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

    // Mock returns 3 unique items but Milk has 2 bboxes — expect 4 rects total.
    const rects = svg.locator('.bbox-rect');
    await expect(rects).toHaveCount(4);

    // Hover first row to activate the rect, then screenshot for visual inspection.
    await card.locator('[data-testid="item-row"]').nth(0).hover();
    await page.screenshot({ path: '/tmp/bbox-test.png' });
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
    const activeRect = svg.locator(`.bbox-rect[data-item-id="${itemID}"]`).first();
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

  test('hovering merged item highlights all its bboxes', async ({ page }) => {
    const areaID = await createAreaWithPhoto(page);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    const rows = card.locator('[data-testid="item-row"]');
    const svg = card.locator('.photo-wrapper .bbox-overlay');

    // Find the Milk row — it has 2 bboxes after merging.
    let milkRow = rows.nth(0);
    for (let i = 0; i < 3; i++) {
      const name = await rows.nth(i).locator('.item-name-cell').textContent();
      if (name?.toLowerCase().includes('milk')) {
        milkRow = rows.nth(i);
        break;
      }
    }

    const milkID = await milkRow.getAttribute('data-item-id');
    expect(milkID).toBeTruthy();

    // Hover the Milk row.
    await milkRow.hover();

    // All rects for Milk should be active.
    const milkRects = svg.locator(`.bbox-rect[data-item-id="${milkID}"]`);
    const milkRectCount = await milkRects.count();
    expect(milkRectCount).toBe(2);

    for (let i = 0; i < milkRectCount; i++) {
      await expect(milkRects.nth(i)).toHaveClass(/bbox-active/);
    }
  });

  test('clicking an item row locks the bbox highlight', async ({ page }) => {
    const areaID = await createAreaWithPhoto(page);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    const rows = card.locator('[data-testid="item-row"]');
    const svg = card.locator('.photo-wrapper .bbox-overlay');

    // Exit edit mode so toggleBBox is active.
    const body = page.locator('body');
    if (await body.evaluate((el) => el.hasAttribute('data-edit-mode'))) {
      await page.locator('[data-testid="edit-mode-btn"]').click();
    }

    const firstRow = rows.nth(0);
    const itemID = await firstRow.getAttribute('data-item-id');


    // Click the row.
    await firstRow.click();

    // Bbox should be locked — rect active, row active.
    const activeRect = svg.locator(`.bbox-rect[data-item-id="${itemID}"]`).first();
    await expect(activeRect).toHaveClass(/bbox-active/);
    await expect(firstRow).toHaveClass(/bbox-row-active/);

    // Move mouse away — highlight must STAY (tap-locked).
    await card.locator('.area-card-header').hover();
    await expect(activeRect).toHaveClass(/bbox-active/);
  });

  test('clicking the same item again unlocks the highlight', async ({ page }) => {
    const areaID = await createAreaWithPhoto(page);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    const rows = card.locator('[data-testid="item-row"]');
    const svg = card.locator('.photo-wrapper .bbox-overlay');

    const body = page.locator('body');
    if (await body.evaluate((el) => el.hasAttribute('data-edit-mode'))) {
      await page.locator('[data-testid="edit-mode-btn"]').click();
    }

    const firstRow = rows.nth(0);
    const itemID = await firstRow.getAttribute('data-item-id');
    const activeRect = svg.locator(`.bbox-rect[data-item-id="${itemID}"]`).first();

    // Lock then unlock.
    await firstRow.click();
    await expect(activeRect).toHaveClass(/bbox-active/);
    await firstRow.click();
    await expect(activeRect).not.toHaveClass(/bbox-active/);
  });

  test('clicking a different item switches the lock', async ({ page }) => {
    const areaID = await createAreaWithPhoto(page);
    const card = page.locator(`[data-testid="area-card-${areaID}"]`);
    const rows = card.locator('[data-testid="item-row"]');
    const svg = card.locator('.photo-wrapper .bbox-overlay');

    const body = page.locator('body');
    if (await body.evaluate((el) => el.hasAttribute('data-edit-mode'))) {
      await page.locator('[data-testid="edit-mode-btn"]').click();
    }

    const firstRow = rows.nth(0);
    const secondRow = rows.nth(1);
    const firstID = await firstRow.getAttribute('data-item-id');
    const secondID = await secondRow.getAttribute('data-item-id');

    await firstRow.click();
    await secondRow.click();

    // Second item should now be active, first should not.
    await expect(svg.locator(`.bbox-rect[data-item-id="${secondID}"]`).first()).toHaveClass(/bbox-active/);
    await expect(svg.locator(`.bbox-rect[data-item-id="${firstID}"]`).first()).not.toHaveClass(/bbox-active/);
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
