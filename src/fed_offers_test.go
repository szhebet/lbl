package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"libapp/src/config"
)

// insertOfferRLWithUser is like insertOfferReadList but also returns the id of
// the owning viewer user (needed for the user-facing offers API).
func insertOfferRLWithUser(t *testing.T, db *sql.DB, uid string) (string, int) {
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
	return rlID, uID
}

// TestServerOfferFirstWinsAndOffersAPI verifies the multi-offer scenario:
// several servers offer DIFFERENT books for one request — only the FIRST offer
// is linked to the read_list record, later offers are just imported into the
// library; the owner sees all offers with received timestamps and can link a
// different one manually.
func TestServerOfferFirstWinsAndOffersAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")
	db := setupTestDB()
	var rlA string
	var ownerID int
	var srvID int
	// Cleanup must be tied to db.Close(): a bare t.Cleanup would run after the
	// pool is closed and silently leak rows.
	defer func() {
		cleanupOfferFixture(db, rlA, ownerID, "aaaaaaa9-1111-1111-1111-111111111111", true)
		if srvID > 0 {
			db.Exec("DELETE FROM users WHERE id = $1", srvID)
		}
		db.Close()
	}()

	db.Exec(`DELETE FROM edition_files WHERE edition_id >= 2950000`)
	db.Exec(`DELETE FROM editions WHERE id >= 2950000`)
	db.Exec(`DELETE FROM works WHERE id >= 2950000`)
	db.Exec(`DELETE FROM persons WHERE id >= 2950000`)

	cfg := offerTestCfg(t)
	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	require.NoError(t, db.QueryRow(`INSERT INTO users (username, password_hash, role)
		VALUES ($1,$2,'server') RETURNING id`,
		"offer_server_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&srvID))
	serverToken := generateToken(srvID, "offer_server", "server")

	const uidA = "aaaaaaa9-1111-1111-1111-111111111111"
	rlA, ownerID = insertOfferRLWithUser(t, db, uidA)
	ownerToken := generateToken(ownerID, "offer_owner", "viewer")

	// Two neighbours offering different books.
	data1 := makeFB2Zip("Оффер-Первый", "АвторПервый", "ТестПервый")
	mock1 := newFedMockNeighbourMeta("Оффер-Первый", "АвторПервый ТестПервый", 2_950_001, 2_950_002, 2_950_003, data1)
	defer mock1.Close()
	data2 := makeFB2Zip("Оффер-Второй", "АвторВторой", "ТестВторой")
	mock2 := newFedMockNeighbourMeta("Оффер-Второй", "АвторВторой ТестВторой", 2_950_101, 2_950_102, 2_950_103, data2)
	defer mock2.Close()

	encPass, _ := nc.Encrypt("peerpass")
	insertN := func(url string) int {
		var nid int
		require.NoError(t, db.QueryRow(`INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
			VALUES ($1,'','',$3,$2) RETURNING id`, url, encPass, "peeruser").Scan(&nid))
		return nid
	}
	nid1 := insertN(mock1.srv.URL)
	defer db.Exec("DELETE FROM api_neighbours WHERE id=$1", nid1)
	nid2 := insertN(mock2.srv.URL)
	defer db.Exec("DELETE FROM api_neighbours WHERE id=$1", nid2)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db); c.Set("config", cfg); c.Set("user_id", srvID); c.Next() })
	grp := r.Group("/api/v1/server")
	grp.Use(requireAuthMiddleware(), serverOnlyMiddleware())
	grp.POST("/book/offer", serverOfferBook(db, nc))

	post := func(raw []byte) *httptest.ResponseRecorder {
		req, _ := http.NewRequest("POST", "/api/v1/server/book/offer", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+serverToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	meta1 := fedBookMetadata{
		Work:    fedWorkMeta{ID: 2_950_002, OriginalTitle: "Оффер-Первый", WorkType: "novel"},
		Edition: fedEditionMeta{ID: 2_950_001, WorkID: 2_950_002, Title: "Оффер-Первый", IsComplete: true},
		Authors: []fedAuthorMeta{{ID: 2_950_003, FirstName: "АвторПервый", LastName: "ТестПервый", Role: "author"}},
	}
	rec := post(makeOfferBody(mock1.srv.URL, uidA, rlA, meta1))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp1 struct {
		OK     bool `json:"ok"`
		Linked bool `json:"linked"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp1))
	assert.True(t, resp1.OK)
	assert.True(t, resp1.Linked, "the first offer must be linked to the request")

	var bookID int64
	require.NoError(t, db.QueryRow(`SELECT book_id FROM read_list WHERE id=$1::uuid`, rlA).Scan(&bookID))
	assert.EqualValues(t, 2_950_001, bookID, "first offer's edition must be linked")
	var fulfilledURL string
	require.NoError(t, db.QueryRow(
		`SELECT fulfilled_by_url FROM fed_outgoing_requests WHERE read_list_id=$1::uuid AND fulfilled_at IS NOT NULL`, rlA).Scan(&fulfilledURL))
	assert.Equal(t, mock1.srv.URL, fulfilledURL)

	// Second server offers a DIFFERENT book → imported but NOT linked.
	meta2 := fedBookMetadata{
		Work:    fedWorkMeta{ID: 2_950_102, OriginalTitle: "Оффер-Второй", WorkType: "novel"},
		Edition: fedEditionMeta{ID: 2_950_101, WorkID: 2_950_102, Title: "Оффер-Второй", IsComplete: true},
		Authors: []fedAuthorMeta{{ID: 2_950_103, FirstName: "АвторВторой", LastName: "ТестВторой", Role: "author"}},
	}
	rec2 := post(makeOfferBody(mock2.srv.URL, uidA, rlA, meta2))
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	var resp2 struct {
		OK     bool `json:"ok"`
		Linked bool `json:"linked"`
	}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp2))
	assert.True(t, resp2.OK)
	assert.False(t, resp2.Linked, "a later offer must not relink the request")

	// The second book IS in the library now.
	var eid2 int
	require.NoError(t, db.QueryRow(`SELECT id FROM editions WHERE id=2950101`).Scan(&eid2))
	assert.Equal(t, 2_950_101, eid2)
	// …but the link still points at the first offer.
	require.NoError(t, db.QueryRow(`SELECT book_id FROM read_list WHERE id=$1::uuid`, rlA).Scan(&bookID))
	assert.EqualValues(t, 2_950_001, bookID)
	// fulfilled marker still points at the first responding server.
	require.NoError(t, db.QueryRow(
		`SELECT fulfilled_by_url FROM fed_outgoing_requests WHERE read_list_id=$1::uuid AND fulfilled_at IS NOT NULL`, rlA).Scan(&fulfilledURL))
	assert.Equal(t, mock1.srv.URL, fulfilledURL)

	// ─── User-facing offers API ───
	ro := gin.New()
	ro.Use(func(c *gin.Context) { c.Set("db", db); c.Next() })
	og := ro.Group("/api/v1/user/readlist")
	og.Use(authMiddleware())
	{
		og.GET("/:id/offers", getReadListOffers(db))
		og.POST("/:id/offers/link", linkReadListOffer(db))
	}

	getOffers := func(token string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest("GET", "/api/v1/user/readlist/"+rlA+"/offers", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		ro.ServeHTTP(rec, req)
		return rec
	}

	// A stranger cannot see the offers.
	assert.Equal(t, http.StatusNotFound, getOffers(serverToken).Code)

	rec3 := getOffers(ownerToken)
	require.Equal(t, http.StatusOK, rec3.Code, rec3.Body.String())
	var list struct {
		Items []fedOfferItem `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &list))
	require.Len(t, list.Items, 2)
	// Linked first, then the other; both carry received timestamps.
	assert.True(t, list.Items[0].Linked)
	assert.False(t, list.Items[1].Linked)
	assert.Equal(t, mock1.srv.URL, list.Items[0].SourceURL)
	assert.NotEmpty(t, list.Items[0].ReceivedAt)
	assert.NotEmpty(t, list.Items[1].ReceivedAt)
	assert.NotNil(t, list.Items[1].EditionID)
	assert.Equal(t, 2_950_101, *list.Items[1].EditionID)
	otherOfferID := list.Items[1].ID

	// The owner picks the second offer → it becomes the linked book.
	linkBody, _ := json.Marshal(map[string]interface{}{"offer_id": otherOfferID})
	reqLink, _ := http.NewRequest("POST", "/api/v1/user/readlist/"+rlA+"/offers/link", bytes.NewReader(linkBody))
	reqLink.Header.Set("Content-Type", "application/json")
	reqLink.Header.Set("Authorization", "Bearer "+ownerToken)
	rec4 := httptest.NewRecorder()
	ro.ServeHTTP(rec4, reqLink)
	require.Equal(t, http.StatusOK, rec4.Code, rec4.Body.String())

	require.NoError(t, db.QueryRow(`SELECT book_id FROM read_list WHERE id=$1::uuid`, rlA).Scan(&bookID))
	assert.EqualValues(t, 2_950_101, bookID, "user-chosen offer must be linked")
	require.NoError(t, db.QueryRow(
		`SELECT fulfilled_by_url FROM fed_outgoing_requests WHERE read_list_id=$1::uuid AND fulfilled_at IS NOT NULL`, rlA).Scan(&fulfilledURL))
	assert.Equal(t, mock2.srv.URL, fulfilledURL, "fulfilled marker follows the chosen offer")

	rec5 := getOffers(ownerToken)
	require.Equal(t, http.StatusOK, rec5.Code)
	require.NoError(t, json.Unmarshal(rec5.Body.Bytes(), &list))
	require.Len(t, list.Items, 2)
	for _, it := range list.Items {
		if it.ID == otherOfferID {
			assert.True(t, it.Linked)
		} else {
			assert.False(t, it.Linked)
		}
	}
}

