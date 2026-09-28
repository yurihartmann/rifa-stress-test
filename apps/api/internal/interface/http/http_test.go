package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/app/usecase"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/dto"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database/model"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/repository"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/testkit"
	"gorm.io/gorm"
	"log/slog"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Set(now time.Time) {
	c.mu.Lock()
	c.now = now
	c.mu.Unlock()
}

func newApp(t *testing.T) (*gin.Engine, *gorm.DB, *testClock) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testkit.OpenSQLite(t)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clock := &testClock{now: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	raffles := repository.NewRaffleRepository()
	orders := repository.NewOrderRepository()
	payments := repository.NewPaymentRepository()
	events := repository.NewPaymentEventRepository()
	tickets := repository.NewTicketRepository()
	queue := repository.NewQueueRepository()
	engine := NewRouter(Dependencies{
		DB:             db,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		AdminToken:     "admin-secret",
		WebhookSecret:  "wh-secret",
		GetRaffle:      usecase.NewGetRaffle(raffles),
		OpenOrder:      usecase.NewOpenOrder(raffles, orders, payments, 15*time.Minute, clock.Now, nil),
		ConfirmPayment: usecase.NewConfirmPayment(raffles, orders, payments, events, queue, clock.Now, nil),
		ListPurchases:  usecase.NewListPurchasesByEmail(orders, raffles, tickets),
		CreateRaffle:   usecase.NewCreateRaffle(raffles, clock.Now),
		UpdateRaffle:   usecase.NewUpdateRaffleStatus(raffles, tickets, clock.Now),
	})
	return engine, db, clock
}

func perform(engine *gin.Engine, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return out
}

