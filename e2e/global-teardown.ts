import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

export default async function globalTeardown() {
  const pidFile = process.env._E2E_PID_FILE;
  if (!pidFile || !fs.existsSync(pidFile)) {
    console.warn('[teardown] pid file not found, skipping cleanup');
    return;
  }

  let state: { appPid: number; mockPid: number; tmpDir: string };
  try {
    state = JSON.parse(fs.readFileSync(pidFile, 'utf8'));
  } catch {
    console.warn('[teardown] could not read pid file');
    return;
  }

  // Terminate processes.
  for (const [name, pid] of [['kitchinv', state.appPid], ['mock-ollama', state.mockPid]] as const) {
    try {
      process.kill(pid, 'SIGTERM');
      console.log(`[teardown] sent SIGTERM to ${name} (pid ${pid})`);
    } catch (e: any) {
      if (e.code !== 'ESRCH') {
        console.warn(`[teardown] could not kill ${name} (pid ${pid}):`, e.message);
      }
    }
  }

  // Clean up temp dir.
  try {
    fs.rmSync(state.tmpDir, { recursive: true, force: true });
    console.log(`[teardown] removed temp dir ${state.tmpDir}`);
  } catch (e: any) {
    console.warn('[teardown] could not remove temp dir:', e.message);
  }

  // Remove pid file.
  try {
    fs.unlinkSync(pidFile);
  } catch { /* ignore */ }
}
