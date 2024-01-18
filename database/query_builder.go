package database

import (
	"fmt"
	"golang.org/x/exp/slices"
	"reflect"
	"sort"
	"strings"
)

// QueryBuilder is an addon for the [DB] type that takes care of all the database statement building shenanigans.
// The recommended use of QueryBuilder is to only use it to generate a single query at a time and not two different
// ones. If for instance you want to generate `INSERT` and `SELECT` queries, it is best to use two different
// QueryBuilder instances. You can use the DB#QueryBuilder() method to get fully initialised instances each time.
type QueryBuilder struct {
	subject         any
	columns         []string
	excludedColumns []string

	// Indicates whether the generated columns should be sorted in ascending order before generating the
	// actual statements. This is intended for unit tests only and shouldn't be necessary for production code.
	sort bool
}

// SetColumns sets the DB columns to be used when building the statements.
// When you do not want the columns to be extracted dynamically, you can use this method to specify them manually.
// Returns the current *[QueryBuilder] receiver and allows you to chain some method calls.
func (qb *QueryBuilder) SetColumns(columns ...string) *QueryBuilder {
	qb.columns = columns
	return qb
}

// SetExcludedColumns excludes the given columns from all the database statements.
// Returns the current *[QueryBuilder] receiver and allows you to chain some method calls.
func (qb *QueryBuilder) SetExcludedColumns(columns ...string) *QueryBuilder {
	qb.excludedColumns = columns
	return qb
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
		`INSERT INTO "%s" ("%s") VALUES (:%s)`,
		TableName(qb.subject),
		strings.Join(columns, `", "`),
		strings.Join(columns, ", :"),
	), len(columns)
}

// InsertIgnore returns an INSERT statement for the query builders subject for
// which the database ignores rows that have already been inserted.
func (qb *QueryBuilder) InsertIgnore(db *DB) (string, int) {
	columns := qb.BuildColumns(db)

	var clause string
	switch db.DriverName() {
	case MySQL:
		// MySQL treats UPDATE id = id as a no-op.
		clause = fmt.Sprintf(`ON DUPLICATE KEY UPDATE "%[1]s" = "%[1]s"`, columns[0])
	case PostgreSQL:
		clause = fmt.Sprintf("ON CONFLICT ON CONSTRAINT %s DO NOTHING", qb.getPgsqlOnConflictConstraint())
	default:
		panic("Driver unsupported: " + db.DriverName())
	}

	return fmt.Sprintf(
		`INSERT INTO "%s" ("%s") VALUES (:%s) %s`,
		TableName(qb.subject),
		strings.Join(columns, `", "`),
		strings.Join(columns, ", :"),
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
		set = append(set, fmt.Sprintf(`"%[1]s" = :%[1]s`, col))
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
		updateColumns = db.columnMap.Columns(upserter.Upsert())
	} else {
		updateColumns = qb.BuildColumns(db)
	}

	return qb.UpsertColumns(db, updateColumns...)
}

// UpsertColumns returns an upsert statement for the query builders subject and the specified update columns.
func (qb *QueryBuilder) UpsertColumns(db *DB, updateColumns ...string) (string, int) {
	var clause, setFormat string
	switch db.DriverName() {
	case MySQL:
		clause = "ON DUPLICATE KEY UPDATE"
		setFormat = `"%[1]s" = VALUES("%[1]s")`
	case PostgreSQL:
		clause = fmt.Sprintf("ON CONFLICT ON CONSTRAINT %s DO UPDATE SET", qb.getPgsqlOnConflictConstraint())
		setFormat = `"%[1]s" = EXCLUDED."%[1]s"`
	default:
		panic("Driver unsupported: " + db.DriverName())
	}

	set := make([]string, 0, len(updateColumns))
	for _, col := range updateColumns {
		set = append(set, fmt.Sprintf(setFormat, col))
	}

	insertColumns := qb.BuildColumns(db)

	return fmt.Sprintf(
		`INSERT INTO "%s" ("%s") VALUES (:%s) %s %s`,
		TableName(qb.subject),
		strings.Join(insertColumns, `", "`),
		strings.Join(insertColumns, ", :"),
		clause,
		strings.Join(set, ", "),
	), len(insertColumns)
}

// Where returns a WHERE clause with named placeholder conditions built from the
// specified scoper/column combined with the AND operator.
func (qb *QueryBuilder) Where(db *DB, subject any) (string, int) {
	t := reflect.TypeOf(subject)
	if t == nil { // Subject is a nil interface value.
		return "", 0
	}

	var columns []string
	if t.Kind() == reflect.String {
		columns = []string{subject.(string)}
	} else if t.Kind() == reflect.Struct || t.Kind() == reflect.Pointer {
		if scoper, ok := subject.(Scoper); ok {
			return qb.Where(db, scoper.Scope())
		}

		columns = db.columnMap.Columns(subject)
	} else { // This should never happen unless someone wants to do some silly things.
		panic(fmt.Sprintf("qb.Where: unknown subject type provided: %q", t.Kind().String()))
	}

	where := make([]string, 0, len(columns))
	for _, col := range columns {
		where = append(where, fmt.Sprintf(`"%[1]s" = :%[1]s`, col))
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
		columns = db.columnMap.Columns(qb.subject)
	}

	if len(qb.excludedColumns) > 0 {
		columns = slices.DeleteFunc(slices.Clone(columns), func(column string) bool {
			return slices.Contains(qb.excludedColumns, column)
		})
	}

	if qb.sort {
		// The order in which the columns appear is not guaranteed as we extract the columns dynamically
		// from the struct. So, we've to sort them here to be able to test the generated statements.
		sort.Strings(columns)
	}

	return slices.Clip(columns)
}

// getPgsqlOnConflictConstraint returns the constraint name of the current [QueryBuilder]'s subject.
// If the subject does not implement the PgsqlOnConflictConstrainter interface, it will simply return
// the table name prefixed with `pk_`.
func (qb *QueryBuilder) getPgsqlOnConflictConstraint() string {
	if constrainter, ok := qb.subject.(PgsqlOnConflictConstrainter); ok {
		return constrainter.PgsqlOnConflictConstraint()
	}

	return "pk_" + TableName(qb.subject)
}
