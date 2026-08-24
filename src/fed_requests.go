package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"libapp/src/config"
)

// ─── Federated book-request distribution ───────────────────────
//
// A "book request" advertised to neighbours is an admin-approved entry stored
// in fed_outgoing_requests. It is created from a user read_list request ONLY
// after an admin presses «Запросить по федерации» on the "Запросы" tab. The
// distributor never reads raw user requests directly.
//
// Delivery state is persisted in fed_request_outbox so it survives restarts.
// When a neighbour is unreachable the request is retried every
// retry_interval_sec, for up to retry_window_sec, then given up.
//
// Requests received from peers are stored in fed_incoming_requests and shown
// on the "Управление" page in an admin-only tab.

// fedBookRequest is one book the distributer wants to advertise to neighbours.
// UID is the message identifier carried to the peer and used for dedup — a
// message is delivered to a server only once.
type fedBookRequest struct {
	UID        string `json:"uid"`
	Bookname   string `json:"bookname"`
	Author     string `json:"author"`
	Priority   int    `json:"priority"`
	ReadListID string `json:"read_list_id"`
}

// fedBookRequest is also the wire payload for a push batch.
type fedPushBatch struct {
	SourceURL string           `json:"source_url"`
	Requests  []fedBookRequest `json:"requests"`
}

// fedRequestsDistributor delivers user book requests to peer servers and
// stores the ones it receives from them.
type fedRequestsDistributor struct {
	db        *sql.DB
	nc        *NeighbourCrypto
	cfg       *config.FederationConfig
	publicURL string
	runNow    chan chan struct{}
	mu        sync.Mutex
	lastRun   time.Time
	stop      chan struct{}
}

func newFedRequestsDistributor(db *sql.DB, nc *NeighbourCrypto, cfg *config.FederationConfig, publicURL string) *fedRequestsDistributor {
	return &fedRequestsDistributor{
		db:        db,
		nc:        nc,
		cfg:       cfg,
		publicURL: publicURL,
		runNow:    make(chan chan struct{}, 1),
		stop:      make(chan struct{}),
	}
}

