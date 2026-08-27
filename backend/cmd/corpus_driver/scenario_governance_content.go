package main

// Governance content scenario — official-HTTP-only harness for /search/content
// enforce observation. See docs/operations or the B6.3 task for the doctrine.
//
// CONSTITUTIONAL POSTURE:
//   - Pure HTTP. The scenario does NOT use serverboot.InitServices, does NOT
//     touch the DB / Redis / Firebase / Midtrans clients directly, and does
//     NOT import any application package. It speaks only to the running
//     core_server over HTTP, exactly as a mobile client would.
//   - Every request and response is captured to disk under the run's
//     artifact directory. Nothing is lost between runs.
//   - The scenario is best-effort: if a step is blocked by a real production
//     invariant (e.g. email-verification gate), it records the blocker and
//     proceeds with the remaining observable steps. It NEVER fabricates a
//     state it could not legitimately produce.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// governanceContentConfig holds the flag-driven inputs for the scenario.
type governanceContentConfig struct {
	BaseURL   string
	OutputDir string
	Keyword   string
	Timeout   time.Duration
	Verbose   bool
}

// stepResult is the per-step observation row written into summary.json.
type stepResult struct {
	Step     string `json:"step"`
	Status   string `json:"status"` // ok | skipped | blocked | error
	HTTP     int    `json:"http,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Artifact string `json:"artifact,omitempty"` // path relative to OutputDir
}

// searchObservation captures the analytic facts extracted from one
// /search/content response body so the operator can answer the B6.8
// observation questions without re-parsing the raw JSON.
type searchObservation struct {
	HTTPStatus      int            `json:"http_status"`
	RowCount        int            `json:"row_count"`
	LifecycleTally  map[string]int `json:"lifecycle_tally"`
	ExpectedID      string         `json:"expected_id"`
	ExpectedIDFound bool           `json:"expected_id_found"`
	Note            string         `json:"note,omitempty"`
}

// summary is the top-level scenario report serialized to summary.json.
type summary struct {
	Scenario      string       `json:"scenario"`
	Started       string       `json:"started"`
	Finished      string       `json:"finished"`
	BaseURL       string       `json:"base_url"`
	Keyword       string       `json:"keyword"`
	RunID         string       `json:"run_id"`
	Verdict       string       `json:"verdict"`
	EvaluatorMode string       `json:"evaluator_mode_observed,omitempty"`
	Steps         []stepResult `json:"steps"`

	// B6.8 observation block — answers the explicit operator questions.
	ActiveSearchObservation  *searchObservation `json:"active_search_observation,omitempty"`
	DeletedSearchObservation *searchObservation `json:"deleted_search_observation,omitempty"`
	EnforcementAppliedTicked bool               `json:"enforcement_applied_ticked"`
	EnforcementAppliedNote   string             `json:"enforcement_applied_note,omitempty"`
	ShadowContinuityNote     string             `json:"shadow_continuity_note,omitempty"`
}

// runScenarioGovernanceContent is the entry point dispatched by main.go.
func runScenarioGovernanceContent(cfg governanceContentConfig) error {
	started := time.Now().UTC()
	runID := started.Format("20060102T150405Z")
	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join("scenario_logs", "governance-content-"+runID)
	}
	if cfg.Keyword == "" {
		cfg.Keyword = "labudagov" + runID
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir %s: %w", cfg.OutputDir, err)
	}

	sum := &summary{
		Scenario: "scenario-governance-content",
		Started:  started.Format(time.RFC3339),
		BaseURL:  cfg.BaseURL,
		Keyword:  cfg.Keyword,
		RunID:    runID,
		Verdict:  "PARTIAL",
		Steps:    make([]stepResult, 0, 16),
	}

	client := &http.Client{Timeout: cfg.Timeout}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout*8)
	defer cancel()

	// ─── preflight ────────────────────────────────────────────────────────
	if step := preflight(ctx, client, cfg); step.Status == "error" {
		sum.Steps = append(sum.Steps, step)
		sum.Verdict = "BLOCKED_PREFLIGHT"
		return finishSummary(sum, cfg, "preflight failed: "+step.Reason)
	} else {
		sum.Steps = append(sum.Steps, step)
	}

	// ─── metrics baseline ─────────────────────────────────────────────────
	sum.Steps = append(sum.Steps, captureMetrics(ctx, client, cfg, "metrics_before.txt"))
	sum.EvaluatorMode = sniffEvaluatorMode(filepath.Join(cfg.OutputDir, "metrics_before.txt"))

	// ─── auth: author ─────────────────────────────────────────────────────
	// `-verified-` in the mock bearer string opts the dev mock Firebase
	// verifier into emitting `email_verified: true`. See
	// pkg/firebase/firebase.go VerifyIDTokenMock + isVerifiedMockToken
	// (Batch B6.4). The downstream auth handler writes email_verified_at
	// at user-create time so RequireInteractionAuthority later passes for
	// content creation. Real Firebase auth never reaches this path —
	// AuthClient is nil only in DEV_MOCK_FIREBASE_AUTH=true mode.
	authorToken := "governance-author-verified-" + runID
	authStep, authorID := mintToken(ctx, client, cfg, authorToken, "auth_author.json", "governanceauthor"+shortID(runID))
	sum.Steps = append(sum.Steps, authStep)
	if authStep.Status != "ok" {
		return finishSummary(sum, cfg, "author auth failed")
	}

	// ─── auth: viewer ─────────────────────────────────────────────────────
	// Verified viewer keeps the corpus symmetrical and lets us add viewer-
	// authored interaction scenarios in future without re-minting.
	viewerToken := "governance-viewer-verified-" + runID
	viewerStep, _ := mintToken(ctx, client, cfg, viewerToken, "auth_viewer.json", "governanceviewer"+shortID(runID))
	sum.Steps = append(sum.Steps, viewerStep)
	if viewerStep.Status != "ok" {
		return finishSummary(sum, cfg, "viewer auth failed")
	}

	sum.Steps = append(sum.Steps, captureMetrics(ctx, client, cfg, "metrics_after_auth.txt"))

	// ─── create content (author) ──────────────────────────────────────────
	// Best-effort: this commonly blocks on EMAIL_VERIFICATION_REQUIRED for
	// brand-new mock users, which is a real production invariant (the
	// scenario must NOT fake around it). We try and record exactly what
	// happens so the operator can decide whether to follow up with an
	// email-verification helper batch.
	createStep, _ := createContent(ctx, client, cfg, authorToken, "create_content_1.json")
	sum.Steps = append(sum.Steps, createStep)

	sum.Steps = append(sum.Steps, captureMetrics(ctx, client, cfg, "metrics_after_create.txt"))

	createdID := ""
	if createStep.Status == "ok" {
		createdID = createStep.Detail
	}

	// ─── active search (BEFORE delete) ────────────────────────────────────
	// B6.8: search the keyword while the created row is still active so we
	// can record the live lifecycle value the server actually emits for an
	// active ContentCard. mustBePresent=true classifies the step as
	// "blocked" if the expected id is absent (which would mean projection
	// or evaluator silently dropped an active row — a real signal).
	activeStep, activeObs := searchContent(
		ctx, client, cfg, viewerToken,
		"search_content_active", "search_content_active.json",
		createdID, true,
	)
	sum.Steps = append(sum.Steps, activeStep)
	sum.ActiveSearchObservation = activeObs
	sum.Steps = append(sum.Steps, captureMetrics(ctx, client, cfg, "metrics_after_search_active.txt"))

	// ─── delete + post-delete metrics snapshot ────────────────────────────
	// The doctrine forbids fabricating moderation/deletion via SQL. The only
	// canonical content-state mutation routes available WITHOUT admin
	// capability are: DELETE /api/v1/contents/:id (owner-only). Block /
	// moderate / redact require admin or are not yet wired as official
	// dev-accessible routes; we explicitly skip them.
	if createStep.Status == "ok" && createdID != "" {
		delStep := deleteContent(ctx, client, cfg, authorToken, createdID)
		sum.Steps = append(sum.Steps, delStep)
	} else {
		sum.Steps = append(sum.Steps, stepResult{
			Step:   "delete_content",
			Status: "skipped",
			Reason: "no content was created in this run",
		})
	}
	sum.Steps = append(sum.Steps, captureMetrics(ctx, client, cfg, "metrics_after_delete.txt"))
	sum.Steps = append(sum.Steps, stepResult{
		Step:   "moderate_or_redact_content",
		Status: "skipped",
		Reason: "no official non-admin route to flip lifecycle to unavailable; admin moderation route not exercised in this batch",
	})
	sum.Steps = append(sum.Steps, stepResult{
		Step:   "block_user_relation",
		Status: "skipped",
		Reason: "block-user official route not exercised in this batch; out of scope",
	})

	// ─── deleted search (AFTER delete) ────────────────────────────────────
	// B6.8: re-run the same keyword query. mustBePresent=false records
	// "blocked" status if the deleted row is still visible (a real signal
	// either of evaluator semantics or of soft-delete projection drift).
	// Absence is acceptable and expected — legacy SQL filters deleted rows
	// out at projection per the further-restrict-only doctrine.
	deletedStep, deletedObs := searchContent(
		ctx, client, cfg, viewerToken,
		"search_content_deleted", "search_content_deleted.json",
		createdID, false,
	)
	sum.Steps = append(sum.Steps, deletedStep)
	sum.DeletedSearchObservation = deletedObs
	sum.Steps = append(sum.Steps, captureMetrics(ctx, client, cfg, "metrics_after_search_deleted.txt"))

	// ─── post-run observation synthesis ──────────────────────────────────
	// Update the evaluator-mode sniff against the final scrape (the
	// pre-search snapshot at B6.6 was always empty because CounterVec
	// label combinations only appear after their first .Inc()).
	if late := sniffEvaluatorMode(filepath.Join(cfg.OutputDir, "metrics_after_search_deleted.txt")); late != "" {
		sum.EvaluatorMode = late
	}
	sum.EnforcementAppliedTicked, sum.EnforcementAppliedNote = sniffEnforcementApplied(
		filepath.Join(cfg.OutputDir, "metrics_before.txt"),
		filepath.Join(cfg.OutputDir, "metrics_after_search_deleted.txt"),
	)
	sum.ShadowContinuityNote = sniffShadowContinuity(
		filepath.Join(cfg.OutputDir, "metrics_after_search_active.txt"),
		filepath.Join(cfg.OutputDir, "metrics_after_search_deleted.txt"),
	)

	// Verdict synthesis ─────────────────────────────────────────────────────
	sum.Verdict = synthesizeVerdict(sum)
	_ = authorID
	return finishSummary(sum, cfg, "")
}

// preflight verifies the backend is reachable and ready before the scenario
// burns auth/state on it. Returns a step row; status="error" aborts the run.
func preflight(ctx context.Context, client *http.Client, cfg governanceContentConfig) stepResult {
	url := cfg.BaseURL + "/health/ready"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	res, err := client.Do(req)
	if err != nil {
		return stepResult{Step: "preflight", Status: "error", Reason: "backend not reachable at " + cfg.BaseURL + ": " + err.Error()}
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		return stepResult{Step: "preflight", Status: "error", HTTP: res.StatusCode, Reason: "/health/ready not 200", Detail: truncate(string(body), 200)}
	}
	return stepResult{Step: "preflight", Status: "ok", HTTP: 200}
}

// captureMetrics scrapes /metrics and writes the body to the artifact dir.
// The endpoint is public (no auth) per routes_core.go. The full text body is
// kept because grepping across timepoints is the canonical observation flow.
func captureMetrics(ctx context.Context, client *http.Client, cfg governanceContentConfig, filename string) stepResult {
	url := cfg.BaseURL + "/metrics"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	res, err := client.Do(req)
	if err != nil {
		return stepResult{Step: "metrics:" + filename, Status: "error", Reason: err.Error()}
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	path := filepath.Join(cfg.OutputDir, filename)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return stepResult{Step: "metrics:" + filename, Status: "error", Reason: err.Error()}
	}
	return stepResult{Step: "metrics:" + filename, Status: "ok", HTTP: res.StatusCode, Artifact: filename}
}

// sniffEvaluatorMode best-effort reads enforce_mode_total label out of a
// metrics scrape so the summary can report the live mode the operator's
// backend was running in.
func sniffEvaluatorMode(metricsPath string) string {
	b, err := os.ReadFile(metricsPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	for _, ln := range lines {
		// e.g. search_evaluator_enforce_mode_total{endpoint="content",mode="enforce",...} 1
		if !strings.Contains(ln, "enforce_mode_total") {
			continue
		}
		if i := strings.Index(ln, `mode="`); i >= 0 {
			rest := ln[i+len(`mode="`):]
			if j := strings.Index(rest, `"`); j > 0 {
				return rest[:j]
			}
		}
	}
	return ""
}

