package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ─── Peer-library search API (role: server) ───────────────────
//
// These endpoints are called by other library servers (api_neighbours) to
// federate catalog search. Authentication is a JWT issued to a user with the
// "server" role (see serverOnlyMiddleware).

// ServerSearchRequest is the body of POST /api/v1/server/search. At least one
// of the fields must be non-empty.
type ServerSearchRequest struct {
	Query  string `json:"query"`
	Author string `json:"author"`
	Title  string `json:"title"`
}

// ServerBook is one catalog hit returned to the requesting server.
type ServerBook struct {
	WorkID    int      `json:"work_id"`
	EditionID int      `json:"edition_id"`
	Author    string   `json:"author"`
	Title     string   `json:"title"`
	Year      int      `json:"year,omitempty"`
	Formats   []string `json:"formats,omitempty"`
}

// ─── Edition metadata for federation imports ──────────────────
//
// A peer server that wants to import a book from us asks for the full
// author/work/edition objects (with the remote identifiers) so it can recreate
// them locally without changing the identifiers. The objects are also used for
// ID-conflict detection and overwrite/create-new resolution.

type fedWorkMeta struct {
	ID               int    `json:"id"`
	UID              string `json:"uid,omitempty"`
	OriginalTitle    string `json:"original_title"`
	OriginalLanguage string `json:"original_language"`
	FirstPublished   *int   `json:"first_published"`
	WorkType         string `json:"work_type"`
	Annotation       string `json:"annotation"`
	WordCount        *int   `json:"word_count"`
}

type fedEditionMeta struct {
	ID           int    `json:"id"`
	UID          string `json:"uid,omitempty"`
	WorkID       int    `json:"work_id"`
	ISBN         string `json:"isbn"`
	EAN          string `json:"ean"`
	UDC          string `json:"udc"`
	BBK          string `json:"bbk"`
	Title        string `json:"title"`
	Language     string `json:"language"`
	Publisher    string `json:"publisher"`
	Year         *int   `json:"year"`
	City         string `json:"city"`
	Pages        *int   `json:"pages"`
	Series       string `json:"series"`
	SeriesNumber string `json:"series_number"`
	Annotation   string `json:"annotation"`
	Source       string `json:"source"`
	IsComplete   bool   `json:"is_complete"`
	Quality      string `json:"quality"`
}

type fedAuthorMeta struct {
	ID         int    `json:"id"`
	UID        string `json:"uid,omitempty"`
	FirstName  string `json:"first_name"`
	MiddleName string `json:"middle_name"`
	LastName   string `json:"last_name"`
	Role       string `json:"role"`
}

type fedGenreMeta struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type fedFileMeta struct {
	ID         int    `json:"id"`
	FormatID   int    `json:"format_id"`
	FormatName string `json:"format_name"`
	FileSize   int64  `json:"file_size"`
	FileHash   string `json:"file_hash"`
}

type fedBookMetadata struct {
	Work    fedWorkMeta     `json:"work"`
	Edition fedEditionMeta  `json:"edition"`
	Authors []fedAuthorMeta `json:"authors"`
	Genres  []fedGenreMeta  `json:"genres"`
	Files   []fedFileMeta   `json:"files"`
}

func serverPing() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"server":  "Home Library Manager",
			"api":     "v1",
			"version": currentDBVersion,
		})
	}
}

