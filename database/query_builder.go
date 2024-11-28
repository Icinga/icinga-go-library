package database

import (
	"fmt"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/jmoiron/sqlx/reflectx"
	"slices"
	"strings"
)

type QueryBuilder interface {
	InsertStatement(stmt InsertStatement) string

	InsertSelectStatement(stmt InsertSelectStatement) string

	SelectStatement(stmt SelectStatement) string

	BuildColumns(entity Entity, columns []string, excludedColumns []string) []string
}

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

func (qb *queryBuilder) InsertStatement(stmt InsertStatement) string {
	columns := qb.BuildColumns(stmt.Entity(), stmt.Columns(), stmt.ExcludedColumns())
	into := stmt.Table()
	if into == "" {
		into = TableName(stmt.Entity())
	}

	return fmt.Sprintf(
		`INSERT INTO "%s" ("%s") VALUES (%s)`,
		into,
		strings.Join(columns, `", "`),
		fmt.Sprintf(":%s", strings.Join(columns, ", :")),
	)
}

func (qb *queryBuilder) InsertSelectStatement(stmt InsertSelectStatement) string {
	selectStmt := qb.SelectStatement(stmt.Select())
	columns := qb.BuildColumns(stmt.Entity(), stmt.Columns(), stmt.ExcludedColumns())
	into := stmt.Table()
	if into == "" {
		into = TableName(stmt.Entity())
	}

	return fmt.Sprintf(
		`INSERT INTO "%s" ("%s") %s`,
		into,
		strings.Join(columns, `", "`),
		selectStmt,
	)
}

func (qb *queryBuilder) SelectStatement(stmt SelectStatement) string {
	columns := qb.BuildColumns(stmt.Entity(), stmt.Columns(), stmt.ExcludeColumns())
	from := stmt.Table()
	if from == "" {
		from = TableName(stmt.Entity())
	}
	where := stmt.Where()
	if where != "" {
		where = fmt.Sprintf(" WHERE %s", where)
	}

	return fmt.Sprintf(
		`SELECT "%s" FROM "%s"%s`,
		strings.Join(columns, `", "`),
		from,
		where,
	)
}

func (qb *queryBuilder) BuildColumns(entity Entity, columns []string, excludedColumns []string) []string {
	var deltaColumns []string
	if len(columns) > 0 {
		deltaColumns = columns
	} else {
		deltaColumns = qb.columnMap.Columns(entity)
	}

	if len(excludedColumns) > 0 {
		deltaColumns = slices.DeleteFunc(
			deltaColumns,
			func(column string) bool {
				return slices.Contains(excludedColumns, column)
			},
		)
	}

	return deltaColumns[:len(deltaColumns):len(deltaColumns)]
}
