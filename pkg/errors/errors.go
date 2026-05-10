package errors

import "fmt"

// AppError — кастомная ошибка приложения с кодом для HTTP-ответа
type AppError struct {
	Code    string // внутренний код ошибки (e.g. "not_found", "conflict")
	Message string // человекочитаемое сообщение
	HTTP    int    // HTTP статус-код
	Err     error  // оригинальная (wrapped) ошибка
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// Конструкторы для частых ошибок

func NotFound(msg string, err error) *AppError {
	return &AppError{Code: "not_found", Message: msg, HTTP: 404, Err: err}
}

func Conflict(msg string, err error) *AppError {
	return &AppError{Code: "conflict", Message: msg, HTTP: 409, Err: err}
}

func BadRequest(msg string, err error) *AppError {
	return &AppError{Code: "bad_request", Message: msg, HTTP: 400, Err: err}
}

func Unauthorized(msg string, err error) *AppError {
	return &AppError{Code: "unauthorized", Message: msg, HTTP: 401, Err: err}
}

func Internal(msg string, err error) *AppError {
	return &AppError{Code: "internal", Message: msg, HTTP: 500, Err: err}
}

// Is — проверяет, является ли err конкретным типом AppError
// Используется для errors.Is(err, &AppError{Code: "not_found"})
func IsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if ok := As(err, &appErr); ok {
		return appErr, true
	}
	return nil, false
}

// As — аналог errors.As для AppError
func As(err error, target **AppError) bool {
	for err != nil {
		if e, ok := err.(*AppError); ok {
			*target = e
			return true
		}
		// Собственный unwrap через интерфейс
		if u, ok := err.(interface{ Unwrap() error }); ok {
			err = u.Unwrap()
		} else {
			break
		}
	}
	return false
}
