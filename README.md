# Library Management System (LMS)

Веб-приложение для управления домашней библиотекой книг с поддержкой OPDS-каталога для чтения на электронных читалках.

## Функциональность

- **Управление книгами**: добавление, редактирование, удаление книг с метаданными (название, год, ISBN, аннотация)
- **Авторы и жанры**: иерархическая структура авторов с книгами, категоризация по жанрам
- **Система тегов**: гибкая маркировка книг тегами
- **Обложки книг**: загрузка и отображение обложек
- **Импорт книг**: массовый асинхронный импорт из файлов (FB2, EPUB, PDF, DOC, DOCX) и ZIP-архивов
- **LLM-распознавание**: автоматическое определение названия и автора по тексту первых страниц (Ollama/OpenAI-compatible)
- **Полка (избранное)**: возможность отмечать книги для быстрого доступа
- **Поиск**: поиск по названию, автору, ISBN, жанру с поддержкой ё→е и регистронезависимости
- **OPDS-каталог**: доступ к библиотеке с читалок через OPDS-протокол

### Поддерживаемые форматы

| Формат | Метаданные | Примечание |
|--------|-----------|------------|
| FB2 | Нативные (автор, жанры, аннотация, обложка, ISBN) | XML-разбор, поддержка CP1251/KOI8-R/ISO-8859-5 |
| EPUB | Нативные (автор, ISBN, издатель, язык) | Разбор container.xml + OPF |
| PDF | LLM-распознавание (первые 3 страницы, до 2000 символов) | github.com/ledongthuc/pdf |
| DOCX | LLM-распознавание (первые 3 страницы) | word/document.xml |
| DOC | LLM-распознавание (первые 3 страницы) | OLE2 + UTF-16LE (mscfb) |
| ZIP | Автоопределение формата внутри | FB2, EPUB, PDF, DOC, DOCX |

### OPDS-каталог

Читалки могут подключаться к каталогу по адресу `/api/v1/opds/catalog.xml`:

| Эндпоинт | Описание |
|----------|----------|
| `GET /api/v1/opds/catalog.xml` | Корневой каталог |
| `GET /api/v1/opds/latest.xml` | Последние добавленные книги |
| `GET /api/v1/opds/genres.xml` | Список жанров |
| `GET /api/v1/opds/genre/:id.xml` | Книги жанра |
| `GET /api/v1/opds/search.xml?q=` | Поиск |
| `GET /api/v1/opds/book/:id` | Скачивание книги |

## Конфигурация

Приложение читает `config.toml` из текущей директории (или `CONFIG_PATH` env var).
Пример — `config.toml.example`:

```toml
[server]
port = 9091
bind = "0.0.0.0"
enable_delete = true
log_level = "info"

[directories]
bookarch = "bookarch"
temp = "tempfld"
logs = "logs"
templates = "templates"
static = "static"

[database]
host = "localhost"
port = 5432
name = "library"
user = "postgres"
password = "postgres"
sslmode = "disable"

[llm]
base_url = "http://192.168.1.2:11434"
model = "phi4:latest"
token = ""
timeout = 60
prompt = "По тексту первых страниц книги определи автора и название..."
prompt2= "По фрагменту текста определи автора и название книги..."
```

### Переменные окружения (переопределяют config.toml)

| Переменная | Описание |
|------------|----------|
| `DATABASE_URL` | Полная строка подключения к БД |
| `PORT` | Порт HTTP-сервера |

## Сборка и запуск

### Требования

- Go 1.25+
- PostgreSQL 17+

### Локальный запуск

```bash
# Клонирование репозитория
git clone <repository_url>
cd lbl

# Настройка базы данных
sudo -u postgres psql -d librarydb -f db/scripts/init_db.sql

# Копирование и правка конфига
cp config.toml.example config.toml
# Отредактируйте config.toml под свои параметры

# Сборка
go build -o library_app ./src/

# Запуск
./library_app
```

Приложение доступно по адресу: http://localhost:9091

### Docker

```bash
# Сборка all-in-one образа (приложение + PostgreSQL)
docker build -f Dockerfile.all-in-one -t library-app .

# Запуск
docker run -d -p 9091:9091 --name library library-app
```

## API эндпоинты

