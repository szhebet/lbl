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
| POST | `/api/v1/genres` | Create genre |
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

```bash
# Build
go build -o library_app ./src/

# Run (requires config.toml + PostgreSQL)
./library_app

# With env overrides
DATABASE_URL="host=..." PORT=9091 ./library_app
```

Run persistently:
```bash
nohup ./library_app > library_app.log 2>&1 &
```

## Testing
```bash
go test ./src/...
```
