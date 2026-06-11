package main

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type LoginRequest struct {
	Username          string `json:"username" binding:"required"`
	Password          string `json:"password" binding:"required"`
	DeviceName        string `json:"device_name"`
	DeviceFingerprint string `json:"device_fingerprint"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

func loginUser(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		if req.DeviceName == "" {
			req.DeviceName = "Unknown device"
		}

		var user User
		var passwordHash string
		err := db.QueryRow(`
			SELECT id, username, COALESCE(email, ''), role, created_at
			FROM users WHERE username = $1
		`, req.Username).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt)

		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"user_not_found": true,
				"username":       req.Username,
			})
			return
		}

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		err = db.QueryRow("SELECT password_hash FROM users WHERE id = $1", user.ID).Scan(&passwordHash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный пароль"})
			return
		}

		if req.DeviceFingerprint != "" {
			_, err = db.Exec(`
				INSERT INTO user_devices (user_id, device_name, device_fingerprint)
				VALUES ($1, $2, $3)
				ON CONFLICT (device_fingerprint) DO UPDATE SET device_name = EXCLUDED.device_name
			`, user.ID, req.DeviceName, req.DeviceFingerprint)
		}

		token := generateToken(user.ID, user.Username, user.Role)

		c.JSON(http.StatusOK, AuthResponse{
			Token: token,
			User:  user,
		})
	}
}

func createUser(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		if req.DeviceName == "" {
			req.DeviceName = "Unknown device"
		}

		var existingID int
		err := db.QueryRow("SELECT id FROM users WHERE username = $1", req.Username).Scan(&existingID)
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		var user User
		err = db.QueryRow(`
			INSERT INTO users (username, password_hash, role)
			VALUES ($1, $2, 'viewer')
			RETURNING id, username, COALESCE(email, ''), role, created_at
		`, req.Username, string(hashedPassword)).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}

		if req.DeviceFingerprint != "" {
			_, err = db.Exec(`
				INSERT INTO user_devices (user_id, device_name, device_fingerprint)
				VALUES ($1, $2, $3)
				ON CONFLICT (device_fingerprint) DO UPDATE SET device_name = EXCLUDED.device_name
			`, user.ID, req.DeviceName, req.DeviceFingerprint)
		}

		token := generateToken(user.ID, user.Username, user.Role)

		c.JSON(http.StatusCreated, AuthResponse{
			Token: token,
			User:  user,
		})
	}
}
