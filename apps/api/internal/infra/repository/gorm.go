package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	domainrepo "github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/repository"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database/model"
	"gorm.io/gorm/clause"
)

type RaffleRepository struct{}

func NewRaffleRepository() *RaffleRepository { return &RaffleRepository{} }

func (r *RaffleRepository) FindByID(ctx context.Context, id uuid.UUID) (entity.Raffle, error) {
	return r.FindOneByFilters(ctx, domainrepo.Query{Filters: []domainrepo.Filter{domainrepo.Eq(domainrepo.ColumnID, id)}})
}

func (r *RaffleRepository) FindOneByFilters(ctx context.Context, query domainrepo.Query) (entity.Raffle, error) {
	if query.Limit == 0 {
		query.Limit = 1
	}
	rows, err := r.FindAllByFilters(ctx, query)
	if err != nil {
		return entity.Raffle{}, err
	}
	if len(rows) == 0 {
		return entity.Raffle{}, entity.ErrNotFound
	}
	return rows[0], nil
}

func (r *RaffleRepository) FindAllByFilters(ctx context.Context, query domainrepo.Query) ([]entity.Raffle, error) {
	tx, err := database.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	db, err := applyQuery(tx.Model(&model.Raffle{}), query)
	if err != nil {
		return nil, err
	}
	var rows []model.Raffle
	if err := db.Find(&rows).Error; err != nil {
		return nil, mapErr(err)
	}
	out := make([]entity.Raffle, 0, len(rows))
	for _, row := range rows {
		out = append(out, raffleToEntity(row))
	}
	return out, nil
}

func (r *RaffleRepository) Create(ctx context.Context, raffle entity.Raffle) error {
	tx, err := database.FromContext(ctx)
	if err != nil {
		return err
	}
	row := model.Raffle{
		ID:               raffle.ID,
		Slug:             raffle.Slug,
		Title:            raffle.Title,
		Description:      raffle.Description,
		TicketPriceCents: raffle.TicketPriceCents,
		TotalTickets:     raffle.TotalTickets,
		ReservedTickets:  raffle.ReservedTickets,
		SoldTickets:      raffle.SoldTickets,
		Status:           string(raffle.Status),
		CreatedAt:        raffle.CreatedAt,
		UpdatedAt:        raffle.UpdatedAt,
	}
	return mapErr(tx.Create(&row).Error)
}

func (r *RaffleRepository) Update(ctx context.Context, query domainrepo.Query, update domainrepo.Update) (int64, error) {
	return updateWhere(ctx, &model.Raffle{}, query, update)
}

