package telemetry

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"gorm.io/gorm"
)

type Provider struct {
	logger   *slog.Logger
	db       *gorm.DB
	shutdown func(context.Context) error
	Metrics  *Metrics
}

type Metrics struct {
	httpDuration     metric.Float64Histogram
	httpCount        metric.Int64Counter
	reserveDuration  metric.Float64Histogram
	orders           metric.Int64Counter
	ticketsAssigned  metric.Int64Counter
	generationFailed metric.Int64Counter
	snapshot         *queueSnapshot
}

type queueSnapshot struct {
	mu            sync.Mutex
	queueDepth    map[string]int64
	orderDepth    map[string]int64
	oldestPending float64
}

func NewLogger(serviceName string) *slog.Logger {
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(&traceHandler{Handler: base}).With("service", serviceName)
}

func Setup(ctx context.Context, serviceName, endpoint string, db *gorm.DB, logger *slog.Logger) (*Provider, error) {
	endpoint, err := normalizeEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		attribute.String("service.name", serviceName),
	))
	if err != nil {
		return nil, err
	}

	traceOpts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	metricOpts := []sdkmetric.Option{sdkmetric.WithResource(res)}
	shutdowns := make([]func(context.Context) error, 0, 2)
	if endpoint != "" {
		traceExp, expErr := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
		if expErr != nil {
			return nil, expErr
		}
		traceOpts = append(traceOpts, sdktrace.WithBatcher(traceExp))
		shutdowns = append(shutdowns, traceExp.Shutdown)
		metricExp, expErr := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(endpoint))
		if expErr != nil {
			return nil, expErr
		}
		metricOpts = append(metricOpts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(5*time.Second))))
		shutdowns = append(shutdowns, metricExp.Shutdown)
		if logger != nil {
			logger.Info("otlp exporter configured")
		}
	}

	tracerProvider := sdktrace.NewTracerProvider(traceOpts...)
	meterProvider := sdkmetric.NewMeterProvider(metricOpts...)
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	shutdowns = append(shutdowns, tracerProvider.Shutdown, meterProvider.Shutdown)

	metrics, err := newMetrics(meterProvider.Meter(serviceName))
	if err != nil {
		return nil, err
	}
	return &Provider{
		logger:  logger,
		db:      db,
		Metrics: metrics,
		shutdown: func(ctx context.Context) error {
			var first error
			for _, shutdown := range shutdowns {
				if err := shutdown(ctx); err != nil && first == nil {
					first = err
				}
			}
			return first
		},
	}, nil
}

func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.shutdown == nil {
		return nil
	}
	return p.shutdown(ctx)
}

func (p *Provider) Observe(ctx context.Context) {
	if p == nil || p.db == nil || p.Metrics == nil {
		return
	}
	p.Metrics.snapshot.collect(ctx, p.db, p.logger)
}

func newMetrics(meter metric.Meter) (*Metrics, error) {
	httpDuration, err := meter.Float64Histogram("http.server.request.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	httpCount, err := meter.Int64Counter("http.server.request.count")
	if err != nil {
		return nil, err
	}
	reserveDuration, err := meter.Float64Histogram("raffle.reservation.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	orders, err := meter.Int64Counter("raffle.orders")
	if err != nil {
		return nil, err
	}
	ticketsAssigned, err := meter.Int64Counter("raffle.tickets.assigned")
	if err != nil {
		return nil, err
	}
	generationFailed, err := meter.Int64Counter("raffle.tickets.generation_failures")
	if err != nil {
		return nil, err
	}
	snapshot := &queueSnapshot{queueDepth: map[string]int64{}, orderDepth: map[string]int64{}}
	queueDepth, err := meter.Int64ObservableGauge("raffle.queue.depth")
	if err != nil {
		return nil, err
	}
	oldest, err := meter.Float64ObservableGauge("raffle.queue.oldest_pending_age", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	orderGauge, err := meter.Int64ObservableGauge("raffle.orders.current")
	if err != nil {
		return nil, err
	}
	_, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		snapshot.mu.Lock()
		defer snapshot.mu.Unlock()
		for status, count := range snapshot.queueDepth {
			observer.ObserveInt64(queueDepth, count, metric.WithAttributes(attribute.String("status", status)))
		}
		for status, count := range snapshot.orderDepth {
			observer.ObserveInt64(orderGauge, count, metric.WithAttributes(attribute.String("status", status)))
		}
		observer.ObserveFloat64(oldest, snapshot.oldestPending)
		return nil
	}, queueDepth, oldest, orderGauge)
	if err != nil {
		return nil, err
	}
	return &Metrics{
		httpDuration:     httpDuration,
		httpCount:        httpCount,
		reserveDuration:  reserveDuration,
		orders:           orders,
		ticketsAssigned:  ticketsAssigned,
		generationFailed: generationFailed,
		snapshot:         snapshot,
	}, nil
}

func (m *Metrics) RecordHTTP(ctx context.Context, route, method string, status int, duration time.Duration) {
	if m == nil {
		return
	}
	attrs := metric.WithAttributes(
		attribute.String("route", route),
		attribute.String("method", method),
		attribute.Int("status", status),
	)
	m.httpDuration.Record(ctx, duration.Seconds(), attrs)
	m.httpCount.Add(ctx, 1, attrs)
}

func (m *Metrics) RecordReservation(ctx context.Context, duration time.Duration, outcome string) {
	if m == nil {
		return
	}
	m.reserveDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attribute.String("tx.outcome", outcome)))
}

func (m *Metrics) OrderEntered(ctx context.Context, status string) {
	if m == nil {
		return
	}
	m.orders.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status)))
}

func (m *Metrics) TicketsAssigned(ctx context.Context, quantity int) {
	if m == nil || quantity <= 0 {
		return
	}
	m.ticketsAssigned.Add(ctx, int64(quantity))
}

func (m *Metrics) RecordGenerationFailure(ctx context.Context) {
	if m == nil {
		return
	}
	m.generationFailed.Add(ctx, 1)
}

func (s *queueSnapshot) collect(ctx context.Context, db *gorm.DB, logger *slog.Logger) {
	type countRow struct {
		Status string
		Count  int64
	}
	var queueRows []countRow
	if err := db.WithContext(ctx).Raw(`SELECT status, COUNT(*) AS count FROM queue GROUP BY status`).Scan(&queueRows).Error; err != nil {
		if logger != nil {
			logger.WarnContext(ctx, "queue metrics query failed")
		}
		return
	}
	var orderRows []countRow
	if err := db.WithContext(ctx).Raw(`SELECT status, COUNT(*) AS count FROM orders GROUP BY status`).Scan(&orderRows).Error; err != nil {
		if logger != nil {
			logger.WarnContext(ctx, "order metrics query failed")
		}
		return
	}
	var oldest *float64
	if err := db.WithContext(ctx).Raw(`SELECT EXTRACT(EPOCH FROM (NOW() - MIN(available_at))) FROM queue WHERE status = 'pending'`).Scan(&oldest).Error; err != nil {
		if logger != nil {
			logger.WarnContext(ctx, "queue age query failed")
		}
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queueDepth = map[string]int64{}
	for _, row := range queueRows {
		s.queueDepth[row.Status] = row.Count
	}
	s.orderDepth = map[string]int64{}
	for _, row := range orderRows {
		s.orderDepth[row.Status] = row.Count
	}
	s.oldestPending = 0
	if oldest != nil && *oldest > 0 {
		s.oldestPending = *oldest
	}
}

func normalizeEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", nil
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return "", errInvalidEndpoint
	}
	return strings.TrimRight(endpoint, "/"), nil
}
