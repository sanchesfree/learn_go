package handler

import (
	"context"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"booking-service/pkg/httputil"
)

// ==============================
// Middleware — цепочка обработчиков
// ==============================
// Middleware в Go — это func(http.Handler) http.Handler.
// Каждый оборачивает следующий, образуя chain.
// Порядок важен: recovery должен быть первым (самым внешним).

// withRecovery — ловит panic и возвращает 500 вместо краша сервера.
// Это ОБЯЗАТЕЛЬНЫЙ middleware для любого Go HTTP-сервиса.
func (h *BookingHandler) withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				h.logger.Error("panic recovered",
					"panic", rec,
					"stack", string(debug.Stack()),
					"path", r.URL.Path,
				)
				httputil.JSON(w, http.StatusInternalServerError, map[string]string{
					"error": "internal server error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// withLogging — логирует каждый запрос. Используем slog (stdlib в Go 1.21+).
func (h *BookingHandler) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// responseWriter wrapper чтобы捕获 статус-код
		rw := &statusWriter{ResponseWriter: w, status: 200}

		next.ServeHTTP(rw, r)

		h.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// withRequestID — добавляет уникальный ID запроса в контекст.
// Нужен для трассировки запросов через логи и распределённые системы.
func (h *BookingHandler) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateID()
		}

		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withAuth — простейшая авторизация по заголовку X-User-ID.
// На проде здесь был бы JWT или session-based auth.
func (h *BookingHandler) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Пропускаем health check без авторизации
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			userID = "anonymous" // Для демо — пропускаем без авторизации
		}

		ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withRateLimit — простой rate limiter на sync.Map.
// На проде используют token bucket (golang.org/x/time/rate) или Redis-based.
func (h *BookingHandler) withRateLimit(next http.Handler) http.Handler {
	type visitor struct {
		tokens    int
		lastCheck time.Time
	}

	var (
		visitors sync.Map
		rate     = 10            // запросов
		burst    = 20            // максимальный burst
		interval = time.Second   // за секунду
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := strings.Split(r.RemoteAddr, ":")[0]

		v, _ := visitors.LoadOrStore(ip, &visitor{tokens: burst, lastCheck: time.Now()})
		vis := v.(*visitor)

		// Refill tokens
		now := time.Now()
		elapsed := now.Sub(vis.lastCheck)
		if elapsed > interval {
			vis.tokens += int(elapsed/interval) * rate
			if vis.tokens > burst {
				vis.tokens = burst
			}
			vis.lastCheck = now
		}

		if vis.tokens <= 0 {
			httputil.JSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded",
			})
			return
		}

		vis.tokens--
		next.ServeHTTP(w, r)
	})
}

// withCORS — базовый CORS для фронтенда
func (h *BookingHandler) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID, X-Request-ID")

		// Preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// --- Helpers ---

const ctxKeyRequestID contextKey = "request_id"

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func generateID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	// crypto/rand для уникальности, но для learning project достаточно time + counter
	nanos := time.Now().UnixNano()
	for i := range b {
		b[i] = charset[(nanos+int64(i*37))%int64(len(charset))]
	}
	return string(b)
}
