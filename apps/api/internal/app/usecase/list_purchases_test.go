package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/repository/mem"
)

func TestListPurchasesIsolatesEmailAndFormatsNumbers(t *testing.T) {
	store := mem.New()
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	raffleID := uuid.New()
	orderA := uuid.New()
	orderB := uuid.New()
	store.Raffles[raffleID] = entity.Raffle{ID: raffleID, Slug: "car", Title: "Carro", TotalTickets: 1000}
	store.Orders[orderA] = entity.Order{ID: orderA, RaffleID: raffleID, Email: "a@b.com", Quantity: 1, AmountCents: 100, Status: entity.OrderStatusPaid, CreatedAt: now}
	store.Orders[orderB] = entity.Order{ID: orderB, RaffleID: raffleID, Email: "other@b.com", Quantity: 1, AmountCents: 100, Status: entity.OrderStatusPaid, CreatedAt: now.Add(time.Minute)}
	linked := orderA
	store.Tickets[uuid.New()] = entity.Ticket{ID: uuid.New(), RaffleID: raffleID, OrderID: &linked, Number: 1}
	free := uuid.New()
	store.Tickets[free] = entity.Ticket{ID: free, RaffleID: raffleID, Number: 2}
	other := orderB
	store.Tickets[uuid.New()] = entity.Ticket{ID: uuid.New(), RaffleID: raffleID, OrderID: &other, Number: 3}

	useCase := NewListPurchasesByEmail(store.OrdersRepo(), store.RafflesRepo(), store.TicketsRepo())
	empty, err := useCase.Execute(context.Background(), "nobody@b.com")
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty = %v %v", empty, err)
	}
	purchases, err := useCase.Execute(context.Background(), "a@b.com")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(purchases) != 1 || len(purchases[0].Tickets) != 1 || purchases[0].Tickets[0].Number != 1 {
		t.Fatalf("purchases = %+v", purchases)
	}
}
