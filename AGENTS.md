# Home Library Manager — Project Guide

## Project Overview

Home library management web application built with Go and PostgreSQL.
Provides a RESTful API + OPDS catalog for managing a personal book collection.
Runs on Raspberry Pi. Android WebView wrapper available (see `src_android/`). HTTPS via nginx.

## Testing Convention

After any code change, the application MUST be rebuilt and started (see Build & Run).
The application MUST be left running after all testing is done so subsequent testing
sessions can verify changes immediately.

## Skills

- **update-icons** (`.opencode/skills/update-icons/SKILL.md`): Regenerates the site
  favicon (`static/favicon.ico`, `static/favicon.svg`) and the Android APK launcher
  icons (legacy `mipmap-*/ic_launcher.png` + adaptive `mipmap-*/ic_launcher_fg.png`,
  drawables) from a single source image/SVG, syncs APK web assets, and verifies on
  the live server. **MUST be loaded whenever the user asks to update/replace the
  project icons** (e.g. "обнови иконки", "сделай favicon", "замени иконку"). It
  covers the critical pitfall: embedded BMP data-URIs often have alpha=0 on every
  pixel and silently render invisible via `rsvg-convert` — always verify layers
  before building icons.

## Directory Structure

```
lbl/
├── bookarch/         # Book archive files (ZIP format, one per edition)
├── backup/           # DB backups (created before migrations, kept on failure)
├── certres/          # SSL certificates + generation scripts
│   ├── generate-certs.sh
│   ├── generate-keystore.sh
│   ├── generate-client-cert.sh
│   └── generate-nginx-certs.sh  # (optional) copies to fullchain.pem
├── db/
│   └── scripts/      # Database scripts
├── logs/             # Application logs
├── .opencode/
│   └── skills/update-icons/SKILL.md  # Icon regeneration skill (see Skills)
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
├── nginx.conf        # nginx HTTPS + proxy configuration (docker-compose: app:8080)
├── nginx-standalone.conf  # nginx HTTPS for standalone host-net mode (127.0.0.1:9091)
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

### Config Sections

| Section | Fields | Description |
|---------|--------|-------------|
| `[server]` | `port`, `bind`, `enable_delete`, `log_level` | HTTP server settings |
| `[server]` | `jwt_secret`, `token_ttl` | JWT secret key (auto-generated if empty), token TTL in hours |
| `[directories]` | `bookarch`, `temp`, `logs`, `templates`, `static`, `backup` | File system paths |
| `[database]` | `host`, `port`, `name`, `user`, `password`, `sslmode`, `pgdata` (all-in-one) | PostgreSQL connection |
| `[llm]` | `base_url`, `model`, `token`, `prompt`, `prompt2`, `timeout` | LLM endpoint settings |

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
- Settings: backup_dir path (read from DB, synced from config)
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
pkill -x library_app 2>/dev/null; sleep 1

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

### Standalone nginx (host-net mode)

`nginx.conf` proxies to `app:8080` (docker-compose service name) and does NOT work for
the standalone host-net deployment where the app listens on `127.0.0.1:9091`.
Use `nginx-standalone.conf` instead — it proxies to `127.0.0.1:9091` and forwards
`X-Platform`. Start nginx in host-net mode (no port publishing):

```bash
docker rm -f library-nginx 2>/dev/null
docker run -d --name library-nginx --net=host --restart unless-stopped \
  -v /home/sergey/git/aitest/agents/lbl/nginx-standalone.conf:/etc/nginx/nginx.conf:ro \
  -v /home/sergey/git/aitest/agents/lbl/certres/server.crt:/etc/nginx/certs/server.crt:ro \
  -v /home/sergey/git/aitest/agents/lbl/certres/server.key:/etc/nginx/certs/server.key:ro \
  nginx:alpine
