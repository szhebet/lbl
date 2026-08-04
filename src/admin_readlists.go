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

// AdminReadListItem is a read-list entry of another user, visible only to
// users who are a parent of the list owner (via user_parents).
type AdminReadListItem struct {
	ID         string `json:"id"`
	UserID     int    `json:"user_id"`
	Username   string `json:"username"`
	Listname   string `json:"listname"`
	Bookname   string `json:"bookname"`
	Author     string `json:"author"`
	Priority   int    `json:"priority"`
	Status     string `json:"status"`
	Comment    string `json:"comment"`
	CreatedAt  string `json:"created_at"`
	OnShelf    bool   `json:"on_shelf"`
	BookID     *int   `json:"book_id,omitempty"`
	EditionID  *int   `json:"edition_id,omitempty"`
	CreatedBy  int    `json:"created_by"`
	CreatedByU string `json:"created_by_username"`
}

// parentOfOwnerExpr returns a SQL condition that is true when the current user
// is a parent (in user_parents) of the given read_list owner.
const parentOfOwnerCondition = `EXISTS (SELECT 1 FROM user_parents up WHERE up.parent_id = %d AND up.user_id = rl.user_id)`

// canManageReadListItem reports whether uid is a parent of the owner of the
// read-list item with the given UUID.
func canManageReadListItem(db *sql.DB, uid int, itemID string) (bool, error) {
	var ok bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM read_list rl
			JOIN user_parents up ON up.user_id = rl.user_id AND up.parent_id = $1
			WHERE rl.id = $2::uuid AND rl.deleted = FALSE
		)`, uid, itemID).Scan(&ok)
	if err != nil {
		return false, err
	}
	return ok, nil
}

// readListFilter holds the filter criteria parsed from query parameters. It is
// shared by the list endpoint and the bulk actions (shelf / delete / status).
type readListFilter struct {
	UserIDs    []int
	Listnames  []string
	Listname   string
	Bookname   string
	Author     string
	Statuses   []string
}

func parseReadListFilter(c *gin.Context) readListFilter {
	var f readListFilter

	for _, part := range strings.Split(c.Query("user_ids"), ",") {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && id > 0 {
			f.UserIDs = append(f.UserIDs, id)
		}
	}
	for _, part := range strings.Split(c.Query("listnames"), ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			f.Listnames = append(f.Listnames, part)
		}
	}
	for _, part := range strings.Split(c.Query("statuses"), ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			f.Statuses = append(f.Statuses, part)
		}
	}
	f.Listname = c.Query("listname")
	f.Bookname = c.Query("bookname")
	f.Author = c.Query("author")
	return f
}

// buildWhereClause renders the WHERE clause for the shared filter plus the
// parent-of-owner condition. Returns the clause (starting with "WHERE"),
// the positional arguments and the next free argument number.
func (f readListFilter) buildWhereClause(uid int) (string, []interface{}, int) {
	whereClause := fmt.Sprintf(`WHERE rl.deleted = FALSE AND %s`, fmt.Sprintf(parentOfOwnerCondition, uid))
	args := []interface{}{}
	argNum := 1

	if len(f.UserIDs) > 0 {
		whereClause += fmt.Sprintf(" AND rl.user_id = ANY($%d::int[])", argNum)
		args = append(args, f.UserIDs)
		argNum++
	}
	if len(f.Listnames) > 0 {
		whereClause += fmt.Sprintf(" AND rl.listname = ANY($%d::text[])", argNum)
		args = append(args, f.Listnames)
		argNum++
	}
	if f.Listname != "" {
		whereClause += fmt.Sprintf(" AND rl.listname ILIKE $%d", argNum)
		args = append(args, "%"+f.Listname+"%")
		argNum++
	}
	if f.Bookname != "" {
		whereClause += fmt.Sprintf(" AND rl.bookname ILIKE $%d", argNum)
		args = append(args, "%"+f.Bookname+"%")
		argNum++
	}
	if f.Author != "" {
		whereClause += fmt.Sprintf(" AND rl.author ILIKE $%d", argNum)
		args = append(args, "%"+f.Author+"%")
		argNum++
	}
	if len(f.Statuses) > 0 {
		whereClause += fmt.Sprintf(" AND rl.status::text = ANY($%d::text[])", argNum)
		args = append(args, f.Statuses)
		argNum++
	}
	return whereClause, args, argNum
}

// linkBookToMatchingChildren attaches the given edition (book_id) to every
// non-deleted read_list row of the current user's children whose normalized
// bookname and author match the supplied values. authorID is only set when
// non-nil (so a title-only match keeps its existing author link).
func linkBookToMatchingChildren(db *sql.DB, uid int, bookID *int, authorID *int, bookname, author string) {
	if bookID == nil || *bookID <= 0 {
		return
	}
	// Match on normalized (lowercase + ё→е) bookname and author.
	db.Exec(`
		UPDATE read_list rl SET book_id = $1, author_id = COALESCE($2, author_id), updated_at = NOW()
		WHERE rl.deleted = FALSE
		  AND rl.user_id IN (SELECT user_id FROM user_parents WHERE parent_id = $3)
		  AND LOWER(REPLACE(rl.bookname, 'ё', 'е')) = LOWER(REPLACE($4, 'ё', 'е'))
		  AND LOWER(REPLACE(rl.author, 'ё', 'е')) = LOWER(REPLACE($5, 'ё', 'е'))
	`, *bookID, authorID, uid, bookname, author)
}

func adminListReadLists(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)
		if uid == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}

		limit := parseLimit(c.DefaultQuery("limit", "50"), 50)
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		flt := parseReadListFilter(c)
		whereClause, args, argNum := flt.buildWhereClause(uid)

		baseQuery := `
			FROM read_list rl
			JOIN users u ON u.id = rl.user_id
			LEFT JOIN users cu ON cu.id = rl.created_by
			LEFT JOIN editions e ON e.id = rl.book_id
			%s
		`
		fromClause := fmt.Sprintf(baseQuery, whereClause)

		var total int
		if err := db.QueryRow("SELECT COUNT(*) "+fromClause, args...).Scan(&total); err != nil {
			adminInternalError(c, err)
			return
		}

		query := fmt.Sprintf(`
			SELECT rl.id::text, rl.user_id, u.username, rl.listname, rl.bookname,
				rl.author, rl.priority, rl.status::text, rl.comment, rl.created_at,
				COALESCE(e.on_shelf, false), rl.book_id, rl.created_by, cu.username
			%s
			ORDER BY rl.created_at DESC
			LIMIT $%d OFFSET $%d
		`, fromClause, argNum, argNum+1)
		queryArgs := append(args, limit, offset)

		rows, err := db.Query(query, queryArgs...)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()

		items := make([]AdminReadListItem, 0)
		for rows.Next() {
			var item AdminReadListItem
			var bookID sql.NullInt64
			var createdBy sql.NullInt64
			var createdByU sql.NullString
			if err := rows.Scan(&item.ID, &item.UserID, &item.Username, &item.Listname,
				&item.Bookname, &item.Author, &item.Priority, &item.Status, &item.Comment,
				&item.CreatedAt, &item.OnShelf, &bookID, &createdBy, &createdByU); err != nil {
				adminInternalError(c, err)
				return
			}
			if bookID.Valid {
				id := int(bookID.Int64)
				item.BookID = &id
				item.EditionID = &id
			}
			if createdBy.Valid {
				item.CreatedBy = int(createdBy.Int64)
			}
			item.CreatedByU = createdByU.String
			items = append(items, item)
		}

		c.JSON(http.StatusOK, gin.H{"total": total, "items": items})
	}
}

func adminUpdateReadListItem(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)
		id := c.Param("id")

		allowed, err := canManageReadListItem(db, uid, id)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		if !allowed {
			c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена"})
			return
		}

		var req CreateReadListRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		if !validReadListStatuses[req.Status] {
			req.Status = "Не заполнено"
		}
		if !validLookingFor[req.LookingFor] {
			req.LookingFor = "Нет"
		}

		result, err := db.Exec(`
			UPDATE read_list SET
				listname = $1, bookname = $2, author = $3, priority = $4,
				author_id = $5, book_id = $6, comment = $7, status = $8::user_book_status,
				looking_for = $9, updated_at = NOW()
			WHERE id = $10::uuid AND user_id IN (
				SELECT user_id FROM user_parents WHERE parent_id = $11
			)
		`, req.Listname, req.Bookname, req.Author, req.Priority,
			req.AuthorID, req.BookID, req.Comment, req.Status, req.LookingFor, id, uid)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена"})
			return
		}

		// Sync status to the owner's user_books if the item has a book_id
		if req.BookID != nil && *req.BookID > 0 {
			db.Exec(`INSERT INTO user_books (user_id, edition_id, status)
				SELECT rl.user_id, rl.book_id, $1::user_book_status
				FROM read_list rl WHERE rl.id = $2::uuid
				ON CONFLICT (user_id, edition_id) DO UPDATE SET
					status = $1::user_book_status, updated_at = CURRENT_TIMESTAMP
			`, req.Status, id)
		}

		// A parent loading a book attaches it to all matching entries of children
		linkBookToMatchingChildren(db, uid, req.BookID, req.AuthorID, req.Bookname, req.Author)

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func adminDeleteReadListItem(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)
		id := c.Param("id")

		allowed, err := canManageReadListItem(db, uid, id)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		if !allowed {
			c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена"})
			return
		}

		result, err := db.Exec(`
			UPDATE read_list SET deleted = TRUE, updated_at = NOW()
			WHERE id = $1::uuid AND user_id IN (
				SELECT user_id FROM user_parents WHERE parent_id = $2
			)
		`, id, uid)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Запись не найдена"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// AdminChild is a user for whom the current user is a parent.
type AdminChild struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// adminListChildren returns the list of users where the current user is a
// parent (in user_parents), used for the children filter / list creation.
func adminListChildren(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)
		if uid == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}

		rows, err := db.Query(`
			SELECT u.id, u.username
			FROM user_parents up
			JOIN users u ON u.id = up.user_id
			WHERE up.parent_id = $1
			ORDER BY u.username
		`, uid)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()

		children := make([]AdminChild, 0)
		for rows.Next() {
			var ch AdminChild
			if err := rows.Scan(&ch.ID, &ch.Username); err != nil {
				adminInternalError(c, err)
				return
			}
			children = append(children, ch)
		}
		c.JSON(http.StatusOK, gin.H{"items": children})
	}
}

// adminListReadListNames returns the union of all distinct list names of the
// current user's children, used for the multi-select list filter.
func adminListReadListNames(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)
		if uid == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}

		rows, err := db.Query(`
			SELECT DISTINCT rl.listname
			FROM read_list rl
			JOIN user_parents up ON up.user_id = rl.user_id AND up.parent_id = $1
			WHERE rl.deleted = FALSE AND rl.listname <> ''
			ORDER BY rl.listname
		`, uid)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()

		names := make([]string, 0)
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				adminInternalError(c, err)
				return
			}
			names = append(names, name)
		}
		c.JSON(http.StatusOK, gin.H{"items": names})
	}
}

// adminCreateReadListItems creates read-list entries for several children at
// once. Each user_id must be a child of the current user; a list entry is
// created for every selected child.
func adminCreateReadListItems(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)
		if uid == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}

		var req struct {
			UserIDs  []int  `json:"user_ids"`
			Listname string `json:"listname"`
			Bookname string `json:"bookname"`
			Author   string `json:"author"`
			AuthorID *int   `json:"author_id"`
			BookID   *int   `json:"book_id"`
			Comment  string `json:"comment"`
			Status   string `json:"status"`
			Priority int    `json:"priority"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		if len(req.UserIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Выберите хотя бы одного ребёнка"})
			return
		}
		if req.Listname == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите название списка"})
			return
		}
		if !validReadListStatuses[req.Status] {
			req.Status = "Не заполнено"
		}

		// Verify all selected users are children of the current user.
		for _, childID := range req.UserIDs {
			var ok bool
			err := db.QueryRow(`
				SELECT EXISTS (
					SELECT 1 FROM user_parents WHERE parent_id = $1 AND user_id = $2
				)`, uid, childID).Scan(&ok)
			if err != nil {
				adminInternalError(c, err)
				return
			}
			if !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "Пользователь не является вашим ребёнком"})
				return
			}
		}

		created := make([]ReadListItem, 0, len(req.UserIDs))
		for _, childID := range req.UserIDs {
			var item ReadListItem
			err := db.QueryRow(`
				INSERT INTO read_list (id, listname, bookname, author, priority, user_id, comment, status, author_id, book_id, created_by, updated_at)
				VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7::user_book_status, $8, $9, $10, NOW())
				RETURNING id::text, created_at
			`, req.Listname, req.Bookname, req.Author, req.Priority, childID, req.Comment, req.Status,
				req.AuthorID, req.BookID, uid).Scan(&item.ID, &item.CreatedAt)
			if err != nil {
				adminInternalError(c, err)
				return
			}
			item.Listname = req.Listname
			item.Bookname = req.Bookname
			item.Author = req.Author
			item.Priority = req.Priority
			item.UserID = childID
			item.Comment = req.Comment
			item.Status = req.Status
			item.AuthorID = req.AuthorID
			item.BookID = req.BookID
			created = append(created, item)
		}

		// A parent loading a book attaches it to all matching entries of children
		linkBookToMatchingChildren(db, uid, req.BookID, req.AuthorID, req.Bookname, req.Author)

		c.JSON(http.StatusCreated, gin.H{"items": created})
	}
}

