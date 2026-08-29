package main

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
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
		Total  int           `json:"total"`
		Limit  int           `json:"limit"`
		Offset string        `json:"offset"`
		Books  []BookDetails `json:"books"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(response.Books), 1)
}

func TestGetBooksByAuthorID(t *testing.T) {
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

	// Find an author with editions (capped so the default pagination limit
	// does not truncate the result list)
	var authorID int
	var expectedCount int
	err := db.QueryRow(`
		SELECT wc.person_id, COUNT(DISTINCT e.id)
		FROM editions e
		JOIN works w ON w.id = e.work_id
		JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
		GROUP BY wc.person_id
		HAVING COUNT(DISTINCT e.id) BETWEEN 1 AND 100
		ORDER BY COUNT(DISTINCT e.id) DESC
		LIMIT 1
	`).Scan(&authorID, &expectedCount)
	require.NoError(t, err)
	require.Greater(t, expectedCount, 0)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/books?author_id=%d&limit=100", authorID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Total int           `json:"total"`
		Books []BookDetails `json:"books"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, expectedCount, response.Total)
	assert.Equal(t, expectedCount, len(response.Books))

	// Every returned edition must actually have that author
	for _, b := range response.Books {
		var n int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM editions e
			JOIN works w ON w.id = e.work_id
			JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
			WHERE e.id = $1 AND wc.person_id = $2
		`, b.EditionID, authorID).Scan(&n)
		require.NoError(t, err)
		assert.Equal(t, 1, n, "книга %d должна принадлежать автору %d", b.EditionID, authorID)
	}
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
		Limit  int           `json:"limit"`
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
		Title:         "Test Book Part 1",
		Author:        "Multi Author",
		ISBN:          "1234567890123",
		PublishedYear: 2023,
		Genre:         "Test",
		Description:   "First test book created via API",
		Language:      "eng",
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
		Title:         "Test Book Part 2",
		Author:        "Multi Author",
		ISBN:          "1234567890124",
		PublishedYear: 2024,
		Genre:         "Test",
		Description:   "Second test book created via API",
		Language:      "eng",
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
		Title:         "Book to Update",
		Author:        "Updater",
		ISBN:          "9876543210987",
		PublishedYear: 2022,
		Genre:         "Update Test",
		Description:   "A book that will be updated",
		Language:      "eng",
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
		Title:         "Book to Delete",
		Author:        "Deleter",
		ISBN:          "1111111111111",
		PublishedYear: 2021,
		Genre:         "Delete Test",
		Description:   "A book that will be deleted",
		Language:      "eng",
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
		Title:    "Original Title",
		Author:   "Original Author",
		Language: "eng",
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
		"authors": []map[string]interface{}{},
		"genres":  []int{},
		"tags":    []int{},
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
		"work":    map[string]interface{}{},
		"edition": map[string]interface{}{"year": 2025},
		"authors": []map[string]interface{}{},
		"genres":  []int{},
		"tags":    []int{},
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
		"authors": []map[string]interface{}{},
		"genres":  []int{},
		"tags":    []int{},
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
		"work":    map[string]interface{}{},
		"edition": map[string]interface{}{},
		"authors": []map[string]interface{}{{"id": newAuthorID, "role": "author"}},
		"genres":  []int{},
		"tags":    []int{},
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
		"work":    map[string]interface{}{},
		"edition": map[string]interface{}{"isbn": isbnValue},
		"authors": []map[string]interface{}{},
		"genres":  []int{},
		"tags":    []int{},
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
		Limit  int           `json:"limit"`
		Offset string        `json:"offset"`
		Books  []BookDetails `json:"books"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 20, response.Limit)
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
		Limit  int           `json:"limit"`
		Offset string        `json:"offset"`
		Books  []BookDetails `json:"books"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 10, response.Limit)
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
		Total         int               `json:"total"`
		Page          int               `json:"page"`
		Limit         int               `json:"limit"`
		TotalWorks    int               `json:"total_works"`
		TotalEditions int               `json:"total_editions"`
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

func TestReadListSyncConflictServerNewer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	var userID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "rl_conflict_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&userID)
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

	clientUUID := "f1e2d3c4-b5a6-7890-abcd-ef1234567890"

	// Step 1: Create item (client A)
	createReq := map[string]interface{}{
		"id":       clientUUID,
		"listname": "conflict-test",
		"bookname": "Original Book",
		"author":   "Original Author",
		"priority": 1,
		"comment":  "",
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
	json.Unmarshal(w.Body.Bytes(), &created)
	createdUpdatedAt := created.UpdatedAt

	// Step 2: Another client modifies the item (simulating server-side update)
	time.Sleep(5 * time.Millisecond)
	updateReq := map[string]interface{}{
		"listname": "conflict-test",
		"bookname": "Server Updated Book",
		"author":   "Server Author",
		"priority": 10,
		"comment":  "server edit",
		"status":   "Читаю",
	}
	body2, _ := json.Marshal(updateReq)
	req2, _ := http.NewRequest("PUT", "/api/v1/user/readlist/"+clientUUID, bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	var serverUpdated ReadListItem
	json.Unmarshal(w2.Body.Bytes(), &serverUpdated)
	serverUpdatedAt := serverUpdated.UpdatedAt
	assert.Greater(t, serverUpdatedAt, createdUpdatedAt, "server updated_at must be > created updated_at")

	// Step 3: Client A tries to push stale data (with outdated updated_at)
	time.Sleep(2 * time.Millisecond)
	staleReq := map[string]interface{}{
		"id":         clientUUID,
		"listname":   "conflict-test",
		"bookname":   "Client Stale Edit",
		"author":     "Stale Author",
		"priority":   1,
		"comment":    "stale client edit",
		"status":     "Не заполнено",
		"updated_at": createdUpdatedAt,
	}
	staleBody, _ := json.Marshal(staleReq)
	req3, _ := http.NewRequest("PUT", "/api/v1/user/readlist/"+clientUUID, bytes.NewReader(staleBody))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)

	// Must get 409 Conflict with server's current state
	require.Equal(t, http.StatusConflict, w3.Code, "stale update must return 409")

	var conflictResp struct {
		Error      string        `json:"error"`
		ServerItem *ReadListItem `json:"server_item"`
	}
	err = json.Unmarshal(w3.Body.Bytes(), &conflictResp)
	require.NoError(t, err)
	require.NotNil(t, conflictResp.ServerItem, "409 must include server_item")
	assert.Equal(t, "Server Updated Book", conflictResp.ServerItem.Bookname, "server_item must have server's bookname")
	assert.Equal(t, "Server Author", conflictResp.ServerItem.Author, "server_item must have server's author")
	assert.Equal(t, 10, conflictResp.ServerItem.Priority, "server_item must have server's priority")
	assert.Equal(t, "Читаю", conflictResp.ServerItem.Status, "server_item must have server's status")
	assert.Equal(t, serverUpdatedAt, conflictResp.ServerItem.UpdatedAt, "server_item updated_at must match server")

	// Step 4: Verify GET still returns server's version (not corrupted by stale push)
	getReq, _ := http.NewRequest("GET", "/api/v1/user/readlist?limit=9999", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, getReq)
	require.Equal(t, http.StatusOK, w4.Code)

	var listResp struct {
		Total int            `json:"total"`
		Items []ReadListItem `json:"items"`
	}
	json.Unmarshal(w4.Body.Bytes(), &listResp)
	assert.GreaterOrEqual(t, listResp.Total, 1)

	var found *ReadListItem
	for i := range listResp.Items {
		if listResp.Items[i].ID == clientUUID {
			found = &listResp.Items[i]
			break
		}
	}
	require.NotNil(t, found, "item must be found in listing")
	assert.Equal(t, "Server Updated Book", found.Bookname, "GET must return server's version")
	assert.Equal(t, "Server Author", found.Author)
	assert.Equal(t, 10, found.Priority)
	assert.Equal(t, "Читаю", found.Status)
	assert.Equal(t, serverUpdatedAt, found.UpdatedAt, "GET must return latest updated_at")
}

// ─── Suggestion API Tests ──────────────────────────────────────

func TestSuggestionsCreateHideAndList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-sug-secret")

	db := setupTestDB()
	defer db.Close()

	// Create admin user
	var adminID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'admin') RETURNING id
	`, "sug_admin_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&adminID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM suggestions WHERE user_id = $1", adminID)
	defer db.Exec("DELETE FROM read_list WHERE user_id = $1", adminID)
	defer db.Exec("DELETE FROM users WHERE id = $1", adminID)

	adminToken := generateToken(adminID, "sug_admin", "admin")

	// Create a viewer user (who will have read_list items)
	var viewerID int
	err = db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id
	`, "sug_viewer_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&viewerID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", viewerID)

	// Add a genre name so suggestions work has a genre reference
	var genreID int
	_ = db.QueryRow("INSERT INTO genres (name) VALUES ('SuggestionsTest') ON CONFLICT DO NOTHING RETURNING id").Scan(&genreID)

	// Create read_list items with looking_for != 'Нет'
	rlID1 := mustGenerateUUID()
	rlID2 := mustGenerateUUID()
	rlID3 := mustGenerateUUID()

	_, err = db.Exec(`
		INSERT INTO read_list (id, listname, bookname, author, priority, user_id, comment, status, looking_for, deleted)
		VALUES ($1::uuid, 'default', 'Test Book One', 'Test Author', 1, $2, '', 'Не заполнено', 'Да, локально', false)
	`, rlID1, viewerID)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO read_list (id, listname, bookname, author, priority, user_id, comment, status, looking_for, deleted)
		VALUES ($1::uuid, 'default', 'Test Book Two', 'Another Author', 2, $2, '', 'Не заполнено', 'Да, по федерации', false)
	`, rlID2, viewerID)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO read_list (id, listname, bookname, author, priority, user_id, comment, status, looking_for, deleted)
		VALUES ($1::uuid, 'default', 'Test Book Three', 'Third Author', 3, $2, '', 'Не заполнено', 'Нет', false)
	`, rlID3, viewerID)
	require.NoError(t, err)

	defer db.Exec("DELETE FROM read_list WHERE id IN ($1::uuid, $2::uuid, $3::uuid)", rlID1, rlID2, rlID3)

	// Set up router with admin auth middleware + suggestions routes
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	adminGroup := r.Group("/api/v1/admin")
	adminGroup.Use(func(c *gin.Context) {
		// Simulate adminAuthMiddleware but without full JWT check for test
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || len(authHeader) < 8 || authHeader[:7] != "Bearer " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		tokenStr := authHeader[7:]
		claims, err := validateToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}
		if uid, ok := claims["user_id"].(float64); ok {
			c.Set("user_id", int(uid))
		}
		if role, ok := claims["role"].(string); ok {
			c.Set("role", role)
		}
		c.Next()
	})
	{
		adminGroup.GET("/suggestions", adminListSuggestions(db))
		adminGroup.POST("/suggestions", adminCreateSuggestions(db))
		adminGroup.GET("/suggestions/readlist/:id", adminGetReadListSuggestions(db))
		adminGroup.DELETE("/suggestions/:id", adminDeleteSuggestion(db))
	}

	// ── Test 1: List suggestions (hidden=no default) — should show items without admin's suggestion ──
	req, _ := http.NewRequest("GET", "/api/v1/admin/suggestions", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var listResp struct {
		Total int              `json:"total"`
		Items []SuggestionItem `json:"items"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &listResp)
	require.NoError(t, err)

	// Should see rlID1 and rlID2 (both have looking_for != 'Нет'), but NOT rlID3 (looking_for = 'Нет')
	found1 := false
	found2 := false
	found3 := false
	for _, item := range listResp.Items {
		if item.ReadListID == rlID1 {
			found1 = true
		}
		if item.ReadListID == rlID2 {
			found2 = true
		}
		if item.ReadListID == rlID3 {
			found3 = true
		}
	}
	assert.True(t, found1, "rlID1 should appear (looking_for = 'Да, локально')")
	assert.True(t, found2, "rlID2 should appear (looking_for = 'Да, по федерации')")
	assert.False(t, found3, "rlID3 should NOT appear (looking_for = 'Нет')")
	assert.False(t, listResp.Items[0].HasSuggestion, "items should not have suggestion initially")

	// ── Test 2: Hide rlID1 (create suggestion with hidden=true, no edition) ──
	hideBody, _ := json.Marshal(CreateSuggestionsRequest{
		ReadListID: rlID1,
		Items: []CreateSuggestionsRequestItem{
			{EditionID: nil, Hidden: true},
		},
	})
	req2, _ := http.NewRequest("POST", "/api/v1/admin/suggestions", bytes.NewReader(hideBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+adminToken)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	// Now list with hidden=no — rlID1 should NOT appear (hidden by admin)
	req3, _ := http.NewRequest("GET", "/api/v1/admin/suggestions?hidden=no", nil)
	req3.Header.Set("Authorization", "Bearer "+adminToken)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	require.Equal(t, http.StatusOK, w3.Code)

	err = json.Unmarshal(w3.Body.Bytes(), &listResp)
	require.NoError(t, err)

	found1 = false
	found2 = false
	for _, item := range listResp.Items {
		if item.ReadListID == rlID1 {
			found1 = true
		}
		if item.ReadListID == rlID2 {
			found2 = true
		}
	}
	assert.False(t, found1, "rlID1 should NOT appear after hide (hidden=no)")
	assert.True(t, found2, "rlID2 should still appear")

	// ── Test 3: List with hidden=yes — rlID1 should appear ──
	req4, _ := http.NewRequest("GET", "/api/v1/admin/suggestions?hidden=yes", nil)
	req4.Header.Set("Authorization", "Bearer "+adminToken)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	require.Equal(t, http.StatusOK, w4.Code)

	err = json.Unmarshal(w4.Body.Bytes(), &listResp)
	require.NoError(t, err)

	found1 = false
	found2 = false
	for _, item := range listResp.Items {
		if item.ReadListID == rlID1 {
			found1 = true
			assert.True(t, item.HasSuggestion, "rlID1 should have suggestion")
			assert.NotNil(t, item.SuggestionID, "rlID1 should have suggestion_id")
			assert.NotNil(t, item.SuggHidden, "rlID1 should have sugg_hidden")
			if item.SuggHidden != nil {
				assert.True(t, *item.SuggHidden, "rlID1 should be hidden")
			}
		}
		if item.ReadListID == rlID2 {
			found2 = true
		}
	}
	assert.True(t, found1, "rlID1 should appear (hidden=yes)")
	assert.False(t, found2, "rlID2 should NOT appear (hidden=yes)")

	// ── Test 4: List with hidden=all — both should appear ──
	req5, _ := http.NewRequest("GET", "/api/v1/admin/suggestions?hidden=all", nil)
	req5.Header.Set("Authorization", "Bearer "+adminToken)
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, req5)
	require.Equal(t, http.StatusOK, w5.Code)

	err = json.Unmarshal(w5.Body.Bytes(), &listResp)
	require.NoError(t, err)

	found1 = false
	found2 = false
	for _, item := range listResp.Items {
		if item.ReadListID == rlID1 {
			found1 = true
		}
		if item.ReadListID == rlID2 {
			found2 = true
		}
	}
	assert.True(t, found1, "rlID1 should appear (hidden=all)")
	assert.True(t, found2, "rlID2 should appear (hidden=all)")

	// ── Test 5: Get existing suggestions for rlID1 ──
	req6, _ := http.NewRequest("GET", "/api/v1/admin/suggestions/readlist/"+rlID1, nil)
	req6.Header.Set("Authorization", "Bearer "+adminToken)
	w6 := httptest.NewRecorder()
	r.ServeHTTP(w6, req6)
	require.Equal(t, http.StatusOK, w6.Code)

	var readResp struct {
		Items []struct {
			ID        int  `json:"id"`
			EditionID *int `json:"edition_id"`
			Hidden    bool `json:"hidden"`
		} `json:"items"`
		Delivered struct {
			EditionID int `json:"edition_id"`
		} `json:"delivered"`
	}
	err = json.Unmarshal(w6.Body.Bytes(), &readResp)
	require.NoError(t, err)
	suggestions := readResp.Items
	require.NotNil(t, suggestions)
	assert.GreaterOrEqual(t, len(suggestions), 1, "should have at least one suggestion for rlID1")
	assert.True(t, suggestions[0].Hidden, "suggestion should be hidden")
	assert.Nil(t, suggestions[0].EditionID, "edition should be null for hide")

	// ── Test 6: Delete the suggestion ──
	sugID := suggestions[0].ID
	req7, _ := http.NewRequest("DELETE", "/api/v1/admin/suggestions/"+strconv.Itoa(sugID), nil)
	req7.Header.Set("Authorization", "Bearer "+adminToken)
	w7 := httptest.NewRecorder()
	r.ServeHTTP(w7, req7)
	require.Equal(t, http.StatusOK, w7.Code)

	// Verify it's gone: hidden=yes should show nothing now
	req8, _ := http.NewRequest("GET", "/api/v1/admin/suggestions?hidden=yes", nil)
	req8.Header.Set("Authorization", "Bearer "+adminToken)
	w8 := httptest.NewRecorder()
	r.ServeHTTP(w8, req8)
	require.Equal(t, http.StatusOK, w8.Code)

	err = json.Unmarshal(w8.Body.Bytes(), &listResp)
	require.NoError(t, err)
	assert.Equal(t, 0, len(listResp.Items), "no hidden items after deleting the only suggestion")
}

func TestNeighboursCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	// Create a test admin user
	uname := "nbr_admin_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var adminID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'admin') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&adminID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", adminID)

	adminToken := generateToken(adminID, uname, "admin")
	require.NotEmpty(t, adminToken)

	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	// Register the admin group exactly like main.go does.
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("role", "admin")
		c.Next()
	})
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	{
		adminNeighbours := admin.Group("/neighbours")
		adminNeighbours.Use(adminOnlyMiddleware())
		{
			adminNeighbours.GET("", adminGetNeighbours(db))
			adminNeighbours.GET("/:id", adminGetNeighbour(db))
			adminNeighbours.POST("", adminCreateNeighbour(db, nc))
			adminNeighbours.PUT("/:id", adminUpdateNeighbour(db, nc))
			adminNeighbours.DELETE("/:id", adminDeleteNeighbour(db))
		}
	}
	auth := func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
	}

	// ── Create ──
	createBody := map[string]string{
		"url":         "https://peer.example.com",
		"server_cert": "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----",
		"client_cert": "-----BEGIN CERTIFICATE-----\nCLI\n-----END CERTIFICATE-----",
		"username":    "syncuser",
		"password":    "s3cr3t-pass",
	}
	bodyJSON, _ := json.Marshal(createBody)
	req, _ := http.NewRequest("POST", "/api/v1/admin/neighbours", bytes.NewReader(bodyJSON))
	auth(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "create failed: %s", w.Body.String())

	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Greater(t, created.ID, 0)
	defer db.Exec("DELETE FROM api_neighbours WHERE id = $1", created.ID)

	// Verify password stored encrypted (different from plaintext, decryptable).
	var enc string
	err = db.QueryRow(`SELECT password_encrypted FROM api_neighbours WHERE id = $1`, created.ID).Scan(&enc)
	require.NoError(t, err)
	assert.NotEqual(t, "s3cr3t-pass", enc, "password must be encrypted at rest")
	plain, err := nc.Decrypt(enc)
	require.NoError(t, err)
	assert.Equal(t, "s3cr3t-pass", plain)

	// ── GET: has_password=true, no password/cert leak beyond certs ──
	req, _ = http.NewRequest("GET", "/api/v1/admin/neighbours", nil)
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var list []Neighbour
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	var found *Neighbour
	for i := range list {
		if list[i].ID == created.ID {
			found = &list[i]
			break
		}
	}
	require.NotNil(t, found, "created neighbour must appear in GET list")
	assert.Equal(t, "https://peer.example.com", found.URL)
	assert.Equal(t, "syncuser", found.Username)
	assert.True(t, found.HasPassword)
	assert.Contains(t, found.ServerCert, "BEGIN CERTIFICATE")
	assert.NotContains(t, w.Body.String(), "s3cr3t-pass", "plaintext password must not appear in response")
	assert.NotContains(t, w.Body.String(), "password_encrypted")

	// ── GET single: prefills the edit form, no password leak ──
	req, _ = http.NewRequest("GET", "/api/v1/admin/neighbours/"+strconv.Itoa(created.ID), nil)
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "get one failed: %s", w.Body.String())
	var one Neighbour
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &one))
	assert.Equal(t, created.ID, one.ID)
	assert.Equal(t, "https://peer.example.com", one.URL)
	assert.Equal(t, "syncuser", one.Username)
	assert.True(t, one.HasPassword)
	assert.Contains(t, one.ServerCert, "BEGIN CERTIFICATE")
	assert.Contains(t, one.ClientCert, "BEGIN CERTIFICATE")
	assert.NotContains(t, w.Body.String(), "s3cr3t-pass", "plaintext password must not leak on GET single")

	// GET single of a missing id → 404
	req, _ = http.NewRequest("GET", "/api/v1/admin/neighbours/9999999", nil)
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)

	// ── PUT: change password + url ──
	updateBody := map[string]interface{}{
		"url":            "https://peer2.example.com",
		"username":       "syncuser2",
		"password":       "new-pass-42",
		"clear_password": false,
	}
	bodyJSON, _ = json.Marshal(updateBody)
	req, _ = http.NewRequest("PUT", "/api/v1/admin/neighbours/"+strconv.Itoa(created.ID), bytes.NewReader(bodyJSON))
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "update failed: %s", w.Body.String())

	err = db.QueryRow(`SELECT password_encrypted FROM api_neighbours WHERE id = $1`, created.ID).Scan(&enc)
	require.NoError(t, err)
	plain, err = nc.Decrypt(enc)
	require.NoError(t, err)
	assert.Equal(t, "new-pass-42", plain)

	// ── PUT: clear_password removes the password ──
	clearBody := map[string]interface{}{
		"url":            "https://peer2.example.com",
		"clear_password": true,
	}
	bodyJSON, _ = json.Marshal(clearBody)
	req, _ = http.NewRequest("PUT", "/api/v1/admin/neighbours/"+strconv.Itoa(created.ID), bytes.NewReader(bodyJSON))
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	err = db.QueryRow(`SELECT password_encrypted FROM api_neighbours WHERE id = $1`, created.ID).Scan(&enc)
	require.NoError(t, err)
	assert.Equal(t, "", enc, "clear_password must wipe the stored password")

	// ── DELETE ──
	req, _ = http.NewRequest("DELETE", "/api/v1/admin/neighbours/"+strconv.Itoa(created.ID), nil)
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var cnt int
	err = db.QueryRow(`SELECT COUNT(*) FROM api_neighbours WHERE id = $1`, created.ID).Scan(&cnt)
	require.NoError(t, err)
	assert.Equal(t, 0, cnt)
}

func TestNeighboursHTTPSAndDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	uname := "nbrsec_admin_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var adminID int
	err := db.QueryRow(`INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'admin') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&adminID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", adminID)
	adminToken := generateToken(adminID, uname, "admin")

	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db); c.Set("role", "admin"); c.Next() })
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	ng := admin.Group("/neighbours")
	ng.Use(adminOnlyMiddleware())
	ng.GET("", adminGetNeighbours(db))
	ng.GET("/:id", adminGetNeighbour(db))
	ng.POST("", adminCreateNeighbour(db, nc))
	ng.PUT("/:id", adminUpdateNeighbour(db, nc))
	ng.DELETE("/:id", adminDeleteNeighbour(db))

	auth := func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
	}

	// ── HTTP URL rejected on create ──
	body := `{"url":"http://insecure.example.com","username":"u","password":"p"}`
	req, _ := http.NewRequest("POST", "/api/v1/admin/neighbours", strings.NewReader(body))
	auth(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "HTTPS")

	// ── Bare hostname (no scheme) rejected ──
	body = `{"url":"peer.example.com","username":"u","password":"p"}`
	req, _ = http.NewRequest("POST", "/api/v1/admin/neighbours", strings.NewReader(body))
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "HTTPS")

	// ── HTTPS URL accepted on create ──
	body = `{"url":"https://secure.example.com","username":"u","password":"p"}`
	req, _ = http.NewRequest("POST", "/api/v1/admin/neighbours", strings.NewReader(body))
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	var created struct{ ID int }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	defer db.Exec("DELETE FROM api_neighbours WHERE id = $1", created.ID)

	// ── GET: disabled=false by default ──
	req, _ = http.NewRequest("GET", "/api/v1/admin/neighbours/"+strconv.Itoa(created.ID), nil)
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var one Neighbour
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &one))
	assert.False(t, one.Disabled)

	// ── PUT: set disabled ──
	disabledBody := `{"url":"https://secure.example.com","username":"u","clear_password":true,"disabled":true}`
	req, _ = http.NewRequest("PUT", "/api/v1/admin/neighbours/"+strconv.Itoa(created.ID), strings.NewReader(disabledBody))
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	req, _ = http.NewRequest("GET", "/api/v1/admin/neighbours/"+strconv.Itoa(created.ID), nil)
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &one))
	assert.True(t, one.Disabled)

	// ── PUT: change URL without password → 400 ──
	noPassBody := `{"url":"https://other.example.com","username":"u"}`
	req, _ = http.NewRequest("PUT", "/api/v1/admin/neighbours/"+strconv.Itoa(created.ID), strings.NewReader(noPassBody))
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, strings.ToLower(w.Body.String()), "пароль")

	// ── PUT: change URL with password → 200 ──
	withPassBody := `{"url":"https://other.example.com","username":"u","password":"newpass"}`
	req, _ = http.NewRequest("PUT", "/api/v1/admin/neighbours/"+strconv.Itoa(created.ID), strings.NewReader(withPassBody))
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestNeighboursAdminOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	// Editor (not admin) must be rejected by adminOnlyMiddleware.
	uname := "nbr_editor_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var editorID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'editor') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&editorID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", editorID)

	editorToken := generateToken(editorID, uname, "editor")

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("role", "editor")
		c.Next()
	})
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	{
		adminNeighbours := admin.Group("/neighbours")
		adminNeighbours.Use(adminOnlyMiddleware())
		{
			adminNeighbours.GET("", adminGetNeighbours(db))
		}
	}

	req, _ := http.NewRequest("GET", "/api/v1/admin/neighbours", nil)
	req.Header.Set("Authorization", "Bearer "+editorToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestRegisterServerRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	// Registration with role=server must create a server account and return a
	// token whose claims carry the server role.
	uname := "srv_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	regBody := map[string]string{
		"username": uname,
		"password": "secret123",
		"role":     "server",
	}
	bodyJSON, _ := json.Marshal(regBody)
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})
	r.POST("/api/v1/auth/register", createUser(db))
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "register failed: %s", w.Body.String())

	var resp AuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "server", resp.User.Role)
	require.NotEmpty(t, resp.Token)
	claims, err := validateToken(resp.Token)
	require.NoError(t, err)
	assert.Equal(t, "server", claims["role"])

	// Cleanup
	db.Exec("DELETE FROM users WHERE username = $1", uname)

	// editor/admin must NOT be self-registrable.
	for _, bad := range []string{"editor", "admin"} {
		badBody, _ := json.Marshal(map[string]string{
			"username": uname + "_" + bad, "password": "secret123", "role": bad,
		})
		req2, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(badBody))
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		require.Equal(t, http.StatusBadRequest, w2.Code, "role %s must not be self-registrable", bad)
	}
}

func TestServerSearchAPIAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	// Create a server-role account.
	uname := "srv_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var srvID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'server') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&srvID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", srvID)

	// Create a viewer account (must be rejected).
	vname := "vw_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var viewerID int
	err = db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'viewer') RETURNING id`,
		vname, "$2a$10$dummyhash").Scan(&viewerID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", viewerID)

	srvToken := generateToken(srvID, uname, "server")
	viewerToken := generateToken(viewerID, vname, "viewer")

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})
	srv := r.Group("/api/v1/server")
	srv.Use(requireAuthMiddleware(), serverOnlyMiddleware())
	{
		srv.GET("/ping", serverPing())
		srv.POST("/search", serverSearchBooks(db))
	}

	// Ping with server role → 200.
	req, _ := http.NewRequest("GET", "/api/v1/server/ping", nil)
	req.Header.Set("Authorization", "Bearer "+srvToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var pingResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pingResp))
	assert.Equal(t, true, pingResp["ok"])

	// Ping with viewer role → 403.
	req, _ = http.NewRequest("GET", "/api/v1/server/ping", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)

	// Ping without token → 401.
	req, _ = http.NewRequest("GET", "/api/v1/server/ping", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// Search with an empty body → 400.
	req, _ = http.NewRequest("POST", "/api/v1/server/search", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer "+srvToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// Search with a title that definitely exists in the test library.
	req, _ = http.NewRequest("POST", "/api/v1/server/search",
		bytes.NewReader([]byte(`{"title":"Test Book"}`)))
	req.Header.Set("Authorization", "Bearer "+srvToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "search failed: %s", w.Body.String())

	var searchResp struct {
		Total int          `json:"total"`
		Books []ServerBook `json:"books"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &searchResp))
	require.GreaterOrEqual(t, searchResp.Total, 1, "expected at least one match")
	for _, b := range searchResp.Books {
		assert.NotEmpty(t, b.Title)
		assert.Greater(t, b.EditionID, 0)
	}
}

// TestServerMetadataAPINullFields covers a regression where serverBookMetadata
// failed with "converting NULL to string is unsupported" whenever an edition
// had NULL metadata columns (isbn, language, publisher, …) or a file with
// NULL file_size/file_hash.
func TestServerMetadataAPINullFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	// Work + edition with most metadata left NULL, file with NULL size/hash.
	var workID, editionID, fileID, personID int
	require.NoError(t, db.QueryRow(
		`INSERT INTO works (original_title) VALUES ('Книга с NULL-полями') RETURNING id`).Scan(&workID))
	require.NoError(t, db.QueryRow(
		`INSERT INTO editions (work_id, title) VALUES ($1, 'Издание с NULL') RETURNING id`, workID).Scan(&editionID))
	require.NoError(t, db.QueryRow(
		`INSERT INTO persons (last_name) VALUES ('Фамилия') RETURNING id`).Scan(&personID))
	_, err := db.Exec(`INSERT INTO work_contributors (work_id, person_id, role) VALUES ($1, $2, 'author')`, workID, personID)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(
		`INSERT INTO edition_files (edition_id, format_id, file_path)
		 VALUES ($1, 1, '/tmp/nonexistent.fb2') RETURNING id`, editionID).Scan(&fileID))
	defer func() {
		db.Exec(`DELETE FROM edition_files WHERE id = $1`, fileID)
		db.Exec(`DELETE FROM work_contributors WHERE work_id = $1`, workID)
		db.Exec(`DELETE FROM editions WHERE id = $1`, editionID)
		db.Exec(`DELETE FROM works WHERE id = $1`, workID)
		db.Exec(`DELETE FROM persons WHERE id = $1`, personID)
	}()

	// Server-role account.
	uname := "srv_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var srvID int
	require.NoError(t, db.QueryRow(
		`INSERT INTO users (username, password_hash, role)
		 VALUES ($1, $2, 'server') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&srvID))
	defer db.Exec("DELETE FROM users WHERE id = $1", srvID)
	srvToken := generateToken(srvID, uname, "server")

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})
	srv := r.Group("/api/v1/server")
	srv.Use(requireAuthMiddleware(), serverOnlyMiddleware())
	srv.GET("/metadata/:id", serverBookMetadata(db))

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/server/metadata/%d", editionID), nil)
	req.Header.Set("Authorization", "Bearer "+srvToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "metadata failed: %s", w.Body.String())

	var meta fedBookMetadata
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &meta))
	assert.Equal(t, editionID, meta.Edition.ID)
	assert.Equal(t, workID, meta.Edition.WorkID)
	assert.Equal(t, "Издание с NULL", meta.Edition.Title)
	// All NULL columns must come back as empty strings, not JSON null.
	assert.Empty(t, meta.Edition.ISBN)
	assert.Empty(t, meta.Edition.Language)
	assert.Empty(t, meta.Edition.Publisher)
	assert.Empty(t, meta.Edition.Series)
	assert.Nil(t, meta.Edition.Year)
	assert.Equal(t, "Книга с NULL-полями", meta.Work.OriginalTitle)
	require.Len(t, meta.Authors, 1)
	assert.Equal(t, personID, meta.Authors[0].ID)
	assert.Equal(t, "Фамилия", meta.Authors[0].LastName)
	require.Len(t, meta.Files, 1)
	assert.Equal(t, fileID, meta.Files[0].ID)
	assert.Equal(t, int64(0), meta.Files[0].FileSize)
	assert.Empty(t, meta.Files[0].FileHash)

	// Non-existent edition → 404.
	req, _ = http.NewRequest("GET", "/api/v1/server/metadata/999999999", nil)
	req.Header.Set("Authorization", "Bearer "+srvToken)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func testSelfSignedCert(t *testing.T) string {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestFederationSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	// Admin token for the calling server.
	uname := "fed_admin_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var adminID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'admin') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&adminID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", adminID)
	adminToken := generateToken(adminID, uname, "admin")

	// Mock neighbour server that speaks the peer API.
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/login":
			var lr LoginRequest
			json.NewDecoder(r.Body).Decode(&lr)
			if lr.Username != "peeruser" || lr.Password != "peerpass" {
				http.Error(w, `{"error":"bad creds"}`, http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token": "peer-jwt",
				"user":  map[string]interface{}{"username": "peeruser", "role": "server"},
			})
		case "/api/v1/server/search":
			if r.Header.Get("Authorization") != "Bearer peer-jwt" {
				http.Error(w, `{"error":"no auth"}`, http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"total": 1,
				"books": []ServerBook{{WorkID: 5, EditionID: 9, Author: "Mock Author", Title: "Mock Book", Year: 2001, Formats: []string{"FB2"}}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mock.Close()

	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	// Snapshot existing neighbours and clear the table so the live neighbour
	// (http://192.168.95.200:9091/) does not participate in this test.
	backupRows, err := db.Query(
		`SELECT id, url, COALESCE(server_cert,''), COALESCE(client_cert,''), COALESCE(username,''), COALESCE(password_encrypted,'')
		 FROM api_neighbours`)
	require.NoError(t, err)
	type nBackup struct {
		id         int
		url        string
		serverCert string
		clientCert string
		username   string
		enc        string
	}
	var backups []nBackup
	for backupRows.Next() {
		var b nBackup
		require.NoError(t, backupRows.Scan(&b.id, &b.url, &b.serverCert, &b.clientCert, &b.username, &b.enc))
		backups = append(backups, b)
	}
	backupRows.Close()
	_, err = db.Exec(`DELETE FROM api_neighbours`)
	require.NoError(t, err)
	restore := func() {
		for _, b := range backups {
			db.Exec(`INSERT INTO api_neighbours (id, url, server_cert, client_cert, username, password_encrypted)
				VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO UPDATE SET
				url=$2, server_cert=$3, client_cert=$4, username=$5, password_encrypted=$6`,
				b.id, b.url, b.serverCert, b.clientCert, b.username, b.enc)
		}
	}
	defer restore()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("role", "admin")
		c.Next()
	})
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	{
		admin.POST("/federation/search", adminOnlyMiddleware(), adminFederationSearch(db, nc))
	}
	auth := func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
	}

	// With zero neighbours → neighbours: 0, empty results.
	req, _ := http.NewRequest("POST", "/api/v1/admin/federation/search",
		bytes.NewReader([]byte(`{"title":"Mock"}`)))
	auth(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var empty struct {
		Neighbours int                `json:"neighbours"`
		Results    []FederationResult `json:"results"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &empty))
	assert.Equal(t, 0, empty.Neighbours)
	assert.Empty(t, empty.Results)

	// Add a neighbour pointing at the mock server (password stored encrypted).
	encPass, err := nc.Encrypt("peerpass")
	require.NoError(t, err)
	var nid int
	err = db.QueryRow(`
		INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		mock.URL, testSelfSignedCert(t), "", "peeruser", encPass).Scan(&nid)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM api_neighbours WHERE id = $1", nid)

	// Empty search body → 400.
	req, _ = http.NewRequest("POST", "/api/v1/admin/federation/search", bytes.NewReader([]byte(`{}`)))
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// Valid search → mock neighbour returns one book.
	req, _ = http.NewRequest("POST", "/api/v1/admin/federation/search?limit=20",
		bytes.NewReader([]byte(`{"title":"Mock"}`)))
	auth(req)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "federation search failed: %s", w.Body.String())

	var resp struct {
		Neighbours int                `json:"neighbours"`
		Results    []FederationResult `json:"results"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Neighbours)
	require.Len(t, resp.Results, 1)
	res := resp.Results[0]
	assert.Equal(t, nid, res.NeighbourID)
	assert.Empty(t, res.Error)
	assert.Equal(t, 1, res.Total)
	require.Len(t, res.Books, 1)
	assert.Equal(t, "Mock Book", res.Books[0].Title)
	assert.Equal(t, "Mock Author", res.Books[0].Author)

	// Editor role must be rejected (admin-only route).
	editorUname := "fed_editor_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var editorID int
	err = db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'editor') RETURNING id`,
		editorUname, "$2a$10$dummyhash").Scan(&editorID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", editorID)
	editorToken := generateToken(editorID, editorUname, "editor")
	req, _ = http.NewRequest("POST", "/api/v1/admin/federation/search",
		bytes.NewReader([]byte(`{"title":"Mock"}`)))
	req.Header.Set("Authorization", "Bearer "+editorToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "editor must not run federated search")
}

