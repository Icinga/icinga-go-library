package database

type UpsertStatement interface {
	Into(table string) UpsertStatement

	SetColumns(columns ...string) UpsertStatement

	SetExcludedColumns(columns ...string) UpsertStatement

	Entity() Entity

	Table() string

	Columns() []string

	ExcludedColumns() []string
}

func NewUpsert(entity Entity) UpsertStatement {
	return &upsertStatement{
		entity: entity,
	}
}

type upsertStatement struct {
	entity          Entity
	table           string
	columns         []string
	excludedColumns []string
}

func (i *upsertStatement) Into(table string) UpsertStatement {
	i.table = table

	return i
}

func (i *upsertStatement) SetColumns(columns ...string) UpsertStatement {
	i.columns = columns

	return i
}

func (i *upsertStatement) SetExcludedColumns(columns ...string) UpsertStatement {
	i.excludedColumns = columns

	return i
}

func (i *upsertStatement) Entity() Entity {
	return i.entity
}

func (i *upsertStatement) Table() string {
	return i.table
}

func (i *upsertStatement) Columns() []string {
	return i.columns
}

func (i *upsertStatement) ExcludedColumns() []string {
	return i.excludedColumns
}