```

Symptom that the compose-based config is mounted: nginx exits with
`host not found in upstream "app"` in `docker logs library-nginx`.

### Federation setup (2nd instance + dual-site nginx)

A second app instance (`library-app2`) runs in host-net mode on **9092**
against database **library2** in the same PostgreSQL cluster. It is launched
from the same `library-app:latest` image with `config2.toml` mounted as
`/config.toml` and its own `bookarch2/tempfld2/logs2/backup2/apk2` dirs
mounted at the matching container paths. `config2.toml` must set
`port = 9092`, `name = "library2"` and a distinct `jwt_secret`.

`nginx-federation.conf` (mounted on `library-nginx`, host-net) serves two
sites with two self-signed certs from `certres/site_a/` and `certres/site_b/`:

| Port | Backend | Cert |
|------|---------|------|
| 443  | app1 `127.0.0.1:9091` | site_a |
| 444  | app1 `127.0.0.1:9091` | site_a |
| 445  | app2 `127.0.0.1:9092` | site_b |

**Federated search**: admin presses «Поиск по федерации» on the «Запросы»
tab of `/admin`. `POST /api/v1/admin/federation/search` (admin only) walks
every `api_neighbours` row, decrypts the stored password (`NeighbourCrypto`),
logs in on the neighbour (JWT, role `server`), and forwards the search to
`POST /api/v1/server/search`. The neighbour's `server_cert` is added to the
TLS trust pool (self-signed) and `client_cert` (combined cert+key PEM) is
used for mutual TLS. Response: `{neighbours, results:[{neighbour_id, url,
error?, total, books:[{work_id, edition_id, author, title, year, formats}]}]}`.
With `?stop_on_first=1` neighbours are queried sequentially and the search
stops at the first neighbour that returns at least one book (used by the UI
form). An unavailable/erroring neighbour is reported as `error` in its result
entry and the search **continues** with the remaining neighbours (tested by
`TestFederationSearchContinuesAfterError`). The form («Поиск по федерации»
modal) runs the search only when the «Искать» button is pressed (not on Enter),
uses a wide modal that does not close on an outside click, places the «Искать»
button below the author/title/limit fields, and renders a flat table
`УРЛ сервера | Книга | Автор | [Загрузить]`.

**Federated download + import**: each result row's «Загрузить» button calls
`POST /api/v1/admin/federation/import` (admin only) with `{neighbour_id,
edition_id}`. The handler logs in on the neighbour (same TLS trust), downloads
`GET /api/v1/server/download/:id` (server-role; reuses `downloadBook`, serves
the stored single-format archive as `.zip`), and feeds the bytes into the
standard `importFile` pipeline (content-hash duplicate detection, FB2/EPUB
metadata or LLM recognition). Response mirrors `/api/v1/import/file`
(`{duplicate?, message, work_id, edition_id, file_path, title, authors,
parsed?}`).

To federate two instances: register/login a `server`-role account on each,
then add a neighbour on each pointing at the other's HTTPS endpoint with that
account's credentials and the other site's self-signed cert as `server_cert`
(see `src/federation.go`).

### Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `bind: address already in use` | Old process still listening | `docker rm -f library-app 2>/dev/null; pkill -x library_app 2>/dev/null` |
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
- UUID migration for readlist (migration 3.0):
  - `read_list.id` SERIAL → UUID PK
  - `updated_at` / `synced_at` TIMESTAMP columns
  - All Go CRUD handlers updated for UUID + timestamps
  - All tests migrated to UUID
- Android offline SQLite bridge (`src_android/…/ReadListDB.java`):
  - `readlist_items` table mirroring PostgreSQL schema
  - `offline_queue` table for pending mutations
  - CRUD methods: `replaceAll`, `queryAll`, `upsertItem`, `deleteItem`, `clearAll`
  - Queue management: `enqueue`, `enqueueDelete`, `getPendingQueue`, `getPendingCount`, `clearQueue`, `dequeue`
- `@JavascriptInterface` bridge (`MainActivity.java`):
  - `AndroidReadListDB` object exposing all ReadListDB methods to JS
- JS offline layer (`static/js/offline.js`):
  - `ReadListStore` — in-memory cache backed by Android SQLite
  - `OfflineQueue` — mutations queued locally, stored in SQLite
  - `SyncService` — push pending mutations → pull full server state
- Frontend offline support (`static/js/app.js`, `templates/index.html`):
  - `loadReadlist()` falls back to `ReadListStore.query()` when offline
  - `loadReadlistNames()` falls back to `ReadListStore` cache
  - `openEditReadlistModal()` uses `ReadListStore.getById()` when offline
  - Create/edit form submits via `ReadListStore.upsert()` + `OfflineQueue.enqueue()` when offline
  - Delete via `ReadListStore.remove()` + `OfflineQueue.enqueueDelete()`
  - `setReadlistItemStatus()` updates locally when offline
  - Online/offline detection with status indicators
  - Sync button + pending count badge in readlist filters
  - Auto-sync on reconnect or app start
- Полная переработка офлайн-синхронизации:
  - Локальная SQLite = источник истины для отображения
  - Фоновая асинхронная синхронизация Push (dirty по updated_at > synced_at) + Pull
  - User-scoping: синхронизация только для текущего пользователя, очистка чужих данных при смене УЗ
  - Индикация: #mobileUserBtn (зелёный/жёлтый/красный) + счётчик dirty + кнопка синхронизации
  - `sync_task_spec.md` — полная спецификация алгоритма
  - `auth-changed` event для повторного init при логине/логауте
- **Серверный конфликт-детектор + тест** (`src/reading.go`):
  - Добавлено поле `UpdatedAt` в `CreateReadListRequest`
  - `updateReadListItem` проверяет: если `server.updated_at > client.updated_at` → 409 с `server_item` в теле
  - `isServerNewer()` — парсит оба timestamps как `time.Time` с поддержкой PG и RFC3339 форматов
  - `offline.js`: при 409 применяет серверное состояние через `applyServerItem`
  - `TestReadListSyncConflictServerNewer` — полный цикл: create → server-update → stale push → 409 → GET подтверждает серверную версию
- **Статика встроена в APK** (`MainActivity.java`, `build-android.sh`, `auth.js`):
  - `build-android.sh`: копирует `static/css/*.css`, `static/js/*.js`, `templates/*.html`, `service-worker.js`, `favicon.*` в `assets/www/` перед сборкой
  - `shouldInterceptRequest()` перехватывает запросы к `/`, `/admin`, `/static/*`, `/service-worker.js`, `/favicon.*` и отдаёт из APK-ассетов (без сети)
  - Мобильные инъекции (CSS-линк, mobile-top-bar, Android JS) применяются на лету в Java — полный аналог серверной обработки
  - При логине `auth.js` вызывает `AndroidTokenBridge.setForceNetworkRefresh(true)` и перезагружает страницу — все файлы загружаются свежими с сервера
  - После загрузки флаг сбрасывается, последующие страницы снова идут из ассетов

### In Progress
- *(none)*

### Done (latest)
- **Кнопка «Тест» на вкладке «Серверы» (`/administer`) + API проверки подключения**:
  - Бэкенд `src/federation.go`: `adminFederationTest` (`POST /api/v1/admin/federation/test`, admin only, `{neighbour_id}`) — та же роль, что у импорта книг из федерации (`adminOnlyMiddleware`). Загружает запись `api_neighbours`, логинится на соседе сохранёнными логином/паролем через `loginToNeighbour` (mTLS-клиент, расшифровка `NeighbourCrypto`) и шлёт тестовое сообщение `GET /api/v1/server/ping`. Успех → 200 `{ok:true}`. Любая ошибка (дешифровка, подключение, не-200 от соседа) логируется в лог сервера (`[FEDERATION TEST] ... FAILED: ...`) и возвращается как **HTTP 502** с `{ok:false, error}`; несуществующий сосед → 404; маршрут в `src/main.go` рядом с `federation/search`/`federation/import`.
  - Фронтенд `static/js/admin.js` + `static/css/admin.css` + `templates/admin_users.html`: в строке каждого сервера на вкладке «Серверы» добавлена кнопка «Тест» (`test-neighbour`, делегирование кликов); `testNeighbour(id, btn)` асинхронно вызывает endpoint и меняет состояние кнопки: успех → зелёная (`.test-ok`) с текстом «Тест: Ок», ошибка → красная (`.test-fail`) с текстом «Тест: fail».
  - Тест `TestFederationTest` (`src/main_test.go`): mock-сосед с новым `/api/v1/server/ping`-хендлером (счётчик `pingHits` в `fedMock`); успех (200, ping ровно 1 раз), недоступный сосед `http://127.0.0.1:1` → 502 с `ok:false`, несуществующий сосед → 404, editor → 403.
  - Live E2E: app1 (9091) против живого app2 (id=13, `https://127.0.0.1:445`) → `{"ok":true}` HTTP 200; мёртвый сосед (невалидный зашифрованный пароль) → HTTP 502 + строка `[FEDERATION TEST] ... FAILED: Не удалось расшифровать пароль...` в `docker logs library-app`; временный сосед удалён, БД чистая.
  - Проверено: `go test -count=1 ./src/` зелёный, `node --check static/js/admin.js` OK, APK-ассеты (`admin_users.html`, `admin.js`, `admin.css`) синхронизированы, бинарь/образ пересобраны, `library-app` (9091) перезапущен, 9091/9092/443/444/445 → 200, контейнеры оставлены запущенными.
- **Импорт книги из федерации с сохранением удалённых ID + конфликт-модалка «Перезаписать / Создать новую / Отменить»**:
  - Бэкенд `src/server_api.go`: новый `GET /api/v1/server/metadata/:id` (роль server) отдаёт автор/произведение/издание/файлы с удалёнными ID (`fedBookMetadata`); маршрут в `src/main.go` (serverGrp). Исправлен баг «converting NULL to string is unsupported» — все nullable-колонки editions/works обёрнуты в `COALESCE` (метаданные изданий с NULL ISBN/языком/издательством больше не дают 500). Новый тест `TestServerMetadataAPINullFields` (издание со всеми NULL-полями + файл с NULL size/hash, 404 на отсутствующее).
  - Бэкенд `src/federation.go`: `adminFederationImport` (`POST /api/v1/admin/federation/import`, admin only, `{neighbour_id, edition_id, mode}`) полностью переписан — не `importFile`, а собственный конвейер: fetch metadata → download zip (`/server/download/:id`) → `fedAnalyzeBook` (DetectZipContent → формат/формат ID/внутренний sha256) → `findDuplicateByHash` → `analyzeFedConflicts` → `fedCreateLocal`/`fedOverwriteLocal`. Режимы: `""` (импорт, при конфликте → **HTTP 409** с `conflict/remote/conflicts/found`), `overwrite` (замена строк с удалёнными ID), `create_new` (свежие ID, переиспользует работу по заголовку+авторам и автора через нечёткий поиск). Хелперы: `fedWriteArchive` (однофайловый zip с внутренним расширением), `fedRelPath`, `fedSyncSequence`, `fedIsbn`/`fedLanguageCode` (проверка уникальности), `fedInsertContributors`/`fedInsertGenres`.
  - Исправлен UNIQUE(`file_hash`) коллапс: overwrite/create_new книги, чьё содержимое уже есть локально (напр. create_new затем overwrite того же издания), падал с 23505. Новый `fedStorableHash(tx, hash, editionID)` — если хэш уже принадлежит другому изданию, копия сохраняется с NULL-хэшем.
  - Исправлена семантика overwrite для авторов: `fedInsertContributors` в режиме !newIDs теперь сначала ищет персону, занимающую удалённый ID, и **заменяет её данные** (иначе конфликт автора оставался неразрешённым после overwrite); только потом fuzzy-поиск, потом INSERT с удалённым ID.
  - Фронтенд: `templates/admin.html` модалка `#fedConflictModal` (таблица remote/work/edition ID, блок `found`, кнопки «Перезаписать»/«Создать новую»/«Отменить»), `static/js/admin.js` — `federationImport(…, mode)`: при 409+conflict показывает модалку, `fedConflictResolve(mode)` повторяет импорт; статусы `overwritten/created_new/created/дубликат/ошибка`. CSS `.fed-conflict-*`.
  - Тесты `src/main_test.go`: `TestFederationImport` обновлён (удалённые ID author/work/edition сохраняются, дубликат, невалидный сосед, editor→403); `TestFederationImportConflict` — 409 без изменений, create_new (новые ID, fuzzy-автор, старые строки целы), overwrite при занятом хэше (NULL-hash) + **замена персоны с удалённым ID**; `newFedMockNeighbourMeta` для метаданных с явными ID.
  - Live E2E (nginx 444→app1, 445→app2): metadata по HTTPS работает (был 500); импорт «Понедельник…» → `duplicate:true`; конфликтная книга (удалённые ID 900001 на обоих серверах) → 409 с `found`, create_new → новые work/edition + архив, overwrite → замена строк и персоны + NULL-hash. Тестовые данные и файлы вычищены с обеих БД, последовательности восстановлены.
  - **Инфраструктура**: найден и исправлен баг развёртывания app2 — `config2.toml` использует host-имена каталогов (`bookarch2/tempfld2/logs2`), а контейнер монтировал их в `/bookarch:/tempfld:/logs`, поэтому `/server/download` на app2 всегда отдавал 404 «File not found on disk». Монтирования переведены на `/bookarch2:/tempfld2:/logs2:/backup2` (совпадают с именами из конфига). Проверено: `go test -count=1 ./src/` зелёный, 9091/9092/443/444/445 → 200, контейнеры и nginx оставлены запущенными.
- **Кнопка «Искать» в форме федерации перенесена под поля «Автор»/«Название»/«Максимум результатов» + подтверждено продолжение поиска при недоступном соседе**:
  - Фронтенд `static/js/admin.js` (`openFederationSearchModal`): блок с кнопкой «Искать» перемещён из-под строки «Название или автор» в самый низ формы — под `.fed-extra` (поля «Автор (уточнение)», «Название (уточнение)», «Максимум результатов с сервера»), перед `#fedResults`. Порядок в DOM проверен jsdom'ом (fedQuery → fedAuthor → fedTitle → fedLimit → fedSearchBtn → fedResults).
  - Поведение «недоступный сосед не прерывает поиск» уже было реализовано в `adminFederationSearch` (`src/federation.go`): при `stop_on_first=1` соседи обходятся последовательно, ошибка/недоступность соседа записывается в `result.error` и поиск продолжается, остановка — только при первом соседе с `len(books)>0` (в параллельном режиме ошибки обрабатываются по каждому соседу отдельно).
  - Новый тест `TestFederationSearchContinuesAfterError` (`src/main_test.go`): первый сосед недоступен (`http://127.0.0.1:1`, connection refused, URL сортируется первым), второй — рабочий мок; проверено: `results` содержит 2 записи (ошибка + книга), рабочий сосед опрошен ровно 1 раз, поиск остановился после находки.
  - Live E2E: в БД library временно добавлен мёртвый сосед `http://127.0.0.1:1` → `stop_on_first=1` поиск «Понедельник» вернул `error: ...connection refused` для мёртвого и книгу «Понедельник начинается в субботу» с app2 (445); мёртвый сосед удалён, БД в исходном состоянии. `go test -count=1 ./src/` зелёный, `node --check` OK, APK-ассеты (`admin.js`) синхронизированы, бинарь/образ пересобраны, `library-app` (9091) и `library-app2` (9092) перезапущены (200), nginx 444/445 → 401, контейнеры оставлены запущенными.
- **Вернута кнопка «Закрыть» в форму «Поиск по федерации» + устранена ошибка при «Искать»**:
  - Кнопка «Закрыть»: в `openFederationSearchModal` (`static/js/admin.js`) футер модалки больше не скрывается целиком — скрывается только submit-кнопка, кнопка «Отмена» переименовывается в «Закрыть» и остаётся видимой. `openAdminModal` теперь всегда восстанавливает футер в исходное состояние (submit виден, текст «Сохранить»), чтобы последующие модалки не наследовали скрытый футер.
  - Enter в поле запроса больше не перезагружает страницу: `adminForm.onsubmit` в режиме федерации установлен в `function(e){ e.preventDefault(); }` (раньше скрытый submit-кнопка была «default button» формы — Enter давал полную перезагрузку GET-запросом).
  - Ошибка при «Искать» — красный блок `.fed-error` `http://192.168.95.200:9091/ — Не удалось подключиться... EOF` из-за нерабочего тестового соседа (id=3, username `test`, хост блокируется шлюзом с политикой deny → POST login даёт EOF). Сосед удалён из БД library; поиск теперь возвращает только таблицу результатов без блока ошибок.
  - Диагностика: эндпоинт `/api/v1/admin/federation/search?limit=…&stop_on_first=1` проверен curl'ом (2.5s); полный клиентский путь воспроизведён в jsdom (`templates/admin.html` + реальный `static/js/admin.js` + Node fetch с browser-style URL-резолвом против живого сервера 9091) — результаты рендерятся корректно, ошибок JS нет; `node --check static/js/admin.js` OK, `go test -count=1 ./src/` зелёный.
  - Удалён мусорный файл `src_android/app/src/main/assets/www/static/css/admin.js` (старая копия admin.js, ошибочно скопированная в каталог css; ничем не референсится). APK-ассеты (`admin.js`, `admin.css`, `admin.html`) синхронизированы. Бинарь пересобран, образ `library-app:latest` пересоздан, `library-app` (9091) и `library-app2` (9092) перезапущены (200), nginx 444/445 → 401, оба контейнера и nginx оставлены запущенными.
- **Доработка формы «Поиск по федерации» + скачивание/импорт книги с соседа**:
  - Бэкенд `src/federation.go`: `adminFederationImport` (`POST /api/v1/admin/federation/import`, admin only, `{neighbour_id, edition_id}`) — логинится на соседе (server-роль, тот же TLS trust pool через `loginToNeighbour`, вынесен из `queryNeighbour`), скачивает `GET /api/v1/server/download/:id`, байты отдаёт в штатный `importFile` (дубликат по content-hash, метаданные FB2/EPUB или LLM). Ответ как у `/api/v1/import/file` (`{duplicate?, message, work_id, edition_id, file_path, title, authors, parsed?}`).
  - Бэкенд: `serverGrp.GET("/download/:id", downloadBook(db))` (`src/main.go`) — переиспользует `downloadBook` и отдаёт соседу хранимый архив как `.zip` под server-ролью.
  - Бэкенд: `stop_on_first=1` в `adminFederationSearch` — последовательный обход соседей с остановкой на первом, вернувшем книги; параллельный режим сохранён.
  - Фронтенд `static/js/admin.js` + `static/css/admin.css`: модалка стала широкой (`rl-modal-wide`) и не закрывается кликом мимо (`rl-modal-locked`), кнопка «Искать» под строкой запроса (поиск только по клику, не по Enter, футер скрыт), результаты — плоская таблица `УРЛ сервера | Книга | Автор | [Загрузить]` (`renderFederationResults`/`federationImport`), статусы импорта (ок/дубликат/ошибка) под кнопкой.
  - Тесты `src/main_test.go`: `TestFederationImport` (мок-сосед с login+download, импорт FB2 из ZIP, повторный импорт → duplicate, 404 по неверному соседу, editor → 403, очистка), `TestFederationSearchStopOnFirst` (2 мока, запрос только первого сервера, `searchHits==1`); хелперы `newFedMockNeighbour`/`makeFB2Zip`/`backupNeighbours`/`cleanupImportedBook`.
  - E2E на живых инстансах: app1 (9091) `stop_on_first=1` по «Понедельник» → ошибка недоступного соседа id=3 + находка на app2 (445); импорт «Понедельник» из app2 в app1 → `duplicate:true`; импорт «Беседы с учениками» (Гурджиев, FB2, edition 850) из app1 в app2 → создан `bookarch2/00001/Besedy_s_uchenikami.zip` (work 4), повторный импорт → duplicate; тестовые данные вычищены (книга, персона, файл). `go test -count=1 ./src/` зелёный, `node --check` OK, APK-ассеты (`static/js/admin.js`, `static/css/admin.css`) синхронизированы, оба контейнера и nginx перезапущены и оставлены запущенными.
- **Кнопка «Поиск по федерации» + инфраструктура из 2 инстансов**: 
  - Бэкенд `src/federation.go`: `POST /api/v1/admin/federation/search` (admin only) — перебирает `api_neighbours`, расшифровывает пароль (`NeighbourCrypto`), логинится на соседе (JWT, роль `server`), шлёт `POST /api/v1/server/search`. TLS: `server_cert` соседа добавляется в trust pool (самоподписанный), `client_cert` (combined PEM cert+key) — для mutual TLS. Ответ `{neighbours, results:[{neighbour_id, url, error?, total, books:[{work_id, edition_id, author, title, year, formats}]}]}`, параллельность 3, таймаут 30с, `books` всегда массив.
  - Фронтенд: кнопка «Поиск по федерации» на вкладке «Запросы» `/admin` (`templates/admin.html`), модалка с полями query/author/title/limit и рендер результатов по соседям (`static/js/admin.js`: `setupFederationSearch`/`openFederationSearchModal`/`renderFederationResults`), CSS `.fed-block`/`.fed-header`/`.fed-error` в `admin.css`. Для роли editor кнопка скрыта (admin only).
  - Тест `TestFederationSearch` в `main_test.go`: мок-сосед (httptest) с login+search, проверка пустой/заполненной таблицы соседей, editor → 403; соседи из живой БД бэкапятся/восстанавливаются.
  - Инфраструктура: вторая БД `library2` в том же кластере (bootstraп на хосте бинарником с `config2.toml`, где есть pg_dump для бэкапов миграций), контейнер `library-app2` на host-net порту 9092 (тот же образ `library-app:latest`, монтирование `config2.toml` + `bookarch2/tempfld2/logs2/backup2/apk2`), `config2.toml` (port=9092, dbname=library2, отдельный jwt_secret).
  - nginx: `nginx-federation.conf` — два сайта с разными самоподписанными сертификатами `certres/site_a/` (CN=library-site-a) и `certres/site_b/` (CN=library-site-b): 443/444 → app1 (9091), 445 → app2 (9092); HTTP 80 → 301 HTTPS; форвард `X-Platform`.
  - Интеграционное тестирование E2E: app1→app2 по `https://127.0.0.1:445` (книга «Понедельник начинается в субботу», Стругацкие, FB2) и app2→app1 по `https://127.0.0.1:444` (2 книги «Беседы…») — вход на соседа с ролью server через самоподписанный сертификат работает в обе стороны; вариант без совпадений возвращает `books: []`. Учётные записи для теста: `fed_admin1/fed_admin2` (admin, пароль `fed-admin-pass`) и `fed_server1/fed_server2` (server, `fed-server-pass`) на соответствующих БД; сосед на app1 → app2 (id=13), на app2 → app1 (id=1).
  - Проверено: `go test -count=1 ./src/` зелёный, `node --check` OK, APK-ассеты (`admin.html`, `admin.js`, `admin.css`) синхронизированы, все эндпоинты отвечают (80→301, 443/444/445→401, 9091/9092→200), оба контейнера и nginx оставлены запущенными.
- **Favicon и иконка APK на основе `tmp/book.svg`**: SVG — «стоящая книга» (Inkscape, viewBox 0 0 841.89 595.28, встроенный BMP-«обложка» + векторные тёмные контуры #1b1918, видимый арт 613x1066 на прозрачном фоне). Установлены `librsvg2-bin` (`rsvg-convert`) и `imagemagick` (через `apt`; `pip`/pypi недоступен 403, npm ок). Из арта собраны: (1) `static/favicon.ico` — многоразмерный ICO 16/32/48/64 на белом фоне, арт на 82%; (2) `static/favicon.svg` — квадрат 64x64 с встроенным PNG base64 (браузеры отдают предпочтение SVG); (3) легаси-иконки `mipmap-*dpi/ic_launcher.png` 48/72/96/144/192; (4) адаптивные иконки API 26+: новые `mipmap-*dpi/ic_launcher_fg.png` 108/162/216/324/432 (арт на 55% — в безопасной зоне), `drawable/ic_launcher_foreground.xml` переведён на `<bitmap android:src="@mipmap/ic_launcher_fg">`, `ic_launcher_background.xml` залит белым (#ffffff) вместо #3a3a3a. Все favicon-файлы синхронизированы в APK-ассеты (`assets/www/` и `assets/www/static/`). Проверено: `go test ./src/...` зелёный, `identify` по всем mipmap верен, сервер отдаёт `/static/favicon.ico` 200 (13094 b) и `/static/favicon.svg` 200 (2940 b) через https.
- **Исправлен пропущенный слой в иконках**: встроенный BMP-«обложка» в `tmp/book.svg` имел alpha=0 у всех 156 792 пикселей (файл 564x278, 32bpp BGRA, все байты alpha нулевые) — rsvg-convert рендерил его как полностью прозрачный, поэтому на иконках была только тёмная векторная «стоящая книга» без изображения книги на фоне. Исправление: декодирован BMP, alpha принудительно выставлен в 255, переупакован в PNG base64 и подставлен в SVG (`book_fixed`), рендер стал 872x1066 (включая обложку). Все иконки перегенерированы из исправленного рендера: `static/favicon.ico` (24358 b), `static/favicon.svg` (4252 b), все `mipmap-*` и `ic_launcher_fg`. Синхронизировано в APK-ассеты. Проверено ASCII-визуализацией: на иконке видны и обложка (рисунок открытой книги), и векторный силуэт; `go test ./src/...` зелёный; сервер отдаёт новые файлы 200.
- **Восстановлен доступ к серверу**: nginx-контейнер `library-nginx` упал с `host not found in upstream "app"` — конфиг `nginx.conf` проксирует на docker-compose имя `app:8080`, а standalone-приложение работает на host-сети на порту 9091. Создан `nginx-standalone.conf` (proxy на `127.0.0.1:9091`, форвард `X-Platform`) и nginx перезапущен в `--net=host`. Проверено: `https://localhost/` → 200, HTTP → 301 HTTPS, логин через HTTPS OK, `/admin` → 200, детекция android через прокси работает, `rlCreateFromTextBtn` отдаётся.
- **Layout вкладки «Программы чтения» (админка)**: тулбар перестроен в 2 колонки — слева фильтры (дропдауны Дети/Списки/Статус в первой строке, «Название книги» + «Автор» + «Применить» со второй строки), справа рамка «Массовые операции» (заголовок сверху, кнопка «Создать из текста», затем На полку / Установить статус / Создать список / Удалить). Исправлено смещение кнопки «Удалить» (`btn-danger` имел `margin-bottom:15px` — добавлен `.rl-bulk-buttons .btn { margin-bottom:0 }`). Новый CSS: `.rl-toolbar`, `.rl-filters-col`, `.rl-filter-row`, `.rl-bulk-top`. Вкладка «Программы чтения» скрыта в APK: `body.android .admin-tab[data-tab="readlists"]`/`#tab-readlists { display:none }` в `mobile.css` (детекция через `<body class="android">`, инжектится и сервером, и `MainActivity`). Проверено: `go test ./src/` зелёный, `node --check` OK, контейнер перезапущен (шаблон кэшируется), `/admin` отдаёт `rl-toolbar`/`rlCreateFromTextBtn`, `/static/css/mobile.css` содержит правило скрытия, E2E «Создать из текста» через API работает, тестовые данные вычищены, APK-ассеты синхронизированы.
- **Кнопка «Создать из текста» в «Массовых операциях»** (`rlCreateFromTextBtn`): модалка с выбором детей (пикер переиспользует `initChildPicker`/`collectChildPick`), названием списка (префилл из единственного выбранного фильтра-списка), textarea «по одной книге на строку, формат «Автор — Название»» и селектом статуса. `parseBookFromTextLine()` разбирает строки (поддержка `—`/`–`/`-`), для каждой строки вызывается `POST /readlists` (одни и те же user_ids/listname/status), итог — alert с количеством созданных/ошибок. Кнопка привязана в `setupReadlistsFilters`.
- **Статические тесты фронтенда `src/js_static_test.go`**: защита от класса бага, где Go-тесты проходят, а JS сломан (регрессия `19d1ad8` — делегирование вызывало удалённые `editUser`/`deleteUser`). Проверяют: (1) все bare-вызовы функций в JS определены в одном из JS-файлов страницы; (2) `getElementById('X').addEventListener` — ID существует в HTML или создаётся динамически в JS; (3) `<script src="/static/js/...">` указывает на существующий файл; (4) API-пути в JS зарегистрированы в main.go (с разрешением group-префиксов). Также найден и исправлен латентный баг: `appDebug(...)` в `offline.js:7` вызывался, но не был определён — теперь `window.appDebug(...)`. Проверено: тест ловит исходную регрессию (`deleteUser`/`editUser` не определены → FAIL), весь `go test ./src/` зелёный. `offline.js` синхронизирован в `src_android/app/src/main/assets/www/static/js/offline.js`.
- **Восстановлено управление пользователями/авторами/жанрами в `static/js/admin.js`**: рефакторинг в коммите `19d1ad8` удалил `addUserBtn`/`addAuthorBtn`/`addGenreBtn`-обработчики и функции `editUser`/`deleteUser`/`editAuthor`/`deleteAuthor`/`editGenre`/`deleteGenre` (кнопка «+ Создать пользователя» на админке не работала). Функции восстановлены из `git show 19d1ad8^`, жанровые мутации переведены на `/api/v1/genres` (в admin-группе только GET). Файл синхронизирован в `src_android/app/src/main/assets/www/static/js/admin.js`. Проверено: `node --check` OK, полный цикл create/update/delete пользователя через API работает.
- Реализована поддержка env-переменных `LIBAPP_JWT_SECRET` и `LIBAPP_TOKEN_TTL` в `src/config/config.go` (`applyEnv`) — документация в README/AGENTS.md и код теперь совпадают.
- Проверено соответствие конфигурации: `config.toml.example` = структура `Config` в коде; все `LIBAPP_*` из README/AGENTS читаются кодом; env.example (volume-mount пути для docker-compose) не является конфигом приложения.

### Blocked
- `.rar` files in example (10_the_active_side_of_infinity.rar, 11_the_wheel_of_time.rar) are not supported – no RAR extraction; user has not requested this.

## Key Decisions
- Server differentiates desktop vs Android (via `X-Platform: android` header) and injects mobile-specific CSS/JS.
- Shelf extraction writes the final extracted format to `tempfld/shelf/{edition_id}/`; the original ZIP remains untouched in `bookarch/`.
- Double-ZIP bug fix ensures the stored archive in `bookarch/` contains the actual book file instead of a nested ZIP.
- nginx is the recommended HTTPS reverse proxy for production; the Docker override (`docker-compose-nginx.yml`) adds it without modifying the base compose file.
- Session cookie Secure flag is hardcoded to `false` — acceptable for local/RPi deployment, but should be made dynamic for production.

### APK Offline Algorithm

**All static files** (`static/css/`, `static/js/`, `templates/`, `service-worker.js`, `favicon.*`) are bundled into the APK assets at build time by `build-android.sh`. The SPA loads entirely from assets — no network needed for the UI.

#### Startup Flow

1. `loadUrl(TARGET_URL)` — WebView navigates to server URL
2. `shouldInterceptRequest(WebResourceRequest)` intercepts the request:
   - `/` → `serveIndexFromAssets()` — reads `www/index.html` from APK, injects mobile CSS/JS, returns as `WebResourceResponse`
   - `/admin` → `serveAdminFromAssets()` — same with admin template
   - `/static/*` → `serveFromAssets()` — reads from `www/static/`
   - `/service-worker.js`, `/favicon.*` → served from assets
   - All other paths → `null` (let network handle)
3. When `forceNetworkRefresh` is `true` (set after login via `AndroidTokenBridge.setForceNetworkRefresh`), `shouldInterceptRequest` returns `null` for ALL requests — forces fresh load from server
4. `onPageFinished` fires after asset-served page loads → evaluates JS to check for content selectors (`.container`, `.tabs`, etc.). If content found, hides debug panel
5. **5-second watchdog** (`startupTimeoutRunnable`): if no content detected by JS, calls `loadOfflinePage()` as last resort
6. `loadOfflinePage()` reads `www/offline.html` from assets, displays via `loadDataWithBaseURL`

#### API Calls (Offline Behavior)

- **No `AndroidHttpProxy`** — fetch API is native (async, WebView-managed)
- The SPA calls `/api/v1/*` via standard `fetch()` → WebView sends HTTP request natively
- Self-signed certificates: `onReceivedSslError` → `handler.proceed()` (trust all)
- `X-Platform: android` header is added by the SPA's JS (via `fetch` interceptor in `auth.js`)
- If server is unreachable, native fetch rejects with a network error → SPA handles it (shows error state, no blocking)

#### Why No AndroidHttpProxy

The `0f723fe94` commit introduced `AndroidHttpProxy` — a synchronous `@JavascriptInterface` bridge that routed all `/api/` fetch calls through Java `HttpURLConnection`. This caused:

- JS thread blocked during every API call (synchronous bridge)
- 30s+ UI freeze when server unreachable (connect timeout)
- No concurrent requests possible

The fix: removed `AndroidHttpProxy` entirely. WebView's native fetch is fully async and handles timeouts, retries, and concurrency correctly.

#### `offline.html`

A static fallback page at `src_android/app/src/main/assets/www/offline.html`. Self-contained (inlined CSS/JS). Used only when:
- Server unreachable AND Service Worker has no cached page (first visit)
- Startup watchdog (5s) detects no content after asset load

When changing the SPA UI, `offline.html` must be manually kept in sync (same structure/bridges). See the table below: | What changed | Need to update offline.html? |
|---|---|
| CSS styling | **Yes** — inline styles |
| Readlist card structure | **Yes** — hardcoded HTML |
| JS bridge API (`AndroidReadListDB.*`) | **Yes** — direct calls |
| Template structure (`index.html`) | If mobile layout changes |
| New API endpoints | No |
| Backend logic | No |

### DB Migration + Backup Policy

**Before every migration, the application creates a `pg_dump` backup** in the configured `backup_dir`.

- `backup_dir` must be set in `config.toml` (`[directories] backup`) or via `LIBAPP_DIR_BACKUP`.
- If `backup_dir` is empty and pending migrations exist, the application **refuses to start** with a clear error message.
- Backup filename format: `library_{currentVersion}_before_{targetVersion}.sql`
- On successful migration, the backup file is **kept** (not deleted) for safety.
- To force a manual backup: run `pg_dump` directly or use the application's backup mechanism.
- The backup path is also stored in the `settings` DB table and visible in the admin panel (Settings tab).

## Next Steps
- Add graceful shutdown (SIGTERM/SIGINT handler) to avoid connection drops on restart.
- Add `db.SetMaxOpenConns` / `db.SetMaxIdleConns` configuration for connection pool tuning.
- Add health-check endpoint (`/health`).
- Make session cookie Secure flag dynamic based on request scheme.
- Harden Dockerfile: add non-root user, pin `alpine` digest.
- Harden TLS: add `tls.Config` with `MinVersion: tls.VersionTLS12`.
- Add magic-bytes validation for file uploads.
