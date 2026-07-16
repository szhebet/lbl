package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"libapp/src/config"
)

func TestMain(m *testing.M) {
	// Set up test environment
	os.Setenv("DATABASE_URL", "host=localhost port=5432 user=postgres password=postgres dbname=library sslmode=disable")
	os.Setenv("PORT", "8081")

	// Run tests
	code := m.Run()
	os.Exit(code)
}

func setupTestDB() *sql.DB {
	dbURL := os.Getenv("DATABASE_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	// Use a temp dir for test backups so migrations pass
	tmpBackup, err := os.MkdirTemp("", "library-test-backup")
	if err != nil {
		panic(err)
	}

	testCfg := config.DefaultConfig()
	testCfg.Directories.Backup = tmpBackup
	testCfg.Database.Host = "localhost"
	testCfg.Database.Port = 5432
	testCfg.Database.User = "postgres"
	testCfg.Database.Password = "postgres"
	testCfg.Database.Name = "library"
	testCfg.Database.SSLMode = "disable"

	if err := runMigrations(db, testCfg); err != nil {
		panic(err)
	}

	return db
}

func TestGetBooks(t *testing.T) {
	// Set up Gin in test mode
	gin.SetMode(gin.TestMode)

	// Create a router
	r := gin.New()

	// Set up test database
	db := setupTestDB()
	defer db.Close()

	// Add middleware
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	// Register route
	r.GET("/api/v1/books", getBooks(db))

	// Create a request to test the endpoint
	req, _ := http.NewRequest("GET", "/api/v1/books", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Check the response
	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Total  int            `json:"total"`
		Limit  string         `json:"limit"`
		Offset string         `json:"offset"`
		Books  []BookDetails  `json:"books"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(response.Books), 1)
}

func TestGetBook(t *testing.T) {
	// Set up Gin in test mode
	gin.SetMode(gin.TestMode)

	// Create a router
	r := gin.New()

	// Set up test database
	db := setupTestDB()
	defer db.Close()

	// Add middleware
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	// Register routes
	r.GET("/api/v1/books", getBooks(db))
	r.GET("/api/v1/books/:id", getBook(db))

	// First, get a list of books to get a valid ID
	req, _ := http.NewRequest("GET", "/api/v1/books", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Total  int           `json:"total"`
		Limit  string        `json:"limit"`
		Offset string        `json:"offset"`
		Books  []BookDetails `json:"books"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Greater(t, len(response.Books), 0)

	// Now test getting a specific book by ID
	testID := response.Books[0].EditionID
	req, _ = http.NewRequest("GET", "/api/v1/books/", nil)
	req.URL.Path = "/api/v1/books/" + strconv.Itoa(testID)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Check the response
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse the response
	var book BookDetails
	err = json.Unmarshal(w.Body.Bytes(), &book)
	assert.NoError(t, err)
	assert.Equal(t, testID, book.EditionID)
}

func TestCreateBook(t *testing.T) {
	// Set up Gin in test mode
	gin.SetMode(gin.TestMode)

	// Create a router
	r := gin.New()

	// Set up test database
	db := setupTestDB()
	defer db.Close()

	// Add middleware
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	// Register route
	r.POST("/api/v1/books", createBook(db))

	// Create first book
	newBook1 := CreateBookRequest{
		Title:       "Test Book Part 1",
		Author:      "Multi Author",
		ISBN:        "1234567890123",
		PublishedYear: 2023,
		Genre:       "Test",
		Description: "First test book created via API",
		Language:    "eng",
	}

	bookJSON1, _ := json.Marshal(newBook1)
	req1, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req1.Body = io.NopCloser(bytes.NewReader(bookJSON1))
	req1.Header.Set("Content-Type", "application/json")

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusCreated, w1.Code)

	var createdBook1 BookDetails
	err := json.Unmarshal(w1.Body.Bytes(), &createdBook1)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, createdBook1.EditionID)
	assert.Equal(t, "Test Book Part 1", createdBook1.EditionTitle)

	// Create second book with SAME author - should reuse existing author
	newBook2 := CreateBookRequest{
		Title:       "Test Book Part 2",
		Author:      "Multi Author",
		ISBN:        "1234567890124",
		PublishedYear: 2024,
		Genre:       "Test",
		Description: "Second test book created via API",
		Language:    "eng",
	}

	bookJSON2, _ := json.Marshal(newBook2)
	req2, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req2.Body = io.NopCloser(bytes.NewReader(bookJSON2))
	req2.Header.Set("Content-Type", "application/json")

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusCreated, w2.Code)

	var createdBook2 BookDetails
	err = json.Unmarshal(w2.Body.Bytes(), &createdBook2)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, createdBook2.EditionID)
	assert.Equal(t, "Test Book Part 2", createdBook2.EditionTitle)

	// Verify both books are associated with the same author
	assert.Equal(t, createdBook1.Authors.String, createdBook2.Authors.String)

	// Verify in database that only ONE author exists with this name
	var authorCount int
	err = db.QueryRow("SELECT COUNT(*) FROM persons WHERE last_name = 'Multi Author'").Scan(&authorCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, authorCount, "Should be only one author with name 'Multi Author'")
}

func TestUpdateBook(t *testing.T) {
	// Set up Gin in test mode
	gin.SetMode(gin.TestMode)

	// Create a router
	r := gin.New()

	// Set up test database
	db := setupTestDB()
	defer db.Close()

	// Add middleware
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	// Register routes
	r.POST("/api/v1/books", createBook(db))
	r.PUT("/api/v1/books/:id", updateBook(db))
	r.GET("/api/v1/books/:id", getBook(db))

	// First, create a book to update
	newBook := CreateBookRequest{
		Title:       "Book to Update",
		Author:      "Updater",
		ISBN:        "9876543210987",
		PublishedYear: 2022,
		Genre:       "Update Test",
		Description: "A book that will be updated",
		Language:    "eng",
	}

	bookJSON, _ := json.Marshal(newBook)
	req, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req.Body = io.NopCloser(bytes.NewReader(bookJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var createdBook BookDetails
	err := json.Unmarshal(w.Body.Bytes(), &createdBook)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, createdBook.EditionID)

	// Now update the book
	updateReq := UpdateBookRequest{
		Title:       func() *string { s := "Updated Book Title"; return &s }(),
		Description: func() *string { s := "Updated description"; return &s }(),
	}

	updateJSON, _ := json.Marshal(updateReq)
	req, _ = http.NewRequest("PUT", "/api/v1/books/", nil)
	req.URL.Path = "/api/v1/books/" + strconv.Itoa(createdBook.EditionID)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON))
	req.Header.Set("Content-Type", "application/json")

	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Check the response
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse the updated book
	var updatedBook BookDetails
	err = json.Unmarshal(w.Body.Bytes(), &updatedBook)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Book Title", updatedBook.EditionTitle)
}

