package audit

import "context"

// Repository is the persistence boundary for audit logs.
//
// Contracts:
//   - Record: append a single immutable log row.
//   - List / GetByID: read access for the admin review UI.
type Repository interface {
	Record(ctx context.Context, log Log) (Log, error)
	List(ctx context.Context, filter ListFilter) ([]Log, error)
	GetByID(ctx context.Context, id string) (Log, error)
}
