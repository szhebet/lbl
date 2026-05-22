package main

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ReadingProgress struct {
	ID          int    `json:"id"`
	UserID      int    `json:"user_id"`
	EditionID   int    `json:"edition_id"`
	Progress    int    `json:"progress"`
	Finished    bool   `json:"finished"`
	Rating      int    `json:"rating"`
	Notes       string `json:"notes"`
	StartedAt   string `json:"started_at"`
	FinishedAt  string `json:"finished_at"`
}

func getReadingProgress(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt("user_id")
		editionID := c.Query("edition_id")

		query := `SELECT id, user_id, edition_id, progress, finished, rating, notes, started_at, finished_at 
		          FROM reading_progress WHERE user_id = $1`
		args := []interface{}{userID}

		if editionID != "" {
			query += " AND edition_id = $2"
			args = append(args, editionID)
		}

		rows, err := db.Query(query, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var progressList []ReadingProgress
		for rows.Next() {
			var p ReadingProgress
			if err := rows.Scan(&p.ID, &p.UserID, &p.EditionID, &p.Progress, &p.Finished, &p.Rating, &p.Notes, &p.StartedAt, &p.FinishedAt); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			progressList = append(progressList, p)
		}

		c.JSON(http.StatusOK, progressList)
	}
}

func updateReadingProgress(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt("user_id")
		editionID := c.Param("id")

		var req struct {
			Progress int  `json:"progress"`
			Finished bool `json:"finished"`
			Rating   int  `json:"rating"`
			Notes    string `json:"notes"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		_, err := db.Exec(`
			INSERT INTO reading_progress (user_id, edition_id, progress, finished, rating, notes)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (user_id, edition_id) 
			DO UPDATE SET 
				progress = EXCLUDED.progress,
				finished = EXCLUDED.finished,
				rating = COALESCE(EXCLUDED.rating, reading_progress.rating),
				notes = COALESCE(EXCLUDED.notes, reading_progress.notes)
		`, userID, editionID, req.Progress, req.Finished, req.Rating, req.Notes)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Progress updated"})
	}
}

func deleteReadingProgress(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt("user_id")
		editionID := c.Param("id")

		_, err := db.Exec("DELETE FROM reading_progress WHERE user_id = $1 AND edition_id = $2", userID, editionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Progress deleted"})
	}
}