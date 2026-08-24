package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"libapp/src/config"
	"libapp/src/utils"
)

// ─── Federated search (this server queries its neighbours) ─────
//
// The admin presses "Поиск по федерации" on the "Запросы" tab. The handler
// walks every entry in api_neighbours, authenticates against each neighbour
// with the stored server-role credentials (password decrypted via
// NeighbourCrypto), and forwards the search to the neighbour's
// /api/v1/server/search endpoint. Self-signed TLS is supported: the
// neighbour's server certificate is added to the trust pool and its client
// certificate (if a combined cert+key PEM was stored) is used for mutual TLS.

// FederationResult is the outcome of querying a single neighbour.
type FederationResult struct {
	NeighbourID int          `json:"neighbour_id"`
	URL         string       `json:"url"`
	Error       string       `json:"error,omitempty"`
	Total       int          `json:"total"`
	Books       []ServerBook `json:"books"`
}

type federationNeighbour struct {
	id         int
	url        string
	serverCert string
	clientCert string
	username   string
	passwordEnc string
}

// adminFederationSearch POST /api/v1/admin/federation/search (admin only).
// With ?stop_on_first=1 neighbours are queried sequentially and the search
// stops at the first neighbour that returns at least one book.
func adminFederationSearch(db *sql.DB, nc *NeighbourCrypto) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ServerSearchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
			return
		}
		req.Query = strings.TrimSpace(req.Query)
		req.Author = strings.TrimSpace(req.Author)
		req.Title = strings.TrimSpace(req.Title)
		if req.Query == "" && req.Author == "" && req.Title == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите хотя бы одно поле: query, author или title"})
			return
		}

		limit := 20
		if v := c.Query("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}
		stopOnFirst := c.Query("stop_on_first") == "1"

		neighbours, err := loadFederationNeighbours(db)
		if err != nil {
			adminInternalError(c, err)
			return
		}

		if len(neighbours) == 0 {
			c.JSON(http.StatusOK, gin.H{"neighbours": 0, "results": []FederationResult{}})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		results := make([]FederationResult, 0, len(neighbours))
		if stopOnFirst {
			// Randomize the order of traversal, seeded by the current system
			// time, so the first server in the list is not always hit first
			// (or always missed when it is down).
			order := rand.New(rand.NewSource(time.Now().UnixNano())).Perm(len(neighbours))
			for _, idx := range order {
				res := queryNeighbour(ctx, nc, neighbours[idx], req, limit)
				results = append(results, res)
				if res.Error == "" && len(res.Books) > 0 {
					break
				}
			}
		} else {
			results = make([]FederationResult, len(neighbours))
			var wg sync.WaitGroup
			sem := make(chan struct{}, 3)
			for i := range neighbours {
				wg.Add(1)
				sem <- struct{}{}
				go func(i int) {
					defer wg.Done()
					defer func() { <-sem }()
					results[i] = queryNeighbour(ctx, nc, neighbours[i], req, limit)
				}(i)
			}
			wg.Wait()
		}

		c.JSON(http.StatusOK, gin.H{
			"neighbours": len(neighbours),
			"results":    results,
		})
	}
}

// federationTestRequest is the body of POST /api/v1/admin/federation/test.
type federationTestRequest struct {
	NeighbourID int `json:"neighbour_id"`
}

// adminFederationTest POST /api/v1/admin/federation/test (admin only).
// Logs in to the neighbour with the stored server-role credentials (the same
// flow as federated search/import) and sends a test message to the neighbour's
// GET /api/v1/server/ping. On success returns 200 {ok:true}; on any failure the
// error is written to the server log and returned as HTTP 502 with {ok:false,
// error}.
func adminFederationTest(db *sql.DB, nc *NeighbourCrypto) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req federationTestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
			return
		}
		if req.NeighbourID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите neighbour_id"})
			return
		}

		var n federationNeighbour
		err := db.QueryRow(`
			SELECT id, url, server_cert, client_cert, username, password_encrypted
			FROM api_neighbours WHERE id = $1`, req.NeighbourID).
			Scan(&n.id, &n.url, &n.serverCert, &n.clientCert, &n.username, &n.passwordEnc)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Сосед не найден"})
				return
			}
			adminInternalError(c, err)
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		defer cancel()

		client, base, token, errMsg := loginToNeighbour(ctx, nc, n)
		if errMsg != "" {
			log.Printf("[FEDERATION TEST] neighbour id=%d url=%q FAILED: %s", n.id, n.url, errMsg)
			c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": errMsg})
			return
		}

		pingReq, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/server/ping", nil)
		if err != nil {
			log.Printf("[FEDERATION TEST] neighbour id=%d url=%q FAILED: %s", n.id, n.url, err.Error())
			c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error()})
			return
		}
		pingReq.Header.Set("Authorization", "Bearer "+token)
		pingResp, err := client.Do(pingReq)
		if err != nil {
			log.Printf("[FEDERATION TEST] neighbour id=%d url=%q FAILED: %s", n.id, n.url, err.Error())
			c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": "Ошибка подключения: " + err.Error()})
			return
		}
		defer pingResp.Body.Close()
		if pingResp.StatusCode != http.StatusOK {
			msg := fmt.Sprintf("Сервер вернул ошибку (%d)", pingResp.StatusCode)
			log.Printf("[FEDERATION TEST] neighbour id=%d url=%q FAILED: %s", n.id, n.url, msg)
			c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": msg})
			return
		}

		log.Printf("[FEDERATION TEST] neighbour id=%d url=%q OK", n.id, n.url)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// loadFederationNeighbours loads all neighbour rows ordered by URL.
