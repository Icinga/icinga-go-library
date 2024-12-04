package database

// SelectStatement is the interface for building SELECT statements.
type SelectStatement interface {
	// From sets the table name for the SELECT statement.
	// Overrides the table name provided by the entity.
	From(table string) SelectStatement

	// SetColumns sets the columns to be selected.
	SetColumns(columns ...string) SelectStatement

	// SetExcludedColumns sets the columns to be excluded from the SELECT statement.
	// Excludes also columns set by SetColumns.
	SetExcludedColumns(columns ...string) SelectStatement

	// SetWhere sets the where clause for the SELECT statement.
	SetWhere(where string) SelectStatement

	// Entity returns the entity associated with the SELECT statement.
	Entity() Entity

	// Table returns the table name for the SELECT statement.
	Table() string

	// Columns returns the columns to be selected.
	Columns() []string

	// ExcludedColumns returns the columns to be excluded from the SELECT statement.
	ExcludedColumns() []string

	// Where returns the where clause for the SELECT statement.
	Where() string
}

// NewSelectStatement returns a new selectStatement for the given entity.
func NewSelectStatement(entity Entity) SelectStatement {
	return &selectStatement{
		entity: entity,
	}
}

// selectStatement is the default implementation of the SelectStatement interface.
type selectStatement struct {
	entity          Entity
	table           string
	columns         []string
	excludedColumns []string
	where           string
}

func (s *selectStatement) From(table string) SelectStatement {
	s.table = table

	return s
}

func (s *selectStatement) SetColumns(columns ...string) SelectStatement {
	s.columns = columns

	return s
}

func (s *selectStatement) SetExcludedColumns(columns ...string) SelectStatement {
	s.excludedColumns = columns

	return s
}

func (s *selectStatement) SetWhere(where string) SelectStatement {
	s.where = where

	return s
}

func (s *selectStatement) Entity() Entity {
	return s.entity
}

func (s *selectStatement) Table() string {
	return s.table
}

func (s *selectStatement) Columns() []string {
	return s.columns
}

func (s *selectStatement) ExcludedColumns() []string {
	return s.excludedColumns
}

func (s *selectStatement) Where() string {
	return s.where
}
