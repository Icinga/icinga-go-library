package database

import (
	"fmt"
	"github.com/icinga/icinga-go-library/driver"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"reflect"
	"sort"
	"strings"
)

// QueryBuilder is an addon for the DB type that takes care of all the database statement building shenanigans.
// Note: This type is designed primarily for one-off use (monouso) and subsequent disposal and should only be
// used to generate a single database query type.
type QueryBuilder struct {
	subject         any
	columns         []string
	excludedColumns []string

	// Indicates whether the generated columns should be sorted in ascending order before generating the
	// actual statements. This is intended for unit tests only and shouldn't be necessary for production code.
	sort bool
}

// NewQB returns a fully initialized *QueryBuilder instance for the given subject/struct.
func NewQB(subject any) *QueryBuilder {
	return &QueryBuilder{subject: subject}
}

// SetColumns sets the DB columns to be used when building the statements.
// When you do not want the columns to be extracted dynamically, you can use this method to specify them manually.
func (qb *QueryBuilder) SetColumns(columns ...string) {
	qb.columns = columns
}

// SetExcludedColumns excludes the given columns from all the database statements.
func (qb *QueryBuilder) SetExcludedColumns(columns ...string) {
	qb.excludedColumns = columns
}

// Delete returns a DELETE statement for the query builders subject filtered by ID.
func (qb *QueryBuilder) Delete() string {
	return qb.DeleteBy("id")
}

// DeleteBy returns a DELETE statement for the query builders subject filtered by the given column.
func (qb *QueryBuilder) DeleteBy(column string) string {
	return fmt.Sprintf(`DELETE FROM "%s" WHERE "%s" IN (?)`, TableName(qb.subject), column)
}

// Insert returns an INSERT INTO statement for the query builders subject.
func (qb *QueryBuilder) Insert(db *DB) (string, int) {
	columns := qb.BuildColumns(db)

	return fmt.Sprintf(
		`INSERT INTO "%s" ("%s") VALUES (%s)`,
		TableName(qb.subject),
		strings.Join(columns, `", "`),
		fmt.Sprintf(":%s", strings.Join(columns, ", :")),
	), len(columns)
}

// InsertIgnore returns an INSERT statement for the query builders subject for
// which the database ignores rows that have already been inserted.
func (qb *QueryBuilder) InsertIgnore(db *DB) (string, int) {
	columns := qb.BuildColumns(db)
	table := TableName(qb.subject)

	var clause string
	switch db.DriverName() {
	case driver.MySQL:
		// MySQL treats UPDATE id = id as a no-op.
		clause = fmt.Sprintf(`ON DUPLICATE KEY UPDATE "%s" = "%s"`, columns[0], columns[0])
	case driver.PostgreSQL:
		var constraint string
		if constrainter, ok := qb.subject.(PgsqlOnConflictConstrainter); ok {
			constraint = constrainter.PgsqlOnConflictConstraint()
		} else {
			constraint = "pk_" + table
		}

		clause = fmt.Sprintf("ON CONFLICT ON CONSTRAINT %s DO NOTHING", constraint)
	default:
		db.logger.Fatalw("Driver unsupported", zap.String("driver", db.DriverName()))
	}

	return fmt.Sprintf(
		`INSERT INTO "%s" ("%s") VALUES (%s) %s`,
		table,
		strings.Join(columns, `", "`),
		fmt.Sprintf(":%s", strings.Join(columns, ", :")),
		clause,
	), len(columns)
}

// Select returns a SELECT statement from the query builders subject and the already set columns.
// If no columns are set, they will be extracted from the query builders subject.
// When the query builders subject is of type Scoper, a WHERE clause is appended to the statement.
func (qb *QueryBuilder) Select(db *DB) string {
	var scoper Scoper
	if sc, ok := qb.subject.(Scoper); ok {
		scoper = sc
	}

	return qb.SelectScoped(db, scoper)
}

// SelectScoped returns a SELECT statement from the query builders subject and the already set columns filtered
// by the given scoper/column. When no columns are set, they will be extracted from the query builders subject.
// The argument scoper must either be of type Scoper, string or nil to get SELECT statements without a WHERE clause.
func (qb *QueryBuilder) SelectScoped(db *DB, scoper any) string {
	query := fmt.Sprintf(`SELECT "%s" FROM "%s"`, strings.Join(qb.BuildColumns(db), `", "`), TableName(qb.subject))
	where, placeholders := qb.Where(db, scoper)
	if placeholders > 0 {
		query += ` WHERE ` + where
	}

	return query
}

// Update returns an UPDATE statement for the query builders subject filter by ID column.
func (qb *QueryBuilder) Update(db *DB) (string, int) {
	return qb.UpdateScoped(db, "id")
}

