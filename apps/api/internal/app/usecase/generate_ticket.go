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

type GenerateTicket struct {
	orders   repository.OrderRepository
	tickets  repository.TicketRepository
	queue    repository.QueueRepository
	now      func() time.Time
	recorder Recorder
}

func NewGenerateTicket(
	orders repository.OrderRepository,
	tickets repository.TicketRepository,
	queue repository.QueueRepository,
	now func() time.Time,
	recorder Recorder,
) *GenerateTicket {
	return &GenerateTicket{
		orders:   orders,
		tickets:  tickets,
		queue:    queue,
		now:      clockOrNow(now),
		recorder: recorderOrNop(recorder),
	}
}

func (u *GenerateTicket) Execute(ctx context.Context, job entity.QueueItem) error {
	order, err := u.orders.FindByID(ctx, job.AggregateID)
	if err != nil {
		return err
	}
	setSpanAttrs(trace.SpanFromContext(ctx), order.RaffleID.String(), order.ID.String(), "", order.Quantity)
	if order.Status != entity.OrderStatusPaid {
		return entity.ErrOrderNotPaid
	}
	linked, err := u.tickets.FindAllByFilters(ctx, repository.Query{Filters: []repository.Filter{
		repository.Eq(repository.ColumnOrderID, order.ID),
	}})
	if err != nil {
		return fmt.Errorf("count linked tickets: %w", err)
	}
	if len(linked) >= order.Quantity {
		return u.markDone(ctx, job.ID)
	}

	remaining := order.Quantity - len(linked)
	now := u.now()
	assigned := 0
	for attempt := 0; attempt < 2 && remaining > 0; attempt++ {
		picked, err := u.tickets.FindAllByFilters(ctx, repository.Query{
			Filters: []repository.Filter{
				repository.Eq(repository.ColumnRaffleID, order.RaffleID),
				repository.IsNull(repository.ColumnOrderID),
			},
			Sorts: []repository.Sort{{Random: true}},
			Limit: remaining,
			Lock:  repository.LockUpdateSkipLocked,
		})
		if err != nil {
			return fmt.Errorf("draw tickets: %w", err)
		}
		if len(picked) == 0 {
			continue
		}
		ids := make([]uuid.UUID, len(picked))
		for i, ticket := range picked {
			ids[i] = ticket.ID
		}
		rows, err := u.tickets.Update(ctx, repository.Query{Filters: []repository.Filter{
			repository.In(repository.ColumnID, ids),
			repository.IsNull(repository.ColumnOrderID),
		}}, repository.Update{Assignments: []repository.Assignment{
			{Column: repository.ColumnOrderID, Value: order.ID},
			{Column: "assigned_at", Value: now},
		}})
		if err != nil {
			return fmt.Errorf("link tickets: %w", err)
		}
		assigned += int(rows)
		remaining -= int(rows)
	}
	if remaining > 0 {
		return entity.ErrTicketShortfall
	}
	if err := u.markDone(ctx, job.ID); err != nil {
		return err
	}
	u.recorder.TicketsAssigned(ctx, assigned)
	return nil
}

func (u *GenerateTicket) markDone(ctx context.Context, id uuid.UUID) error {
	now := u.now()
	rows, err := u.queue.Update(ctx, repository.Query{Filters: []repository.Filter{
		repository.Eq(repository.ColumnID, id),
	}}, repository.Update{Assignments: []repository.Assignment{
		{Column: repository.ColumnStatus, Value: string(entity.QueueStatusDone)},
		{Column: repository.ColumnLockedAt, SetNull: true},
		{Column: "locked_by", SetNull: true},
		{Column: repository.ColumnUpdatedAt, Value: now},
	}})
	if err != nil {
		return fmt.Errorf("mark queue done: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("mark queue done: no row")
	}
	return nil
}