func TestAdminWithoutToken(t *testing.T) {
	engine, _, _ := newApp(t)
	rec := perform(engine, http.MethodPost, "/v1/admin/raffles", dto.CreateRaffleRequest{
		Slug: "car", Title: "Car", TicketPriceCents: 100, TotalTickets: 5,
	}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	body := decode[dto.ErrorResponse](t, rec)
	if body.Code != dto.CodeUnauthorized {
		t.Fatalf("code %s", body.Code)
	}
	rec = perform(engine, http.MethodPost, "/v1/payments/webhooks", dto.WebhookRequest{
		EventID: "evt", PaymentID: uuid.NewString(), Status: "paid",
	}, map[string]string{"Authorization": "Bearer admin-secret"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("admin token confirmed a payment: %d %s", rec.Code, rec.Body.String())
	}
}

func TestEmailNormalizationAndWebhookReplay(t *testing.T) {
	engine, db, clock := newApp(t)
	admin := map[string]string{"Authorization": "Bearer admin-secret"}
	created := perform(engine, http.MethodPost, "/v1/admin/raffles", dto.CreateRaffleRequest{
		Slug: "car", Title: "Carro", Description: "", TicketPriceCents: 250, TotalTickets: 1000,
	}, admin)
	if created.Code != http.StatusCreated {
		t.Fatalf("create %d %s", created.Code, created.Body.String())
	}
	raffle := decode[dto.RaffleResponse](t, created)
	if raffle.Status != "draft" || raffle.AvailableTickets != 1000 {
		t.Fatalf("raffle %+v", raffle)
	}
	opened := perform(engine, http.MethodPatch, "/v1/admin/raffles/"+raffle.ID, dto.UpdateRaffleStatusRequest{Status: "open"}, admin)
	if opened.Code != http.StatusOK {
		t.Fatalf("open %d %s", opened.Code, opened.Body.String())
	}
	var ticketCount int64
	if err := db.Model(&model.Ticket{}).Count(&ticketCount).Error; err != nil || ticketCount != 1000 {
		t.Fatalf("tickets %d err %v", ticketCount, err)
	}

	orderRec := perform(engine, http.MethodPost, "/v1/raffles/car/orders", dto.CreateOrderRequest{
		Email: "  Foo@Example.COM ", Quantity: 1,
	}, nil)
	if orderRec.Code != http.StatusCreated {
		t.Fatalf("order %d %s", orderRec.Code, orderRec.Body.String())
	}
	order := decode[dto.OrderResponse](t, orderRec)
	if order.Email != "foo@example.com" || order.AmountCents != 250 || len(order.Tickets) != 0 || order.Payment.QRCodePayload == "" {
		t.Fatalf("order %+v", order)
	}
	list := perform(engine, http.MethodGet, "/v1/purchases?email=foo@example.com", nil, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list %d %s", list.Code, list.Body.String())
	}
	purchases := decode[dto.PurchaseListResponse](t, list)
	if purchases.Email != "foo@example.com" || len(purchases.Purchases) != 1 {
		t.Fatalf("purchases %+v", purchases)
	}
	other := perform(engine, http.MethodGet, "/v1/purchases?email=other@example.com", nil, nil)
	otherBody := decode[dto.PurchaseListResponse](t, other)
	if other.Code != http.StatusOK || len(otherBody.Purchases) != 0 {
		t.Fatalf("other %d %+v", other.Code, otherBody)
	}

	badSecret := perform(engine, http.MethodPost, "/v1/payments/webhooks", dto.WebhookRequest{
		EventID: "evt-1", PaymentID: order.Payment.PaymentID, Status: "paid",
	}, map[string]string{"X-Webhook-Secret": "nope"})
	if badSecret.Code != http.StatusUnauthorized {
		t.Fatalf("secret %d", badSecret.Code)
	}
	paid := perform(engine, http.MethodPost, "/v1/payments/webhooks", dto.WebhookRequest{
		EventID: "evt-1", PaymentID: order.Payment.PaymentID, Status: "paid",
	}, map[string]string{"X-Webhook-Secret": "wh-secret"})
	if paid.Code != http.StatusOK || paid.Body.String() != "{}" {
		t.Fatalf("paid %d %s", paid.Code, paid.Body.String())
	}
	replay := perform(engine, http.MethodPost, "/v1/payments/webhooks", dto.WebhookRequest{
		EventID: "evt-1", PaymentID: order.Payment.PaymentID, Status: "paid",
	}, map[string]string{"X-Webhook-Secret": "wh-secret"})
	if replay.Code != http.StatusOK {
		t.Fatalf("replay %d %s", replay.Code, replay.Body.String())
	}
	var sold int
	if err := db.Model(&model.Raffle{}).Select("sold_tickets").Scan(&sold).Error; err != nil {
		t.Fatal(err)
	}
	if sold != 1 {
		t.Fatalf("sold %d", sold)
	}
	var jobs int64
	if err := db.Model(&model.Queue{}).Count(&jobs).Error; err != nil || jobs != 1 {
		t.Fatalf("jobs %d err %v", jobs, err)
	}
	raffleUUID, err := uuid.Parse(raffle.ID)
	if err != nil {
		t.Fatal(err)
	}
	orderUUID, err := uuid.Parse(order.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Ticket{}).Where("raffle_id = ? AND number = ?", raffleUUID, 1).Update("order_id", orderUUID).Error; err != nil {
		t.Fatal(err)
	}
	withTicket := perform(engine, http.MethodGet, "/v1/purchases?email=foo@example.com", nil, nil)
	listed := decode[dto.PurchaseListResponse](t, withTicket)
	if len(listed.Purchases) != 1 || len(listed.Purchases[0].Tickets) != 1 || listed.Purchases[0].Tickets[0].Number != "0001" {
		t.Fatalf("tickets %+v", listed.Purchases)
	}

	clock.Set(clock.Now().Add(time.Hour))
	second := perform(engine, http.MethodPost, "/v1/raffles/car/orders", dto.CreateOrderRequest{Email: "late@example.com", Quantity: 1}, nil)
	if second.Code != http.StatusCreated {
		t.Fatalf("second %d %s", second.Code, second.Body.String())
	}
	late := decode[dto.OrderResponse](t, second)
	clock.Set(clock.Now().Add(16 * time.Minute))
	expired := perform(engine, http.MethodPost, "/v1/payments/webhooks", dto.WebhookRequest{
		EventID: "evt-late", PaymentID: late.Payment.PaymentID, Status: "paid",
	}, map[string]string{"X-Webhook-Secret": "wh-secret"})
	if expired.Code != http.StatusConflict {
		t.Fatalf("expired %d %s", expired.Code, expired.Body.String())
	}
	if decode[dto.ErrorResponse](t, expired).Code != dto.CodeOrderExpired {
		t.Fatalf("body %s", expired.Body.String())
	}
	var status string
	if err := db.Model(&model.Order{}).Where("id = ?", late.OrderID).Select("status").Scan(&status).Error; err != nil {
		t.Fatal(err)
	}
	if status != string(entity.OrderStatusPendingPayment) {
		t.Fatalf("expired order reopened as %s", status)
	}
}

func TestInsufficientTicketsAndClosedRaffle(t *testing.T) {
	engine, _, _ := newApp(t)
	admin := map[string]string{"Authorization": "Bearer admin-secret"}
	created := perform(engine, http.MethodPost, "/v1/admin/raffles", dto.CreateRaffleRequest{
		Slug: "hat", Title: "Hat", TicketPriceCents: 100, TotalTickets: 1,
	}, admin)
	raffle := decode[dto.RaffleResponse](t, created)
	perform(engine, http.MethodPatch, "/v1/admin/raffles/"+raffle.ID, dto.UpdateRaffleStatusRequest{Status: "open"}, admin)
	first := perform(engine, http.MethodPost, "/v1/raffles/hat/orders", dto.CreateOrderRequest{Email: "a@b.com", Quantity: 1}, nil)
	if first.Code != http.StatusCreated {
		t.Fatalf("first %d %s", first.Code, first.Body.String())
	}
	second := perform(engine, http.MethodPost, "/v1/raffles/hat/orders", dto.CreateOrderRequest{Email: "b@b.com", Quantity: 1}, nil)
	if second.Code != http.StatusConflict || decode[dto.ErrorResponse](t, second).Code != dto.CodeInsufficientTickets {
		t.Fatalf("second %d %s", second.Code, second.Body.String())
	}
	closed := perform(engine, http.MethodPatch, "/v1/admin/raffles/"+raffle.ID, dto.UpdateRaffleStatusRequest{Status: "closed"}, admin)
	if closed.Code != http.StatusOK {
		t.Fatalf("close %d", closed.Code)
	}
	after := perform(engine, http.MethodPost, "/v1/raffles/hat/orders", dto.CreateOrderRequest{Email: "c@b.com", Quantity: 1}, nil)
	if after.Code != http.StatusConflict || decode[dto.ErrorResponse](t, after).Code != dto.CodeRaffleClosed {
		t.Fatalf("closed %d %s", after.Code, after.Body.String())
	}
}

func TestCORSPreflightDoesNotOpenTransaction(t *testing.T) {
	engine, _, _ := newApp(t)
	res := perform(engine, http.MethodOptions, "/v1/raffles/hat/orders", nil, map[string]string{
		"Origin":                        "http://localhost:8081",
		"Access-Control-Request-Method": "POST",
	})
	if res.Code != http.StatusNoContent {
		t.Fatalf("status %d", res.Code)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8081" {
		t.Fatalf("origin %q", got)
	}
	if got := res.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("missing allow headers")
	}
}
