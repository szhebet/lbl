# Домашняя библиотека

Веб-приложение для управления домашней коллекцией книг с REST API, OPDS-каталогом (для читалок), SPA-интерфейсом и Android-приложением (WebView) с офлайн-режимом для списка чтения.

## Возможности

- **Управление книгами** — добавление, редактирование, удаление книг с метаданными (название, автор, год, ISBN, аннотация, издатель, обложка)
- **Иерархия авторов и жанров** — дерево автор → произведение → издание с фильтрацией
- **Импорт книг** — массовый асинхронный импорт из файлов и ZIP-архивов с прогрессом и отменой
- **LLM-распознавание** — автоматическое определение названия и автора по тексту первых страниц (PDF, DOC, DOCX) через Ollama / OpenAI-совместимый API
- **Проверка дубликатов** — SHA-256 хеш контента; если книга уже есть, импорт пропускается
- **Полка (избранное)** — быстрый доступ к отмеченным книгам (общая для всех пользователей)
- **Список чтения** — персональные списки книг для каждого пользователя (планирование чтения с приоритетами, статусы «Читаю», «Прочитано», «Отложил», «Бросил»)
- **Запросы книг** — пользователь отмечает, что ищет книгу (локально / по федерации); администратор видит заявки, подбирает книгу из каталога или импортирует файл как предложение
- **Федерация библиотек** — объединение нескольких независимых инстансов в сеть: поиск книги по каталогам соседних серверов («Поиск 1й круг»), импорт найденной книги с сохранением исходных ID и взаимным TLS, рассылка заявок на поиск книг соседям и предложения книг пользователю («Предложить книгу», первый оффер привязывается к заявке — first-offer-wins)
- **Офлайн-синхронизация списка чтения** (Android) — локальная SQLite-копия, очередь изменений, фоновая синхронизация с разрешением конфликтов (last-write-wins)
- **Статус чтения** — отслеживание прогресса: не начато, читаю, прочитано (ведется для каждого пользователя)
- **Поиск** — по автору, названию, жанру, дате; ё→е, регистронезависимый, GIN trgm индексы
- **Ролевая модель** — viewer (просмотр), editor (каталог + админка без пользователей), admin (полный доступ)
- **OPDS-каталог** — доступ к библиотеке с электронных читалок через OPDS 1.2
- **SPA-интерфейс** — четыре вкладки (Авторы, Книги, Жанры, Список чтения)
- **Общая полка** — отдельная страница `/shelf/` со всеми книгами на полке и безопасными токен-ссылками на скачивание
- **Админ-панель** — управление пользователями, каталогом, тегами, запросами книг, настройками LLM
- **Бэкап БД перед миграциями** — автоматический `pg_dump` в `backup_dir` перед применением миграций
- **Автообновление APK** — приложение проверяет версию на сервере и предлагает установить новую
- **Android WebView** — мобильное приложение (WebView) с офлайн-запасной страницей и HTTPS через nginx

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

### Docker Compose (рекомендуется)

```bash
git clone https://github.com/szhebet/lbl.git
cd lbl

cp config.toml.example config.toml
cp env.example .env
# Отредактировать .env (пароль, пути)
# Отредактировать config.toml (jwt_secret, база данных)

docker compose up -d --build
```

Приложение: http://localhost:9092

### Docker Compose + nginx (HTTPS)

```bash
# 1. Сгенерировать сертификаты
cd certres && chmod +x generate-certs.sh && ./generate-certs.sh && cd ..

# 2. Отредактировать docker-compose.yml, при необходимости отключить публикацию лишних портов в app секции
# 3. Запустить с nginx
docker compose -f docker-compose.yml -f docker-compose-nginx.yml up -d --build
```

Приложение: https://localhost

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

### Параметры федерации (`[federation]`)

