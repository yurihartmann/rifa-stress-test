package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
)

type RaffleRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (entity.Raffle, error)
	FindOneByFilters(ctx context.Context, query Query) (entity.Raffle, error)
	FindAllByFilters(ctx context.Context, query Query) ([]entity.Raffle, error)
	Create(ctx context.Context, raffle entity.Raffle) error
	Update(ctx context.Context, query Query, update Update) (int64, error)
}

type OrderRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (entity.Order, error)
	FindOneByFilters(ctx context.Context, query Query) (entity.Order, error)
	FindAllByFilters(ctx context.Context, query Query) ([]entity.Order, error)
	Create(ctx context.Context, order entity.Order) error
	Update(ctx context.Context, query Query, update Update) (int64, error)
}

type PaymentRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (entity.Payment, error)
	FindOneByFilters(ctx context.Context, query Query) (entity.Payment, error)
	FindAllByFilters(ctx context.Context, query Query) ([]entity.Payment, error)
	Create(ctx context.Context, payment entity.Payment) error
	Update(ctx context.Context, query Query, update Update) (int64, error)
}

type PaymentEventRepository interface {
	FindByID(ctx context.Context, eventID string) (entity.PaymentEvent, error)
	FindOneByFilters(ctx context.Context, query Query) (entity.PaymentEvent, error)
	FindAllByFilters(ctx context.Context, query Query) ([]entity.PaymentEvent, error)
	Create(ctx context.Context, event entity.PaymentEvent) error
	Update(ctx context.Context, query Query, update Update) (int64, error)
}

type TicketRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (entity.Ticket, error)
	FindOneByFilters(ctx context.Context, query Query) (entity.Ticket, error)
	FindAllByFilters(ctx context.Context, query Query) ([]entity.Ticket, error)
	Create(ctx context.Context, ticket entity.Ticket) error
	Update(ctx context.Context, query Query, update Update) (int64, error)
}

type QueueRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (entity.QueueItem, error)
	FindOneByFilters(ctx context.Context, query Query) (entity.QueueItem, error)
	FindAllByFilters(ctx context.Context, query Query) ([]entity.QueueItem, error)
	Create(ctx context.Context, item entity.QueueItem) error
	Update(ctx context.Context, query Query, update Update) (int64, error)
}
