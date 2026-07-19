// Package audit implements Phase 10 (BR-049): a durable record of admin actions.
// Every mutating admin request (create/update/delete on catalog, vouchers,
// orders, etc.) is captured with the acting user, the action, the target
// resource, and a small metadata blob. Reads are exposed to admins for review.
package audit

import (
	"errors"
	"time"
)

// ErrValidation is returned when a log entry is missing required fields.
var ErrValidation = errors.New("invalid audit log entry")

// Log is a single audit record (mirrors the audit_logs table in migration 000007).
type Log struct {
	ID           string         `json:"id"`
	ActorUserID  string         `json:"actor_user_id,omitempty"`
	ActorEmail   string         `json:"actor_email"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}

// Entry is the input used to record an action. ActorUserID is optional (some
// actions are system-driven); Action + ResourceType are required.
type Entry struct {
	ActorUserID  string
	ActorEmail   string
	Action       string
	ResourceType string
	ResourceID   string
	Metadata     map[string]any
}

// ListFilter for admin audit-log queries.
type ListFilter struct {
	ResourceType string
	ResourceID   string
	ActorUserID  string
	Limit        int
	Offset       int
}
