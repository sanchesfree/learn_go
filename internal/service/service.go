package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	apperrors "booking-service/pkg/errors"

	"booking-service/internal/model"
	"booking-service/internal/repository"
)

// BookingService — бизнес-логика бронирования.
// Слой service изолирует HTTP-обработчики от деталей хранения.
type BookingService struct {
	rooms    *repository.RoomRepository
	bookings *repository.BookingRepository
	cache    *repository.CacheRepository
	logger   *slog.Logger
}

func NewBookingService(
	rooms *repository.RoomRepository,
	bookings *repository.BookingRepository,
	cache *repository.CacheRepository,
	logger *slog.Logger,
) *BookingService {
	return &BookingService{
		rooms:    rooms,
		bookings: bookings,
		cache:    cache,
		logger:   logger,
	}
}

// =====================
// Room operations
// =====================

func (s *BookingService) CreateRoom(ctx context.Context, room model.Room) (model.Room, error) {
	created, err := s.rooms.Create(ctx, room)
	if err != nil {
		s.logger.Error("room creation failed", "error", err)
		return model.Room{}, apperrors.Internal("failed to create room", err)
	}
	return created, nil
}

func (s *BookingService) GetRoom(ctx context.Context, id int64) (model.Room, error) {
	// Сначала пробуем кэш
	room, err := s.cache.GetRoom(ctx, id)
	if err == nil {
		return room, nil // cache hit
	}
	if err != repository.ErrCacheMiss {
		s.logger.Warn("cache error, falling back to db", "error", err)
	}

	// Cache miss → идём в БД
	room, err = s.rooms.GetByID(ctx, id)
	if err != nil {
		return model.Room{}, apperrors.NotFound(fmt.Sprintf("room %d not found", id), err)
	}

	// Пишем в кэш асинхронно (fire and forget)
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.cache.SetRoom(bgCtx, room); err != nil {
			s.logger.Warn("failed to cache room", "room_id", room.ID, "error", err)
		}
	}()

	return room, nil
}

func (s *BookingService) ListRooms(ctx context.Context, page, pageSize int) ([]model.Room, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	rooms, total, err := s.rooms.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, apperrors.Internal("failed to list rooms", err)
	}
	return rooms, total, nil
}

func (s *BookingService) DeleteRoom(ctx context.Context, id int64) error {
	err := s.rooms.Delete(ctx, id)
	if err != nil {
		return apperrors.NotFound(fmt.Sprintf("room %d not found", id), err)
	}
	// Инвалидируем кэш
	s.cache.InvalidateRoom(ctx, id)
	return nil
}

// =====================
// Booking operations
// =====================

// CreateBooking — создаёт бронирование.
// Это ключевая операция, где нужна забота о конкурентности:
// два запроса могут одновременно попробовать забронировать одно время.
func (s *BookingService) CreateBooking(ctx context.Context, req model.CreateBookingRequest, userID string) (model.Booking, error) {
	// 1. Валидация
	if err := validateBookingRequest(req); err != nil {
		return model.Booking{}, apperrors.BadRequest(err.Error(), nil)
	}

	// 2. Проверяем существование комнаты
	_, err := s.rooms.GetByID(ctx, req.RoomID)
	if err != nil {
		return model.Booking{}, apperrors.NotFound("room not found", err)
	}

	// 3. Берём distributed lock на комнату (на случай если EXCLUDE constraint не хватает)
	lockKey := fmt.Sprintf("room:%d:book", req.RoomID)
	locked, err := s.cache.AcquireLock(ctx, lockKey, 5*time.Second)
	if err != nil {
		s.logger.Warn("failed to acquire lock, relying on DB constraint", "error", err)
	}
	if locked {
		defer func() {
			if err := s.cache.ReleaseLock(context.Background(), lockKey); err != nil {
				s.logger.Warn("failed to release lock", "error", err)
			}
		}()
	}

	// 4. Проверяем конфликт времени (дублируем проверку БД на уровне приложения)
	conflict, err := s.bookings.HasConflict(ctx, req.RoomID, req.StartTime, req.EndTime)
	if err != nil {
		return model.Booking{}, apperrors.Internal("failed to check availability", err)
	}
	if conflict {
		return model.Booking{}, apperrors.Conflict(
			"this time slot is already booked",
			fmt.Errorf("room %d has conflict for %s - %s", req.RoomID, req.StartTime, req.EndTime),
		)
	}

	// 5. Создаём
	booking := model.Booking{
		RoomID:    req.RoomID,
		UserID:    userID,
		Title:     req.Title,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Status:    model.StatusActive,
	}

	created, err := s.bookings.Create(ctx, booking)
	if err != nil {
		if isConflictMsg(err) {
			return model.Booking{}, apperrors.Conflict("this time slot is already booked", err)
		}
		return model.Booking{}, apperrors.Internal("failed to create booking", err)
	}

	// 6. Инвалидируем кэш бронирований для этой комнаты
	s.cache.InvalidateBookingsByRoom(ctx, req.RoomID, req.StartTime)

	s.logger.Info("booking created",
		"booking_id", created.ID,
		"room_id", req.RoomID,
		"user_id", userID,
	)

	return created, nil
}

