# Architecture

## Project layout

```
kitchinv/
├── cmd/kitchinv/main.go          # Entry point; dependency wiring
├── internal/
│   ├── config/                   # Env-var config loading
│   ├── db/
│   │   ├── db.go                 # Open SQLite, WAL mode, run migrations
│   │   └── migrations/           # 3 migration pairs (areas, photos, items)
│   ├── domain/types.go           # Area, Photo, Item structs
│   ├── store/
│   │   ├── area_store.go
│   │   ├── photo_store.go
│   │   └── item_store.go         # Includes case-insensitive search
│   ├── vision/
│   │   ├── vision.go             # VisionAnalyzer interface + shared prompt
│   │   ├── parse.go              # Parse "name | qty | notes" response lines
│   │   ├── ollama/               # Ollama adapter (HTTP)
│   │   └── claude/               # Claude adapter (Anthropic Messages API)
│   ├── photostore/
│   │   ├── photostore.go         # PhotoStore interface
│   │   └── local/                # Filesystem adapter with path-traversal guard
│   ├── service/area_service.go   # Business logic: upload → analyze → persist
│   └── web/
│       ├── server.go             # ServeMux routing + render helpers
│       ├── handler_area.go
│       ├── handler_upload.go
│       ├── handler_search.go
│       └── templates/            # Embedded html/template files
│           ├── base.html
│           ├── pages/            # areas, area_detail, search
│           └── partials/         # area_card, item_list, search_results
├── Dockerfile                    # Multi-stage, CGO_ENABLED=0 static binary
├── docker-compose.yml            # App + Ollama sidecar
└── Makefile
```

## Component overview

```
┌──────────┐    ┌──────────────┐    ┌──────────────────┐
│  Browser │───▶│  web.Server  │───▶│  AreaService     │
│  (HTMX)  │    │  (handlers)  │    │  (orchestration) │
└──────────┘    └──────────────┘    └────────┬─────────┘
                                             │
                    ┌────────────────────────┼─────────────────────┐
                    ▼                        ▼                     ▼
             ┌─────────────┐        ┌──────────────┐    ┌──────────────────┐
             │ store layer │        │ VisionAnalyzer│    │   PhotoStore     │
             │ (SQLite)    │        │ (interface)   │    │   (interface)    │
             └─────────────┘        └──────┬───────┘    └────────┬─────────┘
                                           │                     │
                                    ┌──────┴──────┐       ┌──────┴──────┐
                                    │ Ollama      │       │ local fs    │
                                    │ Claude      │       └─────────────┘
                                    └─────────────┘
```

Both `VisionAnalyzer` and `PhotoStore` are Go interfaces. Swap the backend by changing an environment variable — no code changes required.

The binary is fully static (`CGO_ENABLED=0`, pure-Go SQLite via `modernc.org/sqlite`), so the Docker image is a minimal Alpine container with no C runtime dependency.

## HTTP routes

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/` | Redirect to `/areas` |
| `GET` | `/areas` | List all areas |
| `POST` | `/areas` | Create area; returns `area_card` partial (HTMX) |
| `GET` | `/areas/{id}` | Area detail: photo + item list |
| `POST` | `/areas/{id}/photos` | Upload photo → analyze → replace items; returns `item_list` partial (HTMX) |
| `GET` | `/areas/{id}/photo` | Serve raw photo bytes |
| `DELETE` | `/areas/{id}` | Delete area; redirects to `/areas` (HTMX) |
| `GET` | `/search?q=...` | Search items across all areas |

HTMX handlers detect the `HX-Request: true` header and return only the relevant partial instead of a full page.
