package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/app/usecase"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/dto"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
)

type handler struct {
	webhookSecret  string
	getRaffle      *usecase.GetRaffle
	openOrder      *usecase.OpenOrder
	confirmPayment *usecase.ConfirmPayment
	listPurchases  *usecase.ListPurchasesByEmail
	createRaffle   *usecase.CreateRaffle
	updateRaffle   *usecase.UpdateRaffleStatus
	log            loggerFunc
}

type loggerFunc func(*gin.Context, error)

func (h *handler) report(c *gin.Context, err error) {
	if h.log != nil {
		h.log(c, err)
	}
}

func raffleResponse(raffle entity.Raffle) dto.RaffleResponse {
	return dto.RaffleResponse{
		ID:               raffle.ID.String(),
		Slug:             raffle.Slug,
		Title:            raffle.Title,
		Description:      raffle.Description,
		TicketPriceCents: raffle.TicketPriceCents,
		TotalTickets:     raffle.TotalTickets,
		Status:           string(raffle.Status),
		AvailableTickets: raffle.AvailableTickets(),
	}
}

func normalizeEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	if email == "" || !strings.Contains(email, "@") || strings.Contains(email, " ") {
		return "", entity.Validation("email is invalid")
	}
	return email, nil
}

// GetRaffle godoc
//
//	@Summary		Get a raffle by slug
//	@Description	Public storefront view. available_tickets is total minus reserved minus sold.
//	@Tags			raffles
//	@Produce		json
//	@Param			slug	path		string	true	"Raffle slug"
//	@Success		200		{object}	dto.RaffleResponse
//	@Failure		404		{object}	dto.ErrorResponse
//	@Failure		500		{object}	dto.ErrorResponse
//	@Router			/raffles/{slug} [get]
func (h *handler) GetRaffle(c *gin.Context) {
	raffle, err := h.getRaffle.Execute(c.Request.Context(), c.Param("slug"))
	if err != nil {
		h.report(c, err)
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, raffleResponse(raffle))
}

// CreateOrder godoc
//
//	@Summary		Open a checkout order
//	@Description	Reserves ticket capacity and returns a pending simulated payment. Numbers are not chosen here.
//	@Tags			orders
//	@Accept			json
//	@Produce		json
//	@Param			slug	path		string					true	"Raffle slug"
//	@Param			input	body		dto.CreateOrderRequest	true	"Checkout"
//	@Success		201		{object}	dto.OrderResponse
//	@Failure		400		{object}	dto.ErrorResponse
//	@Failure		404		{object}	dto.ErrorResponse
//	@Failure		409		{object}	dto.ErrorResponse
//	@Failure		500		{object}	dto.ErrorResponse
//	@Router			/raffles/{slug}/orders [post]
func (h *handler) CreateOrder(c *gin.Context) {
	var body dto.CreateOrderRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		validation(c, "invalid request")
		return
	}
	email, err := normalizeEmail(body.Email)
	if err != nil {
		writeError(c, err)
		return
	}
	order, payment, err := h.openOrder.Execute(c.Request.Context(), usecase.OpenOrderInput{
		Slug:     c.Param("slug"),
		Email:    email,
		Quantity: body.Quantity,
	})
	if err != nil {
		h.report(c, err)
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.OrderResponse{
		OrderID:     order.ID.String(),
		RaffleSlug:  c.Param("slug"),
		Email:       order.Email,
		Quantity:    order.Quantity,
		AmountCents: order.AmountCents,
		Status:      string(order.Status),
		ExpiresAt:   order.ExpiresAt,
		Payment: dto.PaymentResponse{
			PaymentID:     payment.ID.String(),
			QRCodePayload: payment.QRCodePayload,
		},
		Tickets: []dto.TicketNumber{},
	})
}

// ConfirmPayment godoc
//
//	@Summary		Confirm a simulated payment
//	@Description	Applies a paid webhook once. Replays of the same event_id return 200 without selling again.
//	@Tags			payments
//	@Accept			json
//	@Produce		json
//	@Param			X-Webhook-Secret	header		string				true	"Webhook secret"
//	@Param			input				body		dto.WebhookRequest	true	"Payment event"
//	@Success		200					{object}	dto.Empty
//	@Failure		400					{object}	dto.ErrorResponse
//	@Failure		401					{object}	dto.ErrorResponse
//	@Failure		404					{object}	dto.ErrorResponse
//	@Failure		409					{object}	dto.ErrorResponse
//	@Failure		500					{object}	dto.ErrorResponse
//	@Security		WebhookSecret
//	@Router			/payments/webhooks [post]
func (h *handler) ConfirmPayment(c *gin.Context) {
	if !webhookAuthorized(c, h.webhookSecret) {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Code: dto.CodeUnauthorized, Message: "unauthorized"})
		return
	}
	var body dto.WebhookRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		validation(c, "invalid request")
		return
	}
	if body.Status != string(entity.PaymentStatusPaid) {
		validation(c, "status must be paid")
		return
	}
	paymentID, err := uuid.Parse(body.PaymentID)
	if err != nil {
		validation(c, "payment_id is invalid")
		return
	}
	err = h.confirmPayment.Execute(c.Request.Context(), usecase.ConfirmPaymentInput{
		EventID:   strings.TrimSpace(body.EventID),
		PaymentID: paymentID,
		Status:    body.Status,
	})
	if err != nil {
		h.report(c, err)
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.Empty{})
}

