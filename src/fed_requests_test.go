package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"libapp/src/config"
)

// fedRequestsMock is a mock neighbouring server that records the pushed
// request batches it receives.
type fedRequestsMock struct {
	srv      *httptest.Server
	pushHits int32
	lastBatch fedPushBatch
}

func newFedRequestsMock() *fedRequestsMock {
	m := &fedRequestsMock{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		case "/api/v1/server/requests/push":
			if r.Header.Get("Authorization") != "Bearer peer-jwt" {
				http.Error(w, `{"error":"no auth"}`, http.StatusUnauthorized)
				return
			}
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &m.lastBatch)
			m.pushHits++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"received": len(m.lastBatch.Requests)})
		default:
			http.NotFound(w, r)
		}
	}))
	return m
}

func (m *fedRequestsMock) Close() { m.srv.Close() }

// insertNeighbourWithCrypto adds a neighbour row whose password is encrypted
// with the given crypto, returns its id.
func insertNeighbourWithCrypto(t *testing.T, db *sql.DB, nc *NeighbourCrypto, url string) int {
	t.Helper()
	encPass, err := nc.Encrypt("peerpass")
	require.NoError(t, err)
	var nid int
	err = db.QueryRow(`INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`, url, testSelfSignedCert(t), "", "peeruser", encPass).Scan(&nid)
	require.NoError(t, err)
	return nid
}

func TestFedReceiveRequestPush(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")
	db := setupTestDB()
	defer db.Close()

	// Server-role token.
	uname := "fed_server_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var uid int
	require.NoError(t, db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1,$2,'server') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&uid))
	defer db.Exec("DELETE FROM users WHERE id = $1", uid)
	token := generateToken(uid, uname, "server")

	// Clean any pre-existing incoming rows for the mock source.
	mockURL := "http://test-peer"
	defer db.Exec("DELETE FROM fed_incoming_requests WHERE source_url = $1", mockURL)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db); c.Next() })
	grp := r.Group("/api/v1/server")
	grp.Use(requireAuthMiddleware(), serverOnlyMiddleware())
	grp.POST("/requests/push", serverReceiveRequestPush(db))

	// Missing auth → 401.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/server/requests/push", bytes.NewReader([]byte(`{}`)))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Valid push → 200 with received count.
	body := `{"source_url":"http://test-peer","requests":[
		{"bookname":"Книга А","author":"Иванов","priority":5},
		{"bookname":"","author":"пустое имя"},
		{"bookname":"Книга Б","author":"Петров","priority":1}
	]}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/server/requests/push", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp struct{ Received int `json:"received"` }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Received) // empty bookname skipped

	// Duplicate push does not duplicate rows.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/server/requests/push", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var total int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM fed_incoming_requests WHERE source_url=$1`, mockURL).Scan(&total))
	assert.Equal(t, 2, total)

	// Wrong role (viewer) → 403.
	var vID int
	require.NoError(t, db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1,$2,'viewer') RETURNING id`,
		"fed_view_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&vID))
	defer db.Exec("DELETE FROM users WHERE id = $1", vID)
	vToken := generateToken(vID, "x", "viewer")
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/server/requests/push", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+vToken)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestFedReceiveRequestPushUidDedup verifies UID-based dedup on the receiving
// server: the same uid is stored once, and re-delivery is reported as
// "exists" so the sender can mark its outbox delivered and not resend.
func TestFedReceiveRequestPushUidDedup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")
	db := setupTestDB()
	defer db.Close()

	uname := "fed_uid_server_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var uid int
	require.NoError(t, db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1,$2,'server') RETURNING id`,
		uname, "$2a$10$dummyhash").Scan(&uid))
	defer db.Exec("DELETE FROM users WHERE id = $1", uid)
	token := generateToken(uid, uname, "server")

	dedupSrc := "http://uid-peer"
	defer db.Exec("DELETE FROM fed_incoming_requests WHERE source_url = $1", dedupSrc)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db); c.Next() })
	grp := r.Group("/api/v1/server")
	grp.Use(requireAuthMiddleware(), serverOnlyMiddleware())
	grp.POST("/requests/push", serverReceiveRequestPush(db))

	msgUID := "11111111-2222-3333-4444-555555555555"
	body := `{"source_url":"` + dedupSrc + `","requests":[
		{"uid":"` + msgUID + `","bookname":"Книга UID","author":"Автор","priority":4}
	]}`

	post := func() (int, int) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/server/requests/push", bytes.NewReader([]byte(body)))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var resp struct {
			Received int `json:"received"`
			Exists   int `json:"exists"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		return resp.Received, resp.Exists
	}

	// First push → newly stored.
	received, exists := post()
	assert.Equal(t, 1, received)
	assert.Equal(t, 0, exists)

	// Second push of the same uid → already present, not stored again.
	received, exists = post()
	assert.Equal(t, 0, received)
	assert.Equal(t, 1, exists)

	var total int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM fed_incoming_requests WHERE source_url=$1`, dedupSrc).Scan(&total))
	assert.Equal(t, 1, total)

	// The stored message carries the uid, type UUID (36 chars).
	var storedUID string
	require.NoError(t, db.QueryRow(`SELECT uid::text FROM fed_incoming_requests WHERE source_url=$1`, dedupSrc).Scan(&storedUID))
	assert.Equal(t, msgUID, storedUID)
}

