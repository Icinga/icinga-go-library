package database

import (
	"context"
	"github.com/icinga/icinga-go-library/testutils"
	"testing"
	"time"
)

type UpsertStreamedTestData struct {
	Entities  []User
	Statement UpsertStatement
	Callbacks []OnSuccess[User]
}

func TestUpsertStreamed(t *testing.T) {
	tests := []testutils.TestCase[[]User, UpsertStreamedTestData]{
		{
			Name: "Insert",
			Expected: []User{
				{Id: 1, Name: "Alice Johnson", Age: 25, Email: "alice.johnson@example.com"},
				{Id: 2, Name: "Bob Smith", Age: 30, Email: "bob.smith@example.com"},
				{Id: 3, Name: "Charlie Brown", Age: 22, Email: "charlie.brown@example.com"},
				{Id: 4, Name: "Diana Prince", Age: 28, Email: "diana.prince@example.com"},
				{Id: 5, Name: "Evan Davis", Age: 35, Email: "evan.davis@example.com"},
				{Id: 6, Name: "Fiona White", Age: 27, Email: "fiona.white@example.com"},
				{Id: 7, Name: "George King", Age: 29, Email: "george.king@example.com"},
				{Id: 8, Name: "Hannah Moore", Age: 31, Email: "hannah.moore@example.com"},
			},
			Data: UpsertStreamedTestData{
				Entities: []User{
					{Id: 5, Name: "Evan Davis", Age: 35, Email: "evan.davis@example.com"},
					{Id: 6, Name: "Fiona White", Age: 27, Email: "fiona.white@example.com"},
					{Id: 7, Name: "George King", Age: 29, Email: "george.king@example.com"},
					{Id: 8, Name: "Hannah Moore", Age: 31, Email: "hannah.moore@example.com"},
				},
			},
		},
		{
			Name: "Update",
			Expected: []User{
				{Id: 1, Name: "Alice Johnson", Age: 25, Email: "alice.johnson@example.com"},
				{Id: 2, Name: "Bob Smith", Age: 30, Email: "bob.smith@example.com"},
				{Id: 3, Name: "Evan Davis", Age: 35, Email: "evan.davis@example.com"},
				{Id: 4, Name: "Fiona White", Age: 27, Email: "fiona.white@example.com"},
			},
			Data: UpsertStreamedTestData{
				Entities: []User{
					{Id: 3, Name: "Evan Davis", Age: 35, Email: "evan.davis@example.com"},
					{Id: 4, Name: "Fiona White", Age: 27, Email: "fiona.white@example.com"},
				},
			},
		},
		{
			Name: "InsertAndUpdate",
			Expected: []User{
				{Id: 1, Name: "Alice Johnson", Age: 25, Email: "alice.johnson@example.com"},
				{Id: 2, Name: "Bob Smith", Age: 30, Email: "bob.smith@example.com"},
				{Id: 3, Name: "Charlie Brown", Age: 22, Email: "charlie.brown@example.com"},
				{Id: 4, Name: "George King", Age: 29, Email: "george.king@example.com"},
				{Id: 5, Name: "Hannah Moore", Age: 31, Email: "hannah.moore@example.com"},
				{Id: 6, Name: "Fiona White", Age: 27, Email: "fiona.white@example.com"},
			},
			Data: UpsertStreamedTestData{
				Entities: []User{
					{Id: 5, Name: "Evan Davis", Age: 35, Email: "evan.davis@example.com"},
					{Id: 6, Name: "Fiona White", Age: 27, Email: "fiona.white@example.com"},
					{Id: 4, Name: "George King", Age: 29, Email: "george.king@example.com"},
					{Id: 5, Name: "Hannah Moore", Age: 31, Email: "hannah.moore@example.com"},
				},
			},
		},
		{
			Name: "WithStatement",
			Expected: []User{
				{Id: 1, Name: "Alice Johnson", Age: 25, Email: "alice.johnson@example.com"},
				{Id: 2, Name: "Bob Smith", Age: 30, Email: "bob.smith@example.com"},
				{Id: 3, Name: "Charlie Brown", Age: 22, Email: "charlie.brown@example.com"},
				{Id: 4, Name: "Diana Prince", Age: 28, Email: "diana.prince@example.com"},
				{Id: 5, Name: "Evan Davis", Age: 35, Email: "evan.davis@example.com"},
				{Id: 6, Name: "Fiona White", Age: 27, Email: "fiona.white@example.com"},
			},
			Data: UpsertStreamedTestData{
				Entities: []User{
					{Id: 5, Name: "Evan Davis", Age: 35, Email: "evan.davis@example.com"},
					{Id: 6, Name: "Fiona White", Age: 27, Email: "fiona.white@example.com"},
				},
				Statement: NewUpsertStatement(&User{}),
			},
		},
		{
			Name:  "WithFalseStatement",
			Error: testutils.ErrorContains("can't perform"), // TODO (jr): is it the right way?
			Data: UpsertStreamedTestData{
				Entities: []User{
					{Id: 5, Name: "test5", Age: 50, Email: "test5@test.com"},
				},
				Statement: NewUpsertStatement(&User{}).Into("false_table"),
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.Name, tst.F(func(data UpsertStreamedTestData) ([]User, error) {
			var (
				upsertError error
				ctx, cancel = context.WithCancel(context.Background())
				entities    = make(chan User)
				logs        = getTestLogging()
				db          = getTestDb(logs)
			)

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

			var actual []User

			time.Sleep(time.Second)

			err := db.Select(&actual, "SELECT * FROM user")
			if err != nil {
				t.Fatalf("cannot select from database: %v", err)
			}

			cancel()
			_ = db.Close()

			return actual, upsertError
		}))

	}
}

// TODO (jr)
//func TestUpsertStreamedCallback(t *testing.T) {
//	tests := []testutils.TestCase[any, UpsertStreamedTestData]{
//		{
//			Name: "OneCallback",
//			Data: UpsertStreamedTestData{
//				Callbacks: []OnSuccess[User]{
//					func(ctx context.Context, affectedRows []User) error {
//
//					},
//				},
//			},
//		},
//	}
//}

// TODO (jr)
// func TestUpsertStreamedEarlyDbClose(t *testing.T) {
//
// }
