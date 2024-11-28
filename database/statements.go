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
