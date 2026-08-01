package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"meetroom/internal/cache"
	"meetroom/internal/config"
	"meetroom/internal/handler"
	"meetroom/internal/model"
	"meetroom/internal/repository"
	"meetroom/internal/router"
	"meetroom/internal/service"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	godotenv.Load()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		slog.Error("failed to connect database", "error", err)
		os.Exit(1)
	}

	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("failed to get sql db", "error", err)
		os.Exit(1)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := db.AutoMigrate(&model.Room{}, &model.Availability{}, &model.Booking{}); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS btree_gist").Error; err != nil {
		slog.Error("failed to create btree_gist extension", "error", err)
		os.Exit(1)
	}
	// GORM stores time.Time as timestamptz, so the exclusion range must be
	// tstzrange (tsrange only accepts timestamp without time zone). This
	// constraint is the final safety net against double-booking, so startup
	// fails if it cannot be created.
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
		slog.Error("failed to create no_overlap_bookings exclusion constraint", "error", err)
		os.Exit(1)
	}

	cacheClient := cache.New(cfg.RedisURL, cfg.RedisPass, cfg.RedisDB)
	defer cacheClient.Close()

	ctx := context.Background()
	if err := cacheClient.Ping(ctx); err != nil {
		slog.Warn("redis not available", "error", err)
	}

	roomRepo := repository.NewRoomRepository(db)
	availRepo := repository.NewAvailabilityRepository(db)
	bookingRepo := repository.NewBookingRepository(db)

	roomSvc := service.NewRoomService(roomRepo, availRepo, bookingRepo, cacheClient)
	bookingSvc := service.NewBookingService(bookingRepo, roomRepo, availRepo, cacheClient)

	roomHandler := handler.NewRoomHandler(roomSvc)
	bookingHandler := handler.NewBookingHandler(bookingSvc)

	r := router.Setup(roomHandler, bookingHandler)

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	go func() {
		slog.Info("server starting", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server exited gracefully")
}
