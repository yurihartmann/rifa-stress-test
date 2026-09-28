package dto

import "time"

const (
	CodeValidationError     = "validation_error"
	CodeUnauthorized        = "unauthorized"
	CodeNotFound            = "not_found"
	CodeInsufficientTickets = "insufficient_tickets"
	CodeRaffleClosed        = "raffle_closed"
	CodeOrderExpired        = "order_expired"
	CodeInternal            = "internal"
)

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Empty struct{}

type RaffleResponse struct {
	ID               string `json:"id"`
	Slug             string `json:"slug"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	TicketPriceCents int64  `json:"ticket_price_cents"`
	TotalTickets     int    `json:"total_tickets"`
	Status           string `json:"status"`
	AvailableTickets int    `json:"available_tickets"`
}

type CreateRaffleRequest struct {
	Slug             string `json:"slug" binding:"required"`
	Title            string `json:"title" binding:"required"`
	Description      string `json:"description"`
	TicketPriceCents int64  `json:"ticket_price_cents" binding:"required,gt=0"`
	TotalTickets     int    `json:"total_tickets" binding:"required,gt=0"`
}

type UpdateRaffleStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type RaffleListResponse struct {
	Raffles []RaffleResponse `json:"raffles"`
}

type CreateOrderRequest struct {
	Email    string `json:"email" binding:"required"`
	Quantity int    `json:"quantity" binding:"required,gt=0"`
}

type PaymentResponse struct {
	PaymentID     string `json:"payment_id"`
	QRCodePayload string `json:"qr_code_payload"`
}

type TicketNumber struct {
	Number string `json:"number"`
}

type OrderResponse struct {
	OrderID     string          `json:"order_id"`
	RaffleSlug  string          `json:"raffle_slug"`
	Email       string          `json:"email"`
	Quantity    int             `json:"quantity"`
	AmountCents int64           `json:"amount_cents"`
	Status      string          `json:"status"`
	ExpiresAt   time.Time       `json:"expires_at"`
	Payment     PaymentResponse `json:"payment"`
	Tickets     []TicketNumber  `json:"tickets"`
}

type WebhookRequest struct {
	EventID   string `json:"event_id" binding:"required"`
	PaymentID string `json:"payment_id" binding:"required"`
	Status    string `json:"status" binding:"required"`
}

type PurchaseResponse struct {
	OrderID     string         `json:"order_id"`
	RaffleSlug  string         `json:"raffle_slug"`
	RaffleTitle string         `json:"raffle_title"`
	Quantity    int            `json:"quantity"`
	AmountCents int64          `json:"amount_cents"`
	Status      string         `json:"status"`
	Tickets     []TicketNumber `json:"tickets"`
}

type PurchaseListResponse struct {
	Email     string             `json:"email"`
	Purchases []PurchaseResponse `json:"purchases"`
}
