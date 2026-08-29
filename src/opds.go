package main

import (
	"database/sql"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func setupOPDSRoutes(api *gin.RouterGroup, db *sql.DB) {
	opds := api.Group("/opds")
	{
		opds.GET("/catalog.xml", opdsRoot(db))
		opds.GET("/latest.xml", opdsLatest(db))
		opds.GET("/genres.xml", opdsGenres(db))
		opds.GET("/genre/:id.xml", opdsGenreBooks(db))
		opds.GET("/search.xml", opdsSearch(db))
		opds.GET("/book/:id", opdsDownload(db))
	}
}

func opdsRoot(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/atom+xml; charset=utf-8")
		c.Header("Access-Control-Allow-Origin", "*")

		baseURL := getBaseURL(c)

		xml := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opds="http://opds-spec.org/2010/catalog">
  <title>Библиотека</title>
  <id>` + baseURL + `/opds/catalog.xml</id>
  <updated>` + getCurrentTime() + `</updated>
  <author><name>Library</name></author>

  <link rel="self" href="` + baseURL + `/opds/catalog.xml" type="application/atom+xml;profile=opds-catalog"/>

  <entry>
    <title>Последние книги</title>
    <id>` + baseURL + `/opds/latest.xml</id>
    <updated>` + getCurrentTime() + `</updated>
    <link href="` + baseURL + `/opds/latest.xml" type="application/atom+xml;profile=opds-catalog" rel="subsection"/>
  </entry>

  <entry>
    <title>По жанрам</title>
    <id>` + baseURL + `/opds/genres.xml</id>
    <updated>` + getCurrentTime() + `</updated>
    <link href="` + baseURL + `/opds/genres.xml" type="application/atom+xml;profile=opds-catalog" rel="subsection"/>
  </entry>

  <entry>
    <title>Поиск</title>
    <id>tag:search</id>
    <updated>` + getCurrentTime() + `</updated>
    <link href="` + baseURL + `/opds/search.xml" type="application/atom+xml;profile=opds-catalog" rel="search" title="Search"/>
  </entry>
</feed>`
		c.String(http.StatusOK, xml)
	}
}

func opdsLatest(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/atom+xml; charset=utf-8")
		c.Header("Access-Control-Allow-Origin", "*")

		limit := strconv.Itoa(parseLimit(c.DefaultQuery("limit", "50"), 50))
		offset := c.DefaultQuery("offset", "0")

		baseURL := getBaseURL(c)

		rows, err := db.Query(`
			SELECT 
				e.id, e.title, COALESCE(e.upload_date, e.created_at) as upload_date,
				COALESCE(STRING_AGG(DISTINCT p.last_name || ' ' || COALESCE(p.first_name, ''), '; '), '') as authors,
				COALESCE(STRING_AGG(DISTINCT g.name, '; '), '') as genres,
				e.cover_path,
				ef.file_path
			FROM editions e
			LEFT JOIN works w ON w.id = e.work_id
			LEFT JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
			LEFT JOIN persons p ON p.id = wc.person_id
			LEFT JOIN work_genres wg ON wg.work_id = w.id
			LEFT JOIN genres g ON g.id = wg.genre_id
			LEFT JOIN edition_files ef ON ef.edition_id = e.id AND ef.is_primary = true
			GROUP BY e.id, e.title, e.upload_date, e.created_at, e.cover_path, ef.file_path
			ORDER BY COALESCE(e.upload_date, e.created_at) DESC
			LIMIT $1 OFFSET $2
		`, limit, offset)
		if err != nil {
			c.String(http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		xml := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opds="http://opds-spec.org/2010/catalog">
  <title>Последние книги</title>
  <id>` + baseURL + `/opds/latest.xml</id>
  <updated>` + getCurrentTime() + `</updated>
  <link rel="self" href="` + baseURL + `/opds/latest.xml" type="application/atom+xml;profile=opds-catalog"/>
  <link rel="start" href="` + baseURL + `/opds/catalog.xml" type="application/atom+xml;profile=opds-catalog"/>
  <link rel="up" href="` + baseURL + `/opds/catalog.xml" type="application/atom+xml;profile=opds-catalog"/>
`

		for rows.Next() {
			var id int
			var title, uploadDate, authors, genres, coverPath, filePath sql.NullString

			if err := rows.Scan(&id, &title, &uploadDate, &authors, &genres, &coverPath, &filePath); err != nil {
				continue
			}

			entryID := fmt.Sprintf("urn:uuid:%d", id)
			updatedTime := formatDate(uploadDate.String)

			category := genres.String
			if category == "" {
				category = "Other"
			}

			entry := fmt.Sprintf(`
  <entry>
    <title>%s</title>
    <id>%s</id>
    <updated>%s</updated>
    <author><name>%s</name></author>
    <category label="genre">%s</category>
    <summary>%s</summary>
`, escapeXML(title.String), entryID, updatedTime, escapeXML(authors.String), escapeXML(category), escapeXML(title.String))

			if coverPath.Valid && coverPath.String != "" {
				entry += fmt.Sprintf(`    <link href="%s/opds/cover/%d" type="image/jpeg" rel="image"/>
`, baseURL, id)
			}

			if filePath.Valid && filePath.String != "" {
				entry += fmt.Sprintf(`    <link href="%s/opds/book/%d" type="application/zip" rel="http://opds-spec.org/acquisition"/>
`, baseURL, id)
			}

			entry += `  </entry>`
			xml += entry
		}

		xml += `</feed>`
		c.String(http.StatusOK, xml)
	}
}

