# Домашняя библиотека

Веб-приложение для управления домашней коллекцией книг с REST API, OPDS-каталогом (для читалок) и SPA-интерфейсом.

## Возможности

- **Управление книгами** — добавление, редактирование, удаление книг с метаданными (название, автор, год, ISBN, аннотация, издатель, обложка)
- **Иерархия авторов и жанров** — дерево автор → произведение → издание с фильтрацией
- **Импорт книг** — массовый асинхронный импорт из файлов и ZIP-архивов с прогрессом и отменой
- **LLM-распознавание** — автоматическое определение названия и автора по тексту первых страниц (PDF, DOC, DOCX) через Ollama / OpenAI-совместимый API
- **Проверка дубликатов** — SHA-256 хеш контента; если книга уже есть, импорт пропускается
- **Полка (избранное)** — быстрый доступ к отмеченным книгам (общая для всех пользователей)
- **Статус чтения** — отслеживание прогресса: не начато, читаю, прочитано (ведется для каждого пользователя системы)
- **Поиск** — по автору, названию, жанру, дате; ё→е, регистронезависимый, GIN trgm индексы
- **Ролевая модель** — viewer (просмотр), editor (каталог + админка без пользователей), admin (полный доступ)
- **OPDS-каталог** — доступ к библиотеке с электронных читалок через OPDS 1.2
- **SPA-интерфейс** — три вкладки (Авторы, Книги, Жанры) + Импорт
- **Админ-панель** — управление пользователями, каталогом, настройками LLM

### Поддерживаемые форматы

| Формат | Метаданные | Распознавание |
|--------|-----------|---------------|
| FB2 | Нативные (автор, жанры, аннотация, обложка, ISBN, издатель, язык) | XML-парсер (CP1251, KOI8-R) |
| EPUB | Нативные (автор, ISBN, издатель, язык, жанры) | container.xml + OPF |
| PDF | LLM (первые 3 страницы, до 2000 символов) | github.com/ledongthuc/pdf |
| DOCX | LLM (первые 3 страницы) | word/document.xml |
| DOC | LLM (первые 3 страницы) | OLE2 + UTF-16LE (mscfb) |
| ZIP | Автоопределение формата внутри | FB2, EPUB, PDF, DOC, DOCX |


## Быстрый старт
Проще всего установить через docker compose, см раздел ниже

### Docker Compose

```bash
git clone https://github.com/szhebet/lbl.git
cd lbl

cp config.toml.example config.toml
cp env.example .env
# Отредактировать .env (пароль, пути)

docker compose up -d --build
```

Приложение: http://localhost:9092


Для тех, кто ищет варианты посложнее...
### Локальный запуск

#### Требования

- Go 1.25+
- PostgreSQL 17+

#### Инструкции

```bash
git clone https://github.com/szhebet/lbl.git
cd lbl

# Копировать и отредактировать конфиг
cp config.toml.example config.toml

# Сборка
go build -o library_app ./src/

# Запуск (БД должна быть доступна, схема создаётся автоматически)
./library_app
```

Приложение доступно по адресу: http://localhost:9091

## Конфигурация

Приложение читает настройки в следующем порядке (последний имеет наивысший приоритет):

1. **Значения по умолчанию** (встроенные)
2. **Переменные окружения** с префиксом `LIBAPP_` (и `PORT` / `DATABASE_URL` для обратной совместимости)
3. **Файл `config.toml`** (текущая директория или `CONFIG_PATH`)

### Параметры сервера

| Поле | По умолчанию | Описание |
|------|-------------|----------|
| `port` | `9091` | Порт HTTP |
| `bind` | `0.0.0.0` | Адрес привязки |
| `enable_delete` | `false` | Разрешить удаление книг |
| `log_level` | `info` | Уровень логирования |
| `jwt_secret` | (автогенерация) | Секретный ключ JWT |
| `token_ttl` | 24 | Время жизни токена (часы) |

### Переменные окружения

