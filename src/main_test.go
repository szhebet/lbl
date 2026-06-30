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

	// Test connection
	err = db.Ping()
	if err != nil {
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