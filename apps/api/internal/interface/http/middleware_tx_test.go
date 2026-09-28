package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database/model"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/repository"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/testkit"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestTransactionCommitRollbackAndPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testkit.OpenSQLite(t)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewRaffleRepository()
	engine := gin.New()
	engine.POST("/write", transactionMiddleware(db, nil, nil), func(c *gin.Context) {
		switch c.Query("mode") {
		case "fail":
			_ = repo.Create(c.Request.Context(), sampleRaffle())
			c.JSON(http.StatusConflict, gin.H{"code": "insufficient_tickets"})
		case "panic":
			_ = repo.Create(c.Request.Context(), sampleRaffle())
			panic("boom")
		default:
			if err := repo.Create(c.Request.Context(), sampleRaffle()); err != nil {
				t.Errorf("create: %v", err)
				c.Status(http.StatusInternalServerError)
				return
			}
			c.JSON(http.StatusCreated, gin.H{"ok": true})
		}
	})

	ok := httptest.NewRecorder()
	engine.ServeHTTP(ok, httptest.NewRequest(http.MethodPost, "/write", nil))
	if ok.Code != http.StatusCreated {
		t.Fatalf("commit status %d body %s", ok.Code, ok.Body.String())
	}
	if countRaffles(t, db) != 1 {
		t.Fatalf("committed rows = %d", countRaffles(t, db))
	}

	fail := httptest.NewRecorder()
	engine.ServeHTTP(fail, httptest.NewRequest(http.MethodPost, "/write?mode=fail", nil))
	if fail.Code != http.StatusConflict {
		t.Fatalf("rollback status %d", fail.Code)
	}
	if countRaffles(t, db) != 1 {
		t.Fatalf("rows after rollback = %d", countRaffles(t, db))
	}

	boom := httptest.NewRecorder()
	engine.ServeHTTP(boom, httptest.NewRequest(http.MethodPost, "/write?mode=panic", nil))
	if boom.Code != http.StatusInternalServerError {
		t.Fatalf("panic status %d body %s", boom.Code, boom.Body.String())
	}
	if countRaffles(t, db) != 1 {
		t.Fatalf("rows after panic = %d", countRaffles(t, db))
	}
}

func TestCommitFailureReturns500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

	engine := gin.New()
	engine.POST("/ok", transactionMiddleware(db, nil, nil), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/ok", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() == `{"ok":true}` {
		t.Fatal("success body was released after commit failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func sampleRaffle() entity.Raffle {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return entity.Raffle{
		ID:               uuid.New(),
		Slug:             uuid.NewString(),
		Title:            "Car",
		TicketPriceCents: 100,
		TotalTickets:     10,
		Status:           entity.RaffleStatusDraft,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func countRaffles(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&model.Raffle{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}
