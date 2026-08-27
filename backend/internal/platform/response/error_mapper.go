package response

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	negotiationApp "github.com/labuda/backend/internal/commerce/negotiation/application"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	"github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/internal/identity/auth"
	paymentrepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ErrorMapping contains the HTTP status code and error response details for a given error
type ErrorMapping struct {
	StatusCode int
	Code       string
	Message    string
}

// ============================================================================
// Domain Error Type Checkers
// ============================================================================

// IsInvalidTransitionError checks if err is *entity.InvalidTransitionError
func IsInvalidTransitionError(err error) bool {
	var e *entity.InvalidTransitionError
	return errors.As(err, &e)
}

// IsDisputeActiveError checks if err is *entity.DisputeActiveError
func IsDisputeActiveError(err error) bool {
	var e *entity.DisputeActiveError
	return errors.As(err, &e)
}

// IsInvalidEscrowStatusError checks if err is *entity.InvalidEscrowStatusError
func IsInvalidEscrowStatusError(err error) bool {
	var e *entity.InvalidEscrowStatusError
	return errors.As(err, &e)
}

// IsErrInvalidRefundAmount checks if err is *entity.ErrInvalidRefundAmount
func IsErrInvalidRefundAmount(err error) bool {
	var e *entity.ErrInvalidRefundAmount
	return errors.As(err, &e)
}

// IsErrAlreadyResolved checks if err is *entity.ErrAlreadyResolved
func IsErrAlreadyResolved(err error) bool {
	var e *entity.ErrAlreadyResolved
	return errors.As(err, &e)
}

// IsErrInvalidStateForPartialRefund checks if err is *entity.ErrInvalidStateForPartialRefund
func IsErrInvalidStateForPartialRefund(err error) bool {
	var e *entity.ErrInvalidStateForPartialRefund
	return errors.As(err, &e)
}

// ============================================================================
// Negotiation Domain Error Type Checkers (PASS_8A / F3)
//
// Before this pass, every negotiation business rejection fell through to the
// generic 500 default below, because none of these types were recognised
// here — including negotiationEntity.InvalidTransitionError, which is a
// distinct Go type from order/entity.InvalidTransitionError above despite
// the identical name (errors.As matches by concrete type, not name).
// ============================================================================

func isErrNegotiationRoomMismatch(err error) bool {
	var e *negotiationApp.ErrNegotiationRoomMismatch
	return errors.As(err, &e)
}

func isErrCannotNegotiateWithSelf(err error) bool {
	var e *negotiationApp.ErrCannotNegotiateWithSelf
	return errors.As(err, &e)
}

func isErrNegotiationBlockedByRelationship(err error) bool {
	var e *negotiationApp.ErrNegotiationBlockedByRelationship
	return errors.As(err, &e)
}

func isErrActiveSessionExists(err error) bool {
	var e *negotiationApp.ErrActiveSessionExists
	return errors.As(err, &e)
}

func isErrNegotiationResourceNotFound(err error) bool {
	var e *negotiationApp.ErrResourceNotFound
	return errors.As(err, &e)
}

func isErrResourceNotNegotiable(err error) bool {
	var e *negotiationApp.ErrResourceNotNegotiable
	return errors.As(err, &e)
}

func isErrResourceTypeNotImplemented(err error) bool {
	var e *negotiationApp.ErrResourceTypeNotImplemented
	return errors.As(err, &e)
}

func isErrNegotiationExpired(err error) bool {
	var e *negotiationApp.ErrNegotiationExpired
	return errors.As(err, &e)
}

func isNegotiationUnauthorizedParticipant(err error) bool {
	var e *negotiationEntity.UnauthorizedParticipantError
	return errors.As(err, &e)
}

func isNegotiationNotBuyer(err error) bool {
	var e *negotiationEntity.NotBuyerError
	return errors.As(err, &e)
}

func isNegotiationNotSeller(err error) bool {
	var e *negotiationEntity.NotSellerError
	return errors.As(err, &e)
}