// ─── Federation: download + import, stop-on-first ─────────────

// fedMock is a minimal neighbour that speaks the peer API and can serve a
// downloadable book archive for import tests.
type fedMock struct {
	srv           *httptest.Server
	searchHits    int32
	downloadHits  int32
	metadataHits  int32
	pingHits      int32
	metadataTitle string
}

func (m *fedMock) Close() { m.srv.Close() }

func newFedMockNeighbour(title, author string, editionID int, bookData []byte) *fedMock {
	// Default remote identifiers, large enough to never collide with the live
	// library data used by the tests.
	return newFedMockNeighbourMeta(title, author, editionID, 2_400_001, 2_400_002, bookData)
}

// newFedMockNeighbourMeta is like newFedMockNeighbour but exposes explicit
// remote identifiers for work and author via the metadata endpoint.
func newFedMockNeighbourMeta(title, author string, editionID, workID, authorID int, bookData []byte) *fedMock {
	m := &fedMock{metadataTitle: title}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var lr LoginRequest
		json.NewDecoder(r.Body).Decode(&lr)
		if lr.Username != "peeruser" || lr.Password != "peerpass" {
			http.Error(w, `{"error":"bad creds"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"token": "peer-jwt",
			"user":  map[string]interface{}{"username": "peeruser", "role": "server"},
		})
	})
	mux.HandleFunc("/api/v1/server/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer peer-jwt" {
			http.Error(w, `{"error":"no auth"}`, http.StatusUnauthorized)
			return
		}
		atomic.AddInt32(&m.pingHits, 1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "server": "Home Library Manager", "api": "v1"})
	})
	mux.HandleFunc("/api/v1/server/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer peer-jwt" {
			http.Error(w, `{"error":"no auth"}`, http.StatusUnauthorized)
			return
		}
		atomic.AddInt32(&m.searchHits, 1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total": 1,
			"books": []ServerBook{{WorkID: workID, EditionID: editionID, Author: author, Title: title, Year: 2020, Formats: []string{"FB2"}}},
		})
	})
	mux.HandleFunc("/api/v1/server/metadata/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer peer-jwt" {
			http.Error(w, `{"error":"no auth"}`, http.StatusUnauthorized)
			return
		}
		atomic.AddInt32(&m.metadataHits, 1)
		fn, ln := "", author
		if fields := strings.Fields(author); len(fields) > 1 {
			fn, ln = fields[0], strings.Join(fields[1:], " ")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fedBookMetadata{
			Work:    fedWorkMeta{ID: workID, OriginalTitle: title, OriginalLanguage: "rus", WorkType: "novel"},
			Edition: fedEditionMeta{ID: editionID, WorkID: workID, Title: title, Language: "rus", IsComplete: true},
			Authors: []fedAuthorMeta{{ID: authorID, FirstName: fn, LastName: ln, Role: "author"}},
			Genres:  []fedGenreMeta{},
			Files:   []fedFileMeta{},
		})
	})
	mux.HandleFunc("/api/v1/server/download/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer peer-jwt" {
			http.Error(w, `{"error":"no auth"}`, http.StatusUnauthorized)
			return
		}
		atomic.AddInt32(&m.downloadHits, 1)
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="fed_book.zip"`)
		w.Write(bookData)
	})
	m.srv = httptest.NewServer(mux)
	return m
}

