package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken: token bozuk, suresi dolmus veya imzasi tutmuyor.
var ErrInvalidToken = errors.New("auth: gecersiz token")

// TokenManager JWT uretir ve dogrular.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl}
}

// claims: token'in icine gomdugumuz veriler. "sub" (subject) = kullanici id.
type claims struct {
	jwt.RegisteredClaims
}

// Generate bir kullanici id'si icin imzali JWT uretir.
func (m *TokenManager) Generate(userID string) (string, error) {
	now := time.Now()
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	// HS256: simetrik imza. Sunucu hem imzalar hem dogrular, ayni secret ile.
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return tok.SignedString(m.secret)
}

// Parse bir token string'ini dogrular ve icindeki kullanici id'sini doner.
func (m *TokenManager) Parse(tokenStr string) (userID string, err error) {
	var c claims
	_, err = jwt.ParseWithClaims(tokenStr, &c, func(t *jwt.Token) (any, error) {
		// KRITIK guvenlik kontrolu: token'in "alg" alanina koru koru guvenme.
		// Saldirgan alg'i "none" yapip imzasiz token yollamaya calisabilir.
		// Sadece bizim kullandigimiz yontemi kabul et.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("beklenmeyen imza yontemi: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return "", ErrInvalidToken
	}
	if c.Subject == "" {
		return "", ErrInvalidToken
	}
	return c.Subject, nil
}
