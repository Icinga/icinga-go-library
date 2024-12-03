package database

type UpdateStatement interface {
	SetTable(table string) UpdateStatement

	SetSet(set string) UpdateStatement

	SetWhere(where string) UpdateStatement

	Entity() Entity

	Table() string

	Set() string

	Where() string

	apply(opts *updateOptions)
}

func NewUpdateStatement(entity Entity) UpdateStatement {
	return &updateStatement{
		entity: entity,
	}
}

type updateStatement struct {
	entity Entity
	table  string
	set    string
	where  string
}

func (u *updateStatement) SetTable(table string) UpdateStatement {
	u.table = table

	return u
}

func (u *updateStatement) SetSet(set string) UpdateStatement {
	u.set = set

	return u
}

func (u *updateStatement) SetWhere(where string) UpdateStatement {
	u.where = where

	return u
}

func (u *updateStatement) Entity() Entity {
	return u.entity
}

func (u *updateStatement) Table() string {
	return u.table
}

func (u *updateStatement) Set() string {
	return u.set
}

func (u *updateStatement) Where() string {
	return u.where
}

func (u *updateStatement) apply(opts *updateOptions) {
	opts.stmt = u
}

type UpdateOption interface {
	apply(opts *updateOptions)
}

type UpdateOptionFunc func(opts *updateOptions)

func (f UpdateOptionFunc) apply(opts *updateOptions) {
	f(opts)
}

func WithOnUpdate(onUpdate ...OnSuccess[any]) UpdateOption {
	return UpdateOptionFunc(func(opts *updateOptions) {
		opts.onUpdate = onUpdate
	})
}

type updateOptions struct {
	stmt     UpdateStatement
	onUpdate []OnSuccess[any]
}
