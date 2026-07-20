// @title Library Management API
// @version 1.0
// @description API for managing a personal book library
// @host localhost:9091
// @BasePath /api/v1
package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"libapp/src/config"
	"libapp/src/utils"
)

//go:embed schema.sql
var embeddedSchema string

//go:embed migration_1.1.sql
var embeddedMigration11 string

//go:embed migration_2.0.sql
var embeddedMigration20 string

//go:embed migration_2.1.sql
var embeddedMigration21 string

//go:embed migration_2.2.sql
var embeddedMigration22 string

//go:embed migration_2.3.sql
var embeddedMigration23 string

//go:embed migration_2.4.sql
var embeddedMigration24 string

//go:embed migration_2.5.sql
var embeddedMigration25 string

//go:embed migration_3.0.sql
var embeddedMigration30 string

//go:embed migration_3.1.sql
var embeddedMigration31 string

//go:embed migration_4.0.sql
var embeddedMigration40 string

const currentDBVersion = "4.0"

type migration struct {
	Version     string
	Description string
	SQL         string
}

var migrations = []migration{
	{
		Version:     "1.0",
		Description: "Initial schema",
		SQL:         stripSchema(embeddedSchema),
	},
	{
		Version:     "1.1",
		Description: "Release 1.0",
		SQL:         stripSchema(embeddedMigration11),
	},
	{
		Version:     "2.0",
		Description: "Placeholder — future schema changes go here",
		SQL:         stripSchema(embeddedMigration20),
	},
	{
		Version:     "2.1",
		Description: "Add user_devices table",
		SQL:         stripSchema(embeddedMigration21),
	},
	{
		Version:     "2.2",
		Description: "Add user_books table",
		SQL:         stripSchema(embeddedMigration22),
	},
	{
		Version:     "2.3",
		Description: "Add read_list table",
		SQL:         stripSchema(embeddedMigration23),
	},
	{
		Version:     "2.4",
		Description: "Triggers for read_list ↔ user_books status sync",
		SQL:         stripSchema(embeddedMigration24),
	},
	{
		Version:     "2.5",
		Description: "Add refresh_tokens table",
		SQL:         stripSchema(embeddedMigration25),
	},
	{
		Version:     "3.0",
		Description: "Read list UUID PK + timestamps for offline sync",
		SQL:         stripSchema(embeddedMigration30),
	},
	{
		Version:     "3.1",
		Description: "Read list soft delete (deleted BOOLEAN)",
		SQL:         stripSchema(embeddedMigration31),
	},
	{
		Version:     "4.0",
		Description: "Settings table and backup infrastructure",
		SQL:         stripSchema(embeddedMigration40),
	},
}

func parseVersion(v string) (major, minor int) {
	parts := strings.SplitN(v, ".", 2)
	if len(parts) > 0 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return
}

func versionGreater(a, b string) bool {
	amaj, amin := parseVersion(a)
	bmaj, bmin := parseVersion(b)
	if amaj != bmaj {
		return amaj > bmaj
	}
	return amin > bmin
}

func runMigrations(db *sql.DB, cfg *config.Config) error {
	// Ensure db_version table exists
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS db_version (
		version     VARCHAR(20) NOT NULL,
		updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("failed to create db_version table: %w", err)
	}

	// Get current version
	var currentVer string
	err = db.QueryRow(`SELECT version FROM db_version ORDER BY updated_at DESC LIMIT 1`).Scan(&currentVer)
	if err == sql.ErrNoRows || err != nil {
		currentVer = "0.0"
		_, err = db.Exec(`INSERT INTO db_version (version) VALUES ('0.0')`)
		if err != nil {
			return fmt.Errorf("failed to insert initial version: %w", err)
		}
	}

	// Sort migrations by version
	sorted := make([]migration, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return versionGreater(sorted[j].Version, sorted[i].Version)
	})

	// Check if any pending migrations exist
	hasPending := false
	for _, m := range sorted {
		if m.SQL == "" {
			continue
		}
		if versionGreater(m.Version, currentVer) {
			hasPending = true
			break
		}
	}

	// Require backup_dir when pending migrations exist
	if hasPending {
		backupDir := cfg.Directories.Backup
		if backupDir == "" {
			return fmt.Errorf("BACKUP DIRECTORY NOT SET — ABORTING FOR SAFETY.\n" +
				"Before running migrations, configure [directories] backup_dir in config.toml\n" +
				"or set the LIBAPP_DIR_BACKUP environment variable.\n" +
				"Migration would modify the database without a safety backup.")
		}
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory %s: %w", backupDir, err)
		}
	}

	// Apply pending migrations
	applied := 0
	for _, m := range sorted {
		if m.SQL == "" {
			continue
		}
		if versionGreater(m.Version, currentVer) {
			// Create backup before this migration
			backupFile := filepath.Join(cfg.Directories.Backup,
				fmt.Sprintf("library_%s_before_%s.sql", currentVer, m.Version))
			log.Printf("Creating backup: %s", backupFile)
			if err := createDBBackup(cfg, backupFile); err != nil {
				return fmt.Errorf("backup failed before migration %s: %w", m.Version, err)
			}

			log.Printf("Running migration: %s — %s", m.Version, m.Description)
			_, err := db.Exec(m.SQL)
			if err != nil {
				return fmt.Errorf("migration %s failed: %w", m.Version, err)
			}
			// Update version after successful migration
			_, err = db.Exec(`UPDATE db_version SET version = $1, updated_at = NOW()`, m.Version)
			if err != nil {
				return fmt.Errorf("failed to update version after migration %s: %w", m.Version, err)
			}
			applied++

			currentVer = m.Version
		}
	}

	if applied > 0 {
		log.Printf("DB migration complete: %d script(s) applied", applied)
	} else {
		log.Printf("DB is up to date (version %s)", currentVer)
	}
	return nil
}

func createDBBackup(cfg *config.Config, filePath string) error {
	cmd := exec.Command("pg_dump",
		"-h", cfg.Database.Host,
		"-p", strconv.Itoa(cfg.Database.Port),
		"-U", cfg.Database.User,
		"-d", cfg.Database.Name,
		"-f", filePath,
		"--no-owner",
		"--no-privileges",
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+cfg.Database.Password)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_dump failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func createDatabase(cfg *config.Config) error {
	adminDSN := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.SSLMode,
	)
	adminDB, err := sql.Open("postgres", adminDSN)
	if err != nil {
		return fmt.Errorf("cannot connect to postgres database: %w", err)
	}
	defer adminDB.Close()

	_, err = adminDB.Exec(fmt.Sprintf("CREATE DATABASE %s ENCODING 'UTF8'", pq.QuoteIdentifier(cfg.Database.Name)))
	if err != nil {
		return fmt.Errorf("cannot create database: %w", err)
	}
	log.Printf("Database %q created", cfg.Database.Name)

	targetDSN := cfg.DSN()
	targetDB, err := sql.Open("postgres", targetDSN)
	if err != nil {
		return fmt.Errorf("cannot connect to new database: %w", err)
	}
	defer targetDB.Close()

	if _, err := targetDB.Exec(stripSchema(embeddedSchema)); err != nil {
		return fmt.Errorf("cannot apply schema: %w", err)
	}

	log.Printf("Schema applied to database %q", cfg.Database.Name)
	return nil
}

func stripSchema(schema string) string {
	var clean strings.Builder
	for _, line := range strings.Split(schema, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "\\") {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(t), "DROP DATABASE") ||
			strings.HasPrefix(strings.ToUpper(t), "CREATE DATABASE") {
			continue
		}
		clean.WriteString(line + "\n")
	}
	return clean.String()
}

func normalizeStr(s string) string {
	if s == "" {
		return s
	}
	return strings.ReplaceAll(s, "ё", "е")
}

func normalizeQuery(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(strings.ReplaceAll(s, "ё", "е"))
}

type ImportItem struct {
	File   string `json:"file"`
	Status string `json:"status"`
	Title  string `json:"title,omitempty"`
	Error  string `json:"error,omitempty"`
}

type ImportState struct {
	Running   bool         `json:"running"`
	Total     int          `json:"total"`
	Completed int          `json:"completed"`
	Errors    int          `json:"errors"`
	Items     []ImportItem `json:"items"`
	StartTime int64        `json:"start_time"`
}

type ImportManager struct {
	mu     sync.RWMutex
	state  ImportState
	cancel context.CancelFunc
	db     *sql.DB
	cfg    *config.Config
}

var importManager *ImportManager

func NewImportManager(db *sql.DB, cfg *config.Config) *ImportManager {
	return &ImportManager{db: db, cfg: cfg}
}

var supportedExts = map[string]bool{
	".fb2": true, ".fb2.zip": true, ".epub": true,
	".zip": true, ".pdf": true, ".doc": true, ".docx": true,
}

func isSupportedFile(name string) bool {
	low := strings.ToLower(name)
	if supportedExts[low] {
		return true
	}
	ext := filepath.Ext(low)
	if ext == ".zip" {
		base := strings.TrimSuffix(low, ".zip")
		if strings.HasSuffix(base, ".fb2") || strings.HasSuffix(base, ".pdf") || strings.HasSuffix(base, ".doc") || strings.HasSuffix(base, ".docx") || strings.HasSuffix(base, ".epub") {
			return true
		}
	}
	return supportedExts[ext]
}

func (im *ImportManager) Start(dirPath string, files []string) error {
	im.mu.Lock()
	defer im.mu.Unlock()
	if im.state.Running {
		return fmt.Errorf("import already in progress")
	}
	ctx, cancel := context.WithCancel(context.Background())
	im.cancel = cancel
	items := make([]ImportItem, len(files))
	for i, f := range files {
		items[i] = ImportItem{File: f, Status: "pending"}
	}
	im.state = ImportState{
		Running:   true,
		Total:     len(files),
		Completed: 0,
		Errors:    0,
		Items:     items,
		StartTime: time.Now().Unix(),
	}
	go im.run(ctx, dirPath)
	return nil
}

