package main

import (
	"errors"
	"flag"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var storagePath, migrationsPath, migrationsTable string
	var down bool

	flag.StringVar(&storagePath, "storage-path", "", "storage path")
	flag.StringVar(&migrationsPath, "migrations-path", "", "migrations path")
	flag.StringVar(&migrationsTable, "migrations-table", "migrations", "migrations table name")
	flag.BoolVar(&down, "down", false, "run down migrations")

	flag.Parse()

	if storagePath == "" {
		panic("storage path is empty")
	}

	if migrationsPath == "" {
		panic("migrations path is empty")
	}

	m, err := migrate.New(
		"file://"+migrationsPath,
		fmt.Sprintf("sqlite3://%s?x-migrations-table=%s", storagePath, migrationsTable),
	)
	if err != nil {
		panic(err)
	}

	defer m.Close()

	if down {
		err = m.Down()
	} else {
		err = m.Up()
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		panic(err)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("no migrations to apply")
		return
	}

	direction := "up"
	if down {
		direction = "down"
	}
	fmt.Printf("migrations applied (%s)\n", direction)
}
