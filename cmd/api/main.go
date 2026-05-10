package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"booking-service/internal/config"
	"booking-service/internal/handler"
	"booking-service/internal/repository"
	"booking-service/internal/service"
	"booking-service/internal/worker"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	// ============================================
	// 1. Инициализация логгера (slog — stdlib с Go 1.21)
	// ============================================
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// ============================================
	// 2. Загрузка конфигурации
	// ============================================
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger.Info("config loaded",
		"db_host", cfg.DBHost,
		"redis_addr", cfg.RedisAddr,
	)

	// ============================================
	// 3. Подключение к PostgreSQL
	// ============================================
	db, err := sqlx.Connect("postgres", cfg.DSN())
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Настройки пула соединений — ВАЖНО для продакшена
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	logger.Info("connected to database")

	// ============================================
	// 4. Подключение к Redis
	// ============================================
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer redisClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to redis")

	// ============================================
	// 5. Миграция БД
	// ============================================
	if err := runMigrations(db); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("migrations completed")

	// ============================================
	// 6. Инициализация слоёв (Dependency Injection)
	// ============================================
	// В Go нет DI-фреймворков (вроде Spring), всё собирается вручную.
	// Это нормально и считается best practice.

	roomRepo := repository.NewRoomRepository(db)
	bookingRepo := repository.NewBookingRepository(db)
	cacheRepo := repository.NewCacheRepository(redisClient)

	bookingService := service.NewBookingService(roomRepo, bookingRepo, cacheRepo, logger)
	bookingHandler := handler.NewBookingHandler(bookingService, logger)

	// ============================================
	// 7. Запуск HTTP сервера
	// ============================================
	addr := fmt.Sprintf("%s:%d", cfg.HTTPAddr, cfg.HTTPPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      bookingHandler.Router(),
		ReadTimeout:  10 * time.Second,  // максимальное время чтения запроса
		WriteTimeout: 30 * time.Second,  // максимальное время записи ответа
		IdleTimeout:  120 * time.Second, // keep-alive
	}

	// ============================================
	// 8. Запуск background worker
	// ============================================
	workerInstance := worker.NewWorker(bookingService, cfg.WorkerInterval, logger)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	go func() {
		if err := workerInstance.Start(workerCtx); err != nil {
			logger.Error("worker stopped", "error", err)
		}
	}()

	// ============================================
	// 9. Graceful shutdown — КРИТИЧЕСКИ ВАЖНО
	// ============================================
	// Без этого: при деплое теряются запросы в полёте,
	// соединения не закрываются, файлы не flushed.

	go func() {
		logger.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Ждём сигнала SIGINT (Ctrl+C) или SIGTERM (docker stop, k8s)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("shutdown signal received", "signal", sig.String())

	// Даём 30 секунд на завершение текущих запросов
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Останавливаем worker
	workerCancel()

	// Graceful shutdown HTTP сервера
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced shutdown", "error", err)
	}

	logger.Info("server stopped gracefully")
}

// runMigrations — простейший раннер миграций.
// На проде используют golang-migrate, goose, или atlas.
func runMigrations(db *sqlx.DB) error {
	// Читаем SQL-файл миграции
	migrationSQL := `
	CREATE EXTENSION IF NOT EXISTS "btree_gist";

	CREATE TABLE IF NOT EXISTS rooms (
		id          BIGSERIAL PRIMARY KEY,
		name        VARCHAR(100) NOT NULL,
		capacity    INT NOT NULL CHECK (capacity > 0),
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS bookings (
		id          BIGSERIAL PRIMARY KEY,
		room_id     BIGINT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
		user_id     VARCHAR(100) NOT NULL,
		title       VARCHAR(200) NOT NULL,
		start_time  TIMESTAMPTZ NOT NULL,
		end_time    TIMESTAMPTZ NOT NULL,
		status      VARCHAR(20) NOT NULL DEFAULT 'active',
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		EXCLUDE USING gist (
			room_id WITH =,
			tstzrange(start_time, end_time, '[)') WITH &&
		) WHERE (status = 'active')
	);

	CREATE INDEX IF NOT EXISTS idx_bookings_room_id ON bookings(room_id);
	CREATE INDEX IF NOT EXISTS idx_bookings_user_id ON bookings(user_id);
	CREATE INDEX IF NOT EXISTS idx_bookings_start_time ON bookings(start_time);
	`

	_, err := db.Exec(migrationSQL)
	return err
}