// mintToken POSTs /api/v1/auth/firebase/exchange with the mock Firebase ID
// token in the request body. Returns (step, authoredUserID). The mock-mode
// Firebase client (DEV_MOCK_FIREBASE_AUTH=true) hashes the token string to a
// deterministic UID so this scenario is idempotent across runs with the same
// runID.
func mintToken(ctx context.Context, client *http.Client, cfg governanceContentConfig, mockToken, filename, username string) (stepResult, string) {
	url := cfg.BaseURL + "/api/v1/auth/firebase/exchange"
	body := map[string]string{"firebase_id_token": mockToken}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return stepResult{Step: "auth:" + filename, Status: "error", Reason: err.Error()}, ""
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, filename), respBody, 0o644); err != nil {
		return stepResult{Step: "auth:" + filename, Status: "error", Reason: err.Error()}, ""
	}
	if res.StatusCode != 200 {
		return stepResult{Step: "auth:" + filename, Status: "error", HTTP: res.StatusCode, Artifact: filename, Reason: "auth not 200", Detail: truncate(string(respBody), 240)}, ""
	}
	// Extract user_id for downstream artifact correlation. The body has the
	// shape {success, data:{user_id, access_token, refresh_token, ...}}.
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			UserID                    string `json:"user_id"`
			AccessToken               string `json:"access_token"`
			RefreshToken              string `json:"refresh_token"`
			RequiresProfileCompletion bool   `json:"requires_profile_completion"`
		} `json:"data"`
	}
	_ = json.Unmarshal(respBody, &envelope)
	if !envelope.Success || envelope.Data.UserID == "" || envelope.Data.AccessToken == "" || envelope.Data.RefreshToken == "" || envelope.Data.RequiresProfileCompletion {
		return stepResult{
			Step:     "auth:" + filename,
			Status:   "blocked",
			HTTP:     200,
			Artifact: filename,
			Reason:   "firebase exchange did not return a full session",
			Detail:   truncate(string(respBody), 240),
		}, ""
	}
	return stepResult{Step: "auth:" + filename, Status: "ok", HTTP: 200, Artifact: filename, Detail: envelope.Data.UserID}, envelope.Data.UserID
}

