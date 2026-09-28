package mem

import (
	"context"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/repository"
)

type UpdateRecord struct {
	Entity string
	Query  repository.Query
	Update repository.Update
	Rows   int64
}

type Store struct {
	mu       sync.Mutex
	Raffles  map[uuid.UUID]entity.Raffle
	Orders   map[uuid.UUID]entity.Order
	Payments map[uuid.UUID]entity.Payment
	Events   map[string]entity.PaymentEvent
	Tickets  map[uuid.UUID]entity.Ticket
	Queue    map[uuid.UUID]entity.QueueItem
	Updates  []UpdateRecord
	Finds    []repository.Query
}

func New() *Store {
	return &Store{
		Raffles:  map[uuid.UUID]entity.Raffle{},
		Orders:   map[uuid.UUID]entity.Order{},
		Payments: map[uuid.UUID]entity.Payment{},
		Events:   map[string]entity.PaymentEvent{},
		Tickets:  map[uuid.UUID]entity.Ticket{},
		Queue:    map[uuid.UUID]entity.QueueItem{},
	}
}

func (s *Store) RafflesRepo() repository.RaffleRepository   { return &raffleRepo{s} }
func (s *Store) OrdersRepo() repository.OrderRepository     { return &orderRepo{s} }
func (s *Store) PaymentsRepo() repository.PaymentRepository { return &paymentRepo{s} }
func (s *Store) EventsRepo() repository.PaymentEventRepository {
	return &eventRepo{s}
}
func (s *Store) TicketsRepo() repository.TicketRepository { return &ticketRepo{s} }
func (s *Store) QueueRepo() repository.QueueRepository    { return &queueRepo{s} }

func (s *Store) record(entityName string, query repository.Query, update repository.Update, rows int64) {
	s.Updates = append(s.Updates, UpdateRecord{Entity: entityName, Query: query, Update: update, Rows: rows})
}

type raffleRepo struct{ store *Store }

func (r *raffleRepo) FindByID(_ context.Context, id uuid.UUID) (entity.Raffle, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	row, ok := r.store.Raffles[id]
	if !ok {
		return entity.Raffle{}, entity.ErrNotFound
	}
	return row, nil
}

func (r *raffleRepo) FindOneByFilters(ctx context.Context, query repository.Query) (entity.Raffle, error) {
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

func (r *raffleRepo) FindAllByFilters(_ context.Context, query repository.Query) ([]entity.Raffle, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	var rows []entity.Raffle
	for _, row := range r.store.Raffles {
		if matchRaffle(row, query.Filters) {
			rows = append(rows, row)
		}
	}
	return limit(rows, query.Limit), nil
}

func (r *raffleRepo) Create(_ context.Context, raffle entity.Raffle) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	for _, existing := range r.store.Raffles {
		if existing.Slug == raffle.Slug || existing.ID == raffle.ID {
			return entity.ErrConflict
		}
	}
	r.store.Raffles[raffle.ID] = raffle
	return nil
}

func (r *raffleRepo) Update(_ context.Context, query repository.Query, update repository.Update) (int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	var rows int64
	for id, row := range r.store.Raffles {
		if !matchRaffle(row, query.Filters) {
			continue
		}
		r.store.Raffles[id] = applyRaffle(row, update)
		rows++
	}
	r.store.record("raffle", query, update, rows)
	return rows, nil
}

type orderRepo struct{ store *Store }

func (r *orderRepo) FindByID(_ context.Context, id uuid.UUID) (entity.Order, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	row, ok := r.store.Orders[id]
	if !ok {
		return entity.Order{}, entity.ErrNotFound
	}
	return row, nil
}