func loadFederationNeighbours(db *sql.DB) ([]federationNeighbour, error) {
	rows, err := db.Query(`
		SELECT id, url, server_cert, client_cert, username, password_encrypted
		FROM api_neighbours ORDER BY url`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	neighbours := make([]federationNeighbour, 0)
	for rows.Next() {
		var n federationNeighbour
		if err := rows.Scan(&n.id, &n.url, &n.serverCert, &n.clientCert,
			&n.username, &n.passwordEnc); err != nil {
			return nil, err
		}
		neighbours = append(neighbours, n)
	}
	return neighbours, rows.Err()
}

// federationImportRequest is the body of POST /api/v1/admin/federation/import.
//
//	Mode ""          – initial call: imports only when there are no identifier
//	                  conflicts; otherwise returns HTTP 409 with the found local
//	                  book so the admin can choose the resolution.
//	Mode "overwrite" – re-import keeping the remote identifiers, replacing the
//	                  locally-stored work/edition rows that occupy them.
//	Mode "create_new" – import with freshly generated identifiers, keeping the
//	                  existing local records untouched.
type federationImportRequest struct {
	NeighbourID int    `json:"neighbour_id"`
	EditionID   int    `json:"edition_id"`
	Mode        string `json:"mode"`
}

// fedConflictSummary lists which remote identifiers are already occupied on the
// home server.
type fedConflictSummary struct {
	AuthorIDs []int `json:"authors"` // person ids that already exist locally
	Work      bool  `json:"work"`    // a local work already uses the remote work id
	Edition   bool  `json:"edition"` // a local edition already uses the remote edition id
}

func (cs *fedConflictSummary) any() bool {
	return cs != nil && (len(cs.AuthorIDs) > 0 || cs.Work || cs.Edition)
}

// fedFoundBook is the locally-found edition shown to the admin on a conflict.
type fedFoundBook struct {
	EditionID int    `json:"edition_id"`
	Title     string `json:"title"`
	Author    string `json:"author"`
}

// adminFederationImport POST /api/v1/admin/federation/import (admin only).
// Imports the given edition from the neighbour preserving the remote
// identifiers of the author, work and edition objects. Before importing it
// (a) fetches the author/work/edition metadata, (b) fuzzy-checks that the
// authors do not already exist on the home server (creating them with the same
// id otherwise), (c) detects identifier conflicts and asks the admin how to
// resolve them.
func adminFederationImport(db *sql.DB, nc *NeighbourCrypto) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req federationImportRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
			return
		}
		if req.NeighbourID <= 0 || req.EditionID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите neighbour_id и edition_id"})
			return
		}
		switch req.Mode {
		case "", "overwrite", "create_new":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный mode"})
			return
		}

		var n federationNeighbour
		err := db.QueryRow(`
			SELECT id, url, server_cert, client_cert, username, password_encrypted
			FROM api_neighbours WHERE id = $1`, req.NeighbourID).
			Scan(&n.id, &n.url, &n.serverCert, &n.clientCert, &n.username, &n.passwordEnc)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Сосед не найден"})
				return
			}
			adminInternalError(c, err)
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		defer cancel()

		client, base, token, errMsg := loginToNeighbour(ctx, nc, n)
		if errMsg != "" {
			c.JSON(http.StatusBadGateway, gin.H{"error": errMsg})
			return
		}

		// 1. Fetch the author / work / edition objects from the neighbour.
		meta, err := fetchFedMetadata(ctx, client, base, token, req.EditionID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Не удалось получить данные книги: " + err.Error()})
			return
		}

		// 2. Download the stored archive (a single-format zip).
		data, err := downloadFedEdition(ctx, client, base, token, req.EditionID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Не удалось скачать книгу: " + err.Error()})
			return
		}

		cfg := getConfig(c)

		// 3. Detect the inner format and compute the content hash.
		innerHash, formatName, formatID, err := fedAnalyzeBook(data, db)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось распознать формат книги: " + err.Error()})
			return
		}

		// 4. On the initial call, reject exact duplicates by content hash.
		if req.Mode == "" {
			if dup := findDuplicateByHash(db, innerHash); dup != nil {
				c.JSON(http.StatusOK, gin.H{
					"duplicate": true,
					"message":   fmt.Sprintf("Книга уже существует в библиотеке: %s — %s", dup.authors, dup.title),
					"file_hash": dup.hash,
					"title":     dup.title,
					"authors":   dup.authors,
				})
				return
			}
		}

		// 5. Check whether the remote identifiers are already in use locally.
		conflicts, err := analyzeFedConflicts(db, meta)
		if err != nil {
			adminInternalError(c, err)
			return
		}

		if conflicts.any() && req.Mode == "" {
			// Report the collision and let the admin choose how to proceed.
			remote := gin.H{
				"work_id":    meta.Work.ID,
				"edition_id": meta.Edition.ID,
				"title":      meta.Edition.Title,
				"author":     fedAuthorsDisplay(meta),
			}
			resp := gin.H{
				"conflict": true,
				"error":    "На домашнем сервере уже есть записи с такими же идентификаторами",
				"remote":   remote,
				"conflicts": gin.H{
					"authors": conflicts.AuthorIDs,
					"work":    conflicts.Work,
					"edition": conflicts.Edition,
				},
			}
			if found := findLocalBookForConflict(db, meta); found != nil {
				resp["found"] = found
			}
			c.JSON(http.StatusConflict, resp)
			return
		}

		userID := c.GetInt("user_id")
		var workID, editionID int
		var relPath string
		mode := "created"
		switch {
		case conflicts.any() && req.Mode == "overwrite":
			workID, editionID, relPath, err = fedOverwriteLocal(db, cfg, meta, data, innerHash, formatName, formatID, userID)
			mode = "overwritten"
		case conflicts.any() && req.Mode == "create_new":
			workID, editionID, relPath, err = fedCreateLocal(db, cfg, meta, data, innerHash, formatName, formatID, userID, true)
			mode = "created_new"
		default:
			workID, editionID, relPath, err = fedCreateLocal(db, cfg, meta, data, innerHash, formatName, formatID, userID, false)
			mode = "created"
		}
		if err != nil {
			adminInternalError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":    "Book imported successfully",
			"mode":       mode,
			"work_id":    workID,
			"edition_id": editionID,
			"file_path":  relPath,
			"title":      meta.Edition.Title,
			"authors":    fedAuthorsDisplay(meta),
		})
	}
}

// ─── Helpers ───────────────────────────────────────────────────

// fetchFedMetadata GET /api/v1/server/metadata/:id — returns the neighbour's
// author/work/edition objects for the given edition.
func fetchFedMetadata(ctx context.Context, client *http.Client, base, token string, editionID int) (*fedBookMetadata, error) {
	url := fmt.Sprintf("%s/api/v1/server/metadata/%d", base, editionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var meta fedBookMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// downloadFedEdition GET /api/v1/server/download/:id — downloads the stored
// archive (a single-format zip) of the given edition.
func downloadFedEdition(ctx context.Context, client *http.Client, base, token string, editionID int) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v1/server/download/%d", base, editionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// fedAnalyzeBook detects the inner (non-archive) format of the downloaded zip
// and returns the content hash over that inner content, the format name and
// its id in the formats table.
func fedAnalyzeBook(data []byte, db *sql.DB) (innerHash, formatName string, formatID int, err error) {
	inner := data
	formatName = "FB2.ZIP"
	if zipResult, zipErr := utils.DetectZipContent(data); zipErr == nil && zipResult.Content != nil {
		switch zipResult.ContentType {
		case utils.ZipContentUnknown:
		default:
			inner = zipResult.Content
			formatName = utils.ZipContentTypeToFormatName(zipResult.ContentType)
		}
	}
	h := sha256.Sum256(inner)
	innerHash = hex.EncodeToString(h[:])
	if err := db.QueryRow("SELECT id FROM formats WHERE name=$1", formatName).Scan(&formatID); err != nil {
		formatID = 1
	}
	return innerHash, formatName, formatID, nil
}

// findDuplicateByHash returns the duplicate book info when a file with the same
// content hash already exists in the library, or nil otherwise.
func findDuplicateByHash(db *sql.DB, hashStr string) *duplicateInfo {
	var title, authors string
	err := db.QueryRow(`
		SELECT w.original_title,
			STRING_AGG(p.last_name || ' ' || COALESCE(p.first_name, ''), ', ' ORDER BY p.last_name)
		FROM edition_files ef
		JOIN editions e ON ef.edition_id = e.id
		JOIN works w ON e.work_id = w.id
		LEFT JOIN work_contributors wc ON w.id = wc.work_id AND wc.role = 'author'
		LEFT JOIN persons p ON wc.person_id = p.id
		WHERE ef.file_hash = $1
		GROUP BY w.original_title`, hashStr).Scan(&title, &authors)
	if err != nil {
		return nil
	}
	if authors == "" {
		authors = "Неизвестный автор"
	}
	return &duplicateInfo{title: title, authors: authors, hash: hashStr}
}

// analyzeFedConflicts checks which of the remote identifiers (authors, work,
// edition) already exist on the home server.
func analyzeFedConflicts(db *sql.DB, meta *fedBookMetadata) (*fedConflictSummary, error) {
	cs := &fedConflictSummary{AuthorIDs: []int{}}
	for _, a := range meta.Authors {
		var exists bool
		if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM persons WHERE id=$1)", a.ID).Scan(&exists); err != nil {
			return nil, err
		}
		if exists {
			cs.AuthorIDs = append(cs.AuthorIDs, a.ID)
		}
	}
	if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM works WHERE id=$1)", meta.Work.ID).Scan(&cs.Work); err != nil {
		return nil, err
	}
	if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM editions WHERE id=$1)", meta.Edition.ID).Scan(&cs.Edition); err != nil {
		return nil, err
	}
	return cs, nil
}