// createContent POSTs /api/v1/contents with the keyword embedded in the
// caption so search can find it. Returns (step, contentID).
//
// The Idempotency-Key header is required by [content_handler.go:312-315]
// for safe-retry semantics on the create-content route. The key is derived
// deterministically from the run's keyword + a per-call suffix so the same
// run is idempotent on retry, while different runs never collide.
func createContent(ctx context.Context, client *http.Client, cfg governanceContentConfig, mockToken, filename string) (stepResult, string) {
	endpoint := cfg.BaseURL + "/api/v1/contents"
	body := map[string]any{
		"type":           "post",
		"caption":        cfg.Keyword + " — governance corpus driver scenario content (run " + cfg.OutputDir + ")",
		"visibility":     "public",
		"allow_comments": true,
	}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+mockToken)
	req.Header.Set("Idempotency-Key", "governance-content-create-"+cfg.Keyword+"-1")
	res, err := client.Do(req)
	if err != nil {
		return stepResult{Step: "create_content", Status: "error", Reason: err.Error()}, ""
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	_ = os.WriteFile(filepath.Join(cfg.OutputDir, filename), respBody, 0o644)

	switch res.StatusCode {
	case 200, 201:
		var env struct {
			Success bool `json:"success"`
			Data    struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		_ = json.Unmarshal(respBody, &env)
		return stepResult{Step: "create_content", Status: "ok", HTTP: res.StatusCode, Artifact: filename, Detail: env.Data.ID}, env.Data.ID
	case 403:
		// Surface the canonical blocker (EMAIL_VERIFICATION_REQUIRED) without
		// pretending the content was created. The harness deliberately stops
		// short of bypassing this — fixing it is a separate batch.
		return stepResult{
			Step:     "create_content",
			Status:   "blocked",
			HTTP:     403,
			Artifact: filename,
			Reason:   "interaction-authority middleware refused (likely EMAIL_VERIFICATION_REQUIRED for fresh mock user)",
			Detail:   truncate(string(respBody), 240),
		}, ""
	default:
		return stepResult{Step: "create_content", Status: "error", HTTP: res.StatusCode, Artifact: filename, Detail: truncate(string(respBody), 240)}, ""
	}
}

// deleteContent exercises the canonical owner-DELETE path on the previously-
// created content. Recorded as the only official-flow tombstone candidate.
func deleteContent(ctx context.Context, client *http.Client, cfg governanceContentConfig, mockToken, contentID string) stepResult {
	if contentID == "" {
		return stepResult{Step: "delete_content", Status: "skipped", Reason: "no content id to delete"}
	}
	url := cfg.BaseURL + "/api/v1/contents/" + contentID
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	req.Header.Set("Authorization", "Bearer "+mockToken)
	res, err := client.Do(req)
	if err != nil {
		return stepResult{Step: "delete_content", Status: "error", Reason: err.Error()}
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	_ = os.WriteFile(filepath.Join(cfg.OutputDir, "delete_content.json"), respBody, 0o644)
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return stepResult{Step: "delete_content", Status: "ok", HTTP: res.StatusCode, Artifact: "delete_content.json", Detail: contentID}
	}
	return stepResult{Step: "delete_content", Status: "error", HTTP: res.StatusCode, Artifact: "delete_content.json", Detail: truncate(string(respBody), 240)}
}

// searchContent issues GET /api/v1/search/content?q=<keyword>&limit=10
// as the viewer and parses the envelope. Lifecycle values are extracted
// from the response so the operator can confirm B5/B5.2 wire compatibility.
//
// The query parameter is named `q` per [search_handler.go:169]
// (`form:"q" binding:"required"`); url.QueryEscape keeps the harness honest
// for keywords that ever contain reserved characters even though the
// auto-generated default is ASCII-safe.
//
// B6.8 — parameterized so the same code path serves both the active-search
// (rows-must-include-expected-id) and the deleted-search (rows-must-NOT-
// include-expected-id) calls. The function returns BOTH a stepResult for
// the summary's step table AND a searchObservation that captures the
// analytic facts (lifecycle tally, expected-id presence, row count) for
// the B6.8 observation block. Either side may be nil-safe consumed by the
// orchestrator.
func searchContent(
	ctx context.Context,
	client *http.Client,
	cfg governanceContentConfig,
	mockToken string,
	stepName, filename string,
	expectedID string,
	mustBePresent bool,
) (stepResult, *searchObservation) {
	endpoint := cfg.BaseURL + "/api/v1/search/content?q=" + url.QueryEscape(cfg.Keyword) + "&limit=10"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+mockToken)
	res, err := client.Do(req)
	if err != nil {
		return stepResult{Step: stepName, Status: "error", Reason: err.Error()}, nil
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	_ = os.WriteFile(filepath.Join(cfg.OutputDir, filename), respBody, 0o644)
	if res.StatusCode != 200 {
		return stepResult{Step: stepName, Status: "error", HTTP: res.StatusCode, Artifact: filename, Detail: truncate(string(respBody), 240)},
			&searchObservation{HTTPStatus: res.StatusCode, ExpectedID: expectedID}
	}
	// Parse just enough to count rows and tally lifecycle values. Tolerate
	// either the bare data object or the wrapped {success, data} envelope.
	var raw map[string]any
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return stepResult{Step: stepName, Status: "error", HTTP: 200, Artifact: filename, Reason: "response not valid JSON: " + err.Error()},
			&searchObservation{HTTPStatus: 200, ExpectedID: expectedID}
	}
	data := raw
	if d, ok := raw["data"].(map[string]any); ok {
		data = d
	}
	contents, _ := data["contents"].([]any)
	lifecycleTally := map[string]int{}
	expectedFound := false
	for _, c := range contents {
		row, _ := c.(map[string]any)
		// id matching: tolerate both top-level "id" and a nested "card.id"
		// (per B5 PublicCard contract).
		rowID, _ := row["id"].(string)
		if rowID == "" {
			if card, ok := row["card"].(map[string]any); ok {
				rowID, _ = card["id"].(string)
			}
		}
		if expectedID != "" && rowID == expectedID {
			expectedFound = true
		}
		// lifecycle tally: capture the wire field value verbatim. Tolerate
		// absence (legacy rows or active-default emit no field) by labeling
		// the bucket "<unset>".
		lc, hasLC := row["lifecycle"].(string)
		if !hasLC || lc == "" {
			lc = "<unset>"
		}
		lifecycleTally[lc]++
	}
	obs := &searchObservation{
		HTTPStatus:      200,
		RowCount:        len(contents),
		LifecycleTally:  lifecycleTally,
		ExpectedID:      expectedID,
		ExpectedIDFound: expectedFound,
	}

	// Assertion classification. The harness records observed facts always;
	// the step's `status` field reflects whether facts matched expectation.
	tallyJSON, _ := json.Marshal(lifecycleTally)
	detail := fmt.Sprintf("rows=%d expected_id_found=%v lifecycle_tally=%s", len(contents), expectedFound, string(tallyJSON))
	step := stepResult{
		Step:     stepName,
		Status:   "ok",
		HTTP:     200,
		Artifact: filename,
		Detail:   detail,
	}
	if expectedID != "" {
		if mustBePresent && !expectedFound {
			step.Status = "blocked"
			step.Reason = "expected content id not present in active-search result"
			obs.Note = "active search did not surface the created row"
		}
		if !mustBePresent && expectedFound {
			step.Status = "blocked"
			step.Reason = "deleted content id still visible after delete"
			obs.Note = "deleted-search response still contains the row — investigate evaluator further-restrict semantics"
		}
	}
	return step, obs
}