func (im *ImportManager) Cancel() {
	im.mu.RLock()
	cancel := im.cancel
	im.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

func (im *ImportManager) Status() ImportState {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.state
}

func (im *ImportManager) updateItemStatus(idx int, status, title, errMsg string) {
	im.mu.Lock()
	defer im.mu.Unlock()
	if idx < len(im.state.Items) {
		im.state.Items[idx].Status = status
		if title != "" {
			im.state.Items[idx].Title = title
		}
		if errMsg != "" {
			im.state.Items[idx].Error = errMsg
		}
		if status == "done" {
			im.state.Completed++
		} else if status == "error" {
			im.state.Errors++
		}
	}
}

func (im *ImportManager) run(ctx context.Context, dirPath string) {
	defer func() {
		im.mu.Lock()
		im.state.Running = false
		im.cancel = nil
		im.mu.Unlock()
		tmpDir := filepath.Clean(im.cfg.Directories.Temp)
		if strings.HasPrefix(filepath.Clean(dirPath), tmpDir) {
			os.RemoveAll(dirPath)
		}
	}()
	processDirectoryImport(ctx, dirPath, im.db, im.cfg, im.state.Items, im.updateItemStatus)
}

// BookDetails represents the denormalized book information from the view
type BookDetails struct {
	WorkID           int             `json:"work_id"`
	OriginalTitle    string          `json:"original_title"`
	OriginalLanguage sql.NullString  `json:"original_language,omitempty"`
	FirstPublished   sql.NullInt64   `json:"first_published,omitempty"`
	WorkType         sql.NullString  `json:"work_type,omitempty"`
	EditionID        int             `json:"edition_id"`
	EditionTitle     string          `json:"edition_title"`
	EditionLanguage  sql.NullString  `json:"edition_language,omitempty"`
	ISBN             sql.NullString  `json:"isbn,omitempty"`
	Publisher        sql.NullString  `json:"publisher,omitempty"`
	Year             sql.NullInt64   `json:"year,omitempty"`
	Pages            sql.NullInt64   `json:"pages,omitempty"`
	Series           sql.NullString  `json:"series,omitempty"`
	SeriesNumber     sql.NullString  `json:"series_number,omitempty"`
	Quality          sql.NullString  `json:"quality,omitempty"`
	Authors          sql.NullString  `json:"authors,omitempty"`
	Translators      sql.NullString  `json:"translators,omitempty"`
	Genres           sql.NullString  `json:"genres,omitempty"`
	AvailableFormats sql.NullString  `json:"available_formats,omitempty"`
	FormatCount      int             `json:"format_count,omitempty"` // This is a count, so it's never NULL
	PrimaryFilePath  sql.NullString  `json:"primary_file_path,omitempty"`
	ReadingProgress  sql.NullInt64   `json:"reading_progress,omitempty"`
	Rating           sql.NullInt64   `json:"rating,omitempty"`
	FinishedAt       sql.NullString  `json:"finished_at,omitempty"`
	CreatedAt        string          `json:"created_at,omitempty"` // This is from TIMESTAMP DEFAULT NOW(), so it's never NULL
	UpdatedAt        string          `json:"updated_at,omitempty"` // This is from TIMESTAMP DEFAULT NOW() and updated by trigger, so it's never NULL
	UploadDate       string          `json:"upload_date,omitempty"`
	OnShelf          bool            `json:"on_shelf"`
	ShelfOrder       int             `json:"shelf_order"`
}

// CreateBookRequest represents the request body for creating a book
type CreateBookRequest struct {
	Title       string `json:"title" binding:"required"`
	Author      string `json:"author" binding:"required"`
	ISBN        string `json:"isbn,omitempty"`
	PublishedYear int    `json:"published_year,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Description string `json:"description,omitempty"`
	Language    string `json:"language,omitempty"` // ISO 639-2/B code
}

// UpdateBookRequest represents the request body for updating a book
type UpdateBookRequest struct {
	Title       *string `json:"title,omitempty"`
	Author      *string `json:"author,omitempty"`
	ISBN        *string `json:"isbn,omitempty"`
	PublishedYear *int    `json:"published_year,omitempty"` // Pointer to distinguish between 0 and unset
	Genre       *string `json:"genre,omitempty"`
	Description *string `json:"description,omitempty"`
	Language    *string `json:"language,omitempty"` // ISO 639-2/B code
	Publisher   *string `json:"publisher,omitempty"`
}

func internalError(c *gin.Context, err error) {
	log.Printf("Internal error: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Внутренняя ошибка сервера"})
}

func normalizeYear(year sql.NullInt64) sql.NullInt64 {
	if year.Valid && year.Int64 == 0 {
		return sql.NullInt64{Valid: false}
	}
	return year
}

func getConfig(c *gin.Context) *config.Config {
	if cfg, exists := c.Get("config"); exists {
		return cfg.(*config.Config)
	}
	return config.DefaultConfig()
}

func recognizeBook(text string, cfg *config.Config) *utils.LLMResult {
	if cfg.LLM.BaseURL == "" {
		return nil
	}
	if len(text) > 2000 {
		text = text[:2000]
	}
	return utils.RecognizeBook(text, cfg.LLM.BaseURL, cfg.LLM.Model, cfg.LLM.Token, cfg.LLM.Prompt, cfg.LLM.Prompt2, cfg.LLM.Timeout, true)
}

// ---- Rate limiting ----

type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string]int
	window   time.Duration
	limit    int
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		attempts: make(map[string]int),
		window:   window,
		limit:    limit,
	}
	go func() {
		for {
			time.Sleep(window)
			rl.mu.Lock()
			rl.attempts = make(map[string]int)
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.attempts[key]++
	return rl.attempts[key] <= rl.limit
}

var loginLimiter = newRateLimiter(10, time.Minute)
var registerLimiter = newRateLimiter(5, time.Minute)
var writeLimiter = newRateLimiter(60, time.Minute)

func rateLimitMiddleware(rl *rateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if uid, exists := c.Get("user_id"); exists {
			key = fmt.Sprintf("u:%d", uid.(int))
		}
		if !rl.allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Слишком много запросов. Попробуйте позже."})
			return
		}
		c.Next()
	}
}

// ---- Security headers middleware ----

func securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "same-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'")
		c.Header("Strict-Transport-Security", "max-age=31533600; includeSubDomains")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}

// ---- Min password length ----

const minPasswordLength = 6

func main() {
	cfg, err := config.Load("")
	if err != nil {
		log.Fatal("Error loading config: ", err)
	}

	log.Printf("Connecting to database: %s", cfg.DSNDisplay())

	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "3D000" {
			log.Printf("Database %q does not exist, creating...", cfg.Database.Name)
			if err := createDatabase(cfg); err != nil {
				log.Fatal("Failed to create database: ", err)
			}
			db.Close()
			db, err = sql.Open("postgres", cfg.DSN())
			if err != nil {
				log.Fatal("Error reconnecting: ", err)
			}
			if err := db.Ping(); err != nil {
				log.Fatal("Error pinging after creation: ", err)
			}
		} else {
			log.Fatal("Error pinging database: ", err)
		}
	}

	log.Println("Connected to database")

	if err := runMigrations(db, cfg); err != nil {
		log.Fatal("Migration failed: ", err)
	}

	importManager = NewImportManager(db, cfg)

	initJWTSecret(cfg.Server.JWTSecret)
	initTokenTTL(cfg.Server.TokenTTL)

	if cfg.Server.LogLevel != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), securityHeadersMiddleware())
	r.MaxMultipartMemory = 100 << 20 // 100 MB max for all uploads

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("config", cfg)
		c.Next()
	})

// Auth routes (rate-limited, no JWT required)
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/login", func(c *gin.Context) {
			ip := c.ClientIP()
			if !loginLimiter.allow(ip) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Слишком много запросов. Попробуйте позже."})
				return
			}
			loginUser(db)(c)
		})
		auth.POST("/register", func(c *gin.Context) {
			ip := c.ClientIP()
			if !registerLimiter.allow(ip) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Слишком много запросов. Попробуйте позже."})
				return
			}
			createUser(db)(c)
		})
		auth.POST("/refresh", func(c *gin.Context) {
			ip := c.ClientIP()
			if !loginLimiter.allow(ip) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Слишком много запросов. Попробуйте позже."})
				return
			}
			refreshToken(db)(c)
		})
	}

	// Read-only guest routes
	api := r.Group("/api/v1")
	{
		setupOPDSRoutes(api, db)

		api.GET("/books", getBooks(db))
		api.GET("/books/search", searchBooks(db))
		api.GET("/books/:id", getBook(db))
		api.GET("/books/:id/extended", getBookExtended(db))
		api.GET("/authors", getAuthors(db))
		api.GET("/genres", getGenres(db))
		api.GET("/genres/tree", getGenreTree(db))
		api.GET("/genres/:id/authors", getGenreAuthors(db))
		api.GET("/tags", getTags(db))
		api.GET("/persons", getPersons(db))
		api.GET("/persons/:id", getPerson(db))
		api.GET("/languages", getLanguages(db))
		api.GET("/config", getAppConfig())
		api.GET("/import/status", getImportStatus())
		api.GET("/books/:id/download", downloadBook(db))
		api.GET("/shelf/count", getShelfCount(db))
	}

	// Write API routes (require auth)
	write := r.Group("/api/v1")
	write.Use(requireAuthMiddleware())
	write.Use(rateLimitMiddleware(writeLimiter))
	{
		write.POST("/books", createBook(db))
		write.PUT("/books/:id", updateBook(db))
		write.DELETE("/books/:id", deleteBook(db))
		write.PUT("/books/:id/extended", updateBookExtended(db))
		write.PUT("/persons/:id", updatePerson(db))
		write.POST("/genres", createGenre(db))
		write.PUT("/genres/:id", updateGenre(db))
		write.DELETE("/genres/:id", deleteGenre(db))
		write.POST("/tags", createTag(db))
		write.POST("/import/file", importBookFile(db))
		write.POST("/import/upload", importUploadFiles())
		write.POST("/import/directory", startImport())
		write.POST("/import/cancel", cancelImport())
		write.POST("/books/:id/cover", uploadCover(db))
		write.PUT("/books/:id/shelf", updateBookShelf(db))
		write.PUT("/shelf/clear", clearShelf(db))

		// User-book status
		write.GET("/user/books", listUserBooks(db))
		write.GET("/user/books/:edition_id", getUserBook(db))
		write.PUT("/user/books/:edition_id", setUserBook(db))

		// Read list
		write.GET("/user/readlist", getReadListItems(db))
		write.POST("/user/readlist", createReadListItem(db))
		write.GET("/user/readlist/names", getReadListNames(db))
		write.PUT("/user/readlist/:id", updateReadListItem(db))
		write.DELETE("/user/readlist/:id", deleteReadListItem(db))
	}

	// Admin API routes (require editor+ authentication)
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	{
		// User management — admin only
		adminUsers := admin.Group("/users")
		adminUsers.Use(adminOnlyMiddleware())
		{
			adminUsers.GET("", adminGetUsers(db))
			adminUsers.GET("/:id", adminGetUser(db))
			adminUsers.POST("", adminCreateUser(db))
			adminUsers.PUT("/:id", adminUpdateUser(db))
			adminUsers.DELETE("/:id", adminDeleteUser(db))
		}
		admin.GET("/persons", adminGetPersons(db))
		admin.POST("/persons", adminCreatePerson(db))
		admin.PUT("/persons/:id", adminUpdatePerson(db))
		admin.DELETE("/persons/:id", adminDeletePerson(db))
		admin.GET("/tags", adminGetTags(db))
		admin.PUT("/tags/:id", adminUpdateTag(db))
		admin.DELETE("/tags/:id", adminDeleteTag(db))
		admin.GET("/genres", adminGetGenres(db))
		admin.GET("/settings", adminGetSettings(db))
		admin.PUT("/settings", adminUpdateSettings(db))
	}

	// Serve static files with cache-busting headers for JS
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/static/js/") || strings.HasPrefix(c.Request.URL.Path, "/static/css/") || c.Request.URL.Path == "/" || c.Request.URL.Path == "/admin" || c.Request.URL.Path == "/service-worker.js" {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}
		c.Next()
	})
	r.Static("/static", "./static")

	// Favicon
	r.StaticFile("/favicon.ico", "./static/favicon.svg")

	// Service Worker (served at root for default scope "/")
	r.GET("/service-worker.js", func(c *gin.Context) {
		c.Header("Service-Worker-Allowed", "/")
		c.File("./static/service-worker.js")
	})

	// Serve templates with mobile platform detection
	mobileTopBarIndex := `<div class="mobile-top-bar">
    <a href="/admin" class="mobile-admin-btn" title="Администрирование">А</a>
    <span class="mobile-top-spacer"></span>
    <button class="mobile-user-btn" id="mobileUserBtn" title="Пользователь">☰</button>
</div>`
	mobileTopBarAdmin := `<div class="mobile-top-bar">
    <a href="/" class="mobile-back-btn" title="Назад к библиотеке">←</a>
    <span class="mobile-top-title">Админ</span>
    <span class="mobile-top-spacer"></span>
    <button class="mobile-user-btn" id="mobileUserBtn" title="Пользователь">☰</button>
</div>`
	serveIndex := serveTemplate("./templates/index.html", "index.html", mobileTopBarIndex)
	serveAdmin := serveTemplate("./templates/admin.html", "admin.html", mobileTopBarAdmin)
	// isMobilePlatform checks if the request comes from a mobile app
	isMobilePlatform := func(c *gin.Context) bool {
		if c.GetHeader("X-Platform") == "android" {
			return true
		}
		ua := c.GetHeader("User-Agent")
		return strings.Contains(ua, "Android") || strings.Contains(ua, "Mobile")
	}
	r.GET("/", func(c *gin.Context) {
		serveIndex(c, isMobilePlatform(c))
	})
	r.GET("/admin", func(c *gin.Context) {
		serveAdmin(c, isMobilePlatform(c))
	})
	r.GET("/shelf/", getShelfPage(db))
	r.GET("/api/v1/shelf/clear", clearShelf(db))

	// Debug endpoints (admin only)
	r.GET("/debug/goroutines", adminAuthMiddleware(), func(c *gin.Context) {
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		c.Data(http.StatusOK, "text/plain; charset=utf-8", buf[:n])
	})

	addr := fmt.Sprintf("%s:%d", cfg.Server.Bind, cfg.Server.Port)

	// HTTPS (TLS) is terminated by nginx — no TLS server in Go.
	// If you need direct HTTPS without nginx, uncomment the block below.
	/*
	tlsCertFile := "./certres/server.crt"
	tlsKeyFile := "./certres/server.key"
	if _, err := os.Stat(tlsCertFile); err == nil {
		tlsAddr := fmt.Sprintf("%s:9443", cfg.Server.Bind)
		log.Printf("Starting HTTPS on %s\n", tlsAddr)
		go func() {
			tlsConfig := &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
			server := &http.Server{
				Addr:      tlsAddr,
				Handler:   r.Handler(),
				TLSConfig: tlsConfig,
			}
			if err := server.ListenAndServeTLS(tlsCertFile, tlsKeyFile); err != nil {
				log.Printf("HTTPS server error: %v\n", err)
			}
		}()
	} else {
		log.Printf("TLS cert not found at %s, HTTPS disabled\n", tlsCertFile)
	}
	*/

	log.Printf("Starting HTTP on %s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}

// detectImageType validates the image magic bytes and returns the detected
// MIME type, or "" if the data is not a supported image.
func detectImageType(data []byte) string {
	if len(data) < 12 {
		return ""
	}
	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A {
		return "image/png"
	}
	// WEBP: "RIFF" .... "WEBP"
	if data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' &&
		data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P' {
		return "image/webp"
	}
	return ""
}

func uploadCover(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := getConfig(c)

		editionID := c.Param("id")

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)

		file, _, err := c.Request.FormFile("cover")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
			return
		}
		defer file.Close()

		// Read into memory to validate magic bytes (don't trust Content-Type header).
		data, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read cover"})
			return
		}
		contentType := detectImageType(data)
		if contentType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported file type. Use JPEG, PNG, or WebP"})
			return
		}

		var editionExists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM editions WHERE id = $1)", editionID).Scan(&editionExists)
		if err != nil || !editionExists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Edition not found"})
			return
		}

		// Remove any previously stored cover so a changed extension doesn't orphan it.
		var oldCover sql.NullString
		db.QueryRow("SELECT cover_path FROM editions WHERE id = $1", editionID).Scan(&oldCover)
		if oldCover.Valid && oldCover.String != "" {
			os.Remove(oldCover.String)
		}

		coverDir := filepath.Join(cfg.Directories.Bookarch, "covers")
		os.MkdirAll(coverDir, 0755)

		ext := ".jpg"
		if contentType == "image/png" {
			ext = ".png"
		} else if contentType == "image/webp" {
			ext = ".webp"
		}
		filename := fmt.Sprintf("cover_%s%s", editionID, ext)
		coverPath := filepath.Join(coverDir, filename)

		if err := os.WriteFile(coverPath, data, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save cover"})
			return
		}

		_, err = db.Exec("UPDATE editions SET cover_path = $1 WHERE id = $2", coverPath, editionID)
		if err != nil {
			os.Remove(coverPath)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update edition"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":    "Cover uploaded successfully",
			"cover_url":  "/static/covers/" + filename,
			"edition_id": editionID,
		})
	}
}

func getBooks(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authorFilter := c.Query("author")
		bookFilter := c.Query("book")
		genreFilter := c.Query("genre")
		sortBy := c.DefaultQuery("sort_by", "original_title")
		sortOrder := c.DefaultQuery("sort_order", "asc")
		limit := c.DefaultQuery("limit", "50")
		offset := c.DefaultQuery("offset", "0")

		allowedSorts := map[string]string{
			"original_title":    "original_title",
			"upload_date":       "upload_date",
			"authors":          "authors",
			"available_formats": "available_formats",
			"year":             "NULLIF(year, 0)",
		}
		sortCol, ok := allowedSorts[sortBy]
		if !ok {
			sortCol = "original_title"
		}
		if sortOrder != "desc" {
			sortOrder = "asc"
		}
		nullsLast := ""
		if sortBy == "year" {
			nullsLast = " NULLS LAST"
		}

		whereClause := " WHERE 1=1"
		args := []interface{}{}
		argNum := 1

		if authorFilter != "" || bookFilter != "" {
			conditions := []string{}
			if authorFilter != "" {
				q := "%" + normalizeQuery(authorFilter) + "%"
				conditions = append(conditions, fmt.Sprintf("p.lower_fio LIKE $%d", argNum))
				args = append(args, q)
				argNum++
			}
			if bookFilter != "" {
				q := "%" + normalizeQuery(bookFilter) + "%"
				conditions = append(conditions, fmt.Sprintf("w.lower_original_title LIKE $%d", argNum))
				args = append(args, q)
				argNum++
			}
			whereClause += fmt.Sprintf(` AND edition_id IN (
				SELECT DISTINCT e.id FROM editions e
				JOIN works w ON w.id = e.work_id
				LEFT JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
				LEFT JOIN persons p ON p.id = wc.person_id
				WHERE %s
			)`, strings.Join(conditions, " OR "))
		}

		if genreFilter != "" {
			whereClause += fmt.Sprintf(" AND genres ILIKE $%d", argNum)
			args = append(args, "%"+genreFilter+"%")
			argNum++
		}

		dateFrom := c.Query("date_from")
		dateTo := c.Query("date_to")
		if dateFrom != "" {
			whereClause += fmt.Sprintf(" AND upload_date >= $%d::date", argNum)
			args = append(args, dateFrom)
			argNum++
		}
		if dateTo != "" {
			whereClause += fmt.Sprintf(" AND upload_date < ($%d::date + interval '1 day')", argNum)
			args = append(args, dateTo)
			argNum++
		}

		// Count total matching
		var total int
		countQuery := "SELECT COUNT(*) FROM book_details" + whereClause
		err := db.QueryRow(countQuery, args...).Scan(&total)
		if err != nil {
			internalError(c, err)
			return
		}

		// Main query with sort and pagination
		query := fmt.Sprintf(`
			SELECT 
				work_id, original_title, original_language, first_published, work_type,
				edition_id, edition_title, edition_language, isbn, publisher, year, pages,
				series, series_number, quality, authors, translators, genres,
				available_formats, format_count, primary_file_path,
				reading_progress, rating, finished_at, created_at, updated_at, upload_date,
				on_shelf, shelf_order
			FROM book_details%s
			ORDER BY %s %s%s
			LIMIT $%d OFFSET $%d
		`, whereClause, sortCol, sortOrder, nullsLast, argNum, argNum+1)
		queryArgs := append(args, limit, offset)

		rows, err := db.Query(query, queryArgs...)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		var books []BookDetails
		for rows.Next() {
			var book BookDetails
			if err := rows.Scan(
				&book.WorkID, &book.OriginalTitle, &book.OriginalLanguage, &book.FirstPublished, &book.WorkType,
				&book.EditionID, &book.EditionTitle, &book.EditionLanguage, &book.ISBN, &book.Publisher, &book.Year, &book.Pages,
				&book.Series, &book.SeriesNumber, &book.Quality, &book.Authors, &book.Translators, &book.Genres,
				&book.AvailableFormats, &book.FormatCount, &book.PrimaryFilePath,
				&book.ReadingProgress, &book.Rating, &book.FinishedAt, &book.CreatedAt, &book.UpdatedAt, &book.UploadDate,
				&book.OnShelf, &book.ShelfOrder,
			); err != nil {
				internalError(c, err)
				return
			}
			book.Year = normalizeYear(book.Year)
			books = append(books, book)
		}

		if err = rows.Err(); err != nil {
			internalError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"total":  total,
			"limit":  limit,
			"offset": offset,
			"books":  books,
		})
	}
}

func searchBooks(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("q")
		genre := c.Query("genre")
		language := c.Query("language")
		yearFrom := c.Query("year_from")
		yearTo := c.Query("year_to")
		limit := c.DefaultQuery("limit", "20")
		offset := c.DefaultQuery("offset", "0")

		sqlQuery := `
			SELECT 
				work_id, original_title, original_language, first_published, work_type,
				edition_id, edition_title, edition_language, isbn, publisher, year, pages,
				series, series_number, quality, authors, translators, genres,
				available_formats, format_count, primary_file_path,
				reading_progress, rating, finished_at, created_at, updated_at, upload_date,
				on_shelf, shelf_order
			FROM book_details
			WHERE 1=1`
		
		args := []interface{}{}
		argIndex := 1

		if query != "" {
			query = "%" + normalizeQuery(query) + "%"
			sqlQuery += fmt.Sprintf(` AND edition_id IN (
				SELECT DISTINCT e.id FROM editions e
				JOIN works w ON w.id = e.work_id
				LEFT JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
				LEFT JOIN persons p ON p.id = wc.person_id
				WHERE w.lower_original_title LIKE $%d OR p.lower_fio LIKE $%d
			)`, argIndex, argIndex+1)
			args = append(args, query, query)
			argIndex += 2
		}

		if genre != "" {
			sqlQuery += fmt.Sprintf(" AND genres ILIKE $%d", argIndex)
			args = append(args, "%"+genre+"%")
			argIndex++
		}

		if language != "" {
			sqlQuery += fmt.Sprintf(" AND (original_language = $%d OR edition_language = $%d)", argIndex, argIndex)
			args = append(args, language)
			argIndex++
		}

		if yearFrom != "" {
			sqlQuery += fmt.Sprintf(" AND first_published >= $%d", argIndex)
			args = append(args, yearFrom)
			argIndex++
		}

		if yearTo != "" {
			sqlQuery += fmt.Sprintf(" AND first_published <= $%d", argIndex)
			args = append(args, yearTo)
			argIndex++
		}

		// Build WHERE clause for count query (same conditions, no LIMIT/OFFSET)
		whereClause := " WHERE 1=1"
		countArgs := []interface{}{}
		ci := 1
		if query != "" {
			whereClause += fmt.Sprintf(` AND edition_id IN (
				SELECT DISTINCT e.id FROM editions e
				JOIN works w ON w.id = e.work_id
				LEFT JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
				LEFT JOIN persons p ON p.id = wc.person_id
				WHERE w.lower_original_title LIKE $%d OR p.lower_fio LIKE $%d
			)`, ci, ci+1)
			countArgs = append(countArgs, query, query)
			ci += 2
		}
		if genre != "" {
			whereClause += fmt.Sprintf(" AND genres ILIKE $%d", ci)
			countArgs = append(countArgs, "%"+genre+"%")
			ci++
		}
		if language != "" {
			whereClause += fmt.Sprintf(" AND (original_language = $%d OR edition_language = $%d)", ci, ci)
			countArgs = append(countArgs, language)
			ci++
		}
		if yearFrom != "" {
			whereClause += fmt.Sprintf(" AND first_published >= $%d", ci)
			countArgs = append(countArgs, yearFrom)
			ci++
		}
		if yearTo != "" {
			whereClause += fmt.Sprintf(" AND first_published <= $%d", ci)
			countArgs = append(countArgs, yearTo)
			ci++
		}

		var total int
		if err := db.QueryRow("SELECT COUNT(*) FROM book_details"+whereClause, countArgs...).Scan(&total); err != nil {
			internalError(c, err)
			return
		}

		sqlQuery += " ORDER BY original_title LIMIT $" + strconv.Itoa(argIndex) + " OFFSET $" + strconv.Itoa(argIndex+1)
		args = append(args, limit, offset)

		rows, err := db.Query(sqlQuery, args...)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		var books []BookDetails
		for rows.Next() {
			var book BookDetails
			if err := rows.Scan(
				&book.WorkID, &book.OriginalTitle, &book.OriginalLanguage, &book.FirstPublished, &book.WorkType,
				&book.EditionID, &book.EditionTitle, &book.EditionLanguage, &book.ISBN, &book.Publisher, &book.Year, &book.Pages,
				&book.Series, &book.SeriesNumber, &book.Quality, &book.Authors, &book.Translators, &book.Genres,
				&book.AvailableFormats, &book.FormatCount, &book.PrimaryFilePath,
				&book.ReadingProgress, &book.Rating, &book.FinishedAt, &book.CreatedAt, &book.UpdatedAt, &book.UploadDate,
				&book.OnShelf, &book.ShelfOrder,
			); err != nil {
				internalError(c, err)
				return
			}
			book.Year = normalizeYear(book.Year)
			books = append(books, book)
		}

		c.JSON(http.StatusOK, gin.H{
			"total": total,
			"limit": limit,
			"offset": offset,
			"books": books,
		})
	}
}

// getBook returns a handler function that gets a specific book by edition ID
func getBook(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		// Query book from the book_details view by edition ID
		var book BookDetails
		err := db.QueryRow(`
			SELECT 
				work_id, original_title, original_language, first_published, work_type,
				edition_id, edition_title, edition_language, isbn, publisher, year, pages,
				series, series_number, quality, authors, translators, genres,
				available_formats, format_count, primary_file_path,
				reading_progress, rating, finished_at, created_at, updated_at, upload_date
			FROM book_details
			WHERE edition_id = $1
		`, id).Scan(
			&book.WorkID, &book.OriginalTitle, &book.OriginalLanguage, &book.FirstPublished, &book.WorkType,
			&book.EditionID, &book.EditionTitle, &book.EditionLanguage, &book.ISBN, &book.Publisher, &book.Year, &book.Pages,
			&book.Series, &book.SeriesNumber, &book.Quality, &book.Authors, &book.Translators, &book.Genres,
			&book.AvailableFormats, &book.FormatCount, &book.PrimaryFilePath,
			&book.ReadingProgress, &book.Rating, &book.FinishedAt, &book.CreatedAt, &book.UpdatedAt, &book.UploadDate,
		)
		book.Year = normalizeYear(book.Year)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
				return
			}
			internalError(c, err)
			return
		}

		c.JSON(http.StatusOK, book)
	}
}

// createBook returns a handler function that creates a new book
func createBook(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateBookRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			internalError(c, err)
			return
		}
		defer func() {
			if err != nil {
				tx.Rollback()
			} else {
				tx.Commit()
			}
		}()

		// Get or create language
		var languageID string
		if req.Language == "" {
			languageID = "eng" // Default to English
		} else {
			languageID = req.Language
		}

		// Ensure language exists
		var langExists bool
		err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM languages WHERE code = $1)", languageID).Scan(&langExists)
		if err != nil {
			internalError(c, err)
			return
		}
		if !langExists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid language code"})
			return
		}

		// Insert work
		var workID int
		err = tx.QueryRow(`
			INSERT INTO works (original_title, original_language, first_published, work_type, annotation, word_count)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id
		`, req.Title, languageID, req.PublishedYear, "novel", req.Description, 0).Scan(&workID)
		if err != nil {
			internalError(c, err)
			return
		}

		// Get or create author person (using first_name and last_name from Author field)
		// Parse author name - assume format "LastName FirstName" or just "LastName"
		var firstName, lastName string
		authorStr := strings.TrimSpace(req.Author)
		if authorStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Author is required"})
			return
		}

		// Treat entire author string as last name (common for single names like "Pushkin" or "Ivan Turgenev")
		// If there's a comma, assume "Last, First" format
		if strings.Contains(authorStr, ",") {
			parts := strings.Split(authorStr, ",")
			lastName = strings.TrimSpace(parts[0])
			if len(parts) > 1 {
				firstName = strings.TrimSpace(parts[1])
			}
		} else {
			lastName = authorStr
			firstName = ""
		}

		var personID int
		err = tx.QueryRow(`
			INSERT INTO persons (last_name, first_name)
			VALUES ($1, $2)
			ON CONFLICT (first_name, last_name) DO NOTHING
			RETURNING id
		`, lastName, firstName).Scan(&personID)
		if err != nil {
			// If INSERT returned nothing due to conflict, get existing ID
			err = tx.QueryRow(`
				SELECT id FROM persons WHERE last_name = $1 AND (first_name = $2 OR (first_name IS NULL AND $2 = ''))
			`, lastName, firstName).Scan(&personID)
			if err != nil {
				internalError(c, err)
				return
			}
		}

		// Link work to author
		_, err = tx.Exec(`
			INSERT INTO work_contributors (work_id, person_id, role)
			VALUES ($1, $2, 'author')
			ON CONFLICT DO NOTHING
		`, workID, personID)
		if err != nil {
			internalError(c, err)
			return
		}

		// Insert edition
		var editionID int
		err = tx.QueryRow(`
			INSERT INTO editions (work_id, title, language, publisher, year, city, pages, annotation, quality, source, upload_date)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
			RETURNING id
		`, workID, req.Title, languageID, "Self-published", req.PublishedYear, "Self-published", 0, req.Description, "good", "manual").Scan(&editionID)
		if err != nil {
			internalError(c, err)
			return
		}

		// Get or create format (default to EPUB if not specified)
		var formatID int
		err = tx.QueryRow("SELECT id FROM formats WHERE name = 'EPUB'").Scan(&formatID)
		if err != nil {
			internalError(c, err)
			return
		}

		// Insert edition file
		_, err = tx.Exec(`
			INSERT INTO edition_files (edition_id, format_id, file_path, is_primary)
			VALUES ($1, $2, $3, $4)
		`, editionID, formatID, "/books/manual_upload.epub", true)
		if err != nil {
			internalError(c, err)
			return
		}

		// Return created book
		var book BookDetails
		err = tx.QueryRow(`
			SELECT 
				work_id, original_title, original_language, first_published, work_type,
				edition_id, edition_title, edition_language, isbn, publisher, year, pages,
				series, series_number, quality, authors, translators, genres,
				available_formats, format_count, primary_file_path,
				reading_progress, rating, finished_at, created_at, updated_at
			FROM book_details
			WHERE edition_id = $1
		`, editionID).Scan(
			&book.WorkID, &book.OriginalTitle, &book.OriginalLanguage, &book.FirstPublished, &book.WorkType,
			&book.EditionID, &book.EditionTitle, &book.EditionLanguage, &book.ISBN, &book.Publisher, &book.Year, &book.Pages,
			&book.Series, &book.SeriesNumber, &book.Quality, &book.Authors, &book.Translators, &book.Genres,
			&book.AvailableFormats, &book.FormatCount, &book.PrimaryFilePath,
			&book.ReadingProgress, &book.Rating, &book.FinishedAt, &book.CreatedAt, &book.UpdatedAt,
		)
		book.Year = normalizeYear(book.Year)

		if err != nil {
			internalError(c, err)
			return
		}

		c.JSON(http.StatusCreated, book)
	}
}

// updateBook returns a handler function that updates a book
func updateBook(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req UpdateBookRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			internalError(c, err)
			return
		}
		defer func() {
			if err != nil {
				tx.Rollback()
			} else {
				tx.Commit()
			}
		}()

		// Get current edition ID to work with
		var editionID int
		var workID int
		err = tx.QueryRow(`
			SELECT e.id, e.work_id 
			FROM editions e
			WHERE e.id = $1
		`, id).Scan(&editionID, &workID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
				return
			}
			internalError(c, err)
			return
		}

		// Update work if title or other fields provided
		titleChanged := req.Title != nil && *req.Title != ""
		authorChanged := req.Author != nil && *req.Author != ""
		yearChanged := req.PublishedYear != nil
		descriptionChanged := req.Description != nil && *req.Description != ""
		languageChanged := req.Language != nil && *req.Language != ""

		if titleChanged || authorChanged || yearChanged || descriptionChanged || languageChanged {
			// Get current work details
			var currentOriginalTitle string
			var currentOriginalLanguage string
			var currentFirstPublished *int
			var currentAnnotation string
			err = tx.QueryRow(`
				SELECT w.original_title, w.original_language, w.first_published, w.annotation
				FROM works w
				WHERE w.id = $1
			`, workID).Scan(&currentOriginalTitle, &currentOriginalLanguage, &currentFirstPublished, &currentAnnotation)
			if err != nil {
				internalError(c, err)
				return
			}

			// Determine new values
			newTitle := currentOriginalTitle
			if titleChanged {
				newTitle = *req.Title
			}

			newLanguage := currentOriginalLanguage
			if languageChanged {
				newLanguage = *req.Language
			}

			newYear := currentFirstPublished
			if yearChanged {
				newYear = req.PublishedYear
			}

			newAnnotation := currentAnnotation
			if descriptionChanged {
				newAnnotation = *req.Description
			}

			// Update work
			_, err = tx.Exec(`
				UPDATE works 
				SET original_title = $1, original_language = $2, first_published = $3, annotation = $4, updated_at = NOW()
				WHERE id = $5
			`, newTitle, newLanguage, newYear, newAnnotation, workID)
			if err != nil {
				internalError(c, err)
				return
			}
		}

		// Update edition if title, publisher, year, city, pages, annotation, quality provided
		titleChangedForEdition := req.Title != nil && *req.Title != ""
		publisherChanged := req.Publisher != nil && *req.Publisher != ""
		yearChangedForEdition := req.PublishedYear != nil
		descriptionChangedForEdition := req.Description != nil && *req.Description != ""

		if titleChangedForEdition || publisherChanged || yearChangedForEdition || descriptionChangedForEdition {
			// Get current edition details
			var currentEditionTitle string
			var currentPublisher string
			var currentYear *int
			var currentCity string
			var currentPages int
			var currentAnnotation string
			var currentQuality string
			err = tx.QueryRow(`
				SELECT e.title, e.publisher, e.year, e.city, e.pages, e.annotation, e.quality
				FROM editions e
				WHERE e.id = $1
			`, editionID).Scan(&currentEditionTitle, &currentPublisher, &currentYear, &currentCity, &currentPages, &currentAnnotation, &currentQuality)
			if err != nil {
				internalError(c, err)
				return
			}

			// Determine new values
			newEditionTitle := currentEditionTitle
			if titleChangedForEdition {
				newEditionTitle = *req.Title
			}

			newPublisher := currentPublisher
			if publisherChanged {
				newPublisher = *req.Publisher
			}

			newYear := currentYear
			if yearChangedForEdition {
				newYear = req.PublishedYear
			}

			newAnnotation := currentAnnotation
			if descriptionChangedForEdition {
				newAnnotation = *req.Description
			}

			// Update edition
			_, err = tx.Exec(`
				UPDATE editions 
				SET title = $1, publisher = $2, year = $3, annotation = $4, updated_at = NOW()
				WHERE id = $5
			`, newEditionTitle, newPublisher, newYear, newAnnotation, editionID)
			if err != nil {
				internalError(c, err)
				return
			}
		}

		// Return updated book
		var book BookDetails
		err = tx.QueryRow(`
			SELECT 
				work_id, original_title, original_language, first_published, work_type,
				edition_id, edition_title, edition_language, isbn, publisher, year, pages,
				series, series_number, quality, authors, translators, genres,
				available_formats, format_count, primary_file_path,
				reading_progress, rating, finished_at, created_at, updated_at, upload_date
			FROM book_details
			WHERE edition_id = $1
		`, editionID).Scan(
			&book.WorkID, &book.OriginalTitle, &book.OriginalLanguage, &book.FirstPublished, &book.WorkType,
			&book.EditionID, &book.EditionTitle, &book.EditionLanguage, &book.ISBN, &book.Publisher, &book.Year, &book.Pages,
			&book.Series, &book.SeriesNumber, &book.Quality, &book.Authors, &book.Translators, &book.Genres,
			&book.AvailableFormats, &book.FormatCount, &book.PrimaryFilePath,
			&book.ReadingProgress, &book.Rating, &book.FinishedAt, &book.CreatedAt, &book.UpdatedAt, &book.UploadDate,
		)
		book.Year = normalizeYear(book.Year)

		if err != nil {
			internalError(c, err)
			return
		}

		c.JSON(http.StatusOK, book)
	}
}

// deleteBook returns a handler function that deletes a book
func deleteBook(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			internalError(c, err)
			return
		}
		defer func() {
			if err != nil {
				tx.Rollback()
			} else {
				tx.Commit()
			}
		}()

		// Check if book exists
		var exists bool
		err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM editions WHERE id = $1)", id).Scan(&exists)
		if err != nil {
			internalError(c, err)
			return
		}
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
			return
		}

		// Get work_id before deleting the edition
		var workID int
		err = tx.QueryRow("SELECT work_id FROM editions WHERE id = $1", id).Scan(&workID)
		if err != nil {
			internalError(c, err)
			return
		}

		// Delete edition (will cascade to edition_files, reading_progress, etc.)
		_, err = tx.Exec("DELETE FROM editions WHERE id = $1", id)
		if err != nil {
			internalError(c, err)
			return
		}

		// Clean up orphaned work (no remaining editions)
		_, err = tx.Exec("DELETE FROM works WHERE id = $1 AND NOT EXISTS (SELECT 1 FROM editions WHERE work_id = $1)", workID)
		if err != nil {
			internalError(c, err)
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// AuthorWithBooks represents an author with their books and formats
type AuthorWithBooks struct {
	ID         int              `json:"id"`
	FirstName  string           `json:"first_name"`
	LastName   string           `json:"last_name"`
	BooksCount int              `json:"books_count"`
	Books      []BookWithFormats `json:"books"`
}

// BookWithFormats represents a book with its formats
type BookWithFormats struct {
	ID         int            `json:"id"`
	Title      string         `json:"title"`
	Year       *int           `json:"year"`
	OnShelf    bool           `json:"on_shelf"`
	UploadDate string         `json:"upload_date"`
	Formats    []FormatInfo   `json:"formats"`
}

// FormatInfo represents format information
type FormatInfo struct {
	FormatName string `json:"format_name"`
	FilePath   string `json:"file_path"`
}

// getAuthors returns a handler function that gets authors with hierarchical book data
func getAuthors(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get filter parameters
		authorFilter := c.Query("author")
		bookFilter := c.Query("book")
		genreFilter := c.Query("genre")

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 50
		}

		// Common WHERE clause
		whereClause := " WHERE wc.role = 'author'"
		whereArgs := []interface{}{}
		argNum := 1

		if authorFilter != "" {
			whereClause += fmt.Sprintf(" AND p.lower_fio LIKE $%d", argNum)
			whereArgs = append(whereArgs, "%"+normalizeQuery(authorFilter)+"%")
			argNum++
		}

		if bookFilter != "" {
			whereClause += fmt.Sprintf(" AND w.lower_original_title LIKE $%d", argNum)
			whereArgs = append(whereArgs, "%"+normalizeQuery(bookFilter)+"%")
			argNum++
		}

		if genreFilter != "" {
			whereClause += fmt.Sprintf(" AND w.id IN (SELECT wg.work_id FROM work_genres wg JOIN genres g ON g.id = wg.genre_id WHERE LOWER(g.name) LIKE $%d)", argNum)
			whereArgs = append(whereArgs, "%"+strings.ToLower(genreFilter)+"%")
			argNum++
		}

		// Count total
		countQuery := `
			SELECT COUNT(*) FROM (
				SELECT p.id
				FROM persons p
				JOIN work_contributors wc ON wc.person_id = p.id
				JOIN works w ON w.id = wc.work_id
				JOIN editions e ON e.work_id = w.id
		` + whereClause + " GROUP BY p.id) sub"
		var total int
		err := db.QueryRow(countQuery, whereArgs...).Scan(&total)
		if err != nil {
			internalError(c, err)
			return
		}

		// Count total distinct works matching filters
		worksFrom := `
			FROM works w
			JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
			JOIN persons p ON p.id = wc.person_id
			JOIN editions e ON e.work_id = w.id
		` + whereClause
		var totalWorks int
		err = db.QueryRow("SELECT COUNT(DISTINCT w.id) "+worksFrom, whereArgs...).Scan(&totalWorks)
		if err != nil {
			internalError(c, err)
			return
		}

		// Count total distinct edition files matching filters
		filesFrom := `
			FROM edition_files ef
			JOIN editions e ON e.id = ef.edition_id
			JOIN works w ON w.id = e.work_id
			JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
			JOIN persons p ON p.id = wc.person_id
		` + whereClause
		var totalEditions int
		err = db.QueryRow("SELECT COUNT(DISTINCT ef.id) "+filesFrom, whereArgs...).Scan(&totalEditions)
		if err != nil {
			internalError(c, err)
			return
		}

		// Build the main query
		query := `
			SELECT 
				p.id,
				COALESCE(p.first_name, '') as first_name,
				p.last_name,
				COUNT(DISTINCT w.id) as books_count
			FROM persons p
			JOIN work_contributors wc ON wc.person_id = p.id
			JOIN works w ON w.id = wc.work_id
			JOIN editions e ON e.work_id = w.id
		` + whereClause + " GROUP BY p.id, p.first_name, p.last_name ORDER BY p.last_name, p.first_name" +
			fmt.Sprintf(" LIMIT $%d OFFSET $%d", argNum, argNum+1)

		offset := (page - 1) * limit
		queryArgs := append(whereArgs, limit, offset)

		rows, err := db.Query(query, queryArgs...)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		var authors []AuthorWithBooks
		for rows.Next() {
			var author AuthorWithBooks
			if err := rows.Scan(&author.ID, &author.FirstName, &author.LastName, &author.BooksCount); err != nil {
				internalError(c, err)
				return
			}

			// Get books for this author
			booksQuery := `
				SELECT DISTINCT 
					e.id,
					w.original_title,
					e.year,
					e.on_shelf,
					e.upload_date
				FROM works w
				JOIN work_contributors wc ON wc.work_id = w.id
				JOIN editions e ON e.work_id = w.id
				WHERE wc.person_id = $1 AND wc.role = 'author'
			`

			bookArgs := []interface{}{author.ID}
			bookArgNum := 2

			if bookFilter != "" {
				booksQuery += fmt.Sprintf(" AND w.lower_original_title LIKE $%d", bookArgNum)
				bookArgs = append(bookArgs, "%"+normalizeQuery(bookFilter)+"%")
			}

			booksQuery += " ORDER BY w.original_title"

			bookRows, err := db.Query(booksQuery, bookArgs...)
			if err != nil {
				internalError(c, err)
				return
			}

			books := make([]BookWithFormats, 0)
			for bookRows.Next() {
				var book BookWithFormats
				var year sql.NullInt64
				var onShelf bool
				var uploadDate sql.NullString
				if err := bookRows.Scan(&book.ID, &book.Title, &year, &onShelf, &uploadDate); err != nil {
					bookRows.Close()
					internalError(c, err)
					return
				}
			if year.Valid && year.Int64 != 0 {
				yearInt := int(year.Int64)
				book.Year = &yearInt
			}
				book.OnShelf = onShelf
				if uploadDate.Valid {
					book.UploadDate = uploadDate.String
				}

			// Get formats for this book
			formatQuery := `
				SELECT 
					f.name,
					ef.file_path
				FROM edition_files ef
				JOIN formats f ON f.id = ef.format_id
				WHERE ef.edition_id = $1
			`

			formatRows, err := db.Query(formatQuery, book.ID)
				if err != nil {
					bookRows.Close()
					internalError(c, err)
					return
				}

				var formats []FormatInfo
				for formatRows.Next() {
					var format FormatInfo
					if err := formatRows.Scan(&format.FormatName, &format.FilePath); err != nil {
						formatRows.Close()
						bookRows.Close()
						internalError(c, err)
						return
					}
					formats = append(formats, format)
				}
				formatRows.Close()

				book.Formats = formats
				books = append(books, book)
			}
			bookRows.Close()

			if books == nil {
				books = []BookWithFormats{}
			}
			author.Books = books
			authors = append(authors, author)
		}

		if err = rows.Err(); err != nil {
			internalError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"authors":       authors,
			"total":         total,
			"page":          page,
			"limit":         limit,
			"total_works":   totalWorks,
			"total_editions": totalEditions,
		})
	}
}

// UpdatePersonRequest represents the request body for updating a person
type UpdatePersonRequest struct {
	FirstName  string  `json:"first_name"`
	MiddleName string  `json:"middle_name"`
	LastName   string  `json:"last_name"`
	Pseudonym  *string `json:"pseudonym"`
	BirthDate  *string `json:"birth_date"`
	DeathDate  *string `json:"death_date"`
	Biography  *string `json:"biography"`
}

// ExtendedBookData represents the full book data with all related information
type ExtendedBookData struct {
	Work *WorkData `json:"work"`
	Edition *EditionData `json:"edition"`
	Authors []AuthorData `json:"authors"`
	Genres []GenreData `json:"genres"`
	Files []FileData `json:"files"`
	Tags []TagData `json:"tags"`
	TOC []TOCEntryData `json:"toc"`
}

// WorkData represents the work table fields
type WorkData struct {
	ID               int             `json:"id"`
	OriginalTitle    string          `json:"original_title"`
	OriginalLanguage sql.NullString  `json:"original_language"`
	FirstPublished   sql.NullInt64   `json:"first_published"`
	WorkType         sql.NullString  `json:"work_type"`
	Annotation       sql.NullString  `json:"annotation"`
	WordCount        sql.NullInt64   `json:"word_count"`
}

// EditionData represents the edition table fields
type EditionData struct {
	ID            int             `json:"id"`
	Title         string          `json:"title"`
	Language      sql.NullString  `json:"language"`
	ISBN          sql.NullString  `json:"isbn"`
	EAN           sql.NullString  `json:"ean"`
	UDC           sql.NullString  `json:"udc"`
	BBK           sql.NullString  `json:"bbk"`
	Publisher     sql.NullString  `json:"publisher"`
	Year          sql.NullInt64   `json:"year"`
	City          sql.NullString  `json:"city"`
	Pages         sql.NullInt64   `json:"pages"`
	Series        sql.NullString  `json:"series"`
	SeriesNumber  sql.NullString  `json:"series_number"`
	Annotation    sql.NullString  `json:"annotation"`
	Source        sql.NullString  `json:"source"`
	IsComplete    bool            `json:"is_complete"`
	Quality       sql.NullString  `json:"quality"`
	UploadDate    string          `json:"upload_date"`
}

// AuthorData represents an author with role
type AuthorData struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

// GenreData represents a genre
type GenreData struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ParentID *int   `json:"parent_id,omitempty"`
}

// GenreWithAuthors represents a genre with its authors and books
type GenreWithAuthors struct {
	ID          int                `json:"id"`
	Name        string             `json:"name"`
	ParentID    *int               `json:"parent_id"`
	Description *string            `json:"description"`
	Authors     []AuthorWithBooks  `json:"authors"`
	Children    []GenreWithAuthors `json:"children"`
}

// FileData represents an edition file
type FileData struct {
	ID           int             `json:"id"`
	FormatID     int             `json:"format_id"`
	FormatName   string          `json:"format_name"`
	FilePath     string          `json:"file_path"`
	FileSize     sql.NullInt64   `json:"file_size"`
	PageCount    sql.NullInt64   `json:"page_count"`
	HasOCR       bool            `json:"has_ocr"`
	IsPrimary    bool            `json:"is_primary"`
}

// TagData represents a tag
type TagData struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// TOCEntryData represents a table of contents entry
type TOCEntryData struct {
	ID        int             `json:"id"`
	ParentID  sql.NullInt64  `json:"parent_id"`
	Level     int            `json:"level"`
	Title     string         `json:"title"`
	Position  sql.NullInt64  `json:"position"`
	SortOrder int            `json:"sort_order"`
}

// UpdateBookExtendedRequest represents the request for updating extended book data
type UpdateBookExtendedRequest struct {
	Work     map[string]interface{} `json:"work"`
	Edition  map[string]interface{} `json:"edition"`
	Authors  []AuthorUpdateData     `json:"authors"`
	Genres   []int                  `json:"genres"`
	Tags     []int                  `json:"tags"`
}

// WorkUpdateData represents work fields to update
type WorkUpdateData struct {
	OriginalTitle    string `json:"original_title"`
	OriginalLanguage string `json:"original_language"`
	FirstPublished   *int   `json:"first_published"`
	WorkType         string `json:"work_type"`
	Annotation       string `json:"annotation"`
	WordCount        *int   `json:"word_count"`
}

// EditionUpdateData represents edition fields to update
type EditionUpdateData struct {
	Title        string `json:"title"`
	Language     string `json:"language"`
	ISBN         string `json:"isbn"`
	EAN          string `json:"ean"`
	UDC          string `json:"udc"`
	BBK          string `json:"bbk"`
	Publisher    string `json:"publisher"`
	Year         *int   `json:"year"`
	City         string `json:"city"`
	Pages        *int   `json:"pages"`
	Series       string `json:"series"`
	SeriesNumber string `json:"series_number"`
	Annotation   string `json:"annotation"`
	Source       string `json:"source"`
	IsComplete   bool   `json:"is_complete"`
	Quality      string `json:"quality"`
}

// AuthorUpdateData represents author data to add/update
type AuthorUpdateData struct {
	ID        int    `json:"id,omitempty"`        // existing person ID
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"` // author, translator, editor, etc.
}

// updatePerson returns a handler function that updates a person
func updatePerson(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req UpdatePersonRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec(`
			UPDATE persons SET
				first_name = COALESCE(NULLIF($1,''), first_name),
				middle_name = COALESCE(NULLIF($2,''), middle_name),
				last_name = COALESCE(NULLIF($3,''), last_name),
				pseudonym = $4,
				birth_date = NULLIF($5,'')::date,
				death_date = NULLIF($6,'')::date,
				biography = $7
			WHERE id = $8
		`, req.FirstName, req.MiddleName, req.LastName, req.Pseudonym, req.BirthDate, req.DeathDate, req.Biography, id)
		if err != nil {
			internalError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Person updated successfully"})
	}
}

// getBookExtended returns extended book data with all related information
func getBookExtended(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var workID int
		err := db.QueryRow("SELECT work_id FROM editions WHERE id = $1", id).Scan(&workID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
				return
			}
			internalError(c, err)
			return
		}

		var bookData ExtendedBookData

		// Get work data
		var work WorkData
		err = db.QueryRow(`
			SELECT id, original_title, original_language, first_published, work_type, annotation, word_count
			FROM works WHERE id = $1
		`, workID).Scan(&work.ID, &work.OriginalTitle, &work.OriginalLanguage, &work.FirstPublished, &work.WorkType, &work.Annotation, &work.WordCount)
		if err != nil {
			internalError(c, err)
			return
		}
		bookData.Work = &work

		// Get edition data
		var edition EditionData
		err = db.QueryRow(`
			SELECT id, title, language, isbn, ean, udc, bbk, publisher, year, city, pages, series, series_number, annotation, source, is_complete, quality, upload_date
			FROM editions WHERE id = $1
		`, id).Scan(&edition.ID, &edition.Title, &edition.Language, &edition.ISBN, &edition.EAN, &edition.UDC, &edition.BBK, &edition.Publisher, &edition.Year, &edition.City, &edition.Pages, &edition.Series, &edition.SeriesNumber, &edition.Annotation, &edition.Source, &edition.IsComplete, &edition.Quality, &edition.UploadDate)
		if err != nil {
			internalError(c, err)
			return
		}
		bookData.Edition = &edition

		// Get authors
		rows, err := db.Query(`
			SELECT p.id, COALESCE(p.first_name, ''), p.last_name, wc.role
			FROM work_contributors wc
			JOIN persons p ON p.id = wc.person_id
			WHERE wc.work_id = $1
		`, workID)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var author AuthorData
			if err := rows.Scan(&author.ID, &author.FirstName, &author.LastName, &author.Role); err != nil {
				internalError(c, err)
				return
			}
			bookData.Authors = append(bookData.Authors, author)
		}

		// Get genres
		rows, err = db.Query(`
			SELECT g.id, g.name
			FROM work_genres wg
			JOIN genres g ON g.id = wg.genre_id
			WHERE wg.work_id = $1
		`, workID)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var genre GenreData
			if err := rows.Scan(&genre.ID, &genre.Name); err != nil {
				internalError(c, err)
				return
			}
			bookData.Genres = append(bookData.Genres, genre)
		}

		// Get files
		rows, err = db.Query(`
			SELECT ef.id, ef.format_id, f.name, ef.file_path, ef.file_size, ef.page_count, ef.has_ocr, ef.is_primary
			FROM edition_files ef
			JOIN formats f ON f.id = ef.format_id
			WHERE ef.edition_id = $1
		`, id)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var file FileData
			if err := rows.Scan(&file.ID, &file.FormatID, &file.FormatName, &file.FilePath, &file.FileSize, &file.PageCount, &file.HasOCR, &file.IsPrimary); err != nil {
				internalError(c, err)
				return
			}
			bookData.Files = append(bookData.Files, file)
		}

		// Get tags
		rows, err = db.Query(`
			SELECT t.id, t.name
			FROM edition_tags et
			JOIN tags t ON t.id = et.tag_id
			WHERE et.edition_id = $1
		`, id)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var tag TagData
			if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
				internalError(c, err)
				return
			}
			bookData.Tags = append(bookData.Tags, tag)
		}

		// Get TOC entries
		rows, err = db.Query(`
			SELECT id, parent_id, level, title, position, sort_order
			FROM toc_entries
			WHERE edition_id = $1
			ORDER BY sort_order
		`, id)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var toc TOCEntryData
			if err := rows.Scan(&toc.ID, &toc.ParentID, &toc.Level, &toc.Title, &toc.Position, &toc.SortOrder); err != nil {
				internalError(c, err)
				return
			}
			bookData.TOC = append(bookData.TOC, toc)
		}

		c.JSON(http.StatusOK, bookData)
	}
}