| Поле | По умолчанию | Описание |
|------|-------------|----------|
| `enabled` | `true` | Включить фоновую рассылку заявок соседям |
| `push_interval_sec` | `300` | Как часто распределитель рассылает одобренные заявки |
| `retry_interval_sec` | `60` | Пауза между повторами для недоступного соседа |
| `retry_window_sec` | `3600` | Сколько времени ретраить, прежде чем пометить доставку `failed` |

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
| `LIBAPP_DIR_APK` | directories.apk_dir | Директория с APK (автообновление) |
| `LIBAPP_DIR_BACKUP` | directories.backup | Директория бэкапов БД |
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
| `LIBAPP_LLM_PROMPT_CONVERT` | llm.prompt_convert | Промпт преобразования списка книг в формат «Автор — Название» (кнопка «LLM-преобразовать» в «Создать из текста») |
| `LIBAPP_FED_ENABLED` | federation.enabled | Включить рассылку заявок соседям |
| `LIBAPP_FED_PUSH_INTERVAL_SEC` | federation.push_interval_sec | Период фоновой рассылки одобренных заявок |
| `LIBAPP_FED_RETRY_INTERVAL_SEC` | federation.retry_interval_sec | Пауза между повторами недоставленного |
| `LIBAPP_FED_RETRY_WINDOW_SEC` | federation.retry_window_sec | Окно ретраев до пометки failed |
| `CONFIG_PATH` | — | Путь к файлу `config.toml` (иначе текущая директория) |

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
- Если `jwt_secret` пуст, генерируется случайный ключ при старте — **все существующие токены станут недействительны после перезапуска**.
- Токен передаётся в заголовке `Authorization: Bearer <token>`.
- Также устанавливается `session_token` cookie (HttpOnly, SameSite=Strict) для обратной совместимости.
- Поддержка refresh-токенов: `POST /api/v1/auth/refresh`.

## API

### Аутентификация (гостевые маршруты — без токена)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/v1/auth/login` | Вход (username, password) → JWT + refresh_token + информация о пользователе |
| POST | `/api/v1/auth/register` | Регистрация (username, password) → viewer |
| POST | `/api/v1/auth/refresh` | Обновление JWT по refresh_token |

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
| GET | `/api/v1/books/:id/download` | Скачать файл (?mode=extracted — распакованный оригинал) |
| POST | `/api/v1/books/:id/cover` | Загрузить обложку (JPEG, PNG, WebP, макс 10 MB) |
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

### Список чтения (Read List)

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/user/readlist` | Список чтения текущего пользователя (сортировка по приоритету) |
| POST | `/api/v1/user/readlist` | Создать запись (UUID id, created_at/updated_at, looking_for, deleted) |
| GET | `/api/v1/user/readlist/names` | Названия списков |
| GET | `/api/v1/user/readlist/:id` | Конкретная запись |
| PUT | `/api/v1/user/readlist/:id` | Обновить запись (конфликт → 409 с `server_item`) |
| DELETE | `/api/v1/user/readlist/:id` | Удалить запись (мягкое удаление, `deleted=true`) |
| GET | `/api/v1/user/readlist/:id/offers` | Предложения книг по этой записи (fed_offers: linked первым, затем новые) |
| POST | `/api/v1/user/readlist/:id/offers/link` | Связать предложение с записью. Тело: `{offer_id}` (число или строка) |
| GET | `/api/v1/user/readlist/offers` | Все предложения по всем спискам пользователя одним запросом (для офлайн-синка APK) |

#### Кнопки LLM в форме «Создать из текста» (админка)

Форма «Создать из текста» (вкладка «Программы чтения») позволяет массово создать записи списков чтения из текста в формате «Автор — Название». Для удобства доступны две кнопки LLM:

- **LLM-промпт** — копирует в буфер обмена промпт из `config.toml` (`llm.prompt_convert`), конкатенированный с текущим текстом поля. Удобно для ручной обработки во внешней LLM.
- **LLM-преобразовать** — отправляет промпт + текст поля в настроенную LLM (`llm.base_url`/`llm.model`/`llm.token`) и заменяет текст поля результатом. После этого текст парсится кнопкой «Применить».

Промпт по умолчанию (`llm.prompt_convert`):
`Преобразуй к формату Автор - Название произведения следующий текст: \n`

Промпт передаётся как одно сообщение `user` в `/v1/chat/completions` (Ollama / OpenAI-совместимый API).

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

### Федерация: соседние серверы (admin only)

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/admin/neighbours` | Список соседних серверов |
| GET | `/api/v1/admin/neighbours/:id` | Информация о конкретном соседе |
| POST | `/api/v1/admin/neighbours` | Добавить соседа (url, username, password, server_cert, client_cert) |
| PUT | `/api/v1/admin/neighbours/:id` | Обновить соседа (пароль не меняется, если не передан; `clear_password` — стереть) |
| DELETE | `/api/v1/admin/neighbours/:id` | Удалить соседа |

