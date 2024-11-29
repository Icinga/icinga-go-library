package database

type SelectStatement interface {
	From(table string) SelectStatement

	SetColumns(columns ...string) SelectStatement

	SetExcludedColumns(columns ...string) SelectStatement

	SetWhere(where string) SelectStatement

	Entity() Entity

	Table() string

	Columns() []string

	ExcludeColumns() []string

	Where() string
}

func NewSelect[T any, V EntityConstraint[T]](entity V) SelectStatement {
	return &selectStatement[T, V]{
		entity: entity,
	}
}

type selectStatement[T any, V EntityConstraint[T]] struct {
	entity          V
	table           string
	columns         []string
	excludedColumns []string
	where           string
}

func (s *selectStatement[T, V]) From(table string) SelectStatement {
	s.table = table

	return s
}

func (s *selectStatement[T, V]) SetColumns(columns ...string) SelectStatement {
	s.columns = columns

	return s
}

func (s *selectStatement[T, V]) SetExcludedColumns(columns ...string) SelectStatement {
	s.excludedColumns = columns

	return s
}

func (s *selectStatement[T, V]) SetWhere(where string) SelectStatement {
	s.where = where

	return s
}

func (s *selectStatement[T, V]) Entity() Entity {
	return s.entity
}

func (s *selectStatement[T, V]) Table() string {
	return s.table
}

func (s *selectStatement[T, V]) Columns() []string {
	return s.columns
}

func (s *selectStatement[T, V]) ExcludeColumns() []string {
	return s.excludedColumns
}

func (s *selectStatement[T, V]) Where() string {
	return s.where
}
