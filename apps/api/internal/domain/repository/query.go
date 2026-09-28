package repository

const CapacityFitsSQL = "reserved_tickets + sold_tickets + ? <= total_tickets"

const (
	ColumnID              = "id"
	ColumnSlug            = "slug"
	ColumnStatus          = "status"
	ColumnEmail           = "email"
	ColumnRaffleID        = "raffle_id"
	ColumnOrderID         = "order_id"
	ColumnEventID         = "event_id"
	ColumnExpiresAt       = "expires_at"
	ColumnAvailableAt     = "available_at"
	ColumnLockedAt        = "locked_at"
	ColumnType            = "type"
	ColumnAggregateID     = "aggregate_id"
	ColumnNumber          = "number"
	ColumnReservedTickets = "reserved_tickets"
	ColumnSoldTickets     = "sold_tickets"
	ColumnAttempts        = "attempts"
	ColumnUpdatedAt       = "updated_at"
	ColumnCreatedAt       = "created_at"
)

type Op string

const (
	OpEq     Op = "eq"
	OpLt     Op = "lt"
	OpLte    Op = "lte"
	OpGt     Op = "gt"
	OpGte    Op = "gte"
	OpIsNull Op = "is_null"
	OpIn     Op = "in"
	OpExpr   Op = "expr"
)

type LockMode string

const LockUpdateSkipLocked LockMode = "update_skip_locked"

type SQLExpr struct {
	SQL  string
	Args []any
}

type Filter struct {
	Column string
	Op     Op
	Value  any
	Expr   SQLExpr
}

type Sort struct {
	Column string
	Desc   bool
	Random bool
}

type Query struct {
	Filters []Filter
	Sorts   []Sort
	Limit   int
	Lock    LockMode
}

type Assignment struct {
	Column  string
	Value   any
	Expr    string
	Args    []any
	SetNull bool
}

type Update struct {
	Assignments []Assignment
}

func Eq(column string, value any) Filter {
	return Filter{Column: column, Op: OpEq, Value: value}
}

func Lt(column string, value any) Filter {
	return Filter{Column: column, Op: OpLt, Value: value}
}

func Lte(column string, value any) Filter {
	return Filter{Column: column, Op: OpLte, Value: value}
}

func Gt(column string, value any) Filter {
	return Filter{Column: column, Op: OpGt, Value: value}
}

func Gte(column string, value any) Filter {
	return Filter{Column: column, Op: OpGte, Value: value}
}

func IsNull(column string) Filter {
	return Filter{Column: column, Op: OpIsNull}
}

func In(column string, value any) Filter {
	return Filter{Column: column, Op: OpIn, Value: value}
}

func Expr(sql string, args ...any) Filter {
	return Filter{Op: OpExpr, Expr: SQLExpr{SQL: sql, Args: args}}
}