// makeFB2Zip wraps a minimal parseable FB2 into a single-entry zip, matching
// how the library stores editions on disk.
func makeFB2Zip(title, authorFirst, authorLast string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, _ := zw.Create("book.fb2")
	fb2 := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<FictionBook>
  <description>
    <title-info>
      <author><first-name>%s</first-name><last-name>%s</last-name></author>
      <book-title>%s</book-title>
      <lang>ru</lang>
      <date>2020</date>
    </title-info>
  </description>
  <body><section><title><p>Глава</p></title><p>Текст.</p></section></body>
</FictionBook>`, authorFirst, authorLast, title)
	fw.Write([]byte(fb2))
	zw.Close()
	return buf.Bytes()
}

// backupNeighbours snapshots api_neighbours, clears the table and returns a
// restore func, so the live neighbours (http://192.168.95.200:9091/) never
// participate in federation tests.
func backupNeighbours(t *testing.T, db *sql.DB) func() {
	rows, err := db.Query(`
		SELECT id, url, COALESCE(server_cert,''), COALESCE(client_cert,''), COALESCE(username,''), COALESCE(password_encrypted,'')
		FROM api_neighbours`)
	require.NoError(t, err)
	type nBackup struct {
		id         int
		url        string
		serverCert string
		clientCert string
		username   string
		enc        string
	}
	var backups []nBackup
	for rows.Next() {
		var b nBackup
		require.NoError(t, rows.Scan(&b.id, &b.url, &b.serverCert, &b.clientCert, &b.username, &b.enc))
		backups = append(backups, b)
	}
	rows.Close()
	_, err = db.Exec(`DELETE FROM api_neighbours`)
	require.NoError(t, err)
	return func() {
		for _, b := range backups {
			db.Exec(`INSERT INTO api_neighbours (id, url, server_cert, client_cert, username, password_encrypted)
				VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO UPDATE SET
				url=$2, server_cert=$3, client_cert=$4, username=$5, password_encrypted=$6`,
				b.id, b.url, b.serverCert, b.clientCert, b.username, b.enc)
		}
	}
}

// cleanupImportedBook removes the DB rows created by a federation import and
// restores the SERIAL sequences so the forced high identifiers used by the
// tests do not leak into the live library data.
func cleanupImportedBook(db *sql.DB, workID, editionID int) {
	db.Exec(`DELETE FROM edition_files WHERE edition_id = $1`, editionID)
	db.Exec(`DELETE FROM editions WHERE id = $1`, editionID)
	db.Exec(`DELETE FROM work_contributors WHERE work_id = $1`, workID)
	db.Exec(`DELETE FROM work_genres WHERE work_id = $1`, workID)
	db.Exec(`DELETE FROM works WHERE id = $1`, workID)
	db.Exec(`DELETE FROM persons p WHERE NOT EXISTS (SELECT 1 FROM work_contributors wc WHERE wc.person_id = p.id)`)
	for _, t := range []string{"persons", "works", "editions", "genres"} {
		db.Exec(fmt.Sprintf(`SELECT setval('%s_id_seq', GREATEST((SELECT COALESCE(MAX(id),1) FROM %s), 1))`, t, t))
	}
}

func TestFederationImport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	uname := "fedimp_admin_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var adminID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'admin') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&adminID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", adminID)
	adminToken := generateToken(adminID, uname, "admin")

	dir := t.TempDir()
	testCfg := config.DefaultConfig()
	testCfg.Directories.Bookarch = filepath.Join(dir, "bookarch")
	testCfg.Directories.Temp = filepath.Join(dir, "temp")

	mock := newFedMockNeighbour("Тестовая книга федерации", "Фед Федеративный", 2_300_007, makeFB2Zip("Тестовая книга федерации", "Фед", "Федеративный"))
	defer mock.Close()

	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	restore := backupNeighbours(t, db)
	defer restore()

	encPass, err := nc.Encrypt("peerpass")
	require.NoError(t, err)
	var nid int
	err = db.QueryRow(`
		INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
		VALUES ($1, '', '', 'peeruser', $2) RETURNING id`,
		mock.srv.URL, encPass).Scan(&nid)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM api_neighbours WHERE id = $1", nid)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("config", testCfg)
		c.Set("role", "admin")
		c.Next()
	})
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	{
		admin.POST("/federation/import", adminOnlyMiddleware(), adminFederationImport(db, nc))
	}
	auth := func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
	}

	body := `{"neighbour_id":` + strconv.Itoa(nid) + `,"edition_id":2300007}`
	req, _ := http.NewRequest("POST", "/api/v1/admin/federation/import", bytes.NewReader([]byte(body)))
	auth(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "import failed: %s", w.Body.String())

	var resp struct {
		Message   string `json:"message"`
		Title     string `json:"title"`
		Mode      string `json:"mode"`
		Authors   string `json:"authors"`
		WorkID    int    `json:"work_id"`
		EditionID int    `json:"edition_id"`
		FileHash  string `json:"file_hash"`
		Conflict  bool   `json:"conflict"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.False(t, resp.Conflict)
	assert.Equal(t, "Тестовая книга федерации", resp.Title)
	assert.Equal(t, "created", resp.Mode)
	require.Greater(t, resp.WorkID, 0)
	require.Greater(t, resp.EditionID, 0)

	// The remote author was not present locally, so a new person must have been
	// created with the SAME id as on the remote server.
	var storedAuthorID int
	err = db.QueryRow(`
		SELECT wc.person_id FROM work_contributors wc WHERE wc.work_id=$1 AND wc.role='author'`, resp.WorkID).Scan(&storedAuthorID)
	require.NoError(t, err)
	assert.Equal(t, 2_400_002, storedAuthorID, "created author id must match the remote author id")

	// The work and edition must carry the remote identifiers.
	var storedWorkID, storedEditionID int
	err = db.QueryRow("SELECT id FROM works WHERE id=$1", 2_400_001).Scan(&storedWorkID)
	require.NoError(t, err)
	assert.Equal(t, 2_400_001, storedWorkID)
	err = db.QueryRow("SELECT id FROM editions WHERE id=$1", 2_300_007).Scan(&storedEditionID)
	require.NoError(t, err)
	assert.Equal(t, 2_300_007, storedEditionID)

	var storedTitle string
	err = db.QueryRow("SELECT title FROM editions WHERE id = $1", resp.EditionID).Scan(&storedTitle)
	require.NoError(t, err)
	assert.Equal(t, "Тестовая книга федерации", storedTitle)

	// Re-import of the same file must report a duplicate (content-hash check).
	req2, _ := http.NewRequest("POST", "/api/v1/admin/federation/import", bytes.NewReader([]byte(body)))
	auth(req2)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code, "second import: %s", w2.Body.String())
	var dupResp struct {
		Duplicate bool `json:"duplicate"`
	}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &dupResp))
	assert.True(t, dupResp.Duplicate, "second import must report a duplicate")

	// Bad neighbour id → 404.
	req3, _ := http.NewRequest("POST", "/api/v1/admin/federation/import",
		bytes.NewReader([]byte(`{"neighbour_id":999999,"edition_id":2300007}`)))
	auth(req3)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	require.Equal(t, http.StatusNotFound, w3.Code)

	// Editor must be rejected (admin-only route).
	editorUname := "fedimp_editor_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var editorID int
	err = db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'editor') RETURNING id`,
		editorUname, "$2a$10$dummyhash").Scan(&editorID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", editorID)
	editorToken := generateToken(editorID, editorUname, "editor")
	req4, _ := http.NewRequest("POST", "/api/v1/admin/federation/import", bytes.NewReader([]byte(body)))
	req4.Header.Set("Authorization", "Bearer "+editorToken)
	req4.Header.Set("Content-Type", "application/json")
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	require.Equal(t, http.StatusForbidden, w4.Code)

	cleanupImportedBook(db, resp.WorkID, resp.EditionID)
}

// TestFederationImportConflict covers the identifier-conflict resolution of the
// federation import: an initial call must return 409 without touching the
// library, "create_new" must generate fresh ids (author reused via fuzzy
// match), and "overwrite" must replace the colliding rows keeping the remote
// identifiers.
func TestFederationImportConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	// Remote identifiers used by the mock metadata.
	const (
		authorID  = 2_500_001
		workID    = 2_500_002
		editionID = 2_500_003
		title     = "Конфликтная книга федерации"
		author    = "Колин Авторович"
	)

	// Pre-seed the local library with rows that occupy the same identifiers.
	_, err := db.Exec(`
		INSERT INTO persons (id, first_name, last_name) VALUES ($1, 'Колин', 'Авторович')`, authorID)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO works (id, original_title) VALUES ($1, 'Старая локальная работа')`, workID)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO editions (id, work_id, title) VALUES ($1, $2, 'Старая локальная книга')`, editionID, workID)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO edition_files (edition_id, format_id, file_path, file_size, file_hash, is_primary)
		VALUES ($1, 1, 'bookarch/fake_old.zip', 10, 'fedtest_old_hash_0000000000000000000000000', true)`, editionID)
	require.NoError(t, err)
	defer cleanupImportedBook(db, workID, editionID)

	uname := "fedconf_admin_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var adminID int
	err = db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'admin') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&adminID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", adminID)
	adminToken := generateToken(adminID, uname, "admin")

	dir := t.TempDir()
	testCfg := config.DefaultConfig()
	testCfg.Directories.Bookarch = filepath.Join(dir, "bookarch")
	testCfg.Directories.Temp = filepath.Join(dir, "temp")

	mock := newFedMockNeighbourMeta(title, author, editionID, workID, authorID,
		makeFB2Zip(title, "Колин", "Авторович"))
	defer mock.Close()

	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	restore := backupNeighbours(t, db)
	defer restore()

	encPass, err := nc.Encrypt("peerpass")
	require.NoError(t, err)
	var nid int
	err = db.QueryRow(`
		INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
		VALUES ($1, '', '', 'peeruser', $2) RETURNING id`,
		mock.srv.URL, encPass).Scan(&nid)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM api_neighbours WHERE id = $1", nid)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("config", testCfg)
		c.Set("role", "admin")
		c.Set("user_id", adminID)
		c.Next()
	})
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	{
		admin.POST("/federation/import", adminOnlyMiddleware(), adminFederationImport(db, nc))
	}
	auth := func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")
	}
	post := func(mode string, wantCode int) (string, int) {
		body := `{"neighbour_id":` + strconv.Itoa(nid) + `,"edition_id":` + strconv.Itoa(editionID) +
			`,"mode":"` + mode + `"}`
		req, _ := http.NewRequest("POST", "/api/v1/admin/federation/import", bytes.NewReader([]byte(body)))
		auth(req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, wantCode, w.Code, "body: %s", w.Body.String())
		return w.Body.String(), w.Code
	}
	type importResp struct {
		Mode      string `json:"mode"`
		WorkID    int    `json:"work_id"`
		EditionID int    `json:"edition_id"`
		Conflict  bool   `json:"conflict"`
		Found     struct {
			EditionID int    `json:"edition_id"`
			Title     string `json:"title"`
		} `json:"found"`
	}

	// 1. Initial import → 409, nothing written to the library.
	body, code := post("", http.StatusConflict)
	require.Equal(t, http.StatusConflict, code)
	var cResp importResp
	require.NoError(t, json.Unmarshal([]byte(body), &cResp))
	assert.True(t, cResp.Conflict)
	assert.Equal(t, editionID, cResp.Found.EditionID)
	assert.Equal(t, "Старая локальная книга", cResp.Found.Title)

	// The local rows must be untouched after the 409.
	var oldTitle string
	err = db.QueryRow(`SELECT original_title FROM works WHERE id=$1`, workID).Scan(&oldTitle)
	require.NoError(t, err)
	assert.Equal(t, "Старая локальная работа", oldTitle)
	err = db.QueryRow(`SELECT title FROM editions WHERE id=$1`, editionID).Scan(&oldTitle)
	require.NoError(t, err)
	assert.Equal(t, "Старая локальная книга", oldTitle)

	// 2. create_new → fresh ids, author reused via fuzzy match, old rows intact.
	body, _ = post("create_new", http.StatusOK)
	var cnResp importResp
	require.NoError(t, json.Unmarshal([]byte(body), &cnResp))
	assert.Equal(t, "created_new", cnResp.Mode)
	require.NotEqual(t, workID, cnResp.WorkID, "create_new must generate a new work id")
	require.NotEqual(t, editionID, cnResp.EditionID, "create_new must generate a new edition id")
	var newTitle string
	err = db.QueryRow(`SELECT original_title FROM works WHERE id=$1`, cnResp.WorkID).Scan(&newTitle)
	require.NoError(t, err)
	assert.Equal(t, title, newTitle)
	var personID int
	err = db.QueryRow(`SELECT wc.person_id FROM work_contributors wc WHERE wc.work_id=$1 AND wc.role='author'`, cnResp.WorkID).Scan(&personID)
	require.NoError(t, err)
	assert.Equal(t, authorID, personID, "author must be fuzzy-matched and reused")
	// Old rows still intact.
	err = db.QueryRow(`SELECT original_title FROM works WHERE id=$1`, workID).Scan(&oldTitle)
	require.NoError(t, err)
	assert.Equal(t, "Старая локальная работа", oldTitle)
	// NOTE: the create_new edition is intentionally NOT cleaned up yet — its
	// file row holds the same content hash as the upcoming overwrite, which
	// exercises the UNIQUE(file_hash) collision path.

	// The person occupying the remote author id is now renamed locally. The
	// overwrite must REPLACE it with the remote data (exact-id match first)
	// rather than fuzzy-matching an unrelated person.
	_, err = db.Exec(`UPDATE persons SET first_name='Псевдо', last_name='Авторчик' WHERE id=$1`, authorID)
	require.NoError(t, err)

	// 3. overwrite → same ids, data replaced with the remote one. The content
	// hash is already held by the create_new edition, so the overwritten file
	// must be stored WITHOUT a dedup hash instead of violating the constraint.
	body, _ = post("overwrite", http.StatusOK)
	var owResp importResp
	require.NoError(t, json.Unmarshal([]byte(body), &owResp))
	assert.Equal(t, "overwritten", owResp.Mode)
	assert.Equal(t, workID, owResp.WorkID)
	assert.Equal(t, editionID, owResp.EditionID)
	err = db.QueryRow(`SELECT original_title FROM works WHERE id=$1`, workID).Scan(&newTitle)
	require.NoError(t, err)
	assert.Equal(t, title, newTitle)
	err = db.QueryRow(`SELECT title FROM editions WHERE id=$1`, editionID).Scan(&newTitle)
	require.NoError(t, err)
	assert.Equal(t, title, newTitle)
	personID = 0
	err = db.QueryRow(`SELECT wc.person_id FROM work_contributors wc WHERE wc.work_id=$1 AND wc.role='author'`, workID).Scan(&personID)
	require.NoError(t, err)
	assert.Equal(t, authorID, personID, "overwrite must link the person occupying the remote id")
	var fn, ln string
	err = db.QueryRow(`SELECT first_name, last_name FROM persons WHERE id=$1`, authorID).Scan(&fn, &ln)
	require.NoError(t, err)
	assert.Equal(t, "Колин", fn, "overwrite must replace the conflicting person's data")
	assert.Equal(t, "Авторович", ln)
	// The hash must now be NULL because the create_new edition already holds it.
	var hashPtr sql.NullString
	err = db.QueryRow(`SELECT file_hash FROM edition_files WHERE edition_id=$1 AND is_primary=true`, editionID).Scan(&hashPtr)
	require.NoError(t, err)
	assert.False(t, hashPtr.Valid, "overwritten file must store a NULL hash when the content is already present locally")
	// And the create_new edition must still own the hash.
	var cnHash string
	err = db.QueryRow(`SELECT file_hash FROM edition_files WHERE edition_id=$1 AND is_primary=true`, cnResp.EditionID).Scan(&cnHash)
	require.NoError(t, err)
	assert.NotEmpty(t, cnHash)
	cleanupImportedBook(db, cnResp.WorkID, cnResp.EditionID)
}