| Переменная | Раздел | Описание |
|------------|--------|----------|
| `LIBAPP_PORT` / `PORT` | server.port | Порт HTTP |
| `LIBAPP_BIND` | server.bind | Адрес привязки |
| `LIBAPP_ENABLE_DELETE` | server.enable_delete | Разрешить удаление |
| `LIBAPP_LOG_LEVEL` | server.log_level | Уровень логирования |
| `LIBAPP_JWT_SECRET` | server.jwt_secret | Секрет JWT |
| `LIBAPP_TOKEN_TTL` | server.token_ttl | TTL токена (часы) |
| `LIBAPP_DIR_BOOKARCH` | directories.bookarch | Хранилище книг |
| `LIBAPP_DIR_TEMP` | directories.temp | Временная директория |
| `LIBAPP_DIR_LOGS` | directories.logs | Директория логов |
| `LIBAPP_DIR_TEMPLATES` | directories.templates | Шаблоны |
| `LIBAPP_DIR_STATIC` | directories.static | Статика |
| `LIBAPP_DATABASE_URL` / `DATABASE_URL` | — | DSN или postgres:// URL |
| `LIBAPP_DB_HOST` | database.host | Хост БД |
| `LIBAPP_DB_PORT` | database.port | Порт БД |
| `LIBAPP_DB_NAME` | database.name | Имя БД |
| `LIBAPP_DB_USER` | database.user | Пользователь БД |
| `LIBAPP_DB_PASSWORD` | database.password | Пароль БД |
| `LIBAPP_DB_SSLMODE` | database.sslmode | Режим SSL |
| `LIBAPP_DB_PGDATA` | database.pgdata | Директория данных (all-in-one) |
| `LIBAPP_LLM_BASE_URL` | llm.base_url | URL LLM-сервера |
| `LIBAPP_LLM_MODEL` | llm.model | Модель LLM |
| `LIBAPP_LLM_TOKEN` | llm.token | Токен аутентификации |
| `LIBAPP_LLM_TIMEOUT` | llm.timeout | Таймаут (сек) |
| `LIBAPP_LLM_PROMPT` | llm.prompt | Основной промпт |
| `LIBAPP_LLM_PROMPT2` | llm.prompt2 | Повторный промпт |

Полный пример — `config.toml.example`.

## Аутентификация и роли

### Первый вход

Если в БД нет ни одного пользователя, первый запрос на `/api/v1/auth/login` автоматически создаёт пользователя с ролью `admin` и переданными учетными данными.

### Роли

| Роль | Доступ |
|------|--------|
| `viewer` | Просмотр каталога, поиск, OPDS, скачивание. Админ-панель недоступна (403). |
| `editor` | Полный доступ к каталогу + админ-панель (вкладки Авторы, Книги, Жанры, Теги, Импорт). Без управления пользователями. |
| `admin` | Полный доступ, включая управление пользователями и настройками. |

### JWT

- Токены HS256 с настраиваемым secret (`jwt_secret`) и TTL (`token_ttl`).
- Если `jwt_secret` пуст, генерируется случайный ключ при старте.
- Токен передаётся в заголовке `Authorization: Bearer <token>`.
- Хранится в `localStorage` на фронтенде.


## API

### Аутентификация (гостевые маршруты — без токена)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/v1/auth/login` | Вход (username, password) → JWT + информация о пользователе |
| POST | `/api/v1/auth/register` | Регистрация (username, password) |

### Книги

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/books` | Список (?author=, ?book=, ?genre=, ?date_from=, ?date_to=, ?sort_by=, ?sort_order=, ?limit=, ?offset=) |
| GET | `/api/v1/books/search` | Полнотекстовый поиск |
| POST | `/api/v1/books` | Создание |
| GET | `/api/v1/books/:id` | Информация |
| PUT | `/api/v1/books/:id` | Обновление |
| DELETE | `/api/v1/books/:id` | Удаление + осиротевшая работа |
| GET | `/api/v1/books/:id/extended` | Расширенная информация (ISBN, аннотация, издатель) |
| PUT | `/api/v1/books/:id/extended` | Обновление расширенных данных |
| PUT | `/api/v1/books/:id/shelf` | Добавить/убрать с полки |
| GET | `/api/v1/books/:id/download` | Скачать файл (ZIP) |
| POST | `/api/v1/books/:id/cover` | Загрузить обложку |
| PUT | `/api/v1/books/:id/reading` | Статус чтения (0=не начато, 1=читаю, 2=прочитано) |
| GET | `/api/v1/user/books` | Книги текущего пользователя со статусом чтения |

### Авторы, жанры, теги

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/authors` | Авторы с деревом книг |
| GET | `/api/v1/genres` | Список жанров |
| GET | `/api/v1/genres/tree` | Дерево жанров с вложенными авторами и книгами (?genre=, ?author=, ?book=) |
| POST | `/api/v1/genres` | Создание жанра |
| PUT | `/api/v1/genres/:id` | Обновление |
| DELETE | `/api/v1/genres/:id` | Удаление (только без книг) |
| GET | `/api/v1/tags` | Список тегов |
| POST | `/api/v1/tags` | Создание тега |
| GET | `/api/v1/persons` | Все персоны |
| PUT | `/api/v1/persons/:id` | Обновление имени |
| GET | `/api/v1/languages` | Список языков |

