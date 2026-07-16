package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type AdminUser struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type PersonData struct {
	ID         int     `json:"id"`
	FirstName  string  `json:"first_name"`
	MiddleName string  `json:"middle_name"`
	LastName   string  `json:"last_name"`
	Pseudonym  *string `json:"pseudonym,omitempty"`
	BirthDate  *string `json:"birth_date,omitempty"`
	DeathDate  *string `json:"death_date,omitempty"`
	Biography  *string `json:"biography,omitempty"`
	PhotoURL   *string `json:"photo_url,omitempty"`
	BooksCount int     `json:"books_count"`
}

// ─── Users ────────────────────────────────────────────────────

func adminGetUsers(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`SELECT id, username, COALESCE(email,''), role, created_at FROM users ORDER BY username`)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()
		users := make([]AdminUser, 0)
		for rows.Next() {
			var u AdminUser
			if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.CreatedAt); err != nil {
				adminInternalError(c, err)
				return
			}
			users = append(users, u)
		}
		c.JSON(http.StatusOK, users)
	}
}

func adminInternalError(c *gin.Context, err error) {
	log.Printf("Admin internal error: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Внутренняя ошибка сервера"})
}

func adminGetUser(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var u AdminUser
		err := db.QueryRow(`SELECT id, username, COALESCE(email,''), role, created_at FROM users WHERE id = $1`, id).
			Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.CreatedAt)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		if err != nil {
			adminInternalError(c, err)
			return
		}
		c.JSON(http.StatusOK, u)
	}
}

func adminCreateUser(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
			Email    string `json:"email"`
			Role     string `json:"role"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		if req.Role == "" {
			req.Role = "viewer"
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		var user AdminUser
		err = db.QueryRow(`
			INSERT INTO users (username, password_hash, email, role)
			VALUES ($1, $2, NULLIF($3,''), $4)
			RETURNING id, username, COALESCE(email,''), role, created_at
		`, req.Username, string(hash), req.Email, req.Role).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}
		c.JSON(http.StatusCreated, user)
	}
}

func adminUpdateUser(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Username *string `json:"username"`
			Password *string `json:"password"`
			Email    *string `json:"email"`
			Role     *string `json:"role"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		if req.Username != nil {
			_, err := db.Exec("UPDATE users SET username = $1 WHERE id = $2", *req.Username, id)
			if err != nil {
				c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
				return
			}
		}
		if req.Password != nil {
			hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
				return
			}
			_, err = db.Exec("UPDATE users SET password_hash = $1 WHERE id = $2", string(hash), id)
			if err != nil {
				adminInternalError(c, err)
				return
			}
		}
		if req.Email != nil {
			_, err := db.Exec("UPDATE users SET email = NULLIF($1,'') WHERE id = $2", *req.Email, id)
			if err != nil {
				adminInternalError(c, err)
				return
			}
		}
		if req.Role != nil {
			_, err := db.Exec("UPDATE users SET role = $1 WHERE id = $2", *req.Role, id)
			if err != nil {
				adminInternalError(c, err)
				return
			}
		}
		_, err := db.Exec("UPDATE users SET updated_at = NOW() WHERE id = $1", id)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		var user AdminUser
		err = db.QueryRow("SELECT id, username, COALESCE(email,''), role, created_at FROM users WHERE id = $1", id).
			Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}

func adminDeleteUser(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		res, err := db.Exec("DELETE FROM users WHERE id = $1", id)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ─── Persons ──────────────────────────────────────────────────

func adminGetPersons(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT p.id, COALESCE(p.first_name,''), COALESCE(p.middle_name,''), p.last_name,
				p.pseudonym, p.birth_date, p.death_date, p.biography, p.photo_url,
				COALESCE((SELECT COUNT(DISTINCT w.id) FROM work_contributors wc
					JOIN works w ON w.id = wc.work_id
					JOIN editions e ON e.work_id = w.id
					WHERE wc.person_id = p.id), 0) as books_count
			FROM persons p ORDER BY p.last_name, p.first_name
		`)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()
		persons := make([]PersonData, 0)
		for rows.Next() {
			var p PersonData
			if err := rows.Scan(&p.ID, &p.FirstName, &p.MiddleName, &p.LastName,
				&p.Pseudonym, &p.BirthDate, &p.DeathDate, &p.Biography, &p.PhotoURL, &p.BooksCount); err != nil {
				adminInternalError(c, err)
				return
			}
			persons = append(persons, p)
		}
		c.JSON(http.StatusOK, persons)
	}
}

func adminCreatePerson(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			FirstName  string  `json:"first_name"`
			MiddleName string  `json:"middle_name"`
			LastName   string  `json:"last_name" binding:"required"`
			Pseudonym  *string `json:"pseudonym"`
			BirthDate  *string `json:"birth_date"`
			DeathDate  *string `json:"death_date"`
			Biography  *string `json:"biography"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		var p PersonData
		err := db.QueryRow(`
			INSERT INTO persons (first_name, middle_name, last_name, pseudonym, birth_date, death_date, biography)
			VALUES ($1, $2, $3, $4, NULLIF($5,'')::date, NULLIF($6,'')::date, $7)
			RETURNING id, COALESCE(first_name,''), COALESCE(middle_name,''), last_name,
				pseudonym, birth_date, death_date, biography, photo_url, 0
		`, req.FirstName, req.MiddleName, req.LastName, req.Pseudonym, req.BirthDate, req.DeathDate, req.Biography).
			Scan(&p.ID, &p.FirstName, &p.MiddleName, &p.LastName, &p.Pseudonym, &p.BirthDate, &p.DeathDate, &p.Biography, &p.PhotoURL, &p.BooksCount)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Person with this name already exists"})
			return
		}
		c.JSON(http.StatusCreated, p)
	}
}

