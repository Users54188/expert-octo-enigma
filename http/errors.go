package http

import (
	"encoding/json"
	"log"
	"net/http"
)

// AppError 应用错误类型
type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
	Internal   error  `json:"-"`
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Internal != nil {
		return e.Internal.Error()
	}
	return e.Message
}

// Unwrap 实现 errors.Unwrap
func (e *AppError) Unwrap() error {
	return e.Internal
}

// 预定义错误码
var (
	ErrServiceUnavailable = &AppError{
		Code:       "SERVICE_UNAVAILABLE",
		Message:    "Service temporarily unavailable",
		HTTPStatus: http.StatusServiceUnavailable,
	}
	ErrInvalidRequest = &AppError{
		Code:       "INVALID_REQUEST",
		Message:    "Invalid request",
		HTTPStatus: http.StatusBadRequest,
	}
	ErrUnauthorized = &AppError{
		Code:       "UNAUTHORIZED",
		Message:    "Unauthorized",
		HTTPStatus: http.StatusUnauthorized,
	}
	ErrNotFound = &AppError{
		Code:       "NOT_FOUND",
		Message:    "Resource not found",
		HTTPStatus: http.StatusNotFound,
	}
	ErrInternal = &AppError{
		Code:       "INTERNAL_ERROR",
		Message:    "Internal server error",
		HTTPStatus: http.StatusInternalServerError,
	}
	ErrTooManyRequests = &AppError{
		Code:       "RATE_LIMIT_EXCEEDED",
		Message:    "Too many requests",
		HTTPStatus: http.StatusTooManyRequests,
	}
)

// NewServiceUnavailable 创建服务不可用错误
func NewServiceUnavailable(internal error) *AppError {
	return &AppError{
		Code:       "SERVICE_UNAVAILABLE",
		Message:    "Service temporarily unavailable",
		HTTPStatus: http.StatusServiceUnavailable,
		Internal:   internal,
	}
}

// NewInvalidRequest 创建无效请求错误
func NewInvalidRequest(message string) *AppError {
	return &AppError{
		Code:       "INVALID_REQUEST",
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewInternalError 创建内部错误
func NewInternalError(internal error) *AppError {
	return &AppError{
		Code:       "INTERNAL_ERROR",
		Message:    "Internal server error",
		HTTPStatus: http.StatusInternalServerError,
		Internal:   internal,
	}
}

// HandleError 统一错误处理
func HandleError(w http.ResponseWriter, err error) {
	// 记录内部错误
	log.Printf("API error: %v", err)

	// 检查是否是 AppError
	if appErr, ok := err.(*AppError); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(appErr.HTTPStatus)
		// #nosec G104 -- response encoding is best-effort
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		})
		return
	}

	// 默认返回内部错误
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	// #nosec G104 -- response encoding is best-effort
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "INTERNAL_ERROR",
			"message": "Internal server error",
		},
	})
}

// SanitizeError 向客户端返回通用错误，避免信息泄露
// 保持向后兼容，内部使用 HandleError
func SanitizeError(w http.ResponseWriter, err error, statusCode int) {
	// 记录内部错误
	log.Printf("API error: %v", err)

	// 如果是 AppError，使用其 HTTPStatus
	if appErr, ok := err.(*AppError); ok {
		statusCode = appErr.HTTPStatus
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	// #nosec G104 -- response encoding is best-effort
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "ERROR",
			"message": "Internal server error",
		},
	})
}
