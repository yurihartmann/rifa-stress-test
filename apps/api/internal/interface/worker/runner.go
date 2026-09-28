package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/app/service"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/app/usecase"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

type Runner struct {
	db       *gorm.DB
	logger   *slog.Logger
	claim    *usecase.ClaimQueue
	generate *usecase.GenerateTicket
	expire   *usecase.ExpireOrders
	fail     *usecase.RecordQueueFailure
	onFail   func(context.Context)
	observe  func(context.Context)
	interval time.Duration
	tracer   trace.Tracer
}

func NewRunner(
	db *gorm.DB,
	logger *slog.Logger,
	claim *usecase.ClaimQueue,
	generate *usecase.GenerateTicket,
	expire *usecase.ExpireOrders,
	fail *usecase.RecordQueueFailure,
	onFail func(context.Context),
	observe func(context.Context),
) *Runner {
	return &Runner{
		db:       db,
		logger:   logger,
		claim:    claim,
		generate: generate,
		expire:   expire,
		fail:     fail,
		onFail:   onFail,
		observe:  observe,
		interval: 200 * time.Millisecond,
		tracer:   otel.Tracer("github.com/yurihartmann/rifa-stress-test/apps/api"),
	}
}

func (r *Runner) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return nil
		}
		r.tick(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (r *Runner) tick(ctx context.Context) {
	defer func() {
		if recovered := recover(); recovered != nil && r.logger != nil {
			r.logger.ErrorContext(ctx, "worker tick panic", "panic", recovered)
		}
	}()
	for i := 0; i < 25; i++ {
		if ctx.Err() != nil {
			return
		}
		var worked bool
		err := r.runTx(ctx, "expire_orders", func(ctx context.Context) error {
			var execErr error
			worked, execErr = r.expire.Execute(ctx)
			return execErr
		})
		if err != nil {
			r.log(ctx, "expire orders", err)
			break
		}
		if !worked {
			break
		}
	}
	for i := 0; i < 25; i++ {
		if ctx.Err() != nil {
			return
		}
		if !r.claimOne(ctx) {
			break
		}
	}
	if r.observe != nil {
		r.observe(ctx)
	}
}

func (r *Runner) claimOne(ctx context.Context) bool {
	var job entity.QueueItem
	var found bool
	err := r.runTx(ctx, "queue.claim", func(ctx context.Context) error {
		var execErr error
		job, found, execErr = r.claim.Execute(ctx)
		return execErr
	})
	if err != nil {
		r.log(ctx, "claim queue", err)
		return false
	}
	if !found {
		return false
	}
	genCtx := service.ExtractTrace(ctx, job.Payload)
	err = r.runTx(genCtx, "GenerateTicket", func(ctx context.Context) error {
		return r.generate.Execute(ctx, job)
	})
	if err == nil {
		return true
	}
	r.log(genCtx, "generate tickets", err)
	if r.onFail != nil {
		r.onFail(genCtx)
	}
	cause := err
	if recErr := r.runTx(ctx, "queue.failure", func(ctx context.Context) error {
		return r.fail.Execute(ctx, job.ID, cause)
	}); recErr != nil {
		r.log(ctx, "record queue failure", recErr)
	}
	return true
}

func (r *Runner) runTx(ctx context.Context, spanName string, fn func(context.Context) error) error {
	ctx, span := r.tracer.Start(ctx, spanName)
	defer span.End()
	err := database.Run(ctx, r.db, fn)
	if err != nil {
		span.SetAttributes(attribute.String("tx.outcome", "rollback"))
		span.RecordError(err)
		return err
	}
	span.SetAttributes(attribute.String("tx.outcome", "commit"))
	return nil
}

func (r *Runner) log(ctx context.Context, operation string, err error) {
	if r.logger == nil || err == nil {
		return
	}
	r.logger.ErrorContext(ctx, operation, "error", err.Error())
}
