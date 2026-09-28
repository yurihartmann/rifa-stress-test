package service

import (
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/repository"
)

func ReserveCapacity(quantity int) ([]repository.Filter, repository.Update) {
	filters := []repository.Filter{
		repository.Eq(repository.ColumnStatus, string(entity.RaffleStatusOpen)),
		repository.Expr(repository.CapacityFitsSQL, quantity),
	}
	update := repository.Update{Assignments: []repository.Assignment{{
		Column: repository.ColumnReservedTickets,
		Expr:   "reserved_tickets + ?",
		Args:   []any{quantity},
	}}}
	return filters, update
}

func MoveReservedToSold(quantity int) ([]repository.Filter, repository.Update) {
	filters := []repository.Filter{
		repository.Gte(repository.ColumnReservedTickets, quantity),
	}
	update := repository.Update{Assignments: []repository.Assignment{
		{Column: repository.ColumnReservedTickets, Expr: "reserved_tickets - ?", Args: []any{quantity}},
		{Column: repository.ColumnSoldTickets, Expr: "sold_tickets + ?", Args: []any{quantity}},
	}}
	return filters, update
}

func ReleaseReserved(quantity int) ([]repository.Filter, repository.Update) {
	filters := []repository.Filter{
		repository.Gte(repository.ColumnReservedTickets, quantity),
	}
	update := repository.Update{Assignments: []repository.Assignment{{
		Column: repository.ColumnReservedTickets,
		Expr:   "reserved_tickets - ?",
		Args:   []any{quantity},
	}}}
	return filters, update
}

func Touch(now any, assignments ...repository.Assignment) []repository.Assignment {
	out := make([]repository.Assignment, 0, len(assignments)+1)
	out = append(out, assignments...)
	out = append(out, repository.Assignment{Column: repository.ColumnUpdatedAt, Value: now})
	return out
}