// TestFedRequestsDistributorDelivers verifies a full background push pass: an
// admin-approved outgoing request is gathered, the outbox gets a pending row,
// the batch is pushed to the neighbouring server and the row flips to
// delivered.
func TestFedRequestsDistributorDelivers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")
	db := setupTestDB()
	defer db.Close()

	// Remove any admin-approved leftovers from an earlier run/env so only this
	// test's own request is eligible for distribution.
	db.Exec(`DELETE FROM fed_request_outbox`)
	db.Exec(`DELETE FROM fed_outgoing_requests`)

	// Neighbour for the calling server.
	backup := backupNeighbours(t, db)
	defer backup()

	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	mock := newFedRequestsMock()
	defer mock.Close()

	nid := insertNeighbourWithCrypto(t, db, nc, mock.srv.URL)
	defer db.Exec("DELETE FROM api_neighbours WHERE id = $1", nid)

	// A user + a read_list row that is a "book request" (looking_for != Нет).
	var uID int
	require.NoError(t, db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1,$2,'viewer') RETURNING id`,
		"fed_req_user_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&uID))
	defer db.Exec("DELETE FROM users WHERE id = $1", uID)
	var rlID string
	require.NoError(t, db.QueryRow(`
		INSERT INTO read_list (id, listname, bookname, author, priority, user_id, looking_for, deleted, status)
		VALUES (gen_random_uuid(), 'default', 'Запрашиваемая книга', 'Иванов И.', 3, $1, 'Да', FALSE, 'Читаю') RETURNING id`, uID).Scan(&rlID))
	defer db.Exec("DELETE FROM read_list WHERE id = $1", rlID)

	fedCfg := config.FederationConfig{Enabled: true, PushIntervalSec: 300, RetryIntervalSec: 1, RetryWindowSec: 10}
	dist := newFedRequestsDistributor(db, nc, &fedCfg, "")

	// Without admin approval the request must NOT be pushed.
	dist.Run()
	assert.Equal(t, int32(0), mock.pushHits, "unapproved request must not be distributed")

	// Admin approves the request → now it is distributed.
	_, err = db.Exec(`
		INSERT INTO fed_outgoing_requests (read_list_id, bookname, author, priority, status)
		VALUES ($1,'Запрашиваемая книга','Иванов И.',3,'approved') ON CONFLICT DO NOTHING`, rlID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM fed_outgoing_requests WHERE read_list_id=$1", rlID)

	dist.Run()

	require.Greater(t, int(mock.pushHits), 0, "expected at least one push")
	assert.Equal(t, "Запрашиваемая книга", mock.lastBatch.Requests[0].Bookname)
	assert.Equal(t, "Иванов И.", mock.lastBatch.Requests[0].Author)
	assert.Equal(t, mock.srv.URL, mock.lastBatch.SourceURL)

	var status string
	require.NoError(t, db.QueryRow(`
		SELECT status FROM fed_request_outbox WHERE neighbour_id=$1 AND bookname='Запрашиваемая книга'`,
		nid).Scan(&status))
	assert.Equal(t, "delivered", status)

	// Second run: no pending rows → no new push.
	mock.pushHits = 0
	dist.Run()
	assert.Equal(t, int32(0), mock.pushHits)
}

// TestFedRequestsDistributorRetryGivesUp verifies that an unreachable neighbour
// receives retries within the retry window and is then given up.
func TestFedRequestsDistributorRetryGivesUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")
	db := setupTestDB()
	defer db.Close()

	backup := backupNeighbours(t, db)
	defer backup()

	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	// Unreachable neighbour (closed server).
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := mock.URL
	mock.Close()

	nid := insertNeighbourWithCrypto(t, db, nc, url)
	defer db.Exec("DELETE FROM api_neighbours WHERE id = $1", nid)

	var uID int
	require.NoError(t, db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1,$2,'viewer') RETURNING id`,
		"fed_retry_user_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&uID))
	defer db.Exec("DELETE FROM users WHERE id = $1", uID)
	var rlID string
	require.NoError(t, db.QueryRow(`
		INSERT INTO read_list (id, listname, bookname, author, priority, user_id, looking_for, deleted, status)
		VALUES (gen_random_uuid(), 'default', 'Непередаваемая книга', 'Сидоров', 1, $1, 'Да', FALSE, 'Читаю') RETURNING id`, uID).Scan(&rlID))
	defer db.Exec("DELETE FROM read_list WHERE id = $1", rlID)

	// Keep the request "young" by backdating created_at (so the first attempt
	// is within the retry window) and use a short window.
	fedCfg := config.FederationConfig{Enabled: true, PushIntervalSec: 300, RetryIntervalSec: 0, RetryWindowSec: 1000}
	dist := newFedRequestsDistributor(db, nc, &fedCfg, "")

	// Admin approval is required before a request can be distributed.
	_, err = db.Exec(`
		INSERT INTO fed_outgoing_requests (read_list_id, bookname, author, priority, status)
		VALUES ($1,'Непередаваемая книга','Сидоров',1,'approved') ON CONFLICT DO NOTHING`, rlID)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM fed_outgoing_requests WHERE read_list_id=$1", rlID)

	// Force the created_at back so our retry-window arithmetic is deterministic.
	db.Exec(`UPDATE fed_request_outbox SET created_at = now() - interval '10 minutes' WHERE neighbour_id=$1`, nid)

	dist.Run()

	// The request was not delivered and no next_retry_at is scheduled once the
	// window is exceeded (elapsed 10min > window passed via first-run attempt is
	// still under window in real time, but our CreatedAt is backdated so need a
	// pass where the give-up fires). Use a window far below the elapsed time.
	db.Exec(`UPDATE fed_request_outbox SET created_at = now() - interval '2 hours' WHERE neighbour_id=$1`, nid)
	dist.Run()

	var status string
	var attempts int
	var nextRetry *time.Time
	require.NoError(t, db.QueryRow(`
		SELECT status, attempts, next_retry_at FROM fed_request_outbox WHERE neighbour_id=$1`,
		nid).Scan(&status, &attempts, &nextRetry))
	assert.Equal(t, "failed", status)
	assert.Nil(t, nextRetry, "given-up request must not keep a retry time")
	assert.GreaterOrEqual(t, attempts, 1)
}