// synthesizeVerdict folds the per-step results and the B6.8 observation
// fields into a one-line conclusion. The verdict is conservative — every
// degraded tier dominates a stronger one.
//
// Tier ladder (strongest to weakest):
//   ACTIVE_DELETE_LIFECYCLE_OBSERVED   — both searches OK + active row
//                                        visible + deleted row absent +
//                                        4 metrics scrapes OK
//   SEARCH_AND_METRICS_OBSERVED        — both searches 200 + metrics OK
//                                        but ID assertions did not match
//   SEARCH_OBSERVED_NO_METRICS         — searches 200 but metrics scrape
//                                        failed
//   METRICS_ONLY                       — metrics scrape OK but neither
//                                        search returned 200
//   PARTIAL                            — anything else
func synthesizeVerdict(sum *summary) string {
	var activeOK, deletedOK, metricsOK, anyError bool
	metricsCount := 0
	for _, s := range sum.Steps {
		if s.Status == "error" {
			anyError = true
		}
		switch {
		case s.Step == "search_content_active" && s.Status == "ok":
			activeOK = true
		case s.Step == "search_content_deleted" && s.Status == "ok":
			deletedOK = true
		case strings.HasPrefix(s.Step, "metrics:") && s.Status == "ok":
			metricsCount++
		}
	}
	metricsOK = metricsCount >= 4 // before, after_auth, after_create active, after_search_active, after_delete, after_search_deleted — at least 4 are essential

	activeRowSeen := sum.ActiveSearchObservation != nil && sum.ActiveSearchObservation.ExpectedIDFound
	deletedRowGone := sum.DeletedSearchObservation != nil && !sum.DeletedSearchObservation.ExpectedIDFound

	switch {
	case activeOK && deletedOK && metricsOK && activeRowSeen && deletedRowGone && !anyError:
		return "ACTIVE_DELETE_LIFECYCLE_OBSERVED"
	case activeOK && deletedOK && metricsOK:
		return "SEARCH_AND_METRICS_OBSERVED"
	case (activeOK || deletedOK) && !metricsOK:
		return "SEARCH_OBSERVED_NO_METRICS"
	case !activeOK && !deletedOK && metricsOK:
		return "METRICS_ONLY"
	default:
		return "PARTIAL"
	}
}