func (r *orderRepo) FindOneByFilters(ctx context.Context, query repository.Query) (entity.Order, error) {
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

func (r *orderRepo) FindAllByFilters(_ context.Context, query repository.Query) ([]entity.Order, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.store.Finds = append(r.store.Finds, query)
	var rows []entity.Order
	for _, row := range r.store.Orders {
		if matchOrder(row, query.Filters) {
			rows = append(rows, row)
		}
	}
	slices.SortFunc(rows, func(a, b entity.Order) int {
		for _, sort := range query.Sorts {
			if sort.Column == repository.ColumnCreatedAt {
				return a.CreatedAt.Compare(b.CreatedAt)
			}
			if sort.Column == repository.ColumnExpiresAt {
				return a.ExpiresAt.Compare(b.ExpiresAt)
			}
		}
		return 0
	})
	return limit(rows, query.Limit), nil
}

func (r *orderRepo) Create(_ context.Context, order entity.Order) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if _, ok := r.store.Orders[order.ID]; ok {
		return entity.ErrConflict
	}
	r.store.Orders[order.ID] = order
	return nil
}

func (r *orderRepo) Update(_ context.Context, query repository.Query, update repository.Update) (int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	var rows int64
	for id, row := range r.store.Orders {
		if !matchOrder(row, query.Filters) {
			continue
		}
		r.store.Orders[id] = applyOrder(row, update)
		rows++
	}
	r.store.record("order", query, update, rows)
	return rows, nil
}

type paymentRepo struct{ store *Store }

func (r *paymentRepo) FindByID(_ context.Context, id uuid.UUID) (entity.Payment, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	row, ok := r.store.Payments[id]
	if !ok {
		return entity.Payment{}, entity.ErrNotFound
	}
	return row, nil
}

func (r *paymentRepo) FindOneByFilters(ctx context.Context, query repository.Query) (entity.Payment, error) {
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

func (r *paymentRepo) FindAllByFilters(_ context.Context, query repository.Query) ([]entity.Payment, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	var rows []entity.Payment
	for _, row := range r.store.Payments {
		if matchPayment(row, query.Filters) {
			rows = append(rows, row)
		}
	}
	return limit(rows, query.Limit), nil
}

func (r *paymentRepo) Create(_ context.Context, payment entity.Payment) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	for _, existing := range r.store.Payments {
		if existing.ID == payment.ID || existing.OrderID == payment.OrderID {
			return entity.ErrConflict
		}
	}
	r.store.Payments[payment.ID] = payment
	return nil
}

func (r *paymentRepo) Update(_ context.Context, query repository.Query, update repository.Update) (int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	var rows int64
	for id, row := range r.store.Payments {
		if !matchPayment(row, query.Filters) {
			continue
		}
		r.store.Payments[id] = applyPayment(row, update)
		rows++
	}
	r.store.record("payment", query, update, rows)
	return rows, nil
}

type eventRepo struct{ store *Store }

func (r *eventRepo) FindByID(_ context.Context, eventID string) (entity.PaymentEvent, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	row, ok := r.store.Events[eventID]
	if !ok {
		return entity.PaymentEvent{}, entity.ErrNotFound
	}
	return row, nil
}

func (r *eventRepo) FindOneByFilters(ctx context.Context, query repository.Query) (entity.PaymentEvent, error) {
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

func (r *eventRepo) FindAllByFilters(_ context.Context, query repository.Query) ([]entity.PaymentEvent, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	var rows []entity.PaymentEvent
	for _, row := range r.store.Events {
		if matchEvent(row, query.Filters) {
			rows = append(rows, row)
		}
	}
	return limit(rows, query.Limit), nil
}

func (r *eventRepo) Create(_ context.Context, event entity.PaymentEvent) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if _, ok := r.store.Events[event.EventID]; ok {
		return entity.ErrAlreadyExists
	}
	r.store.Events[event.EventID] = event
	return nil
}

func (r *eventRepo) Update(context.Context, repository.Query, repository.Update) (int64, error) {
	return 0, nil
}

type ticketRepo struct{ store *Store }

func (r *ticketRepo) FindByID(_ context.Context, id uuid.UUID) (entity.Ticket, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	row, ok := r.store.Tickets[id]
	if !ok {
		return entity.Ticket{}, entity.ErrNotFound
	}
	return row, nil
}

