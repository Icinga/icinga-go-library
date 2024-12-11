package database

import (
	"fmt"
	"github.com/creasty/defaults"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/utils"
	"go.uber.org/zap/zapcore"
	"math/rand"
	"strconv"
	"time"
)

type User struct {
	Id    Id
	Name  string
	Age   int
	Email string
}

type Id int

func (i Id) String() string {
	return strconv.Itoa(int(i))
}

func (m User) ID() ID {
	return m.Id
}

func (m User) SetID(id ID) {
	m.Id = id.(Id)
}

func (m User) Fingerprint() Fingerprinter {
	return m
}

func getTestLogging() *logging.Logging {
	logs, err := logging.NewLoggingFromConfig(
		"Icinga Go Library",
		logging.Config{Level: zapcore.DebugLevel, Output: "console", Interval: time.Second * 10},
	)
	if err != nil {
		utils.PrintErrorThenExit(err, 1)
	}

	return logs
}

func getTestDb(logs *logging.Logging) *DB {
	var defaultOptions Options

	err := defaults.Set(&defaultOptions)
	if err != nil {
		utils.PrintErrorThenExit(err, 1)
	}

	randomName := strconv.Itoa(rand.Int())

	db, err := NewDbFromConfig(
		&Config{Type: "sqlite", Database: fmt.Sprintf(":memory:%s", randomName), Options: defaultOptions},
		logs.GetChildLogger("database"),
		RetryConnectorCallbacks{},
	)
	if err != nil {
		utils.PrintErrorThenExit(err, 1)
	}

	return db
}

func initTestDb(db *DB) {
	_, err := db.Query("DROP TABLE IF EXISTS user")
	if err != nil {
		utils.PrintErrorThenExit(err, 1)
	}

	_, err = db.Query(`CREATE TABLE user ("id" INTEGER PRIMARY KEY, "name" VARCHAR(255) DEFAULT '', "age" INTEGER DEFAULT 0, "email" VARCHAR(255) DEFAULT '')`)
	if err != nil {
		utils.PrintErrorThenExit(err, 1)
	}
}

func prefillTestDb(db *DB) {
	entities := []User{
		{Id: 1, Name: "Alice Johnson", Age: 25, Email: "alice.johnson@example.com"},
		{Id: 2, Name: "Bob Smith", Age: 30, Email: "bob.smith@example.com"},
		{Id: 3, Name: "Charlie Brown", Age: 22, Email: "charlie.brown@example.com"},
		{Id: 4, Name: "Diana Prince", Age: 28, Email: "diana.prince@example.com"},
	}

	for _, entity := range entities {
		_, err := db.NamedExec(`INSERT INTO user ("id", "name", "age", "email") VALUES (:id, :name, :age, :email)`, entity)
		if err != nil {
			utils.PrintErrorThenExit(err, 1)
		}
	}
}
