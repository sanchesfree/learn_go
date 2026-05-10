package httputil

import (
	"encoding/json"
	"net/http"

	apperrors "booking-service/pkg/errors"
)

// Response — стандартный JSON-ответ API
type Response struct {
	OK      bool        `json:"ok"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorBody  `json:"error,omitempty"`
}

// ErrorBody — тело ошибки в ответе
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON — отправляет JSON-ответ
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		OK:   status < 400,
		Data: data,
	})
}

// Error — отправляет ошибку
func Error(w http.ResponseWriter, err error) {
	// Пробуем распаковать AppError
	if appErr, ok := apperrors.IsAppError(err); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(appErr.HTTP)
		json.NewEncoder(w).Encode(Response{
			OK: false,
			Error: &ErrorBody{
				Code:    appErr.Code,
				Message: appErr.Message,
			},
		})
		return
	}

	// Fallback для непонятных ошибок
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(Response{
		OK: false,
		Error: &ErrorBody{
			Code:    "internal",
			Message: "internal server error",
		},
	})
}

// Paginated — отправляет пагинированный ответ
func Paginated(w http.ResponseWriter, data interface{}, total int64, page, pageSize int) {
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"data":        data,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
	})
}

// DecodeJSON — парсит JSON из request body
func DecodeJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}