// findLocalBookForConflict returns the locally-found edition (by the conflicting
// id first, then by a fuzzy title match) to show the admin on a conflict.
func findLocalBookForConflict(db *sql.DB, meta *fedBookMetadata) *fedFoundBook {
	query := `
		SELECT e.id, e.title,
			STRING_AGG(p.last_name || ' ' || COALESCE(p.first_name, ''), ', ' ORDER BY p.last_name) AS author
		FROM editions e
		JOIN works w ON w.id = e.work_id
		LEFT JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
		LEFT JOIN persons p ON wc.person_id = p.id`
	var b fedFoundBook
	var author sql.NullString
	err := db.QueryRow(query+` WHERE e.id = $1 GROUP BY e.id, e.title`, meta.Edition.ID).
		Scan(&b.EditionID, &b.Title, &author)
	if err == nil {
		b.Author = author.String
		return &b
	}
	// Fall back to a fuzzy (trigram) title match.
	norm := strings.ToLower(strings.ReplaceAll(meta.Edition.Title, "ё", "е"))
	err = db.QueryRow(query+` WHERE e.lower_title % $1 GROUP BY e.id, e.title
		ORDER BY similarity(e.lower_title, $1) DESC LIMIT 1`, norm).
		Scan(&b.EditionID, &b.Title, &author)
	if err == nil {
		b.Author = author.String
		return &b
	}
	return nil
}

// fedAuthorsDisplay renders the remote author list as "Last First, Last First".
func fedAuthorsDisplay(meta *fedBookMetadata) string {
	var names []string
	for _, a := range meta.Authors {
		name := strings.TrimSpace(strings.TrimSpace(a.LastName) + " " + strings.TrimSpace(a.FirstName))
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "Неизвестный автор"
	}
	return strings.Join(names, ", ")
}

// fedCreateLocal writes the downloaded book into the local library preserving
// the remote identifiers. With allowNewIDs=true the identifiers are generated
// fresh (used for the "create_new" resolution); authors are still fuzzy-matched
// and reused when possible.
func fedCreateLocal(db *sql.DB, cfg *config.Config, meta *fedBookMetadata, data []byte, innerHash, formatName string, formatID, userID int, allowNewIDs bool) (workID, editionID int, relPath string, err error) {
	archivePath, err := fedWriteArchive(cfg, meta.Edition.Title, formatName, data)
	if err != nil {
		return 0, 0, "", err
	}

	tx, err := db.Begin()
	if err != nil {
		os.Remove(archivePath)
		return 0, 0, "", err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			os.Remove(archivePath)
		}
	}()

	workID, err = fedInsertWork(tx, db, meta, allowNewIDs)
	if err != nil {
		return 0, 0, "", err
	}

	editionID, err = fedInsertEdition(tx, db, meta, workID, meta.Edition.ID, allowNewIDs, userID)
	if err != nil {
		return 0, 0, "", err
	}

	if formatID <= 0 {
		formatID = 1
	}
	fileInfo, _ := os.Stat(archivePath)
	size := int64(0)
	if fileInfo != nil {
		size = fileInfo.Size()
	}
	relPath = fedRelPath(cfg, archivePath)
	_, err = tx.Exec(`
		INSERT INTO edition_files (edition_id, format_id, file_path, file_size, file_hash, is_primary)
		VALUES ($1, $2, $3, $4, $5, true)`,
		editionID, formatID, relPath, size, fedStorableHash(tx, innerHash, editionID))
	if err != nil {
		return 0, 0, "", err
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, "", err
	}
	return workID, editionID, relPath, nil
}

// fedStorableHash returns the hash to persist for a newly stored edition file,
// or nil when another edition already holds it. edition_files.file_hash has a
// UNIQUE constraint; re-importing a book whose content already exists locally
// (e.g. an "overwrite" after a "create_new" of the same book) must therefore
// store the copy without a dedup hash instead of failing the constraint.
func fedStorableHash(tx *sql.Tx, hash string, editionID int) interface{} {
	if hash == "" {
		return nil
	}
	var one int
	err := tx.QueryRow(
		`SELECT 1 FROM edition_files WHERE file_hash = $1 AND edition_id <> $2 LIMIT 1`,
		hash, editionID).Scan(&one)
	if err == nil {
		return nil
	}
	return hash
}

