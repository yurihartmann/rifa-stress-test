package entity

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrValidation          = errors.New("validation error")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInsufficientTickets = errors.New("insufficient tickets")
	ErrRaffleClosed        = errors.New("raffle closed")
	ErrOrderExpired        = errors.New("order expired")
	ErrConflict            = errors.New("conflict")
	ErrAlreadyExists       = errors.New("already exists")
	ErrTicketShortfall     = errors.New("ticket shortfall")
	ErrOrderNotPaid        = errors.New("order not paid")
)

// DomainError is a sentinel with a client-safe message.
type DomainError struct {
	Sentinel error
	Msg      string
}

func (e *DomainError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	if e.Sentinel != nil {
		return e.Sentinel.Error()
	}
	return "domain error"
}

func (e *DomainError) Unwrap() error {
	return e.Sentinel
}

func Validation(message string) error {
	return &DomainError{Sentinel: ErrValidation, Msg: message}
}
