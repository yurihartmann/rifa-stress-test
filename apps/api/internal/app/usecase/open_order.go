package usecase

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/app/service"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/repository"
	"go.opentelemetry.io/otel/trace"
)

type OpenOrderInput struct {
	Slug     string
	Email    string
	Quantity int
}

type OpenOrder struct {
	raffles  repository.RaffleRepository
	orders   repository.OrderRepository
	payments repository.PaymentRepository
	hold     time.Duration
	now      func() time.Time
	recorder Recorder
}

func NewOpenOrder(
	raffles repository.RaffleRepository,
	orders repository.OrderRepository,
	payments repository.PaymentRepository,
	hold time.Duration,
	now func() time.Time,
	recorder Recorder,
) *OpenOrder {
	return &OpenOrder{
		raffles:  raffles,
		orders:   orders,
		payments: payments,
		hold:     hold,
		now:      clockOrNow(now),
		recorder: recorderOrNop(recorder),
	}
}

func (u *OpenOrder) Execute(ctx context.Context, input OpenOrderInput) (entity.Order, entity.Payment, error) {
	if input.Email == "" || input.Quantity <= 0 {
		return entity.Order{}, entity.Payment{}, entity.Validation("email and quantity are required")
	}
	raffle, err := u.raffles.FindOneByFilters(ctx, repository.Query{Filters: []repository.Filter{
		repository.Eq(repository.ColumnSlug, input.Slug),
	}})
	if err != nil {
		return entity.Order{}, entity.Payment{}, err
	}
	parent := trace.SpanFromContext(ctx)
	setSpanAttrs(parent, raffle.ID.String(), "", "", input.Quantity)
	if raffle.Status != entity.RaffleStatusOpen {
		return entity.Order{}, entity.Payment{}, entity.ErrRaffleClosed
	}
	if raffle.TicketPriceCents <= 0 || int64(input.Quantity) > math.MaxInt64/raffle.TicketPriceCents {
		return entity.Order{}, entity.Payment{}, entity.Validation("quantity is invalid")
	}

	filters, update := service.ReserveCapacity(input.Quantity)
	filters = append([]repository.Filter{repository.Eq(repository.ColumnID, raffle.ID)}, filters...)
	now := u.now()
	update.Assignments = service.Touch(now, update.Assignments...)

	capCtx, span := tracer.Start(ctx, "capacity.query")
	setSpanAttrs(span, raffle.ID.String(), "", "", input.Quantity)
	rows, err := u.raffles.Update(capCtx, repository.Query{Filters: filters}, update)
	span.End()
	if err != nil {
		return entity.Order{}, entity.Payment{}, fmt.Errorf("reserve capacity: %w", err)
	}
	if rows == 0 {
		return entity.Order{}, entity.Payment{}, u.reserveMiss(ctx, raffle.ID)
	}

	order := entity.Order{
		ID:          uuid.New(),
		RaffleID:    raffle.ID,
		Email:       input.Email,
		Quantity:    input.Quantity,
		AmountCents: raffle.TicketPriceCents * int64(input.Quantity),
		Status:      entity.OrderStatusPendingPayment,
		ExpiresAt:   now.Add(u.hold),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := u.orders.Create(ctx, order); err != nil {
		return entity.Order{}, entity.Payment{}, fmt.Errorf("create order: %w", err)
	}
	paymentID := uuid.New()
	payment := entity.Payment{
		ID:            paymentID,
		OrderID:       order.ID,
		Status:        entity.PaymentStatusPending,
		QRCodePayload: "lab-payment:" + paymentID.String(),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := u.payments.Create(ctx, payment); err != nil {
		return entity.Order{}, entity.Payment{}, fmt.Errorf("create payment: %w", err)
	}
	setSpanAttrs(parent, raffle.ID.String(), order.ID.String(), payment.ID.String(), input.Quantity)
	u.recorder.OrderEntered(ctx, string(order.Status))
	return order, payment, nil
}

func (u *OpenOrder) reserveMiss(ctx context.Context, id uuid.UUID) error {
	raffle, err := u.raffles.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if raffle.Status != entity.RaffleStatusOpen {
		return entity.ErrRaffleClosed
	}
	return entity.ErrInsufficientTickets
}