// UpdateScoped returns an UPDATE statement for the query builders subject filtered by the given column/scoper.
// The argument scoper must either be of type Scoper, string or nil to get UPDATE statements without a WHERE clause.
func (qb *QueryBuilder) UpdateScoped(db *DB, scoper any) (string, int) {
	columns := qb.BuildColumns(db)
	set := make([]string, 0, len(columns))

	for _, col := range columns {
		set = append(set, fmt.Sprintf(`"%s" = :%s`, col, col))
	}

	placeholders := len(columns)
	query := `UPDATE "%s" SET %s`
	if where, count := qb.Where(db, scoper); count > 0 {
		placeholders += count
		query += ` WHERE ` + where
	}

	return fmt.Sprintf(query, TableName(qb.subject), strings.Join(set, ", ")), placeholders
}

// Upsert returns an upsert statement for the query builders subject.
func (qb *QueryBuilder) Upsert(db *DB) (string, int) {
	var updateColumns []string
	if upserter, ok := qb.subject.(Upserter); ok {
		updateColumns = db.BuildColumns(upserter.Upsert())
	} else {
		updateColumns = qb.BuildColumns(db)
	}

	return qb.UpsertColumns(db, updateColumns...)
}

// UpsertColumns returns an upsert statement for the query builders subject and the specified update columns.
func (qb *QueryBuilder) UpsertColumns(db *DB, updateColumns ...string) (string, int) {
	insertColumns := qb.BuildColumns(db)
	table := TableName(qb.subject)

	var clause, setFormat string
	switch db.DriverName() {
	case driver.MySQL:
		clause = "ON DUPLICATE KEY UPDATE"
		setFormat = `"%[1]s" = VALUES("%[1]s")`
	case driver.PostgreSQL:
		var constraint string
		if constrainter, ok := qb.subject.(PgsqlOnConflictConstrainter); ok {
			constraint = constrainter.PgsqlOnConflictConstraint()
		} else {
			constraint = "pk_" + table
		}

		clause = fmt.Sprintf("ON CONFLICT ON CONSTRAINT %s DO UPDATE SET", constraint)
		setFormat = `"%[1]s" = EXCLUDED."%[1]s"`
	default:
		db.logger.Fatalw("Driver unsupported", zap.String("driver", db.DriverName()))
	}

	set := make([]string, 0, len(updateColumns))
	for _, col := range updateColumns {
		set = append(set, fmt.Sprintf(setFormat, col))
	}

	return fmt.Sprintf(
		`INSERT INTO "%s" ("%s") VALUES (%s) %s %s`,
		table,
		strings.Join(insertColumns, `", "`),
		fmt.Sprintf(":%s", strings.Join(insertColumns, ", :")),
		clause,
		strings.Join(set, ", "),
	), len(insertColumns)
}

// Where returns a WHERE clause with named placeholder conditions built from the
// specified scoper/column combined with the AND operator.
func (qb *QueryBuilder) Where(db *DB, subject any) (string, int) {
	var columns []string
	t := reflect.TypeOf(subject)
	if t.Kind() == reflect.String {
		columns = []string{subject.(string)}
	} else if t.Kind() == reflect.Struct || t.Kind() == reflect.Pointer {
		if scoper, ok := subject.(Scoper); ok {
			return qb.Where(db, scoper.Scope())
		}

		columns = db.BuildColumns(subject)
	}

	where := make([]string, 0, len(columns))
	for _, col := range columns {
		where = append(where, fmt.Sprintf(`"%s" = :%s`, col, col))
	}

	return strings.Join(where, ` AND `), len(columns)
}

// BuildColumns returns all the Query Builder columns (if specified), otherwise they are
// determined dynamically using its subject. Additionally, it checks whether columns need
// to be excluded and proceeds accordingly.
func (qb *QueryBuilder) BuildColumns(db *DB) []string {
	var columns []string
	if len(qb.columns) > 0 {
		columns = qb.columns
	} else {
		columns = db.BuildColumns(qb.subject)
	}

	if len(qb.excludedColumns) > 0 {
		columns = slices.DeleteFunc(append([]string(nil), columns...), func(column string) bool {
			for _, exclude := range qb.excludedColumns {
				if exclude == column {
					return true
				}
			}

			return false
		})
	}

	if qb.sort {
		// The order in which the columns appear is not guaranteed as we extract the columns dynamically
		// from the struct. So, we've to sort them here to be able to test the generated statements.
		sort.SliceStable(columns, func(a, b int) bool {
			return columns[a] < columns[b]
		})
	}

	return columns
}
