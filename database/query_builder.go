package database

import (
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/jmoiron/sqlx/reflectx"
)

type QueryBuilder interface{}

func NewQueryBuilder(driver string) QueryBuilder {
	return &queryBuilder{
		driver:    driver,
		columnMap: NewColumnMap(reflectx.NewMapperFunc("db", strcase.Snake)),
	}
}

type queryBuilder struct {
	driver    string
	columnMap ColumnMap
}
