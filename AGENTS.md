# Project Structure and Guidance

## Project Overview
Home library management web application built with Go and PostgreSQL.
Provides a RESTful API + OPDS catalog for managing a personal book collection.
Runs on a Raspberry Pi.

## Directory Structure
```
lbl/
├── bookarch/         # Book archive files (ZIP format)
├── db/
│   └── scripts/
├── logs/             # Application logs
├── src/              # Go source code
│   ├── main.go       # Entry point, all handlers, routes
│   ├── auth.go       # Auth handlers (unused)
│   ├── export.go     # Export/import handlers (unused)
│   ├── jwt.go        # JWT helpers (unused)
│   ├── opds.go       # OPDS XML catalog
│   ├── reading.go    # Reading progress (unused)
│   ├── recommendations.go  # Recommendations (unused)
│   ├── main_test.go  # Tests
│   ├── schema.sql    # Embedded database schema (tables, indexes, triggers)
│   ├── config/
│   │   └── config.go      # TOML config struct, Load(), DefaultConfig()
│   └── utils/
│       ├── llm_client.go   # OpenAI-compatible LLM client (title/author recognition)
│       ├── pdf_extract.go  # PDF text extraction
│       ├── docx_extract.go # DOCX text extraction
│       ├── doc_extract.go  # Binary DOC text extraction (OLE2)
│       ├── epub.go         # EPUB metadata parsing
│       ├── fb2.go          # FB2 metadata parsing
│       ├── fb2_test.go     # FB2 tests
│       ├── epub_test.go    # EPUB tests
│       └── zip_extract.go  # ZIP content type detection
├── static/           # Frontend assets
│   ├── css/style.css
│   ├── js/app.js
│   ├── js/import.js
│   └── favicon.ico
├── templates/
│   └── index.html    # SPA page
├── tempfld/          # Upload processing
├── testdata/         # Sample books for testing
├── config.toml.example   # Config template
├── Dockerfile            # Multi-stage build
├── Dockerfile.all-in-one # All-in-one (app + DB)
├── go.mod / go.sum       # Go module
├── startup.sh            # Container entrypoint
└── AGENTS.md / README.md
```

## Technology Stack
- **Language**: Go 1.25+
- **Web Framework**: Gin-Gonic
- **Database**: PostgreSQL 17
- **Driver**: github.com/lib/pq
- **Config**: TOML (github.com/BurntSushi/toml)
- **PDF**: github.com/ledongthuc/pdf
- **DOC**: github.com/richardlehane/mscfb
- **Frontend**: Vanilla JS, no frameworks
- **Meta recognition**: Ollama / OpenAI-compatible LLM
- **Target**: Raspberry Pi (ARM/Linux)

## Configuration (config.toml)
The app reads `config.toml` from the current directory (or `CONFIG_PATH` env var).
See `config.toml.example` for all options. Key sections:
- `[server]` — port, bind, enable_delete, log_level
- `[directories]` — bookarch, temp, logs, templates, static paths
- `[database]` — host, port, name, user, password, sslmode
- `[llm]` — base_url, model, token, prompt, prompt2, timeout

Overrides via environment: `DATABASE_URL`, `PORT`.

## API Endpoints

### Books & Metadata
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/books` | List books (?author=, ?book=, ?genre=, ?date_from=, ?date_to=, ?sort_by=, ?sort_order=, ?limit=, ?offset=) |
| GET | `/api/v1/books/search` | Search books |
| POST | `/api/v1/books` | Create book |
| GET | `/api/v1/books/:id` | Get book |
| PUT | `/api/v1/books/:id` | Update book |
| DELETE | `/api/v1/books/:id` | Delete book + orphaned work |
| GET | `/api/v1/books/:id/extended` | Extended info (ISBN, annotation, publisher, etc.) |
| PUT | `/api/v1/books/:id/extended` | Update extended info |
| PUT | `/api/v1/books/:id/shelf` | Toggle shelf (favorites) |
| GET | `/api/v1/books/:id/download` | Download book file (ZIP) |
| POST | `/api/v1/books/:id/cover` | Upload cover image |

### Authors & Genres & Tags
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/authors` | List authors with book tree |
| GET | `/api/v1/genres` | List genres |
| GET | `/api/v1/genres/tree` | Genre hierarchy with nested authors & books (`?genre=`, `?author=`, `?book=`) |
| POST | `/api/v1/genres` | Create genre |
| PUT | `/api/v1/genres/:id` | Update genre |
| DELETE | `/api/v1/genres/:id` | Delete genre (only if no books) |
| GET | `/api/v1/tags` | List tags |
| POST | `/api/v1/tags` | Create tag |
| GET | `/api/v1/persons` | List all persons |
| PUT | `/api/v1/persons/:id` | Update person name |
| GET | `/api/v1/languages` | List languages |