// serverBookMetadata GET /api/v1/server/metadata/:edition_id (role: server).
// Returns the full author/work/edition objects of the given edition so a
// federating peer can recreate them locally without changing identifiers.
func serverBookMetadata(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var meta fedBookMetadata

		err := db.QueryRow(`
			SELECT e.id, e.uid::text, e.work_id, COALESCE(e.title,''), COALESCE(e.language,''), COALESCE(e.isbn,''),
			       COALESCE(e.ean,''), COALESCE(e.udc,''), COALESCE(e.bbk,''),
			       COALESCE(e.publisher,''), e.year, COALESCE(e.city,''), e.pages,
			       COALESCE(e.series,''), COALESCE(e.series_number,''), COALESCE(e.annotation,''),
			       COALESCE(e.source,''), e.is_complete, COALESCE(e.quality,'')
			FROM editions e WHERE e.id = $1`, id).Scan(
			&meta.Edition.ID, &meta.Edition.UID, &meta.Edition.WorkID, &meta.Edition.Title, &meta.Edition.Language,
			&meta.Edition.ISBN, &meta.Edition.EAN, &meta.Edition.UDC, &meta.Edition.BBK,
			&meta.Edition.Publisher, &meta.Edition.Year, &meta.Edition.City, &meta.Edition.Pages,
			&meta.Edition.Series, &meta.Edition.SeriesNumber, &meta.Edition.Annotation,
			&meta.Edition.Source, &meta.Edition.IsComplete, &meta.Edition.Quality)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Edition not found"})
				return
			}
			internalError(c, err)
			return
		}

		err = db.QueryRow(`
			SELECT id, uid::text, COALESCE(original_title,''), COALESCE(original_language,''), first_published,
			       COALESCE(work_type,''), COALESCE(annotation,''), word_count
			FROM works WHERE id = $1`, meta.Edition.WorkID).Scan(
			&meta.Work.ID, &meta.Work.UID, &meta.Work.OriginalTitle, &meta.Work.OriginalLanguage,
			&meta.Work.FirstPublished, &meta.Work.WorkType, &meta.Work.Annotation, &meta.Work.WordCount)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Work not found"})
				return
			}
			internalError(c, err)
			return
		}

		rows, err := db.Query(`
			SELECT p.id, p.uid::text, COALESCE(p.first_name, ''), COALESCE(p.middle_name, ''), p.last_name, wc.role
			FROM work_contributors wc
			JOIN persons p ON p.id = wc.person_id
			WHERE wc.work_id = $1`, meta.Work.ID)
		if err != nil {
			internalError(c, err)
			return
		}
		for rows.Next() {
			var a fedAuthorMeta
			if err := rows.Scan(&a.ID, &a.UID, &a.FirstName, &a.MiddleName, &a.LastName, &a.Role); err != nil {
				rows.Close()
				internalError(c, err)
				return
			}
			meta.Authors = append(meta.Authors, a)
		}
		rows.Close()

		rows, err = db.Query(`
			SELECT g.id, g.name FROM work_genres wg JOIN genres g ON g.id = wg.genre_id
			WHERE wg.work_id = $1`, meta.Work.ID)
		if err != nil {
			internalError(c, err)
			return
		}
		for rows.Next() {
			var g fedGenreMeta
			if err := rows.Scan(&g.ID, &g.Name); err != nil {
				rows.Close()
				internalError(c, err)
				return
			}
			meta.Genres = append(meta.Genres, g)
		}
		rows.Close()

		rows, err = db.Query(`
			SELECT ef.id, ef.format_id, f.name, COALESCE(ef.file_size, 0), COALESCE(ef.file_hash, '')
			FROM edition_files ef JOIN formats f ON f.id = ef.format_id
			WHERE ef.edition_id = $1`, id)
		if err != nil {
			internalError(c, err)
			return
		}
		for rows.Next() {
			var f fedFileMeta
			if err := rows.Scan(&f.ID, &f.FormatID, &f.FormatName, &f.FileSize, &f.FileHash); err != nil {
				rows.Close()
				internalError(c, err)
				return
			}
			meta.Files = append(meta.Files, f)
		}
		rows.Close()

		c.JSON(http.StatusOK, meta)
	}
}

