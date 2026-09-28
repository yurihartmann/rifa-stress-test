package entity

import (
	"time"

	"github.com/google/uuid"
)

const QueueTypeGenerateTicket = "generate_ticket"

type QueueStatus string

const (
	QueueStatusPending    QueueStatus = "pending"
	QueueStatusProcessing QueueStatus = "processing"
	QueueStatusDone       QueueStatus = "done"
	QueueStatusFailed     QueueStatus = "failed"
)

type QueueItem struct {
	ID          uuid.UUID
	Type        string
	AggregateID uuid.UUID
	Payload     []byte
	Status      QueueStatus
	Attempts    int
	AvailableAt time.Time
	LockedAt    *time.Time
	LockedBy    string
	LastError   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
