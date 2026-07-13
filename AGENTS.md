# Home Library Manager — Project Guide

## Project Overview

Home library management web application built with Go and PostgreSQL.
Provides a RESTful API + OPDS catalog for managing a personal book collection.
Runs on Raspberry Pi. Android WebView wrapper available (see `src_android/`). HTTPS via nginx.

## Testing Convention

After any code change, the application MUST be rebuilt and started (see Build & Run).
The application MUST be left running after all testing is done so subsequent testing
sessions can verify changes immediately.

## Directory Structure

```
lbl/
├── bookarch/         # Book archive files (ZIP format, one per edition)
├── certres/          # SSL certificates + generation scripts
│   ├── generate-certs.sh
│   ├── generate-keystore.sh
│   ├── generate-client-cert.sh
│   └── generate-nginx-certs.sh  # (optional) copies to fullchain.pem
├── db/
│   └── scripts/      # Database scripts
├── logs/             # Application logs
├── src/              # Go source code
│   ├── main.go       # Entry point: routes, handlers, ImportManager, DB
│   ├── auth.go       # Login, register, refresh token
│   ├── admin.go      # Admin handlers (users, persons, tags)
│   ├── reading.go    # Reading progress + role middleware
│   ├── jwt.go        # JWT + refresh tokens
│   ├── opds.go       # OPDS XML catalog
│   ├── export.go     # Export/import handlers
│   ├── main_test.go  # Tests
│   ├── schema.sql    # Embedded DB schema (go:embed)
│   ├── migration_{1.1,2.0,2.1,2.2,2.3,2.4,2.5}.sql
│   ├── config/
│   │   └── config.go # TOML config struct, Load(), DefaultConfig()
│   └── utils/
│       ├── llm_client.go   # OpenAI-compatible LLM client
│       ├── pdf_extract.go  # PDF text extraction
│       ├── docx_extract.go # DOCX text extraction
│       ├── doc_extract.go  # Binary DOC text extraction (OLE2)
│       ├── epub.go         # EPUB metadata parsing
│       ├── fb2.go          # FB2 metadata parsing
│       ├── fb2_test.go     # FB2 tests
│       ├── epub_test.go    # EPUB tests
│       └── zip_extract.go  # ZIP content type detection
├── static/
│   ├── css/
│   │   ├── style.css       # Main styles (desktop + @media mobile)
│   │   └── mobile.css      # Android-only styles (body.android)
│   ├── js/
│   │   ├── app.js          # SPA: Authors, Books, Genres tabs
│   │   └── import.js       # Async import with progress polling
│   └── favicon.ico
├── templates/
│   ├── index.html    # SPA main page
│   └── admin.html    # Admin panel SPA
├── tempfld/          # Upload processing + shelf extraction directory
├── testdata/         # Sample books for testing
├── nginx.conf        # nginx HTTPS + proxy configuration
├── docker-compose.yml
├── docker-compose-nginx.yml  # Override: adds nginx service
├── Dockerfile
├── Dockerfile.all-in-one
├── Dockerfile.android
├── Dockerfile.android.sdk
├── startup.sh
├── config.toml.example
├── env.example
├── go.mod / go.sum
├── .apk.conf              # APK build configuration (URL, certs, keystore)
├── .apk.conf.example      # Template for .apk.conf
├── AGENTS.md
├── README.md
└── .gitignore
```

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.25+ |
| Web Framework | Gin-Gonic (github.com/gin-gonic/gin) |
| Database | PostgreSQL 17 |
| Driver | github.com/lib/pq |
| Config | TOML (github.com/BurntSushi/toml) |
| PDF | github.com/ledongthuc/pdf |
| DOC (OLE2) | github.com/richardlehane/mscfb |
| JWT | github.com/golang-jwt/jwt/v5 |
| Frontend | Vanilla JS (SPA), no frameworks |
| LLM | Ollama / OpenAI-compatible API (phi4:latest) |
| Reverse proxy | nginx (alpine) |
| Target | Raspberry Pi (ARM/Linux) |

## Configuration

Reads `config.toml` from `CONFIG_PATH` (env) or current directory.
See `config.toml.example` for all options.

### Config Sections

