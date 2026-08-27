package mapper

import (
	"fmt"

	"github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
)

// GatewayFailureMapper handles mapping of gateway-specific failure reasons
// to our normalized payment attempt failure reasons.
//
// BNR Phase 1: GATEWAY FAILURE MAPPING
// =====================================
// Maps external gateway status codes into normalized failure reasons:
// - gateway_denied: Payment rejected by gateway (insufficient funds, etc)
// - network_error: Network/infrastructure failure
// - user_cancelled: User explicitly cancelled the payment
// - timeout: Payment expired before completion
// - unknown: Unclassified failure
type GatewayFailureMapper struct{}

// NewGatewayFailureMapper creates a new GatewayFailureMapper.
func NewGatewayFailureMapper() *GatewayFailureMapper {
	return &GatewayFailureMapper{}
}

// MapMidtransStatus maps Midtrans transaction status to payment attempt status and failure reason.
//
// Midtrans Status Codes:
// - capture/settlement: Payment successful
// - pending: Payment awaiting completion
// - deny: Payment denied by gateway
// - cancel: Payment cancelled by user
// - expire: Payment expired
func (m *GatewayFailureMapper) MapMidtransStatus(midtransStatus string) (status string, failureReason *string) {
	switch midtransStatus {
	case "capture", "settlement":
		return repository.PaymentAttemptStatusSuccess, nil

	case "pending":
		return repository.PaymentAttemptStatusPending, nil

	case "deny":
		// Payment denied by gateway (insufficient funds, card declined, etc)
		return repository.PaymentAttemptStatusFailed, strPtr(repository.FailureReasonGatewayDenied)

	case "cancel":
		// User cancelled the payment
		return repository.PaymentAttemptStatusCancelled, strPtr(repository.FailureReasonUserCancelled)

	case "expire":
		// Payment expired (timeout)
		return repository.PaymentAttemptStatusTimeout, strPtr(repository.FailureReasonTimeout)

	default:
		// Unknown status - treat as failed with unknown reason
		return repository.PaymentAttemptStatusFailed, strPtr(repository.FailureReasonUnknown)
	}
}

// MapMidtransStatusCode maps Midtrans status_code to normalized failure reason.
//
// Midtrans status_code provides more detailed failure information:
// - 200: Success (capture/settlement)
// - 201: Pending / Authorized
// - 202: Denied / Cancel (general failure)
// - 401: Invalid signature (security issue)
// - 404: Transaction not found
// - 500: System error (network/internal error)
// - 407: Pending (transaction still pending)
func (m *GatewayFailureMapper) MapMidtransStatusCode(statusCode string) (status string, failureReason *string) {
	switch statusCode {
	case "200":
		// Success
		return repository.PaymentAttemptStatusSuccess, nil

	case "201", "407":
		// Pending
		return repository.PaymentAttemptStatusPending, nil

	case "202":
		// General failure - could be deny or cancel, default to gateway_denied
		return repository.PaymentAttemptStatusFailed, strPtr(repository.FailureReasonGatewayDenied)

	case "401":
		// Security issue - treat as gateway_denied but with specific context
		return repository.PaymentAttemptStatusFailed, strPtr(repository.FailureReasonGatewayDenied)

	case "404":
		// Transaction not found - could be network error
		return repository.PaymentAttemptStatusFailed, strPtr(repository.FailureReasonNetworkError)

	case "500", "501", "502", "503", "504":
		// Server errors - network/internal gateway issues
		return repository.PaymentAttemptStatusFailed, strPtr(repository.FailureReasonNetworkError)

	default:
		// Unknown status code
		return repository.PaymentAttemptStatusFailed, strPtr(repository.FailureReasonUnknown)
	}
}

// MapFraudStatus maps Midtrans fraud_status to failure reason.
//
// Fraud detection is separate from payment status but can cause denial:
// - accept: No fraud detected
// - challenge: Suspicious (manual review)
// - deny: Fraud detected, payment denied
func (m *GatewayFailureMapper) MapFraudStatus(fraudStatus string) *string {
	switch fraudStatus {
	case "accept":
		// No fraud issue
		return nil

	case "challenge":
		// Suspicious - manual review, treat as pending
		return nil

	case "deny":
		// Fraud detected - map to gateway_denied
		return strPtr(repository.FailureReasonGatewayDenied)

	default:
		// Unknown fraud status - no specific action
		return nil
	}
}