### Импорт

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/v1/import/upload` | Загрузка файлов (multipart) → асинхронный импорт |
| POST | `/api/v1/import/directory` | Импорт из директории на сервере |
| GET | `/api/v1/import/status` | Статус импорта (running, total, completed, errors, items, start_time) |
| POST | `/api/v1/import/cancel` | Отмена текущего импорта |
| POST | `/api/v1/import/file` | Импорт одного файла (синхронно) |

### Администрирование (admin only)

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/admin/users` | Список пользователей |
| POST | `/api/v1/admin/users` | Создание пользователя |
| PUT | `/api/v1/admin/users/:id` | Обновление пользователя |
| DELETE | `/api/v1/admin/users/:id` | Удаление пользователя |
| GET | `/api/v1/admin/settings` | Настройки приложения |
| PUT | `/api/v1/admin/settings` | Обновление настроек |
| GET | `/api/v1/admin/refresh` | Перераспознать все книги через LLM |

### Полка

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/shelf/count` | Количество книг на полке |
| PUT | `/api/v1/shelf/clear` | Очистить полку |

### Прочее

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/config` | Конфигурация (enable_delete) |
| GET | `/debug/goroutines` | Дамп горутин |

### OPDS

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/opds/catalog.xml` | Корневой каталог |
| GET | `/api/v1/opds/latest.xml` | Последние книги |
| GET | `/api/v1/opds/genres.xml` | Список жанров |
| GET | `/api/v1/opds/genre/:id.xml` | Книги жанра |
| GET | `/api/v1/opds/search.xml?q=` | Поиск |
| GET | `/api/v1/opds/book/:id` | Скачивание |

## Сборка и запуск

```bash
go build -o library_app ./src/
./library_app
```

Все зависимости в `go.mod` — сторонних реестров не требуется.

## Тестирование

```bash
go test -count=1 ./src/
```

Для тестов нужна PostgreSQL (`DATABASE_URL`).

## Структура проекта

```
lbl/
├── bookarch/                  # Хранилище книг (ZIP-архивы)
├── db/scripts/                # Скрипты БД
├── logs/                      # Логи приложения
├── src/                       # Исходный код
│   ├── main.go                # Точка входа: маршруты, хендлеры, ImportManager, БД
│   ├── auth.go                # Логин, автосоздание админа при первом входе
│   ├── reading.go             # Статус чтения + middleware проверки ролей
│   ├── jwt.go                 # Генерация и валидация JWT
│   ├── opds.go                # OPDS XML-каталог
│   ├── export.go              # Экспорт/импорт хендлеры
│   ├── main_test.go           # Тесты
│   ├── schema.sql             # Встраиваемая схема БД (go:embed)
│   ├── migration_1.1.sql      # Миграция: статус чтения + пол (gender)
│   ├── config/
│   │   └── config.go          # Структура конфига, Load(), DefaultConfig()
│   └── utils/
│       ├── llm_client.go      # LLM-клиент (sync.Mutex, ретрай)
│       ├── pdf_extract.go     # PDF → текст (первые 3 стр)
│       ├── docx_extract.go    # DOCX → текст
│       ├── doc_extract.go     # DOC → текст (OLE2)
│       ├── epub.go            # EPUB-метаданные
│       ├── fb2.go             # FB2-метаданные (CP1251, KOI8-R)
│       ├── fb2_test.go        # Тесты FB2
│       ├── epub_test.go       # Тесты EPUB
│       └── zip_extract.go     # Определение контента ZIP
├── static/
│   ├── css/style.css
│   ├── js/app.js              # SPA: вкладки Авторы, Книги, Жанры
│   ├── js/import.js           # Асинхронный импорт с polling прогресса
│   └── favicon.ico
├── templates/
│   ├── index.html             # SPA главная страница (4 вкладки)
│   └── admin.html             # Админ-панель SPA
├── tempfld/                   # Директория загрузки файлов
├── testdata/                  # Тестовые книги
├── config.toml.example        # Пример конфига
├── env.example                # Пример .env для Docker
├── docker-compose.yml         # Docker Compose (БД + приложение)
├── Dockerfile                 # Многоступенчатая сборка
├── Dockerfile.all-in-one      # Всё в одном (БД + приложение)
├── startup.sh                 # Точка входа контейнера
├── go.mod / go.sum
├── AGENTS.md                  # Инструкции для ассистентов
└── README.md
```

## TWA Android App

TWA (Trusted Web Activity) упаковывает веб-приложение в Android APK через WebView.

### Сборка APK

```bash
# Полная сборка (debug + release)
./build-android.sh

