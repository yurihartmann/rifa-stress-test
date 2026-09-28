package setting

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultHTTPAddr         = ":8080"
	defaultWorkerHealthAddr = ":8081"
	defaultOrderHoldTTL     = 15 * time.Minute
)

type Settings struct {
	DatabaseURL          string
	HTTPAddr             string
	WorkerHealthAddr     string
	PaymentWebhookSecret string
	AdminToken           string
	OrderHoldTTL         time.Duration
	QueueLockTimeout     time.Duration
	QueueMaxAttempts     int
	OTLPEndpoint         string
}

func LoadAPI() (Settings, error) {
	settings, err := load()
	if err != nil {
		return Settings{}, err
	}
	if err := settings.requireProcess("api"); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func LoadWorker() (Settings, error) {
	settings, err := load()
	if err != nil {
		return Settings{}, err
	}
	if err := settings.requireProcess("worker"); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func LoadMigrate() (Settings, error) {
	settings, err := load()
	if err != nil {
		return Settings{}, err
	}
	if settings.DatabaseURL == "" {
		return Settings{}, fmt.Errorf("DATABASE_URL is required")
	}
	return settings, nil
}

func load() (Settings, error) {
	_ = godotenv.Load()
	hold := defaultOrderHoldTTL
	if raw := strings.TrimSpace(os.Getenv("ORDER_HOLD_TTL")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return Settings{}, fmt.Errorf("ORDER_HOLD_TTL is invalid")
		}
		hold = parsed
	}
	var lockTimeout time.Duration
	if raw := strings.TrimSpace(os.Getenv("QUEUE_LOCK_TIMEOUT")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return Settings{}, fmt.Errorf("QUEUE_LOCK_TIMEOUT is invalid")
		}
		lockTimeout = parsed
	}
	var attempts int
	if raw := strings.TrimSpace(os.Getenv("QUEUE_MAX_ATTEMPTS")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return Settings{}, fmt.Errorf("QUEUE_MAX_ATTEMPTS is invalid")
		}
		attempts = parsed
	}
	httpAddr := strings.TrimSpace(os.Getenv("HTTP_ADDR"))
	if httpAddr == "" {
		httpAddr = defaultHTTPAddr
	}
	healthAddr := strings.TrimSpace(os.Getenv("WORKER_HEALTH_ADDR"))
	if healthAddr == "" {
		healthAddr = defaultWorkerHealthAddr
	}
	return Settings{
		DatabaseURL:          strings.TrimSpace(os.Getenv("DATABASE_URL")),
		HTTPAddr:             httpAddr,
		WorkerHealthAddr:     healthAddr,
		PaymentWebhookSecret: os.Getenv("PAYMENT_WEBHOOK_SECRET"),
		AdminToken:           os.Getenv("ADMIN_TOKEN"),
		OrderHoldTTL:         hold,
		QueueLockTimeout:     lockTimeout,
		QueueMaxAttempts:     attempts,
		OTLPEndpoint:         strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
	}, nil
}

func (s Settings) requireProcess(name string) error {
	if s.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if strings.TrimSpace(s.PaymentWebhookSecret) == "" {
		return fmt.Errorf("PAYMENT_WEBHOOK_SECRET is required")
	}
	if strings.TrimSpace(s.AdminToken) == "" {
		return fmt.Errorf("ADMIN_TOKEN is required")
	}
	if s.OrderHoldTTL <= 0 {
		return fmt.Errorf("ORDER_HOLD_TTL must be positive")
	}
	if s.QueueLockTimeout <= 0 {
		return fmt.Errorf("QUEUE_LOCK_TIMEOUT is required")
	}
	if s.QueueMaxAttempts <= 0 {
		return fmt.Errorf("QUEUE_MAX_ATTEMPTS is required")
	}
	if name == "api" && s.HTTPAddr == "" {
		return fmt.Errorf("HTTP_ADDR is required")
	}
	return nil
}
