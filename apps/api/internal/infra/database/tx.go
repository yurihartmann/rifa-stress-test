package database

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type txKey struct{}

var ErrNoTransaction = errors.New("database transaction missing from context")

func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// FromContext returns the request or worker transaction.
// A missing transaction is a programming error and does not fall back to the global connection.
func FromContext(ctx context.Context) (*gorm.DB, error) {
	tx, ok := ctx.Value(txKey{}).(*gorm.DB)
	if !ok || tx == nil {
		return nil, ErrNoTransaction
	}
	return tx.WithContext(ctx).Session(&gorm.Session{}), nil
}

// Run commits when fn returns nil and rolls back when fn returns an error or panics.
// Callers own the decision to invoke Run. fn must not begin, commit, or roll back.
func Run(ctx context.Context, db *gorm.DB, fn func(context.Context) error) error {
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("begin transaction: %w", tx.Error)
	}
	finished := false
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback()
			panic(recovered)
		}
		if !finished {
			_ = tx.Rollback()
		}
	}()
	if err := fn(WithTx(ctx, tx)); err != nil {
		return err
	}
	if err := tx.Commit().Error; err != nil {
		_ = tx.Rollback()
		finished = true
		return fmt.Errorf("commit transaction: %w", err)
	}
	finished = true
	return nil
}
