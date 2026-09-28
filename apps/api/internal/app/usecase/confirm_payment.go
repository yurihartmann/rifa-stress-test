package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/app/service"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/repository"
	"go.opentelemetry.io/otel/trace"
)

type ConfirmPaymentInput struct {
	EventID   string
	PaymentID uuid.UUID
	Status    string
}

type ConfirmPayment struct {
	raffles  repository.RaffleRepository
	orders   repository.OrderRepository
	payments repository.PaymentRepository
	events   repository.PaymentEventRepository
	queue    repository.QueueRepository
	now      func() time.Time
	recorder Recorder
}

func NewConfirmPayment(
	raffles repository.RaffleRepository,
	orders repository.OrderRepository,
	payments repository.PaymentRepository,
	events repository.PaymentEventRepository,
	queue repository.QueueRepository,
	now func() time.Time,
	recorder Recorder,
) *ConfirmPayment {
	return &ConfirmPayment{
		raffles:  raffles,
		orders:   orders,
		payments: payments,
		events:   events,
		queue:    queue,
		now:      clockOrNow(now),
		recorder: recorderOrNop(recorder),
	}
}

func (u *ConfirmPayment) Execute(ctx context.Context, input ConfirmPaymentInput) error {
	if input.EventID == "" || input.PaymentID == uuid.Nil {
		return entity.Validation("event_id and payment_id are required")
	}
	if input.Status != string(entity.PaymentStatusPaid) {
		return entity.Validation("status must be paid")
	}
	payment, err := u.payments.FindByID(ctx, input.PaymentID)
	if err != nil {
		return err
	}
	order, err := u.orders.FindByID(ctx, payment.OrderID)
	if err != nil {
		return err
	}
	parent := trace.SpanFromContext(ctx)
	setSpanAttrs(parent, order.RaffleID.String(), order.ID.String(), payment.ID.String(), order.Quantity)

	existing, err := u.events.FindByID(ctx, input.EventID)
	if err == nil && existing.EventID == input.EventID {
		return nil
	}
	if err != nil && !errors.Is(err, entity.ErrNotFound) {
		return fmt.Errorf("load payment event: %w", err)
	}

	now := u.now()
	if payment.Status == entity.PaymentStatusPaid {
		return u.recordEvent(ctx, input.EventID, payment.ID, now)
	}
	if order.Status == entity.OrderStatusExpired || !order.ExpiresAt.After(now) {
		return entity.ErrOrderExpired
	}

	rows, err := u.orders.Update(ctx, repository.Query{Filters: []repository.Filter{
		repository.Eq(repository.ColumnID, order.ID),
		repository.Eq(repository.ColumnStatus, string(entity.OrderStatusPendingPayment)),
		repository.Gt(repository.ColumnExpiresAt, now),
	}}, repository.Update{Assignments: []repository.Assignment{
		{Column: repository.ColumnStatus, Value: string(entity.OrderStatusPaid)},
		{Column: repository.ColumnUpdatedAt, Value: now},
	}})
	if err != nil {
		return fmt.Errorf("mark order paid: %w", err)
	}
	if rows == 0 {
		return u.lostRace(ctx, input.EventID, payment.ID, order.ID, now)
	}

	paidAt := now
	rows, err = u.payments.Update(ctx, repository.Query{Filters: []repository.Filter{
		repository.Eq(repository.ColumnID, payment.ID),
		repository.Eq(repository.ColumnStatus, string(entity.PaymentStatusPending)),
	}}, repository.Update{Assignments: []repository.Assignment{
		{Column: repository.ColumnStatus, Value: string(entity.PaymentStatusPaid)},
		{Column: "paid_at", Value: paidAt},
		{Column: repository.ColumnUpdatedAt, Value: now},
	}})
	if err != nil {
		return fmt.Errorf("mark payment paid: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("mark payment paid: no pending payment")
	}

	filters, update := service.MoveReservedToSold(order.Quantity)
	filters = append([]repository.Filter{repository.Eq(repository.ColumnID, order.RaffleID)}, filters...)
	update.Assignments = service.Touch(now, update.Assignments...)
	capCtx, span := tracer.Start(ctx, "capacity.query")
	setSpanAttrs(span, order.RaffleID.String(), order.ID.String(), payment.ID.String(), order.Quantity)
	rows, err = u.raffles.Update(capCtx, repository.Query{Filters: filters}, update)
	span.End()
	if err != nil {
		return fmt.Errorf("move reserved tickets: %w", err)
	}
	if rows == 0 {
		return entity.ErrInsufficientTickets
	}

	if err := u.recordEvent(ctx, input.EventID, payment.ID, now); err != nil {
		return err
	}
	payload, err := service.QueuePayload(ctx, order.ID, order.RaffleID, payment.ID, order.Quantity)
	if err != nil {
		return fmt.Errorf("encode queue payload: %w", err)
	}
	item := entity.QueueItem{
		ID:          uuid.New(),
		Type:        entity.QueueTypeGenerateTicket,
		AggregateID: order.ID,
		Payload:     payload,
		Status:      entity.QueueStatusPending,
		AvailableAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := u.queue.Create(ctx, item); err != nil {
		return fmt.Errorf("enqueue ticket generation: %w", err)
	}
	u.recorder.OrderEntered(ctx, string(entity.OrderStatusPaid))
	return nil
}

func (u *ConfirmPayment) lostRace(ctx context.Context, eventID string, paymentID, orderID uuid.UUID, now time.Time) error {
	existing, err := u.events.FindByID(ctx, eventID)
	if err == nil && existing.EventID == eventID {
		return nil
	}
	if err != nil && !errors.Is(err, entity.ErrNotFound) {
		return err
	}
	order, err := u.orders.FindByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status == entity.OrderStatusPaid {
		return u.recordEvent(ctx, eventID, paymentID, now)
	}
	return entity.ErrOrderExpired
}

func (u *ConfirmPayment) recordEvent(ctx context.Context, eventID string, paymentID uuid.UUID, now time.Time) error {
	err := u.events.Create(ctx, entity.PaymentEvent{EventID: eventID, PaymentID: paymentID, CreatedAt: now})
	if err != nil && !errors.Is(err, entity.ErrAlreadyExists) {
		return fmt.Errorf("record payment event: %w", err)
	}
	return nil
}
