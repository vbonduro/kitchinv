import { execSync, spawn } from 'child_process';
import * as fs from 'fs';
import * as http from 'http';
import * as os from 'os';
import * as path from 'path';

const ROOT = path.resolve(__dirname, '..');
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

export default async function globalSetup() {
  // Build the binary.
  console.log('[setup] building kitchinv...');
  execSync('make build', { cwd: ROOT, stdio: 'inherit' });

  // Start mock Ollama (shared across all workers).
  console.log(`[setup] starting mock-ollama on port ${OLLAMA_PORT}...`);
  const mockOllama = spawn('node', [path.join(__dirname, 'mock-ollama.js')], {
    env: { ...process.env, MOCK_OLLAMA_PORT: String(OLLAMA_PORT) },
    stdio: 'inherit',
  });

  await waitForPort(OLLAMA_PORT);
  console.log('[setup] mock-ollama ready');

  // Persist mock-ollama PID for teardown.
  const pidFile = path.join(os.tmpdir(), `kitchinv-e2e-${process.pid}.json`);
  fs.writeFileSync(pidFile, JSON.stringify({ mockPid: mockOllama.pid }));
  process.env._E2E_PID_FILE = pidFile;
}
