package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/repository/mem"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestDuplicateWebhookDoesNotMoveCountersOrEnqueueTwice(t *testing.T) {
	store, paymentID := seedPaidPath(t)
	useCase := NewConfirmPayment(store.RafflesRepo(), store.OrdersRepo(), store.PaymentsRepo(), store.EventsRepo(), store.QueueRepo(), fixed(time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)), nil)
	input := ConfirmPaymentInput{EventID: "evt-1", PaymentID: paymentID, Status: "paid"}
	if err := useCase.Execute(context.Background(), input); err != nil {
		t.Fatalf("first confirm: %v", err)
	}
	raffle := onlyRaffle(t, store)
	if raffle.ReservedTickets != 0 || raffle.SoldTickets != 2 {
		t.Fatalf("counters after first confirm: reserved=%d sold=%d", raffle.ReservedTickets, raffle.SoldTickets)
	}
	if len(store.Queue) != 1 || len(store.Events) != 1 {
		t.Fatalf("queue=%d events=%d", len(store.Queue), len(store.Events))
	}
	if err := useCase.Execute(context.Background(), input); err != nil {
		t.Fatalf("replay: %v", err)
	}
	raffle = onlyRaffle(t, store)
	if raffle.ReservedTickets != 0 || raffle.SoldTickets != 2 || len(store.Queue) != 1 || len(store.Events) != 1 {
		t.Fatalf("replay changed state: reserved=%d sold=%d queue=%d events=%d", raffle.ReservedTickets, raffle.SoldTickets, len(store.Queue), len(store.Events))
	}
	if err := useCase.Execute(context.Background(), ConfirmPaymentInput{EventID: "evt-2", PaymentID: paymentID, Status: "paid"}); err != nil {
		t.Fatalf("second event: %v", err)
	}
	raffle = onlyRaffle(t, store)
	if raffle.SoldTickets != 2 || len(store.Queue) != 1 || len(store.Events) != 2 {
		t.Fatalf("second event reapplied sale: sold=%d queue=%d events=%d", raffle.SoldTickets, len(store.Queue), len(store.Events))
	}
}

func TestConfirmPaymentStoresTraceContext(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	ctx, span := provider.Tracer("test").Start(context.Background(), "webhook")
	store, paymentID := seedPaidPath(t)
	useCase := NewConfirmPayment(store.RafflesRepo(), store.OrdersRepo(), store.PaymentsRepo(), store.EventsRepo(), store.QueueRepo(), fixed(time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)), nil)
	if err := useCase.Execute(ctx, ConfirmPaymentInput{EventID: "evt-trace", PaymentID: paymentID, Status: "paid"}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	span.End()
	var payload []byte
	for _, item := range store.Queue {
		payload = item.Payload
	}
	if len(payload) == 0 || !contains(string(payload), "traceparent") {
		t.Fatalf("payload = %s", payload)
	}
}

func seedPaidPath(t *testing.T) (*mem.Store, uuid.UUID) {
	t.Helper()
	store := mem.New()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	raffleID := uuid.New()
	orderID := uuid.New()
	paymentID := uuid.New()
	store.Raffles[raffleID] = entity.Raffle{
		ID: raffleID, Slug: "car", Status: entity.RaffleStatusOpen, TotalTickets: 10, ReservedTickets: 2, TicketPriceCents: 500,
	}
	store.Orders[orderID] = entity.Order{
		ID: orderID, RaffleID: raffleID, Email: "a@b.com", Quantity: 2, AmountCents: 1000,
		Status: entity.OrderStatusPendingPayment, ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
	}
	store.Payments[paymentID] = entity.Payment{
		ID: paymentID, OrderID: orderID, Status: entity.PaymentStatusPending, QRCodePayload: "lab", CreatedAt: now, UpdatedAt: now,
	}
	return store, paymentID
}

func onlyRaffle(t *testing.T, store *mem.Store) entity.Raffle {
	t.Helper()
	for _, raffle := range store.Raffles {
		return raffle
	}
	t.Fatal("missing raffle")
	return entity.Raffle{}
}

func fixed(now time.Time) func() time.Time {
	return func() time.Time { return now }
}

func contains(value, part string) bool {
	return len(value) >= len(part) && (value == part || len(part) == 0 || (len(value) > 0 && (stringIndex(value, part) >= 0)))
}

func stringIndex(value, part string) int {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}

func TestUnknownPayment(t *testing.T) {
	store := mem.New()
	useCase := NewConfirmPayment(store.RafflesRepo(), store.OrdersRepo(), store.PaymentsRepo(), store.EventsRepo(), store.QueueRepo(), nil, nil)
	err := useCase.Execute(context.Background(), ConfirmPaymentInput{EventID: "evt", PaymentID: uuid.New(), Status: "paid"})
	if !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}