func TestDeleteBook(t *testing.T) {
	// Set up Gin in test mode
	gin.SetMode(gin.TestMode)

	// Create a router
	r := gin.New()

	// Set up test database
	db := setupTestDB()
	defer db.Close()

	// Add middleware
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	// Register routes
	r.POST("/api/v1/books", createBook(db))
	r.DELETE("/api/v1/books/:id", deleteBook(db))
	r.GET("/api/v1/books/:id", getBook(db))

	// First, create a book to delete
	newBook := CreateBookRequest{
		Title:       "Book to Delete",
		Author:      "Deleter",
		ISBN:        "1111111111111",
		PublishedYear: 2021,
		Genre:       "Delete Test",
		Description: "A book that will be deleted",
		Language:    "eng",
	}

	bookJSON, _ := json.Marshal(newBook)
	req, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req.Body = io.NopCloser(bytes.NewReader(bookJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var createdBook BookDetails
	err := json.Unmarshal(w.Body.Bytes(), &createdBook)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, createdBook.EditionID)

	// Now delete the book
	req, _ = http.NewRequest("DELETE", "/api/v1/books/", nil)
	req.URL.Path = "/api/v1/books/" + strconv.Itoa(createdBook.EditionID)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Check the response
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify the book is gone
	req, _ = http.NewRequest("GET", "/api/v1/books/", nil)
	req.URL.Path = "/api/v1/books/" + strconv.Itoa(createdBook.EditionID)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateBookExtended(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/books", createBook(db))
	r.GET("/books/:id/extended", getBookExtended(db))
	r.PUT("/books/:id/extended", updateBookExtended(db))

	// Create a test book first
	newBook := CreateBookRequest{
		Title:       "Original Title",
		Author:      "Original Author",
		Language:    "eng",
	}
	bookJSON, _ := json.Marshal(newBook)
	req, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req.Body = io.NopCloser(bytes.NewReader(bookJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var created BookDetails
	err := json.Unmarshal(w.Body.Bytes(), &created)
	assert.NoError(t, err)
	editionID := created.EditionID
	_ = created.WorkID // Keep for reference

	// Test 1: Update title
	updateReq1 := map[string]interface{}{
		"work": map[string]interface{}{
			"original_title": "Updated Title",
		},
		"edition": map[string]interface{}{},
		"authors":  []map[string]interface{}{},
		"genres":   []int{},
		"tags":     []int{},
	}
	updateJSON1, _ := json.Marshal(updateReq1)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON1))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify title was updated
	req, _ = http.NewRequest("GET", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var extended map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &extended)
	work := extended["work"].(map[string]interface{})
	assert.Equal(t, "Updated Title", work["original_title"])

	// Test 2: Update year (edition)
	updateReq2 := map[string]interface{}{
		"work":     map[string]interface{}{},
		"edition": map[string]interface{}{"year": 2025},
		"authors":  []map[string]interface{}{},
		"genres":   []int{},
		"tags":     []int{},
	}
	updateJSON2, _ := json.Marshal(updateReq2)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON2))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify year was updated
	req, _ = http.NewRequest("GET", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &extended)
	edition := extended["edition"].(map[string]interface{})
	year := edition["year"].(map[string]interface{})
	assert.Equal(t, float64(2025), year["Int64"])

	// Test 3: Update annotation
	updateReq3 := map[string]interface{}{
		"work": map[string]interface{}{
			"annotation": "New annotation text",
		},
		"edition": map[string]interface{}{},
		"authors":  []map[string]interface{}{},
		"genres":   []int{},
		"tags":     []int{},
	}
	updateJSON3, _ := json.Marshal(updateReq3)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON3))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify annotation was updated
	req, _ = http.NewRequest("GET", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &extended)
	work = extended["work"].(map[string]interface{})
	annotation := work["annotation"].(map[string]interface{})
	assert.Equal(t, "New annotation text", annotation["String"])

	// Test 4: Update author
	// First create a new person (or use existing)
	_, err = db.Exec(`
		INSERT INTO persons (first_name, last_name) VALUES ('New', 'Author')
		ON CONFLICT (first_name, last_name) DO NOTHING
	`)
	assert.NoError(t, err)

	var newAuthorID int
	err = db.QueryRow("SELECT id FROM persons WHERE last_name = 'Author'").Scan(&newAuthorID)
	assert.NoError(t, err)

	updateReq4 := map[string]interface{}{
		"work":     map[string]interface{}{},
		"edition":  map[string]interface{}{},
		"authors":  []map[string]interface{}{{"id": newAuthorID, "role": "author"}},
		"genres":   []int{},
		"tags":     []int{},
	}
	updateJSON4, _ := json.Marshal(updateReq4)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON4))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify author was updated
	req, _ = http.NewRequest("GET", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &extended)
	authors := extended["authors"].([]interface{})
	assert.Greater(t, len(authors), 0)
}

