package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// InternalError sends a 500 Internal Server Error response (alias for InternalServerError)
func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", message)
}

// SuccessWithPagination sends a successful response with pagination metadata
func SuccessWithPagination(c *gin.Context, data interface{}, page, pageSize int, total int64) {
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
		Meta: &Meta{
			Page:       page,
			PerPage:    pageSize,
			Total:      int(total),
			TotalPages: totalPages,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Conflict sends a 409 Conflict response
func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, "CONFLICT", message)
}

// Gone sends a 410 Gone response
func Gone(c *gin.Context, message string) {
	Error(c, http.StatusGone, "GONE", message)
}

// TooManyRequests sends a 429 Too Many Requests response
func TooManyRequests(c *gin.Context, message string) {
	Error(c, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", message)
}

// ServiceUnavailable sends a 503 Service Unavailable response
func ServiceUnavailable(c *gin.Context, message string) {
	Error(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", message)
}

// Accepted sends a 202 Accepted response
func Accepted(c *gin.Context, data interface{}) {
	c.JSON(http.StatusAccepted, Response{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}


