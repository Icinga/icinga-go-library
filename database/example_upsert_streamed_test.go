package database

import (
	"context"
	"fmt"
	"github.com/creasty/defaults"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/utils"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
	"time"
)

func ExampleUpsertStreamed() {
	var (
		testEntites = []MockEntity{
			{Id: 1, Name: "test1", Age: 10, Email: "test1@test.com"},
			{Id: 2, Name: "test2", Age: 20, Email: "test2@test.com"},
		}
		g, ctx         = errgroup.WithContext(context.Background())
		entities       = make(chan MockEntity, len(testEntites))
		defaultOptions Options
	)

	logs, err := logging.NewLoggingFromConfig(
		"Icinga Go Library",
		logging.Config{Level: zapcore.DebugLevel, Output: "console", Interval: time.Second * 10},
	)
	if err != nil {
		utils.PrintErrorThenExit(err, 1)
	}

	log := logs.GetLogger()

	err = defaults.Set(&defaultOptions)
	if err != nil {
		log.Fatalf("cannot set default options: %v", err)
	}

	db, err := NewDbFromConfig(
		&Config{Type: "sqlite", Database: ":memory:example-upsert", Options: defaultOptions},
		logs.GetChildLogger("database"),
		RetryConnectorCallbacks{},
	)
	if err != nil {
		log.Fatalf("cannot create database: %v", err)
	}

	_, err = db.Query("CREATE TABLE IF NOT EXISTS mock_entity (id INTEGER PRIMARY KEY, name VARCHAR(255), age INTEGER, email VARCHAR(255))")
	if err != nil {
		log.Fatalf("cannot create table in db: %v", err)
	}

	g.Go(func() error {
		return UpsertStreamed(ctx, db, entities)
	})

	for _, entity := range testEntites {
		entities <- entity
	}

	close(entities)

	time.Sleep(time.Second)

	testSelect := &[]MockEntity{}

	err = db.Select(testSelect, "SELECT * FROM mock_entity")
	if err != nil {
		log.Fatalf("cannot select from db: %v", err)
	}

	fmt.Println(*testSelect)

	if err = g.Wait(); err != nil {
		log.Fatalf("upsert error: %v", err)
	}

	_ = db.Close()

	// Output:
	// [{1 test1 10 test1@test.com} {2 test2 20 test2@test.com}]
}