func opdsGenres(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/atom+xml; charset=utf-8")
		c.Header("Access-Control-Allow-Origin", "*")

		baseURL := getBaseURL(c)

		rows, err := db.Query(`
			SELECT g.id, g.name, COUNT(w.id) as book_count
			FROM genres g
			LEFT JOIN work_genres wg ON wg.genre_id = g.id
			LEFT JOIN works w ON w.id = wg.work_id AND w.id IN (SELECT DISTINCT work_id FROM editions)
			GROUP BY g.id, g.name
			ORDER BY g.name
		`)
		if err != nil {
			c.String(http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		xml := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opds="http://opds-spec.org/2010/catalog">
  <title>Жанры</title>
  <id>` + baseURL + `/opds/genres.xml</id>
  <updated>` + getCurrentTime() + `</updated>
  <link rel="self" href="` + baseURL + `/opds/genres.xml" type="application/atom+xml;profile=opds-catalog"/>
  <link rel="start" href="` + baseURL + `/opds/catalog.xml" type="application/atom+xml;profile=opds-catalog"/>
  <link rel="up" href="` + baseURL + `/opds/catalog.xml" type="application/atom+xml;profile=opds-catalog"/>
`

		for rows.Next() {
			var id int
			var name string
			var count int

			if err := rows.Scan(&id, &name, &count); err != nil {
				continue
			}

			xml += fmt.Sprintf(`
  <entry>
    <title>%s (%d)</title>
    <id>`+baseURL+`/opds/genre/%d.xml</id>
    <updated>%s</updated>
    <link href="`+baseURL+`/opds/genre/%d.xml" type="application/atom+xml" rel="subsection"/>
  </entry>
`, escapeXML(name), count, id, getCurrentTime(), id)
		}

		xml += `</feed>`
		c.String(http.StatusOK, xml)
	}
}

func opdsGenreBooks(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/atom+xml; charset=utf-8")
		c.Header("Access-Control-Allow-Origin", "*")

		genreID := c.Param("id")
		baseURL := getBaseURL(c)

		var genreName string
		err := db.QueryRow("SELECT name FROM genres WHERE id = $1", genreID).Scan(&genreName)
		if err != nil {
			c.String(http.StatusNotFound, "Genre not found")
			return
		}

		rows, err := db.Query(`
			SELECT 
				e.id, e.title, e.updated_at,
				COALESCE(STRING_AGG(DISTINCT p.last_name || ' ' || COALESCE(p.first_name, ''), '; '), '') as authors,
				ef.file_path
			FROM editions e
			JOIN works w ON w.id = e.work_id
			JOIN work_genres wg ON wg.work_id = w.id
			LEFT JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
			LEFT JOIN persons p ON p.id = wc.person_id
			LEFT JOIN edition_files ef ON ef.edition_id = e.id AND ef.is_primary = true
			WHERE wg.genre_id = $1
			GROUP BY e.id, e.title, e.updated_at, ef.file_path
			ORDER BY e.title
		`, genreID)
		if err != nil {
			c.String(http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opds="http://opds-spec.org/2010/catalog">
  <title>%s</title>
  <id>`+baseURL+`/opds/genre/%s.xml</id>
  <updated>%s</updated>
  <link rel="self" href="`+baseURL+`/opds/genre/%s.xml" type="application/atom+xml;profile=opds-catalog"/>
  <link rel="start" href="`+baseURL+`/opds/catalog.xml" type="application/atom+xml;profile=opds-catalog"/>
  <link rel="up" href="`+baseURL+`/opds/genres.xml" type="application/atom+xml;profile=opds-catalog"/>
`, escapeXML(genreName), genreID, getCurrentTime(), genreID)

		for rows.Next() {
			var id int
			var title, updated, authors, filePath sql.NullString

			if err := rows.Scan(&id, &title, &updated, &authors, &filePath); err != nil {
				continue
			}

			entry := fmt.Sprintf(`
  <entry>
    <title>%s</title>
    <id>urn:uuid:%d</id>
    <updated>%s</updated>
    <author><name>%s</name></author>
    <category label="genre">%s</category>
`, escapeXML(title.String), id, formatDate(updated.String), escapeXML(authors.String), escapeXML(genreName))

			if filePath.Valid && filePath.String != "" {
				entry += fmt.Sprintf(`    <link href="%s/opds/book/%d" type="application/zip" rel="http://opds-spec.org/acquisition"/>
`, baseURL, id)
			}

			entry += `  </entry>`
			xml += entry
		}

		xml += `</feed>`
		c.String(http.StatusOK, xml)
	}
}

