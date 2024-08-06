package database

import (
	"github.com/creasty/defaults"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"testing"
	"time"
)

func TestQueryBuilder(t *testing.T) {
	t.Parallel()

	t.Run("MySQL", func(t *testing.T) {
		t.Parallel()

		c := &Config{}
		require.NoError(t, defaults.Set(c), "applying config default should not fail")

		db, err := NewDbFromConfig(c, logging.NewLogger(zaptest.NewLogger(t).Sugar(), time.Hour), RetryConnectorCallbacks{})
		require.NoError(t, err)
		require.Equal(t, MySQL, db.DriverName())
		runTests(t, db)
	})

	t.Run("PostgreSQL", func(t *testing.T) {
		t.Parallel()

		c := &Config{Type: "pgsql"}
		require.NoError(t, defaults.Set(c), "applying config default should not fail")

		db, err := NewDbFromConfig(c, logging.NewLogger(zaptest.NewLogger(t).Sugar(), time.Hour), RetryConnectorCallbacks{})
		require.NoError(t, err)
		require.Equal(t, PostgreSQL, db.DriverName())
		runTests(t, db)

		t.Run("OnConflictConstrainter", func(t *testing.T) {
			t.Parallel()

			qb := newQB(db, &pgsqlConstraintName{})
			qb.SetColumns("a")

			stmt, columns := qb.Upsert()
			assert.Equal(t, 1, columns)
			assert.Equal(t, `INSERT INTO "test" ("a") VALUES (:a) ON CONFLICT ON CONSTRAINT idx_custom_constraint DO UPDATE SET "a" = EXCLUDED."a"`, stmt)

			stmt, columns = qb.InsertIgnore()
			assert.Equal(t, 1, columns)
			assert.Equal(t, `INSERT INTO "test" ("a") VALUES (:a) ON CONFLICT ON CONSTRAINT idx_custom_constraint DO NOTHING`, stmt)
		})
	})
}

