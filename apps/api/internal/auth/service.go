package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Service struct {
	repository Repository
	tokens     *TokenManager
}

type LoginResult struct {
	AccessToken string
	ExpiresAt   time.Time
	User        User
}

type CreateUserInput struct{ Email, Password, Name, Role string }

func NewService(repository Repository, tokens *TokenManager) *Service {
	return &Service{repository: repository, tokens: tokens}
}

func (s *Service) Login(ctx context.Context, email, password string) (LoginResult, error) {
	user, err := s.repository.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil || !user.IsActive || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	if s.tokens == nil {
		return LoginResult{}, fmt.Errorf("token manager is not configured")
	}
	token, expiresAt, err := s.tokens.Generate(user)
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate access token: %w", err)
	}
	return LoginResult{AccessToken: token, ExpiresAt: expiresAt, User: user}, nil
}

func (s *Service) CurrentUser(ctx context.Context, id string) (User, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *Service) CreateUser(ctx context.Context, input CreateUserInput) (User, error) {
	if len(input.Password) < 12 {
		return User{}, fmt.Errorf("password must contain at least 12 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	return s.repository.Create(ctx, User{Email: strings.ToLower(strings.TrimSpace(input.Email)), PasswordHash: string(hash), Name: strings.TrimSpace(input.Name), Role: input.Role, IsActive: true})
}
