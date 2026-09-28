package usecase

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/yurihartmann/rifa-stress-test/apps/api"

var tracer = otel.Tracer(tracerName)

type Recorder interface {
	OrderEntered(ctx context.Context, status string)
	TicketsAssigned(ctx context.Context, quantity int)
}

type nopRecorder struct{}

func (nopRecorder) OrderEntered(context.Context, string) {}
func (nopRecorder) TicketsAssigned(context.Context, int) {}

func recorderOrNop(recorder Recorder) Recorder {
	if recorder == nil {
		return nopRecorder{}
	}
	return recorder
}

func clockOrNow(clock func() time.Time) func() time.Time {
	if clock == nil {
		return func() time.Time { return time.Now().UTC() }
	}
	return func() time.Time { return clock().UTC() }
}

func setSpanAttrs(span trace.Span, raffleID, orderID, paymentID string, quantity int) {
	attrs := make([]attribute.KeyValue, 0, 4)
	if raffleID != "" {
		attrs = append(attrs, attribute.String("raffle_id", raffleID))
	}
	if orderID != "" {
		attrs = append(attrs, attribute.String("order_id", orderID))
	}
	if paymentID != "" {
		attrs = append(attrs, attribute.String("payment_id", paymentID))
	}
	if quantity > 0 {
		attrs = append(attrs, attribute.Int("quantity", quantity))
	}
	span.SetAttributes(attrs...)
}
