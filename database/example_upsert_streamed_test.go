package database

import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"time"
)

func ExampleUpsertStreamed() {
	var (
		testEntites = []User{
			{Id: 1, Name: "test1", Age: 10, Email: "test1@test.com"},
			{Id: 2, Name: "test2", Age: 20, Email: "test2@test.com"},
		}
		g, ctx   = errgroup.WithContext(context.Background())
		entities = make(chan User, len(testEntites))
		logs     = getTestLogging()
		db       = getTestDb(logs)
		log      = logs.GetLogger()
		err      error
	)

	_, err = db.Query("CREATE TABLE IF NOT EXISTS user (id INTEGER PRIMARY KEY, name VARCHAR(255), age INTEGER, email VARCHAR(255))")
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

	testSelect := &[]User{}

	err = db.Select(testSelect, "SELECT * FROM user")
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
