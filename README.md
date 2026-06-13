# Домашняя библиотека

Веб-приложение для управления домашней коллекцией книг с REST API, OPDS-каталогом (для читалок) и SPA-интерфейсом.

## Возможности

- **Управление книгами** — добавление, редактирование, удаление книг с метаданными (название, автор, год, ISBN, аннотация, издатель, обложка)
- **Иерархия авторов и жанров** — дерево автор → произведение → издание с фильтрацией
- **Импорт книг** — массовый асинхронный импорт из файлов и ZIP-архивов с прогрессом и отменой
- **LLM-распознавание** — автоматическое определение названия и автора по тексту первых страниц (PDF, DOC, DOCX) через Ollama / OpenAI-совместимый API
- **Проверка дубликатов** — SHA-256 хеш контента; если книга уже есть, импорт пропускается
- **Полка (избранное)** — быстрый доступ к отмеченным книгам
- **Поиск** — по автору, названию, жанру, дате; ё→е, регистронезависимый, GIN trgm индексы
- **OPDS-каталог** — доступ к библиотеке с электронных читалок через OPDS 1.2
- **SPA-интерфейс** — три вкладки (Авторы, Книги, Жанры), кнопка быстрого импорта, прогресс загрузки

### Поддерживаемые форматы

| Формат | Метаданные | Распознавание |
|--------|-----------|---------------|
| FB2 | Нативные (автор, жанры, аннотация, обложка, ISBN, издатель, язык) | XML-парсер |
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

### Переменные окружения

| Переменная | Раздел | Описание |
|------------|--------|----------|
| `LIBAPP_PORT` / `PORT` | server.port | Порт HTTP |
| `LIBAPP_BIND` | server.bind | Адрес привязки |
| `LIBAPP_ENABLE_DELETE` | server.enable_delete | Разрешить удаление |
| `LIBAPP_LOG_LEVEL` | server.log_level | Уровень логирования |
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

## API

### Книги

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/books` | Список (?author=, ?book=, ?genre=, ?date_from=, ?date_to=, ?sort_by=, ?sort_order=, ?limit=, ?offset=) |
| GET | `/api/v1/books/search` | Поиск |
| POST | `/api/v1/books` | Создание |
| GET | `/api/v1/books/:id` | Информация |
| PUT | `/api/v1/books/:id` | Обновление |
| DELETE | `/api/v1/books/:id` | Удаление + осиротевшая работа |
| GET | `/api/v1/books/:id/extended` | Расширенная информация (ISBN, аннотация, издатель) |
| PUT | `/api/v1/books/:id/extended` | Обновление расширенных данных |
| PUT | `/api/v1/books/:id/shelf` | Добавить/убрать с полки |
| GET | `/api/v1/books/:id/download` | Скачать файл (ZIP) |
| POST | `/api/v1/books/:id/cover` | Загрузить обложку |

### Авторы, жанры, теги

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/authors` | Авторы с деревом книг |
| GET | `/api/v1/genres` | Список жанров |
| GET | `/api/v1/genres/tree` | Дерево жанров (?genre=, ?author=, ?book=) |
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
go test -count=1 ./src/...
```

Для тестов нужна PostgreSQL (`DATABASE_URL`).

## Структура проекта

```
lbl/
├── bookarch/                  # Хранилище книг (ZIP-архивы)
├── db/scripts/                # Скрипты БД
├── logs/                      # Логи
├── src/                       # Исходный код
│   ├── main.go                # Точка входа, все хендлеры, маршруты, ImportManager
│   ├── main_test.go           # Тесты
│   ├── schema.sql             # Встраиваемая схема БД (go:embed)
│   ├── migration_1.1.sql      # Миграция (go:embed)
│   ├── opds.go                # OPDS XML-каталог
│   ├── auth.go                # (не используется)
│   ├── jwt.go                 # (не используется)
│   ├── reading.go             # (не используется)
│   ├── recommendations.go     # (не используется)
│   ├── export.go              # (не используется)
│   ├── config/
│   │   └── config.go          # Структура, загрузка из TOML + env
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
│   ├── js/app.js              # SPA: вкладки, авторы, книги, жанры
│   ├── js/import.js           # Импорт с прогрессом и polling
│   └── favicon.ico
├── templates/
│   └── index.html             # SPA (4 вкладки), модальное окно
├── tempfld/                   # Временные файлы загрузки
├── testdata/                  # Тестовые книги
├── config.toml.example        # Пример конфига
├── env.example                # Пример .env для Docker
├── docker-compose.yml         # Docker Compose (БД + приложение)
├── Dockerfile                 # Многоступенчатая сборка
├── Dockerfile.all-in-one      # Всё в одном (БД + приложение)
├── startup.sh                 # Точка входа контейнера
├── go.mod / go.sum
└── AGENTS.md                  # Инструкции для ассистентов
```

## Примечания

- **Схема БД** создаётся автоматически при первом запуске (embedded `schema.sql` + миграции). База данных также создаётся автоматически, если не существует.
- **LLM-вызовы** сериализованы через `sync.Mutex` — при массовом импорте PDF/DOC/DOCX файлы обрабатываются последовательно.
- **Транслитерация** названий книг в пути архива применяется только для определённых названий (из метаданных или LLM). Если название не определено, сохраняется оригинальное имя файла.
- **Дубликаты** проверяются по SHA-256 от содержимого до обращения к LLM.

## Лицензия

MIT