func serverSearchBooks(db *sql.DB) gin.HandlerFunc {
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

		// Mandatory audit log for every federation search request coming from a
		// neighbour server. Written regardless of log_level so admins can always
		// see which server asked what.
		uid, _ := c.Get("user_id")
		role, _ := c.Get("role")
		log.Printf("[FEDERATION SEARCH] from %s (user_id=%v, role=%v): query=%q author=%q title=%q limit=%d",
			c.ClientIP(), uid, role, req.Query, req.Author, req.Title, limit)

		// Search across the book_details view (same matching style as the
		// public /books/search endpoint: normalized lower_ fields, ё→е).
		where := " WHERE 1=1"
		args := []interface{}{}
		idx := 1
		if req.Author != "" {
			where += fmt.Sprintf(` AND LOWER(COALESCE(authors,'')) LIKE $%d`, idx)
			args = append(args, "%"+normalizeQuery(req.Author)+"%")
			idx++
		}
		if req.Title != "" {
			where += fmt.Sprintf(` AND (LOWER(original_title) LIKE $%d OR LOWER(COALESCE(edition_title,'')) LIKE $%d)`, idx, idx+1)
			args = append(args, "%"+normalizeQuery(req.Title)+"%", "%"+normalizeQuery(req.Title)+"%")
			idx += 2
		}
		if req.Query != "" {
			where += fmt.Sprintf(` AND (LOWER(original_title) LIKE $%d OR LOWER(COALESCE(edition_title,'')) LIKE $%d OR LOWER(COALESCE(authors,'')) LIKE $%d)`, idx, idx+1, idx+2)
			args = append(args,
				"%"+normalizeQuery(req.Query)+"%",
				"%"+normalizeQuery(req.Query)+"%",
				"%"+normalizeQuery(req.Query)+"%")
			idx += 3
		}

		var total int
		if err := db.QueryRow("SELECT COUNT(*) FROM book_details"+where, args...).Scan(&total); err != nil {
			internalError(c, err)
			return
		}

		sqlQuery := `
			SELECT work_id, edition_id,
			       COALESCE(authors, ''),
			       COALESCE(NULLIF(edition_title, ''), original_title),
			       COALESCE(year, first_published),
			       COALESCE(available_formats, '')
			FROM book_details` + where +
			" ORDER BY original_title LIMIT $" + strconv.Itoa(idx) +
			" OFFSET $" + strconv.Itoa(idx+1)
		args = append(args, limit, 0)

		rows, err := db.Query(sqlQuery, args...)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		books := make([]ServerBook, 0, total)
		for rows.Next() {
			var b ServerBook
			var year sql.NullInt64
			var formats string
			if err := rows.Scan(&b.WorkID, &b.EditionID, &b.Author, &b.Title, &year, &formats); err != nil {
				internalError(c, err)
				return
			}
			if year.Valid && year.Int64 > 0 {
				b.Year = int(year.Int64)
			}
			for _, f := range strings.Split(formats, ",") {
				if f = strings.TrimSpace(f); f != "" {
					b.Formats = append(b.Formats, f)
				}
			}
			books = append(books, b)
		}

		c.JSON(http.StatusOK, gin.H{
			"total": total,
			"limit": limit,
			"books": books,
		})
	}
}

// ─── Book offer from a neighbour (role: server) ────────────────
//
// POST /api/v1/server/book/offer receives a book that was offered back in
// response to one of this server's pending requests. The body is a JSON
// reference (serverOffer):
//   - source_url   the offering server's public URL (how to reach it back)
//   - uid          the uid of the original request this offer fulfils
//   - read_list_id the requester's stable read_list id (preferred for linking)
//   - edition_id   the offering server's edition id
//   - metadata     author / work / edition objects with original ids
//
// The offered book is pulled from the offering server and imported into the
// local catalog preserving the original identifiers. If a copy is already
// present (by id or content hash) it is simply linked to the originating user
// request instead of being reported as an error.

