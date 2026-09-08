package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"time"
)

type Manager struct {
	secret []byte
	ttl    time.Duration
}
type Claims struct {
	UserID   uint   `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func NewManager(secret string, ttl time.Duration) *Manager { return &Manager{[]byte(secret), ttl} }
func (m *Manager) Issue(id uint, username, role string) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{id, username, role, jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)), IssuedAt: jwt.NewNumericDate(now)}}).SignedString(m.secret)
}
func (m *Manager) Parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) { return m.secret, nil })
	if err != nil {
		return nil, err
	}
	return parsed.Claims.(*Claims), nil
}
