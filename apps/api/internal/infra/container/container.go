package container

import (
	"context"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/app/usecase"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/repository"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/setting"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/telemetry"
	httpapi "github.com/yurihartmann/rifa-stress-test/apps/api/internal/interface/http"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/interface/worker"
	"gorm.io/gorm"
)

type App struct {
	Settings setting.Settings
	DB       *gorm.DB
	Router   *gin.Engine
	Runner   *worker.Runner
	tel      *telemetry.Provider
}

func NewAPI(ctx context.Context) (*App, error) {
	settings, err := setting.LoadAPI()
	if err != nil {
		return nil, err
	}
	return newApp(ctx, settings, "api", true)
}

func NewWorker(ctx context.Context) (*App, error) {
	settings, err := setting.LoadWorker()
	if err != nil {
		return nil, err
	}
	return newApp(ctx, settings, "worker", false)
}

func newApp(ctx context.Context, settings setting.Settings, serviceName string, httpProcess bool) (*App, error) {
	logger := telemetry.NewLogger(serviceName)
	db, err := database.Open(settings.DatabaseURL)
	if err != nil {
		return nil, err
	}
	tel, err := telemetry.Setup(ctx, serviceName, settings.OTLPEndpoint, db, logger)
	if err != nil {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}
	raffles := repository.NewRaffleRepository()
	orders := repository.NewOrderRepository()
	payments := repository.NewPaymentRepository()
	events := repository.NewPaymentEventRepository()
	tickets := repository.NewTicketRepository()
	queue := repository.NewQueueRepository()
	recorder := tel.Metrics

	app := &App{Settings: settings, DB: db, tel: tel}
	if httpProcess {
		app.Router = httpapi.NewRouter(httpapi.Dependencies{
			DB:             db,
			Logger:         logger,
			Metrics:        recorder,
			AdminToken:     settings.AdminToken,
			WebhookSecret:  settings.PaymentWebhookSecret,
			GetRaffle:      usecase.NewGetRaffle(raffles),
			OpenOrder:      usecase.NewOpenOrder(raffles, orders, payments, settings.OrderHoldTTL, nil, recorder),
			ConfirmPayment: usecase.NewConfirmPayment(raffles, orders, payments, events, queue, nil, recorder),
			ListPurchases:  usecase.NewListPurchasesByEmail(orders, raffles, tickets),
			CreateRaffle:   usecase.NewCreateRaffle(raffles, nil),
			UpdateRaffle:   usecase.NewUpdateRaffleStatus(raffles, tickets, nil),
		})
		return app, nil
	}

	workerID, err := workerIdentity()
	if err != nil {
		return nil, err
	}
	app.Runner = worker.NewRunner(
		db,
		logger,
		usecase.NewClaimQueue(queue, workerID, settings.QueueLockTimeout, nil),
		usecase.NewGenerateTicket(orders, tickets, queue, nil, recorder),
		usecase.NewExpireOrders(raffles, orders, nil, recorder),
		usecase.NewRecordQueueFailure(queue, settings.QueueMaxAttempts, nil),
		recorder.RecordGenerationFailure,
		tel.Observe,
	)
	return app, nil
}

func (a *App) Close(ctx context.Context) error {
	var first error
	if a.tel != nil {
		if err := a.tel.Shutdown(ctx); err != nil {
			first = err
		}
	}
	if a.DB != nil {
		sqlDB, err := a.DB.DB()
		if err == nil {
			if closeErr := sqlDB.Close(); closeErr != nil && first == nil {
				first = closeErr
			}
		}
	}
	return first
}

func workerIdentity() (string, error) {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "worker"
	}
	if os.Getpid() == 0 {
		return "", fmt.Errorf("worker identity unavailable")
	}
	return fmt.Sprintf("%s:%d", host, os.Getpid()), nil
}
