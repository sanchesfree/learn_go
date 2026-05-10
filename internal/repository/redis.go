package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"booking-service/internal/model"

	"github.com/redis/go-redis/v9"
)

// CacheRepository — кэширование через Redis
// Покрывает: Redis, serialization, TTL, cache invalidation
type CacheRepository struct {
	client *redis.Client
	prefix string // namespace для ключей, чтобы не конфликтовать с другими сервисами
}

func NewCacheRepository(client *redis.Client) *CacheRepository {
	return &CacheRepository{
		client: client,
		prefix: "booking:",
	}
}

// RoomCache — кэшируем переговорку на 5 минут
func (r *CacheRepository) GetRoom(ctx context.Context, id int64) (model.Room, error) {
	key := r.roomKey(id)

	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			// Cache miss — это нормально, не ошибка
			return model.Room{}, ErrCacheMiss
		}
		return model.Room{}, fmt.Errorf("redis get room: %w", err)
	}

	var room model.Room
	if err := json.Unmarshal(data, &room); err != nil {
		// Если данные битые — удаляем из кэша
		r.client.Del(ctx, key)
		return model.Room{}, fmt.Errorf("redis unmarshal room: %w", err)
	}

	return room, nil
}

func (r *CacheRepository) SetRoom(ctx context.Context, room model.Room) error {
	data, err := json.Marshal(room)
	if err != nil {
		return fmt.Errorf("redis marshal room: %w", err)
	}

	key := r.roomKey(room.ID)
	// SetEX — установка с TTL
	if err := r.client.Set(ctx, key, data, 5*time.Minute).Err(); err != nil {
		return fmt.Errorf("redis set room: %w", err)
	}
	return nil
}

func (r *CacheRepository) InvalidateRoom(ctx context.Context, id int64) error {
	return r.client.Del(ctx, r.roomKey(id)).Err()
}

// BookingsByRoomCache — кэшируем список бронирований комнаты на день
func (r *CacheRepository) GetBookingsByRoom(ctx context.Context, roomID int64, date time.Time) ([]model.Booking, error) {
	key := r.bookingsKey(roomID, date)

	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("redis get bookings: %w", err)
	}

	var bookings []model.Booking
	if err := json.Unmarshal(data, &bookings); err != nil {
		r.client.Del(ctx, key)
		return nil, fmt.Errorf("redis unmarshal bookings: %w", err)
	}

	return bookings, nil
}

func (r *CacheRepository) SetBookingsByRoom(ctx context.Context, roomID int64, date time.Time, bookings []model.Booking) error {
	data, err := json.Marshal(bookings)
	if err != nil {
		return fmt.Errorf("redis marshal bookings: %w", err)
	}

	// Кэшируем на 1 минуту — бронирования часто меняются
	key := r.bookingsKey(roomID, date)
	if err := r.client.Set(ctx, key, data, 1*time.Minute).Err(); err != nil {
		return fmt.Errorf("redis set bookings: %w", err)
	}
	return nil
}

func (r *CacheRepository) InvalidateBookingsByRoom(ctx context.Context, roomID int64, date time.Time) error {
	return r.client.Del(ctx, r.bookingsKey(roomID, date)).Err()
}

// --- Distributed Lock ---
// Простой lock через Redis SET NX EX. На проде используют Redlock или similar.

// AcquireLock — пытается взять lock. Возвращает true, если получилось.
func (r *CacheRepository) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	fullKey := r.prefix + "lock:" + key
	ok, err := r.client.SetNX(ctx, fullKey, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis acquire lock: %w", err)
	}
	return ok, nil
}

// ReleaseLock — отпускает lock
func (r *CacheRepository) ReleaseLock(ctx context.Context, key string) error {
	fullKey := r.prefix + "lock:" + key
	return r.client.Del(ctx, fullKey).Err()
}

// --- Key builders ---

func (r *CacheRepository) roomKey(id int64) string {
	return fmt.Sprintf("%sroom:%d", r.prefix, id)
}

func (r *CacheRepository) bookingsKey(roomID int64, date time.Time) string {
	return fmt.Sprintf("%sbookings:%d:%s", r.prefix, roomID, date.Format("2006-01-02"))
}

// ErrCacheMiss — сигнализирует что данных в кэше нет
var ErrCacheMiss = fmt.Errorf("cache miss")
