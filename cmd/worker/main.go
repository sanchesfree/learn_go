package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"booking-service/internal/config"
	"booking-service/internal/repository"
	"booking-service/internal/service"
	"booking-service/internal/worker"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// DB
	db, err := sqlx.Connect("postgres", cfg.DSN())
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)

	// Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer redisClient.Close()

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}

	// DI
	roomRepo := repository.NewRoomRepository(db)
	bookingRepo := repository.NewBookingRepository(db)
	cacheRepo := repository.NewCacheRepository(redisClient)
	bookingService := service.NewBookingService(roomRepo, bookingRepo, cacheRepo, logger)

	// Worker
	w := worker.NewWorker(bookingService, cfg.WorkerInterval, logger)

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logger.Info("worker shutdown signal received")
		cancel()
	}()

	if err := w.Start(ctx); err != nil {
		logger.Error("worker exited", "error", err)
	}
}
