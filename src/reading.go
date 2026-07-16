package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func mustGenerateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type UserBook struct {
	ID          int     `json:"id"`
	UserID      int     `json:"user_id"`
	EditionID   int     `json:"edition_id"`
	Status      string  `json:"status"`
	Review      string  `json:"review"`
	Rating      *int    `json:"rating"`
	DateStarted *string `json:"date_started"`
	DateRead    *string `json:"date_read"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		userID := 0
		if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenStr := authHeader[7:]
			claims, err := validateToken(tokenStr)
			if err == nil {
				if uid, ok := claims["user_id"].(float64); ok {
					userID = int(uid)
				}
			}
		}
		c.Set("user_id", userID)
		c.Next()
	}
}

func checkUserExists(c *gin.Context, userID int) bool {
	db, exists := c.Get("db")
	if !exists {
		return true
	}
	sqlDB, ok := db.(*sql.DB)
	if !ok {
		return true
	}
	var existsInDB bool
	err := sqlDB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&existsInDB)
	if err != nil || !existsInDB {
		return false
	}
	return true
}

func requireAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || len(authHeader) < 8 || authHeader[:7] != "Bearer " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}
		tokenStr := authHeader[7:]
		claims, err := validateToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Недействительный токен"})
			return
		}
		var userID int
		if uid, ok := claims["user_id"].(float64); ok {
			userID = int(uid)
			c.Set("user_id", userID)
		}
		if role, ok := claims["role"].(string); ok {
			c.Set("role", role)
		}
		if userID > 0 && !checkUserExists(c, userID) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Пользователь не найден или заблокирован"})
			return
		}
		c.Next()
	}
}

func adminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || len(authHeader) < 8 || authHeader[:7] != "Bearer " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}
		tokenStr := authHeader[7:]
		claims, err := validateToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Недействительный токен"})
			return
		}
		var userID int
		if uid, ok := claims["user_id"].(float64); ok {
			userID = int(uid)
			c.Set("user_id", userID)
		}
		role, _ := claims["role"].(string)
		c.Set("role", role)
		if role == "viewer" || role == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Недостаточно прав"})
			return
		}
		if userID > 0 && !checkUserExists(c, userID) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Пользователь не найден или заблокирован"})
			return
		}
		if userID > 0 {
			db, exists := c.Get("db")
			if exists {
				sqlDB, ok := db.(*sql.DB)
				if ok {
					var currentRole string
					err := sqlDB.QueryRow("SELECT role FROM users WHERE id = $1", userID).Scan(&currentRole)
					if err != nil || (currentRole != "admin" && currentRole != "editor") {
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Недостаточно прав"})
						return
					}
				}
			}
		}
		c.Next()
	}
}

func adminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Требуются права администратора"})
			return
		}
		c.Next()
	}
}

func getUserBook(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		uid := userID.(int)
		if uid == 0 {
			c.JSON(http.StatusOK, gin.H{"status": "Не заполнено"})
			return
		}
		editionID := c.Param("edition_id")
		var ub UserBook
		err := db.QueryRow(`
			SELECT id, user_id, edition_id, status::text, COALESCE(review,''),
				rating, date_started, date_read, created_at, updated_at
			FROM user_books WHERE user_id = $1 AND edition_id = $2
		`, uid, editionID).Scan(&ub.ID, &ub.UserID, &ub.EditionID, &ub.Status,
			&ub.Review, &ub.Rating, &ub.DateStarted, &ub.DateRead,
			&ub.CreatedAt, &ub.UpdatedAt)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{"status": "Не заполнено"})
			return
		}
		if err != nil {
			internalError(c, err)
			return
		}
		c.JSON(http.StatusOK, ub)
	}
}

func setUserBook(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		uid := userID.(int)
		if uid == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			return
		}
		editionID := c.Param("edition_id")
		var req struct {
			Status   string  `json:"status"`
			Review   *string `json:"review"`
			Rating   *int    `json:"rating"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		validStatuses := map[string]bool{
			"Не заполнено": true, "Прочитано": true, "Читаю": true,
			"Отложил": true, "Бросил": true,
		}
		if !validStatuses[req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
			return
		}
		_, err := db.Exec(`
			INSERT INTO user_books (user_id, edition_id, status, review, rating, date_started, date_read)
			VALUES ($1, $2, $3::user_book_status, COALESCE($4, ''), $5,
				CASE WHEN $3 = 'Читаю' AND (SELECT date_started FROM user_books WHERE user_id = $1 AND edition_id = $2) IS NULL THEN CURRENT_DATE ELSE NULL END,
				CASE WHEN $3 = 'Прочитано' AND (SELECT date_read FROM user_books WHERE user_id = $1 AND edition_id = $2) IS NULL THEN CURRENT_DATE ELSE NULL END)
			ON CONFLICT (user_id, edition_id) DO UPDATE SET
				status = $3::user_book_status,
				review = COALESCE($4, user_books.review),
				rating = CASE WHEN $5 IS NOT NULL THEN $5 ELSE user_books.rating END,
				date_started = CASE
					WHEN $3 = 'Читаю' AND user_books.date_started IS NULL THEN CURRENT_DATE
					ELSE user_books.date_started
				END,
				date_read = CASE
					WHEN $3 = 'Прочитано' AND user_books.date_read IS NULL THEN CURRENT_DATE
					ELSE user_books.date_read
				END,
				updated_at = CURRENT_TIMESTAMP
		`, uid, editionID, req.Status, req.Review, req.Rating)
		if err != nil {
			internalError(c, err)
			return
		}
		var ub UserBook
		err = db.QueryRow(`
			SELECT id, user_id, edition_id, status::text, COALESCE(review,''),
				rating, date_started, date_read, created_at, updated_at
			FROM user_books WHERE user_id = $1 AND edition_id = $2
		`, uid, editionID).Scan(&ub.ID, &ub.UserID, &ub.EditionID, &ub.Status,
			&ub.Review, &ub.Rating, &ub.DateStarted, &ub.DateRead,
			&ub.CreatedAt, &ub.UpdatedAt)
		if err != nil {
			internalError(c, err)
			return
		}
		c.JSON(http.StatusOK, ub)
	}
}

