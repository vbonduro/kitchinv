import { test as base } from '@playwright/test';
import { spawn, ChildProcess } from 'child_process';
import * as fs from 'fs';
import * as http from 'http';
import * as os from 'os';
import * as path from 'path';

const ROOT = path.resolve(__dirname, '..');
const BASE_APP_PORT    = parseInt(process.env.APP_PORT    || '9090',  10);
const BASE_OLLAMA_PORT = parseInt(process.env.OLLAMA_PORT || '19434', 10);

// Each worker is a separate Node process with its own module scope, so this
// counter is per-worker. Tests within a worker run serially, so incrementing
// it is safe — each test gets a unique slot and ports are free before reuse.
let slotCounter = 0;

/** Poll an HTTP endpoint until it returns 200/3xx. */
function waitForPort(port: number, timeoutMs = 15_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  return new Promise((resolve, reject) => {
    function attempt() {
      const req = http.get(`http://localhost:${port}/`, (res) => {
        res.resume();
        resolve();
      });
      req.on('error', () => {
        if (Date.now() >= deadline) reject(new Error(`Timed out waiting for port ${port}`));
        else setTimeout(attempt, 200);
      });
      req.setTimeout(500, () => { req.destroy(); setTimeout(attempt, 200); });
    }
    attempt();
  });
}

export type TestFixtures = {
  appPort: number;
  ollamaPort: number;
};

/**
 * Per-test server pair. Each test gets its own kitchinv + mock-ollama started
 * fresh and torn down on completion. No shared state, no resetDB needed.
 *
 * Port layout (2 ports per slot, 200 slots per worker):
 *   app    = BASE_APP_PORT    + workerIndex * 400 + slot * 2
 *   ollama = BASE_OLLAMA_PORT + workerIndex * 400 + slot * 2 + 1
 */
export const test = base.extend<TestFixtures>({
  ollamaPort: async ({}, use, workerInfo) => {
    const slot = slotCounter++;
    const port = BASE_OLLAMA_PORT + workerInfo.workerIndex * 400 + slot * 2;
    const mockOllama: ChildProcess = spawn('node', [path.join(__dirname, 'mock-ollama.js')], {
      env: { ...process.env, MOCK_OLLAMA_PORT: String(port) },
      stdio: 'inherit',
    });
    await waitForPort(port);
    await use(port);
    mockOllama.kill('SIGTERM');
  },

  appPort: async ({ ollamaPort }, use, workerInfo) => {
    // App port sits one below the ollama port in each pair.
    const slot = slotCounter - 1;
    const port = BASE_APP_PORT + workerInfo.workerIndex * 400 + slot * 2;
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'kitchinv-e2e-'));
    const photoDir = path.join(tmpDir, 'photos');
    fs.mkdirSync(photoDir, { recursive: true });
    const app: ChildProcess = spawn(path.join(ROOT, 'kitchinv'), [], {
      env: {
        ...process.env,
        LISTEN_ADDR:      `:${port}`,
        DB_PATH:          ':memory:',
        PHOTO_LOCAL_PATH: photoDir,
        VISION_BACKEND:   'ollama',
        OLLAMA_HOST:      `http://localhost:${ollamaPort}`,
        OLLAMA_MODEL:     'moondream',
      },
      stdio: 'inherit',
    });
    await waitForPort(port);
    await use(port);
    app.kill('SIGTERM');
    fs.rmSync(tmpDir, { recursive: true, force: true });
  },

  baseURL: async ({ appPort }, use) => {
    await use(`http://localhost:${appPort}`);
  },
});

export { expect } from '@playwright/test';
