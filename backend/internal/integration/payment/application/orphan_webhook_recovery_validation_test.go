package application

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRecoveredWebhookPayload(t *testing.T) {
	referenceID := uuid.New()
	event := repository.OrphanWebhookEvent{
		EventID:         "evt-1",
		MidtransOrderID: "LAB-ORDER-001",
	}
	validPayment := &repository.Payment{
		ID:            uuid.New(),
		ReferenceType: repository.ReferenceTypeOrder,
		ReferenceID:   &referenceID,
	}
	validNotification := &midtrans.NotificationPayload{
		TransactionID:     "trx-1",
		OrderID:           "LAB-ORDER-001",
		PaymentType:       "bank_transfer",
		TransactionStatus: "settlement",
	}

	t.Run("valid payload", func(t *testing.T) {
		require.Nil(t, validateRecoveredWebhookPayload(event, validPayment, validNotification))
	})

	t.Run("missing transaction id", func(t *testing.T) {
		notification := *validNotification
		notification.TransactionID = ""

		err := validateRecoveredWebhookPayload(event, validPayment, &notification)
		require.NotNil(t, err)
		assert.Equal(t, "missing_transaction_id", err.issueType)
	})

	t.Run("missing order id", func(t *testing.T) {
		notification := *validNotification
		notification.OrderID = ""

		err := validateRecoveredWebhookPayload(event, validPayment, &notification)
		require.NotNil(t, err)
		assert.Equal(t, "missing_order_id", err.issueType)
	})

	t.Run("missing payment identifier", func(t *testing.T) {
		notification := *validNotification
		payment := *validPayment
		payment.ID = uuid.Nil

		err := validateRecoveredWebhookPayload(event, &payment, &notification)
		require.NotNil(t, err)
		assert.Equal(t, "missing_payment_identifier", err.issueType)
	})

	t.Run("missing required field", func(t *testing.T) {
		notification := *validNotification
		notification.PaymentType = ""

		err := validateRecoveredWebhookPayload(event, validPayment, &notification)
		require.NotNil(t, err)
		assert.Equal(t, "missing_required_field", err.issueType)
	})

	t.Run("invalid reference type", func(t *testing.T) {
		notification := *validNotification
		payment := *validPayment
		payment.ReferenceType = "unsupported"

		err := validateRecoveredWebhookPayload(event, &payment, &notification)
		require.NotNil(t, err)
		assert.Equal(t, "invalid_reference_type", err.issueType)
	})

	t.Run("semantically invalid payload", func(t *testing.T) {
		notification := *validNotification
		payment := *validPayment
		payment.ReferenceID = nil

		err := validateRecoveredWebhookPayload(event, &payment, &notification)
		require.NotNil(t, err)
		assert.Equal(t, "missing_payment_identifier", err.issueType)
	})
}