### Федерация: поиск и импорт (admin only)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/v1/admin/federation/search` | Поиск по каталогам соседей (`?limit=`, `?stop_on_first=1`). Тело: `{query, author, title}`. Ответ: `{neighbours, results:[{neighbour_id, url, error?, total, books}]}` |
| POST | `/api/v1/admin/federation/import` | Импорт книги с соседа. Тело: `{neighbour_id, edition_id, mode}`. `mode` = `""` (импорт), `overwrite`, `create_new` |
| POST | `/api/v1/admin/federation/test` | Тест связи с соседом (логин + ping). Тело: `{neighbour_id}` → `{ok:true}` или HTTP 502 |
| POST | `/api/v1/admin/federation/offer` | Предложить книгу по входящей заявке соседа (reference/pull: файл не пересылается — сосед сам скачивает). Тело: `{incoming_request_id, edition_id}` |

### Федерация: рассылка заявок (admin only)

Заявка пользователя (`read_list`, `looking_for != 'Нет'`) уходит соседям только
после одобрения администратором кнопкой **«Запросить по федерации»**.

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/admin/fed/outgoing` | Одобренные исходящие заявки (staging-таблица `fed_outgoing_requests`) |
| POST | `/api/v1/admin/fed/outgoing` | Одобрить заявку и поставить в рассылку. Тело: `{read_list_id}` |
| DELETE | `/api/v1/admin/fed/outgoing/:id` | Отозвать одобрение (pending/failed доставки отменяются) |
| POST | `/api/v1/admin/fed/push-now` | Ручная досылка не дожидаясь тика распределителя |
| GET | `/api/v1/admin/fed/requests` | Входящие заявки от соседей («Запросы соседей») |
| POST | `/api/v1/admin/fed/requests/:id/status` | Отметить входящую: `{status:"new"\|"done"\|"hidden"}` |
| DELETE | `/api/v1/admin/fed/requests/:id` | Удалить входящую заявку |

### Федерация: серверная роль (server only)

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/server/ping` | Проверка доступности (для кнопки «Тест») |
| POST | `/api/v1/server/search` | Поиск книг по запросу соседа. Тело: `{query, author, title}` → `{total, books:[{work_id, edition_id, author, title, year, formats}]}` |
| GET | `/api/v1/server/metadata/:id` | Метаданные издания (автор, произведение, издание, файлы) для импорта |
| GET | `/api/v1/server/download/:id` | Скачивание хранимого архива издания (как `.zip`) |
| POST | `/api/v1/server/requests/push` | Приём пакета заявок от соседа (дедуп по `(source_url, uid)`). Ответ: `{received, exists}` |
| POST | `/api/v1/server/book/offer` | Предложение книги по заявке (передаётся ссылка+метаданные; файл сосед забирает сам через `/server/metadata/:id` + `/server/download/:id`) |

### Запросы книг (editor+)

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/admin/suggestions` | Список заявок (?user=, ?bookname=, ?author=, ?hidden=) |
| POST | `/api/v1/admin/suggestions` | Создать/обновить предложения для заявки |
| GET | `/api/v1/admin/suggestions/readlist/:id` | Предложения конкретной заявки |
| DELETE | `/api/v1/admin/suggestions/:id` | Удалить предложение |
| POST | `/api/v1/admin/suggestions/import` | Импортировать файл и привязать к заявке (multipart) |

### Полка

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/shelf/count` | Количество книг на полке |
| PUT | `/api/v1/shelf/clear` | Очистить полку |
| GET | `/api/v1/shelf/download/:token` | Скачать книгу по токену (без авторизации) |

### Экспорт

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/export/json` | Экспорт каталога в JSON |
| GET | `/api/v1/export/csv` | Экспорт каталога в CSV |
| POST | `/api/v1/import/json` | Импорт каталога из JSON |

### Прочее

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/config` | Конфигурация (enable_delete) |
| GET | `/api/v1/apk/version` | Версия APK на сервере (`{"version":"...","apk_url":"..."}`) |
| GET | `/api/v1/apk/download` | Скачивание `library.apk` |
| GET | `/shelf/` | Страница «Общая полка» (HTML, без авторизации) |
| GET | `/debug/goroutines` | Дамп горутин (admin+editor) |