# Только debug
./build-apk-debug.sh

# Только release
./build-apk-release.sh
```

APK будут в `android-apk/`:

```bash
adb install -r android-apk/app-debug.apk
```

### Настройка клиентского сертификата (mTLS)

Для ограничения доступа к сайту с помощью клиентских сертификатов через nginx:

1. **Сгенерировать сертификаты** (CA + серверный + клиентский):

```bash
cd certres
./generate-certs.sh           # CA + сертификат сервера
./generate-keystore.sh        # Keystore для подписи APK
./generate-assetlinks.sh      # Digital Asset Links
./generate-client-cert.sh     # Клиентский сертификат (для APK)
```

Скрипт `generate-client-cert.sh` проверяет наличие существующего сертификата и переиспользует его при повторном запуске.

2. **Настроить nginx** — добавить в server block:

```nginx
server {
    listen 443 ssl;
    server_name library-app.local;

    ssl_certificate     /path/to/certres/server.crt;
    ssl_certificate_key /path/to/certres/server.key;

    # Client certificate authentication
    ssl_client_certificate /path/to/certres/ca.crt;
    ssl_verify_client on;
    ssl_verify_depth 1;

    # Если нужно разрешить доступ только конкретным клиентам:
    # ssl_verify_client optional_no_ca;  # И проверять ${ssl_client_s_dn} в приложении

    location / {
        proxy_pass http://127.0.0.1:9091;
        proxy_set_header X-Client-Cert $ssl_client_cert;
        proxy_set_header X-Client-Verify $ssl_client_verify;
    }
}
```

3. **Сборка APK** автоматически включает клиентский сертификат (`client.p12` → `res/raw/client_cert.p12`). Приложение WebView отправляет сертификат при запросе со стороны сервера.

4. **Файлы сертификатов:**

| Файл | Назначение |
|------|-----------|
| `ca.crt` | Корневой сертификат CA (для nginx и APK) |
| `ca.key` | Приватный ключ CA (не распространять) |
| `server.crt` | Сертификат сервера (для nginx) |
| `server.key` | Приватный ключ сервера (не распространять) |
| `client.crt` | Клиентский сертификат (для белого списка nginx) |
| `client.key` | Приватный ключ клиента (в APK) |
| `client.p12` | PKCS12 для Android (встраивается в APK) |
| `server.p12` | PKCS12 для Go TLS |

### Структура Android-приложения

```
src_android/
├── app/
│   ├── build.gradle           # Копирование сертификатов в ресурсы
│   └── src/main/
│       ├── AndroidManifest.xml
│       ├── res/raw/
│       │   ├── ca_cert.crt    # Сертификат CA (из certres/)
│       │   └── client_cert.p12 # Клиентский сертификат (из certres/)
│       └── java/app/library/twa/
│           ├── Application.java
│           └── MainActivity.java  # Отправка клиентского сертификата
```

## Примечания

- **Схема БД** создаётся автоматически при первом запуске (embedded `schema.sql` + миграции). База данных также создаётся автоматически, если не существует.
- **LLM-вызовы** сериализованы через `sync.Mutex` — при массовом импорте PDF/DOC/DOCX файлы обрабатываются последовательно.
- **Дубликаты** проверяются по SHA-256 от содержимого до обращения к LLM.
- **Сортировка по году**: книги без года (включая year=0) всегда в конце списка (`NULLIF(year, 0) + NULLS LAST`).
- **Первый вход** создаёт администратора, если в БД нет пользователей.
- **Маршруты без аутентификации**: `GET /`, `GET /static/*`, `GET /favicon.ico`, `POST /api/v1/auth/login`, `POST /api/v1/auth/register`.

## Лицензия

MIT
