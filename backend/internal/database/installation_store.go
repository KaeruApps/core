package database

import (
	"context"
	"fmt"

	"github.com/KaeruApps/core/internal/installation"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InstallationStore struct {
	pool *pgxpool.Pool
}

func NewInstallationStore(pool *pgxpool.Pool) *InstallationStore {
	return &InstallationStore{pool: pool}
}

func (store *InstallationStore) State(ctx context.Context) (installation.State, error) {
	var value string
	if err := store.pool.QueryRow(ctx, `
		SELECT setup_state
		FROM installation_settings
		WHERE singleton = TRUE
	`).Scan(&value); err != nil {
		return "", fmt.Errorf("read installation state: %w", err)
	}

	state, err := installation.ParseState(value)
	if err != nil {
		return "", fmt.Errorf("read installation state: %w", err)
	}
	return state, nil
}