// sniffEnforcementApplied compares two metrics scrapes and reports whether
// any search_shadow_enforcement_applied_total label combination ticked
// across the run. The first parameter is the baseline scrape (typically
// metrics_before.txt) and the second is the final scrape.
//
// Returns (ticked, note). `ticked` is true only when at least one label
// combination's counter strictly increased between the two scrapes;
// label combinations present in only one file are also treated as a tick
// (a counter that didn't exist at t0 but exists at t1 has clearly been
// incremented at least once).
func sniffEnforcementApplied(beforePath, afterPath string) (bool, string) {
	beforeMap := readEnforcementSeries(beforePath)
	afterMap := readEnforcementSeries(afterPath)
	if len(afterMap) == 0 {
		return false, "no enforcement_applied_total series present in the final scrape — counter never incremented; with zero candidate rows surviving legacy SQL projection this is the expected outcome of the further-restrict-only contract"
	}
	for labels, after := range afterMap {
		before := beforeMap[labels]
		if after > before {
			return true, fmt.Sprintf("enforcement_applied_total ticked at labels {%s}: %g → %g", labels, before, after)
		}
	}
	return false, fmt.Sprintf("%d enforcement_applied_total series present but none incremented between baseline and final scrape (likely already at steady state for this run)", len(afterMap))
}

