package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

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
	OriginalTitle    string `json:"original_title"`
	OriginalLanguage string `json:"original_language"`
	FirstPublished   *int   `json:"first_published"`
	WorkType         string `json:"work_type"`
	Annotation       string `json:"annotation"`
	WordCount        *int   `json:"word_count"`
}

type fedEditionMeta struct {
	ID           int    `json:"id"`
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
			SELECT e.id, e.work_id, COALESCE(e.title,''), COALESCE(e.language,''), COALESCE(e.isbn,''),
			       COALESCE(e.ean,''), COALESCE(e.udc,''), COALESCE(e.bbk,''),
			       COALESCE(e.publisher,''), e.year, COALESCE(e.city,''), e.pages,
			       COALESCE(e.series,''), COALESCE(e.series_number,''), COALESCE(e.annotation,''),
			       COALESCE(e.source,''), e.is_complete, COALESCE(e.quality,'')
			FROM editions e WHERE e.id = $1`, id).Scan(
			&meta.Edition.ID, &meta.Edition.WorkID, &meta.Edition.Title, &meta.Edition.Language,
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
			SELECT id, COALESCE(original_title,''), COALESCE(original_language,''), first_published,
			       COALESCE(work_type,''), COALESCE(annotation,''), word_count
			FROM works WHERE id = $1`, meta.Edition.WorkID).Scan(
			&meta.Work.ID, &meta.Work.OriginalTitle, &meta.Work.OriginalLanguage,
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
			SELECT p.id, COALESCE(p.first_name, ''), COALESCE(p.middle_name, ''), p.last_name, wc.role
			FROM work_contributors wc
			JOIN persons p ON p.id = wc.person_id
			WHERE wc.work_id = $1`, meta.Work.ID)
		if err != nil {
			internalError(c, err)
			return
		}
		for rows.Next() {
			var a fedAuthorMeta
			if err := rows.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName, &a.Role); err != nil {
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