// TestFedAdminListRequests verifies the admin-only listing endpoint.
func TestFedAdminListRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")
	db := setupTestDB()
	defer db.Close()

	var uid int
	require.NoError(t, db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1,$2,'admin') RETURNING id`,
		"fed_adm_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&uid))
	defer db.Exec("DELETE FROM users WHERE id = $1", uid)
	adminTok := generateToken(uid, "adm", "admin")

	src := "http://list-peer"
	_, err := db.Exec(`INSERT INTO fed_incoming_requests (source_url, bookname, author, priority)
		VALUES ($1,'Книга','Автор',2)`, src)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM fed_incoming_requests WHERE source_url=$1", src)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db); c.Next() })
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	admin.GET("/fed/requests", adminOnlyMiddleware(), adminFedListRequests(db))
	admin.POST("/fed/requests/:id/status", adminOnlyMiddleware(), adminFedSetRequestStatus(db))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/fed/requests", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/admin/fed/requests", nil)
	req.Header.Set("Authorization", "Bearer "+adminTok)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Items []struct {
			ID       int    `json:"id"`
			Source   string `json:"source_url"`
			Bookname string `json:"bookname"`
			Author   string `json:"author"`
			Status   string `json:"status"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	found := false
	for _, it := range resp.Items {
		if it.Source == src && it.Bookname == "Книга" {
			found = true
			assert.Equal(t, "new", it.Status)
			// Set status done via the endpoint.
			w2 := httptest.NewRecorder()
			req2, _ := http.NewRequest("POST", "/api/v1/admin/fed/requests/"+strconv.Itoa(it.ID)+"/status",
				bytes.NewReader([]byte(`{"status":"done"}`)))
			req2.Header.Set("Authorization", "Bearer "+adminTok)
			req2.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w2, req2)
			assert.Equal(t, http.StatusOK, w2.Code)
			break
		}
	}
	assert.True(t, found, "listed request not found")
}

// TestFedApproveOutgoing verifies the admin-approved outbound staging: a
// read_list request becomes outgoing only after an approve call, listing
// returns it, removal revokes it and cancels its outbox rows. An editor is
// rejected.
func TestFedApproveOutgoing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")
	db := setupTestDB()
	defer db.Close()

	backup := backupNeighbours(t, db)
	defer backup()

	var uID int
	require.NoError(t, db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1,$2,'viewer') RETURNING id`,
		"fed_appr_user_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&uID))
	defer db.Exec("DELETE FROM users WHERE id = $1", uID)
	var rlID string
	require.NoError(t, db.QueryRow(`
		INSERT INTO read_list (id, listname, bookname, author, priority, user_id, looking_for, deleted, status)
		VALUES (gen_random_uuid(), 'default', 'Утверждённая книга', 'Петров П.', 2, $1, 'Да', FALSE, 'Читаю') RETURNING id`, uID).Scan(&rlID))
	defer db.Exec("DELETE FROM read_list WHERE id = $1", rlID)
	defer db.Exec("DELETE FROM fed_outgoing_requests WHERE read_list_id=$1", rlID)

	// Admin + editor tokens.
	var admID int
	require.NoError(t, db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1,$2,'admin') RETURNING id`,
		"fed_appr_adm_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&admID))
	defer db.Exec("DELETE FROM users WHERE id = $1", admID)
	adminTok := generateToken(admID, "adm", "admin")

	var edID int
	require.NoError(t, db.QueryRow(`
		INSERT INTO users (username, password_hash, role) VALUES ($1,$2,'editor') RETURNING id`,
		"fed_appr_ed_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&edID))
	defer db.Exec("DELETE FROM users WHERE id = $1", edID)
	editorTok := generateToken(edID, "ed", "editor")

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db); c.Next() })
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	admin.GET("/fed/outgoing", adminOnlyMiddleware(), adminFedListOutgoing(db))
	admin.POST("/fed/outgoing", adminOnlyMiddleware(), adminFedApproveOutgoing(db))
	admin.DELETE("/fed/outgoing/:id", adminOnlyMiddleware(), adminFedRemoveOutgoing(db))

	// Editor → 403.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/fed/outgoing",
		bytes.NewReader([]byte(`{"read_list_id":"`+rlID+`"}`)))
	req.Header.Set("Authorization", "Bearer "+editorTok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Valid approval → 200 and a row appears.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/admin/fed/outgoing",
		bytes.NewReader([]byte(`{"read_list_id":"`+rlID+`"}`)))
	req.Header.Set("Authorization", "Bearer "+adminTok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var outID int
	require.NoError(t, db.QueryRow(`
		SELECT id FROM fed_outgoing_requests WHERE read_list_id=$1 AND status='approved'`, rlID).Scan(&outID))

	// Re-approval is idempotent (no new row).
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/admin/fed/outgoing",
		bytes.NewReader([]byte(`{"read_list_id":"`+rlID+`"}`)))
	req.Header.Set("Authorization", "Bearer "+adminTok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var cnt int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM fed_outgoing_requests WHERE read_list_id=$1 AND status='approved'`, rlID).Scan(&cnt))
	assert.Equal(t, 1, cnt)

	// List returns it.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/admin/fed/outgoing", nil)
	req.Header.Set("Authorization", "Bearer "+adminTok)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var lr struct {
		Items []struct {
			ID       int    `json:"id"`
			Bookname string `json:"bookname"`
			Status   string `json:"status"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &lr))
	found := false
	for _, it := range lr.Items {
		if it.Bookname == "Утверждённая книга" {
			found = true
			assert.Equal(t, "approved", it.Status)
		}
	}
	assert.True(t, found, "approved request not listed")

	// Non-existent read_list → 404.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/admin/fed/outgoing",
		bytes.NewReader([]byte(`{"read_list_id":"00000000-0000-0000-0000-000000000000"}`)))
	req.Header.Set("Authorization", "Bearer "+adminTok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Remove → status flipped to removed.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/admin/fed/outgoing/"+strconv.Itoa(outID), nil)
	req.Header.Set("Authorization", "Bearer "+adminTok)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var st string
	require.NoError(t, db.QueryRow(`SELECT status FROM fed_outgoing_requests WHERE id=$1`, outID).Scan(&st))
	assert.Equal(t, "removed", st)
}
// offerTestCfg returns a config that points bookarch/temp into fresh
// directories under the repo root (mirroring production, where stored file
// paths are resolved relative to the working directory) so offer tests never
// touch the real repository.
func offerTestCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.DefaultConfig()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	dirs := []string{".offer_bk_" + suffix, ".offer_tmp_" + suffix}
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(d, 0755))
		t.Cleanup(func() { os.RemoveAll(d) })
	}
	cfg.Directories.Bookarch = dirs[0]
	cfg.Directories.Temp = dirs[1]
	return cfg
}