func TestFederationSearchStopOnFirst(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	uname := "fedstop_admin_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var adminID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'admin') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&adminID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", adminID)
	adminToken := generateToken(adminID, uname, "admin")

	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	restore := backupNeighbours(t, db)
	defer restore()

	bookData := makeFB2Zip("Тест", "Фед", "Федеративный")
	mockA := newFedMockNeighbour("Книга A", "Автор A", 1, bookData)
	defer mockA.Close()
	mockB := newFedMockNeighbour("Книга B", "Автор B", 2, bookData)
	defer mockB.Close()

	encPass, err := nc.Encrypt("peerpass")
	require.NoError(t, err)
	var nids []int
	for _, url := range []string{mockA.srv.URL, mockB.srv.URL} {
		var id int
		err = db.QueryRow(`
			INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
			VALUES ($1, '', '', 'peeruser', $2) RETURNING id`,
			url, encPass).Scan(&id)
		require.NoError(t, err)
		nids = append(nids, id)
	}
	defer func() {
		for _, id := range nids {
			db.Exec("DELETE FROM api_neighbours WHERE id = $1", id)
		}
	}()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("role", "admin")
		c.Next()
	})
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	{
		admin.POST("/federation/search", adminOnlyMiddleware(), adminFederationSearch(db, nc))
	}

	req, _ := http.NewRequest("POST", "/api/v1/admin/federation/search?stop_on_first=1",
		bytes.NewReader([]byte(`{"query":"Книга"}`)))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "federation search failed: %s", w.Body.String())

	var resp struct {
		Neighbours int                `json:"neighbours"`
		Results    []FederationResult `json:"results"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 2, resp.Neighbours)
	// The search must stop after the first neighbour that returned books,
	// so only one neighbour is present in the results.
	require.Len(t, resp.Results, 1)
	assert.Empty(t, resp.Results[0].Error)
	require.Len(t, resp.Results[0].Books, 1)
	assert.Contains(t, []string{"Книга A", "Книга B"}, resp.Results[0].Books[0].Title)

	// The second neighbour must not have been contacted at all.
	totalHits := atomic.LoadInt32(&mockA.searchHits) + atomic.LoadInt32(&mockB.searchHits)
	assert.Equal(t, int32(1), totalHits, "stop_on_first must not query more than one neighbour")
}

// TestFederationSearchContinuesAfterError verifies that an unavailable or
// failing neighbour does not abort the search: the query continues with the
// remaining neighbours and stops only when a neighbour returns books.
func TestFederationSearchContinuesAfterError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	uname := "federr_admin_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var adminID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'admin') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&adminID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", adminID)
	adminToken := generateToken(adminID, uname, "admin")

	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	restore := backupNeighbours(t, db)
	defer restore()

	bookData := makeFB2Zip("Тест", "Фед", "Федеративный")

	// Neighbour 1 is down (connection refused). Port 1 is not listening, and
	// its URL sorts before any ephemeral-port mock URL ("http://127.0.0.1:1"
	// < "http://127.0.0.1:3xxxx"), so the failing neighbour is queried first.
	downURL := "http://127.0.0.1:1"
	mockOK := newFedMockNeighbour("Книга B", "Автор B", 2, bookData)
	defer mockOK.Close()

	encPass, err := nc.Encrypt("peerpass")
	require.NoError(t, err)
	var nids []int
	for _, url := range []string{downURL, mockOK.srv.URL} {
		var id int
		err = db.QueryRow(`
			INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
			VALUES ($1, '', '', 'peeruser', $2) RETURNING id`,
			url, encPass).Scan(&id)
		require.NoError(t, err)
		nids = append(nids, id)
	}
	defer func() {
		for _, id := range nids {
			db.Exec("DELETE FROM api_neighbours WHERE id = $1", id)
		}
	}()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("role", "admin")
		c.Next()
	})
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	{
		admin.POST("/federation/search", adminOnlyMiddleware(), adminFederationSearch(db, nc))
	}

	req, _ := http.NewRequest("POST", "/api/v1/admin/federation/search",
		bytes.NewReader([]byte(`{"query":"Книга"}`)))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "federation search failed: %s", w.Body.String())

	var resp struct {
		Neighbours int                `json:"neighbours"`
		Results    []FederationResult `json:"results"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 2, resp.Neighbours)

	// Parallel mode (no stop_on_first) does not randomize the traversal order,
	// so the down neighbour (index 0) is reported as an error and the working
	// one still returns its book — errors never abort the search.
	require.Len(t, resp.Results, 2)
	assert.NotEmpty(t, resp.Results[0].Error, "unavailable neighbour must be reported")
	assert.Empty(t, resp.Results[1].Error)
	require.Len(t, resp.Results[1].Books, 1)
	assert.Equal(t, "Книга B", resp.Results[1].Books[0].Title)

	// The working neighbour was still queried despite the failed one.
	assert.Equal(t, int32(1), atomic.LoadInt32(&mockOK.searchHits),
		"search must continue past the failed neighbour")
}

