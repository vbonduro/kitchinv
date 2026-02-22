import { execSync, spawn } from 'child_process';
import * as fs from 'fs';
import * as http from 'http';
import * as os from 'os';
import * as path from 'path';

const ROOT = path.resolve(__dirname, '..');
const APP_PORT = parseInt(process.env.APP_PORT || '9090', 10);
const OLLAMA_PORT = parseInt(process.env.OLLAMA_PORT || '19434', 10);

/** Poll an HTTP endpoint until it returns any response (including redirects). */
function waitForPort(port: number, timeoutMs = 15_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  return new Promise((resolve, reject) => {
    function attempt() {
      const req = http.get(`http://localhost:${port}/`, (res) => {
        res.resume(); // drain
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

export default async function globalSetup() {
  // Build the binary.
  console.log('[setup] building kitchinv...');
  execSync('make build', { cwd: ROOT, stdio: 'inherit' });

  // Temp dir for DB and photos.
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'kitchinv-e2e-'));
  const dbPath = path.join(tmpDir, 'kitchinv.db');
  const photoDir = path.join(tmpDir, 'photos');
  fs.mkdirSync(photoDir, { recursive: true });

  // Start mock Ollama.
  console.log(`[setup] starting mock-ollama on port ${OLLAMA_PORT}...`);
  const mockOllama = spawn('node', [path.join(__dirname, 'mock-ollama.js')], {
    env: { ...process.env, MOCK_OLLAMA_PORT: String(OLLAMA_PORT) },
    stdio: 'inherit',
  });

  // Start the app server.
  console.log(`[setup] starting kitchinv on port ${APP_PORT}...`);
  const app = spawn(path.join(ROOT, 'kitchinv'), [], {
    env: {
      ...process.env,
      LISTEN_ADDR: `:${APP_PORT}`,
      DB_PATH: dbPath,
      PHOTO_LOCAL_PATH: photoDir,
      VISION_BACKEND: 'ollama',
      OLLAMA_HOST: `http://localhost:${OLLAMA_PORT}`,
      OLLAMA_MODEL: 'moondream',
    },
    stdio: 'inherit',
  });

  // Wait for both processes to be ready.
  await waitForPort(OLLAMA_PORT);
  console.log('[setup] mock-ollama ready');
  await waitForPort(APP_PORT);
  console.log('[setup] kitchinv ready');

  // Persist state for teardown.
  const pidFile = path.join(os.tmpdir(), `kitchinv-e2e-${process.pid}.json`);
  fs.writeFileSync(pidFile, JSON.stringify({
    appPid: app.pid,
    mockPid: mockOllama.pid,
    tmpDir,
  }));

  // Expose pidFile path so teardown can read it.
  process.env._E2E_PID_FILE = pidFile;
}
