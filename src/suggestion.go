package main

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type SuggestionItem struct {
	ID            int     `json:"id"`
	ReadListID    string  `json:"read_list_id"`
	Listname      string  `json:"listname"`
	Bookname      string  `json:"bookname"`
	Author        string  `json:"author"`
	Priority      int     `json:"priority"`
	UserID        int     `json:"user_id"`
	Username      string  `json:"username"`
	LookingFor    string  `json:"looking_for"`
	Comment       string  `json:"comment"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	HasSuggestion bool    `json:"has_suggestion"`
	SuggestionID  *int    `json:"suggestion_id,omitempty"`
	SuggEditionID *int    `json:"sugg_edition_id,omitempty"`
	SuggHidden    *bool   `json:"sugg_hidden,omitempty"`
	EditionTitle  *string `json:"edition_title,omitempty"`
	EditionAuthor *string `json:"edition_author,omitempty"`
}

type CreateSuggestionsRequest struct {
	ReadListID string                         `json:"read_list_id" binding:"required"`
	Items      []CreateSuggestionsRequestItem `json:"items"`
}

type CreateSuggestionsRequestItem struct {
	ID        *int `json:"id,omitempty"`
	EditionID *int `json:"edition_id"`
	Hidden    bool `json:"hidden"`
	Delete    bool `json:"_delete,omitempty"`
}

func adminListSuggestions(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)

		hiddenFilter := c.DefaultQuery("hidden", "no")
		userFilter := c.Query("user")
		booknameFilter := c.Query("bookname")
		authorFilter := c.Query("author")

		whereClause := "WHERE rl.looking_for != 'Нет' AND rl.deleted = FALSE"
		args := []interface{}{}
		argNum := 1

		if booknameFilter != "" {
			whereClause += fmt.Sprintf(" AND rl.bookname ILIKE $%d", argNum)
			args = append(args, "%"+booknameFilter+"%")
			argNum++
		}
		if authorFilter != "" {
			whereClause += fmt.Sprintf(" AND rl.author ILIKE $%d", argNum)
			args = append(args, "%"+authorFilter+"%")
			argNum++
		}
		if userFilter != "" {
			whereClause += fmt.Sprintf(" AND u.username ILIKE $%d", argNum)
			args = append(args, "%"+userFilter+"%")
			argNum++
		}

		if hiddenFilter == "no" {
			whereClause += fmt.Sprintf(" AND NOT EXISTS (SELECT 1 FROM suggestions s WHERE s.read_list_id = rl.id AND s.user_id = $%d AND s.hidden = TRUE)", argNum)
			args = append(args, uid)
			argNum++
		} else if hiddenFilter == "yes" {
			whereClause += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM suggestions s WHERE s.read_list_id = rl.id AND s.user_id = $%d AND s.hidden = TRUE)", argNum)
			args = append(args, uid)
			argNum++
		}

		var total int
		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM read_list rl JOIN users u ON u.id = rl.user_id %s`, whereClause)
		db.QueryRow(countQuery, args...).Scan(&total)

		query := fmt.Sprintf(`
			SELECT rl.id::text, rl.listname, rl.bookname, rl.author, rl.priority,
				rl.user_id, u.username, rl.looking_for, rl.comment, rl.status::text,
				rl.created_at, rl.updated_at,
				s.id AS sugg_id, s.edition_id, s.hidden,
				COALESCE(e.title, '') AS edition_title,
				COALESCE(STRING_AGG(DISTINCT p.last_name || ' ' || COALESCE(p.first_name, ''), ', '), '') AS edition_author
			FROM read_list rl
			JOIN users u ON u.id = rl.user_id
			LEFT JOIN suggestions s ON s.read_list_id = rl.id AND s.user_id = $%d
			LEFT JOIN editions e ON e.id = s.edition_id
			LEFT JOIN works w ON w.id = e.work_id
			LEFT JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
			LEFT JOIN persons p ON p.id = wc.person_id
			%s
			GROUP BY rl.id, rl.listname, rl.bookname, rl.author, rl.priority,
				rl.user_id, u.username, rl.looking_for, rl.comment, rl.status,
				rl.created_at, rl.updated_at,
				s.id, s.edition_id, s.hidden, e.title
			ORDER BY rl.priority DESC, rl.created_at DESC
		`, argNum, whereClause)

		queryArgs := append(args, uid)
		rows, err := db.Query(query, queryArgs...)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()

		items := make([]SuggestionItem, 0)
		for rows.Next() {
			var item SuggestionItem
			var suggID, editionID sql.NullInt64
			var suggHidden sql.NullBool
			var editionTitle, editionAuthor sql.NullString

			if err := rows.Scan(&item.ReadListID, &item.Listname, &item.Bookname, &item.Author,
				&item.Priority, &item.UserID, &item.Username, &item.LookingFor,
				&item.Comment, &item.Status, &item.CreatedAt, &item.UpdatedAt,
				&suggID, &editionID, &suggHidden,
				&editionTitle, &editionAuthor); err != nil {
				adminInternalError(c, err)
				return
			}

			if suggID.Valid {
				item.HasSuggestion = true
				id := int(suggID.Int64)
				item.SuggestionID = &id
				if editionID.Valid {
					eid := int(editionID.Int64)
					item.SuggEditionID = &eid
				}
				if suggHidden.Valid {
					h := suggHidden.Bool
					item.SuggHidden = &h
				}
				if editionTitle.Valid {
					item.EditionTitle = &editionTitle.String
				}
				if editionAuthor.Valid {
					item.EditionAuthor = &editionAuthor.String
				}
			}

			items = append(items, item)
		}

		c.JSON(http.StatusOK, gin.H{"total": total, "items": items})
	}
}

