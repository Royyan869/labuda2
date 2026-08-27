package worker

// ============================================================================
// MIDTRANS PAYOUT ADAPTER — SANDBOX VALIDATION CLASSIFICATION
// ============================================================================
//
// This file documents the verification status of every capability in the
// Midtrans payout adapter (midtrans_payout_gateway.go + pkg/midtrans/payout_client.go).
//
// Classification levels:
//   A — runtime-proven:       executed successfully against real Midtrans sandbox API
//   B — partially validated:  code reviewed, struct shapes match docs, NOT runtime-tested
//   C — unverified:           assumed correct from docs; untested against real API
//   D — blocked/dangerous:    guarded by safety gate; must not be called in production
//
// LIVE SANDBOX VALIDATION STATUS: BLOCKED
// Reason: No sandbox credentials (MIDTRANS_SERVER_KEY) available in this environment.
//         No network access configured for api.sandbox.midtrans.com.
//
// CONCLUSION: Midtrans payout adapter is NOT safe for production or unguarded staging.
//             Manual admin payout remains the ONLY safe payout rail.
// ============================================================================

// MidtransCapabilityLevel classifies validation depth.
type MidtransCapabilityLevel string

const (
	MidtransLevelRuntimeProven      MidtransCapabilityLevel = "A_RUNTIME_PROVEN"
	MidtransLevelPartiallyValidated MidtransCapabilityLevel = "B_PARTIALLY_VALIDATED"
	MidtransLevelUnverified         MidtransCapabilityLevel = "C_UNVERIFIED"
	MidtransLevelBlocked            MidtransCapabilityLevel = "D_BLOCKED_DANGEROUS"
)

// MidtransCapabilityClassification documents one adapter capability.
type MidtransCapabilityClassification struct {
	Capability     string                  `json:"capability"`
	Level          MidtransCapabilityLevel `json:"level"`
	Status         string                  `json:"status"`
	Blocker        string                  `json:"blocker,omitempty"`
	Notes          string                  `json:"notes"`
}

