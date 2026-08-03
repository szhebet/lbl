package main

import (
	"database/sql"
	"fmt"
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

func adminListReadLists(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, _ := c.Get("user_id")
		uid := currentUserID.(int)
		if uid == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}

		userIDsFilter := c.Query("user_ids")
		listnameFilter := c.Query("listname")
		booknameFilter := c.Query("bookname")
		authorFilter := c.Query("author")
		limit := parseLimit(c.DefaultQuery("limit", "50"), 50)
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		whereClause := fmt.Sprintf(`WHERE rl.deleted = FALSE AND %s`, fmt.Sprintf(parentOfOwnerCondition, uid))
		args := []interface{}{}
		argNum := 1

		if userIDsFilter != "" {
			var ids []int
			for _, part := range strings.Split(userIDsFilter, ",") {
				id, err := strconv.Atoi(strings.TrimSpace(part))
				if err == nil && id > 0 {
					ids = append(ids, id)
				}
			}
			if len(ids) > 0 {
				whereClause += fmt.Sprintf(" AND rl.user_id = ANY($%d::int[])", argNum)
				args = append(args, ids)
				argNum++
			}
		}
		if listnameFilter != "" {
			whereClause += fmt.Sprintf(" AND rl.listname ILIKE $%d", argNum)
			args = append(args, "%"+listnameFilter+"%")
			argNum++
		}
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

		baseQuery := `
			FROM read_list rl
			JOIN users u ON u.id = rl.user_id
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
				COALESCE(e.on_shelf, false), rl.book_id
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
			if err := rows.Scan(&item.ID, &item.UserID, &item.Username, &item.Listname,
				&item.Bookname, &item.Author, &item.Priority, &item.Status, &item.Comment,
				&item.CreatedAt, &item.OnShelf, &bookID); err != nil {
				adminInternalError(c, err)
				return
			}
			if bookID.Valid {
				id := int(bookID.Int64)
				item.BookID = &id
				item.EditionID = &id
			}
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
				INSERT INTO read_list (id, listname, bookname, author, priority, user_id, comment, status, updated_at)
				VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7::user_book_status, NOW())
				RETURNING id::text, created_at
			`, req.Listname, req.Bookname, req.Author, req.Priority, childID, req.Comment, req.Status).Scan(&item.ID, &item.CreatedAt)
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
			created = append(created, item)
		}

		c.JSON(http.StatusCreated, gin.H{"items": created})
	}
}
