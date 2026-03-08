package store

import (
	"context"
	"errors"
	"sort"
	"sync"

	"tower/internal/contracts"
)

var ErrSnapshotNotFound = errors.New("session snapshot not found")

type Repository interface {
	AppendEvent(ctx context.Context, event contracts.Event) error
	SaveSnapshot(ctx context.Context, snapshot contracts.SessionSnapshot) error
	Snapshot(ctx context.Context, sessionID contracts.SessionID) (contracts.SessionSnapshot, error)
	ListSnapshots(ctx context.Context) ([]contracts.SessionSnapshot, error)
	RecordAudit(ctx context.Context, entry contracts.AuditEntry) error
	Layout() Layout
}

// MemoryRepository keeps the bootstrap compileable without adding SQLite drivers yet.
type MemoryRepository struct {
	mu        sync.RWMutex
	layout    Layout
	events    []contracts.Event
	snapshots map[contracts.SessionID]contracts.SessionSnapshot
	audits    []contracts.AuditEntry
}

func NewMemoryRepository(layout Layout) *MemoryRepository {
	return &MemoryRepository{
		layout:    layout,
		snapshots: make(map[contracts.SessionID]contracts.SessionSnapshot),
	}
}

func (repo *MemoryRepository) AppendEvent(_ context.Context, event contracts.Event) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.events = append(repo.events, event)
	return nil
}

func (repo *MemoryRepository) SaveSnapshot(_ context.Context, snapshot contracts.SessionSnapshot) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.snapshots[snapshot.SessionID] = snapshot
	return nil
}

func (repo *MemoryRepository) Snapshot(_ context.Context, sessionID contracts.SessionID) (contracts.SessionSnapshot, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	snapshot, ok := repo.snapshots[sessionID]
	if !ok {
		return contracts.SessionSnapshot{}, ErrSnapshotNotFound
	}
	return snapshot, nil
}

func (repo *MemoryRepository) ListSnapshots(_ context.Context) ([]contracts.SessionSnapshot, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	snapshots := make([]contracts.SessionSnapshot, 0, len(repo.snapshots))
	for _, snapshot := range repo.snapshots {
		snapshots = append(snapshots, snapshot)
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].LastActivityAt.After(snapshots[j].LastActivityAt)
	})
	return snapshots, nil
}

func (repo *MemoryRepository) RecordAudit(_ context.Context, entry contracts.AuditEntry) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.audits = append(repo.audits, entry)
	return nil
}

func (repo *MemoryRepository) Layout() Layout {
	return repo.layout
}
