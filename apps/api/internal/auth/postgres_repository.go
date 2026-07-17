package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) FindByEmail(ctx context.Context, email string) (User, error) {
	return r.find(ctx, "SELECT id, email, password_hash, name, role, is_active, created_at FROM users WHERE email = LOWER($1)", email)
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (User, error) {
	return r.find(ctx, "SELECT id, email, password_hash, name, role, is_active, created_at FROM users WHERE id = $1", id)
}

func (r *PostgresRepository) Create(ctx context.Context, user User) (User, error) {
	err := r.pool.QueryRow(ctx, `INSERT INTO users (email, password_hash, name, role, is_active)
VALUES (LOWER($1), $2, $3, $4, $5)
RETURNING id, email, password_hash, name, role, is_active, created_at`, user.Email, user.PasswordHash, user.Name, user.Role, user.IsActive).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.IsActive, &user.CreatedAt)
	return user, err
}

func (r *PostgresRepository) find(ctx context.Context, query string, argument string) (User, error) {
	var user User
	err := r.pool.QueryRow(ctx, query, argument).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.IsActive, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return user, err
}
