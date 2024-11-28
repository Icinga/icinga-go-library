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

func NewSelect(entity Entity) SelectStatement {
	return &selectStatement{
		entity: entity,
	}
}

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

func (s *selectStatement) ExcludeColumns() []string {
	return s.excludedColumns
}

func (s *selectStatement) Where() string {
	return s.where
}