### OPDS

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/opds/catalog.xml` | Корневой каталог |
| GET | `/api/v1/opds/latest.xml` | Последние книги |
| GET | `/api/v1/opds/genres.xml` | Список жанров |
| GET | `/api/v1/opds/genre/:id.xml` | Книги жанра |
| GET | `/api/v1/opds/search.xml?q=` | Поиск |
| GET | `/api/v1/opds/book/:id` | Скачивание |

## Федерация библиотек (соседние серверы)

Несколько независимых инстансов приложения можно объединить в сеть, чтобы искать
и забирать книги друг у друга. Каждый инстанс хранит список «соседей»
(таблица `api_neighbours`) — доверенных серверов, к которым он обращается.

### Схема работы

```
┌────────────┐  (1) POST /admin/federation/search   ┌────────────┐
│  app1      │ ───────────────────────────────────▶ │  app2      │
│  (9091)    │      login (роль server) + search    │  (9092)    │
│            │ ◀───────────────────────────────────  │            │
│  результат │   books + metadata                   │            │
│            │                                      │            │
│            │  (2) POST /admin/federation/import   │            │
│            │ ───────────────────────────────────▶  │            │
│            │      metadata + download archive     │            │
└────────────┘                                      └────────────┘
```

1. **Администратор** на вкладке «Запросы» жмёт «Поиск 1й круг» (форма `Поиск 1й круг`).
2. Приложение входит на каждого соседа учётной записью с ролью `server`
   (пароль расшифровывается из БД через `NeighbourCrypto` — AES-256-GCM),
   проксирует поиск на его `POST /api/v1/server/search`.
3. Найденные книги можно скачать и импортировать себе через
   `POST /api/v1/admin/federation/import` (сохраняются исходные
   author/work/edition ID, при конфликтах администратор выбирает режим).

### Режим «Поиск 1й круг» (`stop_on_first=1`)

- Поиск идёт по всем соседям **последовательно** и останавливается на первом
  сервере, вернувшем хотя бы одну книгу.
- **Порядок обхода серверов рандомизируется** (генератор инициализируется
  текущим системным временем), чтобы нагрузка не падала всегда на один и тот же
  (первый в списке) сервер и не зависела от порядка `ORDER BY url`.
- Недоступный/ошибочный сосед не прерывает поиск: его ошибка попадает в
  `results[].error`, поиск продолжается со следующими серверами.
- Без `stop_on_first` (параллельный режим) запрашиваются все соседи, до 3
  запросов одновременно.

### Добавление соседа

Каждый сосед описывается строкой в таблице `api_neighbours`:

| Поле | Назначение |
|------|-----------|
| `url` | Базовый URL соседа (например `https://host:445`) |
| `username` / `password` | Учётная запись с ролью `server` на соседе |
| `server_cert` | Сертификат сервера соседа (PEM) — добавляется в trust pool для самоподписанных TLS |
| `client_cert` | Клиентский сертификат (combined cert+key PEM) для взаимной TLS-аутентификации |

Пароль хранится зашифрованным (AES-256-GCM); ключ генерируется при первом
использовании и хранится в таблице `settings` (`api_neighbours_key`), поэтому
шифрованные пароли переживают рестарт и смену `jwt_secret`.

Для двусторонней связи двух инстансов требуется:

1. Создать на каждом инстансе учётную запись с ролью `server`.
2. В nginx каждого инстанса включить `ssl_verify_client optional` с
   `ssl_client_certificate` (общий CA), чтобы запрашивать клиентские
   сертификаты у соседей (обычные браузеры при этом работают без сертификата).
3. На app1 в «Серверы» добавить app2 (и наоборот), указав учётные данные
   роли `server`, серверный сертификат и combined client cert+key PEM.
4. Проверить связность кнопкой **«Тест»** в списке серверов (или кнопкой
   «Тест» в фильтре — поочерёдная проверка всех серверов текущей страницы).

### Импорт книги с соседа (режимы конфликта)

