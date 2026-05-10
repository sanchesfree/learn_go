package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"booking-service/internal/model"

	"github.com/jmoiron/sqlx"
)

// RoomRepository — работа с переговорками в PostgreSQL
type RoomRepository struct {
	db *sqlx.DB
}

func NewRoomRepository(db *sqlx.DB) *RoomRepository {
	return &RoomRepository{db: db}
}

func (r *RoomRepository) Create(ctx context.Context, room model.Room) (model.Room, error) {
	var result model.Room
	query := `
		INSERT INTO rooms (name, capacity) VALUES ($1, $2)
		RETURNING id, name, capacity, created_at`

	// QueryRowxContext — возвращает *Row, из которого можно Scan в структуру
	err := r.db.QueryRowxContext(ctx, query, room.Name, room.Capacity).StructScan(&result)
	if err != nil {
		return model.Room{}, fmt.Errorf("room create: %w", err)
	}
	return result, nil
}

func (r *RoomRepository) GetByID(ctx context.Context, id int64) (model.Room, error) {
	var room model.Room
	query := `SELECT id, name, capacity, created_at FROM rooms WHERE id = $1`

	err := r.db.GetContext(ctx, &room, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Room{}, fmt.Errorf("room %d not found: %w", id, err)
		}
		return model.Room{}, fmt.Errorf("room get by id: %w", err)
	}
	return room, nil
}

// List — пагинированный список переговорок
func (r *RoomRepository) List(ctx context.Context, page, pageSize int) ([]model.Room, int64, error) {
	rooms := make([]model.Room, 0)
	var total int64

	// Считаем total (всегда через COUNT, не len(result) — это разные вещи)
	err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM rooms`)
	if err != nil {
		return nil, 0, fmt.Errorf("room count: %w", err)
	}

	// OFFSET / LIMIT для пагинации
	// Важно: сортировка обязательна для стабильной пагинации
	offset := (page - 1) * pageSize
	query := `SELECT id, name, capacity, created_at FROM rooms ORDER BY id LIMIT $1 OFFSET $2`

	err = r.db.SelectContext(ctx, &rooms, query, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("room list: %w", err)
	}

	return rooms, total, nil
}

func (r *RoomRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM rooms WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("room delete: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("room %d not found", id)
	}
	return nil
}

// ============================================
// BookingRepository — работа с бронированиями
// ============================================

type BookingRepository struct {
	db *sqlx.DB
}

func NewBookingRepository(db *sqlx.DB) *BookingRepository {
	return &BookingRepository{db: db}
}

// Create — создаёт бронирование.
// EXCLUDE constraint в БД защищает от пересечений, но мы также проверяем
// на уровне приложения для дружественного сообщения об ошибке.
func (r *BookingRepository) Create(ctx context.Context, booking model.Booking) (model.Booking, error) {
	var result model.Booking
	query := `
		INSERT INTO bookings (room_id, user_id, title, start_time, end_time, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, room_id, user_id, title, start_time, end_time, status, created_at`

	err := r.db.QueryRowxContext(ctx, query,
		booking.RoomID, booking.UserID, booking.Title,
		booking.StartTime, booking.EndTime, booking.Status,
	).StructScan(&result)

	if err != nil {
		// PostgreSQL EXCLUDE violation → "conflicting key value violates exclusion constraint"
		if isConflictError(err) {
			return model.Booking{}, fmt.Errorf("time slot conflict: %w", err)
		}
		return model.Booking{}, fmt.Errorf("booking create: %w", err)
	}
	return result, nil
}

// GetByID — получает бронирование по ID
func (r *BookingRepository) GetByID(ctx context.Context, id int64) (model.Booking, error) {
	var booking model.Booking
	err := r.db.GetContext(ctx, &booking,
		`SELECT id, room_id, user_id, title, start_time, end_time, status, created_at
		 FROM bookings WHERE id = $1`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Booking{}, fmt.Errorf("booking %d not found: %w", id, err)
		}
		return model.Booking{}, fmt.Errorf("booking get: %w", err)
	}
	return booking, nil
}

// ListByRoom — список бронирований для комнаты на конкретный день
func (r *BookingRepository) ListByRoom(ctx context.Context, roomID int64, date time.Time) ([]model.Booking, error) {
	bookings := make([]model.Booking, 0)

	// Ищем бронирования, которые пересекаются с переданным днём
	query := `
		SELECT id, room_id, user_id, title, start_time, end_time, status, created_at
		FROM bookings
		WHERE room_id = $1
		  AND status = 'active'
		  AND tstzrange(start_time, end_time, '[)') && tstzrange($2, $3, '[)')
		ORDER BY start_time`

	// Начало и конец дня
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	err := r.db.SelectContext(ctx, &bookings, query, roomID, dayStart, dayEnd)
	if err != nil {
		return nil, fmt.Errorf("bookings by room: %w", err)
	}
	return bookings, nil
}

// ListByUser — бронирования пользователя
func (r *BookingRepository) ListByUser(ctx context.Context, userID string, page, pageSize int) ([]model.Booking, int64, error) {
	bookings := make([]model.Booking, 0)
	var total int64

	err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM bookings WHERE user_id = $1`, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("booking count by user: %w", err)
	}

	offset := (page - 1) * pageSize
	err = r.db.SelectContext(ctx, &bookings,
		`SELECT id, room_id, user_id, title, start_time, end_time, status, created_at
		 FROM bookings WHERE user_id = $1 ORDER BY start_time DESC LIMIT $2 OFFSET $3`,
		userID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("bookings by user: %w", err)
	}

	return bookings, total, nil
}