func opdsSearch(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/atom+xml; charset=utf-8")
		c.Header("Access-Control-Allow-Origin", "*")

		query := c.Query("q")
		if query == "" {
			c.String(http.StatusBadRequest, "Search query required")
			return
		}

		baseURL := getBaseURL(c)

		searchQ := "%" + strings.ToLower(strings.ReplaceAll(query, "ё", "е")) + "%"
		rows, err := db.Query(`
			SELECT 
				e.id, e.title, e.updated_at,
				COALESCE(STRING_AGG(DISTINCT p.last_name || ' ' || COALESCE(p.first_name, ''), '; '), '') as authors,
				COALESCE(STRING_AGG(DISTINCT g.name, '; '), '') as genres,
				ef.file_path
			FROM editions e
			LEFT JOIN works w ON w.id = e.work_id
			LEFT JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
			LEFT JOIN persons p ON p.id = wc.person_id
			LEFT JOIN work_genres wg ON wg.work_id = w.id
			LEFT JOIN genres g ON g.id = wg.genre_id
			LEFT JOIN edition_files ef ON ef.edition_id = e.id AND ef.is_primary = true
			WHERE w.lower_original_title LIKE $1 OR e.lower_title LIKE $1 OR p.lower_fio LIKE $1
			GROUP BY e.id, e.title, e.updated_at, ef.file_path
			ORDER BY e.title
			LIMIT 100
		`, searchQ)
		if err != nil {
			c.String(http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opds="http://opds-spec.org/2010/catalog">
  <title>Поиск: %s</title>
  <id>`+baseURL+`/opds/search.xml?q=%s</id>
  <updated>%s</updated>
  <link rel="self" href="`+baseURL+`/opds/search.xml?q=%s" type="application/atom+xml;profile=opds-catalog"/>
  <link rel="start" href="`+baseURL+`/opds/catalog.xml" type="application/atom+xml;profile=opds-catalog"/>
`, escapeXML(query), url.QueryEscape(query), getCurrentTime(), url.QueryEscape(query))

		for rows.Next() {
			var id int
			var title, updated, authors, genres, filePath sql.NullString

			if err := rows.Scan(&id, &title, &updated, &authors, &genres, &filePath); err != nil {
				continue
			}

			entry := fmt.Sprintf(`
  <entry>
    <title>%s</title>
    <id>urn:uuid:%d</id>
    <updated>%s</updated>
    <author><name>%s</name></author>
    <category label="genre">%s</category>
`, escapeXML(title.String), id, formatDate(updated.String), escapeXML(authors.String), escapeXML(genres.String))

			if filePath.Valid && filePath.String != "" {
				entry += fmt.Sprintf(`    <link href="%s/opds/book/%d" type="application/zip" rel="http://opds-spec.org/acquisition"/>
`, baseURL, id)
			}

			entry += `  </entry>`
			xml += entry
		}

		xml += `</feed>`
		c.String(http.StatusOK, xml)
	}
}

func opdsDownload(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		editionID := c.Param("id")

		var filePath, title string
		err := db.QueryRow(`
			SELECT ef.file_path, e.title 
			FROM edition_files ef 
			JOIN editions e ON e.id = ef.edition_id 
			WHERE ef.edition_id = $1 AND ef.is_primary = true
		`, editionID).Scan(&filePath, &title)

		if err != nil {
			c.String(http.StatusNotFound, "File not found")
			return
		}

		fullPath := filepath.Join(".", filePath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			c.String(http.StatusNotFound, "File not found on disk")
			return
		}

		safeName := sanitizeFilename(title) + ".zip"
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", safeName, url.QueryEscape(safeName)))
		c.Header("Content-Type", "application/zip")
		c.File(fullPath)
	}
}

func getBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	host := html.EscapeString(c.Request.Host)
	return scheme + "://" + host
}

func getCurrentTime() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func formatDate(dateStr string) string {
	if dateStr == "" {
		return time.Now().UTC().Format("2006-01-02T15:04:05Z")
	}
	if len(dateStr) >= 10 {
		return dateStr[:10] + "T12:00:00Z"
	}
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