Когда книга уже существует локально (совпал content-hash или ID), сервер
возвращает `409` с описанием конфликта, и администратор выбирает:

| Режим | Поведение |
|-------|-----------|
| **Перезаписать** (`overwrite`) | Заменяет локальные строки, сохраняя исходные ID (включая ID автора) |
| **Создать новую** (`create_new`) | Импортирует с новыми ID, переиспользуя найденных по заголовку авторов |
| Отменить | Ничего не меняет |

### Рассылка заявок на поиск книг

Пользователь отмечает в записи списка чтения «Ищу книгу: Да, по федерации»,
но сама заявка никуда не уходит автоматически. Администратор на вкладке
«Запросы» жмёт **«Запросить по федерации»** — заявка копируется в staging-таблицу
`fed_outgoing_requests` (статус `approved`), и фоновый распределитель
(`[federation] push_interval_sec`, по умолчанию 5 минут) рассылает её всем
соседям через `POST /api/v1/server/requests/push`. Кнопка одобрения сразу
досылает пакет вручную (`push-now`), не дожидаясь тика.

Каждая заявка несёт UUID `uid` (регенерируется при повторном одобрении) и
стабильный `read_list_id` — сосед дедуплицирует входящие по паре
`(source_url, uid)` и связывает полученную книгу с исходной записью списка.
Доставка фиксируется в `fed_request_outbox` (по строке на соседа): сообщение
отправляется один раз, недоступный сосед ретраится каждые
`retry_interval_sec` в течение `retry_window_sec`, потом помечается `failed`.
После отзыва одобрения pending/failed доставки отменяются.

Входящие заявки соседей видны администратору во вкладке **«Запросы соседей»**
(статусы new/done/hidden, можно удалить).

### Предложения книг пользователю («Предложить книгу»)

На входящую заявку соседа (вкладка «Запросы соседей») администратор может
кнопкой **«Предложить книгу»** предложить любую книгу из своего каталога.
Передаётся только ссылка и метаданные (`POST /api/v1/server/book/offer`,
роль `server`) — файл предлагающий **не пересылает**: получатель сам скачивает
архив через `/server/metadata/:id` + `/server/download/:id`.

Правило **first-offer-wins**: каждый оффер журналируется в таблицу
`fed_offers`, но привязывается к заявке пользователя (`read_list.book_id`)
только первый ответивший сервер — остальные книги просто попадают в каталог
библиотеки. После первой привязки рассылка этой заявки остальным соседам
прекращается, а на вкладке «Запросы» появляется статус **«книга получена»**.

Пользователь видит все полученные предложения в блоке **«Предложения
серверов»** формы редактирования записи списка чтения и может связать с
записью любой из них (`{offer_id}`), а не только первый пришедший. Локальное
предложение администратора (без федерации) тоже попадает в этот список с
пометкой «предложено администратором». В Android-приложении блок работает
офлайн: предложения кэшируются в SQLite и синхронизируются только в одну
сторону (сервер → клиент); связывание требует сети.

### Безопасность

- Все эндпоинты федерации доступны только роли `admin`.
- Запросы к соседу идут по TLS; самоподписанные сертификаты поддерживаются
  через `server_cert`, взаимная TLS — через `client_cert`.
- Внешние маршруты `/api/v1/server/*` доступны только роли `server`
  (middleware `serverOnlyMiddleware`).