// makeOfferBody builds a JSON reference offer for the receiver endpoint.
func makeOfferBody(sourceURL, uid, readListID string, meta fedBookMetadata) []byte {
	offer := serverOffer{SourceURL: sourceURL, UID: uid, ReadListID: readListID, EditionID: meta.Edition.ID, WorkID: meta.Work.ID, Metadata: meta}
	b, _ := json.Marshal(offer)
	return b
}

// insertOfferReadList creates a user + read_list row and an approved
// fed_outgoing_requests row with the given uid, returning the read_list id.
func insertOfferReadList(t *testing.T, db *sql.DB, uid string) string {
	t.Helper()
	var uID int
	require.NoError(t, db.QueryRow(`INSERT INTO users (username, password_hash, role)
		VALUES ($1,$2,'viewer') RETURNING id`,
		"offer_rl_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&uID))
	t.Cleanup(func() { db.Exec("DELETE FROM users WHERE id = $1", uID) })
	var rlID string
	require.NoError(t, db.QueryRow(`
		INSERT INTO read_list (id, listname, bookname, author, priority, user_id, looking_for, deleted, status)
		VALUES (gen_random_uuid(), 'offer', 'Запрос на книгу', 'Неизвестно', 1, $1, 'Да', FALSE, 'Читаю') RETURNING id::text`, uID).Scan(&rlID))
	t.Cleanup(func() { db.Exec("DELETE FROM read_list WHERE id = $1::uuid", rlID) })
	_, err := db.Exec(`
		INSERT INTO fed_outgoing_requests (read_list_id, bookname, author, priority, status, uid)
		VALUES ($1,'Запрос на книгу','Неизвестно',1,'approved',$2)`, rlID, uid)
	require.NoError(t, err)
	t.Cleanup(func() { db.Exec("DELETE FROM fed_outgoing_requests WHERE uid=$1::uuid", uid) })
	return rlID
}

// TestServerOfferBook verifies the receiving side of the reference-based offer:
// a book that is not present locally is pulled from the offering server and
// imported preserving the original author/work/edition ids, and the originating
// read_list row is linked; an offer whose ids are already local is just linked
// and reported as a duplicate (no error).
func TestServerOfferBook(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")
	db := setupTestDB()
	defer db.Close()

	// Remove any rows left over from an earlier partial run of this test.
	db.Exec(`DELETE FROM work_contributors WHERE work_id >= 2900000`)
	db.Exec(`DELETE FROM edition_files WHERE edition_id >= 2900000`)
	db.Exec(`DELETE FROM editions WHERE id >= 2900000`)
	db.Exec(`DELETE FROM works WHERE id >= 2900000`)
	db.Exec(`DELETE FROM persons WHERE id >= 2900000`)

	cfg := offerTestCfg(t)
	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	var srvID int
	require.NoError(t, db.QueryRow(`INSERT INTO users (username, password_hash, role)
		VALUES ($1,$2,'server') RETURNING id`,
		"offer_server_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&srvID))
	defer db.Exec("DELETE FROM users WHERE id = $1", srvID)
	token := generateToken(srvID, "offer_server", "server")

	const uidA = "aaaaaaa1-1111-1111-1111-111111111111"
	rlA := insertOfferReadList(t, db, uidA)

	// Offering server (mock neighbour) serving metadata + download.
	bookData := makeFB2Zip("Оффер-Приём", "АвторПриём", "ТестПриём")
	mock := newFedMockNeighbourMeta("Оффер-Приём", "АвторПриём ТестПриём", 2_920_002, 2_920_001, 2_920_003, bookData)
	defer mock.Close()
	encPass, _ := nc.Encrypt("peerpass")
	var nid int
	require.NoError(t, db.QueryRow(`INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
		VALUES ($1,'','','peeruser',$2) RETURNING id`, mock.srv.URL, encPass).Scan(&nid))
	defer db.Exec("DELETE FROM api_neighbours WHERE id=$1", nid)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db); c.Set("config", cfg); c.Set("user_id", srvID); c.Next() })
	grp := r.Group("/api/v1/server")
	grp.Use(requireAuthMiddleware(), serverOnlyMiddleware())
	grp.POST("/book/offer", serverOfferBook(db, nc))

	post := func(raw []byte) *httptest.ResponseRecorder {
		req, _ := http.NewRequest("POST", "/api/v1/server/book/offer", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// Case A: book not present locally → pulled and imported with original ids.
	meta := fedBookMetadata{
		Work:    fedWorkMeta{ID: 2_920_001, OriginalTitle: "Оффер-Приём", WorkType: "novel"},
		Edition: fedEditionMeta{ID: 2_920_002, WorkID: 2_920_001, Title: "Оффер-Приём", IsComplete: true},
		Authors: []fedAuthorMeta{{ID: 2_920_003, FirstName: "АвторПриём", LastName: "ТестПриём", Role: "author"}},
	}
	rec := post(makeOfferBody(mock.srv.URL, uidA, rlA, meta))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		OK         bool `json:"ok"`
		Duplicate  bool `json:"duplicate"`
		EditionID  int  `json:"edition_id"`
		WorkID     int  `json:"work_id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.OK)
	assert.False(t, resp.Duplicate)
	assert.Equal(t, 2_920_002, resp.EditionID)

	// Original ids preserved and contributor linked.
	var pid, wid int
	require.NoError(t, db.QueryRow(`SELECT id FROM persons WHERE id=2920003`).Scan(&pid))
	require.NoError(t, db.QueryRow(`SELECT id FROM works WHERE id=2920001 AND original_title='Оффер-Приём'`).Scan(&wid))
	assert.Equal(t, 2_920_003, pid)
	assert.Equal(t, 2_920_001, wid)
	var cnt int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM work_contributors WHERE work_id=2920001 AND person_id=2920003`).Scan(&cnt))
	assert.Equal(t, 1, cnt)
	// The metadata+download endpoints were hit once each.
	assert.EqualValues(t, 1, mock.metadataHits)
	assert.EqualValues(t, 1, mock.downloadHits)
	// The originating request was linked to the imported book.
	var bookIDA int64
	require.NoError(t, db.QueryRow(`SELECT book_id FROM read_list WHERE id=$1::uuid`, rlA).Scan(&bookIDA))
	assert.EqualValues(t, 2920002, bookIDA)
	// The approved outgoing request is marked fulfilled by the offering server.
	var fulfilledURL string
	var fulfilledAt sql.NullTime
	require.NoError(t, db.QueryRow(`SELECT fulfilled_by_url, fulfilled_at FROM fed_outgoing_requests WHERE read_list_id=$1::uuid`, rlA).Scan(&fulfilledURL, &fulfilledAt))
	assert.Equal(t, mock.srv.URL, fulfilledURL)
	assert.True(t, fulfilledAt.Valid, "fulfilled_at should be set")

	// Case B: offer whose content already exists locally → linked by hash,
	// duplicate, no download.
	db.Exec(`DELETE FROM edition_files WHERE edition_id >= 2900000`)
	db.Exec(`DELETE FROM editions WHERE id >= 2900000`)
	db.Exec(`DELETE FROM works WHERE id >= 2900000`)
	db.Exec(`DELETE FROM persons WHERE id >= 2900000`)
	const uidB = "bbbbbbb2-2222-2222-2222-222222222222"
	rlB := insertOfferReadList(t, db, uidB)
	existing := fedBookMetadata{
		Work:    fedWorkMeta{ID: 2_930_001, OriginalTitle: "Уже Есть", WorkType: "novel"},
		Edition: fedEditionMeta{ID: 2_930_002, WorkID: 2_930_001, Title: "Уже Есть", IsComplete: true},
		Authors: []fedAuthorMeta{{ID: 2_930_003, FirstName: "Ид", LastName: "ЕстьЛокально", Role: "author"}},
	}
	innerHash, formatName, formatID, err := fedAnalyzeBook(makeFB2Zip("Уже Есть", "Ид", "ЕстьЛокально"), db)
	require.NoError(t, err)
	_, _, _, err = fedCreateLocal(db, cfg, &existing, makeFB2Zip("Уже Есть", "Ид", "ЕстьЛокально"), innerHash, formatName, formatID, srvID, false)
	require.NoError(t, err)
	// The offer carries the content hash so the receiver dedups by identity.
	existing.Files = []fedFileMeta{{ID: 2_930_002, FileHash: innerHash}}

	rec2 := post(makeOfferBody(mock.srv.URL, uidB, rlB, existing))
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	var resp2 struct {
		OK        bool `json:"ok"`
		Duplicate bool `json:"duplicate"`
		EditionID int  `json:"edition_id"`
	}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp2))
	assert.True(t, resp2.OK)
	assert.True(t, resp2.Duplicate)
	assert.Equal(t, 2_930_002, resp2.EditionID)
	var bookIDB int64
	require.NoError(t, db.QueryRow(`SELECT book_id FROM read_list WHERE id=$1::uuid`, rlB).Scan(&bookIDB))
	assert.EqualValues(t, 2930002, bookIDB)
	// No additional download happened for the already-present book.
	assert.EqualValues(t, 1, mock.downloadHits)
}

