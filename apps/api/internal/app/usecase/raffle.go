package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/repository"
)

type CreateRaffleInput struct {
	Slug             string
	Title            string
	Description      string
	TicketPriceCents int64
	TotalTickets     int
}

type CreateRaffle struct {
	raffles repository.RaffleRepository
	now     func() time.Time
}

func NewCreateRaffle(raffles repository.RaffleRepository, now func() time.Time) *CreateRaffle {
	return &CreateRaffle{raffles: raffles, now: clockOrNow(now)}
}

func (u *CreateRaffle) Execute(ctx context.Context, input CreateRaffleInput) (entity.Raffle, error) {
	slug := strings.TrimSpace(input.Slug)
	title := strings.TrimSpace(input.Title)
	if slug == "" || title == "" {
		return entity.Raffle{}, entity.Validation("slug and title are required")
	}
	if input.TicketPriceCents <= 0 || input.TotalTickets <= 0 {
		return entity.Raffle{}, entity.Validation("ticket price and total tickets must be positive")
	}
	now := u.now()
	raffle := entity.Raffle{
		ID:               uuid.New(),
		Slug:             slug,
		Title:            title,
		Description:      input.Description,
		TicketPriceCents: input.TicketPriceCents,
		TotalTickets:     input.TotalTickets,
		Status:           entity.RaffleStatusDraft,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := u.raffles.Create(ctx, raffle); err != nil {
		if errors.Is(err, entity.ErrConflict) {
			return entity.Raffle{}, entity.Validation("slug already exists")
		}
		return entity.Raffle{}, fmt.Errorf("create raffle: %w", err)
	}
	return raffle, nil
}

type UpdateRaffleStatus struct {
	raffles repository.RaffleRepository
	tickets repository.TicketRepository
	now     func() time.Time
}

func NewUpdateRaffleStatus(raffles repository.RaffleRepository, tickets repository.TicketRepository, now func() time.Time) *UpdateRaffleStatus {
	return &UpdateRaffleStatus{raffles: raffles, tickets: tickets, now: clockOrNow(now)}
}

func (u *UpdateRaffleStatus) Execute(ctx context.Context, id uuid.UUID, status entity.RaffleStatus) (entity.Raffle, error) {
	raffle, err := u.raffles.FindByID(ctx, id)
	if err != nil {
		return entity.Raffle{}, err
	}
	if status == entity.RaffleStatusOpen {
		if err := u.ensurePool(ctx, raffle); err != nil {
			return entity.Raffle{}, err
		}
	}
	now := u.now()
	rows, err := u.raffles.Update(ctx, repository.Query{Filters: []repository.Filter{
		repository.Eq(repository.ColumnID, raffle.ID),
	}}, repository.Update{Assignments: []repository.Assignment{
		{Column: repository.ColumnStatus, Value: string(status)},
		{Column: repository.ColumnUpdatedAt, Value: now},
	}})
	if err != nil {
		return entity.Raffle{}, fmt.Errorf("update raffle status: %w", err)
	}
	if rows == 0 {
		return entity.Raffle{}, entity.ErrNotFound
	}
	raffle.Status = status
	raffle.UpdatedAt = now
	return raffle, nil
}

func (u *UpdateRaffleStatus) ensurePool(ctx context.Context, raffle entity.Raffle) error {
	existing, err := u.tickets.FindAllByFilters(ctx, repository.Query{
		Filters: []repository.Filter{repository.Eq(repository.ColumnRaffleID, raffle.ID)},
		Limit:   1,
	})
	if err != nil {
		return fmt.Errorf("load ticket pool: %w", err)
	}
	if len(existing) > 0 {
		return nil
	}
	createdAt := u.now()
	for number := 1; number <= raffle.TotalTickets; number++ {
		ticket := entity.Ticket{
			ID:        uuid.New(),
			RaffleID:  raffle.ID,
			Number:    number,
			CreatedAt: createdAt,
		}
		if err := u.tickets.Create(ctx, ticket); err != nil {
			return fmt.Errorf("insert ticket %d: %w", number, err)
		}
	}
	return nil
}

type GetRaffle struct {
	raffles repository.RaffleRepository
}

func NewGetRaffle(raffles repository.RaffleRepository) *GetRaffle {
	return &GetRaffle{raffles: raffles}
}

func (u *GetRaffle) Execute(ctx context.Context, slug string) (entity.Raffle, error) {
	raffle, err := u.raffles.FindOneByFilters(ctx, repository.Query{Filters: []repository.Filter{
		repository.Eq(repository.ColumnSlug, strings.TrimSpace(slug)),
	}})
	if err != nil {
		return entity.Raffle{}, err
	}
	return raffle, nil
}

type ListRaffles struct {
	raffles repository.RaffleRepository
}

func NewListRaffles(raffles repository.RaffleRepository) *ListRaffles {
	return &ListRaffles{raffles: raffles}
}

func (u *ListRaffles) Execute(ctx context.Context, status entity.RaffleStatus) ([]entity.Raffle, error) {
	if status != entity.RaffleStatusOpen && status != entity.RaffleStatusClosed {
		return nil, entity.Validation("status must be open or closed")
	}
	raffles, err := u.raffles.FindAllByFilters(ctx, repository.Query{
		Filters: []repository.Filter{repository.Eq(repository.ColumnStatus, string(status))},
		Sorts: []repository.Sort{
			{Column: repository.ColumnCreatedAt, Desc: true},
			{Column: repository.ColumnSlug},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list raffles: %w", err)
	}
	if raffles == nil {
		return []entity.Raffle{}, nil
	}
	return raffles, nil
}