func runTests(t *testing.T, db *DB) {
	t.Run("SetColumns", func(t *testing.T) {
		t.Parallel()

		qb := &QueryBuilder{subject: "test"}
		qb.SetColumns("a", "b")
		assert.Equal(t, []string{"a", "b"}, qb.columns)
	})

	t.Run("ExcludeColumns", func(t *testing.T) {
		t.Parallel()

		qb := &QueryBuilder{subject: &test{}}
		qb.SetExcludedColumns("a", "b")
		assert.Equal(t, []string{"a", "b"}, qb.excludedColumns)
	})

	t.Run("DeleteStatements", func(t *testing.T) {
		t.Parallel()

		qb := &QueryBuilder{subject: &test{}}
		assert.Equal(t, `DELETE FROM "test" WHERE "id" IN (?)`, qb.Delete())
		assert.Equal(t, `DELETE FROM "test" WHERE "foo" IN (?)`, qb.DeleteBy("foo"))
	})

	t.Run("WhereClause", func(t *testing.T) {
		t.Parallel()

		// Is invalid column (1)
		qb := newQB(db, 1)
		assert.PanicsWithValue(t, "qb.Where: unknown subject type provided: \"int\"", func() { _, _ = qb.Where(1) })

		var nilPtr Scoper // Interface nil value
		qb = &QueryBuilder{subject: nilPtr}
		clause, placeholder := qb.Where(nilPtr)
		assert.Equal(t, 0, placeholder)
		assert.Empty(t, clause)

		clause, placeholder = qb.Where("id")
		assert.Equal(t, 1, placeholder)
		assert.Equal(t, "\"id\" = :id", clause)

		assertScoperID := func(clause string, placeholder int) {
			assert.Equal(t, 1, placeholder)
			assert.Equal(t, "\"scoper_id\" = :scoper_id", clause)
		}

		var reference test
		qb = newQB(db, &reference)
		clause, placeholder = qb.Where(&reference)
		assertScoperID(clause, placeholder)

		nonNilPtr := new(test)
		qb = newQB(db, nonNilPtr)
		clause, placeholder = qb.Where(nonNilPtr)
		assertScoperID(clause, placeholder)
	})

	t.Run("InsertStatements", func(t *testing.T) {
		t.Parallel()

		qb := newQB(db, &test{})
		qb.sort = true
		qb.SetExcludedColumns("random")

		stmt, columns := qb.Insert()
		assert.Equal(t, 2, columns)
		assert.Equal(t, `INSERT INTO "test" ("name", "value") VALUES (:name, :value)`, stmt)

		qb.SetExcludedColumns("a", "b")
		qb.SetColumns("a", "b", "c", "d")

		stmt, columns = qb.Insert()
		assert.Equal(t, 2, columns)
		assert.Equal(t, `INSERT INTO "test" ("c", "d") VALUES (:c, :d)`, stmt)

		stmt, columns = qb.InsertIgnore()
		assert.Equal(t, 2, columns)
		if db.DriverName() == MySQL {
			assert.Equal(t, `INSERT INTO "test" ("c", "d") VALUES (:c, :d) ON DUPLICATE KEY UPDATE "c" = "c"`, stmt)
		} else {
			assert.Equal(t, `INSERT INTO "test" ("c", "d") VALUES (:c, :d) ON CONFLICT ON CONSTRAINT pk_test DO NOTHING`, stmt)
		}
	})

	t.Run("SelectStatements", func(t *testing.T) {
		t.Parallel()

		qb := newQB(db, &test{})
		qb.sort = true

		stmt := qb.Select()
		expected := `SELECT "name", "random", "value" FROM "test" WHERE "scoper_id" = :scoper_id`
		assert.Equal(t, expected, stmt)

		qb.SetColumns("name", "random", "value")

		stmt = qb.SelectScoped("name")
		assert.Equal(t, `SELECT "name", "random", "value" FROM "test" WHERE "name" = :name`, stmt)
	})

	t.Run("UpdateStatements", func(t *testing.T) {
		t.Parallel()

		qb := newQB(db, &test{})
		qb.sort = true
		qb.SetExcludedColumns("random")

		stmt, placeholders := qb.Update()
		assert.Equal(t, 3, placeholders)

		expected := `UPDATE "test" SET "name" = :name, "value" = :value WHERE "id" = :id`
		assert.Equal(t, expected, stmt)

		stmt, placeholders = qb.UpdateScoped((&test{}).Scope())
		assert.Equal(t, 3, placeholders)
		assert.Equal(t, `UPDATE "test" SET "name" = :name, "value" = :value WHERE "scoper_id" = :scoper_id`, stmt)

		qb.SetExcludedColumns("a", "b")
		qb.SetColumns("a", "b", "c", "d")

		stmt, placeholders = qb.UpdateScoped("c")
		assert.Equal(t, 3, placeholders)
		assert.Equal(t, 3, placeholders)
		assert.Equal(t, `UPDATE "test" SET "c" = :c, "d" = :d WHERE "c" = :c`, stmt)
	})

	t.Run("UpsertStatements", func(t *testing.T) {
		t.Parallel()

		qb := newQB(db, &test{})
		qb.sort = true
		qb.SetExcludedColumns("random")

		stmt, columns := qb.Upsert()
		assert.Equal(t, 2, columns)

		expected := `INSERT INTO "test" ("name", "value") VALUES (:name, :value)`
		if db.DriverName() == MySQL {
			assert.Equal(t, expected+` ON DUPLICATE KEY UPDATE "name" = VALUES("name"), "value" = VALUES("value")`, stmt)
		} else {
			assert.Equal(t, expected+` ON CONFLICT ON CONSTRAINT pk_test DO UPDATE SET "name" = EXCLUDED."name", "value" = EXCLUDED."value"`, stmt)
		}

		qb.SetExcludedColumns("a", "b")
		qb.SetColumns("a", "b", "c", "d")

		expected = `INSERT INTO "test" ("c", "d") VALUES (:c, :d)`
		stmt, columns = qb.Upsert()
		assert.Equal(t, 2, columns)
		if db.DriverName() == MySQL {
			assert.Equal(t, expected+` ON DUPLICATE KEY UPDATE "c" = VALUES("c"), "d" = VALUES("d")`, stmt)
		} else {
			assert.Equal(t, expected+` ON CONFLICT ON CONSTRAINT pk_test DO UPDATE SET "c" = EXCLUDED."c", "d" = EXCLUDED."d"`, stmt)
		}

		qb.SetExcludedColumns("a")

		expected = `INSERT INTO "test" ("b", "c", "d") VALUES (:b, :c, :d)`
		stmt, columns = qb.UpsertColumns("b", "c")
		assert.Equal(t, 3, columns)
		if db.DriverName() == MySQL {
			assert.Equal(t, expected+` ON DUPLICATE KEY UPDATE "b" = VALUES("b"), "c" = VALUES("c")`, stmt)
		} else {
			assert.Equal(t, expected+` ON CONFLICT ON CONSTRAINT pk_test DO UPDATE SET "b" = EXCLUDED."b", "c" = EXCLUDED."c"`, stmt)
		}
	})
}

func newQB(db *DB, subject any) *QueryBuilder {
	return &QueryBuilder{subject: subject, db: db}
}

type test struct {
	Name   string
	Value  string
	Random string
}

func (t *test) Scope() any {
	return struct {
		ScoperID string
	}{}
}

type pgsqlConstraintName struct {
	*test
}

func (p *pgsqlConstraintName) PgsqlOnConflictConstraint() string {
	return "idx_custom_constraint"
}

func (p *pgsqlConstraintName) TableName() string {
	return "test"
}
