package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/repository/mem"
)

func TestExpireWinsWhenItRunsFirst(t *testing.T) {
	store, paymentID := seedHold(t, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))
	expire := NewExpireOrders(store.RafflesRepo(), store.OrdersRepo(), fixed(time.Date(2026, 6, 1, 14, 0, 0, 0, time.UTC)), nil)
	confirm := NewConfirmPayment(store.RafflesRepo(), store.OrdersRepo(), store.PaymentsRepo(), store.EventsRepo(), store.QueueRepo(), fixed(time.Date(2026, 6, 1, 14, 0, 0, 0, time.UTC)), nil)
	worked, err := expire.Execute(context.Background())
	if err != nil || !worked {
		t.Fatalf("expire worked=%v err=%v", worked, err)
	}
	err = confirm.Execute(context.Background(), ConfirmPaymentInput{EventID: "evt", PaymentID: paymentID, Status: "paid"})
	if !errors.Is(err, entity.ErrOrderExpired) {
		t.Fatalf("confirm error = %v", err)
	}
	assertSingleWinner(t, store, false)
}

func TestPayWinsWhenExpireIsLate(t *testing.T) {
	store, paymentID := seedHold(t, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))
	confirm := NewConfirmPayment(store.RafflesRepo(), store.OrdersRepo(), store.PaymentsRepo(), store.EventsRepo(), store.QueueRepo(), fixed(time.Date(2026, 6, 1, 12, 30, 0, 0, time.UTC)), nil)
	expire := NewExpireOrders(store.RafflesRepo(), store.OrdersRepo(), fixed(time.Date(2026, 6, 1, 15, 0, 0, 0, time.UTC)), nil)
	if err := confirm.Execute(context.Background(), ConfirmPaymentInput{EventID: "evt", PaymentID: paymentID, Status: "paid"}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	worked, err := expire.Execute(context.Background())
	if err != nil || worked {
		t.Fatalf("expire worked=%v err=%v", worked, err)
	}
	assertSingleWinner(t, store, true)
}

func TestExpireAndPayOnlyOneWins(t *testing.T) {
	for i := 0; i < 40; i++ {
		base := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
		store, paymentID := seedHold(t, base)
		confirm := NewConfirmPayment(store.RafflesRepo(), store.OrdersRepo(), store.PaymentsRepo(), store.EventsRepo(), store.QueueRepo(), fixed(base), nil)
		expire := NewExpireOrders(store.RafflesRepo(), store.OrdersRepo(), fixed(base.Add(2*time.Hour)), nil)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_ = confirm.Execute(context.Background(), ConfirmPaymentInput{EventID: "evt", PaymentID: paymentID, Status: "paid"})
		}()
		go func() {
			defer wg.Done()
			<-start
			_, _ = expire.Execute(context.Background())
		}()
		close(start)
		wg.Wait()
		paid := onlyOrder(t, store).Status == entity.OrderStatusPaid
		assertSingleWinner(t, store, paid)
	}
}

func seedHold(t *testing.T, now time.Time) (*mem.Store, uuid.UUID) {
	t.Helper()
	store := mem.New()
	raffleID := uuid.New()
	orderID := uuid.New()
	paymentID := uuid.New()
	store.Raffles[raffleID] = entity.Raffle{
		ID: raffleID, Status: entity.RaffleStatusOpen, TotalTickets: 10, ReservedTickets: 4, TicketPriceCents: 100,
	}
	store.Orders[orderID] = entity.Order{
		ID: orderID, RaffleID: raffleID, Email: "a@b.com", Quantity: 2, AmountCents: 200,
		Status: entity.OrderStatusPendingPayment, ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
	}
	store.Payments[paymentID] = entity.Payment{
		ID: paymentID, OrderID: orderID, Status: entity.PaymentStatusPending, CreatedAt: now, UpdatedAt: now,
	}
	return store, paymentID
}

func assertSingleWinner(t *testing.T, store *mem.Store, paid bool) {
	t.Helper()
	raffle := onlyRaffle(t, store)
	order := onlyOrder(t, store)
	payment := onlyPayment(t, store)
	if raffle.ReservedTickets < 0 || raffle.SoldTickets < 0 || raffle.ReservedTickets+raffle.SoldTickets > raffle.TotalTickets {
		t.Fatalf("capacity invariant broken: %+v", raffle)
	}
	if paid {
		if order.Status != entity.OrderStatusPaid || payment.Status != entity.PaymentStatusPaid {
			t.Fatalf("paid path status order=%s payment=%s", order.Status, payment.Status)
		}
		if raffle.ReservedTickets != 2 || raffle.SoldTickets != 2 || len(store.Queue) != 1 {
			t.Fatalf("paid path counters reserved=%d sold=%d queue=%d", raffle.ReservedTickets, raffle.SoldTickets, len(store.Queue))
		}
		return
	}
	if order.Status != entity.OrderStatusExpired || payment.Status != entity.PaymentStatusPending {
		t.Fatalf("expired path status order=%s payment=%s", order.Status, payment.Status)
	}
	if raffle.ReservedTickets != 2 || raffle.SoldTickets != 0 || len(store.Queue) != 0 {
		t.Fatalf("expired path counters reserved=%d sold=%d queue=%d", raffle.ReservedTickets, raffle.SoldTickets, len(store.Queue))
	}
}

func onlyOrder(t *testing.T, store *mem.Store) entity.Order {
	t.Helper()
	for _, order := range store.Orders {
		return order
	}
	t.Fatal("missing order")
	return entity.Order{}
}

func onlyPayment(t *testing.T, store *mem.Store) entity.Payment {
	t.Helper()
	for _, payment := range store.Payments {
		return payment
	}
	t.Fatal("missing payment")
	return entity.Payment{}
}
