package repository

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/app/service"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/app/usecase"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	domainrepo "github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/repository"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database/model"
)

func TestPostgresCapacityAndTicketDraw(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}
	db, err := database.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	now := time.Now().UTC()
	raffleID := uuid.New()
	repo := NewRaffleRepository()
	if err := database.Run(context.Background(), db, func(ctx context.Context) error {
		return repo.Create(ctx, entity.Raffle{
			ID: raffleID, Slug: "it-" + raffleID.String(), Title: "IT", TicketPriceCents: 100,
			TotalTickets: 5, Status: entity.RaffleStatusOpen, CreatedAt: now, UpdatedAt: now,
		})
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM tickets WHERE raffle_id = ?", raffleID)
		db.Exec("DELETE FROM queue WHERE aggregate_id IN (SELECT id FROM orders WHERE raffle_id = ?)", raffleID)
		db.Exec("DELETE FROM payment_events WHERE payment_id IN (SELECT id FROM payments WHERE order_id IN (SELECT id FROM orders WHERE raffle_id = ?))", raffleID)
		db.Exec("DELETE FROM payments WHERE order_id IN (SELECT id FROM orders WHERE raffle_id = ?)", raffleID)
		db.Exec("DELETE FROM orders WHERE raffle_id = ?", raffleID)
		db.Exec("DELETE FROM raffles WHERE id = ?", raffleID)
	})

	var success int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var rows int64
			err := database.Run(context.Background(), db, func(ctx context.Context) error {
				filters, update := service.ReserveCapacity(3)
				filters = append([]domainrepo.Filter{domainrepo.Eq(domainrepo.ColumnID, raffleID)}, filters...)
				var updateErr error
				rows, updateErr = repo.Update(ctx, domainrepo.Query{Filters: filters}, update)
				return updateErr
			})
			if err != nil {
				t.Errorf("reserve: %v", err)
				return
			}
			if rows == 1 {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("successful reserves = %d", success)
	}
	var reserved int
	if err := db.Model(&model.Raffle{}).Where("id = ?", raffleID).Select("reserved_tickets").Scan(&reserved).Error; err != nil {
		t.Fatal(err)
	}
	if reserved != 3 {
		t.Fatalf("reserved = %d", reserved)
	}

	opener := usecase.NewUpdateRaffleStatus(repo, NewTicketRepository(), func() time.Time { return now })
	if err := database.Run(context.Background(), db, func(ctx context.Context) error {
		_, err := opener.Execute(ctx, raffleID, entity.RaffleStatusOpen)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var numbers []int
	if err := db.Model(&model.Ticket{}).Where("raffle_id = ?", raffleID).Order("number").Pluck("number", &numbers).Error; err != nil {
		t.Fatal(err)
	}
	if len(numbers) != 5 {
		t.Fatalf("pool = %v", numbers)
	}
	for i, number := range numbers {
		if number != i+1 {
			t.Fatalf("numbers = %v", numbers)
		}
	}

	orderA := uuid.New()
	orderB := uuid.New()
	for _, id := range []uuid.UUID{orderA, orderB} {
		if err := db.Create(&model.Order{
			ID: id, RaffleID: raffleID, Email: id.String() + "@example.com", Quantity: 2, AmountCents: 200,
			Status: "paid", ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	draw := func(orderID uuid.UUID, out chan<- []int) {
		var numbers []int
		err := database.Run(context.Background(), db, func(ctx context.Context) error {
			tickets, err := NewTicketRepository().FindAllByFilters(ctx, domainrepo.Query{
				Filters: []domainrepo.Filter{
					domainrepo.Eq(domainrepo.ColumnRaffleID, raffleID),
					domainrepo.IsNull(domainrepo.ColumnOrderID),
				},
				Sorts: []domainrepo.Sort{{Random: true}},
				Limit: 2,
				Lock:  domainrepo.LockUpdateSkipLocked,
			})
			if err != nil {
				return err
			}
			ids := make([]uuid.UUID, len(tickets))
			for i, ticket := range tickets {
				ids[i] = ticket.ID
				numbers = append(numbers, ticket.Number)
			}
			_, err = NewTicketRepository().Update(ctx, domainrepo.Query{Filters: []domainrepo.Filter{
				domainrepo.In(domainrepo.ColumnID, ids),
				domainrepo.IsNull(domainrepo.ColumnOrderID),
			}}, domainrepo.Update{Assignments: []domainrepo.Assignment{
				{Column: domainrepo.ColumnOrderID, Value: orderID},
				{Column: "assigned_at", Value: now},
			}})
			return err
		})
		if err != nil {
			t.Errorf("draw: %v", err)
		}
		out <- numbers
	}
	first := make(chan []int, 1)
	second := make(chan []int, 1)
	go draw(orderA, first)
	go draw(orderB, second)
	setA := <-first
	setB := <-second
	seen := map[int]struct{}{}
	for _, number := range append(setA, setB...) {
		if _, ok := seen[number]; ok {
			t.Fatalf("shared number %d sets %v %v", number, setA, setB)
		}
		seen[number] = struct{}{}
	}
	if len(seen) != 4 {
		t.Fatalf("assigned %v %v", setA, setB)
	}
}
