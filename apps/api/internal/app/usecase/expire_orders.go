package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/app/service"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/repository"
	"go.opentelemetry.io/otel/trace"
)

type ExpireOrders struct {
	raffles  repository.RaffleRepository
	orders   repository.OrderRepository
	now      func() time.Time
	recorder Recorder
}

func NewExpireOrders(raffles repository.RaffleRepository, orders repository.OrderRepository, now func() time.Time, recorder Recorder) *ExpireOrders {
	return &ExpireOrders{raffles: raffles, orders: orders, now: clockOrNow(now), recorder: recorderOrNop(recorder)}
}

func (u *ExpireOrders) Execute(ctx context.Context) (bool, error) {
	now := u.now()
	orders, err := u.orders.FindAllByFilters(ctx, repository.Query{
		Filters: []repository.Filter{
			repository.Eq(repository.ColumnStatus, string(entity.OrderStatusPendingPayment)),
			repository.Lte(repository.ColumnExpiresAt, now),
		},
		Sorts: []repository.Sort{{Column: repository.ColumnExpiresAt}},
		Limit: 1,
		Lock:  repository.LockUpdateSkipLocked,
	})
	if err != nil {
		return false, fmt.Errorf("find expired order: %w", err)
	}
	if len(orders) == 0 {
		return false, nil
	}
	order := orders[0]
	parent := trace.SpanFromContext(ctx)
	setSpanAttrs(parent, order.RaffleID.String(), order.ID.String(), "", order.Quantity)

	rows, err := u.orders.Update(ctx, repository.Query{Filters: []repository.Filter{
		repository.Eq(repository.ColumnID, order.ID),
		repository.Eq(repository.ColumnStatus, string(entity.OrderStatusPendingPayment)),
	}}, repository.Update{Assignments: []repository.Assignment{
		{Column: repository.ColumnStatus, Value: string(entity.OrderStatusExpired)},
		{Column: repository.ColumnUpdatedAt, Value: now},
	}})
	if err != nil {
		return false, fmt.Errorf("expire order: %w", err)
	}
	if rows == 0 {
		return true, nil
	}

	filters, update := service.ReleaseReserved(order.Quantity)
	filters = append([]repository.Filter{repository.Eq(repository.ColumnID, order.RaffleID)}, filters...)
	update.Assignments = service.Touch(now, update.Assignments...)
	capCtx, span := tracer.Start(ctx, "capacity.query")
	setSpanAttrs(span, order.RaffleID.String(), order.ID.String(), "", order.Quantity)
	rows, err = u.raffles.Update(capCtx, repository.Query{Filters: filters}, update)
	span.End()
	if err != nil {
		return false, fmt.Errorf("release reserved tickets: %w", err)
	}
	if rows == 0 {
		return false, fmt.Errorf("release reserved tickets: no raffle row")
	}
	u.recorder.OrderEntered(ctx, string(entity.OrderStatusExpired))
	return true, nil
}
