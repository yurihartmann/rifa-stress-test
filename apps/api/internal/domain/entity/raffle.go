package entity

import (
	"time"

	"github.com/google/uuid"
)

type RaffleStatus string

const (
	RaffleStatusDraft  RaffleStatus = "draft"
	RaffleStatusOpen   RaffleStatus = "open"
	RaffleStatusClosed RaffleStatus = "closed"
)

func ParseRaffleStatus(value string) (RaffleStatus, error) {
	switch RaffleStatus(value) {
	case RaffleStatusDraft, RaffleStatusOpen, RaffleStatusClosed:
		return RaffleStatus(value), nil
	default:
		return "", Validation("status must be draft, open, or closed")
	}
}

type Raffle struct {
	ID               uuid.UUID
	Slug             string
	Title            string
	Description      string
	TicketPriceCents int64
	TotalTickets     int
	ReservedTickets  int
	SoldTickets      int
	Status           RaffleStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (r Raffle) AvailableTickets() int {
	return r.TotalTickets - r.ReservedTickets - r.SoldTickets
}