var allowedWorkFields = map[string]bool{
	"original_title":    true,
	"original_language": true,
	"first_published":   true,
	"work_type":         true,
	"annotation":        true,
	"word_count":        true,
}

var allowedEditionFields = map[string]bool{
	"title":         true,
	"language":      true,
	"isbn":          true,
	"ean":           true,
	"udc":           true,
	"bbk":           true,
	"publisher":     true,
	"year":          true,
	"city":          true,
	"pages":         true,
	"series":        true,
	"series_number": true,
	"annotation":    true,
	"source":        true,
	"is_complete":   true,
	"quality":       true,
}

func safeUpdateFields(data map[string]interface{}, allowed map[string]bool) ([]string, []interface{}, int) {
	updates := []string{}
	args := []interface{}{}
	argNum := 1

	for key, value := range data {
		if !allowed[key] {
			continue
		}
		switch v := value.(type) {
		case string:
			if v != "" {
				updates = append(updates, fmt.Sprintf("%s = $%d", key, argNum))
				args = append(args, value)
				argNum++
			}
		case int, int64, float64:
			updates = append(updates, fmt.Sprintf("%s = $%d", key, argNum))
			args = append(args, value)
			argNum++
		case bool:
			updates = append(updates, fmt.Sprintf("%s = $%d", key, argNum))
			args = append(args, value)
			argNum++
		}
	}
	return updates, args, argNum
}

