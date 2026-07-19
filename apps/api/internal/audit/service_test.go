package audit

import (
	"context"
	"errors"
	"testing"
)

func TestRecordRequiresActionAndResource(t *testing.T) {
	service := NewService(&memoryRepo{})
	_, err := service.Record(context.Background(), Entry{ActorEmail: "admin@shop.id"})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestRecordDefaultsMetadata(t *testing.T) {
	repo := &memoryRepo{}
	service := NewService(repo)
	log, err := service.Record(context.Background(), Entry{
		ActorEmail:   "admin@shop.id",
		Action:       "voucher.create",
		ResourceType: "voucher",
		ResourceID:   "vch-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if log.Metadata == nil {
		t.Fatal("expected metadata to default to empty map")
	}
	if repo.recorded.Action != "voucher.create" {
		t.Fatalf("expected action recorded, got %q", repo.recorded.Action)
	}
}

func TestListAppliesDefaultLimit(t *testing.T) {
	repo := &memoryRepo{}
	service := NewService(repo)
	if _, err := service.List(context.Background(), ListFilter{}); err != nil {
		t.Fatal(err)
	}
	if repo.lastFilter.Limit != 50 {
		t.Fatalf("expected default limit 50, got %d", repo.lastFilter.Limit)
	}
}

// --- test double ---

type memoryRepo struct {
	recorded   Log
	lastFilter ListFilter
}

func (r *memoryRepo) Record(_ context.Context, log Log) (Log, error) {
	log.ID = "log-1"
	r.recorded = log
	return log, nil
}

func (r *memoryRepo) List(_ context.Context, filter ListFilter) ([]Log, error) {
	r.lastFilter = filter
	return []Log{r.recorded}, nil
}

func (r *memoryRepo) GetByID(_ context.Context, id string) (Log, error) {
	if r.recorded.ID == id && r.recorded.ID != "" {
		return r.recorded, nil
	}
	return Log{}, ErrNotFound
}
