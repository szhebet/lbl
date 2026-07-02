package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"time"
)

var (
	jwtSecret []byte
	tokenTTL  = 24 // hours; default 24h
	ErrInvalidToken  = errors.New("invalid token")
	ErrTokenExpired  = errors.New("token expired")
)

type TokenClaims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Exp      int64  `json:"exp,omitempty"`
}

func initJWTSecret(secret string) {
	if secret != "" {
		jwtSecret = []byte(secret)
		return
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		log.Fatal("Failed to generate JWT secret: ", err)
	}
	jwtSecret = []byte(hex.EncodeToString(buf))
	log.Println("Generated random JWT secret (set jwt_secret in config for persistence)")
}

func initTokenTTL(ttlHours int) {
	tokenTTL = ttlHours
	if tokenTTL < 0 {
		tokenTTL = 24
	}
	if tokenTTL == 0 {
		tokenTTL = 24
	}
}

func generateToken(userID int, username, role string) string {
	claims := TokenClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
	}
	if tokenTTL > 0 {
		claims.Exp = time.Now().Add(time.Duration(tokenTTL) * time.Hour).Unix()
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	
	payloadBytes, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	
	signature := computeHMAC(header + "." + payload)
	
	return header + "." + payload + "." + signature
}

func validateToken(tokenString string) (map[string]interface{}, error) {
	parts := splitToken(tokenString)
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	header, payload, signature := parts[0], parts[1], parts[2]

	expected := computeHMACBytes(header + "." + payload)
	sigBytes, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || !hmac.Equal(sigBytes, expected) {
		return nil, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, ErrTokenExpired
		}
	}

	return claims, nil
}

func computeHMAC(message string) string {
	return base64.RawURLEncoding.EncodeToString(computeHMACBytes(message))
}

func computeHMACBytes(message string) []byte {
	h := hmac.New(sha256.New, jwtSecret)
	h.Write([]byte(message))
	return h.Sum(nil)
}

func splitToken(token string) []string {
	result := make([]string, 0, 3)
	start := 0
	dots := 0
	for i, c := range token {
		if c == '.' {
			result = append(result, token[start:i])
			start = i + 1
			dots++
			if dots == 2 {
				result = append(result, token[start:])
				return result
			}
		}
	}
	return result
}