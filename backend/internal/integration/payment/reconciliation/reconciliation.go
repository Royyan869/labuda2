package reconciliation

import "github.com/google/uuid"

// Outcome classifies the canonical reconciliation result.
type Outcome string

const (
	OutcomeSuccessFinalized Outcome = "success_finalized"
	OutcomeTerminalFailure  Outcome = "terminal_failure"
	OutcomeUncertain        Outcome = "uncertain"
	OutcomeAlreadyTerminal  Outcome = "already_terminal"
	OutcomeUnsupported      Outcome = "unsupported"
)

// Result is the typed outcome returned by the canonical reconciliation entrypoint.
type Result struct {
	PaymentID     uuid.UUID `json:"payment_id"`
	ReferenceType string    `json:"reference_type"`
	PaymentStatus string    `json:"payment_status"`
	GatewayStatus string    `json:"gateway_status"`
	TransactionID string    `json:"transaction_id"`
	Outcome       Outcome   `json:"outcome"`
	Notes         string    `json:"notes,omitempty"`
}