- Пароли соседей не возвращаются API наружу: в ответах только флаг
  `has_password`.

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
├── apk/                       # Собранный APK + version.txt (автообновление)
├── backup/                    # Бэкапы БД перед миграциями
├── certres/                   # SSL-сертификаты и скрипты генерации
│   ├── generate-certs.sh      # CA + серверные сертификаты
│   ├── generate-keystore.sh   # Keystore для подписи APK
│   ├── generate-client-cert.sh # Клиентский сертификат (mTLS)
│   └── generate-nginx-certs.sh # fullchain.pem / privkey.pem (опционально)
├── db/scripts/                # Скрипты БД
├── logs/                      # Логи приложения
├── src/                       # Исходный код Go
│   ├── main.go                # Точка входа + все основные хендлеры
│   ├── auth.go                # Логин, регистрация, refresh
│   ├── admin.go               # Админ-хендлеры (пользователи, персоны, теги)
│   ├── reading.go             # Список чтения + статус чтения
│   ├── suggestion.go          # Запросы книг (suggestions) + зеркалирование локальных предложений в fed_offers
│   ├── neighbours.go          # Соседние серверы (api_neighbours CRUD + NeighbourCrypto)
│   ├── federation.go          # Федеративный поиск / импорт / тест связи / офферы / рассылка заявок
│   ├── fed_requests.go        # Фоновый распределитель заявок соседям (fed_outgoing_requests → push)
│   ├── server_api.go          # Серверная роль (/api/v1/server/*): поиск, метаданные, скачивание, приём заявок и офферов
│   ├── jwt.go                 # Генерация и валидация JWT + refresh-токены
│   ├── opds.go                # OPDS XML-каталог
│   ├── export.go              # Экспорт/импорт хендлеры
│   ├── main_test.go           # Тесты
│   ├── schema.sql             # Встраиваемая схема БД (go:embed)
│   ├── migration_*.sql        # Миграции (текущая версия: 5.7)
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
│   ├── css/
│   │   ├── style.css          # Основные стили (десктоп + @media mobile)
│   │   └── mobile.css         # Android-only стили (body.android)
│   ├── js/
│   │   ├── app.js             # SPA: вкладки Авторы, Книги, Жанры, Список чтения
│   │   ├── admin.js           # Админ-панель SPA (включая Запросы)
│   │   ├── auth.js            # Авторизация + Android-мост
│   │   ├── import.js          # Асинхронный импорт с polling прогресса
│   │   └── offline.js         # Офлайн-слой списка чтения (SQLite-мост)
│   └── service-worker.js      # Кэш статики для офлайн-режима
├── templates/
│   ├── index.html             # SPA главная страница (4 вкладки)
│   └── admin.html             # Админ-панель SPA (7 вкладок)
├── tempfld/                   # Директория загрузки/распаковки файлов
├── testdata/                  # Тестовые книги
├── src_android/               # Android WebView приложение
│   └── app/src/main/
│       ├── AndroidManifest.xml
│       ├── java/app/library/twa/   # MainActivity, ReadListDB (SQLite), TokenBridge
│       └── assets/www/             # Статика, встраиваемая в APK, offline.html
├── config.toml.example        # Пример конфига
├── env.example                # Пример .env для Docker
├── .apk.conf.example          # Шаблон конфигурации сборки APK
├── nginx.conf                 # Конфигурация nginx (HTTPS + прокси)
├── docker-compose.yml         # Docker Compose (БД + приложение)
├── docker-compose-nginx.yml   # Override: добавляет nginx (HTTPS)
├── Dockerfile                 # Многоступенчатая сборка Go
├── Dockerfile.all-in-one      # Всё в одном (БД + приложение)
├── Dockerfile.android         # Сборка APK
├── Dockerfile.android.sdk     # Образ SDK для сборки APK
├── startup.sh                 # Точка входа контейнера
├── build-android.sh           # Сборка APK (читает .apk.conf)
├── go.mod / go.sum
├── AGENTS.md                  # Инструкции для ассистентов AI
├── README.md
└── .gitignore
```

## nginx

Для продакшн-развертывания рекомендуется использовать nginx в качестве HTTPS-терминатора.

### Быстрый старт с nginx

```bash
# 1. CA + сертификаты
cd certres && ./generate-certs.sh && cd ..

