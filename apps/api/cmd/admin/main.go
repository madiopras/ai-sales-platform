package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/auth"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/config"
)

func main() {
	email, password := os.Getenv("ADMIN_EMAIL"), os.Getenv("ADMIN_PASSWORD")
	if email == "" || password == "" {
		panic("ADMIN_EMAIL and ADMIN_PASSWORD are required")
	}
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	pool, err := pgxpool.New(context.Background(), cfg.Database.DSN())
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	user, err := auth.NewService(auth.NewPostgresRepository(pool), nil).CreateUser(context.Background(), auth.CreateUserInput{Email: email, Password: password, Name: "Platform Administrator", Role: "admin"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("created admin user %s (%s)\n", user.Email, user.ID)
}