### Книги

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/books` | Список книг (?author=, ?book=, ?genre=, ?date_from=, ?date_to=, ?sort_by=, ?sort_order=, ?limit=, ?offset=) |
| GET | `/api/v1/books/search` | Поиск книг |
| POST | `/api/v1/books` | Создание книги |
| GET | `/api/v1/books/:id` | Информация о книге |
| PUT | `/api/v1/books/:id` | Обновление книги |
| DELETE | `/api/v1/books/:id` | Удаление книги + осиротевшей работы |
| GET | `/api/v1/books/:id/extended` | Расширенная информация (ISBN, аннотация, издатель) |
| PUT | `/api/v1/books/:id/extended` | Обновление расширенных данных |
| PUT | `/api/v1/books/:id/shelf` | Добавить/убрать с полки |
| GET | `/api/v1/books/:id/download` | Скачать файл книги |
| POST | `/api/v1/books/:id/cover` | Загрузить обложку |

### Авторы, жанры, теги

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/authors` | Список авторов с деревом книг |
| GET | `/api/v1/genres` | Список жанров |
| POST | `/api/v1/genres` | Создание жанра |
| GET | `/api/v1/tags` | Список тегов |
| POST | `/api/v1/tags` | Создание тега |
| GET | `/api/v1/persons` | Список всех персон |
| PUT | `/api/v1/persons/:id` | Обновление имени персоны |
| GET | `/api/v1/languages` | Список языков |

### Импорт (асинхронный)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/v1/import/upload` | Загрузка файлов (multipart) → асинхронный импорт |
| POST | `/api/v1/import/directory` | Импорт из директории на сервере |
| GET | `/api/v1/import/status` | Статус импорта (running, total, completed, errors) |
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
| GET | `/api/v1/config` | Конфигурация приложения (enable_delete) |
| GET | `/debug/goroutines` | Дамп горутин |

### OPDS

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/opds/catalog.xml` | Корневой каталог |
| GET | `/api/v1/opds/latest.xml` | Последние книги |
| GET | `/api/v1/opds/genres.xml` | Жанры |
| GET | `/api/v1/opds/genre/:id.xml` | Книги жанра |
| GET | `/api/v1/opds/search.xml?q=` | Поиск |
| GET | `/api/v1/opds/book/:id` | Скачивание книги |

## Структура проекта

```
lbl/
├── bookarch/         # Директория хранения книжных файлов (ZIP-архивы)
├── db/
│   └── scripts/
│       └── init_db.sql  # SQL-скрипт инициализации БД (схема + триггеры + индексы)
├── logs/             # Логи приложения
├── src/              # Исходный код Go
│   ├── main.go       # Точка входа, все хендлеры, маршруты, ImportManager
│   ├── main_test.go  # Тесты
│   ├── opds.go       # OPDS XML-каталог
│   ├── auth.go       # Аутентификация (не используется)
│   ├── jwt.go        # JWT (не используется)
│   ├── reading.go    # Прогресс чтения (не используется)
│   ├── recommendations.go  # Рекомендации (не используется)
│   ├── export.go     # Экспорт/импорт (не используется)
│   ├── config/
│   │   └── config.go      # Структура конфига, загрузка из TOML
│   └── utils/
│       ├── llm_client.go   # OpenAI-совместимый LLM-клиент (sync.Mutex, ретрай с prompt2)
│       ├── pdf_extract.go  # Извлечение текста из PDF (первые 3 страницы)
│       ├── docx_extract.go # Извлечение текста из DOCX (word/document.xml)
│       ├── doc_extract.go  # Извлечение текста из DOC (OLE2 + UTF-16LE)
│       ├── epub.go         # Парсинг EPUB метаданных
│       ├── fb2.go          # Парсинг FB2 метаданных (CP1251, жанры, ISBN)
│       ├── fb2_test.go     # Тесты FB2
│       ├── epub_test.go    # Тесты EPUB
│       └── zip_extract.go  # Определение формата внутри ZIP
├── static/           # Статические файлы
│   ├── css/style.css
│   ├── js/app.js     # SPA: авторы (дерево), книги (таблица с сортировкой/фильтрами)
│   ├── js/import.js  # Асинхронный импорт с polling
│   └── favicon.ico
├── templates/
│   └── index.html    # SPA-шаблон (табы: авторы, книги, импорт)
├── tempfld/          # Временная директория для загрузки и обработки
├── testdata/         # Тестовые файлы книг
├── config.toml.example  # Шаблон конфига
├── Dockerfile        # Многоступенчатая сборка
├── Dockerfile.all-in-one  # Всё в одном (приложение + БД)
├── go.mod / go.sum   # Go-модуль
├── startup.sh        # Точка входа контейнера
└── README.md / AGENTS.md  # Документация
```

## Поиск и фильтрация

- Все поисковые запросы приводятся к нижнему регистру с заменой ё→е (`normalizeQuery()`)
- Индексы GIN trgm на полях `persons.lower_fio`, `works.lower_original_title`, `editions.lower_title`
- Триггер `normalize_search_field` автоматически заполняет lower_ поля через `REPLACE(LOWER(...), 'ё', 'е')`
- Фильтр книг: ?author=, ?book=, ?genre=, ?date_from=, ?date_to=, ?sort_by=(original_title|upload_date|authors|available_formats), ?sort_order=(asc|desc), ?limit=, ?offset=
- Интервал дат: date_from с 00:00, date_to до 23:59

## Тестирование

```bash
go test ./src/...
```

## Лицензия

MIT