func TestUpdateBookExtendedPreservesUniqueFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/books", createBook(db))
	r.PUT("/books/:id/extended", updateBookExtended(db))
	r.GET("/books/:id/extended", getBookExtended(db))

	// Create book with ISBN
	newBook := CreateBookRequest{
		Title:    "Book With ISBN Test",
		Author:   "UniqueField Author",
		Language: "eng",
	}
	bookJSON, _ := json.Marshal(newBook)
	req, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req.Body = io.NopCloser(bytes.NewReader(bookJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var created BookDetails
	err := json.Unmarshal(w.Body.Bytes(), &created)
	assert.NoError(t, err)
	editionID := created.EditionID

	// Get current ISBN value
	var currentISBN sql.NullString
	err = db.QueryRow("SELECT isbn FROM editions WHERE id = $1", editionID).Scan(&currentISBN)
	assert.NoError(t, err)

	// Update ONLY the title (ISBN should remain unchanged)
	updateReq := map[string]interface{}{
		"work": map[string]interface{}{
			"original_title": "Updated Title Only",
		},
		"edition":  map[string]interface{}{},
		"authors":  []map[string]interface{}{},
		"genres":   []int{},
		"tags":     []int{},
	}
	updateJSON, _ := json.Marshal(updateReq)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should succeed without ISBN error
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify title was updated
	req, _ = http.NewRequest("GET", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var extended map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &extended)
	work := extended["work"].(map[string]interface{})
	assert.Equal(t, "Updated Title Only", work["original_title"])

	// Verify ISBN is still the same (unchanged)
	var isbnAfter sql.NullString
	errAfter := db.QueryRow("SELECT isbn FROM editions WHERE id = $1", editionID).Scan(&isbnAfter)
	assert.NoError(t, errAfter)
	assert.Equal(t, currentISBN, isbnAfter)
}

func TestUpdateBookExtendedWithEmptyUniqueFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/books", createBook(db))
	r.PUT("/books/:id/extended", updateBookExtended(db))
	r.GET("/books/:id/extended", getBookExtended(db))

	// Create book without ISBN
	newBook := CreateBookRequest{
		Title:    "Book Without ISBN",
		Author:   "Empty ISBN Author",
		Language: "eng",
	}
	bookJSON, _ := json.Marshal(newBook)
	req, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req.Body = io.NopCloser(bytes.NewReader(bookJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var created BookDetails
	err := json.Unmarshal(w.Body.Bytes(), &created)
	assert.NoError(t, err)
	editionID := created.EditionID

	// Update title when ISBN is empty - should NOT try to set ISBN to NULL
	updateReq := map[string]interface{}{
		"work": map[string]interface{}{
			"original_title": "Updated Title Empty ISBN",
		},
		"edition":  map[string]interface{}{},
		"authors":  []map[string]interface{}{},
		"genres":   []int{},
		"tags":     []int{},
	}
	updateJSON, _ := json.Marshal(updateReq)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify ISBN is still NULL
	var isbn sql.NullString
	err = db.QueryRow("SELECT isbn FROM editions WHERE id = $1", editionID).Scan(&isbn)
	assert.NoError(t, err)
	assert.False(t, isbn.Valid)
}

func TestUpdateBookExtendedDuplicateISBN(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/books", createBook(db))
	r.PUT("/books/:id/extended", updateBookExtended(db))

	// Create two books
	newBook1 := CreateBookRequest{
		Title:    "Book One",
		Author:   "Author One",
		Language: "eng",
	}
	bookJSON1, _ := json.Marshal(newBook1)
	req1, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req1.Body = io.NopCloser(bytes.NewReader(bookJSON1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusCreated, w1.Code)

	var book1 BookDetails
	json.Unmarshal(w1.Body.Bytes(), &book1)

	newBook2 := CreateBookRequest{
		Title:    "Book Two",
		Author:   "Author Two",
		Language: "eng",
	}
	bookJSON2, _ := json.Marshal(newBook2)
	req2, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req2.Body = io.NopCloser(bytes.NewReader(bookJSON2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusCreated, w2.Code)

	var book2 BookDetails
	json.Unmarshal(w2.Body.Bytes(), &book2)

	// Clear any existing ISBN first, then set
	isbnValue := "9780123456789"
	_, errUpdate := db.Exec("UPDATE editions SET isbn = NULL WHERE isbn = $1", isbnValue)
	assert.NoError(t, errUpdate)
	_, errUpdate = db.Exec("UPDATE editions SET isbn = $1 WHERE id = $2", isbnValue, book1.EditionID)
	assert.NoError(t, errUpdate)

	// Try to set same ISBN on book2 - should fail
	updateReq := map[string]interface{}{
		"work":     map[string]interface{}{},
		"edition":  map[string]interface{}{"isbn": isbnValue},
		"authors":  []map[string]interface{}{},
		"genres":   []int{},
		"tags":     []int{},
	}
	updateJSON, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest("PUT", "/books/"+strconv.Itoa(book2.EditionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Внутренняя ошибка сервера")
}

func TestUpdateBookExtendedTitleChange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/books", createBook(db))
	r.PUT("/books/:id/extended", updateBookExtended(db))
	r.GET("/books/:id/extended", getBookExtended(db))

	// Create book
	newBook := CreateBookRequest{
		Title:    "Original Book Title",
		Author:   "Title Change Author",
		Language: "eng",
	}
	bookJSON, _ := json.Marshal(newBook)
	req, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req.Body = io.NopCloser(bytes.NewReader(bookJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var created BookDetails
	err := json.Unmarshal(w.Body.Bytes(), &created)
	assert.NoError(t, err)
	editionID := created.EditionID

	// Update only the edition title (not ISBN)
	updateReq := map[string]interface{}{
		"work": map[string]interface{}{},
		"edition": map[string]interface{}{
			"title": "New Edition Title",
		},
		"authors":  []map[string]interface{}{},
		"genres":   []int{},
		"tags":     []int{},
	}
	updateJSON, _ := json.Marshal(updateReq)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should succeed without any unique constraint errors
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify title was updated
	req, _ = http.NewRequest("GET", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var extended map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &extended)
	edition := extended["edition"].(map[string]interface{})
	assert.Equal(t, "New Edition Title", edition["title"])
}

func TestUpdateBookExtendedIgnoresEmptyStrings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/books", createBook(db))
	r.PUT("/books/:id/extended", updateBookExtended(db))
	r.GET("/books/:id/extended", getBookExtended(db))

	newBook := CreateBookRequest{
		Title:    "Test Empty Strings",
		Author:   "EmptyStr Author",
		Language: "eng",
	}
	bookJSON, _ := json.Marshal(newBook)
	req, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req.Body = io.NopCloser(bytes.NewReader(bookJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var created BookDetails
	err := json.Unmarshal(w.Body.Bytes(), &created)
	assert.NoError(t, err)
	editionID := created.EditionID

	isbnVal := fmt.Sprintf("978%010d", editionID)
	_, _ = db.Exec("UPDATE editions SET isbn = NULL WHERE isbn = $1", isbnVal)
	_, err = db.Exec("UPDATE editions SET isbn = $1 WHERE id = $2", isbnVal, editionID)
	assert.NoError(t, err)

	updateReq := map[string]interface{}{
		"work": map[string]interface{}{
			"original_title": "Updated Title",
		},
		"edition": map[string]interface{}{
			"isbn": "",
			"ean":  "",
			"udc":  "",
			"bbk":  "",
		},
		"authors": []map[string]interface{}{},
		"genres":  []int{},
		"tags":    []int{},
	}
	updateJSON, _ := json.Marshal(updateReq)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var isbn sql.NullString
	err = db.QueryRow("SELECT isbn FROM editions WHERE id = $1", editionID).Scan(&isbn)
	assert.NoError(t, err)
	assert.True(t, isbn.Valid)
	assert.Equal(t, isbnVal, isbn.String)
}

func TestUpdateBookExtendedCorruptedISBN(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/books", createBook(db))
	r.PUT("/books/:id/extended", updateBookExtended(db))

	newBook := CreateBookRequest{
		Title:    "Corrupted ISBN Test",
		Author:   "Corrupt Author",
		Language: "eng",
	}
	bookJSON, _ := json.Marshal(newBook)
	req, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req.Body = io.NopCloser(bytes.NewReader(bookJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var created BookDetails
	err := json.Unmarshal(w.Body.Bytes(), &created)
	assert.NoError(t, err)
	editionID := created.EditionID

	isbnVal := fmt.Sprintf("978%010d1", editionID)
	_, _ = db.Exec("UPDATE editions SET isbn = NULL WHERE isbn = $1", isbnVal)
	_, err = db.Exec("UPDATE editions SET isbn = $1 WHERE id = $2", isbnVal, editionID)
	assert.NoError(t, err)

	updateReq := map[string]interface{}{
		"work":    map[string]interface{}{},
		"edition": map[string]interface{}{"isbn": isbnVal},
		"authors": []map[string]interface{}{},
		"genres":  []int{},
		"tags":    []int{},
	}
	updateJSON, _ := json.Marshal(updateReq)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var isbn string
	err = db.QueryRow("SELECT isbn FROM editions WHERE id = $1", editionID).Scan(&isbn)
	assert.NoError(t, err)
	assert.Equal(t, isbnVal, isbn)
}

func TestImportBookFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/import/file", importBookFile(db))

	testFile := filepath.Join(os.Getenv("HOME"), "git/aitest/agents/lbl/example", "ponedelnikNachVSubbotu.fb2")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file not found:", testFile)
	}

	file, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "ponedelnikNachVSubbotu.fb2")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		t.Fatalf("Failed to copy file: %v", err)
	}
	writer.Close()

	req, _ := http.NewRequest("POST", "/api/v1/import/file", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["parsed"].(bool))
	assert.Equal(t, "Понедельник начинается в субботу", response["title"])
	assert.Equal(t, "ru", response["language"])
	assert.Equal(t, "1964", response["year"])

	if authors, ok := response["authors"].([]interface{}); ok {
		assert.Greater(t, len(authors), 0)
	}

	assert.NotNil(t, response["work_id"])
	assert.NotNil(t, response["edition_id"])
}

func TestSearchBooks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.GET("/api/v1/books/search", searchBooks(db))

	req, _ := http.NewRequest("GET", "/api/v1/books/search?q=test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Total  int           `json:"total"`
		Limit  string        `json:"limit"`
		Offset string        `json:"offset"`
		Books  []BookDetails `json:"books"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "20", response.Limit)
	assert.Equal(t, "0", response.Offset)
}

func TestSearchBooksWithFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.GET("/api/v1/books/search", searchBooks(db))

	req, _ := http.NewRequest("GET", "/api/v1/books/search?genre=fiction&language=eng&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Total  int           `json:"total"`
		Limit  string        `json:"limit"`
		Offset string        `json:"offset"`
		Books  []BookDetails `json:"books"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "10", response.Limit)
}

func TestUploadCoverNoFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.POST("/api/v1/books/:id/cover", uploadCover(db))

	req, _ := http.NewRequest("POST", "/api/v1/books/1/cover", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUploadCoverUnsupportedFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.POST("/api/v1/books/:id/cover", uploadCover(db))

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("cover", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	part.Write([]byte("test"))
	writer.Close()

	req, _ := http.NewRequest("POST", "/api/v1/books/1/cover", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportBookFileUnsupportedFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/import/file", importBookFile(db))

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	part.Write([]byte("test content"))
	writer.Close()

	req, _ := http.NewRequest("POST", "/api/v1/import/file", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "Unsupported")
}

func TestImportBookFileNoFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/import/file", importBookFile(db))

	req, _ := http.NewRequest("POST", "/api/v1/import/file", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "No file")
}

func TestGetBookExtendedReturnsWorkAndEdition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.GET("/api/v1/books/:id/extended", getBookExtended(db))

	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM editions").Scan(&total)
	if err != nil || total == 0 {
		t.Skip("No books in database to test")
	}

	var editionID int
	db.QueryRow("SELECT id FROM editions LIMIT 1").Scan(&editionID)

	req, _ := http.NewRequest("GET", "/api/v1/books/"+strconv.Itoa(editionID)+"/extended", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	work, hasWork := response["work"]
	edition, hasEdition := response["edition"]
	assert.True(t, hasWork, "Response should have 'work' field")
	assert.True(t, hasEdition, "Response should have 'edition' field")
	assert.NotNil(t, work, "Work should not be null")
	assert.NotNil(t, edition, "Edition should not be null")
}

func TestGetAuthors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.GET("/api/v1/authors", getAuthors(db))

	req, _ := http.NewRequest("GET", "/api/v1/authors", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Authors       []AuthorWithBooks `json:"authors"`
		Total         int              `json:"total"`
		Page          int              `json:"page"`
		Limit         int              `json:"limit"`
		TotalWorks    int              `json:"total_works"`
		TotalEditions int              `json:"total_editions"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, response.Total, 0)
	assert.GreaterOrEqual(t, response.Page, 1)
	assert.GreaterOrEqual(t, response.Limit, 1)
	assert.GreaterOrEqual(t, response.TotalEditions, 0)

	if len(response.Authors) > 0 {
		author := response.Authors[0]
		assert.NotEmpty(t, author.LastName)
		assert.GreaterOrEqual(t, author.ID, 1)
	}
}

func TestGetGenres(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.GET("/api/v1/genres", getGenres(db))

	req, _ := http.NewRequest("GET", "/api/v1/genres", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var genres []GenreData
	err := json.Unmarshal(w.Body.Bytes(), &genres)
	assert.NoError(t, err)

	if len(genres) > 0 {
		assert.NotEmpty(t, genres[0].Name)
		assert.GreaterOrEqual(t, genres[0].ID, 1)
	}
}

func TestGetGenreTree(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.GET("/api/v1/genres/tree", getGenreTree(db))

	req, _ := http.NewRequest("GET", "/api/v1/genres/tree", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Genres []GenreWithAuthors `json:"genres"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	if len(response.Genres) > 0 {
		g := response.Genres[0]
		assert.NotEmpty(t, g.Name)
		assert.NotNil(t, g.Children)
	}
}

func TestSortByYear(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.GET("/api/v1/books", getBooks(db))

	t.Run("DESC year sorting", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/books?sort_by=year&sort_order=desc&limit=100", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Books []BookDetails `json:"books"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		noYearFound := false
		lastYear := 9999
		for _, b := range response.Books {
			if b.Year.Valid {
				if noYearFound {
					t.Error("Book with year found after book without year in DESC order")
				}
				y := int(b.Year.Int64)
				if y > lastYear {
					t.Errorf("Not sorted DESC: %d > %d", y, lastYear)
				}
				lastYear = y
			} else {
				noYearFound = true
			}
		}
	})

	t.Run("ASC year sorting", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/books?sort_by=year&sort_order=asc&limit=100", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Books []BookDetails `json:"books"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		noYearFound := false
		lastYear := 0
		for _, b := range response.Books {
			if b.Year.Valid {
				if noYearFound {
					t.Error("Book with year found after book without year in ASC order")
				}
				y := int(b.Year.Int64)
				if y < lastYear {
					t.Errorf("Not sorted ASC: %d < %d", y, lastYear)
				}
				lastYear = y
			} else {
				noYearFound = true
			}
		}
	})
}

func TestGetTagsPersonsLanguages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.GET("/api/v1/tags", getTags(db))
	r.GET("/api/v1/persons", getPersons(db))
	r.GET("/api/v1/languages", getLanguages(db))

	t.Run("tags", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/tags", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("persons", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/persons", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("languages", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/languages", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGetConfigAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				EnableDelete: true,
			},
		}
		c.Set("config", cfg)
		c.Next()
	})
	r.GET("/api/v1/config", func(c *gin.Context) {
		cfg := getConfig(c)
		c.JSON(http.StatusOK, gin.H{
			"enable_delete": cfg.Server.EnableDelete,
		})
	})

	req, _ := http.NewRequest("GET", "/api/v1/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		EnableDelete bool `json:"enable_delete"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.EnableDelete)
}

func TestGetGenreTreeWithFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.GET("/api/v1/genres/tree", getGenreTree(db))

	t.Run("genre filter", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/genres/tree?genre=prose", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("author filter", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/genres/tree?author=test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("book filter", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/genres/tree?book=test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestSetUserBookStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Set up JWT secret
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	// Create a test user
	var userID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) 
		VALUES ($1, $2, 'viewer') 
		RETURNING id
	`, "testuser_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&userID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", userID)

	// Generate a token
	token := generateToken(userID, "testuser", "viewer")
	require.NotEmpty(t, token)

	// Get an existing edition_id from the test database
	var editionID int
	err = db.QueryRow("SELECT id FROM editions LIMIT 1").Scan(&editionID)
	require.NoError(t, err, "No editions found in test database")

	// Set up the router with auth middleware
	r := gin.New()

	// Register user book routes
	userBooks := r.Group("/api/v1/user/books")
	userBooks.Use(authMiddleware())
	{
		userBooks.PUT("/:edition_id", setUserBook(db))
	}

	// Send PUT request to set reading status
	body := map[string]string{"status": "Прочитано"}
	bodyJSON, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/api/v1/user/books/"+strconv.Itoa(editionID), bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Debug: print response
	t.Logf("Response status: %d, body: %s", w.Code, w.Body.String())

	require.Equal(t, http.StatusOK, w.Code)

	// Verify the status was saved in the database
	var savedStatus string
	err = db.QueryRow(`
		SELECT status::text FROM user_books WHERE user_id = $1 AND edition_id = $2
	`, userID, editionID).Scan(&savedStatus)
	require.NoError(t, err)
	assert.Equal(t, "Прочитано", savedStatus)

	// Clean up
	db.Exec("DELETE FROM user_books WHERE user_id = $1 AND edition_id = $2", userID, editionID)
}

func TestSetUserBookStatusUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	var editionID int
	err := db.QueryRow("SELECT id FROM editions LIMIT 1").Scan(&editionID)
	require.NoError(t, err)

	r := gin.New()
	userBooks := r.Group("/api/v1/user/books")
	userBooks.Use(authMiddleware())
	{
		userBooks.PUT("/:edition_id", setUserBook(db))
	}

	// Send PUT without auth header
	body := map[string]string{"status": "Прочитано"}
	bodyJSON, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/api/v1/user/books/"+strconv.Itoa(editionID), bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateReadListItem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'viewer')
		RETURNING id
	`, "rl_test_create_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&userID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM read_list WHERE user_id = $1", userID)
	defer db.Exec("DELETE FROM users WHERE id = $1", userID)

	token := generateToken(userID, "testuser", "viewer")

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.POST("", createReadListItem(db))
	}

	body := map[string]interface{}{
		"listname": "testlist",
		"bookname": "Test Book",
		"author":   "Test Author",
		"priority": 5,
		"comment":  "test comment",
		"status":   "Читаю",
	}
	bodyJSON, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/v1/user/readlist", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	t.Logf("Response: %d, body: %s", w.Code, w.Body.String())

	require.Equal(t, http.StatusCreated, w.Code)

	var item ReadListItem
	err = json.Unmarshal(w.Body.Bytes(), &item)
	require.NoError(t, err)
	assert.Equal(t, "testlist", item.Listname)
	assert.Equal(t, "Test Book", item.Bookname)
	assert.Equal(t, "Test Author", item.Author)
	assert.Equal(t, 5, item.Priority)
	assert.Equal(t, "test comment", item.Comment)
	assert.Equal(t, "Читаю", item.Status)
	assert.Equal(t, userID, item.UserID)
	assert.NotEmpty(t, item.ID)
}

func TestGetReadListItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'viewer')
		RETURNING id
	`, "rl_test_list_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&userID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM read_list WHERE user_id = $1", userID)
	defer db.Exec("DELETE FROM users WHERE id = $1", userID)

	// Insert a test item
	var itemID string
	err = db.QueryRow(`
		INSERT INTO read_list (id, listname, bookname, author, priority, user_id, comment, status, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, 'Читаю'::user_book_status, NOW())
		RETURNING id::text
	`, "default", "Book1", "Author1", 1, userID, "comment1").Scan(&itemID)
	require.NoError(t, err)

	token := generateToken(userID, "testuser", "viewer")

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.GET("", getReadListItems(db))
	}

	req, _ := http.NewRequest("GET", "/api/v1/user/readlist", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Total int            `json:"total"`
		Items []ReadListItem `json:"items"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, response.Total, 1)
	assert.GreaterOrEqual(t, len(response.Items), 1)
}

func TestReadListItemAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.POST("", createReadListItem(db))
		rl.GET("", getReadListItems(db))
	}

	// Test without auth header
	body := map[string]interface{}{
		"listname": "default",
		"bookname": "B",
		"author":   "A",
		"status":   "Не заполнено",
	}
	bodyJSON, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/v1/user/readlist", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// GET without auth header
	req2, _ := http.NewRequest("GET", "/api/v1/user/readlist", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code) // authMiddleware is optional, returns empty
}

func TestReadListItemUserIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	// Create two users
	var user1ID, user2ID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_iso1_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&user1ID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM read_list WHERE user_id IN ($1,$2)", user1ID, user2ID)
	defer db.Exec("DELETE FROM users WHERE id IN ($1,$2)", user1ID, user2ID)

	err = db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_iso2_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&user2ID)
	require.NoError(t, err)

	// User1 creates items
	var item1ID string
	err = db.QueryRow(`
		INSERT INTO read_list (id, listname, bookname, author, priority, user_id, status, updated_at)
		VALUES (gen_random_uuid(), 'default', 'User1Book', 'User1Author', 1, $1, 'Читаю'::user_book_status, NOW())
		RETURNING id::text
	`, user1ID).Scan(&item1ID)
	require.NoError(t, err)

	token2 := generateToken(user2ID, "user2", "viewer")

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.GET("", getReadListItems(db))
	}

	req, _ := http.NewRequest("GET", "/api/v1/user/readlist", nil)
	req.Header.Set("Authorization", "Bearer "+token2)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Total int            `json:"total"`
		Items []ReadListItem `json:"items"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, 0, response.Total, "User2 should not see User1's items")
}

func TestReadListItemUpdateDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_upd_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&userID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM read_list WHERE user_id = $1", userID)
	defer db.Exec("DELETE FROM users WHERE id = $1", userID)

	// Create item
	var itemID string
	err = db.QueryRow(`
		INSERT INTO read_list (id, listname, bookname, author, priority, user_id, status, updated_at)
		VALUES (gen_random_uuid(), 'oldlist', 'OldBook', 'OldAuthor', 1, $1, 'Не заполнено'::user_book_status, NOW())
		RETURNING id::text
	`, userID).Scan(&itemID)
	require.NoError(t, err)

	token := generateToken(userID, "testuser", "viewer")

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.PUT("/:id", updateReadListItem(db))
		rl.DELETE("/:id", deleteReadListItem(db))
		rl.GET("", getReadListItems(db))
	}

	// Update
	updateBody := map[string]interface{}{
		"listname": "newlist",
		"bookname": "NewBook",
		"author":   "NewAuthor",
		"priority": 10,
		"comment":  "updated",
		"status":   "Прочитано",
	}
	bodyJSON, _ := json.Marshal(updateBody)
	req, _ := http.NewRequest("PUT", "/api/v1/user/readlist/"+itemID, bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	t.Logf("Update response: %d, body: %s", w.Code, w.Body.String())
	require.Equal(t, http.StatusOK, w.Code)

	var updated ReadListItem
	err = json.Unmarshal(w.Body.Bytes(), &updated)
	require.NoError(t, err)
	assert.Equal(t, "newlist", updated.Listname)
	assert.Equal(t, "NewBook", updated.Bookname)
	assert.Equal(t, "Прочитано", updated.Status)
	assert.Equal(t, 10, updated.Priority)

	// Delete (soft delete)
	req2, _ := http.NewRequest("DELETE", "/api/v1/user/readlist/"+itemID, nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	var deletedItem ReadListItem
	err = json.Unmarshal(w2.Body.Bytes(), &deletedItem)
	require.NoError(t, err)
	assert.True(t, deletedItem.Deleted, "item must be soft-deleted")

	// Verify soft deleted row still exists with deleted flag
	var deletedFlag bool
	db.QueryRow("SELECT deleted FROM read_list WHERE id = $1::uuid", itemID).Scan(&deletedFlag)
	assert.True(t, deletedFlag, "read_list row must have deleted = TRUE")

	// Verify it doesn't appear in listing
	var listCount int
	db.QueryRow("SELECT COUNT(*) FROM read_list WHERE id = $1::uuid AND deleted = FALSE", itemID).Scan(&listCount)
	assert.Equal(t, 0, listCount, "deleted item must not appear in active listing")
}

func TestReadListNames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_names_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&userID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM read_list WHERE user_id = $1", userID)
	defer db.Exec("DELETE FROM users WHERE id = $1", userID)

	// Create items with different listnames
	_, err = db.Exec(`
		INSERT INTO read_list (id, listname, bookname, user_id, status, updated_at) VALUES
		(gen_random_uuid(), 'favorites', 'B1', $1, 'Не заполнено'::user_book_status, NOW()),
		(gen_random_uuid(), 'favorites', 'B2', $1, 'Не заполнено'::user_book_status, NOW()),
		(gen_random_uuid(), 'wishlist', 'B3', $1, 'Не заполнено'::user_book_status, NOW())
	`, userID)
	require.NoError(t, err)

	token := generateToken(userID, "testuser", "viewer")

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.GET("/names", getReadListNames(db))
	}

	req, _ := http.NewRequest("GET", "/api/v1/user/readlist/names", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var names []string
	err = json.Unmarshal(w.Body.Bytes(), &names)
	require.NoError(t, err)
	assert.Contains(t, names, "favorites")
	assert.Contains(t, names, "wishlist")
	assert.Len(t, names, 2)
}

// ── Sync procedure tests ─────────────────────────────────────────

func TestReadListSyncTimestamps(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_sync_ts_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&userID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM read_list WHERE user_id = $1", userID)
	defer db.Exec("DELETE FROM users WHERE id = $1", userID)

	token := generateToken(userID, "testuser", "viewer")

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.POST("", createReadListItem(db))
		rl.PUT("/:id", updateReadListItem(db))
		rl.GET("", getReadListItems(db))
	}

	// Create item
	createReq := map[string]interface{}{
		"listname": "sync-test",
		"bookname": "Sync Book",
		"author":   "Sync Author",
		"priority": 1,
		"comment":  "sync test",
		"status":   "Читаю",
	}
	body, _ := json.Marshal(createReq)
	req, _ := http.NewRequest("POST", "/api/v1/user/readlist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created ReadListItem
	err = json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)

	// Verify timestamps on create
	assert.NotEmpty(t, created.ID, "UUID must be set")
	assert.NotEmpty(t, created.CreatedAt, "created_at must be set")
	assert.NotEmpty(t, created.UpdatedAt, "updated_at must be set")
	assert.Empty(t, created.SyncedAt, "synced_at must be empty on create")
	assert.Equal(t, userID, created.UserID)

	// Verify updated_at >= created_at
	assert.GreaterOrEqual(t, created.UpdatedAt, created.CreatedAt,
		"updated_at must be >= created_at on create")

	createdAt := created.CreatedAt
	firstUpdatedAt := created.UpdatedAt

	// Wait a moment to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Update item
	updateReq := map[string]interface{}{
		"listname": "sync-test-updated",
		"bookname": "Sync Book Updated",
		"author":   "Sync Author",
		"priority": 5,
		"comment":  "updated",
		"status":   "Прочитано",
	}
	body2, _ := json.Marshal(updateReq)
	req2, _ := http.NewRequest("PUT", "/api/v1/user/readlist/"+created.ID, bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	var updated ReadListItem
	err = json.Unmarshal(w2.Body.Bytes(), &updated)
	require.NoError(t, err)

	// Verify timestamps on update
	assert.Equal(t, createdAt, updated.CreatedAt, "created_at must not change on update")
	assert.NotEqual(t, firstUpdatedAt, updated.UpdatedAt, "updated_at must change on update")
	assert.Greater(t, updated.UpdatedAt, firstUpdatedAt, "updated_at must be newer after update")
	assert.Empty(t, updated.SyncedAt, "synced_at still empty after update (server doesn't set it)")
	assert.Equal(t, "sync-test-updated", updated.Listname)
	assert.Equal(t, "Прочитано", updated.Status)
}

func TestReadListSyncCreateWithClientUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_sync_uuid_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&userID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM read_list WHERE user_id = $1", userID)
	defer db.Exec("DELETE FROM users WHERE id = $1", userID)

	token := generateToken(userID, "testuser", "viewer")

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.POST("", createReadListItem(db))
		rl.GET("", getReadListItems(db))
	}

	// Create item with client-provided UUID (simulates offline creation)
	clientUUID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	createReq := map[string]interface{}{
		"id":       clientUUID,
		"listname": "offline-created",
		"bookname": "Offline Book",
		"author":   "Offline Author",
		"priority": 10,
		"comment":  "created offline",
		"status":   "Не заполнено",
	}
	body, _ := json.Marshal(createReq)
	req, _ := http.NewRequest("POST", "/api/v1/user/readlist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created ReadListItem
	err = json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)

	// Verify client UUID was accepted
	assert.Equal(t, clientUUID, created.ID, "client-provided UUID must be preserved")
	assert.NotEmpty(t, created.CreatedAt)
	assert.NotEmpty(t, created.UpdatedAt)

	// Verify we can fetch it by UUID
	var listResp struct {
		Total int            `json:"total"`
		Items []ReadListItem `json:"items"`
	}
	getReq, _ := http.NewRequest("GET", "/api/v1/user/readlist?limit=9999", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, getReq)
	require.Equal(t, http.StatusOK, w2.Code)

	err = json.Unmarshal(w2.Body.Bytes(), &listResp)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, listResp.Total, 1)

	found := false
	for _, item := range listResp.Items {
		if item.ID == clientUUID {
			found = true
			assert.Equal(t, "offline-created", item.Listname)
			break
		}
	}
	assert.True(t, found, "item with client UUID must be found in listing")
}

func TestReadListSyncPullAll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_sync_pull_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&userID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM read_list WHERE user_id = $1", userID)
	defer db.Exec("DELETE FROM users WHERE id = $1", userID)

	token := generateToken(userID, "testuser", "viewer")

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.POST("", createReadListItem(db))
		rl.PUT("/:id", updateReadListItem(db))
		rl.GET("", getReadListItems(db))
	}

	// Create multiple items with different priorities
	items := []map[string]interface{}{
		{"listname": "list-a", "bookname": "Book A", "author": "Author", "priority": 1, "comment": "", "status": "Читаю"},
		{"listname": "list-b", "bookname": "Book B", "author": "Author", "priority": 3, "comment": "", "status": "Не заполнено"},
		{"listname": "list-c", "bookname": "Book C", "author": "Author", "priority": 2, "comment": "", "status": "Прочитано"},
	}

	createdIDs := make([]string, 0, len(items))
	for _, item := range items {
		body, _ := json.Marshal(item)
		req, _ := http.NewRequest("POST", "/api/v1/user/readlist", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
		var created ReadListItem
		json.Unmarshal(w.Body.Bytes(), &created)
		createdIDs = append(createdIDs, created.ID)
	}

	// Pull all (simulate sync pull phase)
	var listResp struct {
		Total int            `json:"total"`
		Items []ReadListItem `json:"items"`
	}
	getReq, _ := http.NewRequest("GET", "/api/v1/user/readlist?limit=9999&sort_by=priority&sort_order=desc", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, getReq)
	require.Equal(t, http.StatusOK, w.Code)

	err = json.Unmarshal(w.Body.Bytes(), &listResp)
	require.NoError(t, err)
	assert.Equal(t, 3, listResp.Total, "all 3 items returned")
	assert.Len(t, listResp.Items, 3)

	// Verify sort order (priority desc: 3, 2, 1)
	assert.Equal(t, 3, listResp.Items[0].Priority, "first item must have highest priority")
	assert.Equal(t, 2, listResp.Items[1].Priority)
	assert.Equal(t, 1, listResp.Items[2].Priority)

	// Verify all IDs are present in the response
	pulledIDs := make(map[string]bool)
	for _, item := range listResp.Items {
		pulledIDs[item.ID] = true
	}
	for _, id := range createdIDs {
		assert.True(t, pulledIDs[id], "created item must be in pull response: "+id)
	}
}

func TestReadListSyncUpdateTimestampOnReSync(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_sync_rs_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&userID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM read_list WHERE user_id = $1", userID)
	defer db.Exec("DELETE FROM users WHERE id = $1", userID)

	token := generateToken(userID, "testuser", "viewer")

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.POST("", createReadListItem(db))
		rl.PUT("/:id", updateReadListItem(db))
		rl.GET("", getReadListItems(db))
	}

	// Create item
	createReq := map[string]interface{}{
		"listname": "resync-test",
		"bookname": "Resync Book",
		"author":   "Author",
		"priority": 1,
		"comment":  "",
		"status":   "Читаю",
	}
	body, _ := json.Marshal(createReq)
	req, _ := http.NewRequest("POST", "/api/v1/user/readlist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created ReadListItem
	json.Unmarshal(w.Body.Bytes(), &created)

	// Simulate sync: store synced_at = updated_at on client, then modify locally
	// The sync would set synced_at = updated_at after push
	// When the client modifies the item again, updated_at > synced_at (dirty)
	syncedAt := created.UpdatedAt

	// Simulate client-side update (priority change) that makes item dirty again
	time.Sleep(5 * time.Millisecond)
	updateReq := map[string]interface{}{
		"listname": "resync-test",
		"bookname": "Resync Book Updated",
		"author":   "Author Updated",
		"priority": 10,
		"comment":  "re-synced",
		"status":   "Прочитано",
	}
	body2, _ := json.Marshal(updateReq)
	req2, _ := http.NewRequest("PUT", "/api/v1/user/readlist/"+created.ID, bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	var updated ReadListItem
	json.Unmarshal(w2.Body.Bytes(), &updated)

	// Verify the new updated_at is > previous synced_at (dirty detection)
	assert.Greater(t, updated.UpdatedAt, syncedAt,
		"updated_at after re-sync must be > previous synced_at")
	assert.Equal(t, "Resync Book Updated", updated.Bookname)
	assert.Equal(t, "Author Updated", updated.Author)
	assert.Equal(t, 10, updated.Priority)
	assert.Equal(t, "Прочитано", updated.Status)
}

func TestReadListSyncOtherUserNotAffected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	// Create two users
	var user1ID, user2ID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_sync_other1_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&user1ID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM read_list WHERE user_id = $1", user1ID)
	defer db.Exec("DELETE FROM users WHERE id = $1", user1ID)

	err = db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_sync_other2_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&user2ID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM read_list WHERE user_id = $1", user2ID)
	defer db.Exec("DELETE FROM users WHERE id = $1", user2ID)

	token1 := generateToken(user1ID, "user1", "viewer")
	token2 := generateToken(user2ID, "user2", "viewer")

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.POST("", createReadListItem(db))
		rl.GET("", getReadListItems(db))
	}

	// User1 creates an item
	createReq := map[string]interface{}{
		"listname": "user1-list",
		"bookname": "User1 Book",
		"author":   "User1 Author",
		"priority": 1,
		"comment":  "",
		"status":   "Читаю",
	}
	body, _ := json.Marshal(createReq)
	req, _ := http.NewRequest("POST", "/api/v1/user/readlist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token1)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// User2 creates an item
	createReq2 := map[string]interface{}{
		"listname": "user2-list",
		"bookname": "User2 Book",
		"author":   "User2 Author",
		"priority": 5,
		"comment":  "",
		"status":   "Не заполнено",
	}
	body2, _ := json.Marshal(createReq2)
	req2, _ := http.NewRequest("POST", "/api/v1/user/readlist", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token2)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusCreated, w2.Code)

	// User1 pulls — should only see User1's item
	var user1Resp struct {
		Total int            `json:"total"`
		Items []ReadListItem `json:"items"`
	}
	getReq1, _ := http.NewRequest("GET", "/api/v1/user/readlist?limit=9999", nil)
	getReq1.Header.Set("Authorization", "Bearer "+token1)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, getReq1)
	require.Equal(t, http.StatusOK, w3.Code)
	json.Unmarshal(w3.Body.Bytes(), &user1Resp)
	assert.Equal(t, 1, user1Resp.Total, "User1 must see only their item")
	if len(user1Resp.Items) > 0 {
		assert.Equal(t, "User1 Book", user1Resp.Items[0].Bookname)
		assert.Equal(t, user1ID, user1Resp.Items[0].UserID)
	}

	// User2 pulls — should only see User2's item
	var user2Resp struct {
		Total int            `json:"total"`
		Items []ReadListItem `json:"items"`
	}
	getReq2, _ := http.NewRequest("GET", "/api/v1/user/readlist?limit=9999", nil)
	getReq2.Header.Set("Authorization", "Bearer "+token2)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, getReq2)
	require.Equal(t, http.StatusOK, w4.Code)
	json.Unmarshal(w4.Body.Bytes(), &user2Resp)
	assert.Equal(t, 1, user2Resp.Total, "User2 must see only their item")
	if len(user2Resp.Items) > 0 {
		assert.Equal(t, "User2 Book", user2Resp.Items[0].Bookname)
		assert.Equal(t, user2ID, user2Resp.Items[0].UserID)
	}
}

func TestUpdateBookExtendedRemoveAllAuthors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/books", createBook(db))
	r.PUT("/books/:id/extended", updateBookExtended(db))
	r.GET("/books/:id/extended", getBookExtended(db))

	// Create book with author
	newBook := CreateBookRequest{
		Title:    "Remove Authors Test",
		Author:   "Author To Remove",
		Language: "eng",
	}
	bookJSON, _ := json.Marshal(newBook)
	req, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req.Body = io.NopCloser(bytes.NewReader(bookJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created BookDetails
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)
	editionID := created.EditionID

	// Verify author exists in work_contributors
	var workID int
	err = db.QueryRow("SELECT work_id FROM editions WHERE id = $1", editionID).Scan(&workID)
	require.NoError(t, err)

	var initialContributors int
	err = db.QueryRow("SELECT COUNT(*) FROM work_contributors WHERE work_id = $1", workID).Scan(&initialContributors)
	require.NoError(t, err)
	require.Greater(t, initialContributors, 0, "Book should have at least one author")

	// Send empty authors array to remove all authors
	updateReq := map[string]interface{}{
		"work":    map[string]interface{}{},
		"edition": map[string]interface{}{},
		"authors": []map[string]interface{}{},
		"genres":  []int{},
		"tags":    []int{},
	}
	updateJSON, _ := json.Marshal(updateReq)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify all authors are removed from work_contributors
	var remainingContributors int
	err = db.QueryRow("SELECT COUNT(*) FROM work_contributors WHERE work_id = $1", workID).Scan(&remainingContributors)
	assert.NoError(t, err)
	assert.Equal(t, 0, remainingContributors, "All authors should be removed")

	// Verify the GET endpoint also returns empty authors list
	req, _ = http.NewRequest("GET", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var extended map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &extended)
	authors, ok := extended["authors"].([]interface{})
	if !ok {
		assert.Nil(t, extended["authors"], "authors should be nil or empty")
	} else {
		assert.Equal(t, 0, len(authors), "GET should return empty authors list")
	}
}

func TestUpdateBookExtendedRemoveAllGenres(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/books", createBook(db))
	r.PUT("/books/:id/extended", updateBookExtended(db))
	r.GET("/books/:id/extended", getBookExtended(db))

	// Create book
	newBook := CreateBookRequest{
		Title:    "Remove Genres Test",
		Author:   "Genre Remove Author",
		Language: "eng",
	}
	bookJSON, _ := json.Marshal(newBook)
	req, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req.Body = io.NopCloser(bytes.NewReader(bookJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created BookDetails
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)

	var workID int
	err = db.QueryRow("SELECT work_id FROM editions WHERE id = $1", created.EditionID).Scan(&workID)
	require.NoError(t, err)

	// Manually add a genre and link it
	var genreID int
	err = db.QueryRow("INSERT INTO genres (name) VALUES ('DetectiveGenreTest') ON CONFLICT (name) DO NOTHING RETURNING id").Scan(&genreID)
	if err == sql.ErrNoRows {
		err = db.QueryRow("SELECT id FROM genres WHERE name = 'DetectiveGenreTest'").Scan(&genreID)
	}
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO work_genres (work_id, genre_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", workID, genreID)
	require.NoError(t, err)

	// Verify genre exists
	var initialGenres int
	err = db.QueryRow("SELECT COUNT(*) FROM work_genres WHERE work_id = $1", workID).Scan(&initialGenres)
	require.NoError(t, err)
	require.Greater(t, initialGenres, 0, "Book should have at least one genre")

	// Send empty genres array
	updateReq := map[string]interface{}{
		"work":    map[string]interface{}{},
		"edition": map[string]interface{}{},
		"authors": []map[string]interface{}{},
		"genres":  []int{},
		"tags":    []int{},
	}
	updateJSON, _ := json.Marshal(updateReq)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(created.EditionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify all genres are removed
	var remaining int
	err = db.QueryRow("SELECT COUNT(*) FROM work_genres WHERE work_id = $1", workID).Scan(&remaining)
	assert.NoError(t, err)
	assert.Equal(t, 0, remaining, "All genres should be removed")
}

func TestUpdateBookExtendedRemoveAllTags(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/books", createBook(db))
	r.PUT("/books/:id/extended", updateBookExtended(db))
	r.GET("/books/:id/extended", getBookExtended(db))

	// Create book
	newBook := CreateBookRequest{
		Title:    "Remove Tags Test",
		Author:   "Tag Remove Author",
		Language: "eng",
	}
	bookJSON, _ := json.Marshal(newBook)
	req, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req.Body = io.NopCloser(bytes.NewReader(bookJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created BookDetails
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)

	// Add a tag to the book
	var tagID int
	err = db.QueryRow("INSERT INTO tags (name) VALUES ('testtag_remove') ON CONFLICT (name) DO NOTHING RETURNING id").Scan(&tagID)
	if err == sql.ErrNoRows {
		err = db.QueryRow("SELECT id FROM tags WHERE name = 'testtag_remove'").Scan(&tagID)
	}
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO edition_tags (edition_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", created.EditionID, tagID)
	require.NoError(t, err)

	// Verify tag exists
	var initialTags int
	err = db.QueryRow("SELECT COUNT(*) FROM edition_tags WHERE edition_id = $1", created.EditionID).Scan(&initialTags)
	require.NoError(t, err)
	require.Greater(t, initialTags, 0, "Book should have at least one tag")

	// Send empty tags array
	updateReq := map[string]interface{}{
		"work":    map[string]interface{}{},
		"edition": map[string]interface{}{},
		"authors": []map[string]interface{}{},
		"genres":  []int{},
		"tags":    []int{},
	}
	updateJSON, _ := json.Marshal(updateReq)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(created.EditionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify all tags are removed
	var remaining int
	err = db.QueryRow("SELECT COUNT(*) FROM edition_tags WHERE edition_id = $1", created.EditionID).Scan(&remaining)
	assert.NoError(t, err)
	assert.Equal(t, 0, remaining, "All tags should be removed")
}

func TestUpdateBookExtendedNilAuthorsKeepsExisting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/books", createBook(db))
	r.PUT("/books/:id/extended", updateBookExtended(db))

	// Create book with author
	newBook := CreateBookRequest{
		Title:    "Nil Authors Test",
		Author:   "Nil Author Person",
		Language: "eng",
	}
	bookJSON, _ := json.Marshal(newBook)
	req, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req.Body = io.NopCloser(bytes.NewReader(bookJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created BookDetails
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)

	var workID int
	err = db.QueryRow("SELECT work_id FROM editions WHERE id = $1", created.EditionID).Scan(&workID)
	require.NoError(t, err)

	// Verify author exists
	var initialContributors int
	err = db.QueryRow("SELECT COUNT(*) FROM work_contributors WHERE work_id = $1", workID).Scan(&initialContributors)
	require.NoError(t, err)
	require.Greater(t, initialContributors, 0)

	// Send update WITHOUT authors field (should be nil in Go, keep existing authors)
	updateReq := map[string]interface{}{
		"work": map[string]interface{}{
			"original_title": "Nil Authors Updated Title",
		},
		"edition": map[string]interface{}{},
	}
	updateJSON, _ := json.Marshal(updateReq)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(created.EditionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify authors are still there (unchanged)
	var afterContributors int
	err = db.QueryRow("SELECT COUNT(*) FROM work_contributors WHERE work_id = $1", workID).Scan(&afterContributors)
	assert.NoError(t, err)
	assert.Equal(t, initialContributors, afterContributors, "Authors should remain unchanged when authors field is not sent")

	// Verify title was still updated
	var title string
	err = db.QueryRow("SELECT original_title FROM works WHERE id = $1", workID).Scan(&title)
	assert.NoError(t, err)
	assert.Equal(t, "Nil Authors Updated Title", title)
}

func TestCSPHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(securityHeadersMiddleware())
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req, _ := http.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "script-src 'self' 'unsafe-inline'",
		"CSP must allow inline scripts for onclick handlers")
	assert.Contains(t, csp, "style-src 'self' 'unsafe-inline'",
		"CSP must allow inline styles for style attributes")
}

func TestUpdateBookExtendedAddAuthor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	db := setupTestDB()
	defer db.Close()

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.POST("/api/v1/books", createBook(db))
	r.PUT("/books/:id/extended", updateBookExtended(db))
	r.GET("/books/:id/extended", getBookExtended(db))

	// Create a book
	newBook := CreateBookRequest{
		Title:    "Add Author Test",
		Author:   "Initial Author",
		Language: "eng",
	}
	bookJSON, _ := json.Marshal(newBook)
	req, _ := http.NewRequest("POST", "/api/v1/books", nil)
	req.Body = io.NopCloser(bytes.NewReader(bookJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created BookDetails
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)
	editionID := created.EditionID

	// Find the initial author's ID
	var initialAuthorID int
	err = db.QueryRow("SELECT id FROM persons WHERE last_name = 'Initial Author'").Scan(&initialAuthorID)
	require.NoError(t, err)

	// Create a new person to add as second author
	_, err = db.Exec(`INSERT INTO persons (first_name, last_name) VALUES ('Added', 'Author') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
	var addedAuthorID int
	err = db.QueryRow("SELECT id FROM persons WHERE first_name = 'Added' AND last_name = 'Author'").Scan(&addedAuthorID)
	require.NoError(t, err)

	// Add a second author via extended endpoint
	addAuthorReq := map[string]interface{}{
		"work":    map[string]interface{}{},
		"edition": map[string]interface{}{},
		"authors": []map[string]interface{}{
			{"id": initialAuthorID, "role": "author"},
			{"id": addedAuthorID, "role": "author"},
		},
		"genres": []int{},
		"tags":   []int{},
	}
	addJSON, _ := json.Marshal(addAuthorReq)
	req, _ = http.NewRequest("PUT", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	req.Body = io.NopCloser(bytes.NewReader(addJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify both authors exist in response
	req, _ = http.NewRequest("GET", "/books/"+strconv.Itoa(editionID)+"/extended", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var extended map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &extended)
	assert.NoError(t, err)

	authors, ok := extended["authors"].([]interface{})
	require.True(t, ok, "authors must be a non-nil array")
	assert.Len(t, authors, 2, "should have exactly 2 authors")

	// Verify author names are present
	authorNames := make([]string, 0)
	for _, a := range authors {
		author := a.(map[string]interface{})
		parts := make([]string, 0)
		if fn, ok := author["first_name"]; ok {
			if s, isStr := fn.(string); isStr && s != "" {
				parts = append(parts, s)
			}
		}
		if ln, ok := author["last_name"]; ok {
			if s, isStr := ln.(string); isStr && s != "" {
				parts = append(parts, s)
			}
		}
		authorNames = append(authorNames, strings.Join(parts, " "))
	}
	assert.Contains(t, authorNames, "Initial Author", "initial author must be preserved")
	assert.Contains(t, authorNames, "Added Author", "newly added author must appear")
}

// ── Readlist Sync Integration Tests ──────────────────────────

func TestReadListSyncDuplicateUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_dup_uuid_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&userID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM read_list WHERE user_id = $1", userID)
	defer db.Exec("DELETE FROM users WHERE id = $1", userID)

	token := generateToken(userID, "testuser", "viewer")

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.POST("", createReadListItem(db))
		rl.PUT("/:id", updateReadListItem(db))
	}

	clientUUID := "b2c3d4e5-f6a7-8901-bcde-f12345678901"

	// First create succeeds
	createReq := map[string]interface{}{
		"id":       clientUUID,
		"listname": "test-dup",
		"bookname": "Dup Book",
		"author":   "Dup Author",
		"priority": 1,
		"status":   "Не заполнено",
	}
	body, _ := json.Marshal(createReq)
	req, _ := http.NewRequest("POST", "/api/v1/user/readlist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// Second create with same UUID must fail
	req2, _ := http.NewRequest("POST", "/api/v1/user/readlist", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusInternalServerError, w2.Code, "duplicate UUID should fail")
}

func TestReadListSyncPutNonExistent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_put_404_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&userID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", userID)

	token := generateToken(userID, "testuser", "viewer")

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.PUT("/:id", updateReadListItem(db))
	}

	// PUT to non-existent UUID must return 404
	updateReq := map[string]interface{}{
		"listname": "nonexistent",
		"bookname": "Nope",
		"author":   "No One",
		"priority": 0,
		"status":   "Не заполнено",
	}
	body, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest("PUT", "/api/v1/user/readlist/deadbeef-0000-0000-0000-000000000000", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "PUT to non-existent UUID must return 404")
}

func TestReadListSyncFullFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_fullflow_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&userID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM read_list WHERE user_id = $1", userID)
	defer db.Exec("DELETE FROM users WHERE id = $1", userID)

	token := generateToken(userID, "testuser", "viewer")

	r := gin.New()
	rl := r.Group("/api/v1/user/readlist")
	rl.Use(authMiddleware())
	{
		rl.POST("", createReadListItem(db))
		rl.GET("", getReadListItems(db))
		rl.PUT("/:id", updateReadListItem(db))
		rl.DELETE("/:id", deleteReadListItem(db))
	}

	clientUUID := "c3d4e5f6-a7b8-9012-cdef-123456789012"

	// Step 1: Create (simulates offline creation pushed to server)
	createReq := map[string]interface{}{
		"id":       clientUUID,
		"listname": "sync-test",
		"bookname": "Sync Book",
		"author":   "Sync Author",
		"priority": 5,
		"comment":  "initial",
		"status":   "Не заполнено",
	}
	body, _ := json.Marshal(createReq)
	req, _ := http.NewRequest("POST", "/api/v1/user/readlist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created ReadListItem
	err = json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)
	assert.Equal(t, clientUUID, created.ID)
	assert.NotEmpty(t, created.UpdatedAt)
	assert.NotEmpty(t, created.CreatedAt)
	initialUpdatedAt := created.UpdatedAt

	// Step 2: Pull (GET all)
	var listResp struct {
		Total int            `json:"total"`
		Items []ReadListItem `json:"items"`
	}
	getReq, _ := http.NewRequest("GET", "/api/v1/user/readlist?limit=9999", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, getReq)
	require.Equal(t, http.StatusOK, w2.Code)

	err = json.Unmarshal(w2.Body.Bytes(), &listResp)
	require.NoError(t, err)
	require.GreaterOrEqual(t, listResp.Total, 1)

	var pulledItem *ReadListItem
	for i := range listResp.Items {
		if listResp.Items[i].ID == clientUUID {
			pulledItem = &listResp.Items[i]
			break
		}
	}
	require.NotNil(t, pulledItem, "pulled item must be found")
	assert.Equal(t, "sync-test", pulledItem.Listname)

	// Step 3: Update (simulates dirty item pushed)
	updateReq := map[string]interface{}{
		"id":       clientUUID,
		"listname": "sync-test",
		"bookname": "Sync Book Updated",
		"author":   "Sync Author",
		"priority": 5,
		"comment":  "updated via sync",
		"status":   "Читаю",
	}
	body2, _ := json.Marshal(updateReq)
	req3, _ := http.NewRequest("PUT", "/api/v1/user/readlist/"+clientUUID, bytes.NewReader(body2))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	require.Equal(t, http.StatusOK, w3.Code)

	var updated ReadListItem
	err = json.Unmarshal(w3.Body.Bytes(), &updated)
	require.NoError(t, err)
	assert.Equal(t, "Sync Book Updated", updated.Bookname)
	assert.Equal(t, "Читаю", updated.Status)
	assert.Equal(t, "updated via sync", updated.Comment)
	// updated_at must have changed
	assert.NotEqual(t, initialUpdatedAt, updated.UpdatedAt, "updated_at must change after update")

	// Step 4: Re-pull and verify changes
	getReq2, _ := http.NewRequest("GET", "/api/v1/user/readlist?limit=9999", nil)
	getReq2.Header.Set("Authorization", "Bearer "+token)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, getReq2)
	require.Equal(t, http.StatusOK, w4.Code)

	err = json.Unmarshal(w4.Body.Bytes(), &listResp)
	require.NoError(t, err)

	var repulledItem *ReadListItem
	for i := range listResp.Items {
		if listResp.Items[i].ID == clientUUID {
			repulledItem = &listResp.Items[i]
			break
		}
	}
	require.NotNil(t, repulledItem, "repulled item must be found")
	assert.Equal(t, "Sync Book Updated", repulledItem.Bookname)
	assert.Equal(t, "Читаю", repulledItem.Status)
	assert.Equal(t, "updated via sync", repulledItem.Comment)

	// Step 5: Delete (soft delete — simulates sync deletion)
	req4, _ := http.NewRequest("DELETE", "/api/v1/user/readlist/"+clientUUID, nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, req4)
	require.Equal(t, http.StatusOK, w5.Code, "DELETE should return 200")

	var deletedItem ReadListItem
	err = json.Unmarshal(w5.Body.Bytes(), &deletedItem)
	require.NoError(t, err)
	assert.Equal(t, clientUUID, deletedItem.ID)
	assert.True(t, deletedItem.Deleted, "deleted item must have deleted=true")
	assert.NotEmpty(t, deletedItem.UpdatedAt, "deleted item must have updated_at")

	// Verify deletion via pull (deleted items excluded from listing)
	getReq3, _ := http.NewRequest("GET", "/api/v1/user/readlist?limit=9999", nil)
	getReq3.Header.Set("Authorization", "Bearer "+token)
	w6 := httptest.NewRecorder()
	r.ServeHTTP(w6, getReq3)
	require.Equal(t, http.StatusOK, w6.Code)

	err = json.Unmarshal(w6.Body.Bytes(), &listResp)
	require.NoError(t, err)

	for _, item := range listResp.Items {
		assert.NotEqual(t, clientUUID, item.ID, "deleted item must not appear in listing")
	}
}