// Start launches the background loop. It is non-blocking.
func (d *fedRequestsDistributor) Start() {
	go func() {
		interval := time.Duration(d.cfg.PushIntervalSec) * time.Second
		if interval <= 0 {
			interval = 300 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		log.Printf("[FED REQUESTS] background distributor started (interval=%s)", interval)
		for {
			select {
			case <-d.stop:
				return
			case <-ticker.C:
				d.Run()
			case ack := <-d.runNow:
				log.Printf("[FED REQUESTS] manual push triggered")
				d.Run()
				if ack != nil {
					close(ack)
				}
			}
		}
	}()
}

func (d *fedRequestsDistributor) Stop()   { select { case <-d.stop: default: close(d.stop) } }
func (d *fedRequestsDistributor) IsEnabled() bool { return d.cfg.Enabled }

// cancelPendingFedDeliveries stops further distribution of a request once at
// least one neighbour has responded with a book offer: pending/failed outbox
// rows carrying this request's uid are cancelled, so unreachable neighbours
// are not retried anymore. Rows already delivered (the peer has the request)
// are left untouched.
func cancelPendingFedDeliveries(db *sql.DB, uid string) {
	if uid == "" {
		return
	}
	db.Exec(`
		UPDATE fed_request_outbox SET status='cancelled', next_retry_at=NULL,
			last_error='Ответ от сервера уже получен', updated_at=CURRENT_TIMESTAMP
		WHERE uid = $1::uuid AND status IN ('pending','failed')`, uid)
}

// gatherApprovedRequests returns the admin-approved set of book requests to
// push to neighbours. It reads ONLY from fed_outgoing_requests — raw user
// read_list requests are never sent automatically. Requests that have already
// been answered by at least one server (fulfilled_at set) stop being
// distributed.
func (d *fedRequestsDistributor) gatherApprovedRequests() ([]fedBookRequest, error) {
	rows, err := d.db.Query(`
		SELECT DISTINCT g.uid::text, TRIM(g.bookname), TRIM(g.author), g.priority, COALESCE(g.read_list_id::text,'')
		FROM fed_outgoing_requests g
		WHERE g.status = 'approved'
		  AND g.fulfilled_at IS NULL
		  AND TRIM(g.bookname) != ''
		ORDER BY g.priority DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reqs := make([]fedBookRequest, 0)
	for rows.Next() {
		var r fedBookRequest
		if err := rows.Scan(&r.UID, &r.Bookname, &r.Author, &r.Priority, &r.ReadListID); err != nil {
			return nil, err
		}
		reqs = append(reqs, r)
	}
	return reqs, rows.Err()
}

// syncOutbox makes sure every local request has a pending outbox row for every
// neighbour, and marks outbox rows whose underlying request disappeared as
// cancelled. Returns the neighbours plus the due outbox rows.
func (d *fedRequestsDistributor) syncOutbox(reqs []fedBookRequest, neighbours []federationNeighbour, now time.Time) ([]fedOutboxRow, error) {
	if len(neighbours) == 0 {
		return nil, nil
	}
	// Cancel outbox rows for requests that are no longer admin-approved, so an
	// unapproved (removed/revoked) request is never re-delivered.
	_, err := d.db.Exec(`
		UPDATE fed_request_outbox o SET status='cancelled', next_retry_at=NULL,
			last_error='Отозвано администратором', updated_at=CURRENT_TIMESTAMP
		WHERE o.status IN ('pending','failed')
		  AND NOT EXISTS (SELECT 1 FROM fed_outgoing_requests g
		      WHERE g.status = 'approved'
		        AND TRIM(g.bookname) = o.bookname
		        AND TRIM(g.author) = o.author)`)
	if err != nil {
		return nil, err
	}

	// Populate newly-approved requests for each neighbour. The outbox carries
	// one row per (neighbour, bookname, author) — mirrored by the UNIQUE index
	// idx_fed_outbox_neighbour_book — so dedup must be on that key, not on uid:
	// several approved requests may share a title (each has its own uid) and
	// inserting a second same-title row would violate the index and abort the
	// whole pass. The batch for a delivered row carries the uid/read_list_id of
	// whichever approved request first populated it.
	for _, n := range neighbours {
		for _, r := range reqs {
			var existing bool
			err := d.db.QueryRow(`
				SELECT EXISTS(SELECT 1 FROM fed_request_outbox
					WHERE neighbour_id = $1 AND bookname = $2 AND author = $3)`,
				n.id, r.Bookname, r.Author).Scan(&existing)
			if err != nil {
				return nil, err
			}
			if !existing {
				status := "pending"
				if !d.cfg.Enabled {
					status = "failed"
				}
				_, err = d.db.Exec(`
					INSERT INTO fed_request_outbox
						(neighbour_id, uid, bookname, author, priority, status, next_retry_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7)`,
					n.id, r.UID, r.Bookname, r.Author, r.Priority, status, now)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// Load due rows (pending/failed that are due now).
	rows, err := d.db.Query(`
		SELECT o.id, o.neighbour_id, o.uid::text, o.bookname, o.author, o.priority, o.status, o.attempts,
		       COALESCE(g.read_list_id::text,''), COALESCE(o.next_retry_at, o.created_at), o.created_at
		FROM fed_request_outbox o
		LEFT JOIN fed_outgoing_requests g ON g.uid = o.uid
		WHERE o.status IN ('pending','failed')
		  AND COALESCE(o.next_retry_at, o.created_at) <= $1
		ORDER BY o.neighbour_id, o.priority DESC`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]fedOutboxRow, 0)
	for rows.Next() {
		var r fedOutboxRow
		var nextRetry sql.NullTime
		if err := rows.Scan(&r.ID, &r.NeighbourID, &r.UID, &r.Bookname, &r.Author, &r.Priority,
			&r.Status, &r.Attempts, &r.ReadListID, &nextRetry, &r.CreatedAt); err != nil {
			return nil, err
		}
		if nextRetry.Valid {
			r.NextRetryAt = nextRetry.Time
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type fedOutboxRow struct {
	ID          int
	NeighbourID int
	UID         string
	Bookname    string
	Author      string
	Priority    int
	Status      string
	Attempts    int
	ReadListID  string
	NextRetryAt time.Time
	CreatedAt   time.Time
}

// Run performs one full distribution pass. It is safe to call manually.
func (d *fedRequestsDistributor) Run() {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	d.lastRun = now

	neighbours, err := loadFederationNeighbours(d.db)
	if err != nil {
		log.Printf("[FED REQUESTS] failed to load neighbours: %v", err)
		return
	}
	if len(neighbours) == 0 {
		return
	}

	reqs, err := d.gatherApprovedRequests()
	if err != nil {
		log.Printf("[FED REQUESTS] failed to gather local requests: %v", err)
		return
	}

	due, err := d.syncOutbox(reqs, neighbours, now)
	if err != nil {
		log.Printf("[FED REQUESTS] failed to sync outbox: %v", err)
		return
	}
	if len(due) == 0 {
		return
	}

	// Group due rows per neighbour.
	byNeighbour := make(map[int][]fedOutboxRow)
	order := make([]int, 0, len(neighbours))
	for _, r := range due {
		if _, ok := byNeighbour[r.NeighbourID]; !ok {
			order = append(order, r.NeighbourID)
		}
		byNeighbour[r.NeighbourID] = append(byNeighbour[r.NeighbourID], r)
	}

	nbByID := make(map[int]federationNeighbour)
	for _, n := range neighbours {
		nbByID[n.id] = n
	}

	for _, nid := range order {
		n, ok := nbByID[nid]
		if !ok {
			continue
		}
		rows := byNeighbour[nid]
		d.deliverToNeighbour(n, rows, now)
	}
}

// deliverToNeighbour pushes a batch of request for one neighbour and updates
// the outbox (delivered / schedule-retry / give up) accordingly.
func (d *fedRequestsDistributor) deliverToNeighbour(n federationNeighbour, rows []fedOutboxRow, now time.Time) {
	reqs := make([]fedBookRequest, 0, len(rows))
	for _, r := range rows {
		reqs = append(reqs, fedBookRequest{UID: r.UID, Bookname: r.Bookname, Author: r.Author, Priority: r.Priority, ReadListID: r.ReadListID})
	}

	ack, err := d.pushToNeighbour(n, reqs)
	if err == nil {
		for _, r := range rows {
			d.markDelivered(r.ID)
		}
		log.Printf("[FED REQUESTS] neighbour id=%d url=%q delivered %d requests (received=%d exists=%d)", n.id, n.url, len(rows), ack.Received, ack.Exists)
		return
	}

	// Failed. Schedule retry or give up.
	for _, r := range rows {
		attempts := r.Attempts + 1
		elapsed := now.Sub(r.CreatedAt)
		if elapsed >= time.Duration(d.cfg.RetryWindowSec)*time.Second {
			d.markFailed(r.ID, attempts, err.Error())
			log.Printf("[FED REQUESTS] neighbour id=%d url=%q gave up on %q after %v: %v", n.id, n.url, r.Bookname, elapsed, err)
			continue
		}
		next := now.Add(time.Duration(d.cfg.RetryIntervalSec) * time.Second)
		d.scheduleRetry(r.ID, attempts, next, err.Error())
	}
	log.Printf("[FED REQUESTS] neighbour id=%d url=%q unavailable: %v", n.id, n.url, err)
}

// pushToNeighbour logs in on the neighbour and POSTs the batch. Returns the
// receipt counts: Received = messages newly stored by the peer,
// Exists = messages the peer already had (deduped by uid).
func (d *fedRequestsDistributor) pushToNeighbour(n federationNeighbour, reqs []fedBookRequest) (fedPushAck, error) {
	var zero fedPushAck
	if !d.cfg.Enabled {
		return zero, fmt.Errorf("federation disabled")
	}
	if len(reqs) == 0 {
		return zero, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, base, token, errMsg := loginToNeighbour(ctx, d.nc, n)
	if errMsg != "" {
		return zero, fmt.Errorf("%s", errMsg)
	}

	sourceURL := d.publicURL
	if sourceURL == "" {
		// Fall back to the neighbour's own URL when this server has no public
		// URL configured (existing deployments).
		sourceURL = base
	}
	body, _ := json.Marshal(fedPushBatch{SourceURL: sourceURL, Requests: reqs})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		base+"/api/v1/server/requests/push", bytes.NewReader(body))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("peer returned %d", resp.StatusCode)
	}
	var ack fedPushAck
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		return zero, fmt.Errorf("bad ack: %v", err)
	}
	return ack, nil
}

// fedPushAck is the receipt reply from a peer. Received counts messages newly
// stored; Exists counts messages the peer already had (deduped by uid). A
// message counted in either bucket is on the peer and the outbox row is marked
// delivered.
type fedPushAck struct {
	Received int `json:"received"`
	Exists   int `json:"exists"`
}

func (d *fedRequestsDistributor) markDelivered(id int) {
	d.db.Exec(`UPDATE fed_request_outbox SET status='delivered', attempts=attempts+0,
		next_retry_at=NULL, last_error='', updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id)
}

func (d *fedRequestsDistributor) scheduleRetry(id int, attempts int, next time.Time, errMsg string) {
	d.db.Exec(`UPDATE fed_request_outbox SET status='failed', attempts=$2,
		next_retry_at=$3, last_error=$4, updated_at=CURRENT_TIMESTAMP WHERE id=$1`,
		id, attempts, next, errMsg)
}

func (d *fedRequestsDistributor) markFailed(id int, attempts int, errMsg string) {
	d.db.Exec(`UPDATE fed_request_outbox SET status='failed', attempts=$2,
		next_retry_at=NULL, last_error=$3, updated_at=CURRENT_TIMESTAMP WHERE id=$1`,
		id, attempts, errMsg)
}

// serverReceiveRequestPush POST /api/v1/server/requests/push (role: server).
// Stores the batch of book requests a peer sent us and confirms receipt.
// Messages are identified by (source_url, uid): a uid that was already stored
// is counted as "exists" and NOT written again. The peer then marks the
// corresponding outbox row delivered and does not resend it.
func serverReceiveRequestPush(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var batch fedPushBatch
		if err := c.ShouldBindJSON(&batch); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
			return
		}
		batch.SourceURL = strings.TrimSpace(batch.SourceURL)
		received := 0
		exists := 0
		tx, err := db.Begin()
		if err != nil {
			internalError(c, err)
			return
		}
		for _, r := range batch.Requests {
			bookname := strings.TrimSpace(r.Bookname)
			if bookname == "" {
				continue
			}
			existsPostgres := false
			if r.UID != "" {
				// Identify by uid — the dedup key.
				err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM fed_incoming_requests
					WHERE source_url = $1 AND uid = $2)`,
					batch.SourceURL, r.UID).Scan(&existsPostgres)
			} else {
				// Legacy peers without a uid: fall back to bookname+author.
				err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM fed_incoming_requests
					WHERE source_url = $1 AND bookname = $2 AND author = $3)`,
					batch.SourceURL, bookname, r.Author).Scan(&existsPostgres)
			}
			if err != nil {
				tx.Rollback()
				internalError(c, err)
				return
			}
			if existsPostgres {
				exists++
				continue
			}