### Import (async)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/import/upload` | Upload files (multipart) → async import |
| POST | `/api/v1/import/directory` | Import from server directory |
| GET | `/api/v1/import/status` | Poll import progress |
| POST | `/api/v1/import/cancel` | Cancel running import |
| POST | `/api/v1/import/file` | Import single file (sync) |

### Shelf
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/shelf/count` | Books on shelf count |
| PUT | `/api/v1/shelf/clear` | Clear shelf |

### Other
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/config` | App config (enable_delete flag) |
| GET | `/debug/goroutines` | Goroutine dump |
| GET | `/api/v1/opds/catalog.xml` | OPDS root catalog |
| GET | `/api/v1/opds/latest.xml` | OPDS latest books |
| GET | `/api/v1/opds/genres.xml` | OPDS genres |
| GET | `/api/v1/opds/genre/:id.xml` | OPDS books by genre |
| GET | `/api/v1/opds/search.xml?q=` | OPDS search |
| GET | `/api/v1/opds/book/:id` | OPDS download |

## Frontend Pages (SPA)

### Вкладка Авторы
- Hierarchical tree view: author → work → edition
- Pagination (50 per page), summary row (authors, works, editions)
- Filters: by author name, book title, genre (with ё→е normalization)
- Shelf checkboxes per edition, "Добавить на полку" (all expanded), "Очистить полку"
- Edit author/book modal, delete book
- Download edition button

### Вкладка Книги
- Flat table: №, upload date, title, author, format (download link), shelf toggle, edit
- Filters: by author, title, genre (text), date range (from/to with date inputs)
- Column sorting: title, upload date, author, format (click headers)
- Pagination (50 per page)
- Shelf count, clear shelf button

### Вкладка Импорт
- Upload file form (FB2, EPUB, ZIP)
- Import from server directory
- Async progress polling with cancel button

### Вкладка Жанры
- Hierarchical tree view: genre → author → book (edition)
- Filters: by genre name, author name, book title
- Edit genre name inline via modal
- Shelf checkboxes per edition
- Download edition button

## LLM Book Recognition
- Extracts first 3 pages (up to 2000 chars) from PDF/DOC/DOCX
- Sends to OpenAI-compatible LLM (Ollama/llama.cpp)
- Prompt asks for `AUTHOR:` and `BOOKNAME:` in response
- Multiple authors supported (comma-separated in LLM response)
- All LLM calls are serialized via sync.Mutex
- Retry with prompt2 if first call returns empty
- Falls back to filename if LLM is unavailable or times out
- Always logged regardless of log_level

## Import Flow
1. Files uploaded via `/import/upload` → saved to `tempfld/`
2. Async goroutine processes each file:
   - SHA-256 hash duplicate check (blocks duplicate BEFORE LLM call)
   - Format detection (FB2/EPUB have native metadata, PDF/DOC/DOCX need LLM)
   - LLM recognition (if needed)
   - Save as ZIP archive in `bookarch/XXXXX/`
   - Insert into DB (works → editions → edition_files, with ISBN + genres)
3. Progress polled via `/import/status`
4. Cancel via `/import/cancel`
5. Temp directories cleaned up on completion

## Supported Formats
- **FB2** — native metadata (title, authors, genres, annotation, cover, ISBN)
- **EPUB** — native metadata (title, authors, language, ISBN, publisher, genres)
- **PDF** — LLM recognition from extracted text
- **DOCX** — LLM recognition from extracted text
- **DOC** — LLM recognition from extracted text (OLE2 + UTF-16LE)
- **ZIP** — auto-detects content type (FB2, EPUB, PDF, DOC, DOCX inside)

## Search & Filtering
- `normalizeQuery()` converts to lowercase + replaces ё→е for all search strings
- Indices: `persons.lower_fio` (VARCHAR(510) GIN trgm), `works.lower_original_title` (TEXT GIN trgm), `editions.lower_title` (TEXT GIN trgm)
- Trigger function `normalize_search_field()` fills lower_ fields via `REPLACE(LOWER(...), 'ё', 'е')`
- All search queries use indexed lower_ fields

## Build & Run