func listUserBooks(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		uid := userID.(int)
		if uid == 0 {
			c.JSON(http.StatusOK, []interface{}{})
			return
		}
		rows, err := db.Query(`
			SELECT ub.id, ub.user_id, ub.edition_id, ub.status::text,
				COALESCE(ub.review,''), ub.rating, ub.date_started, ub.date_read,
				ub.created_at, ub.updated_at
			FROM user_books ub
			WHERE ub.user_id = $1
			ORDER BY ub.updated_at DESC
		`, uid)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()
		var books []UserBook
		for rows.Next() {
			var ub UserBook
			if err := rows.Scan(&ub.ID, &ub.UserID, &ub.EditionID, &ub.Status,
				&ub.Review, &ub.Rating, &ub.DateStarted, &ub.DateRead,
				&ub.CreatedAt, &ub.UpdatedAt); err != nil {
				internalError(c, err)
				return
			}
			books = append(books, ub)
		}
		c.JSON(http.StatusOK, books)
	}
}

// ReadList types

type ReadListItem struct {
	ID         string `json:"id"`
	Listname   string `json:"listname"`
	Bookname   string `json:"bookname"`
	Author     string `json:"author"`
	Priority   int    `json:"priority"`
	AuthorID   *int   `json:"author_id"`
	BookID     *int   `json:"book_id"`
	UserID     int    `json:"user_id"`
	Comment    string `json:"comment"`
	Status     string `json:"status"`
	Deleted    bool   `json:"deleted"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	SyncedAt   string `json:"synced_at"`
	FormatName string `json:"format_name"`
	OnShelf    bool   `json:"on_shelf"`
	EditionID  *int   `json:"edition_id"`
}

type CreateReadListRequest struct {
	ID        string `json:"id"`
	Listname  string `json:"listname"`
	Bookname  string `json:"bookname"`
	Author    string `json:"author"`
	Priority  int    `json:"priority"`
	AuthorID  *int   `json:"author_id"`
	BookID    *int   `json:"book_id"`
	Comment   string `json:"comment"`
	Status    string `json:"status"`
	Deleted   bool   `json:"deleted"`
	UpdatedAt string `json:"updated_at"`
}

var validReadListStatuses = map[string]bool{
	"Не заполнено": true, "Прочитано": true, "Читаю": true,
	"Отложил": true, "Бросил": true,
}

func getReadListItems(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		uid := userID.(int)
		if uid == 0 {
			c.JSON(http.StatusOK, gin.H{"total": 0, "items": []interface{}{}})
			return
		}

		listname := c.Query("listname")
		bookname := c.Query("bookname")
		author := c.Query("author")
		comment := c.Query("comment")
		sortBy := c.DefaultQuery("sort_by", "priority")
		sortOrder := c.DefaultQuery("sort_order", "desc")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		allowedSorts := map[string]string{
			"created_at": "rl.created_at",
			"priority":   "rl.priority",
			"bookname":   "rl.bookname",
			"author":     "rl.author",
			"status":     "rl.status",
			"comment":    "rl.comment",
			"listname":   "rl.listname",
		}
		sortCol, ok := allowedSorts[sortBy]
		if !ok {
			sortCol = "rl.created_at"
		}
		if sortOrder != "desc" {
			sortOrder = "asc"
		}

		whereClause := "WHERE rl.user_id = $1 AND rl.deleted = FALSE"
		args := []interface{}{uid}
		argNum := 2

		if listname != "" {
			whereClause += fmt.Sprintf(" AND rl.listname = $%d", argNum)
			args = append(args, listname)
			argNum++
		}
		if bookname != "" {
			whereClause += fmt.Sprintf(" AND rl.bookname ILIKE $%d", argNum)
			args = append(args, "%"+bookname+"%")
			argNum++
		}
		if author != "" {
			whereClause += fmt.Sprintf(" AND rl.author ILIKE $%d", argNum)
			args = append(args, "%"+author+"%")
			argNum++
		}
		if comment != "" {
			whereClause += fmt.Sprintf(" AND rl.comment ILIKE $%d", argNum)
			args = append(args, "%"+comment+"%")
			argNum++
		}
		if rlStatus := c.Query("status"); rlStatus != "" {
			statuses := strings.Split(rlStatus, ",")
			placeholders := make([]string, len(statuses))
			for i, s := range statuses {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				placeholders[i] = fmt.Sprintf("$%d", argNum)
				args = append(args, s)
				argNum++
			}
			whereClause += fmt.Sprintf(" AND rl.status IN (%s)", strings.Join(placeholders, ","))
		}
		if exclude := c.Query("exclude_status"); exclude != "" {
			statuses := strings.Split(exclude, ",")
			placeholders := make([]string, len(statuses))
			for i, s := range statuses {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				placeholders[i] = fmt.Sprintf("$%d", argNum)
				args = append(args, s)
				argNum++
			}
			whereClause += fmt.Sprintf(" AND rl.status NOT IN (%s)", strings.Join(placeholders, ","))
		}

		// Count
		var total int
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM read_list rl %s", whereClause)
		db.QueryRow(countQuery, args...).Scan(&total)

		// Fetch
		query := fmt.Sprintf(`
			SELECT rl.id::text, rl.listname, rl.bookname, rl.author, rl.priority,
				rl.author_id, rl.book_id, rl.user_id, rl.comment, rl.status::text,
				rl.deleted, rl.created_at, rl.updated_at, rl.synced_at,
				COALESCE(f.name, '') AS format_name,
				COALESCE(e.on_shelf, false) AS on_shelf,
				e.id AS edition_id
			FROM read_list rl
			LEFT JOIN editions e ON e.id = rl.book_id
			LEFT JOIN edition_files ef ON ef.edition_id = e.id AND ef.is_primary = true
			LEFT JOIN formats f ON f.id = ef.format_id
			%s
			ORDER BY %s %s
			LIMIT $%d OFFSET $%d
		`, whereClause, sortCol, sortOrder, argNum, argNum+1)
		args = append(args, limit, offset)

		rows, err := db.Query(query, args...)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		var items = make([]ReadListItem, 0)
		for rows.Next() {
			var item ReadListItem
			var editionID sql.NullInt64
			var updatedAt, syncedAt sql.NullString
			if err := rows.Scan(&item.ID, &item.Listname, &item.Bookname, &item.Author,
				&item.Priority, &item.AuthorID, &item.BookID, &item.UserID,
				&item.Comment, &item.Status, &item.Deleted, &item.CreatedAt,
				&updatedAt, &syncedAt,
				&item.FormatName, &item.OnShelf, &editionID); err != nil {
				internalError(c, err)
				return
			}
			if editionID.Valid {
				eid := int(editionID.Int64)
				item.EditionID = &eid
			}
			if updatedAt.Valid {
				item.UpdatedAt = updatedAt.String
			}
			if syncedAt.Valid {
				item.SyncedAt = syncedAt.String
			}
			items = append(items, item)
		}

		c.JSON(http.StatusOK, gin.H{"total": total, "items": items})
	}
}

func createReadListItem(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		uid := userID.(int)
		if uid == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}

		var req CreateReadListRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

	if req.Listname == "" {
		req.Listname = "default"
	}
	if !validReadListStatuses[req.Status] {
		req.Status = "Не заполнено"
	}

	// Default priority: max existing priority for this user + 1
	if req.Priority <= 0 {
		var maxPriority int
		db.QueryRow("SELECT COALESCE(MAX(priority), 0) FROM read_list WHERE user_id = $1", uid).Scan(&maxPriority)
		req.Priority = maxPriority + 1
	}

	// Use client-provided UUID or generate one on server
	itemID := req.ID
	if itemID == "" {
		itemID = mustGenerateUUID()
	}

		var item ReadListItem
		var editionID sql.NullInt64
		var updatedAt, syncedAt sql.NullString
		err := db.QueryRow(`
			INSERT INTO read_list (id, listname, bookname, author, priority, author_id, book_id, user_id, comment, status, deleted, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10::user_book_status, $11, NOW())
			RETURNING id::text, listname, bookname, author, priority, author_id, book_id, user_id, comment, status::text, deleted, created_at, updated_at, synced_at
		`, itemID, req.Listname, req.Bookname, req.Author, req.Priority, req.AuthorID, req.BookID,
			uid, req.Comment, req.Status, req.Deleted).Scan(
			&item.ID, &item.Listname, &item.Bookname, &item.Author,
			&item.Priority, &item.AuthorID, &item.BookID, &item.UserID,
			&item.Comment, &item.Status, &item.Deleted, &item.CreatedAt,
			&updatedAt, &syncedAt)

		if err != nil {
			internalError(c, err)
			return
		}
		if updatedAt.Valid {
			item.UpdatedAt = updatedAt.String
		}
		if syncedAt.Valid {
			item.SyncedAt = syncedAt.String
		}

		// Fetch format_name and on_shelf
		if item.BookID != nil {
			db.QueryRow(`
				SELECT COALESCE(f.name, ''), COALESCE(e.on_shelf, false), e.id
				FROM editions e
				LEFT JOIN edition_files ef ON ef.edition_id = e.id AND ef.is_primary = true
				LEFT JOIN formats f ON f.id = ef.format_id
				WHERE e.id = $1
			`, *item.BookID).Scan(&item.FormatName, &item.OnShelf, &editionID)
			if editionID.Valid {
				eid := int(editionID.Int64)
				item.EditionID = &eid
			}
		}

		c.JSON(http.StatusCreated, item)
	}
}

func updateReadListItem(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		uid := userID.(int)
		if uid == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}

		id := c.Param("id")

		var req CreateReadListRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		if !validReadListStatuses[req.Status] {
			req.Status = "Не заполнено"
		}

		// Conflict detection: compare client's updated_at with server's
		if req.UpdatedAt != "" {
			var serverUpdatedAt sql.NullString
			db.QueryRow("SELECT updated_at::text FROM read_list WHERE id = $1::uuid AND user_id = $2", id, uid).Scan(&serverUpdatedAt)
			if serverUpdatedAt.Valid && isServerNewer(serverUpdatedAt.String, req.UpdatedAt) {
				// Server is newer — return 409 with current server state
				var conflictItem ReadListItem
				var editionID sql.NullInt64
				var updAt, syncedAt sql.NullString
				err := db.QueryRow(`
					SELECT rl.id::text, rl.listname, rl.bookname, rl.author, rl.priority,
						rl.author_id, rl.book_id, rl.user_id, rl.comment, rl.status::text,
						rl.deleted, rl.created_at, rl.updated_at, rl.synced_at,
						COALESCE(f.name, ''), COALESCE(e.on_shelf, false), e.id
					FROM read_list rl
					LEFT JOIN editions e ON e.id = rl.book_id
					LEFT JOIN edition_files ef ON ef.edition_id = e.id AND ef.is_primary = true
					LEFT JOIN formats f ON f.id = ef.format_id
					WHERE rl.id = $1::uuid AND rl.user_id = $2
				`, id, uid).Scan(
					&conflictItem.ID, &conflictItem.Listname, &conflictItem.Bookname, &conflictItem.Author,
					&conflictItem.Priority, &conflictItem.AuthorID, &conflictItem.BookID, &conflictItem.UserID,
					&conflictItem.Comment, &conflictItem.Status, &conflictItem.Deleted, &conflictItem.CreatedAt,
					&updAt, &syncedAt,
					&conflictItem.FormatName, &conflictItem.OnShelf, &editionID)
				if err == nil {
					if updAt.Valid {
						conflictItem.UpdatedAt = updAt.String
					}
					if syncedAt.Valid {
						conflictItem.SyncedAt = syncedAt.String
					}
					if editionID.Valid {
						eid := int(editionID.Int64)
						conflictItem.EditionID = &eid
					}
				}
				c.JSON(http.StatusConflict, gin.H{"error": "Конфликт: серверная версия новее", "server_item": conflictItem})
				return
			}
		}

		result, err := db.Exec(`
			UPDATE read_list SET
				listname = $1, bookname = $2, author = $3, priority = $4,
				author_id = $5, book_id = $6, comment = $7, status = $8::user_book_status,
				deleted = $9, updated_at = NOW()
			WHERE id = $10::uuid AND user_id = $11
		`, req.Listname, req.Bookname, req.Author, req.Priority,
			req.AuthorID, req.BookID, req.Comment, req.Status, req.Deleted, id, uid)

		if err != nil {
			internalError(c, err)
			return
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена"})
			return
		}

		var item ReadListItem
		var editionID sql.NullInt64
		var updatedAt, syncedAt sql.NullString
		err = db.QueryRow(`
			SELECT rl.id::text, rl.listname, rl.bookname, rl.author, rl.priority,
				rl.author_id, rl.book_id, rl.user_id, rl.comment, rl.status::text,
				rl.deleted, rl.created_at, rl.updated_at, rl.synced_at,
				COALESCE(f.name, ''), COALESCE(e.on_shelf, false), e.id
			FROM read_list rl
			LEFT JOIN editions e ON e.id = rl.book_id
			LEFT JOIN edition_files ef ON ef.edition_id = e.id AND ef.is_primary = true
			LEFT JOIN formats f ON f.id = ef.format_id
			WHERE rl.id = $1::uuid AND rl.user_id = $2
		`, id, uid).Scan(
			&item.ID, &item.Listname, &item.Bookname, &item.Author,
			&item.Priority, &item.AuthorID, &item.BookID, &item.UserID,
			&item.Comment, &item.Status, &item.Deleted, &item.CreatedAt,
			&updatedAt, &syncedAt,
			&item.FormatName, &item.OnShelf, &editionID)
		if err != nil {
			internalError(c, err)
			return
		}
		if updatedAt.Valid {
			item.UpdatedAt = updatedAt.String
		}
		if syncedAt.Valid {
			item.SyncedAt = syncedAt.String
		}
		if editionID.Valid {
			eid := int(editionID.Int64)
			item.EditionID = &eid
		}

		c.JSON(http.StatusOK, item)
	}
}

func deleteReadListItem(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		uid := userID.(int)
		if uid == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}

		id := c.Param("id")

		result, err := db.Exec("UPDATE read_list SET deleted = TRUE, updated_at = NOW() WHERE id = $1::uuid AND user_id = $2", id, uid)
		if err != nil {
			internalError(c, err)
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена"})
			return
		}

		var item ReadListItem
		var editionID sql.NullInt64
		var updatedAt, syncedAt sql.NullString
		err = db.QueryRow(`
			SELECT rl.id::text, rl.listname, rl.bookname, rl.author, rl.priority,
				rl.author_id, rl.book_id, rl.user_id, rl.comment, rl.status::text,
				rl.deleted, rl.created_at, rl.updated_at, rl.synced_at,
				COALESCE(f.name, ''), COALESCE(e.on_shelf, false), e.id
			FROM read_list rl
			LEFT JOIN editions e ON e.id = rl.book_id
			LEFT JOIN edition_files ef ON ef.edition_id = e.id AND ef.is_primary = true
			LEFT JOIN formats f ON f.id = ef.format_id
			WHERE rl.id = $1::uuid AND rl.user_id = $2
		`, id, uid).Scan(
			&item.ID, &item.Listname, &item.Bookname, &item.Author,
			&item.Priority, &item.AuthorID, &item.BookID, &item.UserID,
			&item.Comment, &item.Status, &item.Deleted, &item.CreatedAt,
			&updatedAt, &syncedAt,
			&item.FormatName, &item.OnShelf, &editionID)
		if updatedAt.Valid {
			item.UpdatedAt = updatedAt.String
		}
		if syncedAt.Valid {
			item.SyncedAt = syncedAt.String
		}
		if editionID.Valid {
			eid := int(editionID.Int64)
			item.EditionID = &eid
		}

		c.JSON(http.StatusOK, item)
	}
}

// isServerNewer compares two timestamp strings robustly.
// Both PG text format (2026-07-16 19:07:55.123456+03) and RFC3339 (2026-07-16T19:07:55+03:00) are accepted.
func isServerNewer(server, client string) bool {
	layouts := []string{
		// PG text format: 2026-07-16 19:08:45.123456+03 (offset without colon)
		"2006-01-02 15:04:05.999999-07",
		"2006-01-02 15:04:05.999999Z07",
		// PG text format with colon: 2026-07-16 19:08:45.123456+03:00
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05.999999Z07:00",
		// RFC3339 variants
		"2006-01-02T15:04:05.999999-07",
		"2006-01-02T15:04:05.999999Z07",
		"2006-01-02T15:04:05.999999-07:00",
		"2006-01-02T15:04:05.999999Z07:00",
		time.RFC3339,
		time.RFC3339Nano,
		// No timezone variants (unlikely but safe)
		"2006-01-02 15:04:05.999999",
		"2006-01-02T15:04:05.999999",
	}
	var sTime, cTime time.Time
	var sOk, cOk bool

	for _, layout := range layouts {
		if !sOk {
			t, err := time.Parse(layout, server)
			if err == nil {
				sTime, sOk = t, true
			}
		}
		if !cOk {
			t, err := time.Parse(layout, client)
			if err == nil {
				cTime, cOk = t, true
			}
		}
	}
	if !sOk || !cOk {
		// Fallback to string comparison if parsing fails
		return server > client
	}
	return sTime.After(cTime)
}

func getReadListNames(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		uid := userID.(int)
		if uid == 0 {
			c.JSON(http.StatusOK, []interface{}{})
			return
		}

		rows, err := db.Query(`
			SELECT DISTINCT listname FROM read_list
			WHERE user_id = $1 AND deleted = FALSE ORDER BY listname
		`, uid)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		var names []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				internalError(c, err)
				return
			}
			names = append(names, name)
		}
		c.JSON(http.StatusOK, names)
	}
}
