package database

import (
	"fmt"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/jmoiron/sqlx/reflectx"
	"github.com/pkg/errors"
	"slices"
	"strings"
)

type QueryBuilder interface {
	InsertStatement(stmt InsertStatement) string

	InsertIgnoreStatement(stmt InsertStatement) (string, error)

	InsertSelectStatement(stmt InsertSelectStatement) string

	SelectStatement(stmt SelectStatement) string

	UpdateStatement(stmt UpdateStatement) (string, error)

	DeleteStatement(stmt DeleteStatement) (string, error)

	DeleteAllStatement(stmt DeleteStatement) (string, error)

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

func (qb *queryBuilder) InsertIgnoreStatement(stmt InsertStatement) (string, error) {
	columns := qb.BuildColumns(stmt.Entity(), stmt.Columns(), stmt.ExcludedColumns())
	into := stmt.Table()
	if into == "" {
		into = TableName(stmt.Entity())
	}

	switch qb.driver {
	case MySQL:
		return fmt.Sprintf(
			`INSERT IGNORE INTO "%s" ("%s") VALUES (%s)`,
			into,
			strings.Join(columns, `", "`),
			fmt.Sprintf(":%s", strings.Join(columns, ", :")),
		), nil
	case PostgreSQL:
		return fmt.Sprintf(
			`INSERT INTO "%s" ("%s") VALUES (%s) ON CONFLICT DO NOTHING`,
			into,
			strings.Join(columns, `", "`),
			fmt.Sprintf(":%s", strings.Join(columns, ", :")),
		), nil
	default:
		return "", errors.New("unknown database driver")
	}
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

func (qb *queryBuilder) UpdateStatement(stmt UpdateStatement) (string, error) {
	table := stmt.Table()
	if table == "" {
		table = TableName(stmt.Entity())
	}
	set := stmt.Set()
	if set == "" {
		return "", errors.New("set cannot be empty")
	}
	where := stmt.Where()
	if where != "" {
		where = fmt.Sprintf(" WHERE %s", where)
	}

	return fmt.Sprintf(
		`UPDATE "%s" SET %s%s`,
		table,
		set,
		where,
	), nil
}

func (qb *queryBuilder) DeleteStatement(stmt DeleteStatement) (string, error) {
	from := stmt.Table()
	if from == "" {
		from = TableName(stmt.Entity())
	}
	where := stmt.Where()
	if where != "" {
		where = fmt.Sprintf(" WHERE %s", where)
	} else {
		return "", errors.New("cannot use DeleteStatement() without where statement - use DeleteAllStatement() instead")
	}

	return fmt.Sprintf(
		`DELETE FROM "%s"%s`,
		from,
		where,
	), nil
}

func (qb *queryBuilder) DeleteAllStatement(stmt DeleteStatement) (string, error) {
	from := stmt.Table()
	if from == "" {
		from = TableName(stmt.Entity())
	}
	where := stmt.Where()
	if where != "" {
		return "", errors.New("cannot use DeleteAllStatement() with where statement - use DeleteStatement() instead")
	}

	return fmt.Sprintf(
		`DELETE FROM "%s"`,
		from,
	), nil
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
