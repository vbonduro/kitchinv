// Per-worker servers (app + mock-ollama) are started and stopped by the
// worker fixture in fixtures.ts. No shared processes to clean up here.
export default async function globalTeardown() {}
