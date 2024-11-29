package database

type DeleteStatement interface {
	From(table string) DeleteStatement

	SetWhere(where string) DeleteStatement

	Entity() Entity

	Table() string

	Where() string
}

func NewDelete[V any, T EntityConstraint[V]](entity T) DeleteStatement {
	return &deleteStatement[V, T]{
		entity: entity,
	}
}

type deleteStatement[V any, T EntityConstraint[V]] struct {
	entity T
	table  string
	where  string
}

func (d *deleteStatement[V, T]) From(table string) DeleteStatement {
	d.table = table

	return d
}

func (d *deleteStatement[V, T]) SetWhere(where string) DeleteStatement {
	d.where = where

	return d
}

func (d *deleteStatement[V, T]) Entity() Entity {
	return d.entity
}

func (d *deleteStatement[V, T]) Table() string {
	return d.table
}

func (d *deleteStatement[V, T]) Where() string {
	return d.where
}
