package repository

import (
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"meetroom/internal/model"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// newTestDB connects to a real PostgreSQL instance for integration tests. It
// skips the test when no database is reachable so `go test ./...` still works
// in environments without Postgres. The DSN comes from the DATABASE_URL env var
// (or the repo-root .env file); the default matches docker-compose.yml.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	_ = godotenv.Load("../../.env") // repo root (tests run from the package dir)
	_ = godotenv.Load()             // package-local .env, if any

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=meetroom password=meetroom dbname=meetroom port=5433 sslmode=disable TimeZone=UTC"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("skipping integration test: cannot connect to database %q (set DATABASE_URL or run `docker compose up -d db`): %v", dsn, err)
	}
	return db
}

func setupSchema(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.AutoMigrate(&model.Room{}, &model.Availability{}, &model.Booking{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS btree_gist").Error; err != nil {
		t.Fatalf("create btree_gist extension: %v", err)
	}
	// The exclusion constraint is the actual safety net for the concurrency
	// test, so the test must fail loudly if it cannot be created instead of
	// silently relying on the (race-unsafe) in-transaction count check.
	if err := db.Exec(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'no_overlap_bookings'
			) THEN
				ALTER TABLE bookings ADD CONSTRAINT no_overlap_bookings
				EXCLUDE USING gist (
					room_id WITH =,
					tstzrange(start_time, end_time) WITH &&
				);
			END IF;
		END$$;
	`).Error; err != nil {
		t.Fatalf("create exclusion constraint: %v", err)
	}
	if err := db.Exec("TRUNCATE bookings, availabilities, rooms RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// TestCreateWithOverlapCheck_Concurrent verifies that concurrent bookings for
// the same time window never both succeed: the exclusion constraint is the
// final arbiter and its violation surfaces as ErrOverlap.
func TestCreateWithOverlapCheck_Concurrent(t *testing.T) {
	db := newTestDB(t)
	setupSchema(t, db)

	room := model.Room{Name: "concurrent-room", Capacity: 10}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}

	repo := NewBookingRepository(db)

	start := time.Now().UTC().Truncate(time.Second)
	end := start.Add(30 * time.Minute)

	const workers = 20
	var wg sync.WaitGroup
	errs := make([]error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = repo.CreateWithOverlapCheck(&model.Booking{
				RoomID:    room.ID,
				StartTime: start,
				EndTime:   end,
			})
		}(i)
	}
	wg.Wait()

	var count int64
	if err := db.Model(&model.Booking{}).Where("room_id = ?", room.ID).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 persisted booking, got %d", count)
	}

	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, ErrOverlap) {
			t.Errorf("unexpected error type: %v", err)
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 success, got %d", successes)
	}
}

// TestCreateWithOverlapCheck_BackToBack verifies that a booking ending exactly
// when another starts is allowed (half-open intervals).
func TestCreateWithOverlapCheck_BackToBack(t *testing.T) {
	db := newTestDB(t)
	setupSchema(t, db)

	room := model.Room{Name: "back-to-back-room", Capacity: 10}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}

	repo := NewBookingRepository(db)
	start := time.Now().UTC().Truncate(time.Second)

	first := &model.Booking{RoomID: room.ID, StartTime: start, EndTime: start.Add(1 * time.Hour)}
	if err := repo.CreateWithOverlapCheck(first); err != nil {
		t.Fatalf("first booking: %v", err)
	}

	second := &model.Booking{RoomID: room.ID, StartTime: start.Add(1 * time.Hour), EndTime: start.Add(2 * time.Hour)}
	if err := repo.CreateWithOverlapCheck(second); err != nil {
		t.Fatalf("back-to-back booking should be allowed, got: %v", err)
	}
}

// TestCreateWithOverlapCheck_Overlap verifies that a genuinely overlapping
// booking is rejected with ErrOverlap.
func TestCreateWithOverlapCheck_Overlap(t *testing.T) {
	db := newTestDB(t)
	setupSchema(t, db)

	room := model.Room{Name: "overlap-room", Capacity: 10}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}

	repo := NewBookingRepository(db)
	start := time.Now().UTC().Truncate(time.Second)

	first := &model.Booking{RoomID: room.ID, StartTime: start, EndTime: start.Add(2 * time.Hour)}
	if err := repo.CreateWithOverlapCheck(first); err != nil {
		t.Fatalf("first booking: %v", err)
	}

	overlap := &model.Booking{RoomID: room.ID, StartTime: start.Add(1 * time.Hour), EndTime: start.Add(3 * time.Hour)}
	if err := repo.CreateWithOverlapCheck(overlap); !errors.Is(err, ErrOverlap) {
		t.Fatalf("expected ErrOverlap, got: %v", err)
	}
}
