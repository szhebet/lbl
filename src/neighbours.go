package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Neighbour represents a trusted remote library server (peer) whose data this
// instance can exchange with. The password is never exposed: only a
// has_password flag is returned, and the encrypted blob lives in the DB.
type Neighbour struct {
	ID          int       `json:"id"`
	URL         string    `json:"url"`
	ServerCert  string    `json:"server_cert"`
	ClientCert  string    `json:"client_cert"`
	Username    string    `json:"username"`
	HasPassword bool      `json:"has_password"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NeighbourCrypto encrypts/decrypts neighbour passwords with AES-256-GCM.
// The 32-byte key is generated on first use and stored (hex-encoded) in the
// settings table, so encrypted passwords survive restarts and config changes
// regardless of the JWT secret.
type NeighbourCrypto struct {
	key []byte
}

// NewNeighbourCrypto loads the encryption key from settings, generating and
// persisting a fresh random key on first use. The INSERT is idempotent so a
// concurrent startup cannot split the key: we always re-read what the DB
// actually holds.
func NewNeighbourCrypto(db *sql.DB) (*NeighbourCrypto, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	hexKey := hex.EncodeToString(key)
	if _, err := db.Exec(
		`INSERT INTO settings (key, value) VALUES ('api_neighbours_key', $1)
		 ON CONFLICT (key) DO NOTHING`, hexKey); err != nil {
		return nil, err
	}
	var stored string
	if err := db.QueryRow(
		`SELECT value FROM settings WHERE key = 'api_neighbours_key'`).Scan(&stored); err != nil {
		return nil, err
	}
	decoded, err := hex.DecodeString(strings.TrimSpace(stored))
	if err != nil {
		return nil, err
	}
	return &NeighbourCrypto{key: decoded}, nil
}

// Encrypt seals a plaintext password. Stored form: base64(nonce || ciphertext).
func (nc *NeighbourCrypto) Encrypt(plain string) (string, error) {
	block, err := aes.NewCipher(nc.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt opens a password previously produced by Encrypt.
func (nc *NeighbourCrypto) Decrypt(enc string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(enc))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(nc.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", err
	}
	nonce := raw[:gcm.NonceSize()]
	plain, err := gcm.Open(nil, nonce, raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// ─── Handlers ──────────────────────────────────────────────────

func adminGetNeighbours(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT id, url, server_cert, client_cert, username,
			       password_encrypted, created_at, updated_at
			FROM api_neighbours ORDER BY url`)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		defer rows.Close()
		neighbours := make([]Neighbour, 0)
		for rows.Next() {
			var n Neighbour
			var enc string
			if err := rows.Scan(&n.ID, &n.URL, &n.ServerCert, &n.ClientCert,
				&n.Username, &enc, &n.CreatedAt, &n.UpdatedAt); err != nil {
				adminInternalError(c, err)
				return
			}
			n.HasPassword = enc != ""
			neighbours = append(neighbours, n)
		}
		c.JSON(http.StatusOK, neighbours)
	}
}

// adminGetNeighbour returns a single neighbour by ID (used to prefill the
// edit-server form in the admin UI).
func adminGetNeighbour(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID"})
			return
		}
		var n Neighbour
		var enc string
		err = db.QueryRow(`
			SELECT id, url, server_cert, client_cert, username,
			       password_encrypted, created_at, updated_at
			FROM api_neighbours WHERE id = $1`, id).
			Scan(&n.ID, &n.URL, &n.ServerCert, &n.ClientCert,
				&n.Username, &enc, &n.CreatedAt, &n.UpdatedAt)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Сервер не найден"})
			} else {
				adminInternalError(c, err)
			}
			return
		}
		n.HasPassword = enc != ""
		c.JSON(http.StatusOK, n)
	}
}

type neighbourRequest struct {
	URL           string `json:"url"`
	ServerCert    string `json:"server_cert"`
	ClientCert    string `json:"client_cert"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	ClearPassword bool   `json:"clear_password"`
}

func (r *neighbourRequest) normalize() {
	r.URL = strings.TrimSpace(r.URL)
	r.ServerCert = strings.TrimSpace(r.ServerCert)
	r.ClientCert = strings.TrimSpace(r.ClientCert)
	r.Username = strings.TrimSpace(r.Username)
	r.Password = strings.TrimSpace(r.Password)
}

func adminCreateNeighbour(db *sql.DB, nc *NeighbourCrypto) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req neighbourRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
			return
		}
		req.normalize()
		if req.URL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "URL обязателен"})
			return
		}
		enc := ""
		if req.Password != "" {
			var err error
			enc, err = nc.Encrypt(req.Password)
			if err != nil {
				adminInternalError(c, err)
				return
			}
		}
		var id int
		err := db.QueryRow(`
			INSERT INTO api_neighbours (url, server_cert, client_cert, username, password_encrypted)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			req.URL, req.ServerCert, req.ClientCert, req.Username, enc).Scan(&id)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": id})
	}
}

func adminUpdateNeighbour(db *sql.DB, nc *NeighbourCrypto) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID"})
			return
		}
		var req neighbourRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
			return
		}
		req.normalize()
		if req.URL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "URL обязателен"})
			return
		}

		var enc string
		switch {
		case req.ClearPassword:
			enc = ""
		case req.Password != "":
			enc, err = nc.Encrypt(req.Password)
			if err != nil {
				adminInternalError(c, err)
				return
			}
		default:
			if err := db.QueryRow(
				`SELECT password_encrypted FROM api_neighbours WHERE id = $1`, id).Scan(&enc); err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Сервер не найден"})
				} else {
					adminInternalError(c, err)
				}
				return
			}
		}

		res, err := db.Exec(`
			UPDATE api_neighbours
			SET url = $1, server_cert = $2, client_cert = $3, username = $4,
			    password_encrypted = $5, updated_at = NOW()
			WHERE id = $6`,
			req.URL, req.ServerCert, req.ClientCert, req.Username, enc, id)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Сервер не найден"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func adminDeleteNeighbour(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID"})
			return
		}
		res, err := db.Exec(`DELETE FROM api_neighbours WHERE id = $1`, id)
		if err != nil {
			adminInternalError(c, err)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Сервер не найден"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