func serverOfferBook(db *sql.DB, nc *NeighbourCrypto) gin.HandlerFunc {
	return func(c *gin.Context) {
		var offer serverOffer
		if err := c.ShouldBindJSON(&offer); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
			return
		}
		if offer.EditionID <= 0 || offer.Metadata.Edition.ID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный offer"})
			return
		}
		cfg := getConfig(c)
		meta := &offer.Metadata

		// 1. A copy of the offered book may already be on this server. The
		// preferred identity is the offered edition's stable cross-server uid:
		// it is unambiguous even when numeric auto-increment ids collide across
		// servers. The content hash is a secondary fallback for peers that do
		// not yet advertise a uid. When a local edition matches, the request is
		// fulfilled by linking that existing copy.
		editionUID := meta.Edition.UID
		if eid := findEditionByUID(db, editionUID); eid > 0 {
			linkOfferToReadList(db, offer.ReadListID, offer.UID, offer.SourceURL, eid)
			c.JSON(http.StatusOK, gin.H{
				"ok":         true,
				"duplicate":  true,
				"message":    "Запрос выполнен: книга привязана к запросу (она уже была в библиотеке)",
				"work_id":    meta.Work.ID,
				"edition_id": eid,
				"title":      meta.Edition.Title,
				"authors":    fedAuthorsDisplay(meta),
			})
			return
		}
		if h := offeredFileHash(meta); h != "" {
			if eid := findEditionIDByHash(db, h); eid > 0 {
				linkOfferToReadList(db, offer.ReadListID, offer.UID, offer.SourceURL, eid)
				c.JSON(http.StatusOK, gin.H{
					"ok":         true,
					"duplicate":  true,
					"message":    "Запрос выполнен: книга привязана к запросу (она уже была в библиотеке)",
					"work_id":    meta.Work.ID,
					"edition_id": eid,
					"title":      meta.Edition.Title,
					"authors":    fedAuthorsDisplay(meta),
				})
				return
			}
		}

		// 2. Pull the book from the offering server (federation-search style).
		data, remoteMeta, errMsg := pullOfferedBook(db, nc, offer)
		if errMsg != "" {
			c.JSON(http.StatusBadGateway, gin.H{"error": errMsg})
			return
		}
		innerHash, formatName, formatID, err := fedAnalyzeBook(data, db)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось распознать формат книги: " + err.Error()})
			return
		}

		// 3. Already present by content hash → link the existing copy.
		eid := 0
		duplicate := false
		if dup := findDuplicateByHash(db, innerHash); dup != nil {
			duplicate = true
			eid = findEditionIDByHash(db, innerHash)
		} else {
			// 4. Import preserving the original ids; if an id collision on the
			// author/work blocks that, fall back to fresh identifiers.
			var workID, editionID int
			workID, editionID, _, err = fedCreateLocal(db, cfg, remoteMeta, data, innerHash, formatName, formatID, c.GetInt("user_id"), false)
			if err != nil {
				workID, editionID, _, err = fedCreateLocal(db, cfg, remoteMeta, data, innerHash, formatName, formatID, c.GetInt("user_id"), true)
				if err != nil {
					adminInternalError(c, err)
					return
				}
			}
			_ = workID
			eid = editionID
		}

		if eid > 0 {
			linkOfferToReadList(db, offer.ReadListID, offer.UID, offer.SourceURL, eid)
		}

		log.Printf("[FED OFFER] stored book for request %q from %q (edition <> %d): edition=%d",
			offer.UID, offer.SourceURL, meta.Work.ID, eid)
		c.JSON(http.StatusOK, gin.H{
			"ok":         true,
			"duplicate":  duplicate,
			"edition_id": eid,
			"work_id":    remoteMeta.Work.ID,
			"title":      meta.Edition.Title,
			"authors":    fedAuthorsDisplay(meta),
		})
	}
}

// offeredFileHash returns the first non-empty content hash advertised by the
// offered edition, which is the cross-server identity of the book.
func offeredFileHash(meta *fedBookMetadata) string {
	for _, f := range meta.Files {
		if f.FileHash != "" {
			return f.FileHash
		}
	}
	return ""
}

// findEditionIDByHash returns the id of the edition that owns a file with the
// given content hash ("" or 0 when absent).
func findEditionIDByHash(db *sql.DB, hashStr string) int {
	var id int
	if err := db.QueryRow(`SELECT edition_id FROM edition_files WHERE file_hash = $1 LIMIT 1`, hashStr).Scan(&id); err != nil {
		return 0
	}
	return id
}

