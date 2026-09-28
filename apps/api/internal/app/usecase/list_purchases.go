package usecase

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/repository"
)

type Purchase struct {
	Order   entity.Order
	Raffle  entity.Raffle
	Tickets []entity.Ticket
}

type ListPurchasesByEmail struct {
	orders  repository.OrderRepository
	raffles repository.RaffleRepository
	tickets repository.TicketRepository
}

func NewListPurchasesByEmail(orders repository.OrderRepository, raffles repository.RaffleRepository, tickets repository.TicketRepository) *ListPurchasesByEmail {
	return &ListPurchasesByEmail{orders: orders, raffles: raffles, tickets: tickets}
}

func (u *ListPurchasesByEmail) Execute(ctx context.Context, email string) ([]Purchase, error) {
	if email == "" {
		return nil, entity.Validation("email is required")
	}
	orders, err := u.orders.FindAllByFilters(ctx, repository.Query{
		Filters: []repository.Filter{repository.Eq(repository.ColumnEmail, email)},
		Sorts:   []repository.Sort{{Column: repository.ColumnCreatedAt}},
	})
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	if len(orders) == 0 {
		return []Purchase{}, nil
	}
	orderIDs := make([]uuid.UUID, len(orders))
	raffleIDs := make([]uuid.UUID, 0, len(orders))
	seenRaffles := map[uuid.UUID]struct{}{}
	for i, order := range orders {
		orderIDs[i] = order.ID
		if _, ok := seenRaffles[order.RaffleID]; !ok {
			seenRaffles[order.RaffleID] = struct{}{}
			raffleIDs = append(raffleIDs, order.RaffleID)
		}
	}
	raffles, err := u.raffles.FindAllByFilters(ctx, repository.Query{Filters: []repository.Filter{
		repository.In(repository.ColumnID, raffleIDs),
	}})
	if err != nil {
		return nil, fmt.Errorf("list raffles: %w", err)
	}
	raffleByID := make(map[uuid.UUID]entity.Raffle, len(raffles))
	for _, raffle := range raffles {
		raffleByID[raffle.ID] = raffle
	}
	tickets, err := u.tickets.FindAllByFilters(ctx, repository.Query{Filters: []repository.Filter{
		repository.In(repository.ColumnOrderID, orderIDs),
	}})
	if err != nil {
		return nil, fmt.Errorf("list tickets: %w", err)
	}
	ticketsByOrder := map[uuid.UUID][]entity.Ticket{}
	for _, ticket := range tickets {
		if ticket.OrderID == nil {
			continue
		}
		ticketsByOrder[*ticket.OrderID] = append(ticketsByOrder[*ticket.OrderID], ticket)
	}
	purchases := make([]Purchase, 0, len(orders))
	for _, order := range orders {
		raffle, ok := raffleByID[order.RaffleID]
		if !ok {
			return nil, fmt.Errorf("raffle %s missing for order %s", order.RaffleID, order.ID)
		}
		linked := ticketsByOrder[order.ID]
		sort.Slice(linked, func(i, j int) bool { return linked[i].Number < linked[j].Number })
		purchases = append(purchases, Purchase{Order: order, Raffle: raffle, Tickets: linked})
	}
	return purchases, nil
}