_, err = tx.Exec(`INSERT INTO fed_incoming_requests
			(source_url, uid, bookname, author, priority, read_list_id) VALUES ($1,NULLIF($2,'')::uuid,$3,$4,$5,NULLIF($6,'')::uuid)`,
			batch.SourceURL, r.UID, bookname, r.Author, r.Priority, r.ReadListID)
			if err != nil {
				tx.Rollback()
				internalError(c, err)
				return
			}
			received++
		}
		if err := tx.Commit(); err != nil {
			internalError(c, err)
			return
		}
		log.Printf("[FED REQUESTS] received %d (%d already present) from %q", received, exists, batch.SourceURL)
		c.JSON(http.StatusOK, gin.H{"received": received, "exists": exists})
	}
}

// adminFedListRequests GET /api/v1/admin/fed/requests (admin only).
func adminFedListRequests(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT id, source_url, bookname, author, priority, status, created_at, updated_at,
			       COALESCE(offered_edition_id,0), COALESCE(offered_title,''), COALESCE(offered_authors,''),
			       COALESCE(delivery_status,''), COALESCE(delivery_error,''), delivered_at
			FROM fed_incoming_requests ORDER BY created_at DESC`)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()
		items := make([]gin.H, 0)
		for rows.Next() {
			var id int
			var src, bookname, author, status string
			var priority, offed int
			var created, updated sql.NullTime
			var offTitle, offAuthors, delStatus, delErr string
			var delivered sql.NullTime
			if err := rows.Scan(&id, &src, &bookname, &author, &priority, &status,
				&created, &updated, &offed, &offTitle, &offAuthors, &delStatus, &delErr, &delivered); err != nil {
				adminInternalError(c, err)
				return
			}
			deliveredAt := ""
			if delivered.Valid {
				deliveredAt = delivered.Time.Format(time.RFC3339)
			}
			items = append(items, gin.H{
				"id": id, "source_url": src, "bookname": bookname, "author": author,
				"priority": priority, "status": status,
				"created_at": created.Time, "updated_at": updated.Time,
				"offered_edition_id": offed, "offered_title": offTitle, "offered_authors": offAuthors,
				"delivery_status": delStatus, "delivery_error": delErr, "delivered_at": deliveredAt,
			})
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}

// adminFedSetRequestStatus POST /api/v1/admin/fed/requests/:id/status (admin).
type fedStatusRequest struct {
	Status string `json:"status"`
}

func adminFedSetRequestStatus(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req fedStatusRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
			return
		}
		switch req.Status {
		case "new", "done", "hidden":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Недопустимый статус"})
			return
		}
		res, err := db.Exec(`UPDATE fed_incoming_requests SET status=$2,
			updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id, req.Status)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Запрос не найден"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// adminFedDeleteRequest DELETE /api/v1/admin/fed/requests/:id (admin). Removes