// Case C (id-collision regression): the offered edition id already exists locally
// as a DIFFERENT book. The receiver must NOT link that wrong local edition just
// because the numeric id matches — it must pull and import the offered book, so
// the request is fulfilled with the real content.
func TestServerOfferBookEditionIDCollision(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")
	db := setupTestDB()
	defer db.Close()
	db.Exec(`DELETE FROM work_contributors WHERE work_id >= 2940000`)
	db.Exec(`DELETE FROM edition_files WHERE edition_id >= 2940000`)
	db.Exec(`DELETE FROM editions WHERE id >= 2940000`)
	db.Exec(`DELETE FROM works WHERE id >= 2940000`)
	db.Exec(`DELETE FROM persons WHERE id >= 2940000`)

	cfg := offerTestCfg(t)
	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)
	var srvID int
	require.NoError(t, db.QueryRow(`INSERT INTO users (username, password_hash, role)
		VALUES ($1,$2,'server') RETURNING id`,
		"offer_col_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&srvID))
	defer db.Exec("DELETE FROM users WHERE id = $1", srvID)
	token := generateToken(srvID, "offer_col", "server")
	const uidC = "ccccccc3-3333-3333-3333-333333333333"
	rlC := insertOfferReadList(t, db, uidC)

	// The offered book, served by the mock neighbour, wants edition id 2920002.
	bookData := makeFB2Zip("Оффер-Приём", "АвторПриём", "ТестПриём")
	offHash, formatName, formatID, err := fedAnalyzeBook(bookData, db)
	require.NoError(t, err)
	mock := newFedMockNeighbourMeta("Оффер-Приём", "АвторПриём ТестПриём", 2_920_002, 2_920_001, 2_920_003, bookData)
	defer mock.Close()
	encPass, _ := nc.Encrypt("peerpass")
	var nid int
	require.NoError(t, db.QueryRow(`INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
		VALUES ($1,'','','peeruser',$2) RETURNING id`, mock.srv.URL, encPass).Scan(&nid))
	defer db.Exec("DELETE FROM api_neighbours WHERE id=$1", nid)

	// Pre-create a DIFFERENT local book that already occupies edition id 2920002.
	localMeta := fedBookMetadata{
		Work:    fedWorkMeta{ID: 2_920_001, OriginalTitle: "Местный Захват", WorkType: "novel"},
		Edition: fedEditionMeta{ID: 2_920_002, WorkID: 2_920_001, Title: "Местный Захват", IsComplete: true},
		Authors: []fedAuthorMeta{{ID: 2_920_003, FirstName: "М", LastName: "Захватчик", Role: "author"}},
	}
	locHash, _, _, err := fedAnalyzeBook(makeFB2Zip("Местный Захват", "М", "Захватчик"), db)
	require.NoError(t, err)
	_, _, _, err = fedCreateLocal(db, cfg, &localMeta, makeFB2Zip("Местный Захват", "М", "Захватчик"), locHash, formatName, formatID, srvID, false)
	require.NoError(t, err)

	// Offer the remote book (edition id 2920002, but hash belongs to the offered
	// content, not to the local 2920002). The old buggy code would link the
	// local "Местный Захват"; the fixed code pulls and imports the real book.
	offerMeta := fedBookMetadata{
		Work:    fedWorkMeta{ID: 2_920_001, OriginalTitle: "Оффер-Приём", WorkType: "novel"},
		Edition: fedEditionMeta{ID: 2_920_002, WorkID: 2_920_001, Title: "Оффер-Приём", IsComplete: true},
		Authors: []fedAuthorMeta{{ID: 2_920_003, FirstName: "АвторПриём", LastName: "ТестПриём", Role: "author"}},
		Files:   []fedFileMeta{{ID: 2_920_002, FileHash: offHash}},
	}
	req, _ := http.NewRequest("POST", "/api/v1/server/book/offer", bytes.NewReader(makeOfferBody(mock.srv.URL, uidC, rlC, offerMeta)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db); c.Set("config", cfg); c.Set("user_id", srvID); c.Next() })
	r.POST("/api/v1/server/book/offer", serverOfferBook(db, nc))
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		OK        bool `json:"ok"`
		Duplicate bool `json:"duplicate"`
		EditionID int  `json:"edition_id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.OK)
	assert.False(t, resp.Duplicate)
	// The linked edition must NOT be the colliding local 2920002.
	assert.NotEqual(t, 2_920_002, resp.EditionID)
	var bookIDC int64
	require.NoError(t, db.QueryRow(`SELECT book_id FROM read_list WHERE id=$1::uuid`, rlC).Scan(&bookIDC))
	assert.NotEqualValues(t, 2_920_002, bookIDC)
	// The offered book was downloaded (pull path, not the id short-circuit).
	assert.EqualValues(t, 1, mock.downloadHits)
	// The local colliding book is untouched.
	var locTitle string
	require.NoError(t, db.QueryRow(`SELECT title FROM editions WHERE id=2920002`).Scan(&locTitle))
	assert.Equal(t, "Местный Захват", locTitle)
}
// and an incoming request sends a reference to the requesting neighbour so it
// can pull the book, and the incoming request is marked handled.
func TestAdminFederationOffer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")
	db := setupTestDB()
	defer db.Close()

	db.Exec(`DELETE FROM works WHERE id >= 2910000`)
	db.Exec(`DELETE FROM persons WHERE id >= 2910000`)

	cfg := offerTestCfg(t)
	backup := backupNeighbours(t, db)
	defer backup()
	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	// Mock neighbour that logs in and accepts a JSON reference offer.
	var received serverOffer
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
			json.NewEncoder(w).Encode(map[string]interface{}{"token": "peer-jwt", "user": map[string]interface{}{"username": "peeruser", "role": "server"}})
		case "/api/v1/server/book/offer":
			if r.Header.Get("Authorization") != "Bearer peer-jwt" {
				http.Error(w, `{"error":"no auth"}`, http.StatusUnauthorized)
				return
			}
			json.NewDecoder(r.Body).Decode(&received)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "work_id": received.Metadata.Work.ID, "edition_id": received.EditionID})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mock.Close()

	// Local book created with known ids (so the offer metadata carries them).
	meta := fedBookMetadata{
		Work:    fedWorkMeta{ID: 2_910_001, OriginalTitle: "Локальная книга", WorkType: "novel"},
		Edition: fedEditionMeta{ID: 2_910_002, WorkID: 2_910_001, Title: "Локальная книга", IsComplete: true},
		Authors: []fedAuthorMeta{{ID: 2_910_003, FirstName: "Иван", LastName: "Петров", Role: "author"}},
	}
	bookData := makeFB2Zip("Локальная книга", "Иван", "Петров")
	innerHash, formatName, formatID, err := fedAnalyzeBook(bookData, db)
	require.NoError(t, err)
	workID, editionID, _, err := fedCreateLocal(db, cfg, &meta, bookData, innerHash, formatName, formatID, 1, false)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM editions WHERE id=$1", editionID)
	defer db.Exec("DELETE FROM works WHERE id=$1", workID)
	defer db.Exec("DELETE FROM persons WHERE id=2910003")

	// Incoming request from the mock neighbour with a fixed uid and the
	// requester's stable read_list id.
	const uid = "ccccccc3-3333-3333-3333-333333333333"
	var uID int
	require.NoError(t, db.QueryRow(`INSERT INTO users (username, password_hash, role)
		VALUES ($1,$2,'viewer') RETURNING id`,
		"offer_in_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&uID))
	defer db.Exec("DELETE FROM users WHERE id=$1", uID)
	var rlID string
	require.NoError(t, db.QueryRow(`INSERT INTO read_list (id, listname, bookname, author, priority, user_id, looking_for, deleted, status)
		VALUES (gen_random_uuid(),'offer','Что-то','Кто-то',1,$1,'Да',FALSE,'Читаю') RETURNING id::text`, uID).Scan(&rlID))
	defer db.Exec("DELETE FROM read_list WHERE id=$1::uuid", rlID)
	_, err = db.Exec(`INSERT INTO fed_incoming_requests (source_url, bookname, author, priority, uid, read_list_id)
		VALUES ($1,'Что-то','Кто-то',1,$2,$3)`, mock.URL, uid, rlID)
	require.NoError(t, err)
	var inID int
	require.NoError(t, db.QueryRow(`SELECT id FROM fed_incoming_requests WHERE source_url=$1`, mock.URL).Scan(&inID))
	defer db.Exec("DELETE FROM fed_incoming_requests WHERE id=$1", inID)

	// Neighbour row pointing back at the source.
	encPass, err := nc.Encrypt("peerpass")
	require.NoError(t, err)
	var nid int
	require.NoError(t, db.QueryRow(`INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
		VALUES ($1,'','','peeruser',$2) RETURNING id`, mock.URL, encPass).Scan(&nid))
	defer db.Exec("DELETE FROM api_neighbours WHERE id=$1", nid)

	var admID int
	require.NoError(t, db.QueryRow(`INSERT INTO users (username, password_hash, role)
		VALUES ($1,$2,'admin') RETURNING id`,
		"offer_adm_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&admID))
	defer db.Exec("DELETE FROM users WHERE id=$1", admID)
	adminTok := generateToken(admID, "adm", "admin")

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db); c.Set("config", cfg); c.Set("user_id", admID); c.Next() })
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware())
	admin.POST("/federation/offer", adminOnlyMiddleware(), adminFederationOffer(db, nc))

	body, _ := json.Marshal(map[string]int{"incoming_request_id": inID, "edition_id": editionID})
	req, _ := http.NewRequest("POST", "/api/v1/admin/federation/offer", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminTok)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		OK         bool   `json:"ok"`
		EditionID  int    `json:"edition_id"`
		WorkID     int    `json:"work_id"`
		SourceURL  string `json:"source_url"`
		UID        string `json:"uid"`
		ReadListID string `json:"read_list_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)
	assert.Equal(t, editionID, resp.EditionID)
	assert.Equal(t, mock.URL, resp.SourceURL)
	assert.Equal(t, uid, resp.UID)
	assert.Equal(t, rlID, resp.ReadListID)

	// The neighbour received a reference (no file) with the right identifiers.
	assert.Equal(t, editionID, received.EditionID)
	assert.Equal(t, mock.URL, received.SourceURL)
	assert.Equal(t, uid, received.UID)
	assert.Equal(t, rlID, received.ReadListID)
	assert.Equal(t, 2910001, received.Metadata.Work.ID)
	assert.Equal(t, 2910002, received.Metadata.Edition.ID)

	// The incoming request is marked handled.
	var status string
	require.NoError(t, db.QueryRow(`SELECT status FROM fed_incoming_requests WHERE id=$1`, inID).Scan(&status))
	assert.Equal(t, "done", status)
}

