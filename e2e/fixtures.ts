import { test as base, request } from '@playwright/test';
import { spawn, ChildProcess } from 'child_process';
import * as fs from 'fs';
import * as http from 'http';
import * as os from 'os';
import * as path from 'path';

const ROOT = path.resolve(__dirname, '..');
const BASE_PORT = parseInt(process.env.APP_PORT || '9090', 10);
const OLLAMA_PORT = parseInt(process.env.OLLAMA_PORT || '19434', 10);

/** Poll an HTTP endpoint until it returns any response. */
function waitForPort(port: number, timeoutMs = 15_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  return new Promise((resolve, reject) => {
    function attempt() {
      const req = http.get(`http://localhost:${port}/`, (res) => {
        res.resume();
        resolve();
      });
      req.on('error', () => {
        if (Date.now() >= deadline) {
          reject(new Error(`Timed out waiting for port ${port}`));
        } else {
          setTimeout(attempt, 200);
        }
      });
      req.setTimeout(500, () => {
        req.destroy();
        setTimeout(attempt, 200);
      });
    }
    attempt();
  });
}

export type WorkerFixtures = {
  appPort: number;
};

export type TestFixtures = {
  resetDB: () => Promise<void>;
};

/**
 * Custom test object that provides a per-worker kitchinv server.
 * Each worker gets its own server on a distinct port with an isolated
 * in-memory database.
 */
export const test = base.extend<TestFixtures, WorkerFixtures>({
  appPort: [async ({}, use, workerInfo) => {
    const port = BASE_PORT + workerInfo.workerIndex;

    // Create a temp dir for photos (DB is in-memory via KITCHINV_TEST_MODE).
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), `kitchinv-e2e-w${workerInfo.workerIndex}-`));
    const photoDir = path.join(tmpDir, 'photos');
    fs.mkdirSync(photoDir, { recursive: true });

    // Spawn the app server for this worker.
    const app: ChildProcess = spawn(path.join(ROOT, 'kitchinv'), [], {
      env: {
        ...process.env,
        LISTEN_ADDR: `:${port}`,
        PHOTO_LOCAL_PATH: photoDir,
        VISION_BACKEND: 'ollama',
        OLLAMA_HOST: `http://localhost:${OLLAMA_PORT}`,
        OLLAMA_MODEL: 'moondream',
        KITCHINV_TEST_MODE: '1',
      },
      stdio: 'inherit',
    });

    await waitForPort(port);

    await use(port);

    // Teardown: kill the server and clean up.
    app.kill('SIGTERM');
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }, { scope: 'worker' }],

  // Override baseURL per-worker so page.goto('/areas') uses the right port.
  baseURL: async ({ appPort }, use) => {
    await use(`http://localhost:${appPort}`);
  },

  // Per-test resetDB that hits the correct worker's server.
  resetDB: async ({ appPort }, use) => {
    const reset = async () => {
      const ctx = await request.newContext({ baseURL: `http://localhost:${appPort}` });
      await ctx.post('/control/reset');
      await ctx.dispose();
    };
    await use(reset);
  },
});

export { expect } from '@playwright/test';
