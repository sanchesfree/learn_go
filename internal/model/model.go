package model

import (
	"time"
)

// Room — переговорка
type Room struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Capacity  int       `json:"capacity" db:"capacity"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Booking — бронирование переговорки
type Booking struct {
	ID        int64     `json:"id" db:"id"`
	RoomID    int64     `json:"room_id" db:"room_id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Title     string    `json:"title" db:"title"`
	StartTime time.Time `json:"start_time" db:"start_time"`
	EndTime   time.Time `json:"end_time" db:"end_time"`
	Status    string    `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// CreateBookingRequest — запрос на создание бронирования
type CreateBookingRequest struct {
	RoomID    int64     `json:"room_id"`
	Title     string    `json:"title"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// UpdateBookingRequest — запрос на обновление бронирования
type UpdateBookingRequest struct {
	Title     *string    `json:"title,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
}

// RoomWithAvailability — переговорка с информацией о доступности
type RoomWithAvailability struct {
	Room         Room       `json:"room"`
	Availability []TimeSlot `json:"availability"`
}

// TimeSlot — временной слот доступности
type TimeSlot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// PaginatedResponse — пагинированный ответ
type PaginatedResponse[T any] struct {
	Data       []T   `json:"data"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

// --- Status константы ---
const (
	StatusActive    = "active"
	StatusCancelled = "cancelled"
)