The app listens on **`0.0.0.0:9091`** by default (configurable in `config.toml` `[server]` section).

### Prerequisites

- PostgreSQL must be running and accessible (see `config.toml` `[database]` section)
- Config file must exist: `config.toml` in the project root or `CONFIG_PATH` env var

### Quick start

```bash
# Build
go build -o library_app ./src/

# Run in foreground (Ctrl+C to stop)
./library_app

# With env overrides
DATABASE_URL="host=..." PORT=9091 ./library_app
```

### Run persistently (background)

```bash
# Build (if needed)
go build -o library_app ./src/

# Kill previous instance (if any) — use `kill` by PID, NOT `pkill -f`
# because pkill -f "library_app" matches the grep process itself and hangs.
# Instead:
#   kill $(pgrep -f "^\./library_app$") 2>/dev/null; sleep 1

# Start in background
nohup ./library_app > library_app.log 2>&1 &

# Verify
sleep 2
curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:9091/
# Should print 200
```

> **Note**: The app listens on `0.0.0.0:9091`. Use `127.0.0.1:9091` or `localhost:9091` to access it locally. External access requires firewall rules on the host.

## Launch for Other Agents (Docker host-net mode)

The app runs inside the opencode Docker container, but its port is **not exposed to the host** by default. Other agents need the site accessible via the browser on the host machine. The solution: create a minimal Docker image from the binary and run it with `--net=host`.

### Prerequisites
- PostgreSQL must be running and accessible (see `config.toml` `[database]` section)
- Binary must be built: `go build -o library_app ./src/`
- Docker socket must be available (`/var/run/docker.sock`)

### Launch sequence

```bash
# 1. Kill any previously running instance (inside container or old Docker container)
docker rm -f library-app 2>/dev/null
pkill -f library_app 2>/dev/null; sleep 1

# 2. Create minimal Docker image from the binary
cd /tmp && mkdir -p library-app
cp /home/sergey/git/aitest/agents/lbl/library_app library-app/
cp -r /home/sergey/git/aitest/agents/lbl/templates /home/sergey/git/aitest/agents/lbl/static /home/sergey/git/aitest/agents/lbl/config.toml library-app/
tar -cf library-app.tar -C library-app .
docker import library-app.tar library-app:latest
rm -rf /tmp/library-app /tmp/library-app.tar

# 3. Run with host networking (accessible on host's localhost:9091)
docker run -d --name library-app --net=host \
  -v /home/sergey/git/aitest/agents/lbl/config.toml:/config.toml \
  -v /home/sergey/git/aitest/agents/lbl/bookarch:/bookarch \
  -v /home/sergey/git/aitest/agents/lbl/tempfld:/tempfld \
  -v /home/sergey/git/aitest/agents/lbl/logs:/logs \
  -v /home/sergey/git/aitest/agents/lbl/templates:/templates \
  -v /home/sergey/git/aitest/agents/lbl/static:/static \
  library-app /library_app

# 4. Verify
sleep 2
curl -s -o /dev/null -w "%{http_code}" http://localhost:9091/
# Should print 200
```

### Verification

```bash
# Check container is running
docker ps --filter name=library-app

# Check logs
docker logs library-app --tail 5

# Test API
curl -s http://localhost:9091/api/v1/config
curl -s http://localhost:9091/api/v1/books?limit=1

# Test frontend (should return HTML)
curl -s http://localhost:9091/ | head -3
```

### Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `bind: address already in use` | Old process still listening | Run `pkill -f library_app; docker rm -f library-app` first |

> **Note**: `pkill -f library_app` may hang if it matches its own grep/process. Use `kill $(pgrep -f '^\./library_app$') 2>/dev/null` instead.
| `connection refused` | Container not started or port mismatch | Check `docker logs library-app`; verify config.toml port is 9091 |
| PostgreSQL connection errors | DB not running | Check `pg_isready`; start with `sudo pg_ctlcluster $(pg_lsclusters -h | head -1 | awk '{print $1}') main start` |
| Docker Hub pull fails (403) | No registry access | Use `docker import` approach above (does not require pulling images) |

### Stop

```bash
docker rm -f library-app
```

### Rebuild

After code changes, rebuild the binary first, then repeat the full launch sequence:

```bash
go build -o library_app ./src/
```

## Testing

After every code change, run all tests **except** data reload (`TestImportBookFile` requires external test files):

```bash
go test -run 'Test[^I]|TestImport[^B]|TestImportBook[^F]' -count=1 ./src/
```
