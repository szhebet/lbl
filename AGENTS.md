# Project Structure and Guidance

## Project Overview
This is a home library management application (website) built with Go and PostgreSQL.
The application is designed to run in a container on a Raspberry Pi.
It provides a RESTful API for managing a collection of books.

## Directory Structure
```
lbl/
├── bookarch/         # Directory for storing book archives (e.g., PDF, EPUB)
├── db/               # Database-related files
│   └── scripts/      # SQL scripts for database initialization and migrations
├── src/              # Go source code
│   ├── main.go       # Application entry point
│   ├── handlers/     # HTTP request handlers
│   ├── models/       # Data models and database interactions
│   ├── routes/       # Route definitions
│   ├── middleware/   # Custom middleware
│   ├── utils/        # Utility functions
│   └── config/       # Configuration files
├── tempfld/          # Temporary folder for file processing (upload, unpack, publish)
├── testdata/         # Sample book data for testing
├── Dockerfile        # Docker container definition
├── go.mod            # Go module dependencies
├── go.sum            # Go module dependency checksums
└── AGENTS.md         # This file: project guidance and structure
```

## Technology Stack
- **Language**: Go 1.25+
- **Web Framework**: Gin-Gonic
- **Database**: PostgreSQL 17
- **Driver**: github.com/lib/pq
- **Containerization**: Docker
- **Target Platform**: Raspberry Pi (ARM/Linux)

## Setup Instructions

### Prerequisites
1. Go 1.25+ installed
2. PostgreSQL 17+ installed and running
3. (Optional) Docker for containerization

### Local Development
1. Clone the repository (or copy the project files)
2. Initialize the database:
   ```bash
   # Connect to PostgreSQL
   sudo -u postgres psql
   # Then, in the psql prompt:
   \i /path/to/lbl/db/scripts/init_db.sql
   ```
3. Set environment variables (optional, defaults are provided):
   ```bash
   export DATABASE_URL="host=localhost port=5432 user=postgres password=postgres dbname=librarydb sslmode=disable"
   export PORT="8080"
   ```
4. Run the application:
   ```bash
   go run src/main.go
   ```
5. Access the API at `http://localhost:8080/api/v1/books`

### Running with Docker
1. Build the Docker image:
   ```bash
   docker build -t library-app .
   ```
2. Run the container:
   ```bash
   docker run -p 8080:8080 \
     -e DATABASE_URL="host=host.docker.internal port=5432 user=postgres password=postgres dbname=librarydb sslmode=disable" \
     library-app
   ```
   Note: For PostgreSQL running on the host, use `host.docker.internal` as the host from within the container.

## API Endpoints
| Method | Endpoint           | Description         |
|--------|--------------------|---------------------|
| GET    | `/api/v1/books`    | List all books      |
| POST   | `/api/v1/books`    | Create a new book   |
| GET    | `/api/v1/books/:id`| Get a specific book |
| PUT    | `/api/v1/books/:id`| Update a book       |
| DELETE | `/api/v1/books/:id`| Delete a book       |

## Development Workflow
1. Make changes to the Go source code in `src/`
2. Test locally by running `go run src/main.go`
3. Update database schema in `db/scripts/init_db.sql` as needed
4. For Docker changes, rebuild the image and test
5. Follow Go best practices: write tests, maintain clean code

## Future Enhancements
- Add authentication and authorization
- Implement file upload for book archives (store in `bookarch/`)
- Add search functionality
- Create a web frontend (HTML/CSS/JS)
- Implement book metadata extraction from files
- Add user profiles and reading lists
- Export/import functionality

## Notes for Raspberry Pi Deployment
1. Use the `golang:1.25-alpine` builder image for cross-compilation if needed
2. The final Docker image uses `alpine:latest` for minimal size
3. Ensure the container has access to persistent storage for:
   - Database (consider using a volume for PostgreSQL data)
   - Book archives (`bookarch/` directory)
   - Temporary processing (`tempfld/` directory)
## Testing
1. Always leave server running after testing for human check
2. Run application in service mode so it stays running after agent exits

## Running Server Persistently
To ensure the server remains running after agent session ends, use one of these methods:

### Using nohup (recommended for local development)
```bash
cd /home/sergey/git/aitest/agents/lbl
nohup ./library_app > library_app.log 2>&1 &
```
Log will be written to `library_app.log`.

### Using systemd (if available)
Create `/etc/systemd/system/library-app.service`:
```ini
[Unit]
Description=Library App
After=network.target postgresql.service

[Service]
Type=simple
User=root
WorkingDirectory=/home/sergey/git/aitest/agents/lbl
ExecStart=/home/sergey/git/aitest/agents/lbl/library_app
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```
Then enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable library-app
sudo systemctl start library-app
```

### Pre-built binary
The binary is already compiled as `library_app` in the project root. Build with:
```bash
go build -o library_app src/main.go
```
