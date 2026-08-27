package main

// Governance feed scenario — official-HTTP-only harness for /api/v1/feed
// governance convergence observation (C1).
//
// CONSTITUTIONAL POSTURE (mirrors scenario_governance_content.go):
//   - Pure HTTP. Does NOT touch DB / Redis / Firebase / Midtrans
//     directly; speaks only to the running core_server over HTTP.
//   - Every request and response is captured to disk under the run's
//     artifact directory. Nothing is lost between runs.
//   - The scenario is best-effort: if a step is blocked by a real
//     production invariant (e.g. EMAIL_VERIFICATION_REQUIRED for fresh
//     mock users), it records the blocker and proceeds with the
//     remaining observable steps. It NEVER fabricates state.
//
// FLOW (mirrors B6.8 active/delete pair):
//   create author -> create viewer -> viewer follows author ->
//   author creates content -> viewer GET /feed (active) ->
//   author DELETE /contents/:id -> viewer GET /feed (after delete)
//
// Observation block answers the C1 questions:
//   - lifecycle values actually emitted live (active / unavailable / removed)
//   - feed_evaluator_enforcement_applied_total tick (drop|lifecycle_override|unknown_fail_open)
//   - feed shadow request continuity across active/delete snapshots
//   - feed_evaluator_enforce_mode_total label observed live

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

type governanceFeedConfig struct {
	BaseURL   string
	OutputDir string
	Keyword   string
	Timeout   time.Duration
	Verbose   bool
}

type feedObservation struct {
	HTTPStatus        int            `json:"http_status"`
	RowCount          int            `json:"row_count"`
	LifecycleTally    map[string]int `json:"lifecycle_tally"`
	ExpectedID        string         `json:"expected_id"`
	ExpectedIDFound   bool           `json:"expected_id_found"`
	ExpectedLifecycle string         `json:"expected_lifecycle,omitempty"`
	Note              string         `json:"note,omitempty"`
}

type feedSummary struct {
	Scenario      string       `json:"scenario"`
	Started       string       `json:"started"`
	Finished      string       `json:"finished"`
	BaseURL       string       `json:"base_url"`
	Keyword       string       `json:"keyword"`
	RunID         string       `json:"run_id"`
	Verdict       string       `json:"verdict"`
	EvaluatorMode string       `json:"evaluator_mode_observed,omitempty"`
	Steps         []stepResult `json:"steps"`

	ActiveFeedObservation    *feedObservation `json:"active_feed_observation,omitempty"`
	DeletedFeedObservation   *feedObservation `json:"deleted_feed_observation,omitempty"`
	EnforcementAppliedTicked bool             `json:"enforcement_applied_ticked"`
	EnforcementAppliedNote   string           `json:"enforcement_applied_note,omitempty"`
	ShadowContinuityNote     string           `json:"shadow_continuity_note,omitempty"`
}

