package httpapi

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/yurihartmann/rifa-stress-test/apps/api"

var tracer = otel.Tracer(tracerName)

type RequestMetrics interface {
	RecordHTTP(ctx context.Context, route, method string, status int, duration time.Duration)
	RecordReservation(ctx context.Context, duration time.Duration, outcome string)
}

func observability(logger *slog.Logger, metrics RequestMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		ctx, span := tracer.Start(ctx, "http.request", trace.WithAttributes(
			attribute.String("http.method", c.Request.Method),
		))
		c.Request = c.Request.WithContext(ctx)
		recorder := &statusRecorder{ResponseWriter: c.Writer}
		c.Writer = recorder
		started := time.Now()
		defer func() {
			route := c.FullPath()
			if route == "" {
				route = "unmatched"
			}
			status := recorder.Status()
			span.SetAttributes(
				attribute.String("http.route", route),
				attribute.Int("http.status_code", status),
			)
			span.End()
			if metrics != nil {
				metrics.RecordHTTP(ctx, route, c.Request.Method, status, time.Since(started))
			}
			if logger != nil {
				logger.InfoContext(ctx, "http request",
					"method", c.Request.Method,
					"route", route,
					"status", status,
					"duration_ms", time.Since(started).Milliseconds(),
				)
			}
		}()
		c.Next()
	}
}

type statusRecorder struct {
	gin.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	if code > 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) WriteHeaderNow() {
	if r.status == 0 {
		r.status = r.ResponseWriter.Status()
	}
	r.ResponseWriter.WriteHeaderNow()
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = httpStatusOr(r.ResponseWriter.Status(), 200)
	}
	return r.ResponseWriter.Write(data)
}

func (r *statusRecorder) Status() int {
	if r.status != 0 {
		return r.status
	}
	return r.ResponseWriter.Status()
}

func httpStatusOr(status, fallback int) int {
	if status == 0 {
		return fallback
	}
	return status
}
