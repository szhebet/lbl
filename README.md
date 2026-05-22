# Library Management System (LMS)

Веб-приложение для управления домашней библиотекой книг с поддержкой OPDS-каталога для чтения на электронных читалках.

## Функциональность

### Основные возможности

- **Управление книгами**: добавление, редактирование, удаление книг с метаданными (название, год, ISBN, аннотация)
- **Авторы и жанры**: иерархическая структура авторов с книгами, категоризация по жанрам
- **Система тегов**: гибкая маркировка книг тегами
- **Обложки книг**: загрузка и отображение обложек
- **Импорт книг**: массовый импорт из файлов (FB2, EPUB, PDF) и директорий
- **Полка (избранное)**: возможность отмечать книги для быстрого доступа
- **Поиск**: полнотекстовый поиск по названию, автору, ISBN
- **OPDS-каталог**: доступ к библиотеке с читалок через OPDS-протокол

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

### Интерфейс

- Фильтрация по авторам и жанрам
- Отображение книг в иерархической структуре
- Страница "Полка" для избранных книг
- Встроенный OPDS-сервер для читалок

## Переменные окружения

| Переменная | Описание | Значение по умолчанию |
|------------|----------|----------------------|
| `POSTGRES_USER` | Пользователь PostgreSQL | `library` |
| `POSTGRES_PASSWORD` | Пароль PostgreSQL | `library_secret` |
| `POSTGRES_DB` | Имя базы данных | `librarydb` |
| `DATABASE_URL` | Строка подключения к БД | `host=localhost port=5432 user=library password=library_secret dbname=librarydb sslmode=disable` |
| `PORT` | Порт HTTP-сервера | `8080` |

## Сборка и запуск

### Локальная разработка

#### Требования

- Go 1.25+
- PostgreSQL 17+

#### Компиляция

```bash
# Клонирование репозитория
git clone <repository_url>
cd lbl

# Сборка
go build -o library_app src/main.go src/opds.go
```

#### Настройка базы данных

```bash
# Подключение к PostgreSQL
sudo -u postgres psql

# Создание пользователя и базы
CREATE USER library WITH PASSWORD 'library_secret';
CREATE DATABASE librarydb OWNER library;
\q

# Инициализация схемы
sudo -u postgres psql -d librarydb -f db/scripts/init_db.sql
```

#### Запуск

```bash
# С настройками по умолчанию
./library_app

# С кастомными параметрами
export DATABASE_URL="host=localhost port=5432 user=library password=library_secret dbname=librarydb sslmode=disable"
export PORT=8080
./library_app
```

Приложение доступно по адресу: http://localhost:8080

### Docker

#### Сборка образа

```bash
docker build -f Dockerfile.all-in-one -t library-app .
```

#### Запуск контейнера

```bash
# Минимальный запуск (все параметры по умолчанию)
docker run -d -p 8080:8080 --name library library-app

# С кастомными параметрами
docker run -d \
  -p 8080:8080 \
  -e POSTGRES_USER=myuser \
  -e POSTGRES_PASSWORD=mypassword \
  -e POSTGRES_DB=mydb \
  --name library \
  library-app
```

#### Проверка работы

```bash
# Проверка API
curl http://localhost:8080/api/v1/books

# Проверка OPDS-каталога
curl http://localhost:8080/api/v1/opds/catalog.xml

# Проверка счётчика полки
curl http://localhost:8080/api/v1/shelf/count
```

#### Остановка и удаление

```bash
docker stop library
docker rm library
```

## Структура проекта

```
lbl/
├── bookarch/         # Директория хранения книжных файлов
├── db/
│   └── scripts/
│       └── init_db.sql  # SQL-скрипт инициализации БД
├── src/
│   ├── main.go       # Основной код приложения
│   └── opds.go       # OPDS-функциональность
├── static/           # Статические файлы (CSS, JS)
├── templates/        # HTML-шаблоны
├── tempfld/          # Временная директория для обработки
├── Dockerfile.all-in-one  # Docker-образ (приложение + БД)
├── startup.sh        # Скрипт запуска
├── go.mod            # Зависимости Go
└── README.md         # Документация
```

## API эндпоинты

### Книги

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/books` | Список книг |
| POST | `/api/v1/books` | Создание книги |
| GET | `/api/v1/books/:id` | Информация о книге |
| PUT | `/api/v1/books/:id` | Обновление книги |
| DELETE | `/api/v1/books/:id` | Удаление книги |
| GET | `/api/v1/books/:id/extended` | Расширенная информация |
| PUT | `/api/v1/books/:id/extended` | Обновление расширенных данных |
| PUT | `/api/v1/books/:id/shelf` | Добавить/убрать с полки |
| GET | `/api/v1/books/:id/download` | Скачать файл книги |
| POST | `/api/v1/books/:id/cover` | Загрузить обложку |

### Другие ресурсы

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/authors` | Список авторов |
| GET | `/api/v1/genres` | Список жанров |
| POST | `/api/v1/genres` | Создание жанра |
| GET | `/api/v1/tags` | Список тегов |
| POST | `/api/v1/tags` | Создание тега |
| GET | `/api/v1/persons` | Персоны |
| GET | `/api/v1/languages` | Языки |
| GET | `/api/v1/shelf/count` | Количество книг на полке |
| PUT | `/api/v1/shelf/clear` | Очистить полку |
| POST | `/api/v1/import/file` | Импорт книги из файла |
| POST | `/api/v1/import/directory` | Импорт из директории |

### OPDS

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/opds/catalog.xml` | Корневой каталог |
| GET | `/api/v1/opds/latest.xml` | Последние книги |
| GET | `/api/v1/opds/genres.xml` | Жанры |
| GET | `/api/v1/opds/genre/:id.xml` | Книги жанра |
| GET | `/api/v1/opds/search.xml?q=` | Поиск |
| GET | `/api/v1/opds/book/:id` | Скачивание книги |

## Лицензия

MIT