#!/usr/bin/env node
// Mock Ollama server for E2E tests.
// Fakes the /api/generate endpoint, returning a fixed set of items as NDJSON.
//
// Slow mode control:
//   POST /control/slow   → enables slow mode (500ms delay between items)
//   POST /control/fast   → disables slow mode (default)
//   GET  /control/status → returns {"slow": true|false}
//
// The test sets slow mode before uploading, then resets it when done.

'use strict';

const http = require('http');

const PORT = parseInt(process.env.MOCK_OLLAMA_PORT || '19434', 10);

// Items the mock always returns (name | qty | notes format).
const ITEMS = [
  'Milk | 2 liters | semi-skimmed',
  'Butter | 1 block | opened',
  'Orange Juice | 1 carton |',
];

let slowMode = false;

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function handleGenerate(req, res) {
  // Read and parse request body.
  const body = await new Promise((resolve, reject) => {
    let data = '';
    req.on('data', (chunk) => { data += chunk; });
    req.on('end', () => {
      try { resolve(JSON.parse(data)); }
      catch (e) { reject(e); }
    });
    req.on('error', reject);
  });

  const streaming = body.stream !== false; // default true

  if (!streaming) {
    // Non-streaming: return single JSON object.
    const response = ITEMS.join('\n');
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ response, done: true }));
    return;
  }

  // Streaming NDJSON — each chunk is one JSON line.
  res.writeHead(200, { 'Content-Type': 'application/x-ndjson' });

  for (let i = 0; i < ITEMS.length; i++) {
    if (slowMode) await sleep(500);

    // Emit the item text.
    const itemChunk = { response: ITEMS[i], done: false };
    res.write(JSON.stringify(itemChunk) + '\n');

    // Emit the newline separator (how Ollama terminates each line).
    const isLast = i === ITEMS.length - 1;
    const nlChunk = { response: '\n', done: isLast };
    res.write(JSON.stringify(nlChunk) + '\n');
  }

  res.end();
}

const server = http.createServer(async (req, res) => {
  // Health check.
  if (req.method === 'GET' && req.url === '/') {
    res.writeHead(200, { 'Content-Type': 'text/plain' });
    res.end('mock-ollama ok');
    return;
  }

  // Control endpoints for tests.
  if (req.method === 'POST' && req.url === '/control/slow') {
    slowMode = true;
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ slow: true }));
    return;
  }
  if (req.method === 'POST' && req.url === '/control/fast') {
    slowMode = false;
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ slow: false }));
    return;
  }
  if (req.method === 'GET' && req.url === '/control/status') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ slow: slowMode }));
    return;
  }

  if (req.method === 'POST' && req.url === '/api/generate') {
    try {
      await handleGenerate(req, res);
    } catch (err) {
      console.error('mock-ollama error:', err);
      if (!res.headersSent) {
        res.writeHead(500);
        res.end('internal error');
      }
    }
    return;
  }

  res.writeHead(404);
  res.end('not found');
});

server.listen(PORT, () => {
  console.log(`mock-ollama listening on port ${PORT}`);
});

// Graceful shutdown.
process.on('SIGTERM', () => server.close(() => process.exit(0)));
process.on('SIGINT',  () => server.close(() => process.exit(0)));