func runScenarioGovernanceFeed(cfg governanceFeedConfig) error {
	started := time.Now().UTC()
	runID := started.Format("20060102T150405Z")
	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join("scenario_logs", "governance-feed-"+runID)
	}
	if cfg.Keyword == "" {
		cfg.Keyword = "labudafeed" + runID
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir %s: %w", cfg.OutputDir, err)
	}

	sum := &feedSummary{
		Scenario: "scenario-governance-feed",
		Started:  started.Format(time.RFC3339),
		BaseURL:  cfg.BaseURL,
		Keyword:  cfg.Keyword,
		RunID:    runID,
		Verdict:  "PARTIAL",
		Steps:    make([]stepResult, 0, 16),
	}

	client := &http.Client{Timeout: cfg.Timeout}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout*10)
	defer cancel()

	// ─── preflight ────────────────────────────────────────────────────────
	if step := feedPreflight(ctx, client, cfg); step.Status == "error" {
		sum.Steps = append(sum.Steps, step)
		sum.Verdict = "BLOCKED_PREFLIGHT"
		return finishFeedSummary(sum, cfg, "preflight failed: "+step.Reason)
	} else {
		sum.Steps = append(sum.Steps, step)
	}

	// ─── metrics baseline ─────────────────────────────────────────────────
	sum.Steps = append(sum.Steps, feedCaptureMetrics(ctx, client, cfg, "metrics_before.txt"))
	sum.EvaluatorMode = sniffFeedEvaluatorMode(filepath.Join(cfg.OutputDir, "metrics_before.txt"))

	// ─── auth: author ─────────────────────────────────────────────────────
	// IMPORTANT - downstream auth uses the access_token returned by the
	// canonical exchange response. The mock Firebase verifier still hashes the
	// request token to a deterministic UID, so the scenario remains
	// idempotent across runs with the same fixture token.
	authorToken := "governance-feed-author-verified-" + runID
	authorStep, authorID, _ := feedMintToken(ctx, client, cfg, authorToken, "auth_author.json", "feedauthor"+shortID(runID))
	sum.Steps = append(sum.Steps, authorStep)
	if authorStep.Status != "ok" {
		return finishFeedSummary(sum, cfg, "author auth failed")
	}

	// ─── auth: viewer ─────────────────────────────────────────────────────
	viewerToken := "governance-feed-viewer-verified-" + runID
	viewerStep, _, _ := feedMintToken(ctx, client, cfg, viewerToken, "auth_viewer.json", "feedviewer"+shortID(runID))
	sum.Steps = append(sum.Steps, viewerStep)
	if viewerStep.Status != "ok" {
		return finishFeedSummary(sum, cfg, "viewer auth failed")
	}

	// ─── follow: viewer follows author ────────────────────────────────────
	// /feed only surfaces content from followed users; without a follow
	// edge the viewer's feed will be empty regardless of governance state.
	followStep := feedFollow(ctx, client, cfg, viewerToken, authorID)
	sum.Steps = append(sum.Steps, followStep)

	// ─── create content (author) ──────────────────────────────────────────
	createStep, createdID := feedCreateContent(ctx, client, cfg, authorToken, "create_content.json")
	sum.Steps = append(sum.Steps, createStep)
	sum.Steps = append(sum.Steps, feedCaptureMetrics(ctx, client, cfg, "metrics_after_create.txt"))

	// ─── active feed (BEFORE delete) ──────────────────────────────────────
	activeStep, activeObs := feedListFeed(
		ctx, client, cfg, viewerToken,
		"feed_active", "feed_active.json",
		createdID, true, "active",
	)
	sum.Steps = append(sum.Steps, activeStep)
	sum.ActiveFeedObservation = activeObs
	sum.Steps = append(sum.Steps, feedCaptureMetrics(ctx, client, cfg, "metrics_after_feed_active.txt"))

	// ─── delete + post-delete metrics snapshot ────────────────────────────
	if createStep.Status == "ok" && createdID != "" {
		delStep := feedDeleteContent(ctx, client, cfg, authorToken, createdID)
		sum.Steps = append(sum.Steps, delStep)
	} else {
		sum.Steps = append(sum.Steps, stepResult{
			Step:   "delete_content",
			Status: "skipped",
			Reason: "no content was created in this run",
		})
	}
	sum.Steps = append(sum.Steps, feedCaptureMetrics(ctx, client, cfg, "metrics_after_delete.txt"))

	// ─── deleted feed (AFTER delete) ──────────────────────────────────────
	// The legacy SQL projection at feed_repository_impl.go gates on
	// c.status='active', so a hard-deleted row is physically absent from
	// the candidate set. Evaluator cannot recover undershare (further-
	// restrict-only doctrine); the expected outcome is absence.
	deletedStep, deletedObs := feedListFeed(
		ctx, client, cfg, viewerToken,
		"feed_deleted", "feed_deleted.json",
		createdID, false, "",
	)
	sum.Steps = append(sum.Steps, deletedStep)
	sum.DeletedFeedObservation = deletedObs
	sum.Steps = append(sum.Steps, feedCaptureMetrics(ctx, client, cfg, "metrics_after_feed_deleted.txt"))

	// ─── post-run observation synthesis ───────────────────────────────────
	if late := sniffFeedEvaluatorMode(filepath.Join(cfg.OutputDir, "metrics_after_feed_deleted.txt")); late != "" {
		sum.EvaluatorMode = late
	}
	sum.EnforcementAppliedTicked, sum.EnforcementAppliedNote = sniffFeedEnforcementApplied(
		filepath.Join(cfg.OutputDir, "metrics_before.txt"),
		filepath.Join(cfg.OutputDir, "metrics_after_feed_deleted.txt"),
	)
	sum.ShadowContinuityNote = sniffFeedShadowContinuity(
		filepath.Join(cfg.OutputDir, "metrics_after_feed_active.txt"),
		filepath.Join(cfg.OutputDir, "metrics_after_feed_deleted.txt"),
	)

	sum.Verdict = synthesizeFeedVerdict(sum)
	_ = authorID
	return finishFeedSummary(sum, cfg, "")
}