// Cancel — отменяет бронирование (мягкое удаление через смену статуса)
func (r *BookingRepository) Cancel(ctx context.Context, id int64, userID string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE bookings SET status = 'cancelled' WHERE id = $1 AND user_id = $2 AND status = 'active'`,
		id, userID)
	if err != nil {
		return fmt.Errorf("booking cancel: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("booking %d not found or not owned by user %s", id, userID)
	}
	return nil
}

// FindExpired — находит бронирования, которые уже закончились но ещё активны
// Используется воркером для фонового закрытия
func (r *BookingRepository) FindExpired(ctx context.Context, limit int) ([]model.Booking, error) {
	bookings := make([]model.Booking, 0)
	err := r.db.SelectContext(ctx, &bookings,
		`SELECT id, room_id, user_id, title, start_time, end_time, status, created_at
		 FROM bookings
		 WHERE status = 'active' AND end_time < $1
		 ORDER BY end_time ASC LIMIT $2`,
		time.Now(), limit)
	if err != nil {
		return nil, fmt.Errorf("find expired bookings: %w", err)
	}
	return bookings, nil
}

// MarkExpired — помечает просроченные бронирования как отменённые
func (r *BookingRepository) MarkExpired(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	// sqlx.In разворачивает []int64 в ($1, $2, $3, ...)
	query, args, err := sqlx.In(
		`UPDATE bookings SET status = 'expired' WHERE id IN (?) AND status = 'active'`,
		ids)
	if err != nil {
		return fmt.Errorf("mark expired: build query: %w", err)
	}

	// Rebind для PostgreSQL ($1, $2 вместо ?)
	query = r.db.Rebind(query)
	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark expired: exec: %w", err)
	}
	return nil
}

// HasConflict — проверяет, есть ли пересечение по времени для комнаты
func (r *BookingRepository) HasConflict(ctx context.Context, roomID int64, start, end time.Time) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1 FROM bookings
			WHERE room_id = $1
			  AND status = 'active'
			  AND tstzrange(start_time, end_time, '[)') && tstzrange($2, $3, '[)')
		)`

	err := r.db.GetContext(ctx, &exists, query, roomID, start, end)
	if err != nil {
		return false, fmt.Errorf("check conflict: %w", err)
	}
	return exists, nil
}

// --- Helpers ---

func isConflictError(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "exclusion constraint") ||
		strings.Contains(err.Error(), "conflicting key value"))
}