func isNegotiationSessionNotActive(err error) bool {
	var e *negotiationEntity.SessionNotActiveError
	return errors.As(err, &e)
}

func isNegotiationSessionAlreadyTerminal(err error) bool {
	var e *negotiationEntity.SessionAlreadyTerminalError
	return errors.As(err, &e)
}

func isNegotiationInvalidTransition(err error) bool {
	var e *negotiationEntity.InvalidTransitionError
	return errors.As(err, &e)
}

func isNegotiationInvalidPrice(err error) bool {
	var e *negotiationEntity.InvalidPriceError
	return errors.As(err, &e)
}

func isNegotiationNoPrice(err error) bool {
	var e *negotiationEntity.NoPriceError
	return errors.As(err, &e)
}

func isNegotiationStaleProposal(err error) bool {
	var e *negotiationEntity.StaleProposalError
	return errors.As(err, &e)
}

func isNegotiationChatRoomNotSet(err error) bool {
	var e *negotiationEntity.ErrChatRoomNotSet
	return errors.As(err, &e)
}

func isNegotiationMultipleAccepted(err error) bool {
	var e *negotiationEntity.ErrMultipleAcceptedNegotiations
	return errors.As(err, &e)
}

// IsAuthError checks if err is any auth package error
func IsAuthError(err error) bool {
	return errors.Is(err, auth.ErrUnauthorized) ||
		errors.Is(err, auth.ErrAdminRequired) ||
		errors.Is(err, auth.ErrBuyerRequired) ||
		errors.Is(err, auth.ErrSellerRequired) ||
		errors.Is(err, auth.ErrOwnerRequired) ||
		errors.Is(err, auth.ErrInvalidCaller) ||
		errors.Is(err, auth.ErrAccountSuspended) ||
		errors.Is(err, auth.ErrAccountBanned) ||
		errors.Is(err, auth.ErrAccountInactive) ||
		errors.Is(err, auth.ErrAccountRemoved)
}

// IsPaymentError checks if err is payment repository error
func IsPaymentError(err error) bool {
	return errors.Is(err, paymentrepo.ErrInvalidStatusTransition) ||
		errors.Is(err, paymentrepo.ErrReferenceIDRequired)
}

// IsOutboxError checks if err is outbox repository error
func IsOutboxError(err error) bool {
	return errors.Is(err, outboxrepo.ErrEventNotFound) ||
		errors.Is(err, outboxrepo.ErrInvalidStatusTransition)
}

// ============================================================================
// Error Mapper
// ============================================================================

