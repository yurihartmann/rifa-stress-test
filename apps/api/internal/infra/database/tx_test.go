package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRunCommitsAndRollsBack(t *testing.T) {
	db := openSQLite(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	id := uuid.New()
	err := Run(context.Background(), db, func(ctx context.Context) error {
		tx, err := FromContext(ctx)
		if err != nil {
			return err
		}
		return tx.Create(&model.Raffle{
			ID: id, Slug: id.String(), Title: "Car", TicketPriceCents: 10, TotalTickets: 1,
			Status: "draft", CreatedAt: now, UpdatedAt: now,
		}).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if count(t, db) != 1 {
		t.Fatalf("count = %d", count(t, db))
	}
	err = Run(context.Background(), db, func(ctx context.Context) error {
		tx, err := FromContext(ctx)
		if err != nil {
			return err
		}
		if err := tx.Create(&model.Raffle{
			ID: uuid.New(), Slug: uuid.NewString(), Title: "Hat", TicketPriceCents: 10, TotalTickets: 1,
			Status: "draft", CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			return err
		}
		return errors.New("fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if count(t, db) != 1 {
		t.Fatalf("count after rollback = %d", count(t, db))
	}
}

func TestFromContextRequiresTransaction(t *testing.T) {
	if _, err := FromContext(context.Background()); !errors.Is(err, ErrNoTransaction) {
		t.Fatalf("error = %v", err)
	}
}

func openSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func count(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&model.Raffle{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}
