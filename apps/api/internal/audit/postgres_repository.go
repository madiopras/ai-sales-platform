package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a log id doesn't exist.
var ErrNotFound = errors.New("audit log not found")

// PostgresRepository is the pgx-backed implementation of Repository.
type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const auditColumns = `id, actor_user_id, actor_email, action, resource_type,
	resource_id, metadata, created_at`

const selectAuditQuery = `SELECT ` + auditColumns + ` FROM audit_logs`

func (r *PostgresRepository) Record(ctx context.Context, log Log) (Log, error) {
	metadata, err := json.Marshal(log.Metadata)
	if err != nil {
		metadata = []byte("{}")
	}
	row := r.pool.QueryRow(ctx, `INSERT INTO audit_logs (
		actor_user_id, actor_email, action, resource_type, resource_id, metadata)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING `+auditColumns,
		nullUUID(log.ActorUserID), log.ActorEmail, log.Action, log.ResourceType, log.ResourceID, metadata)
	return scanLog(row)
}

func (r *PostgresRepository) List(ctx context.Context, filter ListFilter) ([]Log, error) {
	query := strings.Builder{}
	query.WriteString(selectAuditQuery + ` WHERE 1=1`)
	args := make([]any, 0, 5)
	argNum := 1

	if filter.ResourceType != "" {
		fmt.Fprintf(&query, ` AND resource_type = $%d`, argNum)
		args = append(args, filter.ResourceType)
		argNum++
	}
	if filter.ResourceID != "" {
		fmt.Fprintf(&query, ` AND resource_id = $%d`, argNum)
		args = append(args, filter.ResourceID)
		argNum++
	}
	if filter.ActorUserID != "" {
		fmt.Fprintf(&query, ` AND actor_user_id = $%d`, argNum)
		args = append(args, filter.ActorUserID)
		argNum++
	}
	query.WriteString(` ORDER BY created_at DESC`)
	if filter.Limit > 0 {
		fmt.Fprintf(&query, ` LIMIT $%d`, argNum)
		args = append(args, filter.Limit)
		argNum++
	}
	if filter.Offset > 0 {
		fmt.Fprintf(&query, ` OFFSET $%d`, argNum)
		args = append(args, filter.Offset)
	}

	rows, err := r.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]Log, 0)
	for rows.Next() {
		log, err := scanLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (Log, error) {
	log, err := scanLog(r.pool.QueryRow(ctx, selectAuditQuery+` WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Log{}, ErrNotFound
	}
	return log, err
}

func scanLog(row pgx.Row) (Log, error) {
	var (
		log         Log
		actorUserID *string
		metadata    []byte
	)
	if err := row.Scan(&log.ID, &actorUserID, &log.ActorEmail, &log.Action, &log.ResourceType,
		&log.ResourceID, &metadata, &log.CreatedAt); err != nil {
		return Log{}, err
	}
	if actorUserID != nil {
		log.ActorUserID = *actorUserID
	}
	log.Metadata = map[string]any{}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &log.Metadata)
	}
	return log, nil
}

// nullUUID maps an empty string to NULL so the UUID column accepts
// system-driven actions with no actor.
func nullUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