// MapErrorToResponse maps an error to its HTTP response mapping
func MapErrorToResponse(err error) ErrorMapping {
	// === Domain Errors ===

	// Trade domain errors
	if IsErrInvalidRefundAmount(err) {
		return ErrorMapping{
			StatusCode: http.StatusBadRequest,
			Code:       "INVALID_REFUND_AMOUNT",
			Message:    "Refund amount must be greater than 0 and less than or equal to escrow amount.",
		}
	}

	if IsErrAlreadyResolved(err) {
		return ErrorMapping{
			StatusCode: http.StatusConflict,
			Code:       "ALREADY_RESOLVED",
			Message:    "Trade has already been resolved and cannot be modified.",
		}
	}

	if IsErrInvalidStateForPartialRefund(err) {
		return ErrorMapping{
			StatusCode: http.StatusBadRequest,
			Code:       "INVALID_STATE_FOR_PARTIAL_REFUND",
			Message:    "Trade state does not allow partial refund.",
		}
	}

	if IsInvalidTransitionError(err) {
		return ErrorMapping{
			StatusCode: http.StatusBadRequest,
			Code:       "INVALID_STATE_TRANSITION",
			Message:    "Cannot perform this action in the current state.",
		}
	}

	if IsInvalidEscrowStatusError(err) {
		return ErrorMapping{
			StatusCode: http.StatusBadRequest,
			Code:       "INVALID_ESCROW_STATUS",
			Message:    "Escrow is not in the required state for this operation.",
		}
	}

	if IsDisputeActiveError(err) {
		return ErrorMapping{
			StatusCode: http.StatusConflict,
			Code:       "DISPUTE_ACTIVE",
			Message:    "Cannot complete this operation while a dispute is active.",
		}
	}

	// Negotiation domain errors (PASS_8A / F3)
	if isErrNegotiationRoomMismatch(err) {
		return ErrorMapping{
			StatusCode: http.StatusForbidden,
			Code:       "NEGOTIATION_ROOM_MISMATCH",
			Message:    "This chat room's other participant is not the seller of this for_sale item.",
		}
	}

	if isErrCannotNegotiateWithSelf(err) {
		return ErrorMapping{
			StatusCode: http.StatusBadRequest,
			Code:       "CANNOT_NEGOTIATE_WITH_SELF",
			Message:    "You cannot negotiate on your own for_sale item.",
		}
	}

	if isErrNegotiationBlockedByRelationship(err) {
		return ErrorMapping{
			StatusCode: http.StatusForbidden,
			Code:       "NEGOTIATION_BLOCKED",
			Message:    "This negotiation is not allowed between these users.",
		}
	}

	if isErrActiveSessionExists(err) {
		return ErrorMapping{
			StatusCode: http.StatusConflict,
			Code:       "NEGOTIATION_ACTIVE_SESSION_EXISTS",
			Message:    "You already have an active negotiation for this for_sale item.",
		}
	}

	if isErrNegotiationResourceNotFound(err) {
		return ErrorMapping{
			StatusCode: http.StatusNotFound,
			Code:       "NEGOTIATION_RESOURCE_NOT_FOUND",
			Message:    "The for_sale item for this negotiation was not found.",
		}
	}

	if isErrResourceNotNegotiable(err) {
		return ErrorMapping{
			StatusCode: http.StatusConflict,
			Code:       "NEGOTIATION_RESOURCE_NOT_NEGOTIABLE",
			Message:    "This for_sale item is not available for negotiation.",
		}
	}

	if isErrResourceTypeNotImplemented(err) {
		return ErrorMapping{
			StatusCode: http.StatusBadRequest,
			Code:       "NEGOTIATION_RESOURCE_TYPE_UNSUPPORTED",
			Message:    "This resource type does not support negotiation.",
		}
	}

	if isErrNegotiationExpired(err) {
		return ErrorMapping{
			StatusCode: http.StatusConflict,
			Code:       "NEGOTIATION_EXPIRED",
			Message:    "This negotiation has expired.",
		}
	}

	if isNegotiationUnauthorizedParticipant(err) {
		return ErrorMapping{
			StatusCode: http.StatusForbidden,
			Code:       "NEGOTIATION_UNAUTHORIZED_PARTICIPANT",
			Message:    "You are not a participant in this negotiation.",
		}
	}

	if isNegotiationNotBuyer(err) {
		return ErrorMapping{
			StatusCode: http.StatusForbidden,
			Code:       "NEGOTIATION_BUYER_ONLY",
			Message:    "Only the buyer can perform this action.",
		}
	}

	if isNegotiationNotSeller(err) {
		return ErrorMapping{
			StatusCode: http.StatusForbidden,
			Code:       "NEGOTIATION_SELLER_ONLY",
			Message:    "Only the seller can perform this action.",
		}
	}

	if isNegotiationSessionNotActive(err) || isNegotiationSessionAlreadyTerminal(err) || isNegotiationInvalidTransition(err) {
		return ErrorMapping{
			StatusCode: http.StatusConflict,
			Code:       "NEGOTIATION_INVALID_STATE",
			Message:    "This negotiation cannot be modified in its current state.",
		}
	}

	if isNegotiationInvalidPrice(err) || isNegotiationNoPrice(err) {
		return ErrorMapping{
			StatusCode: http.StatusBadRequest,
			Code:       "NEGOTIATION_INVALID_PRICE",
			Message:    "The proposed price is not valid.",
		}
	}

	if isNegotiationStaleProposal(err) {
		return ErrorMapping{
			StatusCode: http.StatusConflict,
			Code:       "NEGOTIATION_STALE_PROPOSAL",
			Message:    "This price proposal is out of date. Please refresh and try again.",
		}
	}

	if isNegotiationChatRoomNotSet(err) {
		return ErrorMapping{
			StatusCode: http.StatusConflict,
			Code:       "NEGOTIATION_CHAT_ROOM_NOT_SET",
			Message:    "This negotiation is not linked to a chat room.",
		}
	}

	if isNegotiationMultipleAccepted(err) {
		return ErrorMapping{
			StatusCode: http.StatusConflict,
			Code:       "NEGOTIATION_MULTIPLE_ACCEPTED",
			Message:    "You already have an accepted negotiation for this for_sale item.",
		}
	}

	// Authorization errors
	if errors.Is(err, auth.ErrUnauthorized) {
		return ErrorMapping{
			StatusCode: http.StatusUnauthorized,
			Code:       "UNAUTHORIZED",
			Message:    "You do not have permission to perform this action.",
		}
	}

	if errors.Is(err, auth.ErrAdminRequired) {
		return ErrorMapping{
			StatusCode: http.StatusForbidden,
			Code:       "ADMIN_REQUIRED",
			Message:    "This action requires administrator privileges.",
		}
	}

	if errors.Is(err, auth.ErrBuyerRequired) {
		return ErrorMapping{
			StatusCode: http.StatusForbidden,
			Code:       "BUYER_ONLY",
			Message:    "Only the buyer can perform this action.",
		}
	}

	if errors.Is(err, auth.ErrSellerRequired) {
		return ErrorMapping{
			StatusCode: http.StatusForbidden,
			Code:       "SELLER_ONLY",
			Message:    "Only the seller can perform this action.",
		}
	}

	if errors.Is(err, auth.ErrOwnerRequired) {
		return ErrorMapping{
			StatusCode: http.StatusForbidden,
			Code:       "OWNER_ONLY",
			Message:    "Only the resource owner can perform this action.",
		}
	}

	if errors.Is(err, auth.ErrInvalidCaller) {
		return ErrorMapping{
			StatusCode: http.StatusUnauthorized,
			Code:       "INVALID_CALLER",
			Message:    "Invalid caller identification.",
		}
	}

	if errors.Is(err, auth.ErrAccountSuspended) {
		return ErrorMapping{
			StatusCode: http.StatusForbidden,
			Code:       "ACCOUNT_SUSPENDED",
			Message:    "Your account has been suspended.",
		}
	}

	if errors.Is(err, auth.ErrAccountBanned) {
		return ErrorMapping{
			StatusCode: http.StatusForbidden,
			Code:       "ACCOUNT_BANNED",
			Message:    "Your account has been banned.",
		}
	}

	if errors.Is(err, auth.ErrAccountInactive) {
		return ErrorMapping{
			StatusCode: http.StatusForbidden,
			Code:       "ACCOUNT_INACTIVE",
			Message:    "Your account is not active.",
		}
	}

	if errors.Is(err, auth.ErrAccountRemoved) {
		return ErrorMapping{
			StatusCode: http.StatusForbidden,
			Code:       "ACCOUNT_REMOVED",
			Message:    "Your account has been removed.",
		}
	}

	// Payment domain errors
	if errors.Is(err, paymentrepo.ErrInvalidStatusTransition) {
		return ErrorMapping{
			StatusCode: http.StatusConflict,
			Code:       "INVALID_PAYMENT_STATUS",
			Message:    "Payment cannot be processed in its current state.",
		}
	}

	if errors.Is(err, paymentrepo.ErrReferenceIDRequired) {
		return ErrorMapping{
			StatusCode: http.StatusBadRequest,
			Code:       "REFERENCE_REQUIRED",
			Message:    "Payment reference is required.",
		}
	}

	// Outbox errors
	if errors.Is(err, outboxrepo.ErrEventNotFound) {
		return ErrorMapping{
			StatusCode: http.StatusNotFound,
			Code:       "EVENT_NOT_FOUND",
			Message:    "Event not found.",
		}
	}

	if errors.Is(err, outboxrepo.ErrInvalidStatusTransition) {
		return ErrorMapping{
			StatusCode: http.StatusConflict,
			Code:       "INVALID_EVENT_STATUS",
			Message:    "Event cannot be processed in its current state.",
		}
	}

	// === Database Constraint Errors ===

	if db.IsUniqueViolation(err) {
		return ErrorMapping{
			StatusCode: http.StatusConflict,
			Code:       "DUPLICATE_RECORD",
			Message:    "A record with these values already exists.",
		}
	}

	if db.IsCheckViolation(err) {
		return ErrorMapping{
			StatusCode: http.StatusBadRequest,
			Code:       "CONSTRAINT_VIOLATION",
			Message:    "Request violates business rules.",
		}
	}

	if db.IsSerializationFailure(err) || db.IsDeadlock(err) {
		return ErrorMapping{
			StatusCode: http.StatusServiceUnavailable,
			Code:       "RETRYABLE_ERROR",
			Message:    "The server is busy. Please retry the request.",
		}
	}

	// === Default Internal Error ===
	return ErrorMapping{
		StatusCode: http.StatusInternalServerError,
		Code:       "INTERNAL_ERROR",
		Message:    "Something went wrong. Please try again later.",
	}
}

