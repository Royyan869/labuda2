package worker

// CriticalWorkerStatus describes the runtime activation state of a single
// money-safety-critical detector worker, for observability purposes
// (PASS_18R). It is derived from the exact same env-parsing functions used
// to actually construct/start the worker, so this can never drift from what
// is really running.
type CriticalWorkerStatus struct {
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	ShadowMode bool   `json:"shadow_mode"`
	Critical   bool   `json:"critical"`
	// Status is one of "active" (running, alerts live), "shadow" (running,
	// alerts suppressed), or "dark" (not running at all).
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func criticalWorkerStatusFrom(name string, disabled, shadowMode bool) CriticalWorkerStatus {
	enabled := !disabled
	status := "dark"
	reason := name + " is disabled — no escrow/ledger drift detection is running"

	if enabled {
		if shadowMode {
			status = "shadow"
			reason = name + " is running in shadow mode — drift is logged but alerts are suppressed"
		} else {
			status = "active"
			reason = name + " is active — detected drift raises alerts"
		}
	}

	return CriticalWorkerStatus{
		Name:       name,
		Enabled:    enabled,
		ShadowMode: shadowMode,
		Critical:   true,
		Status:     status,
		Reason:     reason,
	}
}

// EscrowIntegrityWorkerStatus reports the current activation state of
// EscrowIntegrityWorker.
func EscrowIntegrityWorkerStatus() CriticalWorkerStatus {
	cfg := ParseEscrowIntegrityConfig()
	return criticalWorkerStatusFrom("EscrowIntegrityWorker", cfg.Disabled, cfg.ShadowMode)
}

// TotalMoneyInvariantWorkerStatus reports the current activation state of
// TotalMoneyInvariantWorker.
func TotalMoneyInvariantWorkerStatus() CriticalWorkerStatus {
	cfg := ParseTotalMoneyInvariantConfig()
	return criticalWorkerStatusFrom("TotalMoneyInvariantWorker", cfg.Disabled, cfg.ShadowMode)
}

// CriticalWorkerStatuses returns the status of every money-safety-critical
// detector worker tracked for runtime observability. PASS_18R scope is
// limited to EscrowIntegrityWorker and TotalMoneyInvariantWorker; other
// workers are out of scope for this status surface.
func CriticalWorkerStatuses() []CriticalWorkerStatus {
	return []CriticalWorkerStatus{
		EscrowIntegrityWorkerStatus(),
		TotalMoneyInvariantWorkerStatus(),
	}
}

// AnyCriticalWorkerDark reports whether any critical worker in the given set
// is fully disabled (not even running in shadow mode). Shadow mode itself is
// not "dark" — it is a deliberate, visible staged-activation state.
func AnyCriticalWorkerDark(statuses []CriticalWorkerStatus) bool {
	for _, s := range statuses {
		if !s.Enabled {
			return true
		}
	}
	return false
}