func (r *ticketRepo) FindOneByFilters(ctx context.Context, query repository.Query) (entity.Ticket, error) {
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

func (r *ticketRepo) FindAllByFilters(_ context.Context, query repository.Query) ([]entity.Ticket, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.store.Finds = append(r.store.Finds, query)
	var rows []entity.Ticket
	for _, row := range r.store.Tickets {
		if matchTicket(row, query.Filters) {
			rows = append(rows, row)
		}
	}
	if len(query.Sorts) > 0 && query.Sorts[0].Random {
		rand.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
	} else {
		slices.SortFunc(rows, func(a, b entity.Ticket) int { return a.Number - b.Number })
	}
	return limit(rows, query.Limit), nil
}

func (r *ticketRepo) Create(_ context.Context, ticket entity.Ticket) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	for _, existing := range r.store.Tickets {
		if existing.ID == ticket.ID || (existing.RaffleID == ticket.RaffleID && existing.Number == ticket.Number) {
			return entity.ErrConflict
		}
	}
	r.store.Tickets[ticket.ID] = ticket
	return nil
}

func (r *ticketRepo) Update(_ context.Context, query repository.Query, update repository.Update) (int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	var rows int64
	for id, row := range r.store.Tickets {
		if !matchTicket(row, query.Filters) {
			continue
		}
		r.store.Tickets[id] = applyTicket(row, update)
		rows++
	}
	r.store.record("ticket", query, update, rows)
	return rows, nil
}

type queueRepo struct{ store *Store }

func (r *queueRepo) FindByID(_ context.Context, id uuid.UUID) (entity.QueueItem, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	row, ok := r.store.Queue[id]
	if !ok {
		return entity.QueueItem{}, entity.ErrNotFound
	}
	return row, nil
}

func (r *queueRepo) FindOneByFilters(ctx context.Context, query repository.Query) (entity.QueueItem, error) {
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

func (r *queueRepo) FindAllByFilters(_ context.Context, query repository.Query) ([]entity.QueueItem, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.store.Finds = append(r.store.Finds, query)
	var rows []entity.QueueItem
	for _, row := range r.store.Queue {
		if matchQueue(row, query.Filters) {
			rows = append(rows, row)
		}
	}
	slices.SortFunc(rows, func(a, b entity.QueueItem) int { return a.AvailableAt.Compare(b.AvailableAt) })
	return limit(rows, query.Limit), nil
}

func (r *queueRepo) Create(_ context.Context, item entity.QueueItem) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	for _, existing := range r.store.Queue {
		if existing.ID == item.ID {
			return entity.ErrConflict
		}
		if item.Type == entity.QueueTypeGenerateTicket && existing.Type == item.Type && existing.AggregateID == item.AggregateID {
			switch existing.Status {
			case entity.QueueStatusPending, entity.QueueStatusProcessing, entity.QueueStatusDone:
				return entity.ErrConflict
			}
		}
	}
	r.store.Queue[item.ID] = item
	return nil
}

func (r *queueRepo) Update(_ context.Context, query repository.Query, update repository.Update) (int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	var rows int64
	for id, row := range r.store.Queue {
		if !matchQueue(row, query.Filters) {
			continue
		}
		r.store.Queue[id] = applyQueue(row, update)
		rows++
	}
	r.store.record("queue", query, update, rows)
	return rows, nil
}

func limit[T any](rows []T, n int) []T {
	if n > 0 && len(rows) > n {
		return rows[:n]
	}
	return rows
}

func matchAll(filters []repository.Filter, match func(repository.Filter) bool) bool {
	for _, filter := range filters {
		if !match(filter) {
			return false
		}
	}
	return true
}

func matchRaffle(row entity.Raffle, filters []repository.Filter) bool {
	return matchAll(filters, func(filter repository.Filter) bool {
		switch filter.Op {
		case repository.OpEq:
			switch filter.Column {
			case repository.ColumnID:
				return row.ID == filter.Value.(uuid.UUID)
			case repository.ColumnSlug:
				return row.Slug == filter.Value.(string)
			case repository.ColumnStatus:
				return string(row.Status) == filter.Value.(string)
			}
		case repository.OpGte:
			if filter.Column == repository.ColumnReservedTickets {
				return row.ReservedTickets >= asInt(filter.Value)
			}
		case repository.OpIn:
			if filter.Column == repository.ColumnID {
				return containsUUID(filter.Value, row.ID)
			}
		case repository.OpExpr:
			if filter.Expr.SQL == repository.CapacityFitsSQL {
				qty := asInt(filter.Expr.Args[0])
				return row.ReservedTickets+row.SoldTickets+qty <= row.TotalTickets
			}
		}
		panic("unhandled raffle filter " + string(filter.Op) + " " + filter.Column)
	})
}

