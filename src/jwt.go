package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

var jwtSecret = []byte("your-secret-key-change-in-production")

var (
	ErrInvalidToken  = errors.New("invalid token")
	ErrTokenExpired  = errors.New("token expired")
)

type TokenClaims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Exp      int64  `json:"exp"`
}

func generateToken(userID int, username, role string) string {
	claims := TokenClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		Exp:      time.Now().Add(24 * time.Hour).Unix(),
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
	expectedSignature := computeHMAC(header + "." + payload)

	if signature != expectedSignature {
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
	h := hmac.New(sha256.New, jwtSecret)
	h.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
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