func adminGetReadListSuggestions(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)
		readListID := c.Param("id")

		rows, err := db.Query(`
			SELECT s.id, s.read_list_id::text, s.edition_id, s.hidden, s.created_at, s.updated_at,
				COALESCE(e.title, '') AS edition_title,
				COALESCE(STRING_AGG(DISTINCT p.last_name || ' ' || COALESCE(p.first_name, ''), ', '), '') AS edition_author
			FROM suggestions s
			LEFT JOIN editions e ON e.id = s.edition_id
			LEFT JOIN works w ON w.id = e.work_id
			LEFT JOIN work_contributors wc ON wc.work_id = w.id AND wc.role = 'author'
			LEFT JOIN persons p ON p.id = wc.person_id
			WHERE s.read_list_id = $1::uuid AND s.user_id = $2
			GROUP BY s.id, s.read_list_id, s.edition_id, s.hidden, s.created_at, s.updated_at, e.title
			ORDER BY s.created_at DESC
		`, readListID, uid)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()

		type SuggestionEntry struct {
			ID            int     `json:"id"`
			ReadListID    string  `json:"read_list_id"`
			EditionID     *int    `json:"edition_id"`
			Hidden        bool    `json:"hidden"`
			CreatedAt     string  `json:"created_at"`
			UpdatedAt     string  `json:"updated_at"`
			EditionTitle  string  `json:"edition_title"`
			EditionAuthor string  `json:"edition_author"`
		}

		items := make([]SuggestionEntry, 0)
		for rows.Next() {
			var item SuggestionEntry
			var editionID sql.NullInt64
			var editionTitle, editionAuthor sql.NullString

			if err := rows.Scan(&item.ID, &item.ReadListID, &editionID, &item.Hidden,
				&item.CreatedAt, &item.UpdatedAt, &editionTitle, &editionAuthor); err != nil {
				adminInternalError(c, err)
				return
			}
			if editionID.Valid {
				eid := int(editionID.Int64)
				item.EditionID = &eid
			}
			if editionTitle.Valid {
				item.EditionTitle = editionTitle.String
			}
			if editionAuthor.Valid {
				item.EditionAuthor = editionAuthor.String
			}
			items = append(items, item)
		}

		c.JSON(http.StatusOK, items)
	}
}

