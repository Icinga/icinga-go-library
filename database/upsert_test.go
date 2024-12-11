package database

import (
	"context"
	"github.com/creasty/defaults"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/testutils"
	"go.uber.org/zap/zapcore"
	"testing"
	"time"
)

type UpsertStreamedTestData struct {
	Entities  []MockEntity
	Statement UpsertStatement
	Callbacks []OnSuccess[MockEntity]
}

func initTestDb(db *DB, logger *logging.Logger) {
	_, err := db.Query("DROP TABLE IF EXISTS mock_entity")
	if err != nil {
		logger.Fatal(err)
	}

	_, err = db.Query(`CREATE TABLE mock_entity ("id" INTEGER PRIMARY KEY, "name" VARCHAR(255), "age" INTEGER, "email" VARCHAR(255))`)
	if err != nil {
		logger.Fatal(err)
	}

	entities := []MockEntity{
		{Id: 1, Name: "test1", Age: 10, Email: "test1@test.com"},
		{Id: 2, Name: "test2", Age: 20, Email: "test2@test.com"},
		{Id: 3, Name: "test3", Age: 30, Email: "test3@test.com"},
		{Id: 4, Name: "test4", Age: 40, Email: "test4@test.com"},
	}

	for _, entity := range entities {
		_, err = db.NamedExec(`INSERT INTO mock_entity ("id", "name", "age", "email") VALUES (:id, :name, :age, :email)`, entity)
		if err != nil {
			logger.Fatal(err)
		}
	}
}

