package database

type DeleteStatement interface {
	From(table string) DeleteStatement

	SetWhere(where string) DeleteStatement

	Entity() Entity

	Table() string

	Where() string
}

func NewDelete(entity Entity) DeleteStatement {
	return &deleteStatement{
		entity: entity,
	}
}

type deleteStatement struct {
	entity Entity
	table  string
	where  string
}

func (d *deleteStatement) From(table string) DeleteStatement {
	d.table = table

	return d
}

func (d *deleteStatement) SetWhere(where string) DeleteStatement {
	d.where = where

	return d
}

func (d *deleteStatement) Entity() Entity {
	return d.entity
}

func (d *deleteStatement) Table() string {
	return d.table
}

func (d *deleteStatement) Where() string {
	return d.where
}