// fedOverwriteLocal re-imports the book keeping the remote identifiers and
// replacing the local work/edition rows that occupy them (other local editions
// and user data are preserved).
func fedOverwriteLocal(db *sql.DB, cfg *config.Config, meta *fedBookMetadata, data []byte, innerHash, formatName string, formatID, userID int) (workID, editionID int, relPath string, err error) {
	archivePath, err := fedWriteArchive(cfg, meta.Edition.Title, formatName, data)
	if err != nil {
		return 0, 0, "", err
	}

	// Best-effort removal of the previously stored primary file.
	if oldPath := fedPrimaryFilePath(db, meta.Edition.ID); oldPath != "" {
		os.Remove(filepath.Join(".", oldPath))
	}

	tx, err := db.Begin()
	if err != nil {
		os.Remove(archivePath)
		return 0, 0, "", err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			os.Remove(archivePath)
		}
	}()

	// Replace the work row (id must equal the remote work id).
	workID, err = fedUpsertWork(tx, db, meta, false)
	if err != nil {
		return 0, 0, "", err
	}

	// Replace the edition row (id must equal the remote edition id).
	editionID, err = fedUpsertEdition(tx, db, meta, workID, userID)
	if err != nil {
		return 0, 0, "", err
	}

	// Replace contributors/genres of the work with the remote ones.
	if _, err = tx.Exec(`DELETE FROM work_contributors WHERE work_id = $1`, workID); err != nil {
		return 0, 0, "", err
	}
	if err = fedInsertContributors(tx, db, workID, meta, false); err != nil {
		return 0, 0, "", err
	}
	if _, err = tx.Exec(`DELETE FROM work_genres WHERE work_id = $1`, workID); err != nil {
		return 0, 0, "", err
	}
	if err = fedInsertGenres(tx, db, workID, meta); err != nil {
		return 0, 0, "", err
	}

	// Replace the stored file.
	if _, err = tx.Exec(`DELETE FROM edition_files WHERE edition_id = $1`, editionID); err != nil {
		return 0, 0, "", err
	}
	if formatID <= 0 {
		formatID = 1
	}
	fileInfo, _ := os.Stat(archivePath)
	size := int64(0)
	if fileInfo != nil {
		size = fileInfo.Size()
	}
	relPath = fedRelPath(cfg, archivePath)
	_, err = tx.Exec(`
		INSERT INTO edition_files (edition_id, format_id, file_path, file_size, file_hash, is_primary)
		VALUES ($1, $2, $3, $4, $5, true)`,
		editionID, formatID, relPath, size, fedStorableHash(tx, innerHash, editionID))
	if err != nil {
		return 0, 0, "", err
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, "", err
	}
	return workID, editionID, relPath, nil
}

// fedPrimaryFilePath returns the stored primary file path of the edition (or
// "" when the edition has no files).
func fedPrimaryFilePath(db *sql.DB, editionID int) string {
	var p string
	if err := db.QueryRow(`SELECT file_path FROM edition_files WHERE edition_id=$1 AND is_primary=true`, editionID).Scan(&p); err != nil {
		return ""
	}
	return p
}

// ─── Admin-offered book sent back to the requesting neighbour ──

type federationOfferRequest struct {
	IncomingRequestID int `json:"incoming_request_id"`
	EditionID         int `json:"edition_id"`
}

// serverOffer is the reference a server sends when it offers one of its local
// books to the server that made a request. Instead of pushing the file bytes,
// the offer carries the identifiers plus this server's public URL so the
// requesting server can pull the book (federation-search style) and attach it
// to the user request that originated the order.
type serverOffer struct {
	SourceURL  string          `json:"source_url"`  // this server's URL as the requester knows it
	UID        string          `json:"uid"`         // the uid of the original request (from fed_incoming_requests)
	ReadListID string          `json:"read_list_id"` // the requester's stable read_list id this offer fulfils
	EditionID  int             `json:"edition_id"`   // the offered edition on this server
	WorkID     int             `json:"work_id"`      // the offered work on this server
	Metadata   fedBookMetadata `json:"metadata"`
}

// adminFederationOffer POST /api/v1/admin/federation/offer (admin only).
// Tells the requesting server to pull a locally-selected book. The requesting
// server imports it (or links the existing copy) and marks the user's request
// as fulfilled.
func adminFederationOffer(db *sql.DB, nc *NeighbourCrypto) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req federationOfferRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
			return
		}
		if req.IncomingRequestID <= 0 || req.EditionID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите incoming_request_id и edition_id"})
			return
		}

		// 1. Find the source server of the request.
		var sourceURL, uid, readListID string
		err := db.QueryRow(`SELECT source_url, COALESCE(uid::text,''), COALESCE(read_list_id::text,'') FROM fed_incoming_requests WHERE id=$1`,
			req.IncomingRequestID).Scan(&sourceURL, &uid, &readListID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Запрос соседа не найден"})
				return
			}
			adminInternalError(c, err)
			return
		}

		var n federationNeighbour
		err = db.QueryRow(`
			SELECT id, url, server_cert, client_cert, username, password_encrypted
			FROM api_neighbours WHERE url = $1`, sourceURL).
			Scan(&n.id, &n.url, &n.serverCert, &n.clientCert, &n.username, &n.passwordEnc)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Сервер-источник не найден в списке соседей"})
				return
			}
			adminInternalError(c, err)
			return
		}

		// 2. Build the local author/work/edition metadata and validate a file exists.
		meta, err := loadLocalBookMetadata(db, req.EditionID)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		if len(meta.Files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "У выбранной книги нет файла"})
			return
		}
		offeredTitle := meta.Edition.Title
		offeredAuthors := fedAuthorsDisplay(meta)
		recordDelivery := func(status, errMsg string) {
			db.Exec(`UPDATE fed_incoming_requests SET offered_edition_id=$1,
				offered_title=$2, offered_authors=$3, delivery_status=$4,
				delivery_error=$5, updated_at=CURRENT_TIMESTAMP WHERE id=$6`,
				req.EditionID, offeredTitle, offeredAuthors, status, errMsg, req.IncomingRequestID)
		}

		// 3. Send a reference to the requesting server (it will pull the book).
		cfg := getConfig(c)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
		defer cancel()
		client, loginBase, token, errMsg := loginToNeighbour(ctx, nc, n)
		if errMsg != "" {
			recordDelivery("failed", errMsg)
			c.JSON(http.StatusBadGateway, gin.H{"error": errMsg})
			return
		}
		// The requester must be able to reach US back, so advertise our own
		// public URL (what the requester has stored in its api_neighbours).
		advert := orDefault(cfg.Server.PublicURL, sourceURL)

		offer := serverOffer{SourceURL: advert, UID: uid, ReadListID: readListID, EditionID: req.EditionID, WorkID: meta.Work.ID, Metadata: *meta}
		bodyBytes, _ := json.Marshal(offer)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
			loginBase+"/api/v1/server/book/offer", bytes.NewReader(bodyBytes))
		if err != nil {
			recordDelivery("failed", err.Error())
			adminInternalError(c, err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(httpReq)
		if err != nil {
			recordDelivery("failed", "Не удалось отправить книгу: "+err.Error())
			c.JSON(http.StatusBadGateway, gin.H{"error": "Не удалось отправить книгу: " + err.Error()})
			return
		}
		defer resp.Body.Close()
		var out map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			recordDelivery("failed", "Некорректный ответ сервера: "+err.Error())
			c.JSON(http.StatusBadGateway, gin.H{"error": "Некорректный ответ сервера: " + err.Error()})
			return
		}
		if resp.StatusCode != http.StatusOK {
			if msg, _ := out["error"].(string); msg != "" {
				recordDelivery("failed", msg)
			} else {
				recordDelivery("failed", fmt.Sprintf("сервер вернул HTTP %d", resp.StatusCode))
			}
			c.JSON(resp.StatusCode, out)
			return
		}

		// 4. The request is fulfilled — the message reached the neighbour and
		// the book is linked/imported there. Mark handled + record delivery.
		delivered := true
		if b, ok := out["ok"].(bool); ok && !b {
			delivered = false
		}
		status := "delivered"
		if !delivered {
			status = "failed"
		}
		db.Exec(`UPDATE fed_incoming_requests SET status='done', offered_edition_id=$1,
			offered_title=$2, offered_authors=$3, delivery_status=$4,
			delivery_error='', delivered_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$5`,
			req.EditionID, offeredTitle, offeredAuthors, status, req.IncomingRequestID)

		out["source_url"] = sourceURL
		out["uid"] = uid
		if readListID != "" {
			out["read_list_id"] = readListID
		}
		c.JSON(http.StatusOK, out)
	}
}

