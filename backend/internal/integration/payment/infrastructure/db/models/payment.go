package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

// PaymentDB represents the payments table
// GORM tags removed - migration now handled via SQL
type PaymentDB struct {
	ID                 uuid.UUID   `json:"id"`
	UserID             uuid.UUID   `json:"user_id"`
	PaymentNumber      string      `json:"payment_number"`
	MidtransOrderID    string      `json:"midtrans_order_id"`
	GrossAmount        money.Money `json:"gross_amount"`
	ServiceFeeAmount   money.Money `json:"service_fee_amount"`
	CoinsToUse         int         `json:"coins_to_use"`
	CoinDiscountAmount money.Money `json:"coin_discount_amount"`
	Status             string      `json:"status"`
	ReferenceType      string      `json:"reference_type"` // order, auction, collection, etc.
	ReferenceID        *uuid.UUID  `json:"reference_id,omitempty"`
	PriceSnapshotID    *uuid.UUID  `json:"price_snapshot_id,omitempty"`

	// Midtrans response fields
	PaymentURL    *string `json:"payment_url,omitempty"`
	TransactionID *string `json:"transaction_id,omitempty"`
	PaymentType   *string `json:"payment_type,omitempty"`

	// Timestamps
	PaidAt    *time.Time `json:"paid_at,omitempty"`
	ExpiredAt *time.Time `json:"expired_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// TableName specifies the table name for PaymentDB
func (PaymentDB) TableName() string {
	return "payments"
}

// PaymentWebhookEventDB represents the payment_webhook_events table
// This is the NEW financial-grade webhook event tracking model
// GORM tags removed - migration now handled via SQL
type PaymentWebhookEventDB struct {
	ID              uuid.UUID  `json:"id"`
	Provider        string     `json:"provider"`
	EventID         string     `json:"event_id"`
	MidtransOrderID *string    `json:"midtrans_order_id,omitempty"`
	PaymentID       *uuid.UUID `json:"payment_id,omitempty"`
	SignatureKey    string     `json:"signature_key"`
	Payload         []byte     `json:"payload"`
	Status          string     `json:"status"` // pending, processing, succeeded, failed, orphaned, manual_review, quarantined, terminal_review
	ErrorMessage    *string    `json:"error_message,omitempty"`
	ReceivedAt      time.Time  `json:"received_at"`
	ProcessedAt     *time.Time `json:"processed_at,omitempty"`
}

// TableName specifies the table name for PaymentWebhookEventDB
func (PaymentWebhookEventDB) TableName() string {
	return "payment_webhook_events"
}

// IsSucceeded returns true if the webhook event was processed successfully
func (e *PaymentWebhookEventDB) IsSucceeded() bool {
	return e.Status == "succeeded"
}

// IsPending returns true if the webhook event is pending processing
func (e *PaymentWebhookEventDB) IsPending() bool {
	return e.Status == "pending"
}

// IsFailed returns true if the webhook event processing failed
func (e *PaymentWebhookEventDB) IsFailed() bool {
	return e.Status == "failed"
}

// MarkAsProcessing marks the event as being processed
func (e *PaymentWebhookEventDB) MarkAsProcessing() {
	e.Status = "processing"
}

// MarkAsSucceeded marks the event as successfully processed
func (e *PaymentWebhookEventDB) MarkAsSucceeded() {
	now := time.Now()
	e.Status = "succeeded"
	e.ProcessedAt = &now
}

// MarkAsFailed marks the event as failed with an error message
func (e *PaymentWebhookEventDB) MarkAsFailed(errMsg string) {
	now := time.Now()
	e.Status = "failed"
	e.ProcessedAt = &now
	e.ErrorMessage = &errMsg
}

// MarkAsOrphaned marks the event as orphaned (payment not found)
func (e *PaymentWebhookEventDB) MarkAsOrphaned(reason string) {
	now := time.Now()
	e.Status = "orphaned"
	e.ProcessedAt = &now
	e.ErrorMessage = &reason
}

// MarkAsManualReview marks the event as requiring manual review.
func (e *PaymentWebhookEventDB) MarkAsManualReview(reason string) {
	now := time.Now()
	e.Status = "manual_review"
	e.ProcessedAt = &now
	e.ErrorMessage = &reason
}

// MarkAsQuarantined marks the event as quarantined due to malformed payload.
func (e *PaymentWebhookEventDB) MarkAsQuarantined(reason string) {
	now := time.Now()
	e.Status = "quarantined"
	e.ProcessedAt = &now
	e.ErrorMessage = &reason
}

// MarkAsTerminalReview marks the event as terminally failed and waiting for manual review.
func (e *PaymentWebhookEventDB) MarkAsTerminalReview(reason string) {
	now := time.Now()
	e.Status = "terminal_review"
	e.ProcessedAt = &now
	e.ErrorMessage = &reason
}
