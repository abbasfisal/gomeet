package repository

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestTranslateDBError_ExclusionViolation(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23P01"}
	if !errors.Is(TranslateDBError(pgErr), ErrOverlap) {
		t.Error("exclusion violation (23P01) should map to ErrOverlap")
	}
}

func TestTranslateDBError_UniqueViolation(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505"}
	if !errors.Is(TranslateDBError(pgErr), ErrConflict) {
		t.Error("unique violation (23505) should map to ErrConflict")
	}
}

func TestTranslateDBError_Passthrough(t *testing.T) {
	plain := errors.New("some other error")
	if TranslateDBError(plain) != plain {
		t.Error("unrecognized errors should pass through unchanged")
	}
	if TranslateDBError(nil) != nil {
		t.Error("nil should stay nil")
	}
}

func TestAsInternal(t *testing.T) {
	err := AsInternal(errors.New("boom"))
	if !errors.Is(err, ErrInternal) {
		t.Error("AsInternal should wrap ErrInternal")
	}
}