// loadLocalBookMetadata returns the author/work/edition objects of a local
// edition, with the original identifiers, for offer to a neighbouring server.
func loadLocalBookMetadata(db *sql.DB, editionID int) (*fedBookMetadata, error) {
	var meta fedBookMetadata
	err := db.QueryRow(`
		SELECT e.id, e.uid::text, e.work_id, COALESCE(e.title,''), COALESCE(e.language,''), COALESCE(e.isbn,''),
		       COALESCE(e.ean,''), COALESCE(e.udc,''), COALESCE(e.bbk,''),
		       COALESCE(e.publisher,''), e.year, COALESCE(e.city,''), e.pages,
		       COALESCE(e.series,''), COALESCE(e.series_number,''), COALESCE(e.annotation,''),
		       COALESCE(e.source,''), e.is_complete, COALESCE(e.quality,'')
		FROM editions e WHERE e.id = $1`, editionID).Scan(
		&meta.Edition.ID, &meta.Edition.UID, &meta.Edition.WorkID, &meta.Edition.Title, &meta.Edition.Language,
		&meta.Edition.ISBN, &meta.Edition.EAN, &meta.Edition.UDC, &meta.Edition.BBK,
		&meta.Edition.Publisher, &meta.Edition.Year, &meta.Edition.City, &meta.Edition.Pages,
		&meta.Edition.Series, &meta.Edition.SeriesNumber, &meta.Edition.Annotation,
		&meta.Edition.Source, &meta.Edition.IsComplete, &meta.Edition.Quality)
	if err != nil {
		return nil, err
	}

	err = db.QueryRow(`
		SELECT id, uid::text, COALESCE(original_title,''), COALESCE(original_language,''), first_published,
		       COALESCE(work_type,''), COALESCE(annotation,''), word_count
		FROM works WHERE id = $1`, meta.Edition.WorkID).Scan(
		&meta.Work.ID, &meta.Work.UID, &meta.Work.OriginalTitle, &meta.Work.OriginalLanguage,
		&meta.Work.FirstPublished, &meta.Work.WorkType, &meta.Work.Annotation, &meta.Work.WordCount)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT p.id, p.uid::text, COALESCE(p.first_name, ''), COALESCE(p.middle_name, ''), p.last_name, wc.role
		FROM work_contributors wc JOIN persons p ON p.id = wc.person_id
		WHERE wc.work_id = $1`, meta.Work.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var a fedAuthorMeta
		if err := rows.Scan(&a.ID, &a.UID, &a.FirstName, &a.MiddleName, &a.LastName, &a.Role); err != nil {
			rows.Close()
			return nil, err
		}
		meta.Authors = append(meta.Authors, a)
	}
	rows.Close()

	rows, err = db.Query(`
		SELECT g.id, g.name FROM work_genres wg JOIN genres g ON g.id = wg.genre_id
		WHERE wg.work_id = $1`, meta.Work.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var g fedGenreMeta
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			rows.Close()
			return nil, err
		}
		meta.Genres = append(meta.Genres, g)
	}
	rows.Close()

	rows, err = db.Query(`
		SELECT ef.id, ef.format_id, f.name, COALESCE(ef.file_size, 0), COALESCE(ef.file_hash, '')
		FROM edition_files ef JOIN formats f ON f.id = ef.format_id
		WHERE ef.edition_id = $1`, editionID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var f fedFileMeta
		if err := rows.Scan(&f.ID, &f.FormatID, &f.FormatName, &f.FileSize, &f.FileHash); err != nil {
			rows.Close()
			return nil, err
		}
		meta.Files = append(meta.Files, f)
	}
	rows.Close()

	return &meta, nil
}

// fedInsertWork inserts the remote work (with the remote id unless newIDs) and
// its authors/genres. In create-new mode an existing local work with the same
// title and authors is reused. In either mode a local work that already carries
// the remote work's stable cross-server uid is reused (identity match), and the
// uid is stamped on newly-created rows so future matches are unambiguous.
func fedInsertWork(tx *sql.Tx, db *sql.DB, meta *fedBookMetadata, newIDs bool) (int, error) {
	if wid := findWorkByUID(db, meta.Work.UID); wid > 0 {
		return wid, nil
	}
	if newIDs {
		if wid := findWorkByTitleAndAuthors(db, meta.Work.OriginalTitle, fedAuthorNames(meta)); wid > 0 {
			return wid, nil
		}
		var id int
		err := tx.QueryRow(`
			INSERT INTO works (uid, original_title, original_language, first_published, work_type, annotation, word_count)
			VALUES (COALESCE($1, gen_random_uuid()), $2, $3, $4, $5, $6, $7) RETURNING id`,
			uidVal(meta.Work.UID), meta.Work.OriginalTitle, fedLanguageCode(tx, meta.Work.OriginalLanguage), meta.Work.FirstPublished,
			orDefault(meta.Work.WorkType, "novel"), strOrNil(meta.Work.Annotation), meta.Work.WordCount).Scan(&id)
		if err != nil {
			return 0, err
		}
		if err = fedInsertContributors(tx, db, id, meta, true); err != nil {
			return 0, err
		}
		if err = fedInsertGenres(tx, db, id, meta); err != nil {
			return 0, err
		}
		return id, nil
	}

	// Preserve the remote work id.
	if _, err := tx.Exec(`
		INSERT INTO works (id, uid, original_title, original_language, first_published, work_type, annotation, word_count)
		VALUES ($1, COALESCE($2, gen_random_uuid()), $3, $4, $5, $6, $7, $8)`,
		meta.Work.ID, uidVal(meta.Work.UID), meta.Work.OriginalTitle, fedLanguageCode(tx, meta.Work.OriginalLanguage), meta.Work.FirstPublished,
		orDefault(meta.Work.WorkType, "novel"), strOrNil(meta.Work.Annotation), meta.Work.WordCount); err != nil {
		return 0, err
	}
	if err := fedInsertContributors(tx, db, meta.Work.ID, meta, false); err != nil {
		return 0, err
	}
	if err := fedInsertGenres(tx, db, meta.Work.ID, meta); err != nil {
		return 0, err
	}
	fedSyncSequence(tx, "works")
	return meta.Work.ID, nil
}

// uidVal returns the uid as a SQL value, or nil so a COALESCE default
// (gen_random_uuid()) applies when the peer did not advertise one.
func uidVal(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// fedUpsertWork inserts or updates the local work row with the remote work id.
func fedUpsertWork(tx *sql.Tx, db *sql.DB, meta *fedBookMetadata, _ bool) (int, error) {
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM works WHERE id=$1)`, meta.Work.ID).Scan(&exists); err != nil {
		return 0, err
	}
	if exists {
		_, err := tx.Exec(`
			UPDATE works SET original_title=$2, original_language=$3, first_published=$4,
				work_type=$5, annotation=$6, word_count=$7
			WHERE id=$1`,
			meta.Work.ID, meta.Work.OriginalTitle, fedLanguageCode(tx, meta.Work.OriginalLanguage), meta.Work.FirstPublished,
			orDefault(meta.Work.WorkType, "novel"), strOrNil(meta.Work.Annotation), meta.Work.WordCount)
		if err != nil {
			return 0, err
		}
		return meta.Work.ID, nil
	}
	_, err := tx.Exec(`
		INSERT INTO works (id, original_title, original_language, first_published, work_type, annotation, word_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		meta.Work.ID, meta.Work.OriginalTitle, fedLanguageCode(tx, meta.Work.OriginalLanguage), meta.Work.FirstPublished,
		orDefault(meta.Work.WorkType, "novel"), strOrNil(meta.Work.Annotation), meta.Work.WordCount)
	if err != nil {
		return 0, err
	}
	fedSyncSequence(tx, "works")
	return meta.Work.ID, nil
}

// fedInsertEdition inserts the remote edition with the remote id unless newIDs
// (create-new mode generates a fresh id). A local edition that already carries
// the remote edition's stable cross-server uid is reused instead of a new row.
func fedInsertEdition(tx *sql.Tx, db *sql.DB, meta *fedBookMetadata, workID, forcedID int, newIDs bool, userID int) (int, error) {
	if eid := findEditionByUID(db, meta.Edition.UID); eid > 0 {
		return eid, nil
	}
	isbn := fedIsbn(tx, meta.Edition.ISBN, forcedID)
	lang := fedLanguageCode(tx, meta.Edition.Language)
	if newIDs {
		var id int
		err := tx.QueryRow(`
			INSERT INTO editions (uid, work_id, isbn, ean, udc, bbk, title, language, publisher, year, city, pages, series, series_number, annotation, source, is_complete, quality, upload_date, uploaded_by)
			VALUES (COALESCE($1, gen_random_uuid()),$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,NOW(),$19) RETURNING id`,
			uidVal(meta.Edition.UID), workID, isbn, strOrNil(meta.Edition.EAN), strOrNil(meta.Edition.UDC), strOrNil(meta.Edition.BBK),
			meta.Edition.Title, lang, strOrNil(meta.Edition.Publisher), meta.Edition.Year, strOrNil(meta.Edition.City),
			meta.Edition.Pages, strOrNil(meta.Edition.Series), strOrNil(meta.Edition.SeriesNumber),
			strOrNil(meta.Edition.Annotation), strOrNil(meta.Edition.Source), meta.Edition.IsComplete,
			orDefault(meta.Edition.Quality, "good"), userID).Scan(&id)
		if err != nil {
			return 0, err
		}
		return id, nil
	}
	_, err := tx.Exec(`
		INSERT INTO editions (id, uid, work_id, isbn, ean, udc, bbk, title, language, publisher, year, city, pages, series, series_number, annotation, source, is_complete, quality, upload_date, uploaded_by)
		VALUES ($1, COALESCE($2, gen_random_uuid()),$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,NOW(),$20)`,
		forcedID, uidVal(meta.Edition.UID), workID, isbn, strOrNil(meta.Edition.EAN), strOrNil(meta.Edition.UDC), strOrNil(meta.Edition.BBK),
		meta.Edition.Title, lang, strOrNil(meta.Edition.Publisher), meta.Edition.Year, strOrNil(meta.Edition.City),
		meta.Edition.Pages, strOrNil(meta.Edition.Series), strOrNil(meta.Edition.SeriesNumber),
		strOrNil(meta.Edition.Annotation), strOrNil(meta.Edition.Source), meta.Edition.IsComplete,
		orDefault(meta.Edition.Quality, "good"), userID)
	if err != nil {
		return 0, err
	}
	fedSyncSequence(tx, "editions")
	return forcedID, nil
}

// fedUpsertEdition inserts or updates the local edition row with the remote id.
func fedUpsertEdition(tx *sql.Tx, db *sql.DB, meta *fedBookMetadata, workID, userID int) (int, error) {
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM editions WHERE id=$1)`, meta.Edition.ID).Scan(&exists); err != nil {
		return 0, err
	}
	if exists {
		_, err := tx.Exec(`
			UPDATE editions SET work_id=$2, isbn=$3, ean=$4, udc=$5, bbk=$6,
				title=$7, language=$8, publisher=$9, year=$10, city=$11, pages=$12,
				series=$13, series_number=$14, annotation=$15, source=$16,
				is_complete=$17, quality=$18, uploaded_by=$19
			WHERE id=$1`,
			meta.Edition.ID, workID, fedIsbn(tx, meta.Edition.ISBN, meta.Edition.ID),
			strOrNil(meta.Edition.EAN), strOrNil(meta.Edition.UDC), strOrNil(meta.Edition.BBK),
			meta.Edition.Title, fedLanguageCode(tx, meta.Edition.Language), strOrNil(meta.Edition.Publisher),
			meta.Edition.Year, strOrNil(meta.Edition.City), meta.Edition.Pages,
			strOrNil(meta.Edition.Series), strOrNil(meta.Edition.SeriesNumber),
			strOrNil(meta.Edition.Annotation), strOrNil(meta.Edition.Source),
			meta.Edition.IsComplete, orDefault(meta.Edition.Quality, "good"), userID)
		if err != nil {
			return 0, err
		}
		return meta.Edition.ID, nil
	}
	_, err := tx.Exec(`
		INSERT INTO editions (id, work_id, isbn, ean, udc, bbk, title, language, publisher, year, city, pages, series, series_number, annotation, source, is_complete, quality, upload_date, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,NOW(),$19)`,
		meta.Edition.ID, workID, fedIsbn(tx, meta.Edition.ISBN, meta.Edition.ID),
		strOrNil(meta.Edition.EAN), strOrNil(meta.Edition.UDC), strOrNil(meta.Edition.BBK),
		meta.Edition.Title, fedLanguageCode(tx, meta.Edition.Language), strOrNil(meta.Edition.Publisher),
		meta.Edition.Year, strOrNil(meta.Edition.City), meta.Edition.Pages,
		strOrNil(meta.Edition.Series), strOrNil(meta.Edition.SeriesNumber),
		strOrNil(meta.Edition.Annotation), strOrNil(meta.Edition.Source),
		meta.Edition.IsComplete, orDefault(meta.Edition.Quality, "good"), userID)
	if err != nil {
		return 0, err
	}
	fedSyncSequence(tx, "editions")
	return meta.Edition.ID, nil
}

