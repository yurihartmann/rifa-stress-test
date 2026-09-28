package entity

import (
	"time"

	"github.com/google/uuid"
)

type PaymentStatus string

const (
	PaymentStatusPending PaymentStatus = "pending"
	PaymentStatusPaid    PaymentStatus = "paid"
)

type Payment struct {
	ID            uuid.UUID
	OrderID       uuid.UUID
	Status        PaymentStatus
	QRCodePayload string
	PaidAt        *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type PaymentEvent struct {
	EventID   string
	PaymentID uuid.UUID
	CreatedAt time.Time
}