func raffleToEntity(row model.Raffle) entity.Raffle {
	return entity.Raffle{
		ID:               row.ID,
		Slug:             row.Slug,
		Title:            row.Title,
		Description:      row.Description,
		TicketPriceCents: row.TicketPriceCents,
		TotalTickets:     row.TotalTickets,
		ReservedTickets:  row.ReservedTickets,
		SoldTickets:      row.SoldTickets,
		Status:           entity.RaffleStatus(row.Status),
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func updateWhere(ctx context.Context, modelValue any, query domainrepo.Query, update domainrepo.Update) (int64, error) {
	if len(query.Filters) == 0 {
		return 0, errors.New("update requires filters")
	}
	tx, err := database.FromContext(ctx)
	if err != nil {
		return 0, err
	}
	values, err := assignmentValues(update)
	if err != nil {
		return 0, err
	}
	db, err := applyFilters(tx.Model(modelValue), query.Filters)
	if err != nil {
		return 0, err
	}
	result := db.Updates(values)
	return result.RowsAffected, mapErr(result.Error)
}

type OrderRepository struct{}

func NewOrderRepository() *OrderRepository { return &OrderRepository{} }

func (r *OrderRepository) FindByID(ctx context.Context, id uuid.UUID) (entity.Order, error) {
	return r.FindOneByFilters(ctx, domainrepo.Query{Filters: []domainrepo.Filter{domainrepo.Eq(domainrepo.ColumnID, id)}})
}

func (r *OrderRepository) FindOneByFilters(ctx context.Context, query domainrepo.Query) (entity.Order, error) {
	if query.Limit == 0 {
		query.Limit = 1
	}
	rows, err := r.FindAllByFilters(ctx, query)
	if err != nil {
		return entity.Order{}, err
	}
	if len(rows) == 0 {
		return entity.Order{}, entity.ErrNotFound
	}
	return rows[0], nil
}

func (r *OrderRepository) FindAllByFilters(ctx context.Context, query domainrepo.Query) ([]entity.Order, error) {
	tx, err := database.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	db, err := applyQuery(tx.Model(&model.Order{}), query)
	if err != nil {
		return nil, err
	}
	var rows []model.Order
	if err := db.Find(&rows).Error; err != nil {
		return nil, mapErr(err)
	}
	out := make([]entity.Order, 0, len(rows))
	for _, row := range rows {
		out = append(out, orderToEntity(row))
	}
	return out, nil
}

func (r *OrderRepository) Create(ctx context.Context, order entity.Order) error {
	tx, err := database.FromContext(ctx)
	if err != nil {
		return err
	}
	row := model.Order{
		ID:          order.ID,
		RaffleID:    order.RaffleID,
		Email:       order.Email,
		Quantity:    order.Quantity,
		AmountCents: order.AmountCents,
		Status:      string(order.Status),
		ExpiresAt:   order.ExpiresAt,
		CreatedAt:   order.CreatedAt,
		UpdatedAt:   order.UpdatedAt,
	}
	return mapErr(tx.Create(&row).Error)
}

func (r *OrderRepository) Update(ctx context.Context, query domainrepo.Query, update domainrepo.Update) (int64, error) {
	return updateWhere(ctx, &model.Order{}, query, update)
}

func orderToEntity(row model.Order) entity.Order {
	return entity.Order{
		ID:          row.ID,
		RaffleID:    row.RaffleID,
		Email:       row.Email,
		Quantity:    row.Quantity,
		AmountCents: row.AmountCents,
		Status:      entity.OrderStatus(row.Status),
		ExpiresAt:   row.ExpiresAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

type PaymentRepository struct{}

func NewPaymentRepository() *PaymentRepository { return &PaymentRepository{} }

func (r *PaymentRepository) FindByID(ctx context.Context, id uuid.UUID) (entity.Payment, error) {
	return r.FindOneByFilters(ctx, domainrepo.Query{Filters: []domainrepo.Filter{domainrepo.Eq(domainrepo.ColumnID, id)}})
}

func (r *PaymentRepository) FindOneByFilters(ctx context.Context, query domainrepo.Query) (entity.Payment, error) {
	if query.Limit == 0 {
		query.Limit = 1
	}
	rows, err := r.FindAllByFilters(ctx, query)
	if err != nil {
		return entity.Payment{}, err
	}
	if len(rows) == 0 {
		return entity.Payment{}, entity.ErrNotFound
	}
	return rows[0], nil
}

func (r *PaymentRepository) FindAllByFilters(ctx context.Context, query domainrepo.Query) ([]entity.Payment, error) {
	tx, err := database.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	db, err := applyQuery(tx.Model(&model.Payment{}), query)
	if err != nil {
		return nil, err
	}
	var rows []model.Payment
	if err := db.Find(&rows).Error; err != nil {
		return nil, mapErr(err)
	}
	out := make([]entity.Payment, 0, len(rows))
	for _, row := range rows {
		out = append(out, paymentToEntity(row))
	}
	return out, nil
}

func (r *PaymentRepository) Create(ctx context.Context, payment entity.Payment) error {
	tx, err := database.FromContext(ctx)
	if err != nil {
		return err
	}
	row := model.Payment{
		ID:            payment.ID,
		OrderID:       payment.OrderID,
		Status:        string(payment.Status),
		QRCodePayload: payment.QRCodePayload,
		PaidAt:        payment.PaidAt,
		CreatedAt:     payment.CreatedAt,
		UpdatedAt:     payment.UpdatedAt,
	}
	return mapErr(tx.Create(&row).Error)
}

func (r *PaymentRepository) Update(ctx context.Context, query domainrepo.Query, update domainrepo.Update) (int64, error) {
	return updateWhere(ctx, &model.Payment{}, query, update)
}

func paymentToEntity(row model.Payment) entity.Payment {
	return entity.Payment{
		ID:            row.ID,
		OrderID:       row.OrderID,
		Status:        entity.PaymentStatus(row.Status),
		QRCodePayload: row.QRCodePayload,
		PaidAt:        row.PaidAt,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

type PaymentEventRepository struct{}

func NewPaymentEventRepository() *PaymentEventRepository { return &PaymentEventRepository{} }

func (r *PaymentEventRepository) FindByID(ctx context.Context, eventID string) (entity.PaymentEvent, error) {
	return r.FindOneByFilters(ctx, domainrepo.Query{Filters: []domainrepo.Filter{domainrepo.Eq(domainrepo.ColumnEventID, eventID)}})
}

func (r *PaymentEventRepository) FindOneByFilters(ctx context.Context, query domainrepo.Query) (entity.PaymentEvent, error) {
	if query.Limit == 0 {
		query.Limit = 1
	}
	rows, err := r.FindAllByFilters(ctx, query)
	if err != nil {
		return entity.PaymentEvent{}, err
	}
	if len(rows) == 0 {
		return entity.PaymentEvent{}, entity.ErrNotFound
	}
	return rows[0], nil
}

func (r *PaymentEventRepository) FindAllByFilters(ctx context.Context, query domainrepo.Query) ([]entity.PaymentEvent, error) {
	tx, err := database.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	db, err := applyQuery(tx.Model(&model.PaymentEvent{}), query)
	if err != nil {
		return nil, err
	}
	var rows []model.PaymentEvent
	if err := db.Find(&rows).Error; err != nil {
		return nil, mapErr(err)
	}
	out := make([]entity.PaymentEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, entity.PaymentEvent{EventID: row.EventID, PaymentID: row.PaymentID, CreatedAt: row.CreatedAt})
	}
	return out, nil
}

func (r *PaymentEventRepository) Create(ctx context.Context, event entity.PaymentEvent) error {
	tx, err := database.FromContext(ctx)
	if err != nil {
		return err
	}
	row := model.PaymentEvent{EventID: event.EventID, PaymentID: event.PaymentID, CreatedAt: event.CreatedAt}
	result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: domainrepo.ColumnEventID}}, DoNothing: true}).Create(&row)
	if result.Error != nil {
		return mapErr(result.Error)
	}
	if result.RowsAffected == 0 {
		return entity.ErrAlreadyExists
	}
	return nil
}

func (r *PaymentEventRepository) Update(ctx context.Context, query domainrepo.Query, update domainrepo.Update) (int64, error) {
	return updateWhere(ctx, &model.PaymentEvent{}, query, update)
}

type TicketRepository struct{}

func NewTicketRepository() *TicketRepository { return &TicketRepository{} }

func (r *TicketRepository) FindByID(ctx context.Context, id uuid.UUID) (entity.Ticket, error) {
	return r.FindOneByFilters(ctx, domainrepo.Query{Filters: []domainrepo.Filter{domainrepo.Eq(domainrepo.ColumnID, id)}})
}

func (r *TicketRepository) FindOneByFilters(ctx context.Context, query domainrepo.Query) (entity.Ticket, error) {
	if query.Limit == 0 {
		query.Limit = 1
	}
	rows, err := r.FindAllByFilters(ctx, query)
	if err != nil {
		return entity.Ticket{}, err
	}
	if len(rows) == 0 {
		return entity.Ticket{}, entity.ErrNotFound
	}
	return rows[0], nil
}

func (r *TicketRepository) FindAllByFilters(ctx context.Context, query domainrepo.Query) ([]entity.Ticket, error) {
	tx, err := database.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	db, err := applyQuery(tx.Model(&model.Ticket{}), query)
	if err != nil {
		return nil, err
	}
	var rows []model.Ticket
	if err := db.Find(&rows).Error; err != nil {
		return nil, mapErr(err)
	}
	out := make([]entity.Ticket, 0, len(rows))
	for _, row := range rows {
		out = append(out, ticketToEntity(row))
	}
	return out, nil
}

func (r *TicketRepository) Create(ctx context.Context, ticket entity.Ticket) error {
	tx, err := database.FromContext(ctx)
	if err != nil {
		return err
	}
	row := model.Ticket{
		ID:         ticket.ID,
		RaffleID:   ticket.RaffleID,
		OrderID:    ticket.OrderID,
		Number:     ticket.Number,
		AssignedAt: ticket.AssignedAt,
		CreatedAt:  ticket.CreatedAt,
	}
	return mapErr(tx.Create(&row).Error)
}

func (r *TicketRepository) Update(ctx context.Context, query domainrepo.Query, update domainrepo.Update) (int64, error) {
	return updateWhere(ctx, &model.Ticket{}, query, update)
}

func ticketToEntity(row model.Ticket) entity.Ticket {
	return entity.Ticket{
		ID:         row.ID,
		RaffleID:   row.RaffleID,
		OrderID:    row.OrderID,
		Number:     row.Number,
		AssignedAt: row.AssignedAt,
		CreatedAt:  row.CreatedAt,
	}
}

type QueueRepository struct{}

func NewQueueRepository() *QueueRepository { return &QueueRepository{} }

func (r *QueueRepository) FindByID(ctx context.Context, id uuid.UUID) (entity.QueueItem, error) {
	return r.FindOneByFilters(ctx, domainrepo.Query{Filters: []domainrepo.Filter{domainrepo.Eq(domainrepo.ColumnID, id)}})
}

func (r *QueueRepository) FindOneByFilters(ctx context.Context, query domainrepo.Query) (entity.QueueItem, error) {
	if query.Limit == 0 {
		query.Limit = 1
	}
	rows, err := r.FindAllByFilters(ctx, query)
	if err != nil {
		return entity.QueueItem{}, err
	}
	if len(rows) == 0 {
		return entity.QueueItem{}, entity.ErrNotFound
	}
	return rows[0], nil
}

func (r *QueueRepository) FindAllByFilters(ctx context.Context, query domainrepo.Query) ([]entity.QueueItem, error) {
	tx, err := database.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	db, err := applyQuery(tx.Model(&model.Queue{}), query)
	if err != nil {
		return nil, err
	}
	var rows []model.Queue
	if err := db.Find(&rows).Error; err != nil {
		return nil, mapErr(err)
	}
	out := make([]entity.QueueItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, queueToEntity(row))
	}
	return out, nil
}

