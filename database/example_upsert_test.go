package database

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/com"
	"golang.org/x/sync/errgroup"
	"time"
)

func ExampleUpsertStreamed() {
	var (
		testEntites = []User{
			{Id: 1, Name: "test1", Age: 10, Email: "test1@test.com"},
			{Id: 2, Name: "test2", Age: 20, Email: "test2@test.com"},
		}
		testSelect = &[]User{}
		g, ctx     = errgroup.WithContext(context.Background())
		entities   = make(chan User, len(testEntites))
		logs       = getTestLogging()
		db         = getTestDb(logs)
		log        = logs.GetLogger()
		err        error
	)
	initTestDb(db)

	g.Go(func() error {
		return UpsertStreamed(ctx, db, entities)
	})

	for _, entity := range testEntites {
		entities <- entity
	}

	close(entities)
	time.Sleep(10 * time.Millisecond)

	if err = db.Select(testSelect, "SELECT * FROM user"); err != nil {
		log.Fatalf("cannot select from db: %v", err)
	}

	fmt.Println(*testSelect)

	if err = g.Wait(); err != nil {
		log.Fatalf("error while upserting entities: %v", err)
	}

	_ = db.Close()

	// Output:
	// [{1 test1 10 test1@test.com} {2 test2 20 test2@test.com}]
}

func ExampleUpsertStreamedWithStatement() {
	var (
		testEntites = []User{
			{Id: 1, Name: "test1"},
			{Id: 2, Name: "test2"},
		}
		testSelect = &[]User{}
		g, ctx     = errgroup.WithContext(context.Background())
		entities   = make(chan User, len(testEntites))
		logs       = getTestLogging()
		db         = getTestDb(logs)
		log        = logs.GetLogger()
		stmt       = NewUpsertStatement(User{}).SetColumns("id", "name")
		err        error
	)
	initTestDb(db)

	g.Go(func() error {
		return UpsertStreamed(ctx, db, entities, WithUpsertStatement(stmt))
	})

	for _, entity := range testEntites {
		entities <- entity
	}

	close(entities)
	time.Sleep(10 * time.Millisecond)

	if err = db.Select(testSelect, "SELECT * FROM user"); err != nil {
		log.Fatalf("cannot select from db: %v", err)
	}

	fmt.Println(*testSelect)

	if err = g.Wait(); err != nil {
		log.Fatalf("error while upserting entities: %v", err)
	}

	_ = db.Close()

	// Output:
	// [{1 test1 0 } {2 test2 0 }]
}

func ExampleUpsertStreamedWithOnUpsert() {
	var (
		testEntites = []User{
			{Id: 1, Name: "test1", Age: 10, Email: "test1@test.com"},
			{Id: 2, Name: "test2", Age: 20, Email: "test2@test.com"},
		}
		callback = func(ctx context.Context, affectedRows []any) (err error) {
			fmt.Printf("number of affected rows: %d\n", len(affectedRows))
			return nil
		}
		testSelect = &[]User{}
		g, ctx     = errgroup.WithContext(context.Background())
		entities   = make(chan User, len(testEntites))
		logs       = getTestLogging()
		db         = getTestDb(logs)
		log        = logs.GetLogger()
		err        error
	)
	initTestDb(db)

	g.Go(func() error {
		return UpsertStreamed(ctx, db, entities, WithOnUpsert(callback))
	})

	for _, entity := range testEntites {
		entities <- entity
	}

	time.Sleep(1 * time.Second)
	close(entities)

	if err = db.Select(testSelect, "SELECT * FROM user"); err != nil {
		log.Fatalf("cannot select from db: %v", err)
	}

	fmt.Println(*testSelect)

	if err = g.Wait(); err != nil {
		log.Fatalf("error while upserting entities: %v", err)
	}

	_ = db.Close()

	// Output:
	// number of affected rows: 2
	// [{1 test1 10 test1@test.com} {2 test2 20 test2@test.com}]
}

func ExampleNamedBulkUpsert() {
	var (
		testEntites = []User{
			{Id: 1, Name: "test1", Age: 10, Email: "test1@test.com"},
			{Id: 2, Name: "test2", Age: 20, Email: "test2@test.com"},
		}
		testSelect = &[]User{}
		g, ctx     = errgroup.WithContext(context.Background())
		entities   = make(chan Entity, len(testEntites))
		logs       = getTestLogging()
		db         = getTestDb(logs)
		log        = logs.GetLogger()
		sem        = db.GetSemaphoreForTable(TableName(User{}))
		err        error
	)
	initTestDb(db)

	stmt, placeholders, err := db.QueryBuilder().UpsertStatement(NewUpsertStatement(User{}))
	if err != nil {
		log.Fatalf("error while building upsert statement: %v", err)
	}

	g.Go(func() error {
		return db.NamedBulkExec(ctx, stmt, placeholders, sem, entities, com.NeverSplit)
	})

	for _, entity := range testEntites {
		entities <- entity
	}

	time.Sleep(1 * time.Second)
	close(entities)

	if err = db.Select(testSelect, "SELECT * FROM user"); err != nil {
		log.Fatalf("cannot select from db: %v", err)
	}

	fmt.Println(*testSelect)

	if err = g.Wait(); err != nil {
		log.Fatalf("error while upserting entities: %v", err)
	}

	_ = db.Close()

	// Output:
	// [{1 test1 10 test1@test.com} {2 test2 20 test2@test.com}]
}