// fedInsertContributors links the remote authors to the given work. Authors are
// fuzzy-matched against the local persons; when not found they are created —
// with the remote id unless newIDs is set.
func fedInsertContributors(tx *sql.Tx, db *sql.DB, workID int, meta *fedBookMetadata, newIDs bool) error {
	for _, a := range meta.Authors {
		if strings.TrimSpace(a.LastName) == "" && strings.TrimSpace(a.FirstName) == "" {
			continue
		}
		role := a.Role
		if role == "" {
			role = "author"
		}
		// Identity match first: a local person that already carries this
		// author's stable cross-server uid is reused regardless of numeric id.
		if pid := findPersonByUID(db, a.UID); pid > 0 {
			if _, err := tx.Exec(`
				INSERT INTO work_contributors (work_id, person_id, role) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
				workID, pid, role); err != nil {
				return err
			}
			continue
		}
		personID := 0
		if !newIDs {
			// Overwrite semantics: a local person occupying the remote id must
			// be replaced with the remote data (otherwise the author conflict
			// reported on the 409 would persist after the overwrite).
			var exists bool
			if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM persons WHERE id=$1)`, a.ID).Scan(&exists); err == nil && exists {
				personID = a.ID
				tx.Exec(`UPDATE persons SET first_name=$2, middle_name=$3, last_name=$4 WHERE id=$1`,
					a.ID, strOrNil(a.FirstName), strOrNil(a.MiddleName), a.LastName)
			}
		}
		if personID == 0 {
			personID = fedFindFuzzyPerson(db, a)
		}
		if personID > 0 {
			if _, err := tx.Exec(`
				INSERT INTO work_contributors (work_id, person_id, role) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
				workID, personID, role); err != nil {
				return err
			}
			continue
		}
		if newIDs {
			var id int
			err := tx.QueryRow(`
				INSERT INTO persons (uid, first_name, middle_name, last_name)
				VALUES (COALESCE($1, gen_random_uuid()), $2, $3, $4) ON CONFLICT (first_name, last_name) DO NOTHING RETURNING id`,
				uidVal(a.UID), strOrNil(a.FirstName), strOrNil(a.MiddleName), a.LastName).Scan(&id)
			if err != nil {
				err = tx.QueryRow(`SELECT id FROM persons WHERE last_name=$1 AND COALESCE(first_name,'')=$2`,
					a.LastName, a.FirstName).Scan(&id)
				if err != nil {
					continue
				}
			}
			personID = id
		} else {
			var id int
			err := tx.QueryRow(`
				INSERT INTO persons (id, uid, first_name, middle_name, last_name)
				VALUES ($1, COALESCE($2, gen_random_uuid()), $3, $4, $5) ON CONFLICT (id) DO NOTHING RETURNING id`,
				a.ID, uidVal(a.UID), strOrNil(a.FirstName), strOrNil(a.MiddleName), a.LastName).Scan(&id)
			if err != nil {
				err = tx.QueryRow(`SELECT id FROM persons WHERE last_name=$1 AND COALESCE(first_name,'')=$2`,
					a.LastName, a.FirstName).Scan(&id)
				if err != nil {
					continue
				}
			}
			personID = id
			fedSyncSequence(tx, "persons")
		}
		if _, err := tx.Exec(`
			INSERT INTO work_contributors (work_id, person_id, role) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
			workID, personID, role); err != nil {
			return err
		}
	}
	return nil
}

// fedInsertGenres links the remote genres to the given work, reusing existing
// genres by name when possible.
func fedInsertGenres(tx *sql.Tx, db *sql.DB, workID int, meta *fedBookMetadata) error {
	for _, g := range meta.Genres {
		if strings.TrimSpace(g.Name) == "" {
			continue
		}
		var genreID int
		var exists bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM genres WHERE id=$1)`, g.ID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			genreID = g.ID
		} else {
			err := tx.QueryRow(`SELECT id FROM genres WHERE name=$1`, g.Name).Scan(&genreID)
			if err != nil {
				err = tx.QueryRow(`
					INSERT INTO genres (id, name) VALUES ($1, $2)
					ON CONFLICT (id) DO NOTHING RETURNING id`, g.ID, g.Name).Scan(&genreID)
				if err != nil {
					err = tx.QueryRow(`SELECT id FROM genres WHERE name=$1`, g.Name).Scan(&genreID)
					if err != nil {
						continue
					}
				}
				fedSyncSequence(tx, "genres")
			}
		}
		if _, err := tx.Exec(`
			INSERT INTO work_genres (work_id, genre_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
			workID, genreID); err != nil {
			return err
		}
	}
	return nil
}