| Section | Fields | Description |
|---------|--------|-------------|
| `[server]` | `port`, `bind`, `enable_delete`, `log_level` | HTTP server settings |
| `[server]` | `jwt_secret`, `token_ttl` | JWT secret key (auto-generated if empty), token TTL in hours |
| `[directories]` | `bookarch`, `temp`, `logs`, `templates`, `static` | File system paths |
| `[database]` | `host`, `port`, `name`, `user`, `password`, `sslmode`, `pgdata` (all-in-one) | PostgreSQL connection |
| `[llm]` | `base_url`, `model`, `token`, `prompt`, `prompt2`, `timeout` | LLM endpoint settings |

### Environment Variable Overrides

| Env Var | Config Field |
|---------|-------------|
| `DATABASE_URL` / `LIBAPP_DATABASE_URL` | DSN (overrides all database.*) |
| `PORT` / `LIBAPP_PORT` | server.port |
| `LIBAPP_JWT_SECRET` | server.jwt_secret |
| `LIBAPP_TOKEN_TTL` | server.token_ttl |
| `LIBAPP_BIND` | server.bind |
| `LIBAPP_ENABLE_DELETE` | server.enable_delete |
| `LIBAPP_LOG_LEVEL` | server.log_level |
| `LIBAPP_DIR_*` | directories.* |
| `LIBAPP_DB_*` | database.* |
| `LIBAPP_LLM_*` | llm.* |