// updateBookExtended updates extended book data
func updateBookExtended(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req UpdateBookExtendedRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			internalError(c, err)
			return
		}
		defer func() {
			if err != nil {
				tx.Rollback()
			} else {
				tx.Commit()
			}
		}()

		var workID int
		err = tx.QueryRow("SELECT work_id FROM editions WHERE id = $1", id).Scan(&workID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
			return
		}

		// Update work - only allowed fields
		if len(req.Work) > 0 {
			updates, args, argNum := safeUpdateFields(req.Work, allowedWorkFields)

			if len(updates) > 0 {
				args = append(args, workID)
				_, err = tx.Exec("UPDATE works SET "+strings.Join(updates, ", ")+", updated_at = NOW() WHERE id = $"+strconv.Itoa(argNum), args...)
				if err != nil {
					internalError(c, err)
					return
				}
			}
		}

		// Update edition - only allowed fields
		if len(req.Edition) > 0 {
			updates, args, argNum := safeUpdateFields(req.Edition, allowedEditionFields)

			if len(updates) > 0 {
				args = append(args, id)
				_, err = tx.Exec("UPDATE editions SET "+strings.Join(updates, ", ")+", updated_at = NOW() WHERE id = $"+strconv.Itoa(argNum), args...)
				if err != nil {
					internalError(c, err)
					return
				}
			}
		}

		// Update authors - first remove all existing
		if req.Authors != nil {
			_, err = tx.Exec("DELETE FROM work_contributors WHERE work_id = $1", workID)
			if err != nil {
				internalError(c, err)
				return
			}

			for _, author := range req.Authors {
				var personID int

				if author.ID > 0 {
					personID = author.ID
				} else {
					err = tx.QueryRow(`
						INSERT INTO persons (last_name, first_name)
						VALUES ($1, $2)
						ON CONFLICT (first_name, last_name) DO UPDATE SET last_name = EXCLUDED.last_name
						RETURNING id
					`, author.LastName, author.FirstName).Scan(&personID)
					if err != nil {
						internalError(c, err)
						return
					}
				}

				role := "author"
				if author.Role != "" {
					role = author.Role
				}

				_, err = tx.Exec(`
					INSERT INTO work_contributors (work_id, person_id, role)
					VALUES ($1, $2, $3)
				`, workID, personID, role)
				if err != nil {
					internalError(c, err)
					return
				}
			}
		}

		// Update genres
		if req.Genres != nil {
			_, err = tx.Exec("DELETE FROM work_genres WHERE work_id = $1", workID)
			if err != nil {
				internalError(c, err)
				return
			}

			for _, genreID := range req.Genres {
				_, err = tx.Exec(`
					INSERT INTO work_genres (work_id, genre_id)
					VALUES ($1, $2)
				`, workID, genreID)
				if err != nil {
					internalError(c, err)
					return
				}
			}
		}

		// Update tags
		if req.Tags != nil {
			_, err = tx.Exec("DELETE FROM edition_tags WHERE edition_id = $1", id)
			if err != nil {
				internalError(c, err)
				return
			}

			for _, tagID := range req.Tags {
				_, err = tx.Exec(`
					INSERT INTO edition_tags (edition_id, tag_id)
					VALUES ($1, $2)
				`, id, tagID)
				if err != nil {
					internalError(c, err)
					return
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Book updated successfully"})
	}
}

// getGenres returns all genres
func getGenres(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query("SELECT id, name, parent_id FROM genres ORDER BY name")
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		var genres []GenreData
		for rows.Next() {
			var genre GenreData
			var parentID sql.NullInt64
			if err := rows.Scan(&genre.ID, &genre.Name, &parentID); err != nil {
				internalError(c, err)
				return
			}
			if parentID.Valid {
				v := int(parentID.Int64)
				genre.ParentID = &v
			}
			genres = append(genres, genre)
		}

		c.JSON(http.StatusOK, genres)
	}
}

