package database

import (
	"github.com/icinga/icinga-go-library/driver"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestQueryBuilder(t *testing.T) {
	t.Parallel()

	conf := Config{
		Type:     "mysql",
		Host:     "localhost",
		Port:     3306,
		Database: "igl-test",
		User:     "igl-test",
		Password: "igl-test",
	}

	driver.Register(nil)
	db, err := NewDbFromConfig(&conf, nil)
	require.NoError(t, err)

	t.Run("SetColumns", func(t *testing.T) {
		qb := NewQB("test")
		qb.SetColumns("a", "b")
		require.Equal(t, []string{"a", "b"}, qb.columns)
	})

	t.Run("ExcludeColumns", func(t *testing.T) {
		qb := NewQB(&test{})
		qb.SetExcludedColumns("a", "b")
		require.Equal(t, []string{"a", "b"}, qb.excludedColumns)
	})

	t.Run("DeleteStatements", func(t *testing.T) {
		qb := NewQB(&test{})
		require.Equal(t, `DELETE FROM "test" WHERE "id" IN (?)`, qb.Delete())
		require.Equal(t, `DELETE FROM "test" WHERE "foo" IN (?)`, qb.DeleteBy("foo"))
	})

	t.Run("InsertStatements", func(t *testing.T) {
		t.Parallel()

		qb := NewQB(&test{})
		qb.sort = true
		qb.SetExcludedColumns("random")

		stmt, columns := qb.Insert(db)
		require.Equal(t, 2, columns)
		require.Equal(t, `INSERT INTO "test" ("name", "value") VALUES (:name, :value)`, stmt)

		qb.SetExcludedColumns("a", "b")
		qb.SetColumns("a", "b", "c", "d")

		stmt, columns = qb.Insert(db)
		require.Equal(t, 2, columns)
		require.Equal(t, `INSERT INTO "test" ("c", "d") VALUES (:c, :d)`, stmt)

		stmt, columns = qb.InsertIgnore(db)
		require.Equal(t, 2, columns)
		require.Equal(t, `INSERT INTO "test" ("c", "d") VALUES (:c, :d) ON DUPLICATE KEY UPDATE "c" = "c"`, stmt)
	})

	t.Run("SelectStatements", func(t *testing.T) {
		t.Parallel()

		qb := NewQB(&test{})
		qb.sort = true

		stmt := qb.Select(db)
		expected := `SELECT "name", "random", "value" FROM "test" WHERE "scoper_id" = :scoper_id`
		require.Equal(t, expected, stmt)

		qb.SetColumns("name", "random", "value")

		stmt = qb.SelectScoped(db, "name")
		require.Equal(t, `SELECT "name", "random", "value" FROM "test" WHERE "name" = :name`, stmt)
	})

	t.Run("UpdateStatements", func(t *testing.T) {
		t.Parallel()

		qb := NewQB(&test{})
		qb.sort = true
		qb.SetExcludedColumns("random")

		stmt, placeholders := qb.Update(db)
		require.Equal(t, 3, placeholders)

		expected := `UPDATE "test" SET "name" = :name, "value" = :value WHERE "id" = :id`
		require.Equal(t, expected, stmt)

		stmt, placeholders = qb.UpdateScoped(db, (&test{}).Scope())
		require.Equal(t, 3, placeholders)
		require.Equal(t, `UPDATE "test" SET "name" = :name, "value" = :value WHERE "scoper_id" = :scoper_id`, stmt)

		qb.SetExcludedColumns("a", "b")
		qb.SetColumns("a", "b", "c", "d")

		stmt, placeholders = qb.UpdateScoped(db, "c")
		require.Equal(t, 3, placeholders)
		require.Equal(t, 3, placeholders)
		require.Equal(t, `UPDATE "test" SET "c" = :c, "d" = :d WHERE "c" = :c`, stmt)
	})

	t.Run("UpsertStatements", func(t *testing.T) {
		t.Parallel()

		qb := NewQB(&test{})
		qb.sort = true
		qb.SetExcludedColumns("random")

		stmt, columns := qb.Upsert(db)
		require.Equal(t, 2, columns)
		require.Equal(t, `INSERT INTO "test" ("name", "value") VALUES (:name, :value) ON DUPLICATE KEY UPDATE "name" = VALUES("name"), "value" = VALUES("value")`, stmt)

		qb.SetExcludedColumns("a", "b")
		qb.SetColumns("a", "b", "c", "d")

		stmt, columns = qb.Upsert(db)
		require.Equal(t, 2, columns)
		require.Equal(t, `INSERT INTO "test" ("c", "d") VALUES (:c, :d) ON DUPLICATE KEY UPDATE "c" = VALUES("c"), "d" = VALUES("d")`, stmt)

		qb.SetExcludedColumns("a")

		stmt, columns = qb.UpsertColumns(db, "b", "c")
		require.Equal(t, 3, columns)
		require.Equal(t, `INSERT INTO "test" ("b", "c", "d") VALUES (:b, :c, :d) ON DUPLICATE KEY UPDATE "b" = VALUES("b"), "c" = VALUES("c")`, stmt)
	})
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
