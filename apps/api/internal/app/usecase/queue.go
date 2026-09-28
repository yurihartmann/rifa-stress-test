package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/repository"
	"go.opentelemetry.io/otel/trace"
)

type ClaimQueue struct {
	queue       repository.QueueRepository
	workerID    string
	lockTimeout time.Duration
	now         func() time.Time
}

func NewClaimQueue(queue repository.QueueRepository, workerID string, lockTimeout time.Duration, now func() time.Time) *ClaimQueue {
	return &ClaimQueue{queue: queue, workerID: workerID, lockTimeout: lockTimeout, now: clockOrNow(now)}
}

func (u *ClaimQueue) Execute(ctx context.Context) (entity.QueueItem, bool, error) {
	now := u.now()
	if err := u.releaseStale(ctx, now); err != nil {
		return entity.QueueItem{}, false, err
	}
	jobs, err := u.queue.FindAllByFilters(ctx, repository.Query{
		Filters: []repository.Filter{
			repository.Eq(repository.ColumnStatus, string(entity.QueueStatusPending)),
			repository.Eq(repository.ColumnType, entity.QueueTypeGenerateTicket),
			repository.Lte(repository.ColumnAvailableAt, now),
		},
		Sorts: []repository.Sort{{Column: repository.ColumnAvailableAt}},
		Limit: 1,
		Lock:  repository.LockUpdateSkipLocked,
	})
	if err != nil {
		return entity.QueueItem{}, false, fmt.Errorf("find queue job: %w", err)
	}
	if len(jobs) == 0 {
		return entity.QueueItem{}, false, nil
	}
	job := jobs[0]
	lockedAt := now
	rows, err := u.queue.Update(ctx, repository.Query{Filters: []repository.Filter{
		repository.Eq(repository.ColumnID, job.ID),
		repository.Eq(repository.ColumnStatus, string(entity.QueueStatusPending)),
	}}, repository.Update{Assignments: []repository.Assignment{
		{Column: repository.ColumnStatus, Value: string(entity.QueueStatusProcessing)},
		{Column: repository.ColumnAttempts, Expr: "attempts + 1"},
		{Column: repository.ColumnLockedAt, Value: lockedAt},
		{Column: "locked_by", Value: u.workerID},
		{Column: repository.ColumnUpdatedAt, Value: now},
	}})
	if err != nil {
		return entity.QueueItem{}, false, fmt.Errorf("claim queue job: %w", err)
	}
	if rows == 0 {
		return entity.QueueItem{}, false, nil
	}
	job.Status = entity.QueueStatusProcessing
	job.Attempts++
	job.LockedAt = &lockedAt
	job.LockedBy = u.workerID
	job.UpdatedAt = now
	setSpanAttrs(trace.SpanFromContext(ctx), "", job.AggregateID.String(), "", 0)
	return job, true, nil
}

func (u *ClaimQueue) releaseStale(ctx context.Context, now time.Time) error {
	cutoff := now.Add(-u.lockTimeout)
	_, err := u.queue.Update(ctx, repository.Query{Filters: []repository.Filter{
		repository.Eq(repository.ColumnStatus, string(entity.QueueStatusProcessing)),
		repository.Lt(repository.ColumnLockedAt, cutoff),
	}}, repository.Update{Assignments: []repository.Assignment{
		{Column: repository.ColumnStatus, Value: string(entity.QueueStatusPending)},
		{Column: repository.ColumnLockedAt, SetNull: true},
		{Column: "locked_by", SetNull: true},
		{Column: repository.ColumnUpdatedAt, Value: now},
	}})
	if err != nil {
		return fmt.Errorf("release stale queue lock: %w", err)
	}
	return nil
}

type RecordQueueFailure struct {
	queue       repository.QueueRepository
	maxAttempts int
	now         func() time.Time
}

func NewRecordQueueFailure(queue repository.QueueRepository, maxAttempts int, now func() time.Time) *RecordQueueFailure {
	return &RecordQueueFailure{queue: queue, maxAttempts: maxAttempts, now: clockOrNow(now)}
}

func (u *RecordQueueFailure) Execute(ctx context.Context, id uuid.UUID, cause error) error {
	job, err := u.queue.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if job.Status != entity.QueueStatusProcessing {
		return nil
	}
	now := u.now()
	status := entity.QueueStatusPending
	assignments := []repository.Assignment{
		{Column: "last_error", Value: truncate(cause.Error(), 512)},
		{Column: repository.ColumnLockedAt, SetNull: true},
		{Column: "locked_by", SetNull: true},
		{Column: repository.ColumnUpdatedAt, Value: now},
	}
	if job.Attempts >= u.maxAttempts {
		status = entity.QueueStatusFailed
	} else {
		assignments = append(assignments, repository.Assignment{Column: repository.ColumnAvailableAt, Value: now})
	}
	assignments = append([]repository.Assignment{{Column: repository.ColumnStatus, Value: string(status)}}, assignments...)
	rows, err := u.queue.Update(ctx, repository.Query{Filters: []repository.Filter{
		repository.Eq(repository.ColumnID, job.ID),
		repository.Eq(repository.ColumnStatus, string(entity.QueueStatusProcessing)),
	}}, repository.Update{Assignments: assignments})
	if err != nil {
		return fmt.Errorf("record queue failure: %w", err)
	}
	if rows == 0 {
		return nil
	}
	return nil
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
