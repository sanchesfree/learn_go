package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"booking-service/internal/model"
	"booking-service/internal/service"
	"booking-service/pkg/httputil"
)

// BookingHandler — HTTP-обработчики API
type BookingHandler struct {
	service *service.BookingService
	logger  *slog.Logger
}

func NewBookingHandler(svc *service.BookingService, logger *slog.Logger) *BookingHandler {
	return &BookingHandler{service: svc, logger: logger}
}

// =====================
// Rooms
// =====================

// CreateRoom godoc
// @Summary Создать переговорку
// @Tags rooms
func (h *BookingHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	var room model.Room
	if err := httputil.DecodeJSON(r, &room); err != nil {
		httputil.Error(w, err)
		return
	}

	created, err := h.service.CreateRoom(r.Context(), room)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusCreated, created)
}

// GetRoom godoc
// @Summary Получить переговорку
// @Tags rooms
func (h *BookingHandler) GetRoom(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	room, err := h.service.GetRoom(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, room)
}

// ListRooms godoc
// @Summary Список переговорок
// @Tags rooms
func (h *BookingHandler) ListRooms(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pagination(r)

	rooms, total, err := h.service.ListRooms(r.Context(), page, pageSize)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Paginated(w, rooms, total, page, pageSize)
}

// DeleteRoom godoc
// @Summary Удалить переговорку
// @Tags rooms
func (h *BookingHandler) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	if err := h.service.DeleteRoom(r.Context(), id); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusNoContent, nil)
}

// =====================
// Bookings
// =====================

// CreateBooking godoc
// @Summary Создать бронирование
// @Tags bookings
func (h *BookingHandler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	var req model.CreateBookingRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, err)
		return
	}

	// Извлекаем user_id из контекста (ставит auth middleware)
	userID, ok := r.Context().Value(ctxKeyUserID).(string)
	if !ok {
		userID = "anonymous" // для демо
	}

	booking, err := h.service.CreateBooking(r.Context(), req, userID)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusCreated, booking)
}

// GetBooking godoc
// @Summary Получить бронирование
// @Tags bookings
func (h *BookingHandler) GetBooking(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	booking, err := h.service.GetBooking(r.Context(), id)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, booking)
}

// GetRoomSchedule godoc
// @Summary Расписание комнаты на день
// @Tags bookings
func (h *BookingHandler) GetRoomSchedule(w http.ResponseWriter, r *http.Request) {
	roomID, err := pathID(r)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	bookings, err := h.service.GetRoomSchedule(r.Context(), roomID, date)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, bookings)
}

// ListUserBookings godoc
// @Summary Бронирования текущего пользователя
// @Tags bookings
func (h *BookingHandler) ListUserBookings(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(ctxKeyUserID).(string)
	if !ok {
		userID = r.URL.Query().Get("user_id") // fallback для демо
		if userID == "" {
			userID = "anonymous"
		}
	}

	page, pageSize := pagination(r)

	bookings, total, err := h.service.ListUserBookings(r.Context(), userID, page, pageSize)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.Paginated(w, bookings, total, page, pageSize)
}

// CancelBooking godoc
// @Summary Отменить бронирование
// @Tags bookings
func (h *BookingHandler) CancelBooking(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httputil.Error(w, err)
		return
	}

	userID, ok := r.Context().Value(ctxKeyUserID).(string)
	if !ok {
		userID = "anonymous"
	}

	if err := h.service.CancelBooking(r.Context(), id, userID); err != nil {
		httputil.Error(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// Health — health check endpoint (для k8s / docker healthcheck)
func (h *BookingHandler) Health(w http.ResponseWriter, r *http.Request) {
	httputil.JSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// --- Helpers ---

type contextKey string

const ctxKeyUserID contextKey = "user_id"

// pathID извлекает ID из пути. В Go 1.22+ ServeMux поддерживает {id} в паттернах,
// но мы парсим вручную для демонстрации.
// ВАЖНО: pathID предполагает формат /resource/{id} или /resource/{id}/subresource
// и извлекает ID по позиции в URL.
func pathID(r *http.Request) (int64, error) {
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	// Паттерны: /rooms/{id}, /bookings/{id}, /rooms/{id}/schedule
	// ID всегда на позиции 2 (index 1 в 0-based)
	if len(parts) >= 3 {
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err == nil {
			return id, nil
		}
	}
	return 0, fmt.Errorf("invalid id in path: %s", r.URL.Path)
}

func pagination(r *http.Request) (page, pageSize int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return
}



// =====================
// Router — маршрутизация
// =====================
// На проде используют chi, gin, или echo. Здесь показан stdlib net/http
// — это важно понимать, т.к. все фреймворки поверх него.

// Router создаёт mux с маршрутами и middleware
func (h *BookingHandler) Router() http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", h.Health)

	// Rooms
	mux.HandleFunc("POST /rooms", h.CreateRoom)
	mux.HandleFunc("GET /rooms", h.ListRooms)
	mux.HandleFunc("GET /rooms/{id}", h.GetRoom)
	mux.HandleFunc("DELETE /rooms/{id}", h.DeleteRoom)

	// Bookings
	mux.HandleFunc("POST /bookings", h.CreateBooking)
	mux.HandleFunc("GET /bookings", h.ListUserBookings)
	mux.HandleFunc("GET /bookings/{id}", h.GetBooking)
	mux.HandleFunc("DELETE /bookings/{id}", h.CancelBooking)
	mux.HandleFunc("GET /rooms/{id}/schedule", h.GetRoomSchedule)

	// Оборачиваем в middleware chain
	var handler http.Handler = mux
	handler = h.withAuth(handler)        // auth первым — чтобы было user_id в контексте
	handler = h.withRateLimit(handler)    // rate limiting
	handler = h.withLogging(handler)      // logging
	handler = h.withRecovery(handler)     // recovery (panic → 500)
	handler = h.withCORS(handler)         // CORS
	handler = h.withRequestID(handler)    // request ID для трассировки

	return handler
}

// RegisterOnShutdown — регистрирует очистку при shutdown
func (h *BookingHandler) RegisterOnShutdown(ctx context.Context) {
	h.logger.Info("handler shutdown registered")
}
