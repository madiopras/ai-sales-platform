package auth

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestLoginGeneratesAccessToken(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := NewTokenManager("a-secure-development-secret-with-32-chars", "test", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(memoryRepository{user: User{ID: "user-1", Email: "admin@example.com", PasswordHash: string(hash), Role: "admin", IsActive: true}}, tokens)
	result, err := service.Login(context.Background(), "admin@example.com", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	if result.AccessToken == "" {
		t.Fatal("expected access token")
	}
	claims, err := tokens.Parse(result.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("expected user ID, got %q", claims.Subject)
	}
}

type memoryRepository struct{ user User }

func (r memoryRepository) FindByEmail(context.Context, string) (User, error) { return r.user, nil }
func (r memoryRepository) FindByID(context.Context, string) (User, error)    { return r.user, nil }
func (r memoryRepository) Create(context.Context, User) (User, error)        { return User{}, nil }