func ExampleNamedBulkUpsertWithOnUpsert() {
	var (
		testEntites = []User{
			{Id: 1, Name: "test1", Age: 10, Email: "test1@test.com"},
			{Id: 2, Name: "test2", Age: 20, Email: "test2@test.com"},
		}
		testSelect = &[]User{}
		callback   = func(ctx context.Context, affectedRows []Entity) (err error) {
			fmt.Printf("number of affected rows: %d\n", len(affectedRows))
			return nil
		}
		g, ctx   = errgroup.WithContext(context.Background())
		entities = make(chan Entity, len(testEntites))
		logs     = getTestLogging()
		db       = getTestDb(logs)
		log      = logs.GetLogger()
		sem      = db.GetSemaphoreForTable(TableName(User{}))
		err      error
	)
	initTestDb(db)

	stmt, placeholders, err := db.QueryBuilder().UpsertStatement(NewUpsertStatement(User{}))
	if err != nil {
		log.Fatalf("error while building upsert statement: %v", err)
	}

	g.Go(func() error {
		return db.NamedBulkExec(ctx, stmt, placeholders, sem, entities, com.NeverSplit, callback)
	})

	for _, entity := range testEntites {
		entities <- entity
	}

	time.Sleep(1 * time.Second)
	close(entities)

	if err = db.Select(testSelect, "SELECT * FROM user"); err != nil {
		log.Fatalf("cannot select from db: %v", err)
	}

	fmt.Println(*testSelect)

	if err = g.Wait(); err != nil {
		log.Fatalf("error while upserting entities: %v", err)
	}

	_ = db.Close()

	// Output:
	// number of affected rows: 2
	// [{1 test1 10 test1@test.com} {2 test2 20 test2@test.com}]
}

func ExampleNamedExecUpsert() {
	var (
		testEntites = []User{
			{Id: 1, Name: "test1", Age: 10, Email: "test1@test.com"},
			{Id: 2, Name: "test2", Age: 20, Email: "test2@test.com"},
		}
		testSelect = &[]User{}
		ctx        = context.Background()
		logs       = getTestLogging()
		db         = getTestDb(logs)
		log        = logs.GetLogger()
		err        error
	)
	initTestDb(db)

	stmt, _, err := db.QueryBuilder().UpsertStatement(NewUpsertStatement(User{}))
	if err != nil {
		log.Fatalf("error while building upsert statement: %v", err)
	}

	for _, entity := range testEntites {
		if _, err = db.NamedExecContext(ctx, stmt, entity); err != nil {
			log.Fatalf("error while upserting entity: %v", err)
		}
	}

	if err = db.Select(testSelect, "SELECT * FROM user"); err != nil {
		log.Fatalf("cannot select from db: %v", err)
	}

	fmt.Println(*testSelect)

	_ = db.Close()

	// Output:
	// [{1 test1 10 test1@test.com} {2 test2 20 test2@test.com}]
}

func ExampleExecUpsert() {
	var (
		testEntites = [][]any{
			{1, "test1", 10, "test1@test.com"},
			{2, "test2", 20, "test2@test.com"},
		}
		testSelect = &[]User{}
		stmt       = `INSERT INTO user ("id", "name", "age", "email") VALUES (?, ?, ?, ?) ON CONFLICT DO UPDATE SET "name" = EXCLUDED."name", "age" = EXCLUDED."age", "email" = EXCLUDED."email"`
		ctx        = context.Background()
		logs       = getTestLogging()
		db         = getTestDb(logs)
		log        = logs.GetLogger()
		err        error
	)
	initTestDb(db)

	//stmt, _, err := db.QueryBuilder().UpsertStatement(NewUpsertStatement(User{}))
	//if err != nil {
	//	log.Fatalf("error while building upsert statement: %v", err)
	//}

	for _, entity := range testEntites {
		if _, err = db.ExecContext(ctx, stmt, entity...); err != nil {
			log.Fatalf("error while upserting entity: %v", err)
		}
	}

	if err = db.Select(testSelect, "SELECT * FROM user"); err != nil {
		log.Fatalf("cannot select from db: %v", err)
	}

	fmt.Println(*testSelect)

	_ = db.Close()

	// Output:
	// [{1 test1 10 test1@test.com} {2 test2 20 test2@test.com}]
}
