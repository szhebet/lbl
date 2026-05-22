package main

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		for _, book := range data.Books {
			title, _ := book["title"].(string)
			authors, _ := book["authors"].(string)
			year, _ := book["year"].(float64)
			
			if title == "" {
				continue
			}

			var workID int
			err := db.QueryRow(`
				INSERT INTO works (original_title, original_language, first_published, work_type)
				VALUES ($1, 'eng', $2, 'novel')
				ON CONFLICT (original_title) DO UPDATE SET original_title = EXCLUDED.original_title
				RETURNING id
			`, title, int(year)).Scan(&workID)

			if err == nil && authors != "" {
				db.Exec(`
					INSERT INTO work_authors (work_id, author_id, role)
					SELECT $1, a.id, 'author'
					FROM authors a WHERE a.name = $2
					ON CONFLICT DO NOTHING
				`, workID, authors)
			}
			imported++
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "Import completed",
			"imported": imported,
		})
	}
}