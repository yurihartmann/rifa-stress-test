package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/repository"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/repository/mem"
)

func TestGenerateTicketRetryDoesNotLinkExtraNumbers(t *testing.T) {
	store := mem.New()
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	raffleID := uuid.New()
	orderID := uuid.New()
	jobID := uuid.New()
	store.Orders[orderID] = entity.Order{ID: orderID, RaffleID: raffleID, Quantity: 2, Status: entity.OrderStatusPaid}
	for number := 1; number <= 2; number++ {
		id := uuid.New()
		linked := orderID
		store.Tickets[id] = entity.Ticket{ID: id, RaffleID: raffleID, OrderID: &linked, Number: number, CreatedAt: now}
	}
	for number := 3; number <= 5; number++ {
		id := uuid.New()
		store.Tickets[id] = entity.Ticket{ID: id, RaffleID: raffleID, Number: number, CreatedAt: now}
	}
	store.Queue[jobID] = entity.QueueItem{ID: jobID, Type: entity.QueueTypeGenerateTicket, AggregateID: orderID, Status: entity.QueueStatusProcessing, Attempts: 1}
	useCase := NewGenerateTicket(store.OrdersRepo(), store.TicketsRepo(), store.QueueRepo(), fixed(now), nil)
	job := store.Queue[jobID]
	if err := useCase.Execute(context.Background(), job); err != nil {
		t.Fatalf("first: %v", err)
	}
	if countLinked(store, orderID) != 2 || store.Queue[jobID].Status != entity.QueueStatusDone {
		t.Fatalf("linked=%d status=%s", countLinked(store, orderID), store.Queue[jobID].Status)
	}
	if err := useCase.Execute(context.Background(), store.Queue[jobID]); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if countLinked(store, orderID) != 2 {
		t.Fatalf("retry linked %d", countLinked(store, orderID))
	}
}

func TestGenerateTicketDrawRequestsSkipLocked(t *testing.T) {
	store := mem.New()
	now := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	raffleID := uuid.New()
	orderID := uuid.New()
	jobID := uuid.New()
	store.Orders[orderID] = entity.Order{ID: orderID, RaffleID: raffleID, Quantity: 2, Status: entity.OrderStatusPaid}
	for number := 1; number <= 5; number++ {
		id := uuid.New()
		store.Tickets[id] = entity.Ticket{ID: id, RaffleID: raffleID, Number: number, CreatedAt: now}
	}
	store.Queue[jobID] = entity.QueueItem{ID: jobID, AggregateID: orderID, Status: entity.QueueStatusProcessing}
	useCase := NewGenerateTicket(store.OrdersRepo(), store.TicketsRepo(), store.QueueRepo(), fixed(now), nil)
	if err := useCase.Execute(context.Background(), store.Queue[jobID]); err != nil {
		t.Fatalf("draw: %v", err)
	}
	if countLinked(store, orderID) != 2 {
		t.Fatalf("linked=%d", countLinked(store, orderID))
	}
	if !sawSkipLocked(store) {
		t.Fatal("draw did not request skip locked")
	}
	if err := useCase.Execute(context.Background(), store.Queue[jobID]); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if countLinked(store, orderID) != 2 {
		t.Fatalf("retry linked=%d", countLinked(store, orderID))
	}
}

func TestGenerateTicketShortfall(t *testing.T) {
	store := mem.New()
	raffleID := uuid.New()
	orderID := uuid.New()
	jobID := uuid.New()
	store.Orders[orderID] = entity.Order{ID: orderID, RaffleID: raffleID, Quantity: 2, Status: entity.OrderStatusPaid}
	store.Tickets[uuid.New()] = entity.Ticket{ID: uuid.New(), RaffleID: raffleID, Number: 1}
	store.Queue[jobID] = entity.QueueItem{ID: jobID, AggregateID: orderID, Status: entity.QueueStatusProcessing}
	useCase := NewGenerateTicket(store.OrdersRepo(), store.TicketsRepo(), store.QueueRepo(), nil, nil)
	err := useCase.Execute(context.Background(), store.Queue[jobID])
	if !errors.Is(err, entity.ErrTicketShortfall) {
		t.Fatalf("error = %v", err)
	}
	if store.Queue[jobID].Status == entity.QueueStatusDone {
		t.Fatal("shortfall marked the job done")
	}
}

func TestRecordQueueFailureKeepsTheSale(t *testing.T) {
	store := mem.New()
	jobID := uuid.New()
	locked := time.Now().UTC()
	store.Queue[jobID] = entity.QueueItem{
		ID: jobID, Status: entity.QueueStatusProcessing, Attempts: 3, LockedAt: &locked, LockedBy: "worker",
	}
	useCase := NewRecordQueueFailure(store.QueueRepo(), 3, nil)
	if err := useCase.Execute(context.Background(), jobID, entity.ErrTicketShortfall); err != nil {
		t.Fatalf("record: %v", err)
	}
	job := store.Queue[jobID]
	if job.Status != entity.QueueStatusFailed || job.Attempts != 3 || job.LockedAt != nil {
		t.Fatalf("job = %+v", job)
	}
}

func TestClaimReleasesStaleWithoutExtraAttempt(t *testing.T) {
	store := mem.New()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	jobID := uuid.New()
	locked := now.Add(-time.Hour)
	store.Queue[jobID] = entity.QueueItem{
		ID: jobID, Type: entity.QueueTypeGenerateTicket, AggregateID: uuid.New(), Status: entity.QueueStatusProcessing,
		Attempts: 2, AvailableAt: now.Add(-time.Hour), LockedAt: &locked, LockedBy: "old",
	}
	useCase := NewClaimQueue(store.QueueRepo(), "worker-1", time.Minute, fixed(now))
	job, found, err := useCase.Execute(context.Background())
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if job.Attempts != 3 || job.Status != entity.QueueStatusProcessing || job.LockedBy != "worker-1" {
		t.Fatalf("job = %+v", job)
	}
}

func countLinked(store *mem.Store, orderID uuid.UUID) int {
	count := 0
	for _, ticket := range store.Tickets {
		if ticket.OrderID != nil && *ticket.OrderID == orderID {
			count++
		}
	}
	return count
}

func sawSkipLocked(store *mem.Store) bool {
	for _, query := range store.Finds {
		if query.Lock == repository.LockUpdateSkipLocked {
			return true
		}
	}
	return false
}
