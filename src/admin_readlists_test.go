package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAdminReadlistsRouter(db *sql.DB) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	{
		admin.GET("/readlists", adminListReadLists(db))
		admin.POST("/readlists", adminCreateReadListItems(db))
		admin.GET("/readlists/children", adminListChildren(db))
		admin.PUT("/readlists/:id", adminUpdateReadListItem(db))
		admin.DELETE("/readlists/:id", adminDeleteReadListItem(db))
	}
	return r
}

// insertUserWithParent inserts a child user and records parentID as its parent.
// Returns child id + token.
func insertUserWithParent(t *testing.T, db *sql.DB, parentID int) (int, string) {
	t.Helper()
	var childID int
	uname := "arl_child_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	err := db.QueryRow(
		`INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'viewer') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&childID)
	require.NoError(t, err)
	t.Cleanup(func() { db.Exec("DELETE FROM users WHERE id = $1", childID) })
	_, err = db.Exec("INSERT INTO user_parents (user_id, parent_id) VALUES ($1, $2)", childID, parentID)
	require.NoError(t, err)
	t.Cleanup(func() { db.Exec("DELETE FROM user_parents WHERE user_id = $1", childID) })
	return childID, generateToken(childID, uname, "viewer")
}

func insertReadListItemFor(t *testing.T, db *sql.DB, userID int, listname, bookname, author, status string) string {
	t.Helper()
	var itemID string
	err := db.QueryRow(`
		INSERT INTO read_list (id, listname, bookname, author, priority, user_id, comment, status, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, 5, $4, 'c', $5::user_book_status, NOW())
		RETURNING id::text
	`, listname, bookname, author, userID, status).Scan(&itemID)
	require.NoError(t, err)
	t.Cleanup(func() { db.Exec("DELETE FROM read_list WHERE id = $1::uuid", itemID) })
	return itemID
}

func TestAdminReadListsParentVisibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)

	childID, _ := insertUserWithParent(t, db, adminID)

	// A stranger (another admin) is NOT a parent of the child
	_, strangerToken := insertAdminUser(t, db)

	itemID := insertReadListItemFor(t, db, childID, "default", "BookX", "AuthorX", "Читаю")

	r := setupAdminReadlistsRouter(db)

	// Admin (parent) sees the item
	w := doJSON(t, r, "GET", "/api/v1/admin/readlists", nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp struct {
		Total int                 `json:"total"`
		Items []AdminReadListItem `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	var found bool
	for _, it := range resp.Items {
		if it.ID == itemID {
			found = true
			assert.Equal(t, childID, it.UserID)
			assert.Equal(t, "BookX", it.Bookname)
			assert.Equal(t, "AuthorX", it.Author)
			assert.Equal(t, "default", it.Listname)
			assert.Equal(t, "Читаю", it.Status)
			assert.NotEmpty(t, it.Username)
		}
	}
	assert.True(t, found, "parent should see the child's read-list item")

	// Stranger (not a parent) does NOT see the item
	w = doJSON(t, r, "GET", "/api/v1/admin/readlists", nil, strangerToken)
	require.Equal(t, http.StatusOK, w.Code)
	resp = struct {
		Total int                 `json:"total"`
		Items []AdminReadListItem `json:"items"`
	}{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	for _, it := range resp.Items {
		assert.NotEqual(t, itemID, it.ID, "stranger should not see the item")
	}
}

func TestAdminReadListsFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-filter-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)
	childID, _ := insertUserWithParent(t, db, adminID)

	insertReadListItemFor(t, db, childID, "Книжный список", "Книга о море", "Иван Мореход", "Читаю")
	insertReadListItemFor(t, db, childID, "Книжный список", "Другая книга", "Пётр Суша", "Прочитано")
	insertReadListItemFor(t, db, childID, "Научная фантастика", "Звёздный корабль", "Автор Космос", "Отложил")

	r := setupAdminReadlistsRouter(db)

	// Filter by listname
	w := doJSON(t, r, "GET", "/api/v1/admin/readlists?listname=Книжный", nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total, "listname filter should match 2 items")

	// Filter by bookname
	w = doJSON(t, r, "GET", "/api/v1/admin/readlists?bookname=море", nil, adminToken)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)

	// Filter by author
	w = doJSON(t, r, "GET", "/api/v1/admin/readlists?author=Космос", nil, adminToken)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)

	// Filter by user_id
	w = doJSON(t, r, "GET", fmt.Sprintf("/api/v1/admin/readlists?user_ids=%d", childID), nil, adminToken)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 3, resp.Total)

	// Filter that matches nothing
	w = doJSON(t, r, "GET", "/api/v1/admin/readlists?listname=nonexistent_zz", nil, adminToken)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Total)
}

func TestAdminReadListsUpdateAndDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-ud-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)
	childID, _ := insertUserWithParent(t, db, adminID)

	itemID := insertReadListItemFor(t, db, childID, "default", "BookX", "AuthorX", "Читаю")

	r := setupAdminReadlistsRouter(db)

	// Non-parent stranger must not be able to update
	_, strangerToken := insertAdminUser(t, db)
	w := doJSON(t, r, "PUT", "/api/v1/admin/readlists/"+itemID, map[string]string{
		"status": "Прочитано",
	}, strangerToken)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	// Parent can update status
	w = doJSON(t, r, "PUT", "/api/v1/admin/readlists/"+itemID, map[string]string{
		"listname": "updated",
		"bookname": "BookY",
		"author":   "AuthorY",
		"status":   "Прочитано",
	}, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var updatedStatus string
	err := db.QueryRow("SELECT status::text FROM read_list WHERE id = $1::uuid", itemID).Scan(&updatedStatus)
	require.NoError(t, err)
	assert.Equal(t, "Прочитано", updatedStatus)

	var updatedListname string
	err = db.QueryRow("SELECT listname FROM read_list WHERE id = $1::uuid", itemID).Scan(&updatedListname)
	require.NoError(t, err)
	assert.Equal(t, "updated", updatedListname)

	// Parent can delete (soft)
	w = doJSON(t, r, "DELETE", "/api/v1/admin/readlists/"+itemID, nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var deleted bool
	err = db.QueryRow("SELECT deleted FROM read_list WHERE id = $1::uuid", itemID).Scan(&deleted)
	require.NoError(t, err)
	assert.True(t, deleted, "item should be soft-deleted")

	// After delete it no longer appears in list
	w = doJSON(t, r, "GET", "/api/v1/admin/readlists", nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Items []AdminReadListItem `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	for _, it := range resp.Items {
		assert.NotEqual(t, itemID, it.ID, "deleted item must not appear")
	}

	// Stranger can't delete either
	itemID2 := insertReadListItemFor(t, db, childID, "default", "BookZ", "AuthorZ", "Читаю")
	_, strangerToken2 := insertAdminUser(t, db)
	w = doJSON(t, r, "DELETE", "/api/v1/admin/readlists/"+itemID2, nil, strangerToken2)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestAdminChildrenList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-children-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)
	childID1, _ := insertUserWithParent(t, db, adminID)
	childID2, _ := insertUserWithParent(t, db, adminID)

	// A child of another parent must NOT appear
	otherAdminID, _ := insertAdminUser(t, db)
	insertUserWithParent(t, db, otherAdminID)

	r := setupAdminReadlistsRouter(db)

	w := doJSON(t, r, "GET", "/api/v1/admin/readlists/children", nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp struct {
		Items []AdminChild `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	ids := map[int]bool{}
	for _, ch := range resp.Items {
		ids[ch.ID] = true
	}
	assert.True(t, ids[childID1], "child1 should be listed")
	assert.True(t, ids[childID2], "child2 should be listed")
	assert.Len(t, resp.Items, 2, "only own children are listed")
}

func TestAdminReadListsCreateForChildren(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-create-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)
	childID1, _ := insertUserWithParent(t, db, adminID)
	childID2, _ := insertUserWithParent(t, db, adminID)
	strangerID, _ := insertAdminUser(t, db)

	r := setupAdminReadlistsRouter(db)

	// No children selected → 400
	w := doJSON(t, r, "POST", "/api/v1/admin/readlists", map[string]interface{}{
		"user_ids": []int{}, "listname": "L", "bookname": "B", "author": "A",
	}, adminToken)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	// No listname → 400
	w = doJSON(t, r, "POST", "/api/v1/admin/readlists", map[string]interface{}{
		"user_ids": []int{childID1}, "listname": "", "bookname": "B", "author": "A",
	}, adminToken)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	// Including a non-child user → 403
	w = doJSON(t, r, "POST", "/api/v1/admin/readlists", map[string]interface{}{
		"user_ids": []int{childID1, strangerID}, "listname": "L", "bookname": "B", "author": "A",
	}, adminToken)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// Valid: create for two children → 201 with two items
	uniqList := "Летнее чтение_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	w = doJSON(t, r, "POST", "/api/v1/admin/readlists", map[string]interface{}{
		"user_ids": []int{childID1, childID2}, "listname": uniqList, "bookname": "Книга",
		"author": "Автор", "status": "Читаю", "comment": "comment", "priority": 3,
	}, adminToken)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created struct {
		Items []ReadListItem `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Len(t, created.Items, 2)

	var cnt int
	err := db.QueryRow("SELECT COUNT(*) FROM read_list WHERE listname = $1 AND deleted = FALSE",
		uniqList).Scan(&cnt)
	require.NoError(t, err)
	assert.Equal(t, 2, cnt)
	t.Cleanup(func() { db.Exec("DELETE FROM read_list WHERE listname = $1", uniqList) })

	// The new items appear in the admin list, filterable by user_ids
	w = doJSON(t, r, "GET", fmt.Sprintf("/api/v1/admin/readlists?user_ids=%d", childID1), nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total, "user_ids filter should match child1's item only")
}

func TestAdminReadListsUserIDsFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-uid-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)
	childID1, _ := insertUserWithParent(t, db, adminID)
	childID2, _ := insertUserWithParent(t, db, adminID)

	insertReadListItemFor(t, db, childID1, "default", "Book1", "Author1", "Читаю")
	insertReadListItemFor(t, db, childID2, "default", "Book2", "Author2", "Читаю")

	r := setupAdminReadlistsRouter(db)

	var resp struct {
		Total int `json:"total"`
	}

	// Both children selected → both items
	w := doJSON(t, r, "GET", fmt.Sprintf("/api/v1/admin/readlists?user_ids=%d,%d", childID1, childID2), nil, adminToken)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)

	// Only child1 → one item
	w = doJSON(t, r, "GET", fmt.Sprintf("/api/v1/admin/readlists?user_ids=%d", childID1), nil, adminToken)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)

	// No filter → both items
	w = doJSON(t, r, "GET", "/api/v1/admin/readlists", nil, adminToken)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)
}
