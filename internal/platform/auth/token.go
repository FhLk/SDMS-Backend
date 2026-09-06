package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

type Claims struct {
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl}
}

func (m *TokenManager) Generate(userID uuid.UUID) (string, error) {
	if userID == uuid.Nil || len(m.secret) == 0 {
		return "", ErrInvalidToken
	}

	now := time.Now().UTC()
	headerJSON, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	claimsJSON, err := json.Marshal(Claims{
		Subject:   userID.String(),
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(m.ttl).Unix(),
	})
	if err != nil {
		return "", err
	}

	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	unsigned := header + "." + payload
	signature := m.sign(unsigned)
	return unsigned + "." + signature, nil
}

func (m *TokenManager) Parse(token string) (uuid.UUID, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(m.secret) == 0 {
		return uuid.Nil, ErrInvalidToken
	}

	unsigned := parts[0] + "." + parts[1]
	expectedSignature, err := base64.RawURLEncoding.DecodeString(m.sign(unsigned))
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(expectedSignature, providedSignature) {
		return uuid.Nil, ErrInvalidToken
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	var header map[string]string
	if err := json.Unmarshal(headerBytes, &header); err != nil || header["alg"] != "HS256" {
		return uuid.Nil, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	if claims.ExpiresAt <= time.Now().UTC().Unix() {
		return uuid.Nil, ErrExpiredToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil || userID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("%w: invalid subject", ErrInvalidToken)
	}
	return userID, nil
}

func (m *TokenManager) sign(unsigned string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(unsigned))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