// createGenre creates a new genre
func createGenre(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var genre GenreData
		err := db.QueryRow("INSERT INTO genres (name) VALUES ($1) RETURNING id, name", req.Name).Scan(&genre.ID, &genre.Name)
		if err != nil {
			internalError(c, err)
			return
		}

		c.JSON(http.StatusCreated, genre)
	}
}

// getGenreTree returns genre hierarchy (without authors/books — loaded lazily)
func getGenreTree(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		genreFilter := c.Query("genre")

		whereClause := ""
		whereArgs := []interface{}{}

		if genreFilter != "" {
			whereClause = " WHERE LOWER(g.name) LIKE $1"
			whereArgs = append(whereArgs, "%"+strings.ToLower(genreFilter)+"%")
		}

		query := `SELECT g.id, g.name, g.parent_id, g.description FROM genres g` + whereClause + ` ORDER BY g.name`

		rows, err := db.Query(query, whereArgs...)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		genreMap := make(map[int]*GenreWithAuthors)
		for rows.Next() {
			var g GenreWithAuthors
			var parentID sql.NullInt64
			var desc sql.NullString
			if err := rows.Scan(&g.ID, &g.Name, &parentID, &desc); err != nil {
				internalError(c, err)
				return
			}
			if parentID.Valid {
				v := int(parentID.Int64)
				g.ParentID = &v
			}
			if desc.Valid {
				g.Description = &desc.String
			}
			g.Authors = []AuthorWithBooks{}
			g.Children = []GenreWithAuthors{}
			genreMap[g.ID] = &g
		}

		for _, g := range genreMap {
			if g.ParentID != nil {
				if parent, ok := genreMap[*g.ParentID]; ok {
					parent.Children = append(parent.Children, *g)
				}
			}
		}

		var roots []GenreWithAuthors
		for _, g := range genreMap {
			if g.ParentID == nil {
				roots = append(roots, *g)
			}
		}
		if roots == nil {
			roots = []GenreWithAuthors{}
		}

		c.JSON(http.StatusOK, gin.H{"genres": roots})
	}
}