func feedPreflight(ctx context.Context, client *http.Client, cfg governanceFeedConfig) stepResult {
	u := cfg.BaseURL + "/health/ready"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	res, err := client.Do(req)
	if err != nil {
		return stepResult{Step: "preflight", Status: "error", Reason: err.Error()}
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		return stepResult{Step: "preflight", Status: "error", HTTP: res.StatusCode, Reason: "/health/ready not 200", Detail: truncate(string(body), 200)}
	}
	return stepResult{Step: "preflight", Status: "ok", HTTP: 200}
}

func feedCaptureMetrics(ctx context.Context, client *http.Client, cfg governanceFeedConfig, filename string) stepResult {
	u := cfg.BaseURL + "/metrics"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	res, err := client.Do(req)
	if err != nil {
		return stepResult{Step: "metrics:" + filename, Status: "error", Reason: err.Error()}
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, filename), body, 0o644); err != nil {
		return stepResult{Step: "metrics:" + filename, Status: "error", Reason: err.Error()}
	}
	return stepResult{Step: "metrics:" + filename, Status: "ok", HTTP: res.StatusCode, Artifact: filename}
}

// sniffFeedEvaluatorMode reads labuda_evaluator_feed_enforce_mode_total
// label out of a metrics scrape.
func sniffFeedEvaluatorMode(metricsPath string) string {
	b, err := os.ReadFile(metricsPath)
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(ln, "labuda_evaluator_feed_enforce_mode_total") {
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

// feedMintToken returns the step + (id, access_token) from the canonical
// Firebase exchange route.
func feedMintToken(ctx context.Context, client *http.Client, cfg governanceFeedConfig, mockToken, filename, username string) (stepResult, string, string) {
	u := cfg.BaseURL + "/api/v1/auth/firebase/exchange"
	body := map[string]string{"firebase_id_token": mockToken}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return stepResult{Step: "auth:" + filename, Status: "error", Reason: err.Error()}, "", ""
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	_ = os.WriteFile(filepath.Join(cfg.OutputDir, filename), respBody, 0o644)
	if res.StatusCode != 200 {
		return stepResult{Step: "auth:" + filename, Status: "error", HTTP: res.StatusCode, Artifact: filename, Reason: "auth not 200", Detail: truncate(string(respBody), 240)}, "", ""
	}
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
		}, "", ""
	}
	return stepResult{Step: "auth:" + filename, Status: "ok", HTTP: 200, Artifact: filename, Detail: envelope.Data.UserID}, envelope.Data.UserID, envelope.Data.AccessToken
}

