package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewTokenManager(secret, issuer string, ttl time.Duration) (*TokenManager, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT secret must contain at least 32 characters")
	}
	return &TokenManager{secret: []byte(secret), issuer: issuer, ttl: ttl}, nil
}

func (m *TokenManager) Generate(user User) (string, time.Time, error) {
	expiresAt := time.Now().Add(m.ttl)
	claims := jwt.MapClaims{"sub": user.ID, "email": user.Email, "role": user.Role, "iss": m.issuer, "iat": time.Now().Unix(), "exp": expiresAt.Unix()}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	return signed, expiresAt, err
}

func (m *TokenManager) Parse(rawToken string) (Claims, error) {
	parsed, err := jwt.Parse(rawToken, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected JWT signing method")
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithExpirationRequired())
	if err != nil || !parsed.Valid {
		return Claims{}, fmt.Errorf("invalid access token")
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, fmt.Errorf("invalid access token claims")
	}
	subject, _ := claims.GetSubject()
	if subject == "" {
		return Claims{}, fmt.Errorf("access token subject is required")
	}
	email, _ := claims["email"].(string)
	role, _ := claims["role"].(string)
	return Claims{Subject: subject, Email: email, Role: role}, nil
}
