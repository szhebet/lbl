package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAdminUserRouter mirrors the admin /users route group from main.go
// using the real adminAuthMiddleware + adminOnlyMiddleware.
func setupAdminUserRouter(db *sql.DB) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	adminUsers := admin.Group("/users")
	adminUsers.Use(adminOnlyMiddleware())
	{
		adminUsers.GET("", adminGetUsers(db))
		adminUsers.GET("/:id", adminGetUser(db))
		adminUsers.POST("", adminCreateUser(db))
		adminUsers.PUT("/:id", adminUpdateUser(db))
		adminUsers.DELETE("/:id", adminDeleteUser(db))
	}
	return r
}

// insertAdminUser inserts an admin user directly and returns id + token.
func insertAdminUser(t *testing.T, db *sql.DB) (int, string) {
	t.Helper()
	var adminID int
	uname := "pa_admin_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	err := db.QueryRow(
		`INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'admin') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&adminID)
	require.NoError(t, err)
	t.Cleanup(func() { db.Exec("DELETE FROM users WHERE id = $1", adminID) })
	return adminID, generateToken(adminID, uname, "admin")
}

func doJSON(t *testing.T, r http.Handler, method, url string, body interface{}, token string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAdminUserParentChildCreateAndList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-parent-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	r := setupAdminUserRouter(db)

	_, token := insertAdminUser(t, db)

	// Create parent and child users via API
	parentID, _ := createUserViaAPI(t, r, db, token, "pa_parent")
	childID, _ := createUserViaAPI(t, r, db, token, "pa_child")

	// Create user "middle" with parent=parentID and child=childID
	createBody := map[string]interface{}{
		"username":   "pa_middle_" + strconv.FormatInt(time.Now().UnixNano(), 36),
		"password":   "secret123",
		"role":       "viewer",
		"parent_ids": []int{parentID},
		"child_ids":  []int{childID},
	}
	w := doJSON(t, r, "POST", "/api/v1/admin/users", createBody, token)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created AdminUser
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, []int{parentID}, created.ParentIDs)
	assert.Equal(t, []int{childID}, created.ChildIDs)
	defer db.Exec("DELETE FROM users WHERE id = $1", created.ID)

	// GET /users must include parent_names for the middle user
	w = doJSON(t, r, "GET", "/api/v1/admin/users", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	var users []AdminUser
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &users))
	var middle AdminUser
	for _, u := range users {
		if u.ID == created.ID {
			middle = u
			break
		}
	}
	assert.NotZero(t, middle.ID, "middle user should be in the list")
	assert.Contains(t, middle.ParentNames, "pa_parent_")

	// GET /users/:id returns both parent_ids and child_ids
	w = doJSON(t, r, "GET", "/api/v1/admin/users/"+strconv.Itoa(created.ID), nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	var got AdminUser
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, []int{parentID}, got.ParentIDs)
	assert.Equal(t, []int{childID}, got.ChildIDs)
}

func createUserViaAPI(t *testing.T, r http.Handler, db *sql.DB, token, prefix string) (int, *httptest.ResponseRecorder) {
	t.Helper()
	body := map[string]string{
		"username": prefix + "_" + strconv.FormatInt(time.Now().UnixNano(), 36),
		"password": "secret123",
		"role":     "viewer",
	}
	w := doJSON(t, r, "POST", "/api/v1/admin/users", body, token)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var u AdminUser
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &u))
	t.Cleanup(func() { db.Exec("DELETE FROM users WHERE id = $1", u.ID) })
	return u.ID, w
}

func TestAdminUserParentChildUpdateAndCycles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-parent-update-secret")

	db := setupTestDB()
	t.Cleanup(func() { db.Close() })

	r := setupAdminUserRouter(db)

	_, token := insertAdminUser(t, db)

	// Create three users
	idA, _ := createUserViaAPI(t, r, db, token, "pa_a")
	idB, _ := createUserViaAPI(t, r, db, token, "pa_b")
	idC, _ := createUserViaAPI(t, r, db, token, "pa_c")

	// Set A's parents to [B] and children to [C]
	upd := map[string]interface{}{
		"parent_ids": []int{idB},
		"child_ids":  []int{idC},
	}
	w := doJSON(t, r, "PUT", "/api/v1/admin/users/"+strconv.Itoa(idA), upd, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var uA AdminUser
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &uA))
	assert.Equal(t, []int{idB}, uA.ParentIDs)
	assert.Equal(t, []int{idC}, uA.ChildIDs)

	// B should now list A as a child, C should list A as a parent
	w = doJSON(t, r, "GET", "/api/v1/admin/users/"+strconv.Itoa(idB), nil, token)
	var uB AdminUser
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &uB))
	assert.Equal(t, []int{idA}, uB.ChildIDs)

	w = doJSON(t, r, "GET", "/api/v1/admin/users/"+strconv.Itoa(idC), nil, token)
	var uC AdminUser
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &uC))
	assert.Equal(t, []int{idA}, uC.ParentIDs)

	// Replace relations: A parents = [C], children = [B] (reverse cycle A→C→A is allowed)
	upd2 := map[string]interface{}{
		"parent_ids": []int{idC},
		"child_ids":  []int{idB},
	}
	w = doJSON(t, r, "PUT", "/api/v1/admin/users/"+strconv.Itoa(idA), upd2, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	uA = AdminUser{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &uA))
	assert.Equal(t, []int{idC}, uA.ParentIDs)
	assert.Equal(t, []int{idB}, uA.ChildIDs)

	// Old relation (B as parent) must be gone
	w = doJSON(t, r, "GET", "/api/v1/admin/users/"+strconv.Itoa(idB), nil, token)
	uB = AdminUser{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &uB))
	assert.Empty(t, uB.ChildIDs, "B should have no children after relation replaced")

	// Partial update: only change parents, children must be preserved
	upd3 := map[string]interface{}{
		"parent_ids": []int{idB},
	}
	w = doJSON(t, r, "PUT", "/api/v1/admin/users/"+strconv.Itoa(idA), upd3, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	uA = AdminUser{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &uA))
	assert.Equal(t, []int{idB}, uA.ParentIDs)
	assert.Equal(t, []int{idB}, uA.ChildIDs, "children preserved when only parents sent")

	// Self-reference allowed: A is its own parent
	upd4 := map[string]interface{}{
		"parent_ids": []int{idA},
	}
	w = doJSON(t, r, "PUT", "/api/v1/admin/users/"+strconv.Itoa(idA), upd4, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	uA = AdminUser{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &uA))
	assert.Equal(t, []int{idA}, uA.ParentIDs, "self-reference as own parent allowed")
}
