package database

import (
	"context"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const (
	maxUsernameLength     = 255
	usernameAllocationKey = int64(180226289805618)
)

func allocateUniqueUsername(ctx context.Context, transaction pgx.Tx, requested string) (string, error) {
	if err := lockUsernameAllocation(ctx, transaction); err != nil {
		return "", err
	}
	return allocateUniqueUsernameLocked(ctx, transaction, requested)
}

func allocateUniqueUsernameLocked(ctx context.Context, transaction pgx.Tx, requested string) (string, error) {
	for suffix := 0; ; suffix++ {
		candidate := suffixedUsername(requested, suffix)
		var exists bool
		if err := transaction.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM users WHERE LOWER(username) = LOWER($1)
			)
		`, candidate).Scan(&exists); err != nil {
			return "", fmt.Errorf("check username availability: %w", err)
		}
		if !exists {
			return candidate, nil
		}
	}
}

func lockUsernameAllocation(ctx context.Context, transaction pgx.Tx) error {
	if _, err := transaction.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, usernameAllocationKey); err != nil {
		return fmt.Errorf("lock username allocation: %w", err)
	}
	return nil
}

func suffixedUsername(username string, suffix int) string {
	if suffix == 0 {
		return truncateRunes(username, maxUsernameLength)
	}
	suffixText := strconv.Itoa(suffix)
	return truncateRunes(username, maxUsernameLength-utf8.RuneCountInString(suffixText)) + suffixText
}

func truncateRunes(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum])
}