// DetermineFinalFailureReason combines multiple signals to determine final failure reason.
//
// Priority order:
// 1. User cancellation (most explicit)
// 2. Fraud denial
// 3. Gateway denial
// 4. Timeout
// 5. Network error
// 6. Unknown
func (m *GatewayFailureMapper) DetermineFinalFailureReason(
	midtransStatus string,
	statusCode string,
	fraudStatus string,
) string {
	// Check for user cancellation first
	status, _ := m.MapMidtransStatus(midtransStatus)
	if status == repository.PaymentAttemptStatusCancelled {
		return repository.FailureReasonUserCancelled
	}

	// Check for fraud denial
	if fraudReason := m.MapFraudStatus(fraudStatus); fraudReason != nil {
		if *fraudReason == repository.FailureReasonGatewayDenied {
			return "fraud_denied" // Special case for fraud
		}
	}

	// Use status code mapping for more detailed failure reason
	_, failureReason := m.MapMidtransStatusCode(statusCode)
	if failureReason != nil {
		return *failureReason
	}

	// Fall back to status mapping
	_, failureReason = m.MapMidtransStatus(midtransStatus)
	if failureReason != nil {
		return *failureReason
	}

	return repository.FailureReasonUnknown
}

// IsPaymentSuccess returns true if the Midtrans status indicates success.
func (m *GatewayFailureMapper) IsPaymentSuccess(midtransStatus string) bool {
	status, _ := m.MapMidtransStatus(midtransStatus)
	return status == repository.PaymentAttemptStatusSuccess
}

// IsPaymentFailed returns true if the Midtrans status indicates failure.
func (m *GatewayFailureMapper) IsPaymentFailed(midtransStatus string) bool {
	status, _ := m.MapMidtransStatus(midtransStatus)
	return status == repository.PaymentAttemptStatusFailed ||
		status == repository.PaymentAttemptStatusCancelled ||
		status == repository.PaymentAttemptStatusTimeout
}

// IsPaymentPending returns true if the Midtrans status indicates pending.
func (m *GatewayFailureMapper) IsPaymentPending(midtransStatus string) bool {
	status, _ := m.MapMidtransStatus(midtransStatus)
	return status == repository.PaymentAttemptStatusPending
}

// CalculateTimeInPaymentSeconds calculates the time spent in payment flow.
// This should be called when payment is completed (success or failure).
func (m *GatewayFailureMapper) CalculateTimeInPaymentSeconds(
	attemptCreatedAt string,
	paymentCompletedAt string,
) *int {
	// This is a placeholder for time calculation
	// In actual implementation, parse the timestamps and calculate difference
	// For now, return nil as this requires proper time parsing
	return nil
}

// GetPaymentAttemptSummary returns a human-readable summary of payment attempt.
func (m *GatewayFailureMapper) GetPaymentAttemptSummary(
	status string,
	failureReason *string,
) string {
	switch status {
	case repository.PaymentAttemptStatusSuccess:
		return "Payment completed successfully"

	case repository.PaymentAttemptStatusPending:
		return "Payment awaiting completion"

	case repository.PaymentAttemptStatusCancelled:
		return "Payment cancelled by user"

	case repository.PaymentAttemptStatusTimeout:
		return "Payment expired"

	case repository.PaymentAttemptStatusFailed:
		if failureReason != nil {
			switch *failureReason {
			case repository.FailureReasonUserCancelled:
				return "Payment cancelled by user"
			case repository.FailureReasonGatewayDenied:
				return "Payment denied by payment provider"
			case repository.FailureReasonNetworkError:
				return "Payment failed due to network error"
			case repository.FailureReasonTimeout:
				return "Payment timed out"
			case repository.FailureReasonUnknown:
				return "Payment failed for unknown reason"
			default:
				return fmt.Sprintf("Payment failed: %s", *failureReason)
			}
		}
		return "Payment failed"

	default:
		return fmt.Sprintf("Payment status: %s", status)
	}
}

// strPtr returns a pointer to the given string.
func strPtr(s string) *string {
	return &s
}


