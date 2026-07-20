package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"libapp/src/utils"
)

func exportBooksJSON(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT 
				work_id, original_title, original_language, first_published, work_type,
				edition_id, edition_title, edition_language, isbn, publisher, year, pages,
				series, series_number, quality, authors, translators, genres
			FROM book_details
			ORDER BY original_title
		`)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		var books []map[string]interface{}
		columns, _ := rows.Columns()
		
		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}
			
			if err := rows.Scan(valuePtrs...); err != nil {
				continue
			}
			
			row := make(map[string]interface{})
			for i, col := range columns {
				row[col] = values[i]
			}
			books = append(books, row)
		}

		c.Header("Content-Disposition", "attachment; filename=library_export.json")
		c.Header("Content-Type", "application/json")
		c.JSON(http.StatusOK, gin.H{
			"exported_at": "",
			"total_books": len(books),
			"books":       books,
		})
	}
}

func exportBooksCSV(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT 
				original_title, authors, publisher, year, pages, series, series_number, isbn
			FROM book_details
			ORDER BY original_title
		`)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		csv := "Title,Authors,Publisher,Year,Pages,Series,Series Number,ISBN\n"
		
		for rows.Next() {
			var title, authors, publisher, series, isbn string
			var year, pages, seriesNum *int
			
			if err := rows.Scan(&title, &authors, &publisher, &year, &pages, &series, &seriesNum, &isbn); err != nil {
				continue
			}
			
			yearStr := ""
			if year != nil {
				yearStr = fmt.Sprintf("%d", *year)
			}
			pagesStr := ""
			if pages != nil {
				pagesStr = fmt.Sprintf("%d", *pages)
			}
			seriesNumStr := ""
			if seriesNum != nil {
				seriesNumStr = fmt.Sprintf("%d", *seriesNum)
			}
			
			csv += fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s\n", 
				escapeCSV(title), escapeCSV(authors), escapeCSV(publisher), 
				yearStr, pagesStr, escapeCSV(series), seriesNumStr, escapeCSV(isbn))
		}

		c.Header("Content-Disposition", "attachment; filename=library_export.csv")
		c.Header("Content-Type", "text/csv")
		c.String(http.StatusOK, csv)
	}
}

func escapeCSV(s string) string {
	if s == "" {
		return ""
	}
	result := ""
	for _, c := range s {
		if c == '"' {
			result += `""`
		} else if c == '\n' || c == '\r' {
			result += " "
		} else {
			result += string(c)
		}
	}
	if result != s || containsComma(s) {
		return `"` + result + `"`
	}
	return result
}

func containsComma(s string) bool {
	for _, c := range s {
		if c == ',' {
			return true
		}
	}
	return false
}

func splitAuthorString(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t'
	})
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func importBooksFromJSON(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var data struct {
			Books []map[string]interface{} `json:"books"`
		}

		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		imported := 0
		skipped := 0
		for _, book := range data.Books {
			rawTitle, _ := book["title"].(string)
			if rawTitle == "" {
				if t, ok := book["original_title"].(string); ok {
					rawTitle = t
				}
			}
			rawTitle = strings.TrimSpace(rawTitle)
			if rawTitle == "" {
				skipped++
				continue
			}

			authorsRaw, _ := book["authors"].(string)
			year := 0
			switch v := book["year"].(type) {
			case float64:
				year = int(v)
			case string:
				fmt.Sscanf(v, "%d", &year)
			}
			if year == 0 {
				switch v := book["first_published"].(type) {
				case float64:
					year = int(v)
				case string:
					fmt.Sscanf(v, "%d", &year)
				}
			}

			normTitle := normalizeQuery(rawTitle)

			// Look up existing work by normalized title, or create it.
			var workID int
			err := db.QueryRow(
				`SELECT id FROM works WHERE lower_original_title = $1 LIMIT 1`,
				normTitle,
			).Scan(&workID)
			if err == sql.ErrNoRows {
				lang := "eng"
				if l, ok := book["original_language"].(string); ok && l != "" {
					lang = l
				}
				err = db.QueryRow(`
					INSERT INTO works (original_title, original_language, first_published, work_type, lower_original_title)
					VALUES ($1, $2, $3, 'novel', $4)
					RETURNING id
				`, rawTitle, lang, nullInt(year), normTitle).Scan(&workID)
			}
			if err != nil {
				skipped++
				continue
			}

			// Link authors via persons + work_contributors.
			for _, name := range splitAuthorString(authorsRaw) {
				firstName, lastName := utils.NormalizeAuthorName(name)
				var personID int
				perr := db.QueryRow(`
					INSERT INTO persons (first_name, last_name, lower_fio)
					VALUES ($1, $2, $3)
					ON CONFLICT (first_name, last_name) DO UPDATE SET last_name = EXCLUDED.last_name
					RETURNING id
				`, nullStr(firstName), lastName, normalizeQuery(firstName+" "+lastName)).Scan(&personID)
				if perr != nil {
					continue
				}
				db.Exec(`
					INSERT INTO work_contributors (work_id, person_id, role)
					VALUES ($1, $2, 'author')
					ON CONFLICT DO NOTHING
				`, workID, personID)
			}
			imported++
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "Import completed",
			"imported": imported,
			"skipped":  skipped,
		})
	}
}

func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func nullInt(i int) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(i), Valid: i != 0}
}