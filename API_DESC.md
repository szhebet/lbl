## API
Не знаю, кто это все будет читать, но возможно для ИИ-доработки проекта впоследствии пригодится. 

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
