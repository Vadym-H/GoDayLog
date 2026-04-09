package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var databaseURL, migrationsPath, migrationsTable, direction string

	flag.StringVar(&databaseURL, "database-url", "", "postgres connection string (e.g. postgres://user:pass@localhost:5432/dbname?sslmode=disable)")
	flag.StringVar(&migrationsPath, "migrations-path", "", "path to migrations directory")
	flag.StringVar(&migrationsTable, "migrations-table", "schema_migrations", "name of migrations table")
	flag.StringVar(&direction, "direction", "up", "migration direction: up or down")
	flag.Parse()

	if databaseURL == "" {
		log.Fatal("-database-url is required")
	}
	if migrationsPath == "" {
		log.Fatal("-migrations-path is required")
	}

	connStr, err := buildDatabaseURL(databaseURL, migrationsTable)
	if err != nil {
		log.Fatalf("invalid -database-url: %v", err)
	}

	sourceURL, err := buildMigrationsSourceURL(migrationsPath)
	if err != nil {
		log.Fatalf("invalid -migrations-path: %v", err)
	}

	m, err := migrate.New(sourceURL, connStr)
	if err != nil {
		log.Fatalf("failed to create migrator: %v", err)
	}
	m.Log = &Log{verbose: true}

	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Printf("error closing migration source: %v", srcErr)
		}
		if dbErr != nil {
			log.Printf("error closing migration db: %v", dbErr)
		}
	}()

	switch direction {
	case "up":
		if err := m.Up(); err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				fmt.Println("no migrations to apply")
				return
			}
			log.Fatalf("migration up failed: %v", err)
		}
		fmt.Println("migrations applied successfully")

	case "down":
		if err := m.Steps(-1); err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				fmt.Println("no migrations to roll back")
				return
			}
			log.Fatalf("migration down failed: %v", err)
		}
		fmt.Println("last migration rolled back")

	case "drop":
		if err := m.Drop(); err != nil {
			log.Fatalf("migration drop failed: %v", err)
		}
		fmt.Println("all migrations dropped")

	default:
		log.Fatalf("unknown direction %q - use 'up', 'down', or 'drop'", direction)
	}
}

func buildDatabaseURL(rawURL, migrationsTable string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("x-migrations-table", migrationsTable)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func buildMigrationsSourceURL(migrationsPath string) (string, error) {
	absPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		return "", err
	}

	path := filepath.ToSlash(absPath)
	if filepath.VolumeName(absPath) != "" {
		// golang-migrate's file source expects Windows drive paths as file://C:/...
		return "file://" + strings.TrimPrefix(path, "/"), nil
	}
	if strings.HasPrefix(path, "/") {
		return "file://" + path, nil
	}

	return "file:///" + path, nil
}

// Log is a logger for golang-migrate.
type Log struct {
	verbose bool
}

// Printf prints formatted output from the migrator.
func (l *Log) Printf(format string, v ...interface{}) {
	fmt.Printf(format, v...)
}

// Verbose returns whether verbose logging is enabled.
func (l *Log) Verbose() bool {
	return l.verbose
}