// feedFollow makes viewer follow author. The follow route is part of the
// social/graph domain; check the actual route shape with metrics.
func feedFollow(ctx context.Context, client *http.Client, cfg governanceFeedConfig, viewerAccess, authorID string) stepResult {
	if authorID == "" {
		return stepResult{Step: "follow_author", Status: "skipped", Reason: "no author id"}
	}
	// Canonical follow route: POST /api/v1/users/:id/follow per
	// social/graph domain wiring. We try this shape; if 404, record blocker.
	u := cfg.BaseURL + "/api/v1/users/" + authorID + "/follow"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	req.Header.Set("Authorization", "Bearer "+viewerAccess)
	res, err := client.Do(req)
	if err != nil {
		return stepResult{Step: "follow_author", Status: "error", Reason: err.Error()}
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	_ = os.WriteFile(filepath.Join(cfg.OutputDir, "follow_author.json"), respBody, 0o644)
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return stepResult{Step: "follow_author", Status: "ok", HTTP: res.StatusCode, Artifact: "follow_author.json"}
	}
	if res.StatusCode == 409 {
		// Already following — same idempotent effect.
		return stepResult{Step: "follow_author", Status: "ok", HTTP: 409, Artifact: "follow_author.json", Detail: "already following"}
	}
	return stepResult{Step: "follow_author", Status: "blocked", HTTP: res.StatusCode, Artifact: "follow_author.json", Reason: "follow route not available or refused", Detail: truncate(string(respBody), 240)}
}

