package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/labuda/backend/internal/platform/logger"
	midtransPayout "github.com/labuda/backend/pkg/midtrans"
	"go.uber.org/zap"
)

// ============================================================================
// MIDTRANS PAYOUT GATEWAY ADAPTER
// ============================================================================
//
// This adapter bridges the Midtrans PayoutClient with our internal PayoutGateway interface.
// It maps between our internal request/response formats and Midtrans-specific formats.
//
// DESIGN DECISIONS:
// - Amount unit: internal PayoutGatewayRequest.Amount is a Rupiah integer
//   (PASS_18H canonical unit — no cents/sen subunit). Sent to Iris as-is.
// - Bank code mapping: uses Midtrans NormalizeBankCode for compatibility
// - Status mapping: maps Midtrans statuses to internal gateway statuses
// - Error classification: determines if errors are retryable or permanent
//
// ============================================================================

// MidtransPayoutGatewayConfig configures the Midtrans Iris payout gateway.
type MidtransPayoutGatewayConfig struct {
	// IrisOperatorKey is the Midtrans Iris operator credential.
	// NOT the same as MIDTRANS_SERVER_KEY — Iris uses separate credentials.
	// Required when SimulateMode=false. Missing key → explicit error at init.
	IrisOperatorKey string

	// IrisApproverKey is used to approve queued payouts via POST /payouts/approve.
	// Optional if the Iris account has auto-approve enabled.
	IrisApproverKey string

	// IsProduction indicates if using production Iris URLs.
	// Production gate requires PAYOUT_ENABLE_PRODUCTION=true in config.
	IsProduction bool

	// SimulateMode enables simulation without real API calls (for testing/mock gateway).
	// When true, Iris credentials are not required.
	SimulateMode bool
}

// MidtransPayoutGateway implements PayoutGateway using Midtrans Disbursement API
type MidtransPayoutGateway struct {
	client *midtransPayout.PayoutClient
	config MidtransPayoutGatewayConfig
	log    *zap.Logger
}

// NewMidtransPayoutGateway creates a Midtrans Iris payout gateway.
// Returns an error if Iris credentials are missing and SimulateMode is false.
func NewMidtransPayoutGateway(config MidtransPayoutGatewayConfig, log *zap.Logger) (*MidtransPayoutGateway, error) {
	if log == nil {
		log = zap.NewNop()
	}

	// Fail-closed: require Iris credentials when not in simulation mode.
	// Mid-server-* keys are NOT accepted by Iris (proven HTTP 401, TASK 58).
	if !config.SimulateMode && config.IrisOperatorKey == "" {
		return nil, fmt.Errorf("Midtrans Iris credentials missing: MIDTRANS_IRIS_OPERATOR_KEY must be set when GatewayProvider=midtrans_payout and SimulateMode=false")
	}

	payoutCfg := &midtransPayout.PayoutClientConfig{
		IrisOperatorKey: config.IrisOperatorKey,
		IsProduction:    config.IsProduction,
	}

	wrapperLog := &logger.Logger{Logger: log}
	client := midtransPayout.NewPayoutClient(payoutCfg, wrapperLog)

	return &MidtransPayoutGateway{
		client: client,
		config: config,
		log:    log,
	}, nil
}

// SubmitPayout submits a payout request to Midtrans
//
// Implements PayoutGateway.SubmitPayout
//
// ACTIVATION GUARD: This gateway is experimental and should only be used
// with explicit operator awareness. In simulation mode, it's safe for testing.
// Outside simulation mode, it requires proper configuration and verification.
func (m *MidtransPayoutGateway) SubmitPayout(ctx context.Context, req PayoutGatewayRequest) (*PayoutGatewayResponse, error) {
	m.log.Info("MidtransPayoutGateway: submitting payout",
		zap.String("external_ref", req.ExternalReferenceID),
		zap.Int64("amount", req.Amount),
		zap.String("bank_code", req.BankName),
		zap.Bool("simulate_mode", m.config.SimulateMode),
		zap.Bool("production_mode", m.config.IsProduction),
	)

	// If simulation mode is enabled, return simulated response
	if m.config.SimulateMode {
		return m.simulatePayout(ctx, req)
	}

	// Production gate: hard-block unless explicitly enabled
	if !m.config.SimulateMode && m.config.IsProduction {
		m.log.Error("MidtransPayoutGateway: production payout blocked — not verified for production use",
			zap.String("external_ref", req.ExternalReferenceID),
		)
		return nil, fmt.Errorf("midtrans Iris production payout not enabled: set PAYOUT_ENABLE_PRODUCTION=true and verify against production Iris endpoint first")
	}

	if !m.config.SimulateMode {
		m.log.Warn("MidtransPayoutGateway: submitting to real Iris sandbox",
			zap.String("external_ref", req.ExternalReferenceID),
			zap.String("iris_base", "app.sandbox.midtrans.com/iris/api/v1"),
		)
	}

	// req.Amount is already a Rupiah integer (PASS_18H) — Iris expects amount as a
	// plain full-IDR-unit string, so no conversion is needed here.
	amountIDR := fmt.Sprintf("%d", req.Amount)

	// Normalize bank code to Iris beneficiary_bank format
	bankCode := midtransPayout.NormalizeBankCode(req.BankName)

	irisItem := &midtransPayout.IrisPayoutItem{
		ExternalID:         req.ExternalReferenceID,
		Amount:             amountIDR,
		BeneficiaryBank:    bankCode,
		BeneficiaryAccount: req.AccountNumber,
		BeneficiaryName:    req.AccountHolder,
		Notes:              "Seller payout disbursement",
	}

	if email, ok := req.Metadata["email"]; ok {
		irisItem.BeneficiaryEmail = email
	}
	if phone, ok := req.Metadata["phone"]; ok {
		irisItem.PhoneNumber = phone
	}

	midtransResp, err := m.client.SubmitPayout(ctx, irisItem)
	if err != nil {
		m.log.Error("MidtransPayoutGateway: Iris submission failed",
			zap.String("external_ref", req.ExternalReferenceID),
			zap.Error(err),
		)
		return nil, err
	}

	return m.mapResponse(midtransResp)
}

