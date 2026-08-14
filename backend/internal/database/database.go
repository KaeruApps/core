package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if err := runMigrations(ctx, databaseURL); err != nil {
		return nil, err
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Core database URL: %w", err)
	}
	config.ConnConfig.RuntimeParams["application_name"] = "kaeru-core"

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open Core database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping Core database: %w", err)
	}
	if err := restrictDatabaseConnections(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

func runMigrations(ctx context.Context, databaseURL string) error {
	databaseConnection, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open Core database for migrations: %w", err)
	}
	defer databaseConnection.Close()

	migrations, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, databaseConnection, migrations)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply Core database migrations: %w", err)
	}

	return nil
}

func restrictDatabaseConnections(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, "SELECT datname FROM pg_database WHERE datallowconn ORDER BY datname")
	if err != nil {
		return fmt.Errorf("list PostgreSQL databases: %w", err)
	}
	databaseNames, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("read PostgreSQL database names: %w", err)
	}

	for _, databaseName := range databaseNames {
		statement := "REVOKE CONNECT ON DATABASE " + pgx.Identifier{databaseName}.Sanitize() + " FROM PUBLIC"
		if _, err := pool.Exec(ctx, statement); err != nil {
			return fmt.Errorf("restrict access to database %q: %w", databaseName, err)
		}
	}

	return nil
}