// readListBulkResult is returned by the bulk read-list actions.
type readListBulkResult struct {
	Total  int `json:"total"`
	Edited int `json:"edited"`
}

// adminBulkShelfReadLists puts on the shelf every edition attached to a
// read-list entry of the current user's children that matches the shared
// filter. Only entries with a linked book_id are affected.
func adminBulkShelfReadLists(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)
		if uid == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}

		flt := parseReadListFilter(c)
		whereClause, args, _ := flt.buildWhereClause(uid)

		query := fmt.Sprintf(`
			SELECT DISTINCT e.id
			FROM read_list rl
			JOIN editions e ON e.id = rl.book_id
			%s
		`, whereClause)
		rows, err := db.Query(query, args...)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()

		cfg := getConfig(c)
		var editionIDs []string
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				adminInternalError(c, err)
				return
			}
			editionIDs = append(editionIDs, strconv.Itoa(id))
		}

		for _, editionID := range editionIDs {
			if err := extractBookForShelf(db, editionID, cfg); err != nil {
				log.Printf("Shelf extract warning for edition %s: %v", editionID, err)
			}
			db.Exec("DELETE FROM shelf_tokens WHERE edition_id = $1", editionID)
			db.QueryRow("INSERT INTO shelf_tokens (token, edition_id) VALUES (gen_random_uuid()::text, $1) RETURNING token", editionID)
			db.Exec("UPDATE editions SET on_shelf = true, shelf_order = COALESCE(shelf_order, 0) + 1 WHERE id = $1", editionID)
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "total": len(editionIDs)})
	}
}

