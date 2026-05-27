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
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"libapp/src/config"
	"libapp/src/utils"
)

const currentDBVersion = "1.0"

type migration struct {
	Version     string
	Description string
	SQL         string
}

var migrations = []migration{
	{
		Version:     "1.0",
		Description: "Initial schema",
		SQL: `
			CREATE TABLE IF NOT EXISTS db_version (
				version     VARCHAR(20) NOT NULL,
				updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
			INSERT INTO db_version (version) VALUES ('1.0')
			ON CONFLICT DO NOTHING;
		`,
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

func runMigrations(db *sql.DB) error {
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

	// Apply pending migrations
	applied := 0
	for _, m := range sorted {
		if m.SQL == "" {
			continue
		}
		if versionGreater(m.Version, currentVer) {
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
		}
	}

	if applied > 0 {
		log.Printf("DB migration complete: %d script(s) applied", applied)
	} else {
		log.Printf("DB is up to date (version %s)", currentVer)
	}
	return nil
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

func main() {
	cfg, err := config.Load("")
	if err != nil {
		log.Fatal("Error loading config: ", err)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = cfg.DSN()
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal("Error pinging database: ", err)
	}

	log.Println("Connected to database")

	if err := runMigrations(db); err != nil {
		log.Fatal("Migration failed: ", err)
	}

	importManager = NewImportManager(db, cfg)

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("config", cfg)
		c.Next()
	})

// API routes
	api := r.Group("/api/v1")
	{
		setupOPDSRoutes(api, db)

		api.GET("/books", getBooks(db))
		api.GET("/books/search", searchBooks(db))
		api.POST("/books", createBook(db))
		api.GET("/books/:id", getBook(db))
		api.PUT("/books/:id", updateBook(db))
		api.DELETE("/books/:id", deleteBook(db))
		api.GET("/books/:id/extended", getBookExtended(db))
		api.PUT("/books/:id/extended", updateBookExtended(db))
		api.GET("/authors", getAuthors(db))
		api.PUT("/persons/:id", updatePerson(db))
		api.GET("/genres", getGenres(db))
		api.POST("/genres", createGenre(db))
		api.GET("/tags", getTags(db))
		api.POST("/tags", createTag(db))
		api.GET("/persons", getPersons(db))
		api.GET("/languages", getLanguages(db))
		api.GET("/config", getAppConfig())
		
		api.POST("/import/file", importBookFile(db))
		api.POST("/import/upload", importUploadFiles())
		api.POST("/import/directory", startImport())
		api.GET("/import/status", getImportStatus())
		api.POST("/import/cancel", cancelImport())
		api.POST("/books/:id/cover", uploadCover(db))
		api.GET("/books/:id/download", downloadBook(db))
		api.PUT("/books/:id/shelf", updateBookShelf(db))
	}

	// Serve static files
	r.Static("/static", "./static")

	// Serve templates
	r.GET("/", func(c *gin.Context) {
		c.File("./templates/index.html")
	})

	r.GET("/shelf/", getShelfPage(db))
	r.PUT("/api/v1/shelf/clear", clearShelf(db))
	r.GET("/api/v1/shelf/count", getShelfCount(db))

	// Debug endpoints (always available)
	r.GET("/debug/goroutines", func(c *gin.Context) {
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		c.Data(http.StatusOK, "text/plain; charset=utf-8", buf[:n])
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = fmt.Sprintf("%d", cfg.Server.Port)
	}
	addr := cfg.Server.Bind + ":" + port
	log.Printf("Starting server on %s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}

func uploadCover(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := getConfig(c)

		editionID := c.Param("id")

		file, header, err := c.Request.FormFile("cover")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
			return
		}
		defer file.Close()

		allowedTypes := map[string]bool{
			"image/jpeg": true,
			"image/png":  true,
			"image/webp": true,
		}
		contentType := header.Header.Get("Content-Type")
		if !allowedTypes[contentType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported file type. Use JPEG, PNG, or WebP"})
			return
		}

		var editionExists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM editions WHERE id = $1)", editionID).Scan(&editionExists)
		if err != nil || !editionExists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Edition not found"})
			return
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

		out, err := os.Create(coverPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save cover"})
			return
		}
		defer out.Close()

		_, err = io.Copy(out, file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save cover"})
			return
		}

		_, err = db.Exec("UPDATE editions SET cover_path = $1 WHERE id = $2", coverPath, editionID)
		if err != nil {
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
			"original_title":   "original_title",
			"upload_date":      "upload_date",
			"authors":          "authors",
			"available_formats": "available_formats",
		}
		sortCol, ok := allowedSorts[sortBy]
		if !ok {
			sortCol = "original_title"
		}
		if sortOrder != "desc" {
			sortOrder = "asc"
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			ORDER BY %s %s
			LIMIT $%d OFFSET $%d
		`, whereClause, sortCol, sortOrder, argNum, argNum+1)
		queryArgs := append(args, limit, offset)

		rows, err := db.Query(query, queryArgs...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			books = append(books, book)
		}

		if err = rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		sqlQuery += " ORDER BY original_title LIMIT $" + strconv.Itoa(argIndex) + " OFFSET $" + strconv.Itoa(argIndex+1)
		args = append(args, limit, offset)

		rows, err := db.Query(sqlQuery, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
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

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Get or create format (default to EPUB if not specified)
		var formatID int
		err = tx.QueryRow("SELECT id FROM formats WHERE name = 'EPUB'").Scan(&formatID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Insert edition file
		_, err = tx.Exec(`
			INSERT INTO edition_files (edition_id, format_id, file_path, is_primary)
			VALUES ($1, $2, $3, $4)
		`, editionID, formatID, "/books/manual_upload.epub", true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Delete edition (will cascade to edition_files, reading_progress, etc.)
		_, err = tx.Exec("DELETE FROM editions WHERE id = $1", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Clean up orphaned work (no remaining editions)
		_, err = tx.Exec("DELETE FROM works WHERE id = $1 AND NOT EXISTS (SELECT 1 FROM editions WHERE work_id = $1)", workID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var authors []AuthorWithBooks
		for rows.Next() {
			var author AuthorWithBooks
			if err := rows.Scan(&author.ID, &author.FirstName, &author.LastName, &author.BooksCount); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				if year.Valid {
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
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				var formats []FormatInfo
				for formatRows.Next() {
					var format FormatInfo
					if err := formatRows.Scan(&format.FormatName, &format.FilePath); err != nil {
						formatRows.Close()
						bookRows.Close()
						c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
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
	ID   int    `json:"id"`
	Name string `json:"name"`
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
	ID   int    `json:"id"`
	Name string `json:"name"`
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

		// Validate required fields
		if req.LastName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Last name is required"})
			return
		}

		// Update person
		result, err := db.Exec(`
			UPDATE persons 
			SET first_name = $1, last_name = $2
			WHERE id = $3
		`, req.FirstName, req.LastName, id)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var author AuthorData
			if err := rows.Scan(&author.ID, &author.FirstName, &author.LastName, &author.Role); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var genre GenreData
			if err := rows.Scan(&genre.ID, &genre.Name); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var file FileData
			if err := rows.Scan(&file.ID, &file.FormatID, &file.FormatName, &file.FilePath, &file.FileSize, &file.PageCount, &file.HasOCR, &file.IsPrimary); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var tag TagData
			if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var toc TOCEntryData
			if err := rows.Scan(&toc.ID, &toc.ParentID, &toc.Level, &toc.Title, &toc.Position, &toc.SortOrder); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			bookData.TOC = append(bookData.TOC, toc)
		}

		c.JSON(http.StatusOK, bookData)
	}
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

		// Update work - iterate over all provided fields
		if len(req.Work) > 0 {
			updates := []string{}
			args := []interface{}{}
			argNum := 1

			for key, value := range req.Work {
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

			if len(updates) > 0 {
				args = append(args, workID)
				_, err = tx.Exec("UPDATE works SET "+strings.Join(updates, ", ")+", updated_at = NOW() WHERE id = $"+strconv.Itoa(argNum), args...)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}
		}

		// Update edition - iterate over all provided fields
		if len(req.Edition) > 0 {
			updates := []string{}
			args := []interface{}{}
			argNum := 1

			for key, value := range req.Edition {
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

			if len(updates) > 0 {
				args = append(args, id)
				_, err = tx.Exec("UPDATE editions SET "+strings.Join(updates, ", ")+", updated_at = NOW() WHERE id = $"+strconv.Itoa(argNum), args...)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}
		}

		// Update authors - first remove all existing
		if len(req.Authors) > 0 {
			_, err = tx.Exec("DELETE FROM work_contributors WHERE work_id = $1", workID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
						c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}
		}

		// Update genres
		if len(req.Genres) > 0 {
			_, err = tx.Exec("DELETE FROM work_genres WHERE work_id = $1", workID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			for _, genreID := range req.Genres {
				_, err = tx.Exec(`
					INSERT INTO work_genres (work_id, genre_id)
					VALUES ($1, $2)
				`, workID, genreID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}
		}

		// Update tags
		if len(req.Tags) > 0 {
			_, err = tx.Exec("DELETE FROM edition_tags WHERE edition_id = $1", id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			for _, tagID := range req.Tags {
				_, err = tx.Exec(`
					INSERT INTO edition_tags (edition_id, tag_id)
					VALUES ($1, $2)
				`, id, tagID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		rows, err := db.Query("SELECT id, name FROM genres ORDER BY name")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var genres []GenreData
		for rows.Next() {
			var genre GenreData
			if err := rows.Scan(&genre.ID, &genre.Name); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, genre)
	}
}

// getTags returns all tags
func getTags(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query("SELECT id, name FROM tags ORDER BY name")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		tags := make([]TagData, 0)
		for rows.Next() {
			var tag TagData
			if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			Name string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var tag TagData
		err := db.QueryRow("INSERT INTO tags (name) VALUES ($1) RETURNING id, name", req.Name).Scan(&tag.ID, &tag.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var persons []AuthorData
		for rows.Next() {
			var person AuthorData
			if err := rows.Scan(&person.ID, &person.FirstName, &person.LastName); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			persons = append(persons, person)
		}

		c.JSON(http.StatusOK, persons)
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var languages []LanguageData
		for rows.Next() {
			var lang LanguageData
			if err := rows.Scan(&lang.Code, &lang.Name, &lang.NativeName); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			languages = append(languages, lang)
		}

		c.JSON(http.StatusOK, languages)
	}
}

// ImportBookFile handles single file upload
func importBookFile(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := getConfig(c)

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
			return
		}
		defer file.Close()

		filename := header.Filename
		ext := strings.ToLower(filepath.Ext(filename))

		tmpDir := cfg.TempBookarchDir()
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory"})
			return
		}

		data, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
			return
		}

	// Phase 1: extract content and native metadata (no LLM yet)
	var bookContent []byte
	var hashStr string
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
			logImportError(filename, "", "Failed to extract from zip: "+zipErr.Error())
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to extract book from archive"})
			return
		}
		bookContent = zipResult.Content
		zipContentType = zipResult.ContentType
		if zipResult.ContentType == utils.ZipContentFB2 {
			bookInfo, parseErr = utils.ParseFB2FromBytes(bookContent)
		}
	case ".epub":
		bookContent = data
		epubInfo, epubErr := utils.ParseEPUBFromBytes(data)
		if epubErr == nil {
			bookInfo = &utils.FB2Book{
				Title:      epubInfo.Title,
				Authors:    epubInfo.Authors,
				Lang:       epubInfo.Lang,
				Year:       epubInfo.Year,
				ISBN:       epubInfo.ISBN,
				Publisher:  epubInfo.Publisher,
				Genres:     epubInfo.Genres,
				Annotation: epubInfo.Annotation,
				Sequence:   epubInfo.Sequence,
			}
		} else {
			parseErr = epubErr
		}
	case ".pdf", ".docx", ".doc":
		bookContent = data
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unsupported file format: %s", ext)})
		return
	}

	if bookContent == nil {
		logImportError(filename, "", "Failed to extract book content")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to extract book content from archive"})
		return
	}

	// Phase 2: duplicate check (hash before LLM)
	hash := sha256.Sum256(bookContent)
	hashStr = hex.EncodeToString(hash[:])

	var existingFileID int
	var existingTitle string
	var existingAuthors string
	err = db.QueryRow(`
		SELECT ef.id, w.original_title,
			STRING_AGG(p.last_name || ' ' || COALESCE(p.first_name, ''), ', ' ORDER BY p.last_name)
		FROM edition_files ef
		JOIN editions e ON ef.edition_id = e.id
		JOIN works w ON e.work_id = w.id
		LEFT JOIN work_contributors wc ON w.id = wc.work_id AND wc.role = 'author'
		LEFT JOIN persons p ON wc.person_id = p.id
		WHERE ef.file_hash = $1
		GROUP BY ef.id, w.original_title
	`, hashStr).Scan(&existingFileID, &existingTitle, &existingAuthors)
	if err == nil {
		if existingAuthors == "" {
			existingAuthors = "Неизвестный автор"
		}
		log.Printf("Duplicate file detected: hash=%s, title='%s', authors='%s'", hashStr, existingTitle, existingAuthors)
		c.JSON(http.StatusOK, gin.H{
			"duplicate": true,
			"message":   fmt.Sprintf("Книга уже существует в библиотеке: %s — %s", existingAuthors, existingTitle),
			"file_hash": hashStr,
			"title":     existingTitle,
			"authors":   existingAuthors,
		})
		return
	}

	// Phase 3: LLM recognition for formats without native metadata
	var llmResult *utils.LLMResult

	needsLLM := ext == ".pdf" || ext == ".docx" || ext == ".doc"
	if ext == ".zip" && zipContentType != utils.ZipContentFB2 {
		needsLLM = true
	}

	if needsLLM {
		switch ext {
		case ".zip":
			switch zipContentType {
			case utils.ZipContentPDF:
				text, textErr := utils.ExtractPDFText(bookContent, 3)
				if textErr == nil {
					llmResult = recognizeBook(text, cfg)
				}
			case utils.ZipContentDOCX:
				text, textErr := utils.ExtractDOCXText(bookContent, 3)
				if textErr == nil {
					llmResult = recognizeBook(text, cfg)
				}
			case utils.ZipContentDOC:
				text, textErr := utils.ExtractDOCText(bookContent, 3)
				if textErr == nil {
					llmResult = recognizeBook(text, cfg)
				}
			}
		case ".pdf":
			text, textErr := utils.ExtractPDFText(bookContent, 3)
			if textErr == nil {
				llmResult = recognizeBook(text, cfg)
			}
		case ".docx":
			text, textErr := utils.ExtractDOCXText(bookContent, 3)
			if textErr == nil {
				llmResult = recognizeBook(text, cfg)
			}
		case ".doc":
			var text string
			var textErr error
			if len(bookContent) > 2 && bookContent[0] == 0x50 && bookContent[1] == 0x4b {
				text, textErr = utils.ExtractDOCXText(bookContent, 3)
			} else {
				text, textErr = utils.ExtractDOCText(bookContent, 3)
			}
			if textErr == nil {
				llmResult = recognizeBook(text, cfg)
			}
		}
	}

		tmpPath := filepath.Join(tmpDir, filename)
		tmpFile, err := os.Create(tmpPath)
		if err != nil {
			logImportError(filename, hashStr, "Failed to create temp file: "+err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save temp file"})
			return
		}
		tmpFile.Write(data)
		tmpFile.Close()

		destDir := cfg.Directories.Bookarch
		if err := os.MkdirAll(destDir, 0755); err != nil {
			logImportError(filename, hashStr, "Failed to create directory: "+err.Error(), cfg)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create bookarch directory"})
			return
		}

		subDir := getNextSubdir(destDir)
		if err := os.MkdirAll(filepath.Join(destDir, subDir), 0755); err != nil {
			logImportError(filename, hashStr, "Failed to create subdirectory: "+err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create subdirectory"})
			return
		}

		baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
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
			logImportError(filename, hashStr, "Failed to create zip file: "+err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create zip file"})
			return
		}

		zipWriter := zip.NewWriter(zipFile)
		fw, err := zipWriter.Create(filename)
		if err != nil {
			zipWriter.Close()
			zipFile.Close()
			logImportError(filename, hashStr, "Failed to create entry in zip: "+err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create zip entry"})
			return
		}

		if _, err := fw.Write(data); err != nil {
			zipWriter.Close()
			zipFile.Close()
			logImportError(filename, hashStr, "Failed to write to zip: "+err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write to zip"})
			return
		}

		if err := zipWriter.Close(); err != nil {
			zipFile.Close()
			logImportError(filename, hashStr, "Failed to close zip: "+err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close zip"})
			return
		}

		if err := zipFile.Close(); err != nil {
			logImportError(filename, hashStr, "Failed to close zip file: "+err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close zip file"})
			return
		}

		os.Remove(tmpPath)

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer func() {
			if err != nil {
				tx.Rollback()
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

		title := filename
		if bookInfo != nil && bookInfo.Title != "" {
			title = bookInfo.Title
		} else if llmResult != nil && llmResult.Title != "" {
			title = llmResult.Title
		}

		var authors []string
		if bookInfo != nil {
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

		workType := "novel"
		if bookInfo != nil && len(bookInfo.Genres) > 0 {
			genreName := bookInfo.Genres[0]
			switch strings.ToLower(genreName) {
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
			if y, parseErr := strconv.Atoi(bookInfo.Year); parseErr == nil {
				year = &y
			}
		}

		var workID int
		if existingWorkID > 0 {
			workID = existingWorkID
			log.Printf("Found existing work id=%d for title='%s'", workID, title)
		} else {
			err = tx.QueryRow(`
				INSERT INTO works (original_title, original_language, first_published, work_type, annotation)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id
			`, title, langCode, year, workType, "").Scan(&workID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}

		if existingWorkID == 0 {
			for _, authorName := range authors {
				firstName, lastName := utils.NormalizeAuthorName(authorName)

				var personID int
				err = tx.QueryRow(`
					INSERT INTO persons (last_name, first_name)
					VALUES ($1, $2)
					ON CONFLICT (first_name, last_name) DO NOTHING
					RETURNING id
				`, lastName, firstName).Scan(&personID)
				if err != nil {
					err = tx.QueryRow(`
						SELECT id FROM persons WHERE last_name = $1 AND (first_name = $2 OR (first_name IS NULL AND $2 = ''))
					`, lastName, firstName).Scan(&personID)
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
						return
					}
				}

				_, err = tx.Exec(`
					INSERT INTO work_contributors (work_id, person_id, role)
					VALUES ($1, $2, 'author')
					ON CONFLICT DO NOTHING
				`, workID, personID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}

			if len(authors) == 0 {
				var personID int
				err = tx.QueryRow(`
					INSERT INTO persons (last_name)
					VALUES ($1)
					ON CONFLICT (first_name, last_name) DO NOTHING
					RETURNING id
				`, "Неизвестный автор").Scan(&personID)
				if err != nil {
					err = tx.QueryRow(`SELECT id FROM persons WHERE last_name = 'Неизвестный автор'`).Scan(&personID)
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
						return
					}
				}

				_, err = tx.Exec(`
					INSERT INTO work_contributors (work_id, person_id, role)
					VALUES ($1, $2, 'author')
				`, workID, personID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}
		}

		if existingWorkID == 0 && bookInfo != nil {
			for _, genreName := range bookInfo.Genres {
				var genreID int
				err = tx.QueryRow(`
					INSERT INTO genres (name) VALUES ($1)
					ON CONFLICT (name) DO NOTHING RETURNING id
				`, genreName).Scan(&genreID)
				if err != nil {
					err = tx.QueryRow(`SELECT id FROM genres WHERE name = $1`, genreName).Scan(&genreID)
					if err != nil {
						continue
					}
				}
				_, err = tx.Exec(`
					INSERT INTO work_genres (work_id, genre_id) VALUES ($1, $2) ON CONFLICT DO NOTHING
				`, workID, genreID)
				if err != nil {
					continue
				}
			}
		}

		publisher := ""
		if bookInfo != nil {
			publisher = bookInfo.Publisher
		}

		var editionID int
		isbn := ""
		if bookInfo != nil {
			isbn = bookInfo.ISBN
		}
		err = tx.QueryRow(`
			INSERT INTO editions (work_id, title, language, publisher, year, source, quality, upload_date, isbn)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), $8)
			RETURNING id
		`, workID, title, langCode, publisher, year, "imported", "good", isbn).Scan(&editionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	var formatID int
	formatName := utils.GetFormatNameByExt(ext)
	if ext == ".zip" {
		formatName = utils.GetFormatNameFromZip(filename)
	}
	err = tx.QueryRow("SELECT id FROM formats WHERE name = $1", formatName).Scan(&formatID)
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
			VALUES ($1, $2, $3, $4, $5, $6)
		`, editionID, formatID, relPath, zipSize, hashStr, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		result := gin.H{
			"message":     "Book imported successfully",
			"work_id":      workID,
			"edition_id":   editionID,
			"file_path":    relPath,
			"title":        title,
		}

		if bookInfo != nil {
			result["parsed"] = true
			result["authors"] = bookInfo.Authors
			result["language"] = bookInfo.Lang
			if bookInfo.Year != "" {
				result["year"] = bookInfo.Year
			}
			if bookInfo.ISBN != "" {
				result["isbn"] = bookInfo.ISBN
			}
		} else if parseErr != nil {
			result["parsed"] = false
			result["parse_error"] = parseErr.Error()
		} else {
			result["parsed"] = true
			if llmResult != nil {
				result["authors"] = llmResult.Authors
			}
		}

		c.JSON(http.StatusCreated, result)
	}
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
			destPath := filepath.Join(tmpDir, fh.Filename)
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				continue
			}
			savedFiles = append(savedFiles, fh.Filename)
		}

		if len(savedFiles) == 0 {
			os.RemoveAll(tmpDir)
			c.JSON(http.StatusBadRequest, gin.H{"error": "No supported files found"})
			return
		}

		err = importManager.Start(tmpDir, savedFiles)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

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

	file, err := os.Open(filePath)
	if err != nil {
		updateFn(idx, "error", "", "Cannot open file")
		return
	}
	data, err := io.ReadAll(file)
	file.Close()
	if err != nil {
		updateFn(idx, "error", "", "Cannot read file")
		return
	}

	var bookContent []byte
	var bookInfo *utils.FB2Book
	var zipContentType utils.ZipContentType

	switch ext {
	case ".fb2":
		bookContent = data
		bookInfo, _ = utils.ParseFB2FromBytes(data)
	case ".zip":
		zipResult, zipErr := utils.DetectZipContent(data)
		if zipErr != nil {
			updateFn(idx, "error", "", "Cannot extract from zip")
			return
		}
		bookContent = zipResult.Content
		zipContentType = zipResult.ContentType
		if zipResult.ContentType == utils.ZipContentFB2 {
			bookInfo, _ = utils.ParseFB2FromBytes(bookContent)
		} else if zipResult.ContentType == utils.ZipContentEPUB {
			epubInfo, _ := utils.ParseEPUBFromBytes(bookContent)
			if epubInfo != nil {
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
		epubInfo, _ := utils.ParseEPUBFromBytes(data)
		if epubInfo != nil {
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
		updateFn(idx, "error", "", "Cannot extract book content")
		return
	}

	hash := sha256.Sum256(bookContent)
	hashStr := hex.EncodeToString(hash[:])

	var existingTitle, existingAuthors string
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
		log.Printf("Duplicate file: hash=%s, title='%s'", hashStr, existingTitle)
		updateFn(idx, "skipped", existingTitle, "")
		return
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
	if bookInfo != nil && bookInfo.Title != "" {
		title = bookInfo.Title
	} else if llmResult != nil && llmResult.Title != "" {
		title = llmResult.Title
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
	os.MkdirAll(destDir, 0755)

	subDir := getNextSubdir(destDir)
	if err := os.MkdirAll(filepath.Join(destDir, subDir), 0755); err != nil {
		updateFn(idx, "error", "", "Cannot create subdirectory")
		return
	}

	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	zipName := baseName + ".zip"
	destPath := filepath.Join(destDir, subDir, zipName)

	idxUnique := 1
	for {
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			break
		}
		zipName = fmt.Sprintf("%s_%d.zip", baseName, idxUnique)
		destPath = filepath.Join(destDir, subDir, zipName)
		idxUnique++
	}

	zipFile, err := os.Create(destPath)
	if err != nil {
		updateFn(idx, "error", "", "Cannot create zip file")
		return
	}

	zipWriter := zip.NewWriter(zipFile)
	fw, err := zipWriter.Create(filepath.Base(filename))
	if err != nil {
		zipWriter.Close()
		zipFile.Close()
		updateFn(idx, "error", "", "Cannot create zip entry")
		return
	}

	if _, err := fw.Write(data); err != nil {
		zipWriter.Close()
		zipFile.Close()
		updateFn(idx, "error", "", "Cannot write to zip")
		return
	}

	if err := zipWriter.Close(); err != nil {
		zipFile.Close()
		updateFn(idx, "error", "", "Cannot close zip writer")
		return
	}

	if err := zipFile.Close(); err != nil {
		updateFn(idx, "error", "", "Cannot close zip file")
		return
	}

	tx, err := db.Begin()
	if err != nil {
		updateFn(idx, "error", "", "Database error")
		return
	}
	defer func() {
		if err != nil {
			tx.Rollback()
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
	var year *int
	if bookInfo != nil && bookInfo.Year != "" {
		if y, parseErr := strconv.Atoi(bookInfo.Year); parseErr == nil {
			year = &y
		}
	}

	var workID int
	var createdWork bool
	if existingWorkID > 0 {
		workID = existingWorkID
		log.Printf("Found existing work id=%d for title='%s'", workID, title)
	} else {
		createdWork = true
		err = tx.QueryRow(`
			INSERT INTO works (original_title, original_language, first_published, work_type)
			VALUES ($1, $2, $3, $4) RETURNING id
		`, title, langCode, year, workType).Scan(&workID)
		if err != nil {
			updateFn(idx, "error", "", err.Error())
			return
		}
	}

	if createdWork {
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
					updateFn(idx, "error", "", "Author error")
					return
				}
			}
			_, err = tx.Exec(`INSERT INTO work_contributors (work_id, person_id, role) VALUES ($1,$2,'author') ON CONFLICT DO NOTHING`,
				workID, personID)
			if err != nil {
				updateFn(idx, "error", "", "Contributor error")
				return
			}
		}
	}

	if createdWork && bookInfo != nil {
		for _, genreName := range bookInfo.Genres {
			var genreID int
			err = tx.QueryRow(`
				INSERT INTO genres (name) VALUES ($1)
				ON CONFLICT (name) DO NOTHING RETURNING id
			`, genreName).Scan(&genreID)
			if err != nil {
				err = tx.QueryRow(`SELECT id FROM genres WHERE name = $1`, genreName).Scan(&genreID)
				if err != nil {
					continue
				}
			}
			_, err = tx.Exec(`
				INSERT INTO work_genres (work_id, genre_id) VALUES ($1, $2) ON CONFLICT DO NOTHING
			`, workID, genreID)
			if err != nil {
				continue
			}
		}
	}

	var editionID int
	publisher := ""
	isbn := ""
	if bookInfo != nil {
		publisher = bookInfo.Publisher
		isbn = bookInfo.ISBN
	}
	err = tx.QueryRow(`
		INSERT INTO editions (work_id, title, language, publisher, year, source, quality, upload_date, isbn)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),$8) RETURNING id
	`, workID, title, langCode, publisher, year, "imported", "good", isbn).Scan(&editionID)
	if err != nil {
		updateFn(idx, "error", "", "Edition error")
		return
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

	relPath := filepath.Join(filepath.Base(cfg.Directories.Bookarch), subDir, zipName)
	_, err = tx.Exec(`
		INSERT INTO edition_files (edition_id, format_id, file_path, file_size, file_hash, is_primary)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, editionID, formatID, relPath, len(bookContent), hashStr, true)
	if err != nil {
		updateFn(idx, "error", "", "File entry error")
		return
	}

	updateFn(idx, "done", title, "")
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

func startImport() gin.HandlerFunc {
	return func(c *gin.Context) {
		dirPath := c.PostForm("directory")
		if dirPath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Directory path not provided"})
			return
		}

		var files []string
		err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			relPath, _ := filepath.Rel(dirPath, path)
			if !isSupportedFile(relPath) {
				return nil
			}
			files = append(files, relPath)
			return nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot walk directory: " + err.Error()})
			return
		}

		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No supported files found in directory"})
			return
		}

		err = importManager.Start(dirPath, files)
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

		var filePath, title string
		err := db.QueryRow(`
			SELECT ef.file_path, e.title 
			FROM edition_files ef 
			JOIN editions e ON e.id = ef.edition_id 
			WHERE ef.edition_id = $1 AND ef.is_primary = true
		`, editionID).Scan(&filePath, &title)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		fullPath := filepath.Join(".", filePath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found on disk"})
			return
		}

		tmpDir := cfg.Directories.Temp
		os.MkdirAll(tmpDir, 0755)

		safeName := fmt.Sprintf("%s_%s.zip", sanitizeFilename(title), editionID)
		tmpPath := filepath.Join(tmpDir, safeName)

		if err := copyFile(fullPath, tmpPath); err != nil {
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
		editionID := c.Param("id")

		var req UpdateShelfRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec("UPDATE editions SET on_shelf = $1, shelf_order = CASE WHEN $1 THEN COALESCE(shelf_order, 0) + 1 ELSE shelf_order END WHERE id = $2", req.OnShelf, editionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Book shelf status updated"})
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
			if filePath.Valid {
				book.FilePath = filePath.String
			}
			if fileSize.Valid {
				book.FileSize = fileSize.Int64
			}
			books = append(books, book)
		}

		html := `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Моя полка</title>
    <link rel="stylesheet" href="/static/css/style.css">
    <style>
        .shelf-table { width: 100%; border-collapse: collapse; margin-top: 20px; }
        .shelf-table th, .shelf-table td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        .shelf-table th { background: #f8f9fa; font-weight: 600; }
        .shelf-table tr:hover { background: #f5f5f5; }
        .shelf-table .size { color: #666; font-size: 12px; }
        .shelf-table .download { color: #3498db; text-decoration: none; }
        .shelf-table .download:hover { text-decoration: underline; }
        .back-link { display: inline-block; margin: 20px 0; color: #3498db; }
    </style>
</head>
<body>
    <div class="container">
        <a href="/" class="back-link">← Назад к библиотеке</a>
        <h1>📚 Моя полка</h1>
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
				downloadLink = `<a href="/api/v1/books/` + fmt.Sprintf("%d", book.ID) + `/download" class="download">⬇ Скачать</a>`
			}

			html += `<tr>
                <td>` + escapeXML(book.Authors) + `</td>
                <td>` + escapeXML(book.Title) + `</td>
                <td class="size">` + sizeStr + `</td>
                <td>` + downloadLink + `</td>
            </tr>`
		}

		html += `</tbody>
        </table>
        <script>
        async function clearShelf() {
            if (!confirm('Удалить все книги с полки?')) return;
            try {
                const response = await fetch('/api/v1/shelf/clear', { method: 'PUT' });
                if (response.ok) {
                    window.location.reload();
                } else {
                    alert('Ошибка при очистке полки');
                }
            } catch (err) {
                alert('Ошибка: ' + err.message);
            }
        }
        </script>
    </div>
</body>
</html>`

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, html)
	}
}

func clearShelf(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, err := db.Exec("UPDATE editions SET on_shelf = false, shelf_order = 0 WHERE on_shelf = true")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Shelf cleared successfully"})
	}
}

func getShelfCount(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM editions WHERE on_shelf = true").Scan(&count)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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