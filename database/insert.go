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

func NewInsert[T any, V EntityConstraint[T]](entity V) InsertStatement {
	return &insertStatement[T, V]{
		entity: entity,
	}
}

type insertStatement[T any, V EntityConstraint[T]] struct {
	entity          V
	table           string
	columns         []string
	excludedColumns []string
}

func (i *insertStatement[T, V]) Into(table string) InsertStatement {
	i.table = table

	return i
}

func (i *insertStatement[T, V]) SetColumns(columns ...string) InsertStatement {
	i.columns = columns

	return i
}

func (i *insertStatement[T, V]) SetExcludedColumns(columns ...string) InsertStatement {
	i.excludedColumns = columns

	return i
}

func (i *insertStatement[T, V]) Entity() Entity {
	return i.entity
}

func (i *insertStatement[T, V]) Table() string {
	return i.table
}

func (i *insertStatement[T, V]) Columns() []string {
	return i.columns
}

func (i *insertStatement[T, V]) ExcludedColumns() []string {
	return i.excludedColumns
}

type InsertSelectStatement interface {
	InsertStatement

	SetSelect(stmt SelectStatement) InsertSelectStatement

	Select() SelectStatement
}

func NewInsertSelect[T any, V EntityConstraint[T]](entity V) InsertSelectStatement {
	return &insertSelectStatement[T, V]{
		insertStatement: insertStatement[T, V]{
			entity: entity,
		},
	}
}

type insertSelectStatement[T any, V EntityConstraint[T]] struct {
	insertStatement[T, V]
	selectStmt SelectStatement
}

func (i *insertSelectStatement[T, V]) SetSelect(stmt SelectStatement) InsertSelectStatement {
	i.selectStmt = stmt

	return i
}

func (i *insertSelectStatement[T, V]) Select() SelectStatement {
	return i.selectStmt
}