// getGenreAuthors returns authors with books for a specific genre (lazy load)
func getGenreAuthors(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		genreID := c.Param("id")
		authorFilter := c.Query("author")
		bookFilter := c.Query("book")

		authorQuery := `
			SELECT p.id, COALESCE(p.first_name, '') as first_name, p.last_name
			FROM persons p
			JOIN work_contributors wc ON wc.person_id = p.id AND wc.role = 'author'
			JOIN works w ON w.id = wc.work_id
			JOIN work_genres wg ON wg.work_id = w.id
			WHERE wg.genre_id = $1
		`
		authorArgs := []interface{}{genreID}
		argNum := 2

		if authorFilter != "" {
			authorQuery += fmt.Sprintf(" AND p.lower_fio LIKE $%d", argNum)
			authorArgs = append(authorArgs, "%"+normalizeQuery(authorFilter)+"%")
			argNum++
		}
		if bookFilter != "" {
			authorQuery += fmt.Sprintf(" AND w.lower_original_title LIKE $%d", argNum)
			authorArgs = append(authorArgs, "%"+normalizeQuery(bookFilter)+"%")
			argNum++
		}

		authorQuery += " GROUP BY p.id, p.first_name, p.last_name ORDER BY p.last_name, p.first_name"

		aRows, err := db.Query(authorQuery, authorArgs...)
		if err != nil {
			internalError(c, err)
			return
		}
		defer aRows.Close()

		authors := []AuthorWithBooks{}
		for aRows.Next() {
			var author AuthorWithBooks
			if err := aRows.Scan(&author.ID, &author.FirstName, &author.LastName); err != nil {
				internalError(c, err)
				return
			}

			booksQuery := `
				SELECT e.id, w.original_title, e.year, e.on_shelf, e.upload_date
				FROM works w
				JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
				JOIN editions e ON e.work_id = w.id
				JOIN work_genres wg ON wg.work_id = w.id
				WHERE wc.person_id = $1 AND wg.genre_id = $2
			`
			bookArgs := []interface{}{author.ID, genreID}
			bArgNum := 3

			if bookFilter != "" {
				booksQuery += fmt.Sprintf(" AND w.lower_original_title LIKE $%d", bArgNum)
				bookArgs = append(bookArgs, "%"+normalizeQuery(bookFilter)+"%")
			}

			booksQuery += " ORDER BY NULLIF(e.year, 0) DESC NULLS LAST, w.original_title"

			bRows, err := db.Query(booksQuery, bookArgs...)
			if err != nil {
				internalError(c, err)
				return
			}

			var books []BookWithFormats
			for bRows.Next() {
				var book BookWithFormats
				var year sql.NullInt64
				var onShelf bool
				var uploadDate sql.NullString
				if err := bRows.Scan(&book.ID, &book.Title, &year, &onShelf, &uploadDate); err != nil {
					bRows.Close()
					internalError(c, err)
					return
				}
				if year.Valid {
					y := int(year.Int64)
					book.Year = &y
				}
				book.OnShelf = onShelf
				if uploadDate.Valid {
					book.UploadDate = uploadDate.String
				}

				formatRows, err := db.Query(`
					SELECT f.name, ef.file_path
					FROM edition_files ef
					JOIN formats f ON f.id = ef.format_id
					WHERE ef.edition_id = $1
				`, book.ID)
				if err != nil {
					bRows.Close()
					internalError(c, err)
					return
				}
				var formats []FormatInfo
				for formatRows.Next() {
					var fi FormatInfo
					if err := formatRows.Scan(&fi.FormatName, &fi.FilePath); err != nil {
						formatRows.Close()
						bRows.Close()
						internalError(c, err)
						return
					}
					formats = append(formats, fi)
				}
				formatRows.Close()
				if formats == nil {
					book.Formats = []FormatInfo{}
				} else {
					book.Formats = formats
				}
				books = append(books, book)
			}
			bRows.Close()

			if books == nil {
				author.Books = []BookWithFormats{}
			} else {
				author.Books = books
			}
			authors = append(authors, author)
		}

		if authors == nil {
			authors = []AuthorWithBooks{}
		}

		c.JSON(http.StatusOK, gin.H{"authors": authors})
	}
}

// updateGenre updates a genre
func updateGenre(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Name        string  `json:"name"`
			Description *string `json:"description"`
			ParentID    *int    `json:"parent_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec(`
			UPDATE genres SET name = COALESCE(NULLIF($1, ''), name),
				description = COALESCE($2, description),
				parent_id = $3
			WHERE id = $4
		`, req.Name, req.Description, req.ParentID, id)
		if err != nil {
			internalError(c, err)
			return
		}

		var genre GenreData
		err = db.QueryRow("SELECT id, name FROM genres WHERE id = $1", id).Scan(&genre.ID, &genre.Name)
		if err != nil {
			internalError(c, err)
			return
		}

		c.JSON(http.StatusOK, genre)
	}
}

// deleteGenre deletes a genre
func deleteGenre(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		tx, err := db.Begin()
		if err != nil {
			internalError(c, err)
			return
		}
		defer tx.Rollback()

		// Remove genre-book associations
		if _, err := tx.Exec("DELETE FROM work_genres WHERE genre_id = $1", id); err != nil {
			internalError(c, err)
			return
		}

		// Orphan child genres (set parent_id to NULL)
		if _, err := tx.Exec("UPDATE genres SET parent_id = NULL WHERE parent_id = $1", id); err != nil {
			internalError(c, err)
			return
		}

		// Delete the genre itself
		if _, err := tx.Exec("DELETE FROM genres WHERE id = $1", id); err != nil {
			internalError(c, err)
			return
		}

		if err := tx.Commit(); err != nil {
			internalError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Жанр удалён"})
	}
}

// getTags returns all tags
func getTags(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query("SELECT id, name, COALESCE(color,''), COALESCE(description,'') FROM tags ORDER BY name")
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		tags := make([]TagData, 0)
		for rows.Next() {
			var tag TagData
			if err := rows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.Description); err != nil {
				internalError(c, err)
				return
			}
			tags = append(tags, tag)
		}

		c.JSON(http.StatusOK, tags)
	}
}

// createTag creates a new tag
func createTag(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name        string `json:"name" binding:"required"`
			Color       string `json:"color"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var tag TagData
		err := db.QueryRow("INSERT INTO tags (name, color, description) VALUES ($1, NULLIF($2,''), NULLIF($3,'')) RETURNING id, name, COALESCE(color,''), COALESCE(description,'')", req.Name, req.Color, req.Description).Scan(&tag.ID, &tag.Name, &tag.Color, &tag.Description)
		if err != nil {
			internalError(c, err)
			return
		}

		c.JSON(http.StatusCreated, tag)
	}
}

// getPersons returns all persons (authors, etc.)
func getPersons(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query("SELECT id, COALESCE(first_name, ''), last_name FROM persons ORDER BY last_name, first_name")
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		var persons []AuthorData
		for rows.Next() {
			var person AuthorData
			if err := rows.Scan(&person.ID, &person.FirstName, &person.LastName); err != nil {
				internalError(c, err)
				return
			}
			persons = append(persons, person)
		}

		c.JSON(http.StatusOK, persons)
	}
}

func getPerson(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var p PersonData
		err := db.QueryRow(`
			SELECT id, COALESCE(first_name,'') as first_name, COALESCE(middle_name,'') as middle_name, last_name,
				pseudonym, birth_date, death_date, biography, photo_url,
				COALESCE((SELECT COUNT(DISTINCT w.id) FROM work_contributors wc
					JOIN works w ON w.id = wc.work_id
					JOIN editions e ON e.work_id = w.id
					WHERE wc.person_id = persons.id), 0) as books_count
			FROM persons WHERE id = $1
		`, id).Scan(&p.ID, &p.FirstName, &p.MiddleName, &p.LastName,
			&p.Pseudonym, &p.BirthDate, &p.DeathDate, &p.Biography, &p.PhotoURL, &p.BooksCount)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
				return
			}
			internalError(c, err)
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

// LanguageData represents a language
type LanguageData struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	NativeName  string `json:"native_name"`
}

// getLanguages returns all languages
func getLanguages(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query("SELECT code, name, native_name FROM languages ORDER BY name")
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		var languages []LanguageData
		for rows.Next() {
			var lang LanguageData
			if err := rows.Scan(&lang.Code, &lang.Name, &lang.NativeName); err != nil {
				internalError(c, err)
				return
			}
			languages = append(languages, lang)
		}

		c.JSON(http.StatusOK, languages)
	}
}

// ImportBookFile handles single file upload
type importFileResult struct {
	title      string
	authors    []string
	workID     int
	editionID  int
	filePath   string
	hashStr    string
	bookInfo   *utils.FB2Book
	llmResult  *utils.LLMResult
	parseErr   error
}

type duplicateInfo struct {
	title   string
	authors string
	hash    string
}

func (d *duplicateInfo) Error() string {
	return fmt.Sprintf("book already exists: %s — %s", d.authors, d.title)
}