// GetMidtransAdapterClassifications returns the full capability audit.
// This is the authoritative record of what is and is not verified.
func GetMidtransAdapterClassifications() []MidtransCapabilityClassification {
	return []MidtransCapabilityClassification{
		{
			Capability: "authentication",
			Level:      MidtransLevelUnverified,
			Status:     "unverified",
			Blocker:    "No MIDTRANS_SERVER_KEY sandbox credential in environment",
			Notes: "Implemented as HTTP Basic Auth: serverKey as username, empty password. " +
				"Matches Midtrans v2 API documentation. Not executed against real API.",
		},
		{
			Capability: "request_signing",
			Level:      MidtransLevelUnverified,
			Status:     "unverified",
			Blocker:    "Cannot verify without network access to sandbox",
			Notes: "Midtrans v2 disbursement does not use HMAC request signing " +
				"(unlike payment collection which uses signature_key). " +
				"This assumption has NOT been confirmed against the live sandbox.",
		},
		{
			Capability: "idempotency_semantics",
			Level:      MidtransLevelUnverified,
			Status:     "unverified",
			Blocker:    "Midtrans deduplication behavior on duplicate external_id not confirmed",
			Notes: "ExternalID is sent in the request body as idempotency key. " +
				"Assumption: re-submitting same external_id returns existing record. " +
				"If Midtrans creates a duplicate instead, double-payout risk exists. " +
				"MUST be confirmed in sandbox before staging promotion.",
		},
		{
			Capability: "response_parsing",
			Level:      MidtransLevelPartiallyValidated,
			Status:     "partially_validated",
			Notes: "PayoutResponse struct covers documented fields: status, id, external_id, " +
				"amount, bank_code, account_number, transaction_time, error_messages. " +
				"JSON field names match Midtrans API docs. " +
				"NOT validated against a real API response payload.",
		},
		{
			Capability: "timeout_and_retry",
			Level:      MidtransLevelPartiallyValidated,
			Status:     "partially_validated",
			Notes: "HTTP client timeout: 30s. Context cancellation propagated via NewRequestWithContext. " +
				"Circuit breaker: open after 5 failures, resets after 30s. " +
				"Real timeout and circuit breaker behavior under load NOT tested.",
		},
		{
			Capability: "error_classification",
			Level:      MidtransLevelPartiallyValidated,
			Status:     "partially_validated",
			Notes: "isPermanentError() matches: 'invalid account', 'account not found', " +
				"'account closed', 'account number', 'bank code', 'validation', 'unauthorized'. " +
				"Logic is reviewed and reasonable. " +
				"Actual Midtrans error message strings NOT confirmed against sandbox.",
		},
		{
			Capability: "bank_code_mapping",
			Level:      MidtransLevelPartiallyValidated,
			Status:     "partially_validated",
			Notes: "MidtransBankCodes covers BCA, Mandiri, BRI, BNI, CIMB, Permata, Danamon, " +
				"Muamalat, BSI, Jago, OCBC, BTN. Based on Midtrans documentation. " +
				"Completeness and case-sensitivity NOT confirmed against bank list API.",
		},
		{
			Capability: "status_mapping",
			Level:      MidtransLevelUnverified,
			Status:     "unverified",
			Blocker:    "Real Midtrans status values not confirmed",
			Notes: "Assumed mapping: PENDING→SUBMITTED, SUCCESS→SETTLED, FAILED→FAILED_FINAL. " +
				"Midtrans may emit additional statuses (PROCESSING, REVERSED, REFUNDED) " +
				"that currently fall through to SUBMITTED. " +
				"Status exhaustiveness NOT confirmed.",
		},
		{
			Capability: "endpoint_urls",
			Level:      MidtransLevelUnverified,
			Status:     "unverified",
			Blocker:    "No network access to api.sandbox.midtrans.com in this environment",
			Notes: "sandboxPayoutURL = https://api.sandbox.midtrans.com/v2/disbursements. " +
				"This is the documented URL but has NOT been reached via a real HTTP call.",
		},
		{
			Capability: "webhook_handling",
			Level:      MidtransLevelUnverified,
			Status:     "unverified",
			Blocker:    "Midtrans payout webhook payload format not confirmed",
			Notes: "webhook_handler.go implements HMAC-SHA256 signature verification. " +
				"The specific field Midtrans uses for signature hash in payout webhooks " +
				"has NOT been validated. May differ from payment webhook format.",
		},
		{
			Capability: "production_mode",
			Level:      MidtransLevelBlocked,
			Status:     "blocked_by_safety_guard",
			Notes: "SubmitPayout() returns error when IsProduction=true and SimulateMode=false: " +
				"'midtrans payout gateway is experimental and not verified for production use'. " +
				"This guard must remain until all C-level items above are promoted to A. " +
				"Manual admin payout is the only safe production rail.",
		},
	}
}

// GetMidtransAdapterSummary returns an operator-readable summary map.
func GetMidtransAdapterSummary() map[string]interface{} {
	items := GetMidtransAdapterClassifications()
	counts := map[string]int{
		string(MidtransLevelRuntimeProven):      0,
		string(MidtransLevelPartiallyValidated): 0,
		string(MidtransLevelUnverified):         0,
		string(MidtransLevelBlocked):            0,
	}
	var blockers []string
	for _, c := range items {
		counts[string(c.Level)]++
		if c.Blocker != "" {
			blockers = append(blockers, c.Capability+": "+c.Blocker)
		}
	}

	return map[string]interface{}{
		"overall_classification":      "NOT_SAFE_FOR_PRODUCTION",
		"overall_classification_note": "Zero capabilities are runtime-proven against real Midtrans sandbox",
		"capability_counts":           counts,
		"blockers":                    blockers,
		"safe_modes":                  []string{"SimulateMode=true (no real API calls)"},
		"required_before_staging_promotion": []string{
			"Obtain MIDTRANS_SERVER_KEY sandbox credential",
			"Confirm auth: POST to /disbursements returns 200 not 401",
			"Confirm idempotency: re-submit same external_id, verify no duplicate",
			"Confirm status mapping: capture PENDING, SUCCESS, FAILED responses",
			"Confirm error messages: trigger validation failure, confirm isPermanentError match",
			"Confirm bank_code format: test BCA and MANDIRI codes against sandbox",
			"Confirm webhook signature field for payout notifications",
		},
	}
}


