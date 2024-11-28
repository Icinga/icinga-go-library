package database

type InsertStatement interface {
	Into(table string) InsertStatement

	SetColumns(columns ...string) InsertStatement

	SetExcludedColumns(columns ...string) InsertStatement

	Entity() Entity

	Table() string

	Columns() []string

	ExcludedColumns() []string
}

func NewInsert(entity Entity) InsertStatement {
	return &insertStatement{
		entity: entity,
	}
}

type insertStatement struct {
	entity          Entity
	table           string
	columns         []string
	excludedColumns []string
}

func (i *insertStatement) Into(table string) InsertStatement {
	i.table = table

	return i
}

func (i *insertStatement) SetColumns(columns ...string) InsertStatement {
	i.columns = columns

	return i
}

func (i *insertStatement) SetExcludedColumns(columns ...string) InsertStatement {
	i.excludedColumns = columns

	return i
}

func (i *insertStatement) Entity() Entity {
	return i.entity
}

func (i *insertStatement) Table() string {
	return i.table
}

func (i *insertStatement) Columns() []string {
	return i.columns
}

func (i *insertStatement) ExcludedColumns() []string {
	return i.excludedColumns
}

type InsertSelectStatement interface {
	InsertStatement

	SetSelect(stmt SelectStatement) InsertSelectStatement

	Select() SelectStatement
}

func NewInsertSelect(entity Entity) InsertSelectStatement {
	return &insertSelectStatement{
		insertStatement: insertStatement{
			entity: entity,
		},
	}
}

type insertSelectStatement struct {
	insertStatement
	selectStmt SelectStatement
}

func (i *insertSelectStatement) SetSelect(stmt SelectStatement) InsertSelectStatement {
	i.selectStmt = stmt

	return i
}

func (i *insertSelectStatement) Select() SelectStatement {
	return i.selectStmt
}

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
