import { test, expect } from '../fixtures';
import { Page, request as playwrightRequest } from '@playwright/test';
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
  const tmpFile = path.join(os.tmpdir(), `e2e-snapshot-${Date.now()}.jpg`);
  fs.writeFileSync(tmpFile, buf);
  return tmpFile;
}

async function createArea(page: Page, name: string): Promise<string> {
  await page.goto('/areas');
  if (!await page.locator('body').evaluate((el) => el.hasAttribute('data-edit-mode'))) {
    await page.locator('[data-testid="edit-mode-btn"]').click();
  }
  await page.click('[data-testid="new-area-btn"]');
  await page.locator('#new-area-dialog').waitFor({ state: 'visible' });
  await page.fill('#new-area-dialog input[name="name"]', name);
  await page.click('#new-area-dialog button[type="submit"]');
  const card = page.locator('.area-card', { hasText: name });
  await card.waitFor({ timeout: 10_000 });
  return (await card.getAttribute('data-testid'))!.replace('area-card-', '');
}

async function uploadPhoto(page: Page, areaID: string, jpegFixture: string) {
  await page.locator(`[data-testid="photo-input-${areaID}"]`).setInputFiles(jpegFixture);
  // Wait for upload to complete: item rows appear.
  await page.locator(`[data-testid="area-card-${areaID}"] [data-testid="item-row"]`).first().waitFor({ timeout: 15_000 });
}

test.describe('Inventory Snapshots', () => {
  test('no snapshot created on first upload', async ({ page, appPort }) => {
    const jpegFixture = createJpegFixture();
    try {
      const name = `E2E Snapshot First ${Date.now()}`;
      const areaID = await createArea(page, name);

      await uploadPhoto(page, areaID, jpegFixture);

      const api = await playwrightRequest.newContext({ baseURL: `http://localhost:${appPort}` });
      try {
        const resp = await api.get(`/areas/${areaID}/snapshots`);
        expect(resp.ok()).toBeTruthy();
        const snapshots = await resp.json();
        expect(snapshots).toHaveLength(0);
      } finally {
        await api.dispose();
      }
    } finally {
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });

  test('second upload creates a snapshot of the previous inventory', async ({ page, appPort }) => {
    const jpegFixture = createJpegFixture();
    try {
      const name = `E2E Snapshot Second ${Date.now()}`;
      const areaID = await createArea(page, name);

      // First upload — mock-ollama returns Milk, Butter, Orange Juice.
      await uploadPhoto(page, areaID, jpegFixture);

      // Second upload — should snapshot the first inventory before replacing.
      await uploadPhoto(page, areaID, jpegFixture);

      const api = await playwrightRequest.newContext({ baseURL: `http://localhost:${appPort}` });
      try {
        const resp = await api.get(`/areas/${areaID}/snapshots`);
        expect(resp.ok()).toBeTruthy();
        const snapshots = await resp.json();
        expect(snapshots).toHaveLength(1);

        const snap = snapshots[0];
        expect(snap.AreaID).toBe(Number(areaID));
        expect(snap.TakenAt).toBeTruthy();

        const names: string[] = snap.Items.map((i: { name: string }) => i.name);
        expect(names).toEqual(expect.arrayContaining(['Milk', 'Butter', 'Orange Juice']));
        expect(snap.Items).toHaveLength(3);
      } finally {
        await api.dispose();
      }
    } finally {
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });

  test('each re-upload adds another snapshot', async ({ page, appPort }) => {
    const jpegFixture = createJpegFixture();
    try {
      const name = `E2E Snapshot Multi ${Date.now()}`;
      const areaID = await createArea(page, name);

      await uploadPhoto(page, areaID, jpegFixture); // upload 1 → no snapshot
      await uploadPhoto(page, areaID, jpegFixture); // upload 2 → snapshot of upload 1
      await uploadPhoto(page, areaID, jpegFixture); // upload 3 → snapshot of upload 2

      const api = await playwrightRequest.newContext({ baseURL: `http://localhost:${appPort}` });
      try {
        const resp = await api.get(`/areas/${areaID}/snapshots`);
        expect(resp.ok()).toBeTruthy();
        const snapshots = await resp.json();
        expect(snapshots).toHaveLength(2);
      } finally {
        await api.dispose();
      }
    } finally {
      try { fs.unlinkSync(jpegFixture); } catch { /* ignore */ }
    }
  });
});