func adminCreateSuggestions(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)

		var req CreateSuggestionsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		// Verify read_list exists
		var rlExists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM read_list WHERE id = $1::uuid)", req.ReadListID).Scan(&rlExists)
		if err != nil || !rlExists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Запись списка чтения не найдена"})
			return
		}

		for _, item := range req.Items {
			if item.Delete && item.ID != nil {
				_, err := db.Exec("DELETE FROM suggestions WHERE id = $1 AND user_id = $2", *item.ID, uid)
				if err != nil {
					adminInternalError(c, err)
					return
				}
				continue
			}

			if item.ID != nil {
				_, err := db.Exec(`
					UPDATE suggestions SET edition_id = $1, hidden = $2, updated_at = NOW()
					WHERE id = $3 AND user_id = $4
				`, item.EditionID, item.Hidden, *item.ID, uid)
				if err != nil {
					adminInternalError(c, err)
					return
				}
			} else {
				// For create with no edition: use separate handling to avoid unique violation
				if item.EditionID == nil {
					// Check if a suggestion without edition already exists for this user+read_list
					var existingID int
					err := db.QueryRow(`
						SELECT id FROM suggestions
						WHERE read_list_id = $1::uuid AND user_id = $2 AND edition_id IS NULL
					`, req.ReadListID, uid).Scan(&existingID)
					if err == nil {
						// Update existing
						_, err = db.Exec(`UPDATE suggestions SET hidden = $1, updated_at = NOW() WHERE id = $2`, item.Hidden, existingID)
						if err != nil {
							adminInternalError(c, err)
							return
						}
						continue
					}
				}
				_, err := db.Exec(`
					INSERT INTO suggestions (read_list_id, edition_id, user_id, hidden)
					VALUES ($1::uuid, $2, $3, $4)
					ON CONFLICT DO NOTHING
				`, req.ReadListID, item.EditionID, uid, item.Hidden)
				if err != nil {
					adminInternalError(c, err)
					return
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Сохранено"})
	}
}

func adminDeleteSuggestion(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)
		id := c.Param("id")

		result, err := db.Exec("DELETE FROM suggestions WHERE id = $1 AND user_id = $2", id, uid)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Удалено"})
	}
}

func adminImportAndSuggest(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := getConfig(c)
		userID := c.GetInt("user_id")

		readListID := c.PostForm("read_list_id")
		if readListID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "read_list_id is required"})
			return
		}

		var rlExists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM read_list WHERE id = $1::uuid)", readListID).Scan(&rlExists)
		if err != nil || !rlExists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Запись списка чтения не найдена"})
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 50<<20)
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
			return
		}
		defer file.Close()

		filename := header.Filename
		ext := strings.ToLower(filepathExt(filename))
		if !isSupportedFile(filename) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported format: " + ext})
			return
		}

		data, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
			return
		}

		res, importErr := importFile(filename, data, ext, db, cfg, userID)
		if importErr != nil {
			var dup *duplicateInfo
			if errors.As(importErr, &dup) {
				c.JSON(http.StatusConflict, gin.H{
					"error":     fmt.Sprintf("Книга уже существует в библиотеке: %s — %s", dup.authors, dup.title),
					"duplicate": true,
					"title":     dup.title,
					"authors":   dup.authors,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": importErr.Error()})
			return
		}

		// Check if suggestion with this edition already exists
		var existingID int
		err = db.QueryRow(`
			SELECT id FROM suggestions
			WHERE read_list_id = $1::uuid AND user_id = $2 AND edition_id = $3
		`, readListID, userID, res.editionID).Scan(&existingID)
		if err == nil {
			_, err = db.Exec(`UPDATE suggestions SET hidden = false, updated_at = NOW() WHERE id = $1`, existingID)
		} else {
			_, err = db.Exec(`
				INSERT INTO suggestions (read_list_id, edition_id, user_id, hidden)
				VALUES ($1::uuid, $2, $3, false)
			`, readListID, res.editionID, userID)
		}
		if err != nil {
			adminInternalError(c, err)
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":    "Книга импортирована и предложена",
			"work_id":    res.workID,
			"edition_id": res.editionID,
			"file_path":  res.filePath,
			"title":      res.title,
		})
	}
}

func filepathExt(name string) string {
	low := strings.ToLower(name)
	if strings.HasSuffix(low, ".fb2.zip") || strings.HasSuffix(low, ".pdf.zip") ||
		strings.HasSuffix(low, ".doc.zip") || strings.HasSuffix(low, ".docx.zip") ||
		strings.HasSuffix(low, ".epub.zip") {
		return ".zip"
	}
	for i := len(low) - 1; i >= 0; i-- {
		if low[i] == '.' {
			return low[i:]
		}
	}
	return ""
}