// findEditionByUID returns the id of a local edition carrying the given stable
// cross-server uid ("", 0 when absent).
func findEditionByUID(db *sql.DB, uid string) int {
	if uid == "" {
		return 0
	}
	var id int
	if err := db.QueryRow(`SELECT id FROM editions WHERE uid::text = $1 LIMIT 1`, uid).Scan(&id); err != nil {
		return 0
	}
	return id
}

// findWorkByUID returns the id of a local work carrying the given stable
// cross-server uid ("", 0 when absent).
func findWorkByUID(db *sql.DB, uid string) int {
	if uid == "" {
		return 0
	}
	var id int
	if err := db.QueryRow(`SELECT id FROM works WHERE uid::text = $1 LIMIT 1`, uid).Scan(&id); err != nil {
		return 0
	}
	return id
}

// findPersonByUID returns the id of a local person carrying the given stable
// cross-server uid ("", 0 when absent).
func findPersonByUID(db *sql.DB, uid string) int {
	if uid == "" {
		return 0
	}
	var id int
	if err := db.QueryRow(`SELECT id FROM persons WHERE uid::text = $1 LIMIT 1`, uid).Scan(&id); err != nil {
		return 0
	}
	return id
}

// linkOfferToReadList attaches the given edition to the user request (read_list
// row) this offer fulfils. The local read_list id is preferred — it is stable
// and survives re-approvals that regenerate the request uid. When it is not
// carried (older peer), the uid is resolved against fed_outgoing_requests.
// Silently does nothing when neither resolves. The approved fed_outgoing_requests
// row is marked fulfilled (with the offering server's URL) so the requester's
// admin sees «книга получена» in the delivery status.
func linkOfferToReadList(db *sql.DB, readListID, uid, sourceURL string, editionID int) {
	if editionID <= 0 {
		return
	}
	var rlID string
	if readListID != "" {
		rlID = readListID
	} else if uid != "" {
		if err := db.QueryRow(`
			SELECT read_list_id FROM fed_outgoing_requests
			WHERE uid::text = $1 ORDER BY id DESC LIMIT 1`, uid).Scan(&rlID); err != nil || rlID == "" {
			return
		}
	} else {
		return
	}
	db.Exec(`UPDATE read_list SET book_id = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1::uuid`, rlID, editionID)
	if sourceURL != "" {
		db.Exec(`UPDATE fed_outgoing_requests SET fulfilled_at = CURRENT_TIMESTAMP, fulfilled_by_url = $2
			WHERE read_list_id = $1 AND status = 'approved'`, rlID, sourceURL)
	}
}

// pullOfferedBook connects to the offering server (found by matching the offer's
// source_url against api_neighbours.url), fetches the authoritative metadata and
// downloads the stored archive of the offered edition.
func pullOfferedBook(db *sql.DB, nc *NeighbourCrypto, offer serverOffer) ([]byte, *fedBookMetadata, string) {
	var n federationNeighbour
	err := db.QueryRow(`
		SELECT id, url, server_cert, client_cert, username, password_encrypted
		FROM api_neighbours WHERE url = $1`, offer.SourceURL).
		Scan(&n.id, &n.url, &n.serverCert, &n.clientCert, &n.username, &n.passwordEnc)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, "Сервер, приславший эту книгу, не найден в списке соседей"
		}
		return nil, nil, "Не удалось найти соседа: " + err.Error()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client, base, token, errMsg := loginToNeighbour(ctx, nc, n)
	if errMsg != "" {
		return nil, nil, errMsg
	}

	remoteMeta, err := fetchFedMetadata(ctx, client, base, token, offer.EditionID)
	if err != nil {
		return nil, nil, "Не удалось получить данные книги: " + err.Error()
	}
	data, err := downloadFedEdition(ctx, client, base, token, offer.EditionID)
	if err != nil {
		return nil, nil, "Не удалось скачать книгу: " + err.Error()
	}
	return data, remoteMeta, ""
}

// serverMetadataLike is a type alias used only to document server
// role endpoints; it is not otherwise referenced.

