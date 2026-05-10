package worker

import (
	"context"
	"log/slog"
	"time"

	"booking-service/internal/service"
)

// Worker — фоновый процесс для периодических задач.
// На проде запускается отдельным бинарником (cmd/worker) или goroutine в api.
// Покрывает: goroutines, tickers, graceful shutdown, context cancellation.
type Worker struct {
	service  *service.BookingService
	interval time.Duration
	logger   *slog.Logger
}

func NewWorker(
	svc *service.BookingService,
	interval time.Duration,
	logger *slog.Logger,
) *Worker {
	return &Worker{
		service:  svc,
		interval: interval,
		logger:   logger,
	}
}

// Start запускает worker. Блокирует до отмены контекста.
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("worker started", "interval", w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Запускаем сразу при старте
	w.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker stopped")
			return ctx.Err()

		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	count, err := w.service.ExpireOldBookings(ctx, 100)
	if err != nil {
		w.logger.Error("worker tick failed", "error", err)
		return
	}

	if count > 0 {
		w.logger.Info("worker tick completed", "expired_count", count)
	}
}
