// Package response provides standard API response structures and helpers
// with swagger documentation for common error scenarios.
package response

// ErrorResponse401 represents a 401 Unauthorized response
// Example response when Firebase token is missing or invalid:
//
//	{
//	  "success": false,
//	  "error": {
//	    "code": "UNAUTHORIZED",
//	    "message": "Firebase authentication required"
//	  },
//	  "timestamp": "2024-01-15T10:30:00Z"
//	}
type ErrorResponse401 struct {
	Success   bool      `json:"success" example:"false"`
	Error     ErrorInfo `json:"error"`
	Timestamp string    `json:"timestamp" example:"2024-01-15T10:30:00Z"`
}

// ErrorResponse403 represents a 403 Forbidden response
// Example response when user lacks permission:
//
//	{
//	  "success": false,
//	  "error": {
//	    "code": "FORBIDDEN",
//	    "message": "You don't have permission to access this resource"
//	  },
//	  "timestamp": "2024-01-15T10:30:00Z"
//	}
type ErrorResponse403 struct {
	Success   bool      `json:"success" example:"false"`
	Error     ErrorInfo `json:"error"`
	Timestamp string    `json:"timestamp" example:"2024-01-15T10:30:00Z"`
}

// ErrorResponse422 represents a 422 Validation Error response
// Example response when request validation fails:
//
//	{
//	  "success": false,
//	  "error": {
//	    "code": "VALIDATION_ERROR",
//	    "message": "Validation failed",
//	    "details": {
//	      "email": "must be a valid email address",
//	      "password": "must be at least 8 characters"
//	    }
//	  },
//	  "timestamp": "2024-01-15T10:30:00Z"
//	}
type ErrorResponse422 struct {
	Success   bool      `json:"success" example:"false"`
	Error     ErrorInfo `json:"error"`
	Timestamp string    `json:"timestamp" example:"2024-01-15T10:30:00Z"`
}

// ErrorResponse500 represents a 500 Internal Server Error response
// Example response when an unexpected error occurs:
//
//	{
//	  "success": false,
//	  "error": {
//	    "code": "INTERNAL_SERVER_ERROR",
//	    "message": "An unexpected error occurred"
//	  },
//	  "timestamp": "2024-01-15T10:30:00Z"
//	}
type ErrorResponse500 struct {
	Success   bool      `json:"success" example:"false"`
	Error     ErrorInfo `json:"error"`
	Timestamp string    `json:"timestamp" example:"2024-01-15T10:30:00Z"`
}

// SuccessResponseWithData represents a successful response with data
// Example:
//
//	{
//	  "success": true,
//	  "message": "User retrieved successfully",
//	  "data": { "id": "123", "email": "user@example.com" },
//	  "timestamp": "2024-01-15T10:30:00Z"
//	}
type SuccessResponseWithData struct {
	Success   bool        `json:"success" example:"true"`
	Message   string      `json:"message,omitempty" example:"User retrieved successfully"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp" example:"2024-01-15T10:30:00Z"`
}

// SuccessResponseWithPagination represents a successful paginated response
// Example:
//
//	{
//	  "success": true,
//	  "data": [{ "id": "1" }, { "id": "2" }],
//	  "meta": { "page": 1, "per_page": 20, "total": 100, "total_pages": 5 },
//	  "timestamp": "2024-01-15T10:30:00Z"
//	}
type SuccessResponseWithPagination struct {
	Success   bool        `json:"success" example:"true"`
	Data      interface{} `json:"data"`
	Meta      *Meta       `json:"meta"`
	Timestamp string      `json:"timestamp" example:"2024-01-15T10:30:00Z"`
}

// Common error codes used across the API
const (
	// AuthErrorCodes - Authentication related error codes
	ErrUnauthorized       = "UNAUTHORIZED"       // 401 - Missing or invalid token
	ErrForbidden          = "FORBIDDEN"          // 403 - Insufficient permissions
	ErrTokenExpired       = "TOKEN_EXPIRED"      // 401 - Firebase token expired
	ErrInvalidToken       = "INVALID_TOKEN"      // 401 - Malformed Firebase token

	// ValidationErrorCodes - Request validation error codes
	ErrValidationFailed   = "VALIDATION_ERROR"   // 422 - Request validation failed
	ErrInvalidInput       = "INVALID_INPUT"      // 400 - Invalid request format
	ErrMissingField       = "MISSING_FIELD"      // 400 - Required field missing

	// BusinessErrorCodes - Business logic error codes
	ErrNotFound           = "NOT_FOUND"          // 404 - Resource not found
	ErrConflict           = "CONFLICT"           // 409 - Resource conflict
	ErrAlreadyExists      = "ALREADY_EXISTS"     // 409 - Resource already exists

	// ServerErrorCodes - Server error codes
	ErrInternalServer     = "INTERNAL_SERVER_ERROR" // 500 - Unexpected error
	ErrDatabaseError      = "DATABASE_ERROR"        // 500 - Database operation failed
	ErrServiceUnavailable = "SERVICE_UNAVAILABLE"   // 503 - Service temporarily unavailable
)