func (r *QueueRepository) Create(ctx context.Context, item entity.QueueItem) error {
	tx, err := database.FromContext(ctx)
	if err != nil {
		return err
	}
	row := model.Queue{
		ID:          item.ID,
		Type:        item.Type,
		AggregateID: item.AggregateID,
		Payload:     append([]byte(nil), item.Payload...),
		Status:      string(item.Status),
		Attempts:    item.Attempts,
		AvailableAt: item.AvailableAt,
		LockedAt:    item.LockedAt,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
	if item.LockedBy != "" {
		lockedBy := item.LockedBy
		row.LockedBy = &lockedBy
	}
	if item.LastError != "" {
		lastError := item.LastError
		row.LastError = &lastError
	}
	return mapErr(tx.Create(&row).Error)
}

func (r *QueueRepository) Update(ctx context.Context, query domainrepo.Query, update domainrepo.Update) (int64, error) {
	return updateWhere(ctx, &model.Queue{}, query, update)
}

func queueToEntity(row model.Queue) entity.QueueItem {
	item := entity.QueueItem{
		ID:          row.ID,
		Type:        row.Type,
		AggregateID: row.AggregateID,
		Payload:     append([]byte(nil), row.Payload...),
		Status:      entity.QueueStatus(row.Status),
		Attempts:    row.Attempts,
		AvailableAt: row.AvailableAt,
		LockedAt:    row.LockedAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
	if row.LockedBy != nil {
		item.LockedBy = *row.LockedBy
	}
	if row.LastError != nil {
		item.LastError = *row.LastError
	}
	return item
}
