package midtrans

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// ProviderState is the ONE canonical semantic state of a Midtrans transaction.
//
// WHY THIS TYPE EXISTS: the gateway wire format (transaction_status plus
// fraud_status) is raw provider data. Before this type, every settle-capable
// consumer (webhook, payment discovery worker, seller sync, dev replay)
// re-derived "is this paid?" from those raw strings, and they disagreed — the
// capture/fraud gate existed in some paths and not others. A raw status string
// must never be interpreted outside this package.
type ProviderState string

const (
	// ProviderStateSettled — the gateway holds the money. Only this state may
	// trigger payment settlement / finalization.
	ProviderStateSettled ProviderState = "settled"

	// ProviderStatePending — the gateway knows the transaction and it has not
	// reached a final success or failure (waiting for the customer, or a
	// capture still held by the fraud gate).
	ProviderStatePending ProviderState = "pending"

	// ProviderStateNotPresent — the gateway is reachable and has no record of
	// this order id. Deliberately NOT an error and never a terminal payment
	// failure: a Snap token exists before the customer opens the payment page,
	// so "no record yet" is a normal, transient answer.
	ProviderStateNotPresent ProviderState = "not_present"

	// ProviderStateFailed — the gateway refused the transaction
	// (deny / cancel / expire).
	ProviderStateFailed ProviderState = "failed"

	// ProviderStateUnknown — any status this contract does not define
	// (including refund / partial_refund, which describe money already
	// returned and are never a settlement or a failure signal). Consumers must
	// mutate nothing.
	ProviderStateUnknown ProviderState = "unknown"
)

// ClassifyProviderState maps the raw gateway status (plus fraud_status) onto the
// canonical provider state. THE ONLY PLACE THIS MAPPING MAY EXIST.
//
// The capture/fraud rule is part of the contract: Midtrans reports a card
// authorization as transaction_status=capture, and that capture is only final
// once fraud_status=accept. An accepted capture and a settlement both mean the
// money is ours; anything else is not.
func ClassifyProviderState(transactionStatus, fraudStatus string) ProviderState {
	switch strings.ToLower(strings.TrimSpace(transactionStatus)) {
	case string(StatusSettlement):
		return ProviderStateSettled
	case string(StatusCapture):
		if strings.EqualFold(strings.TrimSpace(fraudStatus), "accept") {
			return ProviderStateSettled
		}
		return ProviderStatePending
	case string(StatusPending):
		return ProviderStatePending
	case string(StatusDeny), string(StatusCancel), string(StatusExpire):
		return ProviderStateFailed
	default:
		return ProviderStateUnknown
	}
}

// ProviderState classifies a notification payload with the canonical contract.
func (n *NotificationPayload) ProviderState() ProviderState {
	if n == nil {
		return ProviderStateUnknown
	}
	return ClassifyProviderState(n.TransactionStatus, n.FraudStatus)
}

// ProviderStatus is the canonical result of a gateway transaction inquiry.
//
// Notification is non-nil exactly when the gateway returned a record
// (settled / pending / failed / unknown). It is nil for ProviderStateNotPresent.
type ProviderStatus struct {
	State        ProviderState
	Notification *NotificationPayload
}

// QueryProviderState performs the canonical, read-only gateway inquiry for one
// order id. It is the only supported way to ask the gateway about a
// transaction, so every consumer sees the same semantic state for the same
// response.
//
// Error contract: a non-nil error means the gateway could NOT be consulted
// (transport failure, circuit breaker open, or an unexpected provider
// response). It never means "not paid". A reachable gateway with no record of
// the order is a successful call returning ProviderStateNotPresent.
func (c *Client) QueryProviderState(orderID string) (*ProviderStatus, error) {
	// P0-11: Circuit breaker - fail fast if circuit is open
	if !c.cb.allowRequest() {
		c.log.Warn("Midtrans circuit breaker is open - failing fast",
			zap.String("state", c.cb.getState().String()),
		)
		return nil, fmt.Errorf("midtrans circuit breaker is %s - service unavailable", c.cb.getState())
	}

	url := fmt.Sprintf("%s/%s/status", c.getCoreAPIURL(), orderID)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(c.serverKey, "")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.cb.onFailure()
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.cb.onFailure()
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		// The gateway answered; it simply has no such order yet. This is a
		// successful inquiry, not a failure — treating it as one would let a
		// token that the customer has not opened look like a broken gateway.
		c.cb.onSuccess()
		return &ProviderStatus{State: ProviderStateNotPresent}, nil
	}

	if resp.StatusCode != http.StatusOK {
		c.cb.onFailure()
		return nil, &APIError{
			Operation:  "query_provider_state",
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var notification NotificationPayload
	if err := json.Unmarshal(body, &notification); err != nil {
		c.cb.onFailure()
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.cb.onSuccess()

	return &ProviderStatus{
		State:        notification.ProviderState(),
		Notification: &notification,
	}, nil
}

// ParseGrossAmount converts a gateway amount string ("91350.00") into the
// canonical whole-Rupiah integer. Midtrans's ".00" is decimal formatting of a
// whole Rupiah integer, never a cents subunit (PASS_18H), so there is no
// scaling in either direction. Unparseable input yields 0, which can never
// equal a real payment amount and therefore fails amount validation.
func ParseGrossAmount(amountStr string) int64 {
	trimmed := strings.TrimSpace(amountStr)
	if trimmed == "" {
		return 0
	}
	if i := strings.IndexByte(trimmed, '.'); i >= 0 {
		trimmed = trimmed[:i]
	}
	var amount int64
	if _, err := fmt.Sscanf(trimmed, "%d", &amount); err != nil {
		return 0
	}
	return amount
}