func TestUpsertStreamed(t *testing.T) {
	tests := []testutils.TestCase[[]MockEntity, UpsertStreamedTestData]{
		{
			Name: "Insert",
			Expected: []MockEntity{
				{Id: 1, Name: "test1", Age: 10, Email: "test1@test.com"},
				{Id: 2, Name: "test2", Age: 20, Email: "test2@test.com"},
				{Id: 3, Name: "test3", Age: 30, Email: "test3@test.com"},
				{Id: 4, Name: "test4", Age: 40, Email: "test4@test.com"},
				{Id: 5, Name: "test5", Age: 50, Email: "test5@test.com"},
				{Id: 6, Name: "test6", Age: 60, Email: "test6@test.com"},
				{Id: 7, Name: "test7", Age: 70, Email: "test7@test.com"},
				{Id: 8, Name: "test8", Age: 80, Email: "test8@test.com"},
			},
			Data: UpsertStreamedTestData{
				Entities: []MockEntity{
					{Id: 5, Name: "test5", Age: 50, Email: "test5@test.com"},
					{Id: 6, Name: "test6", Age: 60, Email: "test6@test.com"},
					{Id: 7, Name: "test7", Age: 70, Email: "test7@test.com"},
					{Id: 8, Name: "test8", Age: 80, Email: "test8@test.com"},
				},
			},
		},
		{
			Name: "Update",
			Expected: []MockEntity{
				{Id: 1, Name: "test1", Age: 100, Email: "test1@test.com"},
				{Id: 2, Name: "test2", Age: 200, Email: "test2@test.com"},
				{Id: 3, Name: "test3", Age: 300, Email: "test3@test.com"},
				{Id: 4, Name: "test4", Age: 400, Email: "test4@test.com"},
			},
			Data: UpsertStreamedTestData{
				Entities: []MockEntity{
					{Id: 1, Name: "test1", Age: 100, Email: "test1@test.com"},
					{Id: 2, Name: "test2", Age: 200, Email: "test2@test.com"},
					{Id: 3, Name: "test3", Age: 300, Email: "test3@test.com"},
					{Id: 4, Name: "test4", Age: 400, Email: "test4@test.com"},
				},
			},
		},
		{
			Name: "InsertAndUpdate",
			Expected: []MockEntity{
				{Id: 1, Name: "test1", Age: 10, Email: "test1@test.com"},
				{Id: 2, Name: "test2", Age: 20, Email: "test2@test.com"},
				{Id: 3, Name: "test3", Age: 30, Email: "test3@test.com"},
				{Id: 4, Name: "test40", Age: 40, Email: "test40@test.com"},
				{Id: 5, Name: "test50", Age: 50, Email: "test50@test.com"},
				{Id: 6, Name: "test6", Age: 60, Email: "test6@test.com"},
			},
			Data: UpsertStreamedTestData{
				Entities: []MockEntity{
					{Id: 5, Name: "test5", Age: 50, Email: "test5@test.com"},
					{Id: 6, Name: "test6", Age: 60, Email: "test6@test.com"},
					{Id: 4, Name: "test40", Age: 40, Email: "test40@test.com"},
					{Id: 5, Name: "test50", Age: 50, Email: "test50@test.com"},
				},
			},
		},
		{
			Name: "WithStatement",
			Expected: []MockEntity{
				{Id: 1, Name: "test1", Age: 10, Email: "test1@test.com"},
				{Id: 2, Name: "test2", Age: 20, Email: "test2@test.com"},
				{Id: 3, Name: "test3", Age: 30, Email: "test3@test.com"},
				{Id: 4, Name: "test4", Age: 40, Email: "test4@test.com"},
				{Id: 5, Name: "test5", Age: 50, Email: "test5@test.com"},
			},
			Data: UpsertStreamedTestData{
				Entities: []MockEntity{
					{Id: 5, Name: "test5", Age: 50, Email: "test5@test.com"},
				},
				Statement: NewUpsertStatement(&MockEntity{}),
			},
		},
		{
			Name:  "WithFalseStatement",
			Error: testutils.ErrorContains("can't perform"), // TODO (jr): is it the right way?
			Data: UpsertStreamedTestData{
				Entities: []MockEntity{
					{Id: 5, Name: "test5", Age: 50, Email: "test5@test.com"},
				},
				Statement: NewUpsertStatement(&MockEntity{}).Into("false_table"),
			},
		},
	}

	var (
		upsertError    error
		defaultOptions Options
		ctx            = context.Background()
		entities       = make(chan MockEntity)
	)

	logs, err := logging.NewLoggingFromConfig(
		"Icinga Kubernetes",
		logging.Config{Level: zapcore.DebugLevel, Output: "console", Interval: time.Second},
	)
	if err != nil {
		t.Fatalf("cannot configure logging: %v", err)
	}

	err = defaults.Set(&defaultOptions)
	if err != nil {
		t.Fatalf("cannot set default options: %v", err)
	}

	db, err := NewDbFromConfig(
		&Config{Type: "sqlite", Database: ":memory:test-upsert-streamed", Options: defaultOptions},
		logs.GetChildLogger("database"),
		RetryConnectorCallbacks{},
	)
	if err != nil {
		t.Fatalf("cannot configure database: %v", err)
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.F(func(data UpsertStreamedTestData) ([]MockEntity, error) {
			ctx, cancel := context.WithCancel(ctx)

			go func() {
				if tst.Data.Statement != nil {
					upsertError = UpsertStreamed(ctx, db, entities, WithUpsertStatement(tst.Data.Statement))
				} else {
					upsertError = UpsertStreamed(ctx, db, entities)
				}
			}()

			initTestDb(db, logs.GetChildLogger("initTestDb"))

			for _, entity := range tst.Data.Entities {
				entities <- entity
			}

			var actual []MockEntity

			time.Sleep(time.Second)

			err = db.Select(&actual, "SELECT * FROM mock_entity")
			if err != nil {
				t.Fatalf("cannot select from database: %v", err)
			}

			cancel()

			return actual, upsertError
		}))

	}

	_ = db.Close()
}

//func TestUpsertStreamedCallback(t *testing.T) {
//	tests := []testutils.TestCase[any, UpsertStreamedTestData]{
//		{
//			Name: "OneCallback",
//			Data: UpsertStreamedTestData{
//				Callbacks: []OnSuccess[MockEntity]{
//					func(ctx context.Context, affectedRows []MockEntity) error {
//
//					},
//				},
//			},
//		},
//	}
//}
