import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

export default async function globalTeardown() {
  // Find the PID file written by global-setup.
  const pidFile = Object.keys(process.env)
    .filter((k) => k === '_E2E_PID_FILE')
    .map((k) => process.env[k])
    .find(Boolean);

  // Also try the conventional path in case env wasn't passed.
  const candidates = pidFile
    ? [pidFile]
    : fs.readdirSync(os.tmpdir())
        .filter((f) => f.startsWith('kitchinv-e2e-') && f.endsWith('.json'))
        .map((f) => path.join(os.tmpdir(), f));

  for (const file of candidates) {
    try {
      const data = JSON.parse(fs.readFileSync(file, 'utf-8'));
      if (data.mockPid) {
        try { process.kill(data.mockPid, 'SIGTERM'); } catch { /* already dead */ }
      }
      // Legacy: also kill app if present (from old setup).
      if (data.appPid) {
        try { process.kill(data.appPid, 'SIGTERM'); } catch { /* already dead */ }
      }
      fs.unlinkSync(file);
    } catch { /* ignore */ }
  }
}