// TestServerOfferBookUID verifies the cross-server identity match by uid, the
// independent stable identifier added in migration 5.5:
//   - U1: an offer whose edition uid already exists locally is fulfilled by
//     linking that existing edition (duplicate), with NO pull/download, even
//     though the offered numeric id and content differ from the local copy.
//   - U2: two editions that share the numeric id but carry DIFFERENT uids are
//     treated as distinct books — the receiver pulls the offered one instead of
//     linking the local collision victim.
func TestServerOfferBookUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")
	db := setupTestDB()
	defer db.Close()
	db.Exec(`DELETE FROM work_contributors WHERE work_id >= 2950000`)
	db.Exec(`DELETE FROM edition_files WHERE edition_id >= 2950000`)
	db.Exec(`DELETE FROM editions WHERE id >= 2950000`)
	db.Exec(`DELETE FROM works WHERE id >= 2950000`)
	db.Exec(`DELETE FROM persons WHERE id >= 2950000`)

	cfg := offerTestCfg(t)
	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)
	var srvID int
	require.NoError(t, db.QueryRow(`INSERT INTO users (username, password_hash, role)
		VALUES ($1,$2,'server') RETURNING id`,
		"offer_uid_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&srvID))
	defer db.Exec("DELETE FROM users WHERE id = $1", srvID)
	token := generateToken(srvID, "offer_uid", "server")

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db); c.Set("config", cfg); c.Set("user_id", srvID); c.Next() })
	r.POST("/api/v1/server/book/offer", serverOfferBook(db, nc))

	post := func(raw []byte) *httptest.ResponseRecorder {
		req, _ := http.NewRequest("POST", "/api/v1/server/book/offer", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// ---- U1: same edition uid already local → link, no pull ----
	const uidU1 = "aaab0001-0000-0000-0000-000000000001"
	rlU1 := insertOfferReadList(t, db, uidU1)

	localContent := makeFB2Zip("Локальная УИД", "Л", "Уид")
	locHash, formatName, formatID, err := fedAnalyzeBook(localContent, db)
	require.NoError(t, err)
	localMeta := fedBookMetadata{
		Work:    fedWorkMeta{ID: 2_950_001, UID: "bbbc0001-0000-0000-0000-000000000001", OriginalTitle: "Локальная УИД", WorkType: "novel"},
		Edition: fedEditionMeta{ID: 2_950_002, UID: "cccd0001-0000-0000-0000-000000000001", WorkID: 2_950_001, Title: "Локальная УИД", IsComplete: true},
		Authors: []fedAuthorMeta{{ID: 2_950_003, UID: "ddde0001-0000-0000-0000-000000000001", FirstName: "Л", LastName: "Уид", Role: "author"}},
	}
	_, localEdition, _, err := fedCreateLocal(db, cfg, &localMeta, localContent, locHash, formatName, formatID, srvID, false)
	require.NoError(t, err)
	defer db.Exec("DELETE FROM editions WHERE id=$1", localEdition)

	// The offer carries the SAME edition uid but a different numeric id and a
	// different (never-seen) content.
	offered := fedBookMetadata{
		Work:    fedWorkMeta{ID: 9_999_001, UID: "bbbc0001-0000-0000-0000-000000000001", OriginalTitle: "Удалённый УИД", WorkType: "novel"},
		Edition: fedEditionMeta{ID: 9_999_002, UID: "cccd0001-0000-0000-0000-000000000001", WorkID: 9_999_001, Title: "Удалённый УИД", IsComplete: true},
		Authors: []fedAuthorMeta{{ID: 9_999_003, UID: "ddde0001-0000-0000-0000-000000000001", FirstName: "У", LastName: "Уид", Role: "author"}},
	}
	rec := post(makeOfferBody("http://unused-host", uidU1, rlU1, offered))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		OK        bool `json:"ok"`
		Duplicate bool `json:"duplicate"`
		EditionID int  `json:"edition_id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.OK)
	assert.True(t, resp.Duplicate)
	// Linked to the EXISTING local edition by uid (not the offered numeric id).
	assert.Equal(t, localEdition, resp.EditionID)
	var bookU1 int64
	require.NoError(t, db.QueryRow(`SELECT book_id FROM read_list WHERE id=$1::uuid`, rlU1).Scan(&bookU1))
	assert.EqualValues(t, localEdition, bookU1)
	// No pull happened (the uid short-circuit short-circuits before download).
	assert.EqualValues(t, 0, countEditionFiles(db, 9_999_002))

	// ---- U2: same numeric edition id but DIFFERENT uid → distinct, pull ----
	const uidU2 = "aaab0002-0000-0000-0000-000000000002"
	rlU2 := insertOfferReadList(t, db, uidU2)

	bookData := makeFB2Zip("Оффер-УИД", "АвторУид", "ТестУид")
	mock := newFedMockNeighbourMeta("Оффер-УИД", "АвторУид ТестУид", 2_950_002, 2_950_001, 2_950_003, bookData)
	defer mock.Close()
	encPass, _ := nc.Encrypt("peerpass")
	var nid int
	require.NoError(t, db.QueryRow(`INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
		VALUES ($1,'','','peeruser',$2) RETURNING id`, mock.srv.URL, encPass).Scan(&nid))
	defer db.Exec("DELETE FROM api_neighbours WHERE id=$1", nid)

	// Offered edition has numeric id 2950002 (occupied locally by U1's book)
	// but a uid DIFFERENT from the existing one → must NOT link U1's edition.
	dup := fedBookMetadata{
		Work:    fedWorkMeta{ID: 2_950_001, UID: "bbbc0002-0000-0000-0000-000000000001", OriginalTitle: "Оффер-УИД", WorkType: "novel"},
		Edition: fedEditionMeta{ID: 2_950_002, UID: "55b81402-eb64-49f2-9c3a-9ac0a2c3f8e2", WorkID: 2_950_001, Title: "Оффер-УИД", IsComplete: true},
		Authors: []fedAuthorMeta{{ID: 2_950_003, UID: "66b81402-eb64-49f2-9c3a-9ac0a2c3f8e2", FirstName: "АвторУид", LastName: "ТестУид", Role: "author"}},
	}
	rec2 := post(makeOfferBody(mock.srv.URL, uidU2, rlU2, dup))
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	var resp2 struct {
		OK        bool `json:"ok"`
		Duplicate bool `json:"duplicate"`
		EditionID int  `json:"edition_id"`
	}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp2))
	assert.True(t, resp2.OK)
	assert.False(t, resp2.Duplicate)
	// Must NOT be the colliding local 2950002; the real book was pulled.
	assert.NotEqual(t, 2_950_002, resp2.EditionID)
	var bookU2 int64
	require.NoError(t, db.QueryRow(`SELECT book_id FROM read_list WHERE id=$1::uuid`, rlU2).Scan(&bookU2))
	assert.NotEqualValues(t, 2_950_002, bookU2)
	assert.EqualValues(t, 1, mock.downloadHits)
}

// countEditionFiles reports the number of files stored for an edition (helper
// used to detect whether an offer triggered an import).
func countEditionFiles(db *sql.DB, editionID int) int {
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM edition_files WHERE edition_id = $1`, editionID).Scan(&n)
	return n
}
