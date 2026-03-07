# Benchmark Fixtures

Each subdirectory is one fixture. Place your photo and ground truth here:

```
fixtures/
  my-fridge/
    image.jpg          ← your photo (.jpg, .jpeg, .png, or .webp)
    ground_truth.json  ← expected items
```

`ground_truth.json` format:

```json
{
  "items": [
    {"name": "Milk", "quantity": 2},
    {"name": "Eggs", "quantity": 12}
  ]
}
```

`name` — the type of food item (not brand-specific).
`quantity` — how many of that item are visible in the photo.

Run the benchmark:

```bash
# Human-readable output
make benchmark

# JSON output (pipe to jq, save to file, etc.)
make benchmark-json
```

The backend is controlled by env vars — same as the main server:

```bash
VISION_BACKEND=gemini GEMINI_API_KEY=... make benchmark
VISION_BACKEND=claude CLAUDE_API_KEY=... CLAUDE_MODEL=claude-opus-4-6 make benchmark
```