// adminBulkDeleteReadLists soft-deletes every read-list entry matching the
// shared filter that was created by the current user. Entries created by
// other users are left untouched.
func adminBulkDeleteReadLists(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)
		if uid == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}

		flt := parseReadListFilter(c)
		whereClause, args, argNum := flt.buildWhereClause(uid)

		query := fmt.Sprintf(`
			UPDATE read_list rl SET deleted = TRUE, updated_at = NOW()
			%s AND rl.created_by = $%d
		`, whereClause, argNum)
		args = append(args, uid)
		result, err := db.Exec(query, args...)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		edited, _ := result.RowsAffected()

		c.JSON(http.StatusOK, gin.H{"ok": true, "edited": edited})
	}
}

// adminBulkStatusReadLists sets the reading status on every read-list entry
// matching the shared filter. The status is validated against the allowed set.
func adminBulkStatusReadLists(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)
		if uid == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		if !validReadListStatuses[req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Недопустимый статус"})
			return
		}

		flt := parseReadListFilter(c)
		whereClause, args, argNum := flt.buildWhereClause(uid)

		query := fmt.Sprintf(`
			UPDATE read_list rl SET status = $%d::user_book_status, updated_at = NOW()
			%s
		`, argNum, whereClause)
		args = append(args, req.Status)
		result, err := db.Exec(query, args...)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		edited, _ := result.RowsAffected()

		// Sync to the owner's user_books for entries that have a linked book
		db.Exec(`
			INSERT INTO user_books (user_id, edition_id, status)
			SELECT rl.user_id, rl.book_id, $1::user_book_status
			FROM read_list rl
			JOIN user_parents up ON up.user_id = rl.user_id AND up.parent_id = $2
			WHERE rl.deleted = FALSE AND rl.book_id IS NOT NULL
			ON CONFLICT (user_id, edition_id) DO UPDATE SET
				status = EXCLUDED.status, updated_at = CURRENT_TIMESTAMP
		`, req.Status, uid)

		c.JSON(http.StatusOK, gin.H{"ok": true, "edited": edited})
	}
}
