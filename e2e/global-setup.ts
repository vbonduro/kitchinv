import { execSync } from 'child_process';
import * as path from 'path';

const ROOT = path.resolve(__dirname, '..');

export default async function globalSetup() {
  // Build the binary once before any worker starts.
  console.log('[setup] building kitchinv...');
  execSync('make build', { cwd: ROOT, stdio: 'inherit' });
  console.log('[setup] build complete');
}
