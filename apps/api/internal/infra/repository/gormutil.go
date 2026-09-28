package repository

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	domainrepo "github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var columnName = regexp.MustCompile(`^[a-z_]+$`)

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.ErrNotFound
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return entity.ErrConflict
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return entity.ErrConflict
	}
	return err
}

func applyFilters(db *gorm.DB, filters []domainrepo.Filter) (*gorm.DB, error) {
	for _, filter := range filters {
		switch filter.Op {
		case domainrepo.OpExpr:
			db = db.Where(filter.Expr.SQL, filter.Expr.Args...)
		case domainrepo.OpIsNull:
			if err := validateColumn(filter.Column); err != nil {
				return nil, err
			}
			db = db.Where(filter.Column + " IS NULL")
		case domainrepo.OpEq, domainrepo.OpLt, domainrepo.OpLte, domainrepo.OpGt, domainrepo.OpGte, domainrepo.OpIn:
			if err := validateColumn(filter.Column); err != nil {
				return nil, err
			}
			switch filter.Op {
			case domainrepo.OpEq:
				db = db.Where(filter.Column+" = ?", filter.Value)
			case domainrepo.OpLt:
				db = db.Where(filter.Column+" < ?", filter.Value)
			case domainrepo.OpLte:
				db = db.Where(filter.Column+" <= ?", filter.Value)
			case domainrepo.OpGt:
				db = db.Where(filter.Column+" > ?", filter.Value)
			case domainrepo.OpGte:
				db = db.Where(filter.Column+" >= ?", filter.Value)
			case domainrepo.OpIn:
				db = db.Where(filter.Column+" IN ?", filter.Value)
			}
		default:
			return nil, fmt.Errorf("unsupported filter op %q", filter.Op)
		}
	}
	return db, nil
}

func applyQuery(db *gorm.DB, query domainrepo.Query) (*gorm.DB, error) {
	var err error
	db, err = applyFilters(db, query.Filters)
	if err != nil {
		return nil, err
	}
	for _, sort := range query.Sorts {
		if sort.Random {
			db = db.Clauses(clause.OrderBy{Expression: clause.Expr{SQL: "random()"}})
			continue
		}
		if err := validateColumn(sort.Column); err != nil {
			return nil, err
		}
		if sort.Desc {
			db = db.Order(sort.Column + " DESC")
		} else {
			db = db.Order(sort.Column + " ASC")
		}
	}
	if query.Limit > 0 {
		db = db.Limit(query.Limit)
	}
	if query.Lock == domainrepo.LockUpdateSkipLocked {
		db = db.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
	}
	return db, nil
}

func assignmentValues(update domainrepo.Update) (map[string]any, error) {
	if len(update.Assignments) == 0 {
		return nil, errors.New("update requires assignments")
	}
	values := make(map[string]any, len(update.Assignments))
	for _, assignment := range update.Assignments {
		if err := validateColumn(assignment.Column); err != nil {
			return nil, err
		}
		switch {
		case assignment.SetNull:
			values[assignment.Column] = gorm.Expr("NULL")
		case assignment.Expr != "":
			values[assignment.Column] = gorm.Expr(assignment.Expr, assignment.Args...)
		default:
			values[assignment.Column] = assignment.Value
		}
	}
	return values, nil
}

func validateColumn(name string) error {
	if !columnName.MatchString(name) {
		return fmt.Errorf("invalid column %q", name)
	}
	return nil
}