// mapResponse converts an Iris PayoutResponse to internal PayoutGatewayResponse.
func (m *MidtransPayoutGateway) mapResponse(resp *midtransPayout.PayoutResponse) (*PayoutGatewayResponse, error) {
	gatewayResp := &PayoutGatewayResponse{
		// Iris uses reference_no as the gateway-assigned ID; no separate "id" field
		GatewayReferenceID: resp.ReferenceNo,
		Status:             PayoutResponseStatus(resp.MapToGatewayStatus()),
		ErrorType:          PayoutErrorType(resp.GetErrorType()),
	}

	rawJSON, err := json.Marshal(resp)
	if err != nil {
		m.log.Warn("Failed to marshal Iris response", zap.Error(err))
		gatewayResp.RawResponse = fmt.Sprintf("ERROR: %v", err)
	} else {
		gatewayResp.RawResponse = string(rawJSON)
	}

	// Map Iris status strings to human-readable messages
	// Iris status values: "queued", "processed", "failed" (UNVERIFIED at runtime)
	switch resp.Status {
	case "queued":
		gatewayResp.Message = "Payout queued in Midtrans Iris, awaiting approval"
	case "processed":
		gatewayResp.Message = "Payout processed by Midtrans Iris"
	case "failed":
		gatewayResp.Message = resp.FailedReason
		if gatewayResp.Message == "" && len(resp.ErrorMessages) > 0 {
			gatewayResp.Message = resp.ErrorMessages[0]
		}
	}

	m.log.Debug("MidtransPayoutGateway: Iris response mapped",
		zap.String("iris_status", resp.Status),
		zap.String("gateway_status", string(gatewayResp.Status)),
		zap.String("reference_no", gatewayResp.GatewayReferenceID),
		zap.String("error_type", string(gatewayResp.ErrorType)),
	)

	return gatewayResp, nil
}

// simulatePayout simulates a payout without making real API calls
// This is used for testing when SimulateMode is enabled
func (m *MidtransPayoutGateway) simulatePayout(ctx context.Context, req PayoutGatewayRequest) (*PayoutGatewayResponse, error) {
	// Simulate network latency
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}

	// Check for invalid bank codes (even in simulation)
	bankCode := midtransPayout.NormalizeBankCode(req.BankName)
	if !midtransPayout.IsValidBankCode(bankCode) {
		return &PayoutGatewayResponse{
			Status:             PayoutResponseStatusFailed,
			GatewayReferenceID: "SIM_" + req.ExternalReferenceID,
			Message:            fmt.Sprintf("Invalid bank code: %s", req.BankName),
			ErrorType:          ErrorTypePermanent,
			RawResponse:        `{"simulated": true, "reason": "invalid_bank_code"}`,
		}, nil
	}

	// Simulate success
	resp := &PayoutGatewayResponse{
		Status:             PayoutResponseStatusSuccess,
		GatewayReferenceID: "SIM_MIDTRANS_" + req.ExternalReferenceID,
		Message:            "Payout submitted successfully (simulated)",
		ErrorType:          "",
		RawResponse: fmt.Sprintf(`{
			"simulated": true,
			"external_id": "%s",
			"status": "PENDING",
			"amount": %d,
			"bank_code": "%s"
		}`, req.ExternalReferenceID, req.Amount, bankCode),
	}

	m.log.Info("MidtransPayoutGateway: simulated payout",
		zap.String("external_ref", req.ExternalReferenceID),
		zap.String("gateway_ref", resp.GatewayReferenceID),
	)

	return resp, nil
}

// ============================================================================
// STATUS CHECK (for reconciliation)
// ============================================================================