// an incoming federated request from the given neighbour.
func adminFedDeleteRequest(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		res, err := db.Exec(`DELETE FROM fed_incoming_requests WHERE id=$1`, id)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Запрос не найден"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
// immediate background distribution pass.
func adminFedPushNow(db *sql.DB, dist *fedRequestsDistributor) gin.HandlerFunc {
	return func(c *gin.Context) {
		ack := make(chan struct{})
		select {
		case dist.runNow <- ack:
			select {
			case <-ack:
			case <-time.After(60 * time.Second):
			}
		default:
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// ─── Admin-approved outbound requests (fed_outgoing_requests) ───

type fedApproveOutgoingRequest struct {
	ReadListID string `json:"read_list_id" binding:"required"`
}

// adminFedApproveOutgoing POST /api/v1/admin/fed/outgoing (admin only).
// Copies a user read_list request into the admin-approved outbound table.
// Only these rows are ever distributed to neighbours.
func adminFedApproveOutgoing(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req fedApproveOutgoingRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
			return
		}
		var bookname, author string
		var priority int
		err := db.QueryRow(`
			SELECT TRIM(rl.bookname), TRIM(rl.author), rl.priority
			FROM read_list rl
			WHERE rl.id = $1 AND rl.looking_for != 'Нет' AND rl.deleted = FALSE`,
			req.ReadListID).Scan(&bookname, &author, &priority)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Запрос не найден"})
			return
		}
		if err != nil {
			adminInternalError(c, err)
			return
		}
		if bookname == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "У запроса не указана книга"})
			return
		}
		var id int
		var status string
		err = db.QueryRow(`
			INSERT INTO fed_outgoing_requests (read_list_id, bookname, author, priority, status)
			VALUES ($1, $2, $3, $4, 'approved')
			ON CONFLICT (read_list_id) WHERE read_list_id IS NOT NULL
			DO UPDATE SET status='approved', bookname=EXCLUDED.bookname,
			  author=EXCLUDED.author, priority=EXCLUDED.priority,
			  uid=gen_random_uuid(), fulfilled_at=NULL, fulfilled_by_url=NULL,
			  updated_at=CURRENT_TIMESTAMP
			RETURNING id, status`, req.ReadListID, bookname, author, priority).Scan(&id, &status)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "id": id, "read_list_id": req.ReadListID,
			"bookname": bookname, "author": author, "status": status})
	}
}