# 2. Запуск
docker compose -f docker-compose.yml -f docker-compose-nginx.yml up -d --build
```

- HTTP (80) → редирект на HTTPS (443)
- HTTPS (443) → прокси на `app:8080`
- Проброс заголовка `X-Platform` для Android-детекции
- Security headers: HSTS, X-Content-Type-Options, X-Frame-Options

### Самостоятельная настройка

```nginx
server {
    listen 443 ssl;
    http2 on;
    server_name library.example.com;

    ssl_certificate     /path/to/certres/server.crt;
    ssl_certificate_key /path/to/certres/server.key;

    # Рекомендуется для продакшна:
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    location / {
        proxy_pass http://127.0.0.1:9091;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Platform $http_x_platform;
        proxy_buffering off;
    }
}
```

## Android WebView App

Мобильное приложение представляет собой Android WebView-обёртку SPA. Статика (CSS, JS, шаблоны) встраивается в APK при сборке, поэтому интерфейс загружается без сети; при недоступности сервера показывается офлайн-страница.

### Сборка APK

Конфигурация сборки — в файле `.apk.conf` (см. `.apk.conf.example`): целевой URL, пути к сертификатам, keystore, версия приложения.

```bash
# Сборка (debug + release)
./build-android.sh
```

APK будут в `android-apk/`:

```bash
adb install -r android-apk/app-debug.apk
```

### Офлайн-режим

- Все статические файлы (`static/css/`, `static/js/`, `templates/`, `service-worker.js`, `offline.html`) копируются в `src_android/app/src/main/assets/www/` при сборке.
- `shouldInterceptRequest()` отдаёт страницы из ассетов без обращения к сети; после логина принудительно загружает свежие файлы с сервера.
- Список чтения хранится локально в SQLite (`ReadListDB.java`) и синхронизируется с сервером в фоне.
- Офлайн-страница `offline.html` показывается, если сервер недоступен.

### Автообновление APK

APK-приложение автоматически проверяет наличие новой версии на сервере при запуске. Если версия новее — пользователю предлагается скачать и установить обновление.

#### Серверная часть

1. Создать директорию `apk/` в корне проекта (или настроить свой путь через `config.toml`).
2. Положить в неё файлы:

```
apk/
├── library.apk        # signed release APK
├── version.txt        # версия, например "1.2"
```

3. Добавить в `config.toml`:

```toml
[directories]
apk_dir = "apk"
```

**Эндпоинты** (требуют аутентификации):

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/apk/version` | Возвращает `{"version":"1.2","apk_url":"/api/v1/apk/download"}` |
| GET | `/api/v1/apk/download` | Отдаёт `library.apk` с `Content-Type: application/vnd.android.package-archive` |

Если `apk_dir` не задан или файлы отсутствуют — оба эндпоинта возвращают 404, клиент работает без изменений.

#### Клиентская часть (Android)

- При старте (через 2 секунды после загрузки страницы) JavaScript делает запрос к `/api/v1/apk/version`, сравнивает версию с `APK_VERSION_NAME` из `.apk.conf` (вшивается в `Config.java` при сборке).
- Если версия новее — показывается диалог `confirm()`: «Доступна новая версия (X.X). Скачать и установить?»
- При согласии APK скачивается через Java `HttpsURLConnection` (с поддержкой mTLS — используется `createEmbeddedCertSSLSocketFactory()`) и сохраняется в `getCacheDir()`.
- Установка запускается через `FileProvider` + `Intent.ACTION_VIEW`.
- Разрешение `REQUEST_INSTALL_PACKAGES` добавлено в манифест (install-time permission, не требует диалога пользователю). На Android 11+ может потребоваться однократно разрешить «Установка из неизвестных источников» в настройках приложения.

#### Версионирование

Версия приложения задаётся в файле `.apk.conf`:

```ini
APK_VERSION_NAME="1.0"
APK_VERSION_CODE=1
```

`APK_VERSION_NAME` вшивается в `Config.java` при сборке и должен совпадать со строкой в `version.txt` на сервере. Формат версии — произвольная последовательность чисел, разделённых точками (например `1.2.3`, `2.0`).

### Настройка mTLS

Для ограничения доступа к сайту с помощью клиентских сертификатов через nginx:

1. **Сгенерировать сертификаты**:

```bash
cd certres
./generate-certs.sh           # CA + сертификат сервера
./generate-keystore.sh        # Keystore для подписи APK
./generate-client-cert.sh     # Клиентский сертификат (для APK)
```

Скрипт `generate-client-cert.sh` проверяет наличие существующего сертификата и переиспользует его при повторном запуске.

2. **Настроить nginx**:

```nginx
server {
    listen 443 ssl;
    server_name library-app.local;

    ssl_certificate     /path/to/certres/server.crt;
    ssl_certificate_key /path/to/certres/server.key;

    ssl_client_certificate /path/to/certres/ca.crt;
    ssl_verify_client on;
    ssl_verify_depth 1;

    location / {
        proxy_pass http://127.0.0.1:9091;
        proxy_set_header X-Client-Cert $ssl_client_cert;
        proxy_set_header X-Client-Verify $ssl_client_verify;
    }
}
```