// fedFindFuzzyPerson returns the id of a local person whose name fuzzy-matches
// (case-insensitive, ё→е, trigram) the given remote author, or 0.
func fedFindFuzzyPerson(db *sql.DB, a fedAuthorMeta) int {
	fio := strings.TrimSpace(a.LastName + " " + a.FirstName)
	if fio == "" {
		return 0
	}
	norm := strings.ToLower(strings.ReplaceAll(fio, "ё", "е"))
	var id int
	err := db.QueryRow(`
		SELECT p.id FROM persons p
		WHERE p.lower_fio % $1
		ORDER BY similarity(p.lower_fio, $1) DESC
		LIMIT 1`, norm).Scan(&id)
	if err != nil {
		return 0
	}
	return id
}

// fedAuthorNames returns the display names of the remote authors.
func fedAuthorNames(meta *fedBookMetadata) []string {
	names := make([]string, 0, len(meta.Authors))
	for _, a := range meta.Authors {
		names = append(names, strings.TrimSpace(a.LastName+" "+a.FirstName))
	}
	return names
}

// fedIsbn returns the remote ISBN when it is not already used by another local
// edition (the column has a UNIQUE constraint), otherwise nil.
func fedIsbn(tx *sql.Tx, isbn string, selfID int) interface{} {
	if strings.TrimSpace(isbn) == "" {
		return nil
	}
	var exists bool
	if err := tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM editions WHERE isbn=$1 AND id <> $2)`,
		strings.TrimSpace(isbn), selfID).Scan(&exists); err != nil {
		return nil
	}
	if exists {
		return nil
	}
	return strings.TrimSpace(isbn)
}

// fedLanguageCode normalizes a language code and falls back to a language that
// actually exists in the reference table (the columns have an FK constraint).
func fedLanguageCode(tx *sql.Tx, lang string) string {
	code := "eng"
	if lang != "" {
		code = utils.NormalizeLanguage(lang)
	}
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM languages WHERE code=$1)`, code).Scan(&exists); err != nil || !exists {
		return "eng"
	}
	return code
}

