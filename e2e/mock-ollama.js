#!/usr/bin/env node
// Mock Ollama server for E2E tests.
// Fakes the /api/generate endpoint, returning a fixed set of items as NDJSON.
//
// Slow mode control:
//   POST /control/slow   → enables slow mode (500ms delay between items)
//   POST /control/fast   → disables slow mode (default)
//   GET  /control/status → returns {"slow": true|false, "gate": "open"|"closed"}
//
// Gate control (deterministic stream blocking):
//   POST /control/gate/close → next generate call blocks before emitting anything
//   POST /control/gate/open  → releases all waiting streams; they run to completion
//
// Gate workflow:
//   1. POST /control/gate/close
//   2. trigger upload — server commits photo to DB, then blocks at gate
//   3. test makes assertions (photo exists, no items yet)
//   4. POST /control/gate/open — stream flows to completion
//   5. test makes post-stream assertions

'use strict';

const http = require('http');

const PORT = parseInt(process.env.MOCK_OLLAMA_PORT || '19434', 10);

// Items the mock always returns as a structured JSON response.
const JSON_RESPONSE = JSON.stringify({
  status: 'ok',
  items: [
    { name: 'Milk',         quantity: '2 liters',  notes: 'semi-skimmed' },
    { name: 'Butter',       quantity: '1 block',   notes: 'opened' },
    { name: 'Orange Juice', quantity: '1 carton',  notes: null },
  ],
});

let slowMode = false;

// Gate: when closed, streams block before emitting any items until opened.
let gateClosed = false;
let gateWaiters = []; // resolve functions from streams waiting at the gate
let gateWaitCount = 0; // number of streams currently blocked at the gate

// Fail mode: next generate call returns 500 (simulates vision API rejection).
let failNextCall = false;

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// waitForGate returns a promise that resolves immediately if the gate is open,
// or waits until POST /control/gate/open is called.
// While waiting, increments gateWaitCount so tests can poll /control/gate/waiting.
function waitForGate() {
  if (!gateClosed) return Promise.resolve();
  gateWaitCount++;
  return new Promise((resolve) => {
    gateWaiters.push(() => { gateWaitCount--; resolve(); });
  });
}

async function handleGenerate(req, res) {
  if (failNextCall) {
    failNextCall = false;
    res.writeHead(500, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'simulated failure' }));
    return;
  }
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
    // Non-streaming: return single JSON object containing the structured response.
    // In slow mode, delay the response so tests can assert upload-in-progress state.
    if (slowMode) await sleep(2000);
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ response: JSON_RESPONSE, done: true }));
    return;
  }

  // Streaming NDJSON — emit the full JSON response as a single chunk.
  res.writeHead(200, { 'Content-Type': 'application/x-ndjson' });

  await waitForGate();

  if (slowMode) await sleep(500);
  res.write(JSON.stringify({ response: JSON_RESPONSE, done: false }) + '\n');
  res.write(JSON.stringify({ response: '', done: true }) + '\n');
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
  if (req.method === 'POST' && req.url === '/control/fail') {
    failNextCall = true;
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ fail: true }));
    return;
  }
  if (req.method === 'POST' && req.url === '/control/fail/reset') {
    failNextCall = false;
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ fail: false }));
    return;
  }
  if (req.method === 'POST' && req.url === '/control/gate/close') {
    gateClosed = true;
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ gate: 'closed' }));
    return;
  }
  if (req.method === 'POST' && req.url === '/control/gate/open') {
    gateClosed = false;
    const waiting = gateWaiters.splice(0);
    for (const resolve of waiting) resolve();
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ gate: 'open', released: waiting.length }));
    return;
  }
  if (req.method === 'GET' && req.url === '/control/gate/waiting') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ waiting: gateWaitCount }));
    return;
  }
  if (req.method === 'GET' && req.url === '/control/status') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ slow: slowMode, gate: gateClosed ? 'closed' : 'open' }));
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
