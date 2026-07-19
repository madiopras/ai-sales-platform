package audit

import (
	"context"
	"strings"
)

// Service records admin actions and exposes read access for review (BR-049).
type Service struct {
	repository Repository
}

// NewService returns an audit Service backed by the given repository.
func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// Record appends a single audit entry. Recording is best-effort from the
// caller's perspective — a failure to write the log must never block the
// underlying admin action — but this method still surfaces the error so a
// caller (or middleware) can log it.
func (s *Service) Record(ctx context.Context, entry Entry) (Log, error) {
	action := strings.TrimSpace(entry.Action)
	resourceType := strings.TrimSpace(entry.ResourceType)
	if action == "" || resourceType == "" {
		return Log{}, ErrValidation
	}
	metadata := entry.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	return s.repository.Record(ctx, Log{
		ActorUserID:  strings.TrimSpace(entry.ActorUserID),
		ActorEmail:   strings.TrimSpace(entry.ActorEmail),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   strings.TrimSpace(entry.ResourceID),
		Metadata:     metadata,
	})
}

// List returns audit logs for the admin review UI with optional filters.
func (s *Service) List(ctx context.Context, filter ListFilter) ([]Log, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.repository.List(ctx, filter)
}

// Get returns a single audit log by id.
func (s *Service) Get(ctx context.Context, id string) (Log, error) {
	return s.repository.GetByID(ctx, strings.TrimSpace(id))
}