func applyRaffle(row entity.Raffle, update repository.Update) entity.Raffle {
	for _, assignment := range update.Assignments {
		switch assignment.Column {
		case repository.ColumnReservedTickets:
			row.ReservedTickets = applyInt(row.ReservedTickets, assignment)
		case repository.ColumnSoldTickets:
			row.SoldTickets = applyInt(row.SoldTickets, assignment)
		case repository.ColumnStatus:
			row.Status = entity.RaffleStatus(assignment.Value.(string))
		case repository.ColumnUpdatedAt:
			row.UpdatedAt = assignment.Value.(time.Time)
		default:
			panic("unhandled raffle assignment " + assignment.Column)
		}
	}
	return row
}

func matchOrder(row entity.Order, filters []repository.Filter) bool {
	return matchAll(filters, func(filter repository.Filter) bool {
		switch filter.Op {
		case repository.OpEq:
			switch filter.Column {
			case repository.ColumnID:
				return row.ID == filter.Value.(uuid.UUID)
			case repository.ColumnStatus:
				return string(row.Status) == filter.Value.(string)
			case repository.ColumnEmail:
				return row.Email == filter.Value.(string)
			}
		case repository.OpLte:
			if filter.Column == repository.ColumnExpiresAt {
				value := filter.Value.(time.Time)
				return !row.ExpiresAt.After(value)
			}
		case repository.OpGt:
			if filter.Column == repository.ColumnExpiresAt {
				value := filter.Value.(time.Time)
				return row.ExpiresAt.After(value)
			}
		}
		panic("unhandled order filter " + string(filter.Op) + " " + filter.Column)
	})
}

func applyOrder(row entity.Order, update repository.Update) entity.Order {
	for _, assignment := range update.Assignments {
		switch assignment.Column {
		case repository.ColumnStatus:
			row.Status = entity.OrderStatus(assignment.Value.(string))
		case repository.ColumnUpdatedAt:
			row.UpdatedAt = assignment.Value.(time.Time)
		default:
			panic("unhandled order assignment " + assignment.Column)
		}
	}
	return row
}

func matchPayment(row entity.Payment, filters []repository.Filter) bool {
	return matchAll(filters, func(filter repository.Filter) bool {
		switch filter.Op {
		case repository.OpEq:
			switch filter.Column {
			case repository.ColumnID:
				return row.ID == filter.Value.(uuid.UUID)
			case repository.ColumnStatus:
				return string(row.Status) == filter.Value.(string)
			case repository.ColumnOrderID:
				return row.OrderID == filter.Value.(uuid.UUID)
			}
		}
		panic("unhandled payment filter")
	})
}

func applyPayment(row entity.Payment, update repository.Update) entity.Payment {
	for _, assignment := range update.Assignments {
		switch assignment.Column {
		case repository.ColumnStatus:
			row.Status = entity.PaymentStatus(assignment.Value.(string))
		case "paid_at":
			paidAt := assignment.Value.(time.Time)
			row.PaidAt = &paidAt
		case repository.ColumnUpdatedAt:
			row.UpdatedAt = assignment.Value.(time.Time)
		default:
			panic("unhandled payment assignment " + assignment.Column)
		}
	}
	return row
}

func matchEvent(row entity.PaymentEvent, filters []repository.Filter) bool {
	return matchAll(filters, func(filter repository.Filter) bool {
		if filter.Op == repository.OpEq && filter.Column == repository.ColumnEventID {
			return row.EventID == filter.Value.(string)
		}
		panic("unhandled event filter")
	})
}

