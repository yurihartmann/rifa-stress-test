package database

import (
	"fmt"

	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database/model"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.Raffle{},
		&model.Order{},
		&model.Payment{},
		&model.PaymentEvent{},
		&model.Ticket{},
		&model.Queue{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}

func Migrate(db *gorm.DB) error {
	if err := AutoMigrate(db); err != nil {
		return err
	}
	if err := applyConstraints(db); err != nil {
		return err
	}
	return nil
}

func applyConstraints(db *gorm.DB) error {
	checks := []struct {
		name       string
		table      string
		expression string
	}{
		{"raffles_reserved_tickets_non_negative", "raffles", "reserved_tickets >= 0"},
		{"raffles_sold_tickets_non_negative", "raffles", "sold_tickets >= 0"},
		{"raffles_capacity_within_total", "raffles", "reserved_tickets + sold_tickets <= total_tickets"},
		{"raffles_ticket_price_positive", "raffles", "ticket_price_cents > 0"},
		{"raffles_total_tickets_positive", "raffles", "total_tickets > 0"},
		{"orders_quantity_positive", "orders", "quantity > 0"},
		{"orders_amount_positive", "orders", "amount_cents > 0"},
	}
	for _, check := range checks {
		if err := ensureCheck(db, check.name, check.table, check.expression); err != nil {
			return err
		}
	}
	fks := []struct {
		name, table, column, refTable, refColumn string
	}{
		{"orders_raffle_id_fkey", "orders", "raffle_id", "raffles", "id"},
		{"payments_order_id_fkey", "payments", "order_id", "orders", "id"},
		{"payment_events_payment_id_fkey", "payment_events", "payment_id", "payments", "id"},
		{"tickets_raffle_id_fkey", "tickets", "raffle_id", "raffles", "id"},
		{"tickets_order_id_fkey", "tickets", "order_id", "orders", "id"},
	}
	for _, fk := range fks {
		if err := ensureForeignKey(db, fk.name, fk.table, fk.column, fk.refTable, fk.refColumn); err != nil {
			return err
		}
	}
	indexes := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS queue_generate_ticket_aggregate_active ON queue (aggregate_id) WHERE type = 'generate_ticket' AND status IN ('pending', 'processing', 'done')`,
		`CREATE INDEX IF NOT EXISTS tickets_raffle_unassigned ON tickets (raffle_id) WHERE order_id IS NULL`,
		`CREATE INDEX IF NOT EXISTS queue_status_available_at ON queue (status, available_at)`,
	}
	for _, statement := range indexes {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("apply index: %w", err)
		}
	}
	return nil
}

func ensureCheck(db *gorm.DB, name, table, expression string) error {
	statement := fmt.Sprintf(`
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = '%s') THEN
    ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s);
  END IF;
END $$;`, name, table, name, expression)
	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("apply check %s: %w", name, err)
	}
	return nil
}

func ensureForeignKey(db *gorm.DB, name, table, column, refTable, refColumn string) error {
	statement := fmt.Sprintf(`
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = '%s') THEN
    ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s);
  END IF;
END $$;`, name, table, name, column, refTable, refColumn)
	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("apply foreign key %s: %w", name, err)
	}
	return nil
}
