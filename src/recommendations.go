package main

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Recommendation struct {
	WorkID           int     `json:"work_id"`
	Title            string  `json:"title"`
	Authors          string  `json:"authors"`
	SimilarityScore  float64 `json:"similarity_score"`
	RecommendationType string `json:"type"`
}

func getRecommendations(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		workID := c.Query("work_id")
		limit := c.DefaultQuery("limit", "10")

		var recommendations []Recommendation

		if workID != "" {
			recommendations = getSimilarByWork(db, workID, limit)
		} else {
			recommendations = getPopularRecommendations(db, limit)
		}

		c.JSON(http.StatusOK, gin.H{
			"recommendations": recommendations,
			"count": len(recommendations),
		})
	}
}

func getSimilarByWork(db *sql.DB, workID, limit string) []Recommendation {
	query := `
		SELECT DISTINCT 
			w.id as work_id,
			w.original_title,
			COALESCE(string_agg(DISTINCT a.name, ', '), '') as authors,
			COUNT(DISTINCT wg.genre_id) as shared_genres,
			COUNT(DISTINCT wa.author_id) as shared_authors
		FROM works w
		LEFT JOIN work_genres wg ON w.id = wg.work_id
		LEFT JOIN work_authors wa ON w.id = wa.work_id
		LEFT JOIN authors a ON wa.author_id = a.id
		WHERE w.id != $1
		AND (
			EXISTS (SELECT 1 FROM work_genres wg2 WHERE wg2.work_id = $1 AND wg2.genre_id IN (SELECT genre_id FROM work_genres WHERE work_id = w.id))
			OR EXISTS (SELECT 1 FROM work_authors wa2 WHERE wa2.work_id = $1 AND wa2.author_id IN (SELECT author_id FROM work_authors WHERE work_id = w.id))
		)
		GROUP BY w.id, w.original_title
		ORDER BY shared_genres DESC, shared_authors DESC
		LIMIT $2
	`

	rows, err := db.Query(query, workID, limit)
	if err != nil {
		return []Recommendation{}
	}
	defer rows.Close()

	var recs []Recommendation
	for rows.Next() {
		var r Recommendation
		var sharedGenres, sharedAuthors int
		if err := rows.Scan(&r.WorkID, &r.Title, &r.Authors, &sharedGenres, &sharedAuthors); err != nil {
			continue
		}
		r.SimilarityScore = float64(sharedGenres*2 + sharedAuthors*3)
		r.RecommendationType = "similar"
		recs = append(recs, r)
	}

	return recs
}

func getPopularRecommendations(db *sql.DB, limit string) []Recommendation {
	query := `
		SELECT 
			w.id as work_id,
			w.original_title,
			COALESCE(string_agg(DISTINCT a.name, ', '), '') as authors,
			COUNT(DISTINCT e.id) as edition_count,
			COALESCE(SUM(rp.rating), 0) as total_rating,
			COUNT(DISTINCT rp.id) as rating_count
		FROM works w
		LEFT JOIN editions e ON w.id = e.work_id
		LEFT JOIN reading_progress rp ON e.id = rp.edition_id AND rp.finished = true
		LEFT JOIN work_authors wa ON w.id = wa.work_id
		LEFT JOIN authors a ON wa.author_id = a.id
		GROUP BY w.id, w.original_title
		HAVING COUNT(DISTINCT e.id) > 0
		ORDER BY rating_count DESC, total_rating DESC
		LIMIT $1
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return []Recommendation{}
	}
	defer rows.Close()

	var recs []Recommendation
	for rows.Next() {
		var r Recommendation
		var editionCount, ratingCount int
		var totalRating float64
		if err := rows.Scan(&r.WorkID, &r.Title, &r.Authors, &editionCount, &totalRating, &ratingCount); err != nil {
			continue
		}
		r.SimilarityScore = totalRating
		r.RecommendationType = "popular"
		recs = append(recs, r)
	}

	return recs
}

func getRecommendationsByAuthor(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authorID := c.Query("author_id")
		limit := c.DefaultQuery("limit", "5")

		query := `
			SELECT DISTINCT
				w.id as work_id,
				w.original_title,
				COALESCE(string_agg(DISTINCT a.name, ', '), '') as authors,
				w.first_published
			FROM works w
			JOIN work_authors wa ON w.id = wa.work_id
			JOIN authors a ON wa.author_id = a.id
			WHERE wa.author_id = $1
			ORDER BY w.first_published DESC
			LIMIT $2
		`

		rows, err := db.Query(query, authorID, limit)
		if err != nil {
			internalError(c, err)
			return
		}
		defer rows.Close()

		var recs []Recommendation
		for rows.Next() {
			var r Recommendation
			var year *int
			if err := rows.Scan(&r.WorkID, &r.Title, &r.Authors, &year); err != nil {
				continue
			}
			r.SimilarityScore = 1.0
			r.RecommendationType = "same_author"
			recs = append(recs, r)
		}

		c.JSON(http.StatusOK, gin.H{
			"recommendations": recs,
			"count": len(recs),
		})
	}
}