// cleanupOfferFixture removes everything the offer fixtures created. It must
// be called EXPLICITLY at the end of a test (t.Cleanup alone is not enough:
// deferred db.Close() runs before t.Cleanup callbacks, so they would fail
// silently on a closed pool).
func cleanupOfferFixture(db *sql.DB, rlID string, userID int, uid string, contentRange bool) {
	clean := func(query string, args ...interface{}) {
		if _, err := db.Exec(query, args...); err != nil {
			println("CLEANUP FAILED:", query, "->", err.Error())
		}
	}
	if contentRange {
		clean(`DELETE FROM work_contributors WHERE work_id >= 2950000`)
		clean(`DELETE FROM edition_files WHERE edition_id >= 2950000`)
		clean(`DELETE FROM editions WHERE id >= 2950000`)
		clean(`DELETE FROM works WHERE id >= 2950000`)
		clean(`DELETE FROM persons WHERE id >= 2950000`)
	}
	clean(`DELETE FROM fed_request_outbox WHERE uid=$1::uuid`, uid)
	clean(`DELETE FROM fed_outgoing_requests WHERE uid=$1::uuid`, uid)
	clean(`DELETE FROM fed_offers WHERE read_list_id=$1::uuid`, rlID)
	clean(`DELETE FROM read_list WHERE id = $1::uuid`, rlID)
	if userID > 0 {
		clean(`DELETE FROM users WHERE id = $1`, userID)
	}
}