func (s *BookingService) GetBooking(ctx context.Context, id int64) (model.Booking, error) {
	booking, err := s.bookings.GetByID(ctx, id)
	if err != nil {
		return model.Booking{}, apperrors.NotFound(fmt.Sprintf("booking %d not found", id), err)
	}
	return booking, nil
}

// GetRoomSchedule — расписание переговорки на конкретный день
func (s *BookingService) GetRoomSchedule(ctx context.Context, roomID int64, date time.Time) ([]model.Booking, error) {
	// Проверяем что комната существует
	if _, err := s.rooms.GetByID(ctx, roomID); err != nil {
		return nil, apperrors.NotFound("room not found", err)
	}

	// Пробуем кэш
	bookings, err := s.cache.GetBookingsByRoom(ctx, roomID, date)
	if err == nil {
		return bookings, nil
	}
	if err != repository.ErrCacheMiss {
		s.logger.Warn("cache error for bookings", "error", err)
	}

	// Из БД
	bookings, err = s.bookings.ListByRoom(ctx, roomID, date)
	if err != nil {
		return nil, apperrors.Internal("failed to get schedule", err)
	}

	// Кэшируем
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.cache.SetBookingsByRoom(bgCtx, roomID, date, bookings); err != nil {
			s.logger.Warn("failed to cache bookings", "error", err)
		}
	}()

	return bookings, nil
}

func (s *BookingService) ListUserBookings(ctx context.Context, userID string, page, pageSize int) ([]model.Booking, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	bookings, total, err := s.bookings.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, apperrors.Internal("failed to list bookings", err)
	}
	return bookings, total, nil
}

func (s *BookingService) CancelBooking(ctx context.Context, id int64, userID string) error {
	err := s.bookings.Cancel(ctx, id, userID)
	if err != nil {
		return apperrors.NotFound("booking not found or cannot be cancelled", err)
	}

	s.logger.Info("booking cancelled", "booking_id", id, "user_id", userID)
	return nil
}

// ExpireOldBookings — помечает завершённые бронирования (вызывается воркером)
func (s *BookingService) ExpireOldBookings(ctx context.Context, limit int) (int, error) {
	expired, err := s.bookings.FindExpired(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("find expired: %w", err)
	}

	if len(expired) == 0 {
		return 0, nil
	}

	ids := make([]int64, len(expired))
	for i, b := range expired {
		ids[i] = b.ID
	}

	if err := s.bookings.MarkExpired(ctx, ids); err != nil {
		return 0, fmt.Errorf("mark expired: %w", err)
	}

	s.logger.Info("expired bookings processed", "count", len(ids))
	return len(ids), nil
}

// --- Validation ---

func validateBookingRequest(req model.CreateBookingRequest) error {
	if req.RoomID <= 0 {
		return fmt.Errorf("room_id must be positive")
	}
	if req.Title == "" {
		return fmt.Errorf("title is required")
	}
	if req.EndTime.Before(req.StartTime) {
		return fmt.Errorf("end_time must be after start_time")
	}
	if req.EndTime.Equal(req.StartTime) {
		return fmt.Errorf("end_time must be after start_time (zero duration not allowed)")
	}
	// Бронирование максимум на 24 часа
	if req.EndTime.Sub(req.StartTime) > 24*time.Hour {
		return fmt.Errorf("booking cannot exceed 24 hours")
	}
	return nil
}

func isConflictMsg(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "conflict")
}