// adminFedListOutgoing GET /api/v1/admin/fed/outgoing (admin only).
func adminFedListOutgoing(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT id, COALESCE(read_list_id::text,''), bookname, author, priority,
			       status, created_at, updated_at
			FROM fed_outgoing_requests ORDER BY created_at DESC`)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()
		items := make([]gin.H, 0)
		for rows.Next() {
			var id int
			var rl, bookname, author, status string
			var priority int
			var created, updated sql.NullTime
			if err := rows.Scan(&id, &rl, &bookname, &author, &priority, &status,
				&created, &updated); err != nil {
				adminInternalError(c, err)
				return
			}
			items = append(items, gin.H{
				"id": id, "read_list_id": rl, "bookname": bookname, "author": author,
				"priority": priority, "status": status,
				"created_at": created.Time, "updated_at": updated.Time,
			})
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}

// adminFedRemoveOutgoing DELETE /api/v1/admin/fed/outgoing/:id (admin only).
// Revokes an approved request and cancels its pending/failed outbox rows so it
// is not delivered further.
func adminFedRemoveOutgoing(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var bookname, author string
		err := db.QueryRow(`
			UPDATE fed_outgoing_requests SET status='removed',
			  updated_at=CURRENT_TIMESTAMP
			WHERE id = $1 AND status = 'approved'
			RETURNING bookname, author`, id).Scan(&bookname, &author)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена"})
			return
		}
		if err != nil {
			adminInternalError(c, err)
			return
		}
		_, err = db.Exec(`
			UPDATE fed_request_outbox SET status='cancelled', next_retry_at=NULL,
			  updated_at=CURRENT_TIMESTAMP
			WHERE bookname = $1 AND author = $2 AND status IN ('pending','failed')`,
			bookname, author)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}