// readEnforcementSeries parses one scrape and returns a map of
// label-set → counter value for every line matching the
// search_shadow_enforcement_applied_total counter family. Non-matching
// or unparseable lines are skipped silently — this is a best-effort
// observer, not a Prometheus exposition validator.
func readEnforcementSeries(path string) map[string]float64 {
	out := map[string]float64{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(ln, "search_shadow_enforcement_applied_total") {
			continue
		}
		// Format: search_shadow_enforcement_applied_total{...labels...} <value>
		lbrace := strings.IndexByte(ln, '{')
		rbrace := strings.IndexByte(ln, '}')
		if lbrace < 0 || rbrace < lbrace {
			continue
		}
		labels := ln[lbrace+1 : rbrace]
		rest := strings.TrimSpace(ln[rbrace+1:])
		var v float64
		if _, err := fmt.Sscanf(rest, "%f", &v); err != nil {
			continue
		}
		out[labels] = v
	}
	return out
}

// sniffShadowContinuity verifies the shadow runner kept ticking between
// the active-search and deleted-search snapshots. Returns a one-line
// narrative for the summary block.
func sniffShadowContinuity(activePath, deletedPath string) string {
	active := readShadowRequestTotal(activePath)
	deleted := readShadowRequestTotal(deletedPath)
	switch {
	case deleted > active:
		return fmt.Sprintf("shadow continuity confirmed: search_shadow_request_total advanced %g → %g across active-search and deleted-search snapshots", active, deleted)
	case active > 0 && deleted == active:
		return fmt.Sprintf("search_shadow_request_total observed at %g but did not advance between active and deleted snapshots — investigate scrape ordering vs second search dispatch timing", active)
	case active == 0 && deleted == 0:
		return "search_shadow_request_total absent in both snapshots — shadow runner did not dispatch in this run"
	default:
		return fmt.Sprintf("search_shadow_request_total active=%g deleted=%g (unexpected non-monotonic change)", active, deleted)
	}
}