func matchTicket(row entity.Ticket, filters []repository.Filter) bool {
	return matchAll(filters, func(filter repository.Filter) bool {
		switch filter.Op {
		case repository.OpEq:
			switch filter.Column {
			case repository.ColumnID:
				return row.ID == filter.Value.(uuid.UUID)
			case repository.ColumnRaffleID:
				return row.RaffleID == filter.Value.(uuid.UUID)
			case repository.ColumnOrderID:
				return row.OrderID != nil && *row.OrderID == filter.Value.(uuid.UUID)
			}
		case repository.OpIsNull:
			if filter.Column == repository.ColumnOrderID {
				return row.OrderID == nil
			}
		case repository.OpIn:
			switch filter.Column {
			case repository.ColumnID:
				return containsUUID(filter.Value, row.ID)
			case repository.ColumnOrderID:
				return row.OrderID != nil && containsUUID(filter.Value, *row.OrderID)
			}
		}
		panic("unhandled ticket filter " + string(filter.Op) + " " + filter.Column)
	})
}

func applyTicket(row entity.Ticket, update repository.Update) entity.Ticket {
	for _, assignment := range update.Assignments {
		switch assignment.Column {
		case repository.ColumnOrderID:
			id := assignment.Value.(uuid.UUID)
			row.OrderID = &id
		case "assigned_at":
			at := assignment.Value.(time.Time)
			row.AssignedAt = &at
		default:
			panic("unhandled ticket assignment " + assignment.Column)
		}
	}
	return row
}

func matchQueue(row entity.QueueItem, filters []repository.Filter) bool {
	return matchAll(filters, func(filter repository.Filter) bool {
		switch filter.Op {
		case repository.OpEq:
			switch filter.Column {
			case repository.ColumnID:
				return row.ID == filter.Value.(uuid.UUID)
			case repository.ColumnStatus:
				return string(row.Status) == filter.Value.(string)
			case repository.ColumnType:
				return row.Type == filter.Value.(string)
			case repository.ColumnAggregateID:
				return row.AggregateID == filter.Value.(uuid.UUID)
			}
		case repository.OpLte:
			if filter.Column == repository.ColumnAvailableAt {
				return !row.AvailableAt.After(filter.Value.(time.Time))
			}
		case repository.OpLt:
			if filter.Column == repository.ColumnLockedAt {
				if row.LockedAt == nil {
					return false
				}
				return row.LockedAt.Before(filter.Value.(time.Time))
			}
		}
		panic("unhandled queue filter " + string(filter.Op) + " " + filter.Column)
	})
}

func applyQueue(row entity.QueueItem, update repository.Update) entity.QueueItem {
	for _, assignment := range update.Assignments {
		switch assignment.Column {
		case repository.ColumnStatus:
			row.Status = entity.QueueStatus(assignment.Value.(string))
		case repository.ColumnAttempts:
			row.Attempts = applyInt(row.Attempts, assignment)
		case repository.ColumnLockedAt:
			if assignment.SetNull {
				row.LockedAt = nil
			} else {
				at := assignment.Value.(time.Time)
				row.LockedAt = &at
			}
		case "locked_by":
			if assignment.SetNull {
				row.LockedBy = ""
			} else {
				row.LockedBy = assignment.Value.(string)
			}
		case "last_error":
			row.LastError = assignment.Value.(string)
		case repository.ColumnAvailableAt:
			row.AvailableAt = assignment.Value.(time.Time)
		case repository.ColumnUpdatedAt:
			row.UpdatedAt = assignment.Value.(time.Time)
		default:
			panic("unhandled queue assignment " + assignment.Column)
		}
	}
	return row
}

func applyInt(current int, assignment repository.Assignment) int {
	if assignment.Expr == "" {
		return asInt(assignment.Value)
	}
	switch assignment.Expr {
	case "reserved_tickets + ?", "sold_tickets + ?", "attempts + ?":
		return current + asInt(assignment.Args[0])
	case "reserved_tickets - ?", "sold_tickets - ?":
		return current - asInt(assignment.Args[0])
	case "attempts + 1":
		return current + 1
	default:
		panic("unhandled expr " + assignment.Expr)
	}
}

func asInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	default:
		panic("not an int")
	}
}

func containsUUID(value any, id uuid.UUID) bool {
	switch typed := value.(type) {
	case []uuid.UUID:
		return slices.Contains(typed, id)
	default:
		panic("not a uuid slice")
	}
}
