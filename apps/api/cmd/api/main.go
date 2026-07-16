package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/bootstrap"
	"go.uber.org/zap"
)

func main() {
	app, err := bootstrap.New()
	if err != nil {
		panic(err)
	}
	defer app.Close()

	go func() {
		if err := app.Run(); err != nil && !errors.Is(err, bootstrap.ErrServerClosed) {
			app.Logger.Fatal("http server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.Shutdown(ctx); err != nil {
		app.Logger.Error("graceful shutdown failed", zap.Error(err))
	}
}