// readShadowRequestTotal extracts the sum of search_shadow_request_total
// across all label combinations in one scrape. We sum across labels
// because on this surface the only label combination is the canonical
// option_a_handler_post_response, but the helper is label-agnostic for
// future-proofing.
func readShadowRequestTotal(path string) float64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var total float64
	for _, ln := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(ln, "search_shadow_request_total{") {
			continue
		}
		rbrace := strings.IndexByte(ln, '}')
		if rbrace < 0 {
			continue
		}
		rest := strings.TrimSpace(ln[rbrace+1:])
		var v float64
		if _, err := fmt.Sscanf(rest, "%f", &v); err == nil {
			total += v
		}
	}
	return total
}

// finishSummary writes summary.json + a human-readable README.txt to the
// artifact dir and returns the original error (if any) for the caller.
func finishSummary(sum *summary, cfg governanceContentConfig, errMsg string) error {
	sum.Finished = time.Now().UTC().Format(time.RFC3339)
	out, _ := json.MarshalIndent(sum, "", "  ")
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, "summary.json"), out, 0o644); err != nil {
		return fmt.Errorf("write summary.json: %w", err)
	}

	var readme bytes.Buffer
	fmt.Fprintf(&readme, "scenario-governance-content\n")
	fmt.Fprintf(&readme, "  run_id:      %s\n", sum.RunID)
	fmt.Fprintf(&readme, "  base_url:    %s\n", sum.BaseURL)
	fmt.Fprintf(&readme, "  keyword:     %s\n", sum.Keyword)
	fmt.Fprintf(&readme, "  started:     %s\n", sum.Started)
	fmt.Fprintf(&readme, "  finished:    %s\n", sum.Finished)
	fmt.Fprintf(&readme, "  evaluator:   %s\n", sum.EvaluatorMode)
	fmt.Fprintf(&readme, "  verdict:     %s\n\n", sum.Verdict)
	fmt.Fprintf(&readme, "steps:\n")
	for _, s := range sum.Steps {
		fmt.Fprintf(&readme, "  - %-30s %-8s http=%d %s\n", s.Step, s.Status, s.HTTP, oneOf(s.Reason, s.Detail))
	}
	fmt.Fprintf(&readme, "\nobservation:\n")
	if sum.ActiveSearchObservation != nil {
		o := sum.ActiveSearchObservation
		tally, _ := json.Marshal(o.LifecycleTally)
		fmt.Fprintf(&readme, "  active_search:   rows=%d expected_id_found=%v lifecycle_tally=%s\n",
			o.RowCount, o.ExpectedIDFound, string(tally))
	} else {
		fmt.Fprintf(&readme, "  active_search:   (not executed)\n")
	}
	if sum.DeletedSearchObservation != nil {
		o := sum.DeletedSearchObservation
		tally, _ := json.Marshal(o.LifecycleTally)
		fmt.Fprintf(&readme, "  deleted_search:  rows=%d expected_id_found=%v lifecycle_tally=%s\n",
			o.RowCount, o.ExpectedIDFound, string(tally))
	} else {
		fmt.Fprintf(&readme, "  deleted_search:  (not executed)\n")
	}
	fmt.Fprintf(&readme, "  enforcement_applied_ticked: %v\n", sum.EnforcementAppliedTicked)
	if sum.EnforcementAppliedNote != "" {
		fmt.Fprintf(&readme, "    note: %s\n", sum.EnforcementAppliedNote)
	}
	if sum.ShadowContinuityNote != "" {
		fmt.Fprintf(&readme, "  shadow_continuity: %s\n", sum.ShadowContinuityNote)
	}
	_ = os.WriteFile(filepath.Join(cfg.OutputDir, "README.txt"), readme.Bytes(), 0o644)

	if cfg.Verbose {
		fmt.Println(readme.String())
	}
	if errMsg != "" {
		return fmt.Errorf("scenario incomplete: %s (see %s)", errMsg, cfg.OutputDir)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func oneOf(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func shortID(runID string) string {
	// runID is RFC3339-ish like "20060102T150405Z"; use the last 6 chars
	// (HMMSS) so generated usernames stay under the username max length.
	if len(runID) <= 6 {
		return runID
	}
	return strings.ToLower(runID[len(runID)-6:])
}
