package health

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

type Status struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) Live() Status {
	return Status{Status: "ok"}
}

func (s *Service) Ready(ctx context.Context) Status {
	checks := make(map[string]string)

	// Check Postgres connection
	if s.pool != nil {
		if err := s.pool.Ping(ctx); err != nil {
			checks["postgres"] = "error: " + err.Error()
			return Status{
				Status: "unavailable",
				Checks: checks,
			}
		}
		checks["postgres"] = "ok"
	} else {
		checks["postgres"] = "not configured"
	}

	return Status{
		Status: "ok",
		Checks: checks,
	}
}