// ListPurchases godoc
//
//	@Summary		List purchases by email
//	@Description	Returns only orders for the normalized email. An unknown email is an empty list.
//	@Tags			purchases
//	@Produce		json
//	@Param			email	query		string	true	"Customer email"
//	@Success		200		{object}	dto.PurchaseListResponse
//	@Failure		400		{object}	dto.ErrorResponse
//	@Failure		500		{object}	dto.ErrorResponse
//	@Router			/purchases [get]
func (h *handler) ListPurchases(c *gin.Context) {
	email, err := normalizeEmail(c.Query("email"))
	if err != nil {
		writeError(c, err)
		return
	}
	purchases, err := h.listPurchases.Execute(c.Request.Context(), email)
	if err != nil {
		h.report(c, err)
		writeError(c, err)
		return
	}
	response := dto.PurchaseListResponse{Email: email, Purchases: make([]dto.PurchaseResponse, 0, len(purchases))}
	for _, purchase := range purchases {
		tickets := make([]dto.TicketNumber, 0, len(purchase.Tickets))
		for _, ticket := range purchase.Tickets {
			if ticket.OrderID == nil || *ticket.OrderID != purchase.Order.ID {
				continue
			}
			tickets = append(tickets, dto.TicketNumber{
				Number: entity.FormatTicketNumber(ticket.Number, purchase.Raffle.TotalTickets),
			})
		}
		response.Purchases = append(response.Purchases, dto.PurchaseResponse{
			OrderID:     purchase.Order.ID.String(),
			RaffleSlug:  purchase.Raffle.Slug,
			RaffleTitle: purchase.Raffle.Title,
			Quantity:    purchase.Order.Quantity,
			AmountCents: purchase.Order.AmountCents,
			Status:      string(purchase.Order.Status),
			Tickets:     tickets,
		})
	}
	c.JSON(http.StatusOK, response)
}

// CreateRaffle godoc
//
//	@Summary		Create a draft raffle
//	@Description	Admin only. Does not insert ticket rows until the raffle is opened.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Param			input	body		dto.CreateRaffleRequest	true	"Raffle"
//	@Success		201		{object}	dto.RaffleResponse
//	@Failure		400		{object}	dto.ErrorResponse
//	@Failure		401		{object}	dto.ErrorResponse
//	@Failure		500		{object}	dto.ErrorResponse
//	@Security		AdminBearer
//	@Router			/admin/raffles [post]
func (h *handler) CreateRaffle(c *gin.Context) {
	var body dto.CreateRaffleRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		validation(c, "invalid request")
		return
	}
	raffle, err := h.createRaffle.Execute(c.Request.Context(), usecase.CreateRaffleInput{
		Slug:             body.Slug,
		Title:            body.Title,
		Description:      body.Description,
		TicketPriceCents: body.TicketPriceCents,
		TotalTickets:     body.TotalTickets,
	})
	if err != nil {
		h.report(c, err)
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, raffleResponse(raffle))
}

// UpdateRaffleStatus godoc
//
//	@Summary		Change raffle status
//	@Description	Opening a raffle inserts ticket numbers 1..total_tickets once. Closing keeps tickets and orders.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string							true	"Raffle ID"
//	@Param			input	body		dto.UpdateRaffleStatusRequest	true	"Status"
//	@Success		200		{object}	dto.RaffleResponse
//	@Failure		400		{object}	dto.ErrorResponse
//	@Failure		401		{object}	dto.ErrorResponse
//	@Failure		404		{object}	dto.ErrorResponse
//	@Failure		500		{object}	dto.ErrorResponse
//	@Security		AdminBearer
//	@Router			/admin/raffles/{id} [patch]
func (h *handler) UpdateRaffleStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		validation(c, "id is invalid")
		return
	}
	var body dto.UpdateRaffleStatusRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		validation(c, "invalid request")
		return
	}
	status, err := entity.ParseRaffleStatus(body.Status)
	if err != nil {
		writeError(c, err)
		return
	}
	raffle, err := h.updateRaffle.Execute(c.Request.Context(), id, status)
	if err != nil {
		h.report(c, err)
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, raffleResponse(raffle))
}
