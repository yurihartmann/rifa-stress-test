package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/app/service"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	domainrepo "github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/repository"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database/model"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/testkit"
	"gorm.io/gorm"
)

func TestReserveCapacityClause(t *testing.T) {
	db := testkit.OpenSQLite(t)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repo := NewRaffleRepository()
	now := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	raffleID := uuid.New()
	err := database.Run(context.Background(), db, func(ctx context.Context) error {
		return repo.Create(ctx, entity.Raffle{
			ID: raffleID, Slug: "car", Title: "Car", TicketPriceCents: 100, TotalTickets: 5,
			Status: entity.RaffleStatusOpen, CreatedAt: now, UpdatedAt: now,
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	filters, update := service.ReserveCapacity(3)
	filters = append([]domainrepo.Filter{domainrepo.Eq(domainrepo.ColumnID, raffleID)}, filters...)
	statement := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		values, err := assignmentValues(update)
		if err != nil {
			t.Fatal(err)
		}
		filtered, err := applyFilters(tx.Model(&model.Raffle{}), filters)
		if err != nil {
			t.Fatal(err)
		}
		return filtered.Updates(values)
	})
	if !strings.Contains(statement, "reserved_tickets + sold_tickets") || !strings.Contains(statement, "status") {
		t.Fatalf("sql = %s", statement)
	}

	var first, second int64
	if err := database.Run(context.Background(), db, func(ctx context.Context) error {
		var updateErr error
		first, updateErr = repo.Update(ctx, domainrepo.Query{Filters: filters}, update)
		return updateErr
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.Run(context.Background(), db, func(ctx context.Context) error {
		var updateErr error
		second, updateErr = repo.Update(ctx, domainrepo.Query{Filters: filters}, update)
		return updateErr
	}); err != nil {
		t.Fatal(err)
	}
	if first != 1 || second != 0 {
		t.Fatalf("rows first=%d second=%d", first, second)
	}
	var reserved int
	if err := db.Model(&model.Raffle{}).Where("id = ?", raffleID).Select("reserved_tickets").Scan(&reserved).Error; err != nil {
		t.Fatal(err)
	}
	if reserved != 3 {
		t.Fatalf("reserved = %d", reserved)
	}
}
