package entity

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type Ticket struct {
	ID         uuid.UUID
	RaffleID   uuid.UUID
	OrderID    *uuid.UUID
	Number     int
	AssignedAt *time.Time
	CreatedAt  time.Time
}

// FormatTicketNumber renders a ticket number padded to the width of total tickets.
// A raffle of 1000 tickets formats 1 as 0001 and 1000 as 1000.
func FormatTicketNumber(number, totalTickets int) string {
	width := len(strconv.Itoa(totalTickets))
	if width < 1 {
		width = 1
	}
	return fmt.Sprintf("%0*d", width, number)
}