func feedCreateContent(ctx context.Context, client *http.Client, cfg governanceFeedConfig, accessToken, filename string) (stepResult, string) {
	endpoint := cfg.BaseURL + "/api/v1/contents"
	body := map[string]any{
		"type":           "post",
		"caption":        cfg.Keyword + " — governance feed corpus driver scenario",
		"visibility":     "public",
		"allow_comments": true,
	}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Idempotency-Key", "governance-feed-create-"+cfg.Keyword+"-1")
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

func feedDeleteContent(ctx context.Context, client *http.Client, cfg governanceFeedConfig, accessToken, contentID string) stepResult {
	if contentID == "" {
		return stepResult{Step: "delete_content", Status: "skipped", Reason: "no content id to delete"}
	}
	u := cfg.BaseURL + "/api/v1/contents/" + contentID
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
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

// feedListFeed issues GET /api/v1/feed and parses the response. The wire
// shape per feed_handler.go is:
//   { success, data: { data: [...feed items...], next_cursor, has_more } }
// each item: { id, author_id, type, status, lifecycle, card: {lifecycle,...}, ... }
func feedListFeed(
	ctx context.Context,
	client *http.Client,
	cfg governanceFeedConfig,
	accessToken string,
	stepName, filename string,
	expectedID string,
	mustBePresent bool,
	expectedLifecycle string,
) (stepResult, *feedObservation) {
	endpoint := cfg.BaseURL + "/api/v1/feed?limit=20"
	if cfg.Keyword != "" {
		// keyword unused on /feed (it is non-search), but keep the URL
		// canonical for log parity.
		_ = url.QueryEscape(cfg.Keyword)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res, err := client.Do(req)
	if err != nil {
		return stepResult{Step: stepName, Status: "error", Reason: err.Error()}, nil
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	_ = os.WriteFile(filepath.Join(cfg.OutputDir, filename), respBody, 0o644)
	if res.StatusCode != 200 {
		return stepResult{Step: stepName, Status: "error", HTTP: res.StatusCode, Artifact: filename, Detail: truncate(string(respBody), 240)},
			&feedObservation{HTTPStatus: res.StatusCode, ExpectedID: expectedID, ExpectedLifecycle: expectedLifecycle}
	}
	var raw map[string]any
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return stepResult{Step: stepName, Status: "error", HTTP: 200, Artifact: filename, Reason: "response not valid JSON: " + err.Error()},
			&feedObservation{HTTPStatus: 200, ExpectedID: expectedID, ExpectedLifecycle: expectedLifecycle}
	}
	data := raw
	if d, ok := raw["data"].(map[string]any); ok {
		data = d
	}
	// feed_handler.go emits {"data": [...], "next_cursor": ..., "has_more": ...}
	items, _ := data["data"].([]any)
	lifecycleTally := map[string]int{}
	expectedFound := false
	expectedRowLifecycle := ""
	for _, it := range items {
		row, _ := it.(map[string]any)
		rowID, _ := row["id"].(string)
		if expectedID != "" && rowID == expectedID {
			expectedFound = true
			if lc, ok := row["lifecycle"].(string); ok {
				expectedRowLifecycle = lc
			} else if card, ok := row["card"].(map[string]any); ok {
				if lc, ok := card["lifecycle"].(string); ok {
					expectedRowLifecycle = lc
				}
			}
		}
		lc, hasLC := row["lifecycle"].(string)
		if !hasLC || lc == "" {
			lc = "<unset>"
		}
		lifecycleTally[lc]++
	}
	obs := &feedObservation{
		HTTPStatus:        200,
		RowCount:          len(items),
		LifecycleTally:    lifecycleTally,
		ExpectedID:        expectedID,
		ExpectedIDFound:   expectedFound,
		ExpectedLifecycle: expectedRowLifecycle,
	}

	tallyJSON, _ := json.Marshal(lifecycleTally)
	detail := fmt.Sprintf("rows=%d expected_id_found=%v lifecycle_tally=%s expected_row_lifecycle=%q", len(items), expectedFound, string(tallyJSON), expectedRowLifecycle)
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
			step.Reason = "expected content id not present in active feed result"
			obs.Note = "active feed did not surface the created row — investigate follow edge / projection"
		}
		if !mustBePresent && expectedFound {
			// Could be governance lifecycle override path: row visible
			// but lifecycle should be removed/unavailable. Active is a
			// red flag.
			if expectedRowLifecycle == "removed" || expectedRowLifecycle == "unavailable" {
				step.Reason = "row visible after delete via lifecycle override (" + expectedRowLifecycle + ") — governance transport observed live"
			} else {
				step.Status = "blocked"
				step.Reason = "deleted content id still visible after delete with lifecycle=" + expectedRowLifecycle
				obs.Note = "deleted-feed response still contains the row with active lifecycle — investigate further-restrict semantics"
			}
		}
		if expectedLifecycle != "" && expectedRowLifecycle != "" && expectedRowLifecycle != expectedLifecycle {
			obs.Note = obs.Note + fmt.Sprintf(" (expected lifecycle=%q got %q)", expectedLifecycle, expectedRowLifecycle)
		}
	}
	return step, obs
}

func synthesizeFeedVerdict(sum *feedSummary) string {
	var activeOK, deletedOK bool
	metricsCount := 0
	anyError := false
	for _, s := range sum.Steps {
		if s.Status == "error" {
			anyError = true
		}
		switch {
		case s.Step == "feed_active" && s.Status == "ok":
			activeOK = true
		case s.Step == "feed_deleted" && s.Status == "ok":
			deletedOK = true
		case strings.HasPrefix(s.Step, "metrics:") && s.Status == "ok":
			metricsCount++
		}
	}
	metricsOK := metricsCount >= 4
	activeRowSeen := sum.ActiveFeedObservation != nil && sum.ActiveFeedObservation.ExpectedIDFound
	deletedRowGoneOrTombstoned := sum.DeletedFeedObservation != nil &&
		(!sum.DeletedFeedObservation.ExpectedIDFound ||
			sum.DeletedFeedObservation.ExpectedLifecycle == "removed" ||
			sum.DeletedFeedObservation.ExpectedLifecycle == "unavailable")

	switch {
	case activeOK && deletedOK && metricsOK && activeRowSeen && deletedRowGoneOrTombstoned && !anyError:
		return "ACTIVE_DELETE_LIFECYCLE_OBSERVED"
	case activeOK && deletedOK && metricsOK:
		return "FEED_AND_METRICS_OBSERVED"
	case (activeOK || deletedOK) && !metricsOK:
		return "FEED_OBSERVED_NO_METRICS"
	case !activeOK && !deletedOK && metricsOK:
		return "METRICS_ONLY"
	default:
		return "PARTIAL"
	}
}

// sniffFeedEnforcementApplied compares two metrics scrapes for the
// labuda_evaluator_feed_enforcement_applied_total counter family.
func sniffFeedEnforcementApplied(beforePath, afterPath string) (bool, string) {
	beforeMap := readFeedSeries(beforePath, "labuda_evaluator_feed_enforcement_applied_total")
	afterMap := readFeedSeries(afterPath, "labuda_evaluator_feed_enforcement_applied_total")
	if len(afterMap) == 0 {
		return false, "no labuda_evaluator_feed_enforcement_applied_total series present in the final scrape — counter never incremented; if the legacy SQL projection excluded the row (status='active' gate) this is the expected outcome of the further-restrict-only contract"
	}
	for labels, after := range afterMap {
		before := beforeMap[labels]
		if after > before {
			return true, fmt.Sprintf("feed enforcement_applied_total ticked at labels {%s}: %g → %g", labels, before, after)
		}
	}
	return false, fmt.Sprintf("%d feed enforcement_applied_total series present but none incremented between baseline and final scrape (likely no governance row in this run)", len(afterMap))
}

func readFeedSeries(path, prefix string) map[string]float64 {
	out := map[string]float64{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(ln, prefix) {
			continue
		}
		lbrace := strings.IndexByte(ln, '{')
		rbrace := strings.IndexByte(ln, '}')
		labels := ""
		var rest string
		if lbrace >= 0 && rbrace > lbrace {
			labels = ln[lbrace+1 : rbrace]
			rest = strings.TrimSpace(ln[rbrace+1:])
		} else {
			parts := strings.Fields(ln)
			if len(parts) < 2 {
				continue
			}
			rest = parts[len(parts)-1]
		}
		var v float64
		if _, err := fmt.Sscanf(rest, "%f", &v); err != nil {
			continue
		}
		out[labels] = v
	}
	return out
}

func sniffFeedShadowContinuity(activePath, deletedPath string) string {
	active := readFeedShadowRequestTotal(activePath)
	deleted := readFeedShadowRequestTotal(deletedPath)
	switch {
	case deleted > active:
		return fmt.Sprintf("feed shadow continuity confirmed: labuda_evaluator_shadow_request_total{surface=feed} advanced %g → %g", active, deleted)
	case active > 0 && deleted == active:
		return fmt.Sprintf("feed shadow request_total observed at %g but did not advance between active and deleted snapshots", active)
	case active == 0 && deleted == 0:
		return "feed shadow request_total absent in both snapshots — feed shadow runner did not dispatch in this run (EVALUATOR_SHADOW_FEED_ENABLED?)"
	default:
		return fmt.Sprintf("feed shadow request_total active=%g deleted=%g (unexpected non-monotonic change)", active, deleted)
	}
}

func readFeedShadowRequestTotal(path string) float64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var total float64
	for _, ln := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(ln, "labuda_evaluator_shadow_request_total{") {
			continue
		}
		if !strings.Contains(ln, `surface="feed"`) {
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

func finishFeedSummary(sum *feedSummary, cfg governanceFeedConfig, errMsg string) error {
	sum.Finished = time.Now().UTC().Format(time.RFC3339)
	out, _ := json.MarshalIndent(sum, "", "  ")
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, "summary.json"), out, 0o644); err != nil {
		return fmt.Errorf("write summary.json: %w", err)
	}
	var readme bytes.Buffer
	fmt.Fprintf(&readme, "scenario-governance-feed\n")
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
	if sum.ActiveFeedObservation != nil {
		o := sum.ActiveFeedObservation
		tally, _ := json.Marshal(o.LifecycleTally)
		fmt.Fprintf(&readme, "  active_feed:   rows=%d expected_id_found=%v lifecycle=%q tally=%s\n",
			o.RowCount, o.ExpectedIDFound, o.ExpectedLifecycle, string(tally))
	} else {
		fmt.Fprintf(&readme, "  active_feed:   (not executed)\n")
	}
	if sum.DeletedFeedObservation != nil {
		o := sum.DeletedFeedObservation
		tally, _ := json.Marshal(o.LifecycleTally)
		fmt.Fprintf(&readme, "  deleted_feed:  rows=%d expected_id_found=%v lifecycle=%q tally=%s\n",
			o.RowCount, o.ExpectedIDFound, o.ExpectedLifecycle, string(tally))
	} else {
		fmt.Fprintf(&readme, "  deleted_feed:  (not executed)\n")
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