3. **Сборка APK** автоматически включает клиентский сертификат (`client.p12` → `res/raw/client_cert.p12`).

### Файлы сертификатов

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
│   ├── build.gradle           # Версия, подпись из build-extras.gradle
│   └── src/main/
│       ├── AndroidManifest.xml
│       ├── res/raw/
│       │   ├── ca_cert.crt    # Сертификат CA (из certres/)
│       │   └── client_cert.p12 # Клиентский сертификат (из certres/)
│       ├── assets/www/        # Статика, встраиваемая в APK (копируется при сборке)
│       │   ├── index.html, admin.html
│       │   ├── static/        # CSS, JS (включая offline.js)
│       │   ├── offline.html   # Офлайн-страница-запас
│       │   └── service-worker.js
│       └── java/app/library/twa/
│           ├── Application.java
│           ├── MainActivity.java  # WebView, мост к SQLite, mTLS, автообновление
│           └── ReadListDB.java    # SQLite-хранилище списка чтения
```

### Офлайн-синхронизация списка чтения

Список чтения доступен в Android-приложении без сети:

- Локальная копия хранится в SQLite (`readlist_items`), синхронизируется с таблицей `read_list` на сервере.
- Все изменения (создание, правка, удаление, смена статуса) сначала пишутся локально и попадают в очередь.
- Фоновая синхронизация: **push** (отправка локальных изменений с `updated_at > synced_at`) → **pull** (загрузка изменений с сервера). Запускается после каждой мутации и при старте приложения.
- Записи привязаны к текущему пользователю; при смене учётной записи чужие данные очищаются.
- Конфликты разрешаются по принципу last-write-wins: если серверная версия новее, клиент применяет серверную (`409` + `server_item`).

## Примечания

- **Схема БД** создаётся автоматически при первом запуске (embedded `schema.sql` + миграции). База данных также создаётся автоматически, если не существует.
- **LLM-вызовы** сериализованы через `sync.Mutex` — при массовом импорте PDF/DOC/DOCX файлы обрабатываются последовательно.
- **Дубликаты** проверяются по SHA-256 от содержимого до обращения к LLM.
- **Список чтения** отличается от полки: полка — общая для всех, список чтения — персональный для каждого пользователя.
- **Сортировка по году**: книги без года (включая year=0) всегда в конце списка (`NULLIF(year, 0) + NULLS LAST`).
- **Первый вход** создаёт администратора, если в БД нет пользователей.
- **Маршруты без аутентификации**: `GET /`, `GET /static/*`, `GET /favicon.ico`, `POST /api/v1/auth/login`, `POST /api/v1/auth/register`, `POST /api/v1/auth/refresh`, OPDS, `GET /api/v1/shelf/count`, `GET /api/v1/shelf/download/:token`, `GET /shelf/`.
- **Мобильная версия**: сервер определяет Android по заголовку `X-Platform` или User-Agent и добавляет `body class="android"` + `mobile.css`.
- **Полка при скачивании**: ZIP-архив распаковывается в `tempfld/shelf/{edition_id}/`, сервируется оригинальный файл; при убирании с полки — очищается.
- **Бэкап БД**: перед каждой миграцией создаётся `pg_dump` в `backup_dir` (`[directories] backup`). Если миграции ожидают применения, а `backup_dir` не задан — приложение отказывается запускаться. Файл бэкапа: `library_{current}_before_{target}.sql`, сохраняется после успешной миграции.
- **Список чтения**: UUID-идентификаторы, мягкое удаление (`deleted`), отметки времени `created_at`/`updated_at`/`synced_at` для офлайн-синхронизации, поле `looking_for` («ищу книгу»: локально / по федерации).
- **Автообновление APK**: сервер отдаёт версию из `apk/version.txt`; приложение сравнивает её со своей версией (`.apk.conf` → `Config.java`) и предлагает установить новую версию.
- **«Поиск 1й круг» (федерация) доступен только в веб-админке**, в Android-приложении (APK) кнопка скрыта — как и вкладки «Программы чтения» и «Серверы» (`body.android`-правила в `mobile.css`).

## Лицензия

MIT
