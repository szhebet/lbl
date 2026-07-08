package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
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
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

func setSessionCookie(c *gin.Context, token string, ttlHours int) {
	maxAge := ttlHours * 3600
	if maxAge <= 0 {
		maxAge = 86400
	}
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("session_token", token, maxAge, "/", "", false, true)
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

			// Use advisory lock to prevent race conditions on first-user creation
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сервера"})
			return
		}
		defer func() {
			if err != nil {
				tx.Rollback()
			} else {
				tx.Commit()
			}
		}()

		_, err = tx.Exec("SELECT pg_advisory_xact_lock(42)")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сервера"})
			return
		}

		var userCount int
		tx.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)

		if userCount == 0 {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сервера"})
				return
			}
			var user User
			err = tx.QueryRow(`
				INSERT INTO users (username, password_hash, role)
				VALUES ($1, $2, 'admin')
				RETURNING id, username, COALESCE(email, ''), role, created_at
			`, req.Username, string(hashedPassword)).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt)
			if err != nil {
				if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
					c.JSON(http.StatusConflict, gin.H{"error": "Пользователь уже существует"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сервера"})
				return
			}
			if req.DeviceFingerprint != "" {
				db.Exec(`
					INSERT INTO user_devices (user_id, device_name, device_fingerprint)
					VALUES ($1, $2, $3)
					ON CONFLICT (device_fingerprint) DO UPDATE SET device_name = EXCLUDED.device_name
				`, user.ID, req.DeviceName, req.DeviceFingerprint)
			}
			token := generateToken(user.ID, user.Username, user.Role)
			refreshToken, _ := generateRefreshToken(db, user.ID, req.DeviceName, req.DeviceFingerprint)
			setSessionCookie(c, token, tokenTTL)
			c.JSON(http.StatusOK, AuthResponse{Token: token, RefreshToken: refreshToken, User: user})
			return
		}

		var user User
		var passwordHash string
		err = tx.QueryRow(`
			SELECT id, username, COALESCE(email, ''), role, password_hash, created_at
			FROM users WHERE username = $1
		`, req.Username).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &passwordHash, &user.CreatedAt)

		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверное имя пользователя или пароль"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сервера"})
			return
		}

		if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверное имя пользователя или пароль"})
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
		refreshToken, _ := generateRefreshToken(db, user.ID, req.DeviceName, req.DeviceFingerprint)
		setSessionCookie(c, token, tokenTTL)

		c.JSON(http.StatusOK, AuthResponse{
			Token:        token,
			RefreshToken: refreshToken,
			User:         user,
		})
	}
}

func refreshToken(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RefreshTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		userID, err := validateRefreshToken(db, req.RefreshToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
			return
		}

		var user User
		err = db.QueryRow(`
			SELECT id, username, COALESCE(email, ''), role, created_at
			FROM users WHERE id = $1
		`, userID).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		token := generateToken(user.ID, user.Username, user.Role)
		setSessionCookie(c, token, tokenTTL)

		c.JSON(http.StatusOK, RefreshTokenResponse{
			Token: token,
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

		if len(req.Password) < minPasswordLength {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Пароль должен быть не менее " + strconv.Itoa(minPasswordLength) + " символов"})
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
		refreshToken, _ := generateRefreshToken(db, user.ID, req.DeviceName, req.DeviceFingerprint)
		setSessionCookie(c, token, tokenTTL)

		c.JSON(http.StatusCreated, AuthResponse{
			Token:        token,
			RefreshToken: refreshToken,
			User:         user,
		})
	}
}