// fedWriteArchive stores the inner book content as a single-entry zip inside a
// new subdir of the bookarch directory, mirroring the standard import layout.
func fedWriteArchive(cfg *config.Config, title, formatName string, data []byte) (string, error) {
	destDir := cfg.Directories.Bookarch
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}
	subDir := getNextSubdir(destDir)
	if err := os.MkdirAll(filepath.Join(destDir, subDir), 0755); err != nil {
		return "", err
	}

	base := utils.TransliterateFilename(title)
	if base == "" {
		base = "book"
	}
	zipName := base + ".zip"
	destPath := filepath.Join(destDir, subDir, zipName)
	idx := 1
	for {
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			break
		}
		zipName = fmt.Sprintf("%s_%d.zip", base, idx)
		destPath = filepath.Join(destDir, subDir, zipName)
		idx++
	}

	zipFile, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(zipFile)
	fw, err := zw.Create(base + fedInnerExt(formatName))
	if err != nil {
		zw.Close()
		zipFile.Close()
		return "", err
	}
	if _, err := fw.Write(data); err != nil {
		zw.Close()
		zipFile.Close()
		return "", err
	}
	if err := zw.Close(); err != nil {
		zipFile.Close()
		return "", err
	}
	if err := zipFile.Close(); err != nil {
		return "", err
	}
	return destPath, nil
}

// fedInnerExt maps the format name to the file extension of the inner book
// content stored inside the archive.
func fedInnerExt(formatName string) string {
	switch formatName {
	case "FB2":
		return ".fb2"
	case "EPUB":
		return ".epub"
	case "PDF":
		return ".pdf"
	case "DOC":
		return ".doc"
	case "DOCX":
		return ".docx"
	case "MOBI":
		return ".mobi"
	case "AZW3":
		return ".azw3"
	case "DJVU":
		return ".djvu"
	case "RTF":
		return ".rtf"
	case "TXT":
		return ".txt"
	case "HTML":
		return ".html"
	case "CBZ":
		return ".cbz"
	case "CBR":
		return ".cbr"
	default:
		return ".fb2"
	}
}

// fedRelPath builds the DB-relative file path from the absolute archive path.
func fedRelPath(cfg *config.Config, archivePath string) string {
	base := filepath.Base(cfg.Directories.Bookarch)
	rel, err := filepath.Rel(cfg.Directories.Bookarch, archivePath)
	if err != nil {
		return filepath.Base(archivePath)
	}
	return filepath.Join(base, rel)
}

// fedSyncSequence advances a SERIAL sequence to the current max id so that
// explicit-id inserts do not collide with future auto-inserts.
func fedSyncSequence(tx *sql.Tx, table string) {
	tx.Exec(fmt.Sprintf(`SELECT setval('%s_id_seq', GREATEST((SELECT MAX(id) FROM %s), 1))`, table, table))
}

// strOrNil returns nil for empty strings so NULL is stored instead of ''.
func strOrNil(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// orDefault returns def when s is empty.
func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// loginToNeighbour logs in on the neighbour with the stored server-role
// credentials and returns a ready HTTP client, the neighbour base URL and the
// obtained JWT. errMsg is non-empty on failure.
func loginToNeighbour(ctx context.Context, nc *NeighbourCrypto, n federationNeighbour) (client *http.Client, base, token, errMsg string) {
	if n.username == "" {
		return nil, "", "", "Не указан логин сервера-соседа"
	}
	password, err := nc.Decrypt(n.passwordEnc)
	if err != nil {
		return nil, "", "", "Не удалось расшифровать пароль: " + err.Error()
	}

	base = strings.TrimRight(strings.TrimSpace(n.url), "/")
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}

	client = &http.Client{Timeout: 15 * time.Second}
	tr, err := federationTransport(n.serverCert, n.clientCert)
	if err != nil {
		return nil, "", "", "Ошибка TLS-конфигурации: " + err.Error()
	}
	client.Transport = tr

	// Login on the neighbour to obtain a server-role JWT.
	loginBody, _ := json.Marshal(map[string]string{
		"username": n.username,
		"password": password,
	})
	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		base+"/api/v1/auth/login", bytes.NewReader(loginBody))
	if err != nil {
		return nil, "", "", err.Error()
	}
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := client.Do(loginReq)
	if err != nil {
		return nil, "", "", "Не удалось подключиться к серверу: " + err.Error()
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Sprintf("Ошибка авторизации на сервере (%d)", loginResp.StatusCode)
	}
	var loginPayload struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginPayload); err != nil || loginPayload.Token == "" {
		return nil, "", "", "Пустой токен в ответе сервера"
	}
	return client, base, loginPayload.Token, ""
}

// queryNeighbour performs the full login + search round-trip against one peer.
func queryNeighbour(ctx context.Context, nc *NeighbourCrypto, n federationNeighbour, req ServerSearchRequest, limit int) FederationResult {
	res := FederationResult{NeighbourID: n.id, URL: n.url, Books: []ServerBook{}}

	client, base, token, errMsg := loginToNeighbour(ctx, nc, n)
	if errMsg != "" {
		res.Error = errMsg
		return res
	}

	// 2. Forward the search request.
	searchBody, _ := json.Marshal(map[string]string{
		"query":  req.Query,
		"author": req.Author,
		"title":  req.Title,
	})
	searchURL := fmt.Sprintf("%s/api/v1/server/search?limit=%d", base, limit)
	searchReq, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, bytes.NewReader(searchBody))
	if err != nil {
		res.Error = err.Error()
		return res
	}
	searchReq.Header.Set("Content-Type", "application/json")
	searchReq.Header.Set("Authorization", "Bearer "+token)
	searchResp, err := client.Do(searchReq)
	if err != nil {
		res.Error = "Ошибка поиска: " + err.Error()
		return res
	}
	defer searchResp.Body.Close()
	if searchResp.StatusCode != http.StatusOK {
		res.Error = fmt.Sprintf("Ошибка поиска (%d)", searchResp.StatusCode)
		return res
	}
	var searchPayload struct {
		Total int          `json:"total"`
		Books []ServerBook `json:"books"`
	}
	if err := json.NewDecoder(searchResp.Body).Decode(&searchPayload); err != nil {
		res.Error = "Некорректный ответ сервера: " + err.Error()
		return res
	}
	res.Total = searchPayload.Total
	res.Books = searchPayload.Books
	return res
}

// federationTransport builds an HTTP transport that trusts the neighbour's
// server certificate (self-signed) in addition to the system roots, and uses
// the stored client cert+key (combined PEM) for mutual TLS when available.
func federationTransport(serverCert, clientCert string) (*http.Transport, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if serverCert != "" {
		if !pool.AppendCertsFromPEM([]byte(serverCert)) {
			return nil, errors.New("не удалось прочитать серверный сертификат соседа")
		}
		tlsCfg.RootCAs = pool
	}
	if clientCert != "" {
		// Accept a combined PEM (certificate + private key) for mutual TLS.
		if cert, cerr := tls.X509KeyPair([]byte(clientCert), []byte(clientCert)); cerr == nil {
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
	}
	return &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsCfg,
	}, nil
}
