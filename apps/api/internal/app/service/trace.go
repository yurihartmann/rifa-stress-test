package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type queuePayload struct {
	TraceParent string `json:"traceparent,omitempty"`
	TraceState  string `json:"tracestate,omitempty"`
	OrderID     string `json:"order_id"`
	RaffleID    string `json:"raffle_id"`
	PaymentID   string `json:"payment_id"`
	Quantity    int    `json:"quantity"`
}

func QueuePayload(ctx context.Context, orderID, raffleID, paymentID uuid.UUID, quantity int) ([]byte, error) {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return json.Marshal(queuePayload{
		TraceParent: carrier["traceparent"],
		TraceState:  carrier["tracestate"],
		OrderID:     orderID.String(),
		RaffleID:    raffleID.String(),
		PaymentID:   paymentID.String(),
		Quantity:    quantity,
	})
}

func ExtractTrace(ctx context.Context, payload []byte) context.Context {
	if len(payload) == 0 {
		return ctx
	}
	var body queuePayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return ctx
	}
	carrier := propagation.MapCarrier{}
	if body.TraceParent != "" {
		carrier["traceparent"] = body.TraceParent
	}
	if body.TraceState != "" {
		carrier["tracestate"] = body.TraceState
	}
	if len(carrier) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