func importFile(filename string, data []byte, ext string, db *sql.DB, cfg *config.Config) (result *importFileResult, err error) {
	var bookContent []byte
	var bookInfo *utils.FB2Book
	var parseErr error
	var zipContentType utils.ZipContentType

	switch ext {
	case ".fb2":
		bookContent = data
		bookInfo, parseErr = utils.ParseFB2FromBytes(data)
	case ".zip":
		zipResult, zipErr := utils.DetectZipContent(data)
		if zipErr != nil {
			return nil, fmt.Errorf("extract from zip: %w", zipErr)
		}
		bookContent = zipResult.Content
		zipContentType = zipResult.ContentType
		if zipResult.ContentType == utils.ZipContentFB2 {
			bookInfo, parseErr = utils.ParseFB2FromBytes(bookContent)
		} else if zipResult.ContentType == utils.ZipContentEPUB {
			epubInfo, epubErr := utils.ParseEPUBFromBytes(bookContent)
			if epubErr == nil && epubInfo != nil {
				bookInfo = &utils.FB2Book{
					Title: epubInfo.Title, Authors: epubInfo.Authors,
					Lang: epubInfo.Lang, Year: epubInfo.Year, ISBN: epubInfo.ISBN,
					Publisher: epubInfo.Publisher, Genres: epubInfo.Genres,
					Annotation: epubInfo.Annotation, Sequence: epubInfo.Sequence,
				}
			}
		}
	case ".epub":
		bookContent = data
		epubInfo, epubErr := utils.ParseEPUBFromBytes(data)
		if epubErr == nil && epubInfo != nil {
			bookInfo = &utils.FB2Book{
				Title: epubInfo.Title, Authors: epubInfo.Authors,
				Lang: epubInfo.Lang, Year: epubInfo.Year, ISBN: epubInfo.ISBN,
				Publisher: epubInfo.Publisher, Genres: epubInfo.Genres,
				Annotation: epubInfo.Annotation, Sequence: epubInfo.Sequence,
			}
		}
	case ".pdf", ".docx", ".doc":
		bookContent = data
	}

	if bookContent == nil {
		return nil, fmt.Errorf("cannot extract content from %s", ext)
	}

	hash := sha256.Sum256(bookContent)
	hashStr := hex.EncodeToString(hash[:])

	var existingTitle string
	var existingAuthors string
	err = db.QueryRow(`
		SELECT w.original_title,
			STRING_AGG(p.last_name || ' ' || COALESCE(p.first_name, ''), ', ' ORDER BY p.last_name)
		FROM edition_files ef
		JOIN editions e ON ef.edition_id = e.id
		JOIN works w ON e.work_id = w.id
		LEFT JOIN work_contributors wc ON w.id = wc.work_id AND wc.role = 'author'
		LEFT JOIN persons p ON wc.person_id = p.id
		WHERE ef.file_hash = $1
		GROUP BY w.original_title
	`, hashStr).Scan(&existingTitle, &existingAuthors)
	if err == nil {
		if existingAuthors == "" {
			existingAuthors = "Неизвестный автор"
		}
		return nil, &duplicateInfo{title: existingTitle, authors: existingAuthors, hash: hashStr}
	}

	var llmResult *utils.LLMResult
	needsLLM := ext == ".pdf" || ext == ".docx" || ext == ".doc"
	if ext == ".zip" && zipContentType != utils.ZipContentFB2 {
		needsLLM = true
	}
	if needsLLM {
		var text string
		var textErr error
		switch ext {
		case ".zip":
			switch zipContentType {
			case utils.ZipContentPDF:
				text, textErr = utils.ExtractPDFText(bookContent, 3)
			case utils.ZipContentDOCX:
				text, textErr = utils.ExtractDOCXText(bookContent, 3)
			case utils.ZipContentDOC:
				text, textErr = utils.ExtractDOCText(bookContent, 3)
			}
		case ".pdf":
			text, textErr = utils.ExtractPDFText(bookContent, 3)
		case ".docx":
			text, textErr = utils.ExtractDOCXText(bookContent, 3)
		case ".doc":
			if len(bookContent) > 2 && bookContent[0] == 0x50 && bookContent[1] == 0x4b {
				text, textErr = utils.ExtractDOCXText(bookContent, 3)
			} else {
				text, textErr = utils.ExtractDOCText(bookContent, 3)
			}
		}
		if textErr == nil && text != "" && cfg.LLM.BaseURL != "" {
			llmResult = recognizeBook(text, cfg)
		}
	}

	title := filename
	titleResolved := false
	if bookInfo != nil && bookInfo.Title != "" {
		title = bookInfo.Title
		titleResolved = true
	} else if llmResult != nil && llmResult.Title != "" {
		title = llmResult.Title
		titleResolved = true
	}

	var authors []string
	if bookInfo != nil && len(bookInfo.Authors) > 0 {
		authors = bookInfo.Authors
	} else if llmResult != nil && len(llmResult.Authors) > 0 {
		authors = llmResult.Authors
	}
	if len(authors) == 0 {
		authors = []string{"Неизвестный автор"}
	}

	var existingWorkID int
	if title != filename {
		existingWorkID = findWorkByTitleAndAuthors(db, title, authors)
	}

	destDir := cfg.Directories.Bookarch
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("create bookarch dir: %w", err)
	}

	subDir := getNextSubdir(destDir)
	if err := os.MkdirAll(filepath.Join(destDir, subDir), 0755); err != nil {
		return nil, fmt.Errorf("create subdir: %w", err)
	}

	var innerExt string
	if ext == ".zip" {
		innerExt = utils.InnerFileExtFromZipContent(zipContentType)
	} else {
		innerExt = ext
	}

	var baseName string
	if titleResolved {
		baseName = utils.TransliterateFilename(title)
	} else {
		baseName = strings.TrimSuffix(title, ext)
		if ext == ".zip" {
			baseName = strings.TrimSuffix(baseName, innerExt)
		}
	}
	zipName := baseName + ".zip"
	destPath := filepath.Join(destDir, subDir, zipName)

	idx := 1
	for {
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			break
		}
		zipName = fmt.Sprintf("%s_%d.zip", baseName, idx)
		destPath = filepath.Join(destDir, subDir, zipName)
		idx++
	}

	zipFile, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("create zip: %w", err)
	}
	zipWriter := zip.NewWriter(zipFile)
	entryName := baseName + innerExt
	fw, err := zipWriter.Create(entryName)
	if err != nil {
		zipWriter.Close()
		zipFile.Close()
		return nil, fmt.Errorf("create zip entry: %w", err)
	}
	if _, err := fw.Write(bookContent); err != nil {
		zipWriter.Close()
		zipFile.Close()
		return nil, fmt.Errorf("write zip: %w", err)
	}
	if err := zipWriter.Close(); err != nil {
		zipFile.Close()
		return nil, fmt.Errorf("close zip writer: %w", err)
	}
	if err := zipFile.Close(); err != nil {
		return nil, fmt.Errorf("close zip file: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("db begin: %w", err)
	}
		defer func() {
			if err != nil {
				tx.Rollback()
				os.Remove(destPath)
			} else {
				tx.Commit()
			}
		}()

	langCode := "eng"
	if bookInfo != nil && bookInfo.Lang != "" {
		langCode = utils.NormalizeLanguage(bookInfo.Lang)
	}
	var langExists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM languages WHERE code = $1)", langCode).Scan(&langExists)
	if err != nil || !langExists {
		langCode = "eng"
	}

	workType := "novel"
	if bookInfo != nil && len(bookInfo.Genres) > 0 {
		gn := bookInfo.Genres[0]
		switch strings.ToLower(gn) {
		case "poetry", "poem":
			workType = "poem"
		case "story", "short_story":
			workType = "story"
		case "article", "sci_article":
			workType = "article"
		case "essay", "sci_publicistics":
			workType = "article"
		}
	}

	var year *int
	if bookInfo != nil && bookInfo.Year != "" {
		if y, pe := strconv.Atoi(bookInfo.Year); pe == nil {
			year = &y
		}
	}

	var workID int
	if existingWorkID > 0 {
		workID = existingWorkID
		log.Printf("Found existing work id=%d for title='%s'", workID, title)
	} else {
		annotation := ""
		if bookInfo != nil {
			annotation = bookInfo.Annotation
		}
		err = tx.QueryRow(`
			INSERT INTO works (original_title, original_language, first_published, work_type, annotation)
			VALUES ($1, $2, $3, $4, $5) RETURNING id
		`, title, langCode, year, workType, annotation).Scan(&workID)
		if err != nil {
			return nil, fmt.Errorf("insert work: %w", err)
		}
	}

	if existingWorkID == 0 {
		for _, authorName := range authors {
			firstName, lastName := utils.NormalizeAuthorName(authorName)
			var personID int
			err = tx.QueryRow(`
				INSERT INTO persons (last_name, first_name)
				VALUES ($1, $2) ON CONFLICT (first_name, last_name) DO NOTHING RETURNING id
			`, lastName, firstName).Scan(&personID)
			if err != nil {
				err = tx.QueryRow(`SELECT id FROM persons WHERE last_name=$1 AND (first_name=$2 OR (first_name IS NULL AND $2=''))`,
					lastName, firstName).Scan(&personID)
				if err != nil {
					return nil, fmt.Errorf("person lookup: %w", err)
				}
			}
			_, err = tx.Exec(`
				INSERT INTO work_contributors (work_id, person_id, role) VALUES ($1,$2,'author') ON CONFLICT DO NOTHING
			`, workID, personID)
			if err != nil {
				return nil, fmt.Errorf("insert contributor: %w", err)
			}
		}
	}

	if existingWorkID == 0 && bookInfo != nil {
		for _, genreName := range bookInfo.Genres {
			if strings.TrimSpace(genreName) == "" {
				continue
			}
			var genreID int
			err = tx.QueryRow(`
				INSERT INTO genres (name) VALUES ($1) ON CONFLICT (name) DO NOTHING RETURNING id
			`, genreName).Scan(&genreID)
			if err != nil {
				err = tx.QueryRow(`SELECT id FROM genres WHERE name=$1`, genreName).Scan(&genreID)
				if err != nil {
					continue
				}
			}
			_, err = tx.Exec(`
				INSERT INTO work_genres (work_id, genre_id) VALUES ($1,$2) ON CONFLICT DO NOTHING
			`, workID, genreID)
			if err != nil {
				continue
			}
		}
	}

	publisher := ""
	var editionISBN interface{} = nil
	if bookInfo != nil {
		publisher = bookInfo.Publisher
		isbn := strings.SplitN(bookInfo.ISBN, ",", 2)[0]
		isbn = strings.TrimSpace(isbn)
		if isbn != "" {
			var exists bool
			tx.QueryRow("SELECT EXISTS(SELECT 1 FROM editions WHERE isbn=$1)", isbn).Scan(&exists)
			if !exists {
				editionISBN = isbn
			}
		}
	}

	var editionID int
	err = tx.QueryRow(`
		INSERT INTO editions (work_id, title, language, publisher, year, source, quality, upload_date, isbn)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),$8) RETURNING id
	`, workID, title, langCode, publisher, year, "imported", "good", editionISBN).Scan(&editionID)
	if err != nil {
		return nil, fmt.Errorf("insert edition: %w", err)
	}

	formatName := utils.GetFormatNameByExt(ext)
	if ext == ".zip" {
		if zipContentType != utils.ZipContentUnknown {
			formatName = utils.ZipContentTypeToFormatName(zipContentType)
		} else {
			formatName = utils.GetFormatNameFromZip(filename)
		}
	}
	var formatID int
	err = tx.QueryRow("SELECT id FROM formats WHERE name=$1", formatName).Scan(&formatID)
	if err != nil {
		formatID = 1
	}

	fileInfo, _ := os.Stat(destPath)
	zipSize := int64(0)
	if fileInfo != nil {
		zipSize = fileInfo.Size()
	}

	relPath := filepath.Join(filepath.Base(cfg.Directories.Bookarch), subDir, zipName)
	_, err = tx.Exec(`
		INSERT INTO edition_files (edition_id, format_id, file_path, file_size, file_hash, is_primary)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, editionID, formatID, relPath, zipSize, hashStr, true)
	if err != nil {
		return nil, fmt.Errorf("insert edition_file: %w", err)
	}

	return &importFileResult{
		title: title, authors: authors,
		workID: workID, editionID: editionID,
		filePath: relPath, hashStr: hashStr,
		bookInfo: bookInfo, llmResult: llmResult,
		parseErr: parseErr,
	}, nil
}

func importBookFile(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := getConfig(c)

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 50<<20)

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
			return
		}
		defer file.Close()

		filename := header.Filename
		ext := strings.ToLower(filepath.Ext(filename))

		if !isSupportedFile(filename) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported format: " + ext})
			return
		}

		data, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
			return
		}

		res, importErr := importFile(filename, data, ext, db, cfg)
		if importErr != nil {
			var dup *duplicateInfo
			if errors.As(importErr, &dup) {
				log.Printf("Duplicate file detected: hash=%s, title='%s', authors='%s'", dup.hash, dup.title, dup.authors)
				c.JSON(http.StatusOK, gin.H{
					"duplicate": true,
					"message":   fmt.Sprintf("Книга уже существует в библиотеке: %s — %s", dup.authors, dup.title),
					"file_hash": dup.hash,
					"title":     dup.title,
					"authors":   dup.authors,
				})
				return
			}
			logImportError(filename, "", importErr.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": importErr.Error()})
			return
		}

		response := gin.H{
			"message":    "Book imported successfully",
			"work_id":    res.workID,
			"edition_id": res.editionID,
			"file_path":  res.filePath,
			"title":      res.title,
		}
		if res.bookInfo != nil {
			response["parsed"] = true
			response["authors"] = res.bookInfo.Authors
			response["language"] = res.bookInfo.Lang
			if res.bookInfo.Year != "" {
				response["year"] = res.bookInfo.Year
			}
			if res.bookInfo.ISBN != "" {
				response["isbn"] = res.bookInfo.ISBN
			}
		} else if res.parseErr != nil {
			response["parsed"] = false
			response["parse_error"] = res.parseErr.Error()
		} else {
			response["parsed"] = true
			if res.llmResult != nil {
				response["authors"] = res.llmResult.Authors
			}
		}
		c.JSON(http.StatusCreated, response)
	}
}

func sanitizeBasename(name string) string {
	name = filepath.Base(name)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	base = sanitizeFilename(base)
	if base == "" {
		base = "unnamed"
	}
	return base + ext
}

func importUploadFiles() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := getConfig(c)

		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No files provided"})
			return
		}

		files := form.File["files"]
		if len(files) == 0 {
			files = form.File["file"]
		}
		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No files provided"})
			return
		}

		tmpDir := filepath.Join(cfg.Directories.Temp, fmt.Sprintf("upload_%d", time.Now().UnixNano()))
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot create temp directory"})
			return
		}
		cleanup := true
		defer func() {
			if cleanup {
				os.RemoveAll(tmpDir)
			}
		}()

		var savedFiles []string
		for _, fh := range files {
			if !isSupportedFile(fh.Filename) {
				continue
			}
			f, err := fh.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(f)
			f.Close()
			if err != nil {
				continue
			}
			safeName := sanitizeBasename(fh.Filename)
			destPath := filepath.Join(tmpDir, safeName)
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				continue
			}
			savedFiles = append(savedFiles, safeName)
		}

		if len(savedFiles) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No supported files found"})
			return
		}

		err = importManager.Start(tmpDir, savedFiles)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

		cleanup = false // Import manager will clean up on completion
		c.JSON(http.StatusOK, gin.H{
			"started": true,
			"total":   len(savedFiles),
			"tmp_dir": tmpDir,
		})
	}
}

func processDirectoryImport(ctx context.Context, dirPath string, db *sql.DB, cfg *config.Config, items []ImportItem, updateFn func(int, string, string, string)) {
	for i := range items {
		select {
		case <-ctx.Done():
			updateFn(i, "cancelled", "", "")
			return
		default:
		}
		updateFn(i, "processing", "", "")
		importOneFile(ctx, dirPath, items[i].File, i, db, cfg, updateFn)
	}
}

func importOneFile(ctx context.Context, dirPath, filename string, idx int, db *sql.DB, cfg *config.Config, updateFn func(int, string, string, string)) {
	filePath := filepath.Join(dirPath, filename)
	ext := strings.ToLower(filepath.Ext(filename))

	data, err := os.ReadFile(filePath)
	if err != nil {
		updateFn(idx, "error", "", "Cannot read file")
		return
	}

	res, importErr := importFile(filename, data, ext, db, cfg)
	if importErr != nil {
		var dup *duplicateInfo
		if errors.As(importErr, &dup) {
			log.Printf("Duplicate file: hash=%s, title='%s'", dup.hash, dup.title)
			updateFn(idx, "skipped", dup.title, "")
			return
		}
		updateFn(idx, "error", "", importErr.Error())
		return
	}

	updateFn(idx, "done", res.title, "")
}

func findWorkByTitleAndAuthors(db *sql.DB, title string, authors []string) int {
	rows, err := db.Query(`
		SELECT DISTINCT w.id
		FROM works w
		JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
		JOIN persons p ON p.id = wc.person_id
		WHERE LOWER(w.original_title) = LOWER($1)
	`, title)
	if err != nil {
		return 0
	}
	defer rows.Close()

	authorSet := make(map[string]bool)
	for _, a := range authors {
		_, lastName := utils.NormalizeAuthorName(a)
		authorSet[strings.ToLower(lastName)] = true
	}

	for rows.Next() {
		var workID int
		if err := rows.Scan(&workID); err != nil {
			continue
		}

		authorRows, err := db.Query(`
			SELECT p.last_name
			FROM work_contributors wc
			JOIN persons p ON p.id = wc.person_id
			WHERE wc.work_id = $1 AND wc.role = 'author'
		`, workID)
		if err != nil {
			continue
		}

		match := true
		found := make(map[string]bool)
		for authorRows.Next() {
			var lastName string
			authorRows.Scan(&lastName)
			found[strings.ToLower(lastName)] = true
		}
		authorRows.Close()

		for k := range authorSet {
			if !found[k] {
				match = false
				break
			}
		}
		for k := range found {
			if !authorSet[k] {
				match = false
				break
			}
		}

		if match {
			return workID
		}
	}
	return 0
}

func logImportError(filename, hash, errorMsg string, cfg ...*config.Config) {
	logsDir := "./logs"
	if len(cfg) > 0 && cfg[0] != nil {
		logsDir = cfg[0].Directories.Logs
	}
	os.MkdirAll(logsDir, 0755)
	logFile := fmt.Sprintf("%s/import_errors_%s.log", logsDir, time.Now().Format("2006-01-02"))
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to log import error: %v", err)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] File: %s, Hash: %s, Error: %s\n", 
		time.Now().Format("2006-01-02 15:04:05"), filename, hash, errorMsg)
}

var subdirCounter int
var subdirMutex sync.Mutex

func getNextSubdir(baseDir string) string {
	subdirMutex.Lock()
	defer subdirMutex.Unlock()

	files, _ := os.ReadDir(baseDir)
	count := 0
	for _, f := range files {
		if f.IsDir() {
			count++
		}
	}

	if count == 0 || count%100 == 0 {
		subdirCounter++
		return fmt.Sprintf("%05d", subdirCounter)
	}

	existing := make([]int, 0)
	for _, f := range files {
		if f.IsDir() {
			if n, err := strconv.Atoi(f.Name()); err == nil {
				existing = append(existing, n)
			}
		}
	}

	if len(existing) == 0 {
		subdirCounter++
		return fmt.Sprintf("%05d", subdirCounter)
	}

	maxDir := existing[0]
	for _, n := range existing {
		if n > maxDir {
			maxDir = n
		}
	}

	lastDir := filepath.Join(baseDir, fmt.Sprintf("%05d", maxDir))
	lastFiles, _ := os.ReadDir(lastDir)
	if len(lastFiles) >= 100 {
		subdirCounter = maxDir + 1
		return fmt.Sprintf("%05d", subdirCounter)
	}

	return fmt.Sprintf("%05d", maxDir)
}

func safeDirectoryPath(path string, allowedBase string) (string, error) {
	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "..") || strings.HasPrefix(cleaned, "~") {
		return "", fmt.Errorf("invalid directory path")
	}
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("path must be absolute")
	}
	info, err := os.Stat(cleaned)
	if err != nil {
		return "", fmt.Errorf("directory does not exist")
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}
	absAllowed, err := filepath.Abs(allowedBase)
	if err != nil {
		return "", fmt.Errorf("invalid allowed base path")
	}
	rel, err := filepath.Rel(absAllowed, cleaned)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("directory is not within allowed import path: %s", absAllowed)
	}
	return cleaned, nil
}

func startImport() gin.HandlerFunc {
	return func(c *gin.Context) {
		dirPath := c.PostForm("directory")
		if dirPath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Directory path not provided"})
			return
		}

		cfg := getConfig(c)
		safePath, err := safeDirectoryPath(dirPath, cfg.Directories.Bookarch)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Directory is not within allowed import path"})
			return
		}

		var files []string
		err = filepath.Walk(safePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			relPath, _ := filepath.Rel(safePath, path)
			if !isSupportedFile(relPath) {
				return nil
			}
			files = append(files, relPath)
			return nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot walk directory"})
			return
		}

		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No supported files found in directory"})
			return
		}

		err = importManager.Start(safePath, files)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"started": true, "total": len(files)})
	}
}

func getImportStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, importManager.Status())
	}
}

func cancelImport() gin.HandlerFunc {
	return func(c *gin.Context) {
		importManager.Cancel()
		c.JSON(http.StatusOK, gin.H{"cancelled": true})
	}
}

func downloadBook(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := getConfig(c)

		editionID := c.Param("id")
		mode := c.DefaultQuery("mode", "archive")

		var filePath, title string
		var onShelf bool
		err := db.QueryRow(`
			SELECT ef.file_path, e.title, COALESCE(e.on_shelf, false)
			FROM edition_files ef 
			JOIN editions e ON e.id = ef.edition_id 
			WHERE ef.edition_id = $1 AND ef.is_primary = true
		`, editionID).Scan(&filePath, &title, &onShelf)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
				return
			}
			internalError(c, err)
			return
		}

		// Serve extracted file only when explicitly requested with mode=extracted AND book is on shelf
		if mode == "extracted" && onShelf {
			shelfDir := filepath.Join(cfg.Directories.Temp, "shelf", editionID)
			entries, err := os.ReadDir(shelfDir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					extractedPath := filepath.Join(shelfDir, entry.Name())
					ext := strings.ToLower(filepath.Ext(entry.Name()))

					var contentType string
					switch ext {
					case ".fb2":
						contentType = "application/xml"
					case ".pdf":
						contentType = "application/pdf"
					case ".doc":
						contentType = "application/msword"
					case ".docx":
						contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
					case ".epub":
						contentType = "application/epub+zip"
					default:
						contentType = "application/octet-stream"
					}

					downloadName := sanitizeFilename(title) + ext
					c.Header("Content-Description", "File Transfer")
					c.Header("Content-Transfer-Encoding", "binary")
					c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", downloadName, url.QueryEscape(downloadName)))
					c.Header("Content-Type", contentType)
					c.File(extractedPath)
					return
				}
			}
			// Extracted file not found (server restart etc.), extract on demand
			if err := extractBookForShelf(db, editionID, cfg); err == nil {
				// Retry serving after extraction
				entries, err = os.ReadDir(shelfDir)
				if err == nil {
					for _, entry := range entries {
						if entry.IsDir() {
							continue
						}
						extractedPath := filepath.Join(shelfDir, entry.Name())
						ext := strings.ToLower(filepath.Ext(entry.Name()))
						downloadName := sanitizeFilename(title) + ext
						c.Header("Content-Description", "File Transfer")
						c.Header("Content-Transfer-Encoding", "binary")
						c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", downloadName, url.QueryEscape(downloadName)))
						c.Header("Content-Type", "application/octet-stream")
						c.File(extractedPath)
						return
					}
				}
			}
			// Fall through to serve ZIP
		}

		fullPath := filepath.Join(".", filePath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found on disk"})
			return
		}

		tmpDir := cfg.Directories.Temp
		os.MkdirAll(tmpDir, 0755)

		baseName := fmt.Sprintf("%s_%s_", sanitizeFilename(title), editionID)
		safeName := fmt.Sprintf("%s_%s.zip", sanitizeFilename(title), editionID)
		tmpFile, err := os.CreateTemp(tmpDir, baseName+"*.zip")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare file"})
			return
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()

		if err := copyFile(fullPath, tmpPath); err != nil {
			os.Remove(tmpPath)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare file"})
			return
		}

		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", safeName, url.QueryEscape(safeName)))
		c.Header("Content-Type", "application/zip")
		c.File(tmpPath)

		go func() {
			time.Sleep(5 * time.Second)
			os.Remove(tmpPath)
		}()
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return out.Close()
}

func sanitizeFilename(name string) string {
	repl := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return strings.TrimSpace(repl.Replace(name))
}

type UpdateShelfRequest struct {
	OnShelf bool `json:"on_shelf"`
}

func updateBookShelf(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := getConfig(c)
		editionID := c.Param("id")

		var req UpdateShelfRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.OnShelf {
			if err := extractBookForShelf(db, editionID, cfg); err != nil {
				log.Printf("Shelf extract warning for edition %s: %v", editionID, err)
			}
		} else {
			shelfDir := filepath.Join(cfg.Directories.Temp, "shelf", editionID)
			os.RemoveAll(shelfDir)
		}

		_, err := db.Exec("UPDATE editions SET on_shelf = $1, shelf_order = CASE WHEN $1 THEN COALESCE(shelf_order, 0) + 1 ELSE shelf_order END WHERE id = $2", req.OnShelf, editionID)
		if err != nil {
			internalError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Book shelf status updated"})
	}
}

func extractBookForShelf(db *sql.DB, editionID string, cfg *config.Config) error {
	var filePath string
	err := db.QueryRow(`
		SELECT ef.file_path FROM edition_files ef
		WHERE ef.edition_id = $1 AND ef.is_primary = true
	`, editionID).Scan(&filePath)
	if err != nil {
		return fmt.Errorf("file path lookup: %w", err)
	}

	fullPath := filepath.Join(".", filePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found on disk")
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("read archive: %w", err)
	}

	result, err := utils.DetectZipContent(data)
	if err != nil {
		return fmt.Errorf("extract from archive: %w", err)
	}

	var ext string
	switch result.ContentType {
	case utils.ZipContentFB2:
		ext = ".fb2"
	case utils.ZipContentPDF:
		ext = ".pdf"
	case utils.ZipContentDOC:
		ext = ".doc"
	case utils.ZipContentDOCX:
		ext = ".docx"
	case utils.ZipContentEPUB:
		ext = ".epub"
	default:
		ext = ".bin"
	}

	shelfDir := filepath.Join(cfg.Directories.Temp, "shelf", editionID)
	os.RemoveAll(shelfDir)
	os.MkdirAll(shelfDir, 0755)

	outPath := filepath.Join(shelfDir, "book"+ext)
	if err := os.WriteFile(outPath, result.Content, 0644); err != nil {
		return fmt.Errorf("write extracted file: %w", err)
	}

	return nil
}

func serveTemplate(path, name, mobileTopBar string) func(c *gin.Context, isAndroid bool) {
	tpl, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read template %s: %v", path, err)
	}
	htmlContent := string(tpl)

	mobileCSS := `<link rel="stylesheet" href="/static/css/mobile.css">`
	androidBody := `<body class="android">`
	androidJS := `<script>
(function(){
var a=document.body.classList.contains('android');
if(!a)return;
var q=function(s){return document.querySelector(s)};
var qa=function(s){return document.querySelectorAll(s)};

/* Mobile user button: show first letter of username */
function updateMobileUser(){
var btn=document.getElementById('mobileUserBtn');
if(!btn)return;
try{
var stored=localStorage.getItem('auth_user');
if(stored){
var user=JSON.parse(stored);
if(user&&user.username){
btn.textContent=user.username.charAt(0).toUpperCase();
btn.classList.add('logged-in');
return;
}
}
}catch(e){}
btn.textContent='\u2630';
btn.classList.remove('logged-in');
}
window.updateMobileUser=updateMobileUser;
updateMobileUser();
setInterval(updateMobileUser,1000);
document.getElementById('mobileUserBtn')?.addEventListener('click',function(){
if(localStorage.getItem('auth_user')){
if(confirm('Вы хотите завершить сессию пользователя?')){
localStorage.removeItem('auth_token');
localStorage.removeItem('auth_user');
window.location.reload();
}
}else{
var lb=document.getElementById('loginBtn');
if(lb)lb.click();
}
});

/* Re-apply user button when Books tab becomes active */
['books','tab-books'].forEach(function(tabId){
var el=document.getElementById(tabId);
if(!el)return;
var obs=new MutationObserver(function(){
if(el.classList.contains('active'))updateMobileUser();
});
obs.observe(el,{attributes:true,attributeFilter:['class']});
});
})();
</script>`

	return func(c *gin.Context, isAndroid bool) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		if isAndroid {
			html := strings.Replace(htmlContent, "</head>", mobileCSS+"\n</head>", 1)
			html = strings.Replace(html, "<body>", androidBody+"\n    "+mobileTopBar, 1)
			html = strings.Replace(html, "</body>", androidJS+"\n</body>", 1)
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
		} else {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(htmlContent))
		}
	}
}

func getShelfPage(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT 
				e.id,
				COALESCE(STRING_AGG(DISTINCT p.last_name || ' ' || COALESCE(p.first_name, ''), '; '), '') as authors,
				e.title as edition_title,
				ef.file_path,
				ef.file_size
			FROM editions e
			JOIN works w ON w.id = e.work_id
			LEFT JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
			LEFT JOIN persons p ON p.id = wc.person_id
			LEFT JOIN edition_files ef ON ef.edition_id = e.id AND ef.is_primary = true
			WHERE e.on_shelf = true
			GROUP BY e.id, e.title, ef.file_path, ef.file_size
			ORDER BY e.shelf_order DESC, e.title
		`)
		if err != nil {
			c.String(http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		type ShelfBook struct {
			ID          int
			Authors     string
			Title       string
			FilePath    string
			FileSize    int64
		}

		var books []ShelfBook
		for rows.Next() {
			var book ShelfBook
			var filePath sql.NullString
			var fileSize sql.NullInt64
			if err := rows.Scan(&book.ID, &book.Authors, &book.Title, &filePath, &fileSize); err != nil {
				continue
			}
			book.Authors = truncateAuthors(book.Authors)
			if filePath.Valid {
				book.FilePath = filePath.String
			}
			if fileSize.Valid {
				book.FileSize = fileSize.Int64
			}
			books = append(books, book)
		}

		page := `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Общая полка</title>
    <link rel="stylesheet" href="/static/css/style.css">
    <style>
        .shelf-table { width: 100%; border-collapse: collapse; margin-top: 20px; }
        .shelf-table th, .shelf-table td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        .shelf-table th { background: #f8f9fa; font-weight: 600; }
        .shelf-table tr:hover { background: #f5f5f5; }
        .shelf-table .size { color: #666; font-size: 12px; }
        .shelf-table .download { color: #3498db; text-decoration: none; }
        .shelf-table .download:hover { text-decoration: underline; }
        .back-link { display: inline-block; margin: 20px 0; color: #3498db; cursor: pointer; }
    </style>
</head>
<body>
    <div class="container">
        <a href="/" class="back-link" id="backLink">← Назад к библиотеке</a>
        <h1>📚 Общая полка</h1>
        <p>Книг на полке: ` + fmt.Sprintf("%d", len(books)) + `</p>
        ` + func() string {
            if len(books) > 0 {
                return `<button id="clearShelfBtn" class="btn btn-danger" onclick="clearShelf()">Очистить полку</button>`
            }
            return ""
        }() + `
        
        <table class="shelf-table">
            <thead>
                <tr>
                    <th>Автор</th>
                    <th>Произведение</th>
                    <th>Размер</th>
                    <th>Скачать</th>
                </tr>
            </thead>
            <tbody>`

		for _, book := range books {
			sizeStr := "-"
			if book.FileSize > 0 {
				if book.FileSize < 1024*1024 {
					sizeStr = fmt.Sprintf("%.1f KB", float64(book.FileSize)/1024)
				} else {
					sizeStr = fmt.Sprintf("%.1f MB", float64(book.FileSize)/(1024*1024))
				}
			}

			downloadLink := "-"
			if book.FilePath != "" {
				downloadLink = `<a href="/api/v1/books/` + fmt.Sprintf("%d", book.ID) + `/download?mode=extracted" class="download">⬇ Скачать</a>`
			}

			page += `<tr>
                <td>` + html.EscapeString(book.Authors) + `</td>
                <td>` + html.EscapeString(book.Title) + `</td>
                <td class="size">` + sizeStr + `</td>
                <td>` + downloadLink + `</td>
            </tr>`
		}

		page += `</tbody>
        </table>
        <script>
        async function clearShelf() {
            if (!confirm('Удалить все книги с полки?')) return;
            try {
                const response = await fetch('/api/v1/shelf/clear');
                if (response.ok) {
                    window.location.reload();
                } else {
                    alert('Ошибка при очистке полки');
                }
            } catch (err) {
                alert('Ошибка: ' + err.message);
            }
        }
        document.getElementById('backLink').addEventListener('click', function(e) {
            e.preventDefault();
            if (window.history.length > 1) {
                window.history.back();
            } else if (document.referrer) {
                window.location.href = document.referrer;
            } else {
                window.location.href = '/';
            }
        });
        </script>
    </div>
</body></html>`

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, page)
	}
}

func clearShelf(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := getConfig(c)

		_, err := db.Exec("UPDATE editions SET on_shelf = false, shelf_order = 0 WHERE on_shelf = true")
		if err != nil {
			internalError(c, err)
			return
		}

		shelfRoot := filepath.Join(cfg.Directories.Temp, "shelf")
		os.RemoveAll(shelfRoot)

		c.JSON(http.StatusOK, gin.H{"message": "Shelf cleared successfully"})
	}
}

func truncateAuthors(authors string) string {
	if authors == "" {
		return "Неизвестный автор"
	}
	parts := strings.Split(authors, "; ")
	if len(parts) <= 3 {
		return authors
	}
	return strings.Join(parts[:3], "; ") + " (и др)"
}

func getShelfCount(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM editions WHERE on_shelf = true").Scan(&count)
		if err != nil {
			internalError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"count": count})
	}
}

func getAppConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := getConfig(c)
		c.JSON(http.StatusOK, gin.H{
			"enable_delete": cfg.Server.EnableDelete,
		})
	}
}