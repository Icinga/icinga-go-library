package database

import (
	"github.com/icinga/icinga-go-library/testutils"
	"strconv"
	"testing"
)

type MockEntity struct {
	Id    Id
	Name  string
	Age   int
	Email string
}

type Id int

func (i Id) String() string {
	return strconv.Itoa(int(i))
}

func (m MockEntity) ID() ID {
	return m.Id
}

func (m MockEntity) SetID(id ID) {
	m.Id = id.(Id)
}

func (m MockEntity) Fingerprint() Fingerprinter {
	return m
}

type InsertStatementTestData struct {
	Table           string
	Columns         []string
	ExcludedColumns []string
}

type InsertIgnoreStatementTestData struct {
	Driver          string
	Table           string
	Columns         []string
	ExcludedColumns []string
}

type InsertSelectStatementTestData struct {
	Table           string
	Columns         []string
	ExcludedColumns []string
	Select          SelectStatement
}

type UpdateStatementTestData struct {
	Table           string
	Columns         []string
	ExcludedColumns []string
	Where           string
}

type UpsertStatementTestData struct {
	Driver          string
	Table           string
	Columns         []string
	ExcludedColumns []string
}

type DeleteStatementTestData struct {
	Table string
	Where string
}

type DeleteAllStatementTestData struct {
	Table string
}

type SelectStatementTestData struct {
	Table           string
	Columns         []string
	ExcludedColumns []string
	Where           string
}