// TestLinkReadListOfferBodyFormats verifies the user-facing link endpoint
// accepts the offer id as a JSON number AND as a numeric string (DOM dataset
// values are strings — the browser used to send {"offer_id":"2"} and got a
// 400), rejects non-numeric ids and offers without a downloaded book.
func TestLinkReadListOfferBodyFormats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")
	db := setupTestDB()
	var rlD string
	var ownerID int
	defer func() {
		cleanupOfferFixture(db, rlD, ownerID, "ddddddd6-4444-4444-4444-444444444444", true)
		db.Close()
	}()

	db.Exec(`DELETE FROM edition_files WHERE edition_id >= 2950000`)
	db.Exec(`DELETE FROM editions WHERE id >= 2950000`)
	db.Exec(`DELETE FROM works WHERE id >= 2950000`)

	const uidD = "ddddddd6-4444-4444-4444-444444444444"
	rlD, ownerID = insertOfferRLWithUser(t, db, uidD)
	ownerToken := generateToken(ownerID, "offer_owner", "viewer")

	// Two real editions to reference from the offers.
	var wid, eidA, eidB int64
	require.NoError(t, db.QueryRow(`INSERT INTO works (id, original_title) VALUES (2950001,'Тест-Работа') RETURNING id`).Scan(&wid))
	t.Cleanup(func() { db.Exec(`DELETE FROM works WHERE id=2950001`) })
	require.NoError(t, db.QueryRow(`INSERT INTO editions (id, work_id, title) VALUES (2950002,2950001,'Тест-Издание А') RETURNING id`).Scan(&eidA))
	require.NoError(t, db.QueryRow(`INSERT INTO editions (id, work_id, title) VALUES (2950003,2950001,'Тест-Издание Б') RETURNING id`).Scan(&eidB))
	t.Cleanup(func() { db.Exec(`DELETE FROM editions WHERE id IN (2950002,2950003)`) })

	// Three offers: one without a downloaded book, two with local editions.
	_, err := db.Exec(`
		INSERT INTO fed_offers (read_list_id, source_url, remote_work_id, remote_edition_id,
			local_edition_id, title, authors, linked)
		VALUES ($1::uuid,'https://srv-a',1,11,NULL,'Без файла','Автор',FALSE),
		       ($1::uuid,'https://srv-b',2,22,$2,'Книга А','Автор',TRUE),
		       ($1::uuid,'https://srv-c',3,33,$3,'Книга Б','Автор',FALSE)`,
		rlD, eidA, eidB)
	require.NoError(t, err)

	ro := gin.New()
	ro.Use(func(c *gin.Context) { c.Set("db", db); c.Next() })
	og := ro.Group("/api/v1/user/readlist")
	og.Use(authMiddleware())
	{
		og.POST("/:id/offers/link", linkReadListOffer(db))
	}

	link := func(body string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest("POST", "/api/v1/user/readlist/"+rlD+"/offers/link", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+ownerToken)
		rec := httptest.NewRecorder()
		ro.ServeHTTP(rec, req)
		return rec
	}
	var bookID int64

	// Actual offer ids scoped to THIS run's read_list (older runs may have
	// left rows with the same remote_edition_id values behind).
	var idNoFile, idA, idB int64
	require.NoError(t, db.QueryRow(`SELECT id FROM fed_offers WHERE remote_edition_id=11 AND read_list_id=$1::uuid`, rlD).Scan(&idNoFile))
	require.NoError(t, db.QueryRow(`SELECT id FROM fed_offers WHERE remote_edition_id=22 AND read_list_id=$1::uuid`, rlD).Scan(&idA))
	require.NoError(t, db.QueryRow(`SELECT id FROM fed_offers WHERE remote_edition_id=33 AND read_list_id=$1::uuid`, rlD).Scan(&idB))

	// String id (as sent by the SPA when the value comes from a DOM dataset):
	// previously rejected with 400, must now link Книга А.
	rec := link(`{"offer_id":"` + strconv.FormatInt(idA, 10) + `"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, db.QueryRow(`SELECT book_id FROM read_list WHERE id=$1::uuid`, rlD).Scan(&bookID))
	assert.EqualValues(t, eidA, bookID)

	// Numeric id: links Книга Б.
	rec = link(`{"offer_id":` + strconv.FormatInt(idB, 10) + `}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, db.QueryRow(`SELECT book_id FROM read_list WHERE id=$1::uuid`, rlD).Scan(&bookID))
	assert.EqualValues(t, eidB, bookID)

	var linkedID int
	require.NoError(t, db.QueryRow(
		`SELECT id FROM fed_offers WHERE read_list_id=$1::uuid AND linked`, rlD).Scan(&linkedID))
	var wantID int64
	require.NoError(t, db.QueryRow(`SELECT id FROM fed_offers WHERE remote_edition_id=33`).Scan(&wantID))
	assert.EqualValues(t, wantID, linkedID, "only the chosen offer stays linked")
	// fulfilled marker follows the chosen server.
	var fulfilledURL string
	require.NoError(t, db.QueryRow(
		`SELECT fulfilled_by_url FROM fed_outgoing_requests WHERE read_list_id=$1::uuid AND fulfilled_at IS NOT NULL`, rlD).Scan(&fulfilledURL))
	assert.Equal(t, "https://srv-c", fulfilledURL)

	// Non-numeric id → 400.
	assert.Equal(t, http.StatusBadRequest, link(`{"offer_id":"abc"}`).Code)
	// Offer without a downloaded book → 400.
	assert.Equal(t, http.StatusBadRequest, link(`{"offer_id":`+strconv.FormatInt(idNoFile, 10)+`}`).Code)

}

