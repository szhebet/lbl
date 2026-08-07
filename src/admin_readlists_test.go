package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
		admin.GET("/readlists/names", adminListReadListNames(db))
		admin.PUT("/readlists/:id", adminUpdateReadListItem(db))
		admin.DELETE("/readlists/:id", adminDeleteReadListItem(db))
		admin.POST("/readlists/bulk/shelf", adminBulkShelfReadLists(db))
		admin.POST("/readlists/bulk/delete", adminBulkDeleteReadLists(db))
		admin.POST("/readlists/bulk/status", adminBulkStatusReadLists(db))
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

func TestAdminReadListNamesUnion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-names-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)
	childID1, _ := insertUserWithParent(t, db, adminID)
	childID2, _ := insertUserWithParent(t, db, adminID)

	insertReadListItemFor(t, db, childID1, "Книжный список", "Book1", "Author1", "Читаю")
	insertReadListItemFor(t, db, childID1, "Книжный список", "Book2", "Author2", "Читаю")
	insertReadListItemFor(t, db, childID2, "Научная фантастика", "Book3", "Author3", "Отложил")

	// A child of another parent must NOT contribute list names
	otherAdminID, _ := insertAdminUser(t, db)
	otherChildID, _ := insertUserWithParent(t, db, otherAdminID)
	insertReadListItemFor(t, db, otherChildID, "Чужие списки", "Book4", "Author4", "Читаю")

	r := setupAdminReadlistsRouter(db)

	w := doJSON(t, r, "GET", "/api/v1/admin/readlists/names", nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp struct {
		Items []string `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	got := map[string]bool{}
	for _, n := range resp.Items {
		got[n] = true
	}
	assert.True(t, got["Книжный список"], "child1 list should be in union")
	assert.True(t, got["Научная фантастика"], "child2 list should be in union")
	assert.False(t, got["Чужие списки"], "other parent's child list must not leak")
	assert.Equal(t, 2, len(resp.Items))
}

func TestAdminReadListsListnamesFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-listnames-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)
	childID, _ := insertUserWithParent(t, db, adminID)

	insertReadListItemFor(t, db, childID, "Список А", "Book1", "Author1", "Читаю")
	insertReadListItemFor(t, db, childID, "Список Б", "Book2", "Author2", "Читаю")

	r := setupAdminReadlistsRouter(db)

	var resp struct {
		Total int `json:"total"`
	}

	// Both names → both items
	w := doJSON(t, r, "GET", "/api/v1/admin/readlists?listnames="+url.QueryEscape("Список А,Список Б"), nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)

	// Single name → one item
	w = doJSON(t, r, "GET", "/api/v1/admin/readlists?listnames="+url.QueryEscape("Список Б"), nil, adminToken)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)

	// Non-matching name → nothing
	w = doJSON(t, r, "GET", "/api/v1/admin/readlists?listnames="+url.QueryEscape("Нет такого"), nil, adminToken)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Total)
}

func TestAdminReadListsStatusesFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-statuses-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)
	childID, _ := insertUserWithParent(t, db, adminID)

	insertReadListItemFor(t, db, childID, "Список А", "Book1", "Author1", "Читаю")
	insertReadListItemFor(t, db, childID, "Список Б", "Book2", "Author2", "Прочитано")
	insertReadListItemFor(t, db, childID, "Список В", "Book3", "Author3", "Читаю")

	r := setupAdminReadlistsRouter(db)

	var resp struct {
		Total int `json:"total"`
	}

	// Two statuses → matching items only
	w := doJSON(t, r, "GET", "/api/v1/admin/readlists?statuses="+url.QueryEscape("Читаю,Прочитано"), nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 3, resp.Total)

	// Single status → only matching items
	w = doJSON(t, r, "GET", "/api/v1/admin/readlists?statuses="+url.QueryEscape("Читаю"), nil, adminToken)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)

	// Non-matching status → nothing
	w = doJSON(t, r, "GET", "/api/v1/admin/readlists?statuses="+url.QueryEscape("Бросил"), nil, adminToken)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Total)
}

func TestAdminReadListsBulkDeleteOwnOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-bulkdel-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)
	childID, _ := insertUserWithParent(t, db, adminID)

	r := setupAdminReadlistsRouter(db)

	// Entry created by the admin (created_by = admin)
	uniqMine := "Мои_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	w := doJSON(t, r, "POST", "/api/v1/admin/readlists", map[string]interface{}{
		"user_ids": []int{childID}, "listname": uniqMine, "bookname": "Книга 1",
		"author": "Автор 1", "status": "Читаю",
	}, adminToken)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	t.Cleanup(func() { db.Exec("DELETE FROM read_list WHERE listname = $1", uniqMine) })

	// Entry created by someone else (created_by = NULL via direct insert)
	uniqOther := "Чужие_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	insertReadListItemFor(t, db, childID, uniqOther, "Книга 2", "Автор 2", "Прочитано")

	// Bulk delete on a filter matching both → only the admin-created one dies
	w = doJSON(t, r, "POST", "/api/v1/admin/readlists/bulk/delete?listnames="+url.QueryEscape(uniqMine+","+uniqOther), nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Edited int `json:"edited"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, 1, res.Edited, "only own entries may be deleted")

	var mineDeleted, otherDeleted bool
	db.QueryRow(`SELECT deleted FROM read_list WHERE listname = $1`, uniqMine).Scan(&mineDeleted)
	db.QueryRow(`SELECT deleted FROM read_list WHERE listname = $1`, uniqOther).Scan(&otherDeleted)
	assert.True(t, mineDeleted, "own entry must be soft-deleted")
	assert.False(t, otherDeleted, "other user's entry must survive")
}

func TestAdminReadListsBulkStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-bulkstatus-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)
	childID, _ := insertUserWithParent(t, db, adminID)

	uniqList := "Массовый_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	insertReadListItemFor(t, db, childID, uniqList, "Книга 1", "Автор 1", "Читаю")
	insertReadListItemFor(t, db, childID, uniqList, "Книга 2", "Автор 2", "Читаю")
	// A different list that must NOT change
	otherList := "Другой_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	insertReadListItemFor(t, db, childID, otherList, "Книга 3", "Автор 3", "Читаю")

	r := setupAdminReadlistsRouter(db)

	w := doJSON(t, r, "POST", "/api/v1/admin/readlists/bulk/status?listnames="+url.QueryEscape(uniqList), map[string]interface{}{
		"status": "Прочитано",
	}, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Edited int `json:"edited"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, 2, res.Edited)

	var st1, st2, st3 string
	db.QueryRow(`SELECT status::text FROM read_list WHERE listname = $1 AND bookname = 'Книга 1'`, uniqList).Scan(&st1)
	db.QueryRow(`SELECT status::text FROM read_list WHERE listname = $1 AND bookname = 'Книга 2'`, uniqList).Scan(&st2)
	db.QueryRow(`SELECT status::text FROM read_list WHERE listname = $1 AND bookname = 'Книга 3'`, otherList).Scan(&st3)
	assert.Equal(t, "Прочитано", st1)
	assert.Equal(t, "Прочитано", st2)
	assert.Equal(t, "Читаю", st3, "entries outside the filter must be untouched")

	// Invalid status → 400
	w = doJSON(t, r, "POST", "/api/v1/admin/readlists/bulk/status", map[string]interface{}{
		"status": "Недопустимо",
	}, adminToken)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminReadListsBulkShelf(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-bulkshelf-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)
	childID, _ := insertUserWithParent(t, db, adminID)

	// Two real editions
	insertEditionFor := func(title string) int {
		var workID int
		err := db.QueryRow(`INSERT INTO works (original_title) VALUES ($1) RETURNING id`, title).Scan(&workID)
		require.NoError(t, err)
		t.Cleanup(func() { db.Exec("DELETE FROM works WHERE id = $1", workID) })
		var editionID int
		err = db.QueryRow(`INSERT INTO editions (work_id, title) VALUES ($1, $2) RETURNING id`, workID, title).Scan(&editionID)
		require.NoError(t, err)
		t.Cleanup(func() { db.Exec("DELETE FROM editions WHERE id = $1", editionID) })
		return editionID
	}
	ed1 := insertEditionFor("Полка Книга 1")
	ed2 := insertEditionFor("Полка Книга 2")

	uniqList := "Полочный_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	// Entry with book_id set
	var id1 string
	err := db.QueryRow(`
		INSERT INTO read_list (id, listname, bookname, author, priority, user_id, comment, status, book_id, updated_at)
		VALUES (gen_random_uuid(), $1, 'Книга 1', 'Автор 1', 5, $2, 'c', 'Читаю'::user_book_status, $3, NOW())
		RETURNING id::text
	`, uniqList, childID, ed1).Scan(&id1)
	require.NoError(t, err)
	t.Cleanup(func() { db.Exec("DELETE FROM read_list WHERE id = $1::uuid", id1) })
	// Entry WITHOUT book_id must not break the bulk shelf
	var id2 string
	err = db.QueryRow(`
		INSERT INTO read_list (id, listname, bookname, author, priority, user_id, comment, status, book_id, updated_at)
		VALUES (gen_random_uuid(), $1, 'Книга 2', 'Автор 2', 5, $2, 'c', 'Читаю'::user_book_status, $3, NOW())
		RETURNING id::text
	`, uniqList, childID, ed2).Scan(&id2)
	require.NoError(t, err)
	t.Cleanup(func() { db.Exec("DELETE FROM read_list WHERE id = $1::uuid", id2) })

	r := setupAdminReadlistsRouter(db)

	w := doJSON(t, r, "POST", "/api/v1/admin/readlists/bulk/shelf?listnames="+url.QueryEscape(uniqList), nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, 2, res.Total, "both linked editions must go on shelf")

	var on1, on2 bool
	db.QueryRow(`SELECT on_shelf FROM editions WHERE id = $1`, ed1).Scan(&on1)
	db.QueryRow(`SELECT on_shelf FROM editions WHERE id = $1`, ed2).Scan(&on2)
	assert.True(t, on1)
	assert.True(t, on2)
}

func TestAdminReadListsCreatedBy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-createdby-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)
	childID, _ := insertUserWithParent(t, db, adminID)

	// Create via admin endpoint → created_by = the parent admin
	uniqList := "Для создателя_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	r := setupAdminReadlistsRouter(db)
	w := doJSON(t, r, "POST", "/api/v1/admin/readlists", map[string]interface{}{
		"user_ids": []int{childID}, "listname": uniqList, "bookname": "Книга",
		"author": "Автор", "status": "Читаю",
	}, adminToken)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	t.Cleanup(func() { db.Exec("DELETE FROM read_list WHERE listname = $1", uniqList) })

	var createdBy int
	err := db.QueryRow("SELECT created_by FROM read_list WHERE listname = $1", uniqList).Scan(&createdBy)
	require.NoError(t, err)
	assert.Equal(t, adminID, createdBy, "creator must be the parent admin")

	// The list endpoint returns created_by + creator username
	var listResp struct {
		Items []AdminReadListItem `json:"items"`
	}
	w = doJSON(t, r, "GET", "/api/v1/admin/readlists?listname="+url.QueryEscape(uniqList), nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	require.Len(t, listResp.Items, 1)
	assert.Equal(t, adminID, listResp.Items[0].CreatedBy)
	assert.NotEmpty(t, listResp.Items[0].CreatedByU)
}

func TestAdminReadListsCreatedBySelfOnUserCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-createdby-self-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	// A child creates their own read-list item → created_by = the child itself
	adminID, _ := insertAdminUser(t, db)
	childID, childToken := insertUserWithParent(t, db, adminID)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})
	r.Use(authMiddleware())
	r.POST("/api/v1/user/readlist", createReadListItem(db))

	uniqList := "Сам себе_"+strconv.FormatInt(time.Now().UnixNano(), 36)
	w := doJSON(t, r, "POST", "/api/v1/user/readlist", map[string]interface{}{
		"listname": uniqList, "bookname": "Книга", "author": "Автор", "status": "Читаю",
	}, childToken)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	t.Cleanup(func() { db.Exec("DELETE FROM read_list WHERE listname = $1", uniqList) })

	var createdBy int
	err := db.QueryRow("SELECT created_by FROM read_list WHERE listname = $1", uniqList).Scan(&createdBy)
	require.NoError(t, err)
	assert.Equal(t, childID, createdBy, "creator must be the child itself")
}

func TestAdminReadListsBookLinkToChildren(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-arl-booklink-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	adminID, adminToken := insertAdminUser(t, db)
	childID1, _ := insertUserWithParent(t, db, adminID)
	childID2, _ := insertUserWithParent(t, db, adminID)

	// Matching entries among children of the admin
	itemA := insertReadListItemFor(t, db, childID1, "default", "Книга о море", "Иван Мореход", "Читаю")
	itemB := insertReadListItemFor(t, db, childID2, "default", "Книга о море", "Иван Мореход", "Читаю")
	// Non-matching entry (different bookname) must NOT be linked
	itemC := insertReadListItemFor(t, db, childID2, "default", "Другая книга", "Иван Мореход", "Читаю")

	r := setupAdminReadlistsRouter(db)

	// Valid person id for the author field (FK to persons)
	var personID int
	err := db.QueryRow(`INSERT INTO persons (first_name, last_name) VALUES ('Иван', 'Мореход') RETURNING id`).Scan(&personID)
	require.NoError(t, err)
	t.Cleanup(func() { db.Exec("DELETE FROM persons WHERE id = $1", personID) })

	// A real edition (book_id references editions(id))
	var workID int
	err = db.QueryRow(`INSERT INTO works (original_title) VALUES ('Книга о море') RETURNING id`).Scan(&workID)
	require.NoError(t, err)
	t.Cleanup(func() { db.Exec("DELETE FROM works WHERE id = $1", workID) })
	var editionID int
	err = db.QueryRow(`INSERT INTO editions (work_id, title) VALUES ($1, 'Книга о море') RETURNING id`, workID).Scan(&editionID)
	require.NoError(t, err)
	t.Cleanup(func() { db.Exec("DELETE FROM editions WHERE id = $1", editionID) })

	// Create a new item for child1 with book_id set (as if a book was loaded)
	uniqList := "С привязкой_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	w := doJSON(t, r, "POST", "/api/v1/admin/readlists", map[string]interface{}{
		"user_ids": []int{childID1}, "listname": uniqList, "bookname": "Книга о море",
		"author": "Иван Мореход", "status": "Читаю", "author_id": personID, "book_id": editionID,
	}, adminToken)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	t.Cleanup(func() { db.Exec("DELETE FROM read_list WHERE listname = $1", uniqList) })

	// book_id should be attached to matching entries, not to itemC
	var bookID sql.NullInt64
	err = db.QueryRow("SELECT book_id FROM read_list WHERE id = $1::uuid", itemA).Scan(&bookID)
	require.NoError(t, err)
	require.True(t, bookID.Valid)
	assert.Equal(t, int64(editionID), bookID.Int64)

	err = db.QueryRow("SELECT book_id FROM read_list WHERE id = $1::uuid", itemB).Scan(&bookID)
	require.NoError(t, err)
	require.True(t, bookID.Valid)
	assert.Equal(t, int64(editionID), bookID.Int64)

	err = db.QueryRow("SELECT book_id FROM read_list WHERE id = $1::uuid", itemC).Scan(&bookID)
	require.NoError(t, err)
	assert.False(t, bookID.Valid, "non-matching entry must not be linked")

	// Same on update: editing a matching entry links the book to all matches
	var updateResp struct {
		OK bool `json:"ok"`
	}
	w = doJSON(t, r, "PUT", "/api/v1/admin/readlists/"+itemA, map[string]interface{}{
		"listname": "default", "bookname": "Книга о море", "author": "Иван Мореход",
		"status": "Читаю", "book_id": editionID,
	}, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updateResp))

	err = db.QueryRow("SELECT book_id FROM read_list WHERE id = $1::uuid", itemB).Scan(&bookID)
	require.NoError(t, err)
	require.True(t, bookID.Valid)
	assert.Equal(t, int64(editionID), bookID.Int64)
}