// GetPayoutStatus checks the status of a payout by external reference ID
// This is used by the reconciliation worker
func (m *MidtransPayoutGateway) GetPayoutStatus(ctx context.Context, externalRef string) (*PayoutStatusCheck, error) {
	if m.config.SimulateMode {
		return &PayoutStatusCheck{
			ExternalReferenceID: externalRef,
			Status:              "PENDING",
			RawResponse:         `{"simulated": true}`,
		}, nil
	}

	statusResp, err := m.client.GetPayoutStatus(ctx, externalRef)
	if err != nil {
		return nil, err
	}

	return &PayoutStatusCheck{
		ExternalReferenceID: statusResp.ExternalID,
		GatewayReferenceID:  statusResp.ReferenceNo,
		Status:              mapMidtransStatus(statusResp.Status),
		// Amount is a string in Iris; omit cents conversion — not used by reconciliation
		RawResponse: fmt.Sprintf("%+v", statusResp),
	}, nil
}

// PayoutStatusCheck represents the result of a status check
type PayoutStatusCheck struct {
	ExternalReferenceID string
	GatewayReferenceID  string
	Status              string
	Amount              int64
	RawResponse         string
}

// mapMidtransStatus maps Midtrans status to internal status
func mapMidtransStatus(status string) string {
	// Iris status values (UNVERIFIED at runtime — Iris credentials unavailable in TASK 58)
	switch status {
	case "processed":
		return "SETTLED"
	case "queued":
		return "SUBMITTED"
	case "failed":
		return "FAILED_FINAL"
	default:
		return "SUBMITTED"
	}
}

// ============================================================================
// BANK CODE VALIDATION
// ============================================================================

// IsValidBankCode checks if a bank code is supported by Midtrans
func (m *MidtransPayoutGateway) IsValidBankCode(bankCode string) bool {
	return midtransPayout.IsValidBankCode(bankCode)
}

// GetSupportedBankCodes returns a list of supported bank codes
func (m *MidtransPayoutGateway) GetSupportedBankCodes() []string {
	return midtransPayout.GetSupportedBankCodes()
}

// ============================================================================
// HEALTH CHECK
// ============================================================================

// HealthCheck returns the health status of the Midtrans gateway
func (m *MidtransPayoutGateway) HealthCheck(ctx context.Context) error {
	// In simulation mode, always healthy
	if m.config.SimulateMode {
		return nil
	}

	// In production, we could do a lightweight API call here
	// For now, just return nil as the circuit breaker handles failures
	return nil
}

// GetStatus returns the current status of the gateway
//
// HONESTY: This method returns accurate information about the verification status
// of the Midtrans payout integration. Operators should NOT treat this gateway
// as production-ready until all assumptions are verified.
func (m *MidtransPayoutGateway) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"type":                 "midtrans_payout",
		"provider":             "midtrans",
		"disbursement":         true,
		"production":           m.config.IsProduction,
		"simulate_mode":        m.config.SimulateMode,
		"verification_status":  "unverified", // CRITICAL: Not yet verified against real Midtrans API
		"experimental":         true,
		"safe_for_production":  false,
	}

	// Verification items updated from TASK 58/59 runtime evidence
	verificationItems := map[string]interface{}{
		"iris_base_url_sandbox":   "RUNTIME-PROVEN: app.sandbox.midtrans.com/iris/api/v1",
		"iris_endpoint_payouts":   "RUNTIME-PROVEN: /payouts returns 401 (exists, needs Iris key)",
		"auth_mechanism":          "RUNTIME-PROVEN: Basic auth format correct; Iris key type required",
		"iris_operator_key_set":   m.config.IrisOperatorKey != "",
		"iris_approver_key_set":   m.config.IrisApproverKey != "",
		"request_schema":          "PARTIALLY-VALIDATED: Iris field names from spec (beneficiary_name/account/bank)",
		"status_values":           "UNVERIFIED: queued/processed/failed from docs; not runtime-confirmed",
		"webhook_signature":       "UNVERIFIED: Iris webhook signature mechanism unknown",
		"production_url":          "ASSUMED from sandbox pattern — not runtime-verified",
	}
	status["verification_items"] = verificationItems

	warnings := []string{
		"Iris status values (queued/processed/failed) not runtime-verified — Iris credentials required",
		"Production URL assumed from sandbox pattern — verify before enabling production",
		"Webhook signature mechanism for Iris not confirmed",
		"DO NOT enable production without completing sandbox validation with real Iris credentials",
	}
	status["warnings"] = warnings

	return status
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// ValidatePayoutRequest validates a payout request before submission
// Returns an error if the request is invalid
func ValidatePayoutRequest(req PayoutGatewayRequest) error {
	if req.ExternalReferenceID == "" {
		return fmt.Errorf("external_reference_id is required")
	}

	if req.Amount <= 0 {
		return fmt.Errorf("amount must be positive: %d", req.Amount)
	}

	if req.AccountNumber == "" {
		return fmt.Errorf("account_number is required")
	}

	// Minimum amount for Midtrans payout is typically 10,000 IDR
	minAmountIDR := int64(10000) // 10,000 IDR (Rupiah integer, PASS_18H)
	if req.Amount < minAmountIDR {
		return fmt.Errorf("amount must be at least 10,000 IDR: %d", req.Amount)
	}

	// Bank code validation is optional here - will be validated by gateway
	return nil
}


