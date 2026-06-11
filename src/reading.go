package main

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		var books []UserBook
		for rows.Next() {
			var ub UserBook
			if err := rows.Scan(&ub.ID, &ub.UserID, &ub.EditionID, &ub.Status,
				&ub.Review, &ub.Rating, &ub.DateStarted, &ub.DateRead,
				&ub.CreatedAt, &ub.UpdatedAt); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			books = append(books, ub)
		}
		c.JSON(http.StatusOK, books)
	}
}
