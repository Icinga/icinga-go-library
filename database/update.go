package database

type UpdateStatement interface {
	SetTable(table string) UpdateStatement

	SetSet(set string) UpdateStatement

	SetWhere(where string) UpdateStatement

	Entity() Entity

	Table() string

	Set() string

	Where() string
}

func NewUpdate[T any, V EntityConstraint[T]](entity V) UpdateStatement {
	return &updateStatement[T, V]{
		entity: entity,
	}
}

type updateStatement[T any, V EntityConstraint[T]] struct {
	entity V
	table  string
	set    string
	where  string
}

func (u *updateStatement[T, V]) SetTable(table string) UpdateStatement {
	u.table = table

	return u
}

func (u *updateStatement[T, V]) SetSet(set string) UpdateStatement {
	u.set = set

	return u
}

func (u *updateStatement[T, V]) SetWhere(where string) UpdateStatement {
	u.where = where

	return u
}

func (u *updateStatement[T, V]) Entity() Entity {
	return u.entity
}

func (u *updateStatement[T, V]) Table() string {
	return u.table
}

func (u *updateStatement[T, V]) Set() string {
	return u.set
}

func (u *updateStatement[T, V]) Where() string {
	return u.where
}