// TestFedDeliveryCancelledAfterOffer verifies requirement: once ANY server has
// responded with a book offer, further distribution of that request stops —
// pending outbox rows for the remaining neighbours are cancelled and are not
// recreated by subsequent distribution passes.
func TestFedDeliveryCancelledAfterOffer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret("test-secret")
	db := setupTestDB()
	var rlC string
	var ownerID int
	var srvID int
	defer func() {
		cleanupOfferFixture(db, rlC, ownerID, "ccccccc7-3333-3333-3333-333333333333", true)
		if srvID > 0 {
			db.Exec("DELETE FROM users WHERE id = $1", srvID)
		}
		db.Exec("DELETE FROM fed_incoming_requests WHERE source_url LIKE '%127.0.0.1%' AND bookname='Оффер-Отмена'")
		db.Close()
	}()

	db.Exec(`DELETE FROM edition_files WHERE edition_id >= 2950000`)
	db.Exec(`DELETE FROM editions WHERE id >= 2950000`)
	db.Exec(`DELETE FROM works WHERE id >= 2950000`)
	db.Exec(`DELETE FROM persons WHERE id >= 2950000`)

	cfg := offerTestCfg(t)
	nc, err := NewNeighbourCrypto(db)
	require.NoError(t, err)

	backup := backupNeighbours(t, db)
	defer backup()

	require.NoError(t, db.QueryRow(`INSERT INTO users (username, password_hash, role)
		VALUES ($1,$2,'server') RETURNING id`,
		"offer_server_"+strconv.FormatInt(time.Now().UnixNano(), 36), "$2a$10$dummyhash").Scan(&srvID))
	serverToken := generateToken(srvID, "offer_server", "server")

	const uidC = "ccccccc7-3333-3333-3333-333333333333"
	rlC, ownerID = insertOfferRLWithUser(t, db, uidC)

	// Offering neighbour (its endpoints will actually serve the pull)…
	dataC := makeFB2Zip("Оффер-Отмена", "АвторОтмена", "ТестОтмена")
	mock1 := newFedMockNeighbourMeta("Оффер-Отмена", "АвторОтмена ТестОтмена", 2_950_201, 2_950_202, 2_950_203, dataC)
	defer mock1.Close()
	// …and a second, unreachable neighbour with a still-pending delivery.
	nid1 := insertNeighbourWithCrypto(t, db, nc, mock1.srv.URL)
	defer db.Exec("DELETE FROM api_neighbours WHERE id = $1", nid1)
	nid2 := insertNeighbourWithCrypto(t, db, nc, "http://127.0.0.1:9")
	defer db.Exec("DELETE FROM api_neighbours WHERE id = $1", nid2)

	// Outbox: n1 already delivered, n2 still pending retry.
	_, err = db.Exec(`
		INSERT INTO fed_request_outbox (neighbour_id, uid, bookname, author, priority, status)
		VALUES ($1, $3::uuid, 'Запрос на книгу', 'Неизвестно', 1, 'delivered'),
		       ($2, $3::uuid, 'Запрос на книгу', 'Неизвестно', 1, 'pending')`,
		nid1, nid2, uidC)
	require.NoError(t, err)
	t.Cleanup(func() { db.Exec(`DELETE FROM fed_request_outbox WHERE uid=$1::uuid`, uidC) })

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db); c.Set("config", cfg); c.Set("user_id", srvID); c.Next() })
	grp := r.Group("/api/v1/server")
	grp.Use(requireAuthMiddleware(), serverOnlyMiddleware())
	grp.POST("/book/offer", serverOfferBook(db, nc))

	metaC := fedBookMetadata{
		Work:    fedWorkMeta{ID: 2_950_202, OriginalTitle: "Оффер-Отмена", WorkType: "novel"},
		Edition: fedEditionMeta{ID: 2_950_201, WorkID: 2_950_202, Title: "Оффер-Отмена", IsComplete: true},
		Authors: []fedAuthorMeta{{ID: 2_950_203, FirstName: "АвторОтмена", LastName: "ТестОтмена", Role: "author"}},
	}
	raw := makeOfferBody(mock1.srv.URL, uidC, rlC, metaC)
	req, _ := http.NewRequest("POST", "/api/v1/server/book/offer", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+serverToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// The pending row for the unreachable neighbour is cancelled…
	var st2 string
	require.NoError(t, db.QueryRow(
		`SELECT status FROM fed_request_outbox WHERE neighbour_id=$1 AND uid=$2::uuid`, nid2, uidC).Scan(&st2))
	assert.Equal(t, "cancelled", st2)
	// …while the delivered row stays delivered.
	var st1 string
	require.NoError(t, db.QueryRow(
		`SELECT status FROM fed_request_outbox WHERE neighbour_id=$1 AND uid=$2::uuid`, nid1, uidC).Scan(&st1))
	assert.Equal(t, "delivered", st1)

	// The fulfilled request is no longer gathered for distribution, and a
	// fresh distribution pass does NOT resurrect the cancelled rows.
	fedCfg := config.FederationConfig{Enabled: true, PushIntervalSec: 300, RetryIntervalSec: 1, RetryWindowSec: 10}
	dist := newFedRequestsDistributor(db, nc, &fedCfg, "")
	reqs, err := dist.gatherApprovedRequests()
	require.NoError(t, err)
	for _, rq := range reqs {
		assert.NotEqual(t, uidC, rq.UID, "fulfilled request must stop being distributed")
	}
	dist.Run()
	require.NoError(t, db.QueryRow(
		`SELECT status FROM fed_request_outbox WHERE neighbour_id=$1 AND uid=$2::uuid`, nid2, uidC).Scan(&st2))
	assert.Equal(t, "cancelled", st2, "distribution pass must not resurrect cancelled deliveries")

}
