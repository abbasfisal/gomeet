package repository

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound = errors.New("record not found")
	ErrOverlap  = errors.New("booking overlaps with existing booking")
	ErrConflict = errors.New("conflict")
	ErrInternal = errors.New("internal error")
)

// TranslateDBError maps known PostgreSQL errors to domain errors so that
// callers can rely on errors.Is instead of inspecting driver-specific types.
func TranslateDBError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23P01": // exclusion_violation (double-booking race)
			return ErrOverlap
		case "23505": // unique_violation (e.g. duplicate room name)
			return ErrConflict
		}
	}
	return err
}

// AsInternal wraps an unexpected error so handlers can distinguish internal
// failures from client-side validation errors.
func AsInternal(err error) error {
	return fmt.Errorf("%w: %v", ErrInternal, err)
}