func TestInsertStatement(t *testing.T) {
	tests := []testutils.TestCase[string, InsertStatementTestData]{
		{
			Name:     "NoColumnsSet",
			Expected: `INSERT INTO "mock_entity" ("age", "email", "id", "name") VALUES (:age, :email, :id, :name)`,
		},
		{
			Name:     "ColumnsSet",
			Expected: `INSERT INTO "mock_entity" ("email", "id", "name") VALUES (:email, :id, :name)`,
			Data: InsertStatementTestData{
				Columns: []string{"id", "name", "email"},
			},
		},
		{
			Name:     "ExcludedColumnsSet",
			Expected: `INSERT INTO "mock_entity" ("age", "id", "name") VALUES (:age, :id, :name)`,
			Data: InsertStatementTestData{
				ExcludedColumns: []string{"email"},
			},
		},
		{
			Name:     "ColumnsAndExcludedColumnsSet",
			Expected: `INSERT INTO "mock_entity" ("id", "name") VALUES (:id, :name)`,
			Data: InsertStatementTestData{
				Columns:         []string{"id", "name", "email"},
				ExcludedColumns: []string{"email"},
			},
		},
		{
			Name:     "OverrideTableName",
			Expected: `INSERT INTO "custom_table_name" ("email", "id", "name") VALUES (:email, :id, :name)`,
			Data: InsertStatementTestData{
				Table:   "custom_table_name",
				Columns: []string{"id", "name", "email"},
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.F(func(data InsertStatementTestData) (string, error) {
			var actual string
			var err error

			stmt := NewInsertStatement(&MockEntity{}).
				SetColumns(data.Columns...).
				SetExcludedColumns(data.ExcludedColumns...)

			if data.Table != "" {
				stmt.Into(data.Table)
			}

			qb := NewTestQueryBuilder(MySQL)
			actual = qb.InsertStatement(stmt)

			return actual, err

		}))
	}
}

func TestInsertIgnoreStatement(t *testing.T) {
	tests := []testutils.TestCase[string, InsertIgnoreStatementTestData]{
		{
			Name:     "NoColumnsSet_MySQL",
			Expected: `INSERT IGNORE INTO "mock_entity" ("age", "email", "id", "name") VALUES (:age, :email, :id, :name)`,
			Data: InsertIgnoreStatementTestData{
				Driver: MySQL,
			},
		},
		{
			Name:     "ColumnsSet_MySQL",
			Expected: `INSERT IGNORE INTO "mock_entity" ("email", "id", "name") VALUES (:email, :id, :name)`,
			Data: InsertIgnoreStatementTestData{
				Driver:  MySQL,
				Columns: []string{"id", "name", "email"},
			},
		},
		{
			Name:     "ExcludedColumnsSet_MySQL",
			Expected: `INSERT IGNORE INTO "mock_entity" ("age", "id", "name") VALUES (:age, :id, :name)`,
			Data: InsertIgnoreStatementTestData{
				Driver:          MySQL,
				ExcludedColumns: []string{"email"},
			},
		},
		{
			Name:     "ColumnsAndExcludedColumnsSet_MySQL",
			Expected: `INSERT IGNORE INTO "mock_entity" ("id", "name") VALUES (:id, :name)`,
			Data: InsertIgnoreStatementTestData{
				Driver:          MySQL,
				Columns:         []string{"id", "name", "email"},
				ExcludedColumns: []string{"email"},
			},
		},
		{
			Name:     "OverrideTableName_MySQL",
			Expected: `INSERT IGNORE INTO "custom_table_name" ("email", "id", "name") VALUES (:email, :id, :name)`,
			Data: InsertIgnoreStatementTestData{
				Driver:  MySQL,
				Table:   "custom_table_name",
				Columns: []string{"id", "name", "email"},
			},
		},
		{
			Name:     "NoColumnsSet_PostgreSQL",
			Expected: `INSERT INTO "mock_entity" ("age", "email", "id", "name") VALUES (:age, :email, :id, :name) ON CONFLICT DO NOTHING`,
			Data: InsertIgnoreStatementTestData{
				Driver: PostgreSQL,
			},
		},
		{
			Name:     "ColumnsSet_PostgreSQL",
			Expected: `INSERT INTO "mock_entity" ("email", "id", "name") VALUES (:email, :id, :name) ON CONFLICT DO NOTHING`,
			Data: InsertIgnoreStatementTestData{
				Driver:  PostgreSQL,
				Columns: []string{"id", "name", "email"},
			},
		},
		{
			Name:     "ExcludedColumnsSet_PostgreSQL",
			Expected: `INSERT INTO "mock_entity" ("age", "id", "name") VALUES (:age, :id, :name) ON CONFLICT DO NOTHING`,
			Data: InsertIgnoreStatementTestData{
				Driver:          PostgreSQL,
				ExcludedColumns: []string{"email"},
			},
		},
		{
			Name:     "ColumnsAndExcludedColumnsSet_PostgreSQL",
			Expected: `INSERT INTO "mock_entity" ("id", "name") VALUES (:id, :name) ON CONFLICT DO NOTHING`,
			Data: InsertIgnoreStatementTestData{
				Driver:          PostgreSQL,
				Columns:         []string{"id", "name", "email"},
				ExcludedColumns: []string{"email"},
			},
		},
		{
			Name:     "OverrideTableName_PostgreSQL",
			Expected: `INSERT INTO "custom_table_name" ("email", "id", "name") VALUES (:email, :id, :name) ON CONFLICT DO NOTHING`,
			Data: InsertIgnoreStatementTestData{
				Driver:          PostgreSQL,
				Table:           "custom_table_name",
				Columns:         []string{"id", "name", "email"},
				ExcludedColumns: nil,
			},
		},
		{
			Name:  "UnsupportedDriver",
			Error: testutils.ErrorIs(ErrUnsupportedDriver),
			Data: InsertIgnoreStatementTestData{
				Driver:          "abcxyz", // Unsupported driver
				Columns:         []string{"id", "name", "email"},
				ExcludedColumns: nil,
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.F(func(data InsertIgnoreStatementTestData) (string, error) {
			var actual string
			var err error

			stmt := NewInsertStatement(&MockEntity{}).
				SetColumns(data.Columns...).
				SetExcludedColumns(data.ExcludedColumns...)

			if data.Table != "" {
				stmt.Into(data.Table)
			}

			qb := NewTestQueryBuilder(data.Driver)
			actual, err = qb.InsertIgnoreStatement(stmt)

			return actual, err

		}))
	}
}

func TestInsertSelectStatement(t *testing.T) {
	tests := []testutils.TestCase[string, InsertSelectStatementTestData]{
		{
			Name:     "ColumnsSet",
			Expected: `INSERT INTO "mock_entity" ("email", "id", "name") SELECT "email", "id", "name" FROM "mock_entity" WHERE id = :id`,
			Data: InsertSelectStatementTestData{
				Columns: []string{"id", "name", "email"},
				Select:  NewSelectStatement(&MockEntity{}).SetColumns("id", "name", "email").SetWhere("id = :id"),
			},
		},
		{
			Name:     "ExcludedColumnsSet",
			Expected: `INSERT INTO "mock_entity" ("age", "id", "name") SELECT "age", "id", "name" FROM "mock_entity" WHERE id = :id`,
			Data: InsertSelectStatementTestData{
				ExcludedColumns: []string{"email"},
				Select:          NewSelectStatement(&MockEntity{}).SetExcludedColumns("email").SetWhere("id = :id"),
			},
		},
		{
			Name:     "ColumnsAndExcludedColumnsSet",
			Expected: `INSERT INTO "mock_entity" ("id", "name") SELECT "id", "name" FROM "mock_entity" WHERE id = :id`,
			Data: InsertSelectStatementTestData{
				Columns:         []string{"id", "name", "email"},
				ExcludedColumns: []string{"email"},
				Select:          NewSelectStatement(&MockEntity{}).SetColumns("id", "name", "email").SetExcludedColumns("email").SetWhere("id = :id"),
			},
		},
		{
			Name:     "OverrideTableName",
			Expected: `INSERT INTO "custom_table_name" ("email", "id", "name") SELECT "email", "id", "name" FROM "mock_entity" WHERE id = :id`,
			Data: InsertSelectStatementTestData{
				Table:   "custom_table_name",
				Columns: []string{"id", "name", "email"},
				Select:  NewSelectStatement(&MockEntity{}).SetColumns("id", "name", "email").SetWhere("id = :id"),
			},
		},
		{
			Name:  "SelectStatementMissing",
			Error: testutils.ErrorIs(ErrMissingStatementPart),
			Data:  InsertSelectStatementTestData{},
		},
		//{
		//	Name: "InvalidColumnName",
		//	Data: InsertStatementTestData{
		//		Columns:         []string{"id", "name", "email", "invalid_column"},
		//		ExcludedColumns: nil,
		//	},
		//	Error: testutils.ErrorIs(ErrInvalidColumnName),
		//},
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.F(func(data InsertSelectStatementTestData) (string, error) {
			var actual string
			var err error

			stmt := NewInsertSelectStatement(&MockEntity{}).
				SetColumns(data.Columns...).
				SetExcludedColumns(data.ExcludedColumns...)

			if data.Select != nil {
				stmt.SetSelect(data.Select.(SelectStatement))
			}

			if data.Table != "" {
				stmt.Into(data.Table)
			}

			qb := NewTestQueryBuilder(MySQL)
			actual, err = qb.InsertSelectStatement(stmt)

			return actual, err

		}))
	}
}

func TestUpdateStatement(t *testing.T) {
	tests := []testutils.TestCase[string, UpdateStatementTestData]{
		{
			Name:  "NoWhereSet",
			Error: testutils.ErrorIs(ErrMissingStatementPart),
		},
		{
			Name:     "ColumnsSet",
			Expected: `UPDATE "mock_entity" SET "email" = :email, "name" = :name WHERE id = :id`,
			Data: UpdateStatementTestData{
				Columns: []string{"name", "email"},
				Where:   "id = :id",
			},
		},
		{
			Name:     "ExcludedColumnsSet",
			Expected: `UPDATE "mock_entity" SET "email" = :email, "name" = :name WHERE id = :id`,
			Data: UpdateStatementTestData{
				ExcludedColumns: []string{"id", "age"},
				Where:           "id = :id",
			},
		},
		{
			Name:     "OverrideTableName",
			Expected: `UPDATE "custom_table_name" SET "email" = :email, "id" = :id, "name" = :name WHERE id = :id`,
			Data: UpdateStatementTestData{
				Table:   "custom_table_name",
				Columns: []string{"id", "name", "email"},
				Where:   "id = :id",
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.F(func(data UpdateStatementTestData) (string, error) {
			var actual string
			var err error

			stmt := NewUpdateStatement(&MockEntity{}).
				SetColumns(data.Columns...).
				SetExcludedColumns(data.ExcludedColumns...)

			if data.Where != "" {
				stmt.SetWhere(data.Where)
			}

			if data.Table != "" {
				stmt.SetTable(data.Table)
			}

			qb := NewTestQueryBuilder(MySQL)
			actual, err = qb.UpdateStatement(stmt)

			return actual, err

		}))
	}
}

func TestUpsertStatement(t *testing.T) {
	tests := []testutils.TestCase[string, UpsertStatementTestData]{
		{
			Name:     "NoColumnsSet_MySQL",
			Expected: `INSERT INTO "mock_entity" ("age", "email", "id", "name") VALUES (:age, :email, :id, :name) ON DUPLICATE KEY UPDATE "age" = VALUES("age"), "email" = VALUES("email"), "id" = VALUES("id"), "name" = VALUES("name")`,
			Data: UpsertStatementTestData{
				Driver: MySQL,
			},
		},
		{
			Name:     "ColumnsSet_MySQL",
			Expected: `INSERT INTO "mock_entity" ("email", "id", "name") VALUES (:email, :id, :name) ON DUPLICATE KEY UPDATE "email" = VALUES("email"), "id" = VALUES("id"), "name" = VALUES("name")`,
			Data: UpsertStatementTestData{
				Driver:  MySQL,
				Columns: []string{"id", "name", "email"},
			},
		},
		{
			Name:     "ExcludedColumnsSet_MySQL",
			Expected: `INSERT INTO "mock_entity" ("age", "id", "name") VALUES (:age, :id, :name) ON DUPLICATE KEY UPDATE "age" = VALUES("age"), "id" = VALUES("id"), "name" = VALUES("name")`,
			Data: UpsertStatementTestData{
				Driver:          MySQL,
				ExcludedColumns: []string{"email"},
			},
		},
		{
			Name:     "ColumnsAndExcludedColumnsSet_MySQL",
			Expected: `INSERT INTO "mock_entity" ("id", "name") VALUES (:id, :name) ON DUPLICATE KEY UPDATE "id" = VALUES("id"), "name" = VALUES("name")`,
			Data: UpsertStatementTestData{
				Driver:          MySQL,
				Columns:         []string{"id", "name", "email"},
				ExcludedColumns: []string{"email"},
			},
		},
		{
			Name:     "OverrideTableName_MySQL",
			Expected: `INSERT INTO "custom_table_name" ("email", "id", "name") VALUES (:email, :id, :name) ON DUPLICATE KEY UPDATE "email" = VALUES("email"), "id" = VALUES("id"), "name" = VALUES("name")`,
			Data: UpsertStatementTestData{
				Driver:  MySQL,
				Table:   "custom_table_name",
				Columns: []string{"id", "name", "email"},
			},
		},
		{
			Name:     "NoColumnsSet_PostgreSQL",
			Expected: `INSERT INTO "mock_entity" ("age", "email", "id", "name") VALUES (:age, :email, :id, :name) ON CONFLICT ON CONSTRAINT pk_mock_entity DO UPDATE SET "age" = EXCLUDED."age", "email" = EXCLUDED."email", "id" = EXCLUDED."id", "name" = EXCLUDED."name"`,
			Data: UpsertStatementTestData{
				Driver: PostgreSQL,
			},
		},
		{
			Name:     "ColumnsSet_PostgreSQL",
			Expected: `INSERT INTO "mock_entity" ("email", "id", "name") VALUES (:email, :id, :name) ON CONFLICT ON CONSTRAINT pk_mock_entity DO UPDATE SET "email" = EXCLUDED."email", "id" = EXCLUDED."id", "name" = EXCLUDED."name"`,
			Data: UpsertStatementTestData{
				Driver:  PostgreSQL,
				Columns: []string{"id", "name", "email"},
			},
		},
		{
			Name:     "ExcludedColumnsSet_PostgreSQL",
			Expected: `INSERT INTO "mock_entity" ("age", "id", "name") VALUES (:age, :id, :name) ON CONFLICT ON CONSTRAINT pk_mock_entity DO UPDATE SET "age" = EXCLUDED."age", "id" = EXCLUDED."id", "name" = EXCLUDED."name"`,
			Data: UpsertStatementTestData{
				Driver:          PostgreSQL,
				ExcludedColumns: []string{"email"},
			},
		},
		{
			Name:     "ColumnsAndExcludedColumnsSet_PostgreSQL",
			Expected: `INSERT INTO "mock_entity" ("id", "name") VALUES (:id, :name) ON CONFLICT ON CONSTRAINT pk_mock_entity DO UPDATE SET "id" = EXCLUDED."id", "name" = EXCLUDED."name"`,
			Data: UpsertStatementTestData{
				Driver:          PostgreSQL,
				Columns:         []string{"id", "name", "email"},
				ExcludedColumns: []string{"email"},
			},
		},
		{
			Name:     "OverrideTableName_PostgreSQL",
			Expected: `INSERT INTO "custom_table_name" ("email", "id", "name") VALUES (:email, :id, :name) ON CONFLICT ON CONSTRAINT pk_custom_table_name DO UPDATE SET "email" = EXCLUDED."email", "id" = EXCLUDED."id", "name" = EXCLUDED."name"`,
			Data: UpsertStatementTestData{
				Driver:  PostgreSQL,
				Table:   "custom_table_name",
				Columns: []string{"id", "name", "email"},
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.F(func(data UpsertStatementTestData) (string, error) {
			var actual string
			var err error

			stmt := NewUpsertStatement(&MockEntity{}).
				SetColumns(data.Columns...).
				SetExcludedColumns(data.ExcludedColumns...)

			if data.Table != "" {
				stmt.Into(data.Table)
			}

			qb := NewTestQueryBuilder(data.Driver)
			actual, _, err = qb.UpsertStatement(stmt)

			return actual, err

		}))
	}
}

func TestDeleteStatement(t *testing.T) {
	tests := []testutils.TestCase[string, DeleteStatementTestData]{
		{
			Name:  "NoWhereSet",
			Error: testutils.ErrorIs(ErrMissingStatementPart),
		},
		{
			Name:     "WhereSet",
			Expected: `DELETE FROM "mock_entity" WHERE id = :id`,
			Data: DeleteStatementTestData{
				Where: "id = :id",
			},
		},
		{
			Name:     "OverrideTableName",
			Expected: `DELETE FROM "custom_table_name" WHERE id = :id`,
			Data: DeleteStatementTestData{
				Table: "custom_table_name",
				Where: "id = :id",
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.F(func(data DeleteStatementTestData) (string, error) {
			var actual string
			var err error

			stmt := NewDeleteStatement(&MockEntity{})

			if data.Where != "" {
				stmt.SetWhere(data.Where)
			}

			if data.Table != "" {
				stmt.From(data.Table)
			}

			qb := NewTestQueryBuilder(MySQL)
			actual, err = qb.DeleteStatement(stmt)

			return actual, err

		}))
	}
}

func TestDeleteAllStatement(t *testing.T) {
	tests := []testutils.TestCase[string, DeleteAllStatementTestData]{
		{
			Name:     "AutoTableName",
			Expected: `DELETE FROM "mock_entity"`,
		},
		{
			Name:     "OverrideTableName",
			Expected: `DELETE FROM "custom_table_name"`,
			Data: DeleteAllStatementTestData{
				Table: "custom_table_name",
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.F(func(data DeleteAllStatementTestData) (string, error) {
			var actual string
			var err error

			stmt := NewDeleteStatement(&MockEntity{})

			if data.Table != "" {
				stmt.From(data.Table)
			}

			qb := NewTestQueryBuilder(MySQL)
			actual, err = qb.DeleteAllStatement(stmt)

			return actual, err

		}))
	}
}

func TestSelectStatement(t *testing.T) {
	tests := []testutils.TestCase[string, SelectStatementTestData]{
		{
			Name:     "NoColumnsSet",
			Expected: `SELECT "age", "email", "id", "name" FROM "mock_entity"`,
		},
		{
			Name:     "ColumnsSet",
			Expected: `SELECT "email", "id", "name" FROM "mock_entity"`,
			Data: SelectStatementTestData{
				Columns: []string{"id", "name", "email"},
			},
		},
		{
			Name:     "ExcludedColumnsSet",
			Expected: `SELECT "age", "id", "name" FROM "mock_entity"`,
			Data: SelectStatementTestData{
				ExcludedColumns: []string{"email"},
			},
		},
		{
			Name:     "ColumnsAndExcludedColumnsSet",
			Expected: `SELECT "id", "name" FROM "mock_entity"`,
			Data: SelectStatementTestData{
				Columns:         []string{"id", "name", "email"},
				ExcludedColumns: []string{"email"},
			},
		},
		{
			Name:     "OverrideTableName",
			Expected: `SELECT "email", "id", "name" FROM "custom_table_name"`,
			Data: SelectStatementTestData{
				Table:   "custom_table_name",
				Columns: []string{"id", "name", "email"},
			},
		},
		{
			Name:     "WhereSet",
			Expected: `SELECT "age", "email", "id", "name" FROM "mock_entity" WHERE id = :id`,
			Data: SelectStatementTestData{
				Where: "id = :id",
			},
		},
		{
			Name:     "MultipleConditionsWhereSet",
			Expected: `SELECT "age", "email", "id", "name" FROM "mock_entity" WHERE id = :id AND name = :name AND email = :email`,
			Data: SelectStatementTestData{
				Where: "id = :id AND name = :name AND email = :email",
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.F(func(data SelectStatementTestData) (string, error) {
			var actual string
			var err error

			stmt := NewSelectStatement(&MockEntity{}).
				SetColumns(data.Columns...).
				SetExcludedColumns(data.ExcludedColumns...)

			if data.Table != "" {
				stmt.From(data.Table)
			}

			if data.Where != "" {
				stmt.SetWhere(data.Where)
			}

			qb := NewTestQueryBuilder(MySQL)
			actual = qb.SelectStatement(stmt)

			return actual, err

		}))
	}
}
