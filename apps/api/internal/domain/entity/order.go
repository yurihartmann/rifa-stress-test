package entity

import (
	"time"

	"github.com/google/uuid"
)

type OrderStatus string

const (
	OrderStatusPendingPayment OrderStatus = "pending_payment"
	OrderStatusPaid           OrderStatus = "paid"
	OrderStatusExpired        OrderStatus = "expired"
)

type Order struct {
	ID          uuid.UUID
	RaffleID    uuid.UUID
	Email       string
	Quantity    int
	AmountCents int64
	Status      OrderStatus
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
