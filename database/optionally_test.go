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

// testEntity represents a mock structure for Entity.
type testEntity struct {
	Id   ID     `db:"id"`
	Name string `db:"name"`
}

func (t *testEntity) TableName() string                 { return "test_subject" }
func (t *testEntity) PgsqlOnConflictConstraint() string { return "pgsql_constrainter" }

func TestFunctionalQueries(t *testing.T) {
	t.Parallel()

	t.Run("MySQL/MariaDB", func(t *testing.T) {
		t.Parallel()

		c := &Config{}
		require.NoError(t, defaults.Set(c), "applying config default should not fail")

		db, err := NewDbFromConfig(c, logging.NewLogger(zaptest.NewLogger(t).Sugar(), time.Minute), RetryConnectorCallbacks{})
		require.NoError(t, err)
		require.Equal(t, MySQL, db.DriverName())

		runFunctionalTests(t, db)
	})
	t.Run("PostgreSQL", func(t *testing.T) {
		t.Parallel()

		c := &Config{Type: "pgsql"}
		require.NoError(t, defaults.Set(c), "applying config default should not fail")

		db, err := NewDbFromConfig(c, logging.NewLogger(zaptest.NewLogger(t).Sugar(), time.Minute), RetryConnectorCallbacks{})
		require.NoError(t, err)
		require.Equal(t, PostgreSQL, db.DriverName())

		runFunctionalTests(t, db)
	})
}

func runFunctionalTests(t *testing.T, db *DB) {
	t.Run("WithStatement", func(t *testing.T) {
		t.Parallel()

		subject := &testEntity{}

		q := NewInsert(db, subject, WithStatement("INSERT INTO test_subject (id, name) VALUES (:id, :name)", 2)).(*queryable)
		stmt, placeholders := q.buildStmt()
		assert.Equal(t, 2, placeholders)
		assert.Equal(t, "INSERT INTO test_subject (id, name) VALUES (:id, :name)", stmt)

		var upsert string
		if db.DriverName() == PostgreSQL {
			upsert = "INSERT INTO test_subject (id, name) VALUES (:id, :name) ON CONFLICT ON CONSTRAINT pgsql_constrainter DO UPDATE SET name = EXCLUDED.name"
		} else {
			upsert = "INSERT INTO test_subject (id, name) VALUES (:id, :name) ON DUPLICATE KEY UPDATE name = VALUES(name)"
		}
		q = NewUpsert(db, subject, WithStatement(upsert, 2)).(*queryable)
		stmt, placeholders = q.buildStmt()
		assert.Equal(t, 2, placeholders)
		assert.Equal(t, upsert, stmt)

		q = NewUpdate(db, subject, WithStatement("UPDATE test_subject SET name = :name WHERE id = :id", 0)).(*queryable)
		stmt, placeholders = q.buildStmt()
		assert.Equal(t, 0, placeholders)
		assert.Equal(t, "UPDATE test_subject SET name = :name WHERE id = :id", stmt)

		q = NewDelete(db, subject, WithStatement("DELETE FROM test_subject WHERE id = :id", 1)).(*queryable)
		stmt, placeholders = q.buildStmt()
		assert.Equal(t, 1, placeholders)
		assert.Equal(t, "DELETE FROM test_subject WHERE id = :id", stmt)
	})

	t.Run("WithColumns", func(t *testing.T) {
		t.Parallel()

		subject := &testEntity{}

		q := NewInsert(db, subject, WithColumns("name")).(*queryable)
		stmt, placeholders := q.buildStmt()
		assert.Equal(t, 1, placeholders)
		assert.Equal(t, "INSERT INTO \"test_subject\" (\"name\") VALUES (:name)", stmt)

		q = NewUpsert(db, subject, WithColumns("name")).(*queryable)
		stmt, placeholders = q.buildStmt()
		assert.Equal(t, 1, placeholders)
		if db.DriverName() == PostgreSQL {
			assert.Equal(t, "INSERT INTO \"test_subject\" (\"name\") VALUES (:name) ON CONFLICT ON CONSTRAINT pgsql_constrainter DO UPDATE SET \"name\" = EXCLUDED.\"name\"", stmt)
		} else {
			assert.Equal(t, "INSERT INTO \"test_subject\" (\"name\") VALUES (:name) ON DUPLICATE KEY UPDATE \"name\" = VALUES(\"name\")", stmt)
		}

		q = NewUpdate(db, subject, WithColumns("name")).(*queryable)
		stmt, placeholders = q.buildStmt()
		assert.Equal(t, 0, placeholders)
		assert.Equal(t, "UPDATE \"test_subject\" SET \"name\" = :name WHERE \"id\" = :id", stmt)
	})

	t.Run("WithoutColumns", func(t *testing.T) {
		t.Parallel()

		subject := &testEntity{}

		q := NewInsert(db, subject, WithoutColumns("id")).(*queryable)
		stmt, placeholders := q.buildStmt()
		assert.Equal(t, 1, placeholders)
		assert.Equal(t, "INSERT INTO \"test_subject\" (\"name\") VALUES (:name)", stmt)

		q = NewUpsert(db, subject, WithoutColumns("id")).(*queryable)
		stmt, placeholders = q.buildStmt()
		assert.Equal(t, 1, placeholders)
		if db.DriverName() == PostgreSQL {
			assert.Equal(t, "INSERT INTO \"test_subject\" (\"name\") VALUES (:name) ON CONFLICT ON CONSTRAINT pgsql_constrainter DO UPDATE SET \"name\" = EXCLUDED.\"name\"", stmt)
		} else {
			assert.Equal(t, "INSERT INTO \"test_subject\" (\"name\") VALUES (:name) ON DUPLICATE KEY UPDATE \"name\" = VALUES(\"name\")", stmt)
		}

		q = NewUpdate(db, subject, WithoutColumns("id")).(*queryable)
		stmt, placeholders = q.buildStmt()
		assert.Equal(t, 0, placeholders)
		assert.Equal(t, "UPDATE \"test_subject\" SET \"name\" = :name WHERE \"id\" = :id", stmt)
	})

	t.Run("WithByColumns", func(t *testing.T) {
		t.Parallel()

		subject := &testEntity{}

		q := NewUpdate(db, subject, WithoutColumns("id"), WithByColumn("name")).(*queryable)
		stmt, placeholders := q.buildStmt()
		assert.Equal(t, 0, placeholders)
		assert.Equal(t, "UPDATE \"test_subject\" SET \"name\" = :name WHERE \"name\" = :name", stmt)

		q = NewDelete(db, subject, WithByColumn("name")).(*queryable)
		stmt, placeholders = q.buildStmt()
		assert.Equal(t, 0, placeholders)
		assert.Equal(t, "DELETE FROM \"test_subject\" WHERE \"name\" IN (?)", stmt)
	})

	t.Run("WithIgnoreOnError", func(t *testing.T) {
		t.Parallel()

		if db.DriverName() != PostgreSQL {
			t.Skipf("Skipping IgnoreOnError test case for %q driver", db.DriverName())
		}

		subject := &testEntity{}
		q := NewInsert(db, subject, WithColumns("id", "name"), WithIgnoreOnError()).(*queryable)
		stmt, placeholders := q.buildStmt()
		assert.Equal(t, 2, placeholders)
		assert.Equal(t, "INSERT INTO \"test_subject\" (\"id\", \"name\") VALUES (:id, :name) ON CONFLICT ON CONSTRAINT pgsql_constrainter DO NOTHING", stmt)
	})
}
