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
	assert.NotZero(t, item.ID)
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
	_, err = db.Exec(`
		INSERT INTO read_list (listname, bookname, author, priority, user_id, comment, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'Читаю'::user_book_status)
	`, "default", "Book1", "Author1", 1, userID, "comment1")
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
	_, err = db.Exec(`
		INSERT INTO read_list (listname, bookname, author, priority, user_id, status)
		VALUES ('default', 'User1Book', 'User1Author', 1, $1, 'Читаю'::user_book_status)
	`, user1ID)
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
	var itemID int
	err = db.QueryRow(`
		INSERT INTO read_list (listname, bookname, author, priority, user_id, status)
		VALUES ('oldlist', 'OldBook', 'OldAuthor', 1, $1, 'Не заполнено'::user_book_status)
		RETURNING id
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
	req, _ := http.NewRequest("PUT", "/api/v1/user/readlist/"+strconv.Itoa(itemID), bytes.NewReader(bodyJSON))
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

	// Delete
	req2, _ := http.NewRequest("DELETE", "/api/v1/user/readlist/"+strconv.Itoa(itemID), nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusNoContent, w2.Code)

	// Verify deleted
	var count int
	db.QueryRow("SELECT COUNT(*) FROM read_list WHERE id = $1", itemID).Scan(&count)
	assert.Equal(t, 0, count)
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
		INSERT INTO read_list (listname, bookname, user_id, status) VALUES
		('favorites', 'B1', $1, 'Не заполнено'::user_book_status),
		('favorites', 'B2', $1, 'Не заполнено'::user_book_status),
		('wishlist', 'B3', $1, 'Не заполнено'::user_book_status)
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