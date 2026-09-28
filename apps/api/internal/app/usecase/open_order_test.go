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

func TestOpenOrderInsufficientStockCreatesNoOrder(t *testing.T) {
	store := mem.New()
	raffleID := uuid.New()
	store.Raffles[raffleID] = entity.Raffle{
		ID:               raffleID,
		Slug:             "car",
		Title:            "Car",
		TicketPriceCents: 100,
		TotalTickets:     5,
		ReservedTickets:  4,
		Status:           entity.RaffleStatusOpen,
	}
	useCase := NewOpenOrder(store.RafflesRepo(), store.OrdersRepo(), store.PaymentsRepo(), 15*time.Minute, func() time.Time {
		return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	}, nil)

	_, _, err := useCase.Execute(context.Background(), OpenOrderInput{Slug: "car", Email: "a@b.com", Quantity: 2})
	if !errors.Is(err, entity.ErrInsufficientTickets) {
		t.Fatalf("error = %v", err)
	}
	if len(store.Orders) != 0 || len(store.Payments) != 0 {
		t.Fatalf("order or payment was created")
	}
	if store.Raffles[raffleID].ReservedTickets != 4 {
		t.Fatalf("reserved = %d", store.Raffles[raffleID].ReservedTickets)
	}
	if !sawCapacityClause(store) {
		t.Fatal("conditional capacity update was not requested")
	}
}

func TestOpenOrderClosedRaffleCreatesNoOrder(t *testing.T) {
	store := mem.New()
	raffleID := uuid.New()
	store.Raffles[raffleID] = entity.Raffle{
		ID: raffleID, Slug: "car", Status: entity.RaffleStatusClosed, TotalTickets: 5, TicketPriceCents: 100,
	}
	useCase := NewOpenOrder(store.RafflesRepo(), store.OrdersRepo(), store.PaymentsRepo(), time.Minute, nil, nil)
	_, _, err := useCase.Execute(context.Background(), OpenOrderInput{Slug: "car", Email: "a@b.com", Quantity: 1})
	if !errors.Is(err, entity.ErrRaffleClosed) {
		t.Fatalf("error = %v", err)
	}
	if len(store.Orders) != 0 {
		t.Fatal("order was created")
	}
}

func sawCapacityClause(store *mem.Store) bool {
	for _, call := range store.Updates {
		if call.Entity != "raffle" {
			continue
		}
		for _, filter := range call.Query.Filters {
			if filter.Expr.SQL == repository.CapacityFitsSQL {
				return true
			}
		}
	}
	return false
}
