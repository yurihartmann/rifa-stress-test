package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Raffle struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	Slug             string    `gorm:"type:text;not null;uniqueIndex"`
	Title            string    `gorm:"type:text;not null"`
	Description      string    `gorm:"type:text;not null;default:''"`
	TicketPriceCents int64     `gorm:"type:bigint;not null"`
	TotalTickets     int       `gorm:"not null"`
	ReservedTickets  int       `gorm:"not null;default:0"`
	SoldTickets      int       `gorm:"not null;default:0"`
	Status           string    `gorm:"type:text;not null"`
	CreatedAt        time.Time `gorm:"not null"`
	UpdatedAt        time.Time `gorm:"not null"`
}

func (Raffle) TableName() string { return "raffles" }

type Order struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	RaffleID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Email       string    `gorm:"type:text;not null;index"`
	Quantity    int       `gorm:"not null"`
	AmountCents int64     `gorm:"type:bigint;not null"`
	Status      string    `gorm:"type:text;not null;index:orders_status_expires_at,priority:1"`
	ExpiresAt   time.Time `gorm:"not null;index:orders_status_expires_at,priority:2"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (Order) TableName() string { return "orders" }

type Payment struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrderID       uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`
	Status        string    `gorm:"type:text;not null"`
	QRCodePayload string    `gorm:"type:text;not null"`
	PaidAt        *time.Time
	CreatedAt     time.Time `gorm:"not null"`
	UpdatedAt     time.Time `gorm:"not null"`
}

func (Payment) TableName() string { return "payments" }

type PaymentEvent struct {
	EventID   string    `gorm:"type:text;primaryKey"`
	PaymentID uuid.UUID `gorm:"type:uuid;not null;index"`
	CreatedAt time.Time `gorm:"not null"`
}

func (PaymentEvent) TableName() string { return "payment_events" }

type Ticket struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	RaffleID   uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:tickets_raffle_number,priority:1"`
	OrderID    *uuid.UUID `gorm:"type:uuid;index"`
	Number     int        `gorm:"not null;uniqueIndex:tickets_raffle_number,priority:2"`
	AssignedAt *time.Time
	CreatedAt  time.Time `gorm:"not null"`
}

func (Ticket) TableName() string { return "tickets" }

type Queue struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
	Type        string         `gorm:"type:text;not null"`
	AggregateID uuid.UUID      `gorm:"type:uuid;not null"`
	Payload     datatypes.JSON `gorm:"type:jsonb;not null"`
	Status      string         `gorm:"type:text;not null;index:queue_status_available_at,priority:1"`
	Attempts    int            `gorm:"not null;default:0"`
	AvailableAt time.Time      `gorm:"not null;index:queue_status_available_at,priority:2"`
	LockedAt    *time.Time
	LockedBy    *string   `gorm:"type:text"`
	LastError   *string   `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (Queue) TableName() string { return "queue" }