// TestFederationTest verifies the "Тест" connectivity button handler:
// POST /api/v1/admin/federation/test logs in to the neighbour with the stored
// credentials, sends a ping, returns ok on success, logs errors on failure,
// and is admin-only.
func TestFederationTest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")

	db := setupTestDB()
	defer db.Close()

	uname := "fedtest_admin_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var adminID int
	err := db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'admin') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&adminID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", adminID)
	adminToken := generateToken(adminID, uname, "admin")

	var editorID int
	err = db.QueryRow(`
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, 'editor') RETURNING id`,
		uname+"_ed", "$2a$10$dummyhash").Scan(&editorID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM users WHERE id = $1", editorID)
	editorToken := generateToken(editorID, uname+"_ed", "editor")

	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	restore := backupNeighbours(t, db)
	defer restore()

	bookData := makeFB2Zip("Тест", "Фед", "Федеративный")
	mock := newFedMockNeighbour("Книга", "Автор", 1, bookData)
	defer mock.Close()

	encPass, err := nc.Encrypt("peerpass")
	require.NoError(t, err)
	var okID, downID, missID int
	// Working neighbour on the mock server.
	err = db.QueryRow(`
		INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
		VALUES ($1, '', '', 'peeruser', $2) RETURNING id`,
		mock.srv.URL, encPass).Scan(&okID)
	require.NoError(t, err)
	// Unreachable neighbour (connection refused on port 1).
	err = db.QueryRow(`
		INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
		VALUES ($1, '', '', 'peeruser', $2) RETURNING id`,
		"http://127.0.0.1:1", encPass).Scan(&downID)
	require.NoError(t, err)
	// Non-existent neighbour.
	err = db.QueryRow(`SELECT COALESCE(MAX(id),0)+1 FROM api_neighbours`).Scan(&missID)
	require.NoError(t, err)
	defer func() {
		db.Exec("DELETE FROM api_neighbours WHERE id IN ($1,$2)", okID, downID)
	}()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("role", "admin")
		c.Next()
	})
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	{
		admin.POST("/federation/test", adminOnlyMiddleware(), adminFederationTest(db, nc))
	}

	// 1. Success against the working neighbour.
	req, _ := http.NewRequest("POST", "/api/v1/admin/federation/test",
		bytes.NewReader([]byte(`{"neighbour_id":`+strconv.Itoa(okID)+`}`)))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "test failed: %s", w.Body.String())
	var okResp struct {
		Ok bool `json:"ok"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &okResp))
	assert.True(t, okResp.Ok)
	assert.Equal(t, int32(1), atomic.LoadInt32(&mock.pingHits), "neighbour must receive exactly one ping")

	// 2. Failure against the unreachable neighbour → 502, ok=false.
	req, _ = http.NewRequest("POST", "/api/v1/admin/federation/test",
		bytes.NewReader([]byte(`{"neighbour_id":`+strconv.Itoa(downID)+`}`)))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadGateway, w.Code, "unreachable neighbour must fail: %s", w.Body.String())
	var failResp struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &failResp))
	assert.False(t, failResp.Ok)
	assert.NotEmpty(t, failResp.Error)

	// 3. Missing neighbour → 404.
	req, _ = http.NewRequest("POST", "/api/v1/admin/federation/test",
		bytes.NewReader([]byte(`{"neighbour_id":`+strconv.Itoa(missID)+`}`)))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)

	// 4. Role guard: editor is rejected even with a valid payload.
	req, _ = http.NewRequest("POST", "/api/v1/admin/federation/test",
		bytes.NewReader([]byte(`{"neighbour_id":`+strconv.Itoa(okID)+`}`)))
	req.Header.Set("Authorization", "Bearer "+editorToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "editor must not run federation tests")

	// 5. The working neighbour was contacted exactly once across all attempts.
	assert.Equal(t, int32(1), atomic.LoadInt32(&mock.pingHits))
}