// ============================================================================
// Error Response Helpers
// ============================================================================

// RespondWithError logs the error and sends appropriate HTTP response
// This is the main entry point for error handling in handlers
func RespondWithError(c *gin.Context, log *zap.Logger, err error) {
	if err == nil {
		return
	}

	mapping := MapErrorToResponse(err)

	// Log errors with appropriate level
	if mapping.StatusCode >= 500 {
		// Internal server errors - log full error
		if log != nil {
			log.Error("API error",
				zap.Int("status", mapping.StatusCode),
				zap.String("code", mapping.Code),
				zap.String("path", c.Request.URL.Path),
				zap.String("method", c.Request.Method),
				zap.Error(err),
			)
		}
	} else {
		// Client errors (4xx) - log at debug/info level
		if log != nil {
			log.Debug("API client error",
				zap.Int("status", mapping.StatusCode),
				zap.String("code", mapping.Code),
				zap.String("path", c.Request.URL.Path),
				zap.String("error_detail", err.Error()),
			)
		}
	}

	Error(c, mapping.StatusCode, mapping.Code, mapping.Message)
}

// RespondWithErrorAndDetails logs the error and sends HTTP response with details
func RespondWithErrorAndDetails(c *gin.Context, log *zap.Logger, err error, details interface{}) {
	if err == nil {
		return
	}

	mapping := MapErrorToResponse(err)

	// Log errors with appropriate level
	if mapping.StatusCode >= 500 {
		if log != nil {
			log.Error("API error",
				zap.Int("status", mapping.StatusCode),
				zap.String("code", mapping.Code),
				zap.String("path", c.Request.URL.Path),
				zap.String("method", c.Request.Method),
				zap.Error(err),
			)
		}
	} else {
		if log != nil {
			log.Debug("API client error",
				zap.Int("status", mapping.StatusCode),
				zap.String("code", mapping.Code),
				zap.String("path", c.Request.URL.Path),
				zap.String("error_detail", err.Error()),
			)
		}
	}

	ErrorWithDetails(c, mapping.StatusCode, mapping.Code, mapping.Message, details)
}

// ============================================================================
// Panic Recovery Helper
// ============================================================================

// RecoverFromPanic recovers from panics and returns error response
func RecoverFromPanic(c *gin.Context, log *zap.Logger) {
	if r := recover(); r != nil {
		// Log the panic
		if log != nil {
			log.Error("Panic recovered",
				zap.String("path", c.Request.URL.Path),
				zap.String("method", c.Request.Method),
				zap.Any("panic", r),
			)
		}

		// Send internal server error
		InternalServerError(c, "An unexpected error occurred. Please try again.")
		c.Abort()
	}
}

// FormatError creates a wrapped error with context for debugging
// Use this when you need to preserve error context without exposing it to clients
func FormatError(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// WrapError wraps an error with additional context
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