## API Endpoints

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/auth/login` | Login → JWT + refresh_token + user info |
| POST | `/api/v1/auth/register` | Register new viewer user (min password: 6 chars) |
| POST | `/api/v1/auth/refresh` | Exchange refresh_token for new JWT |

- **First login auto-creates admin**: If no users exist in DB, the first login attempt auto-creates an admin user with the provided credentials. Uses `pg_advisory_xact_lock(42)` to prevent race conditions.
- **Refresh tokens** stored as SHA-256 hash in `refresh_tokens` table.
- All authenticated endpoints require `Authorization: Bearer <token>` header.
- Guest routes: `GET /`, `GET /static/*`, `GET /favicon.ico`, `POST /api/v1/auth/login`, `POST /api/v1/auth/register`, `POST /api/v1/auth/refresh`, OPDS.

### Books & Metadata

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/books` | List books (?author=, ?book=, ?genre=, ?date_from=, ?date_to=, ?sort_by=, ?sort_order=, ?limit=, ?offset=) |
| GET | `/api/v1/books/search` | Full-text search |
| POST | `/api/v1/books` | Create book |
| GET | `/api/v1/books/:id` | Get book details |
| PUT | `/api/v1/books/:id` | Update book |
| DELETE | `/api/v1/books/:id` | Delete book + orphaned work |
| GET | `/api/v1/books/:id/extended` | Extended info (ISBN, annotation, publisher, etc.) |
| PUT | `/api/v1/books/:id/extended` | Update extended info |
| PUT | `/api/v1/books/:id/shelf` | Toggle shelf (favorites) |
| GET | `/api/v1/books/:id/download` | Download book file (?mode=extracted for raw file) |
| POST | `/api/v1/books/:id/cover` | Upload cover image (JPEG/PNG/WebP, 10MB max) |
| PUT | `/api/v1/books/:id/reading` | Update reading status (0=not started, 1=in progress, 2=finished) |
| GET | `/api/v1/user/books` | Current user's books with reading status |

### Authors, Genres, Tags

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/authors` | List authors with book tree |
| GET | `/api/v1/genres` | List genres |
| GET | `/api/v1/genres/tree` | Genre hierarchy with nested authors & books |
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
| GET | `/api/v1/import/status` | Poll import progress (running, total, completed, errors, items) |
| POST | `/api/v1/import/cancel` | Cancel running import |
| POST | `/api/v1/import/file` | Import single file (sync) |

### Read List

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/user/readlist` | Current user's read list sorted by priority desc |
| POST | `/api/v1/user/readlist` | Create read list item |
| GET | `/api/v1/user/readlist/names` | Get list names |
| PUT | `/api/v1/user/readlist/:id` | Update read list item |
| DELETE | `/api/v1/user/readlist/:id` | Delete read list item |

### Admin

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/admin/users` | List all users (admin only) |
| POST | `/api/v1/admin/users` | Create user (admin only) |
| PUT | `/api/v1/admin/users/:id` | Update user (admin only) |
| DELETE | `/api/v1/admin/users/:id` | Delete user (admin only) |
| GET | `/api/v1/admin/settings` | Get settings (admin only) |
| PUT | `/api/v1/admin/settings` | Update settings (admin only) |
| GET | `/api/v1/admin/refresh` | Refresh LLM metadata for all books (admin only) |

### Shelf

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/shelf/count` | Books on shelf count |
| PUT | `/api/v1/shelf/clear` | Clear shelf |
| GET | `/shelf/` | Shelf page (HTML) |

### Other

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/config` | App config (enable_delete flag) |
| GET | `/debug/goroutines` | Goroutine dump (admin+editor) |

### OPDS

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/opds/catalog.xml` | OPDS root catalog |
| GET | `/api/v1/opds/latest.xml` | OPDS latest books |
| GET | `/api/v1/opds/genres.xml` | OPDS genres |
| GET | `/api/v1/opds/genre/:id.xml` | OPDS books by genre |
| GET | `/api/v1/opds/search.xml?q=` | OPDS search |
| GET | `/api/v1/opds/book/:id` | OPDS download |

## Role Model

| Role | Access |
|------|--------|
| `viewer` | Browse catalog, search, OPDS, download books. No access to admin panel. |
| `editor` | Full catalog access + admin panel (Authors, Books, Genres, Tags, Import tabs). No Users tab. |
| `admin` | Full access including user management. |

- Permission checks use middleware: `adminAuthMiddleware()` (rejects viewer, allows editor+admin), `adminOnlyMiddleware()` (only admin).
- Admin panel (`/admin`) checks user role client-side; `editor` sees all tabs except Users; `viewer` gets 403.

## JWT Authentication

- Tokens are generated on login: `HS256` with configurable secret (`jwt_secret`) and TTL (`token_ttl` hours).
- If `jwt_secret` is empty, a random secret is generated on startup — tokens invalidated on restart.
- Middleware `authMiddleware()` validates JWT, sets user info in context, redirects to login on failure.
- Token is stored in `localStorage` on the frontend; also in `session_token` cookie (HttpOnly, SameSite=Strict).
- Refresh tokens (SHA-256 hashed in DB) allow getting new JWT without re-login.

## Frontend Pages (SPA)

### Login Page
- Username/password form.
- First login auto-creates admin if no users exist.

### Main Page (`/` — `index.html`)
Three tabs:

**Вкладка Авторы (Authors)**
- Hierarchical tree: author → work → edition
- Pagination (50 per page), summary (authors, works, editions)
- Filters: author name, book title, genre (ё→е normalization)
- Shelf checkboxes per edition, "Добавить на полку", "Очистить полку"
- Edit author/book modal, delete book
- Download edition button (?mode=extracted for raw file)
- Reading status: colored badges + dropdown to update

**Вкладка Книги (Books)**
- Flat table: №, upload date, title, author, format (download link), shelf toggle, edit, reading status
- Filters: author, title, genre, date range (from/to)
- Column sorting: title, upload date, author, format (click headers)
- Pagination (50 per page)
- Shelf count, clear shelf button

**Вкладка Жанры (Genres)**
- Hierarchical tree: genre → author → book (edition)
- Filters: genre name, author name, book title
- Edit genre name inline via modal
- Shelf checkboxes, reading status, download button

**Вкладка Импорт (Import)**
- Upload file form (FB2, EPUB, ZIP, PDF, DOC, DOCX)
- Import from server directory
- Async progress polling with cancel button
- Progress bar and file list

### Admin Page (`/admin` — `admin.html`)
- 5 tabs: Пользователи (admin only), Авторы, Книги, Жанры, Теги, Импорт
- User management: create, edit role, delete users
- All catalog CRUD operations
- Settings: LLM prompt config
- LLM metadata refresh for all books

## LLM Book Recognition

- Extracts first 3 pages (up to 2000 chars) from PDF/DOC/DOCX
- Sends to OpenAI-compatible LLM (Ollama/llama.cpp)
- Prompt asks for `AUTHOR:` and `BOOKNAME:` in response
- Multiple authors: comma-separated in LLM response
- All LLM calls serialized via `sync.Mutex`
- Retry with `prompt2` if first call returns empty
- Falls back to filename if LLM unavailable or timeout
- All LLM requests logged regardless of `log_level`

## Import Flow

1. Files uploaded via `/import/upload` → saved to `tempfld/`
2. Async goroutine processes each file:
   - SHA-256 hash duplicate check (blocks before LLM call)
   - Format detection (FB2/EPUB have native metadata, PDF/DOC/DOCX need LLM)
   - LLM recognition (if needed)
   - Save as single-format file in `bookarch/XXXXX/` (NOT double-ZIP)
   - Insert into DB (works → editions → edition_files, with ISBN + genres)
3. Progress polled via `/import/status`
4. Cancel via `/import/cancel`
5. Temp directories cleaned up on completion

## Supported Formats

| Format | Metadata Source | Recognition |
|--------|----------------|-------------|
| FB2 | Native XML (title, author, genres, annotation, cover, ISBN, publisher, language) | XML parser (CP1251, KOI8-R) |
| EPUB | Native XML (container.xml + OPF: title, author, ISBN, publisher, language, genres) | OPF parser |
| PDF | LLM from extracted text (first 3 pages, ~2000 chars) | github.com/ledongthuc/pdf |
| DOCX | LLM from extracted text (word/document.xml) | ZIP + XML |
| DOC | LLM from extracted text (OLE2 + UTF-16LE) | github.com/richardlehane/mscfb |
| ZIP | Auto-detect content type inside | FB2, EPUB, PDF, DOC, DOCX |

Nested archives: FB2.ZIP inside ZIP is recursively unzipped until a non-archive format is reached. DOCX is treated as final format (not double-unzipped).

## Search & Filtering

- `normalizeQuery()` → lowercase + ё→е for all search strings
- Indexes: `persons.lower_fio` (GIN trgm), `works.lower_original_title` (GIN trgm), `editions.lower_title` (GIN trgm)
- Trigger: `normalize_search_field()` populates lower_ fields via `REPLACE(LOWER(...), 'ё', 'е')`
- All search queries use indexed lower_ fields
- Sort by year: `NULLIF(year, 0)` + `NULLS LAST` — books without year (including year=0) always at end

## Build & Run

App listens on `0.0.0.0:9091` by default.

### Prerequisites

- PostgreSQL 17+ running and accessible
- `config.toml` in project root or `CONFIG_PATH` env var

### Quick Start

```bash
go build -o library_app ./src/
./library_app
```

### Environment Overrides

```bash
DATABASE_URL="host=... port=... user=... password=... dbname=... sslmode=disable" PORT=9091 ./library_app
```

### Docker Compose (with nginx HTTPS)

```bash
# 1. Generate certs
cd certres && ./generate-certs.sh && cd ..

# 2. Start
docker compose -f docker-compose.yml -f docker-compose-nginx.yml up -d --build
```

## nginx Configuration

`nginx.conf` provides:
- HTTP (80) → HTTPS (443) redirect
- HTTPS with SSL using `server.crt` / `server.key` from `certres/`
- `proxy_pass` to `app:8080` (Docker Compose) or `127.0.0.1:9091` (standalone)
- Security headers: HSTS, X-Content-Type-Options, X-Frame-Options, Referrer-Policy
- Forwards `X-Platform` header for Android mobile detection

## Security Considerations (for AI assistants)

When making changes, be aware of:
1. **Session cookie Secure flag**: hardcoded to `false` in `auth.go:41`. If adding HTTPS support, set dynamically based on request scheme.
2. **Dockerfile runs as root**: consider adding `USER nobody` for production.
3. **TLS config**: `http.ListenAndServeTLS` in `main.go:752` uses Go defaults. Add `tls.Config` with `MinVersion: tls.VersionTLS12` for better security.
4. **CSP has `'unsafe-inline'`**: acceptable for SPA with vanilla JS, but consider nonce/hash-based CSP for stricter XSS protection.
5. **File upload validation**: extension-only for import, Content-Type header for covers. Add magic-bytes check for production.
6. **JWT secret**: if empty in config, auto-generated on startup → all tokens invalid on restart. Always remind user to set `jwt_secret`.
7. **OPDS host header**: base URL from `Host` header is used directly in XML output. Escape/validate for production.
8. **Rate limiting**: in-memory only (resets on restart). For sticky production, consider Redis-backed limiter or nginx-level rate limiting.
9. **Input length limits**: only password min-length (6) is enforced. Add max-length validation for user-facing string fields.

## Launch for Other Agents (Docker host-net mode)

Create minimal Docker image from binary and run with `--net=host`:

```bash
# Prerequisites:
# - PostgreSQL must be accessible (on host or network)
# - Binary must be built: go build -o library_app ./src/
# - Docker socket available (/var/run/docker.sock)

cd /home/sergey/git/aitest/agents/lbl

# Kill old instances
docker rm -f library-app 2>/dev/null
pkill -f library_app 2>/dev/null; sleep 1

# Create minimal image
cd /tmp && mkdir -p library-app
cp /home/sergey/git/aitest/agents/lbl/library_app library-app/
cp -r /home/sergey/git/aitest/agents/lbl/templates \
      /home/sergey/git/aitest/agents/lbl/static \
      /home/sergey/git/aitest/agents/lbl/config.toml library-app/
tar -cf library-app.tar -C library-app .
docker import library-app.tar library-app:latest
rm -rf /tmp/library-app /tmp/library-app.tar

# Run
docker run -d --name library-app --net=host \
  -v /home/sergey/git/aitest/agents/lbl/config.toml:/config.toml \
  -v /home/sergey/git/aitest/agents/lbl/bookarch:/bookarch \
  -v /home/sergey/git/aitest/agents/lbl/tempfld:/tempfld \
  -v /home/sergey/git/aitest/agents/lbl/logs:/logs \
  -v /home/sergey/git/aitest/agents/lbl/templates:/templates \
  -v /home/sergey/git/aitest/agents/lbl/static:/static \
  library-app /library_app

# Verify
sleep 2
curl -s -o /dev/null -w "%{http_code}" http://localhost:9091/
# Should return 200
```

### Verification via nginx (if running)

```bash
curl -sk -o /dev/null -w "%{http_code}" https://localhost/
curl -sk https://localhost/api/v1/config
curl -sk -H "X-Platform: android" https://localhost/ | grep -o 'class="android"'
```

### Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `bind: address already in use` | Old process still listening | `docker rm -f library-app 2>/dev/null; pkill -f library_app 2>/dev/null` |
| `connection refused` | Container not started or port mismatch | Check `docker logs library-app`; verify config.toml port is 9091 |
| PostgreSQL errors | DB not running | Check `pg_isready`; start cluster |
| Docker Hub pull fails (403) | No registry access | Use `docker import` (no registry required) |

### Stop

```bash
docker rm -f library-app
```

### Rebuild

```bash
# After code changes:
go build -o library_app ./src/
# Then repeat launch sequence
```

## Testing

After code changes, run all tests:

```bash
go test -count=1 ./src/
```

Tests require PostgreSQL (`DATABASE_URL` env var or config.toml).

## Database

- Schema auto-created on first run (embedded `schema.sql`)
- Migrations applied automatically (embedded `migration_*.sql`)
- Database auto-created if not exists
- Key tables: `users`, `persons`, `works`, `editions`, `edition_files`, `genres`, `edition_genres`, `tags`, `edition_tags`, `shelf`, `reading_status`, `settings`, `user_devices`, `user_books`, `read_list`, `refresh_tokens`
- Indices for search: GIN trgm on `lower_fio`, `lower_original_title`, `lower_title`
- DB version tracked in `db_version` table; current version: `2.5`

## LLM Config

Example `config.toml` for Ollama:

```toml
[llm]
base_url = "http://192.168.95.200:11434"
model = "phi4:latest"
token = ""
prompt = "..."  # System prompt for title/author recognition
prompt2 = "..." # Retry prompt if first attempt returns empty
timeout = 60    # Seconds
```

## Goal
- Maintain a self-hosted Home Library Manager with Go backend, Android WebView wrapper, and a single responsive UI that works on desktop and mobile without horizontal scrolling.

## Constraints & Preferences
- Mobile layout: use `body.android` class injected server-side, `mobile.css`, mobile-top-bar, compact edit/delete buttons, and Android JS for user button.
- Horizontal scrolling on lists (books table, reading list, admin tables, tree views) is unacceptable – all content must fit on phone screen width.
- Default to classic colors; no dark theme.
- Docker build split into SDK image + app build.
- When shelving a book: extract the ZIP to `tempfld/shelf/{edition_id}/`, unzip nested archives recursively, serve the extracted file on download. Clean up on unshelf.
- When importing ZIPs: extract to inner format (FB2, PDF, DOC, DOCX), NOT store a double-ZIP. Recursively unzip nested archives (e.g., FB2.ZIP inside ZIP) until a non-archive format is reached. DOCX is a final format (even though it is technically a ZIP).

## Progress
### Done
- Restored Android-specific server injection: `body class="android"`, `mobile.css`, mobile-top-bar HTML, Android JS for user button.
- Fixed `mobile.css` for phone-screen fit: hidden non-essential columns, `table-layout: fixed`, compact sizes, icon-only buttons.
- Shelf extraction: shelving a book extracts its archive to `tempfld/shelf/{edition_id}/` and serves the raw file on download.
- Fixed import ZIP bug: double-ZIPs no longer created; inner format stored directly.
- Fixed `DetectZipContent` in `zip_extract.go`: `.fb2` entries that are actually ZIP archives are now recursively detected.
- Full import of `/example/` complete: 69 books imported, 2 `.rar` unsupported.
- Fixed JS syntax error in `app.js` (extra `}` in the editAuthor function).
- Fixed readlist desktop table layout: added explicit column widths (`col-comment`, `col-listname`, `col-library`, `col-status`).
- Fixed `mobile.css` duplicates (removed duplicate `.admin-container` rule).
- Fixed download for Android WebView: replaced `window.open` with `DownloadListener` using direct HTTPS connection.
- Fixed download mode: added `?mode=extracted` parameter to serve raw extracted file (not ZIP) for reading apps.
- Changed default readlist sort to priority desc.
- Changed default book priority to max+1.
- Created nginx HTTPS proxy: `nginx.conf`, `docker-compose-nginx.yml`, `certres/generate-nginx-certs.sh`.
- Added `.gitignore` for certificate files and build artifacts.
- CSP fix: added `'unsafe-inline'` to `style-src` and `script-src` — inline styles/event handlers were being blocked
- Added `TestCSPHeader` and `TestUpdateBookExtendedAddAuthor` tests for CSP regression coverage
- Removed TWA-specific files and references (assetlinks, twa-manifest, icons); changed "TWA" → "WebView wrapper"
- Restored APK build files after re-request; switched from `openjdk:17-jdk-slim` to `eclipse-temurin:17-jdk-jammy` (old image removed from Docker Hub)
- Fixed APK Docker build: removed redundant Gradle tasks (`copyCertificates`/`generateClientCert`) that failed in Docker; certs/keystore now copied by `build-android.sh`
- Switched `Dockerfile.android` to use system `gradle` command instead of non-existent `./gradlew`
- Fixed APK Docker build issues:
  - `GRADLE_OPTS` with TLSv1.2 fix for Google Maven repo TLS handshake failures
  - Extended Gradle HTTP timeouts to prevent `Read timed out` errors on slow connections
  - Skipped lint tasks (`-x lint*`) to prevent timeout during `lintVitalAnalyzeRelease`
  - Fixed missing `ca_cert.crt`/`client_cert.p12` resources by ensuring `build-android.sh` copies certs pre-build
- Extracted APK config to `.apk.conf` (shell-sourceable):
  - `APK_TARGET_URL`, cert paths, keystore settings, app identity, SDK versions
  - `build-android.sh` generates `Config.java`, `build-extras.gradle`, copies certs from config paths
  - `MainActivity.java` reads URL and cert password from `Config.java`
  - `build.gradle` reads app identity and keystore from `build-extras.gradle`
  - All generated files cleaned up after build; `.apk.conf` in `.gitignore`, `.apk.conf.example` provided

### In Progress
- (none)

### Blocked
- `.rar` files in example (10_the_active_side_of_infinity.rar, 11_the_wheel_of_time.rar) are not supported – no RAR extraction; user has not requested this.

## Key Decisions
- Server differentiates desktop vs Android (via `X-Platform: android` header) and injects mobile-specific CSS/JS.
- Shelf extraction writes the final extracted format to `tempfld/shelf/{edition_id}/`; the original ZIP remains untouched in `bookarch/`.
- Double-ZIP bug fix ensures the stored archive in `bookarch/` contains the actual book file instead of a nested ZIP.
- nginx is the recommended HTTPS reverse proxy for production; the Docker override (`docker-compose-nginx.yml`) adds it without modifying the base compose file.
- Session cookie Secure flag is hardcoded to `false` — acceptable for local/RPi deployment, but should be made dynamic for production.

## Next Steps
- Add graceful shutdown (SIGTERM/SIGINT handler) to avoid connection drops on restart.
- Add `db.SetMaxOpenConns` / `db.SetMaxIdleConns` configuration for connection pool tuning.
- Add health-check endpoint (`/health`).
- Make session cookie Secure flag dynamic based on request scheme.
- Harden Dockerfile: add non-root user, pin `alpine` digest.
- Harden TLS: add `tls.Config` with `MinVersion: tls.VersionTLS12`.
- Add magic-bytes validation for file uploads.