func adminUpdatePerson(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			FirstName  string  `json:"first_name"`
			MiddleName string  `json:"middle_name"`
			LastName   string  `json:"last_name"`
			Pseudonym  *string `json:"pseudonym"`
			BirthDate  *string `json:"birth_date"`
			DeathDate  *string `json:"death_date"`
			Biography  *string `json:"biography"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		_, err := db.Exec(`
			UPDATE persons SET
				first_name = COALESCE(NULLIF($1,''), first_name),
				middle_name = COALESCE(NULLIF($2,''), middle_name),
				last_name = COALESCE(NULLIF($3,''), last_name),
				pseudonym = $4,
				birth_date = NULLIF($5,'')::date,
				death_date = NULLIF($6,'')::date,
				biography = $7
			WHERE id = $8
		`, req.FirstName, req.MiddleName, req.LastName, req.Pseudonym, req.BirthDate, req.DeathDate, req.Biography, id)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		var p PersonData
		err = db.QueryRow(`
			SELECT p.id, COALESCE(p.first_name,''), COALESCE(p.middle_name,''), p.last_name,
				p.pseudonym, p.birth_date, p.death_date, p.biography, p.photo_url,
				COALESCE((SELECT COUNT(DISTINCT w.id) FROM work_contributors wc
					JOIN works w ON w.id = wc.work_id
					JOIN editions e ON e.work_id = w.id
					WHERE wc.person_id = p.id), 0) as books_count
			FROM persons p WHERE p.id = $1
		`, id).Scan(&p.ID, &p.FirstName, &p.MiddleName, &p.LastName, &p.Pseudonym, &p.BirthDate, &p.DeathDate, &p.Biography, &p.PhotoURL, &p.BooksCount)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

func adminDeletePerson(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		res, err := db.Exec("DELETE FROM persons WHERE id = $1", id)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ─── Tags ─────────────────────────────────────────────────────

func adminUpdateTag(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Name        string  `json:"name"`
			Color       *string `json:"color"`
			Description *string `json:"description"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		_, err := db.Exec(`
			UPDATE tags SET name = COALESCE(NULLIF($1,''), name),
				color = $2, description = $3 WHERE id = $4
		`, req.Name, req.Color, req.Description, id)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Tag name already exists"})
			return
		}
		var tag TagData
		err = db.QueryRow("SELECT id, name FROM tags WHERE id = $1", id).Scan(&tag.ID, &tag.Name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusOK, tag)
	}
}

func adminDeleteTag(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		res, err := db.Exec("DELETE FROM tags WHERE id = $1", id)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ─── Genres (extra — admin-specific list with all fields) ─────

func adminGetGenres(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT g.id, g.name, g.parent_id, g.description,
				COALESCE(p.name, '') as parent_name,
				COALESCE((SELECT COUNT(DISTINCT e.id) FROM work_genres wg
					JOIN editions e ON e.work_id = wg.work_id
					WHERE wg.genre_id = g.id), 0) as books_count
			FROM genres g
			LEFT JOIN genres p ON p.id = g.parent_id
			ORDER BY g.name
		`)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()
		type AdminGenre struct {
			ID          int     `json:"id"`
			Name        string  `json:"name"`
			ParentID    *int    `json:"parent_id,omitempty"`
			Description *string `json:"description,omitempty"`
			ParentName  string  `json:"parent_name"`
			BooksCount  int     `json:"books_count"`
		}
		genres := make([]AdminGenre, 0)
		for rows.Next() {
			var g AdminGenre
			var parentID sql.NullInt64
			var desc sql.NullString
			if err := rows.Scan(&g.ID, &g.Name, &parentID, &desc, &g.ParentName, &g.BooksCount); err != nil {
				adminInternalError(c, err)
				return
			}
			if parentID.Valid {
				v := int(parentID.Int64)
				g.ParentID = &v
			}
			if desc.Valid {
				g.Description = &desc.String
			}
			genres = append(genres, g)
		}
		c.JSON(http.StatusOK, genres)
	}
}

func adminGetTags(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT t.id, t.name, t.color, COALESCE(t.description,''),
				COALESCE((SELECT COUNT(DISTINCT et.edition_id) FROM edition_tags et WHERE et.tag_id = t.id), 0) as books_count
			FROM tags t ORDER BY t.name
		`)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()
		type AdminTag struct {
			ID          int     `json:"id"`
			Name        string  `json:"name"`
			Color       *string `json:"color,omitempty"`
			Description string  `json:"description"`
			BooksCount  int     `json:"books_count"`
		}
		tags := make([]AdminTag, 0)
		for rows.Next() {
			var t AdminTag
			var color sql.NullString
			if err := rows.Scan(&t.ID, &t.Name, &color, &t.Description, &t.BooksCount); err != nil {
				adminInternalError(c, err)
				return
			}
			if color.Valid {
				t.Color = &color.String
			}
			tags = append(tags, t)
		}
		c.JSON(http.StatusOK, tags)
	}
}

// ─── Settings ──────────────────────────────────────────────────

type SettingsData struct {
	BackupDir string `json:"backup_dir"`
}

func adminGetSettings(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`SELECT key, value FROM settings`)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()

		settings := SettingsData{}
		for rows.Next() {
			var key, value string
			if err := rows.Scan(&key, &value); err != nil {
				adminInternalError(c, err)
				return
			}
			switch key {
			case "backup_dir":
				settings.BackupDir = value
			}
		}
		c.JSON(http.StatusOK, settings)
	}
}

func adminUpdateSettings(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SettingsData
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
			return
		}

		_, err := db.Exec(`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW())
			ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`,
			"backup_dir", req.BackupDir)
		if err != nil {
			adminInternalError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
