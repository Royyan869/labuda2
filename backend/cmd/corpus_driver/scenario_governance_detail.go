package main

// D1 — Governance content-detail scenario.
//
// Official-HTTP-only harness for /api/v1/contents/:id governance
// convergence observation. Mirror of scenario_governance_content.go and
// scenario_governance_feed.go but exercising the DETAIL surface doctrine:
//
//   - active content → 200 with status="active" and (D1) lifecycle="active"
//   - deleted content → 404 (existing architectural truth, preserved under
//     enforce; legacy gate continues to fire first)
//   - UNKNOWN → 404 in enforce mode (doctrine §8.5 fail-CLOSED — not
//     observable in this corpus because the corpus runs against a fully-
//     hydrated DB; observable only when overlays fail)
//
// CONSTITUTIONAL POSTURE (same as the sibling scenarios):
//   - Pure HTTP; no DB / Redis / Firebase / Midtrans clients in this
//     process.
//   - Every request and response is captured to disk.
//   - The scenario is best-effort: records blockers rather than fabricating
//     state.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type governanceDetailConfig struct {
	BaseURL   string
	OutputDir string
	Keyword   string
	Timeout   time.Duration
	Verbose   bool
}

type detailObservation struct {
	HTTPStatus     int    `json:"http_status"`
	ContentID      string `json:"content_id"`
	StatusField    string `json:"status_field,omitempty"`
	LifecycleField string `json:"lifecycle_field,omitempty"`
	CardLifecycle  string `json:"card_lifecycle,omitempty"`
	Note           string `json:"note,omitempty"`
}

type detailSummary struct {
	Scenario      string       `json:"scenario"`
	Started       string       `json:"started"`
	Finished      string       `json:"finished"`
	BaseURL       string       `json:"base_url"`
	Keyword       string       `json:"keyword"`
	RunID         string       `json:"run_id"`
	Verdict       string       `json:"verdict"`
	EvaluatorMode string       `json:"evaluator_mode_observed,omitempty"`
	Steps         []stepResult `json:"steps"`

	ActiveDetailObservation  *detailObservation `json:"active_detail_observation,omitempty"`
	DeletedDetailObservation *detailObservation `json:"deleted_detail_observation,omitempty"`

	EnforcementAppliedTicked bool   `json:"enforcement_applied_ticked"`
	EnforcementAppliedNote   string `json:"enforcement_applied_note,omitempty"`
	ShadowContinuityNote     string `json:"shadow_continuity_note,omitempty"`
}

func runScenarioGovernanceDetail(cfg governanceDetailConfig) error {
	started := time.Now().UTC()
	runID := started.Format("20060102T150405Z")
	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join("scenario_logs", "governance-detail-"+runID)
	}
	if cfg.Keyword == "" {
		cfg.Keyword = "labudadetail" + runID
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir %s: %w", cfg.OutputDir, err)
	}

	sum := &detailSummary{
		Scenario: "scenario-governance-detail",
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
	if step := detailPreflight(ctx, client, cfg); step.Status == "error" {
		sum.Steps = append(sum.Steps, step)
		sum.Verdict = "BLOCKED_PREFLIGHT"
		return finishDetailSummary(sum, cfg, "preflight failed: "+step.Reason)
	} else {
		sum.Steps = append(sum.Steps, step)
	}

	// ─── metrics baseline ─────────────────────────────────────────────────
	sum.Steps = append(sum.Steps, detailCaptureMetrics(ctx, client, cfg, "metrics_before.txt"))
	sum.EvaluatorMode = sniffDetailEvaluatorMode(filepath.Join(cfg.OutputDir, "metrics_before.txt"))

	// ─── auth: author ─────────────────────────────────────────────────────
	authorToken := "governance-detail-author-verified-" + runID
	authorStep, authorID, _ := detailMintToken(ctx, client, cfg, authorToken, "auth_author.json", "detailauthor"+shortID(runID))
	sum.Steps = append(sum.Steps, authorStep)
	if authorStep.Status != "ok" {
		return finishDetailSummary(sum, cfg, "author auth failed")
	}

	// ─── auth: viewer ─────────────────────────────────────────────────────
	viewerToken := "governance-detail-viewer-verified-" + runID
	viewerStep, _, _ := detailMintToken(ctx, client, cfg, viewerToken, "auth_viewer.json", "detailviewer"+shortID(runID))
	sum.Steps = append(sum.Steps, viewerStep)
	if viewerStep.Status != "ok" {
		return finishDetailSummary(sum, cfg, "viewer auth failed")
	}
	_ = authorID

	// ─── create content (author) ──────────────────────────────────────────
	createStep, createdID := detailCreateContent(ctx, client, cfg, authorToken, "create_content.json")
	sum.Steps = append(sum.Steps, createStep)
	sum.Steps = append(sum.Steps, detailCaptureMetrics(ctx, client, cfg, "metrics_after_create.txt"))

	// ─── active detail (BEFORE delete) ────────────────────────────────────
	activeStep, activeObs := detailGetContent(
		ctx, client, cfg, viewerToken,
		"detail_active", "detail_active.json",
		createdID, true,
	)
	sum.Steps = append(sum.Steps, activeStep)
	sum.ActiveDetailObservation = activeObs
	sum.Steps = append(sum.Steps, detailCaptureMetrics(ctx, client, cfg, "metrics_after_detail_active.txt"))

	// ─── delete + post-delete metrics snapshot ────────────────────────────
	if createStep.Status == "ok" && createdID != "" {
		delStep := detailDeleteContent(ctx, client, cfg, authorToken, createdID)
		sum.Steps = append(sum.Steps, delStep)
	} else {
		sum.Steps = append(sum.Steps, stepResult{
			Step:   "delete_content",
			Status: "skipped",
			Reason: "no content was created in this run",
		})
	}
	sum.Steps = append(sum.Steps, detailCaptureMetrics(ctx, client, cfg, "metrics_after_delete.txt"))

	// ─── deleted detail (AFTER delete) ────────────────────────────────────
	// Expected: HTTP 404 (existing architectural truth — legacy gate
	// returns 404 for status=deleted non-admin). Detail surface does NOT
	// emit a tombstone JSON payload; the 404 is the tombstone.
	deletedStep, deletedObs := detailGetContent(
		ctx, client, cfg, viewerToken,
		"detail_deleted", "detail_deleted.json",
		createdID, false,
	)
	sum.Steps = append(sum.Steps, deletedStep)
	sum.DeletedDetailObservation = deletedObs
	sum.Steps = append(sum.Steps, detailCaptureMetrics(ctx, client, cfg, "metrics_after_detail_deleted.txt"))

	// ─── post-run observation synthesis ───────────────────────────────────
	if late := sniffDetailEvaluatorMode(filepath.Join(cfg.OutputDir, "metrics_after_detail_deleted.txt")); late != "" {
		sum.EvaluatorMode = late
	}
	sum.EnforcementAppliedTicked, sum.EnforcementAppliedNote = sniffDetailEnforcementApplied(
		filepath.Join(cfg.OutputDir, "metrics_before.txt"),
		filepath.Join(cfg.OutputDir, "metrics_after_detail_deleted.txt"),
	)
	sum.ShadowContinuityNote = sniffDetailShadowContinuity(
		filepath.Join(cfg.OutputDir, "metrics_after_detail_active.txt"),
		filepath.Join(cfg.OutputDir, "metrics_after_detail_deleted.txt"),
	)

	sum.Verdict = synthesizeDetailVerdict(sum)
	return finishDetailSummary(sum, cfg, "")
}

func detailPreflight(ctx context.Context, client *http.Client, cfg governanceDetailConfig) stepResult {
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

func detailCaptureMetrics(ctx context.Context, client *http.Client, cfg governanceDetailConfig, filename string) stepResult {
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

func sniffDetailEvaluatorMode(metricsPath string) string {
	b, err := os.ReadFile(metricsPath)
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(ln, "labuda_evaluator_content_detail_enforce_mode_total") {
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

func detailMintToken(ctx context.Context, client *http.Client, cfg governanceDetailConfig, mockToken, filename, username string) (stepResult, string, string) {
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

func detailCreateContent(ctx context.Context, client *http.Client, cfg governanceDetailConfig, accessToken, filename string) (stepResult, string) {
	endpoint := cfg.BaseURL + "/api/v1/contents"
	body := map[string]any{
		"type":           "post",
		"caption":        cfg.Keyword + " — governance detail corpus driver scenario",
		"visibility":     "public",
		"allow_comments": true,
	}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Idempotency-Key", "governance-detail-create-"+cfg.Keyword+"-1")
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
			Reason:   "interaction-authority middleware refused",
			Detail:   truncate(string(respBody), 240),
		}, ""
	default:
		return stepResult{Step: "create_content", Status: "error", HTTP: res.StatusCode, Artifact: filename, Detail: truncate(string(respBody), 240)}, ""
	}
}

func detailDeleteContent(ctx context.Context, client *http.Client, cfg governanceDetailConfig, accessToken, contentID string) stepResult {
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

// detailGetContent issues GET /api/v1/contents/:id and captures the
// observed status / lifecycle / card.lifecycle values. mustBePresent=true
// asserts an HTTP 200; mustBePresent=false asserts an HTTP 404.
func detailGetContent(
	ctx context.Context,
	client *http.Client,
	cfg governanceDetailConfig,
	mockToken string,
	stepName, filename string,
	contentID string,
	mustBePresent bool,
) (stepResult, *detailObservation) {
	if contentID == "" {
		return stepResult{Step: stepName, Status: "skipped", Reason: "no content id"}, nil
	}
	endpoint := cfg.BaseURL + "/api/v1/contents/" + contentID
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+mockToken)
	res, err := client.Do(req)
	if err != nil {
		return stepResult{Step: stepName, Status: "error", Reason: err.Error()}, nil
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	_ = os.WriteFile(filepath.Join(cfg.OutputDir, filename), respBody, 0o644)

	obs := &detailObservation{
		HTTPStatus: res.StatusCode,
		ContentID:  contentID,
	}

	if res.StatusCode == 200 {
		// Parse for status / lifecycle / card.lifecycle observation.
		var raw map[string]any
		if err := json.Unmarshal(respBody, &raw); err == nil {
			data := raw
			if d, ok := raw["data"].(map[string]any); ok {
				data = d
			}
			if v, ok := data["status"].(string); ok {
				obs.StatusField = v
			}
			if v, ok := data["lifecycle"].(string); ok {
				obs.LifecycleField = v
			}
			if card, ok := data["card"].(map[string]any); ok {
				if v, ok := card["lifecycle"].(string); ok {
					obs.CardLifecycle = v
				}
			}
		}
	}

	detail := fmt.Sprintf("http=%d status=%q lifecycle=%q card.lifecycle=%q", obs.HTTPStatus, obs.StatusField, obs.LifecycleField, obs.CardLifecycle)
	step := stepResult{
		Step:     stepName,
		Status:   "ok",
		HTTP:     res.StatusCode,
		Artifact: filename,
		Detail:   detail,
	}
	if mustBePresent {
		if res.StatusCode != 200 {
			step.Status = "blocked"
			step.Reason = "active detail did not return 200"
			obs.Note = "active detail did not surface created row"
		}
	} else {
		if res.StatusCode != 404 {
			step.Status = "blocked"
			step.Reason = fmt.Sprintf("deleted detail returned %d, expected 404", res.StatusCode)
			obs.Note = "deleted detail did not 404 — investigate gate"
		}
	}
	return step, obs
}

func synthesizeDetailVerdict(sum *detailSummary) string {
	var activeOK, deletedOK bool
	metricsCount := 0
	anyError := false
	for _, s := range sum.Steps {
		if s.Status == "error" {
			anyError = true
		}
		switch {
		case s.Step == "detail_active" && s.Status == "ok":
			activeOK = true
		case s.Step == "detail_deleted" && s.Status == "ok":
			deletedOK = true
		case strings.HasPrefix(s.Step, "metrics:") && s.Status == "ok":
			metricsCount++
		}
	}
	metricsOK := metricsCount >= 4

	activeAs200 := sum.ActiveDetailObservation != nil && sum.ActiveDetailObservation.HTTPStatus == 200
	deletedAs404 := sum.DeletedDetailObservation != nil && sum.DeletedDetailObservation.HTTPStatus == 404

	switch {
	case activeOK && deletedOK && metricsOK && activeAs200 && deletedAs404 && !anyError:
		return "ACTIVE_DELETE_LIFECYCLE_OBSERVED"
	case activeOK && deletedOK && metricsOK:
		return "DETAIL_AND_METRICS_OBSERVED"
	case (activeOK || deletedOK) && !metricsOK:
		return "DETAIL_OBSERVED_NO_METRICS"
	case !activeOK && !deletedOK && metricsOK:
		return "METRICS_ONLY"
	default:
		return "PARTIAL"
	}
}

func sniffDetailEnforcementApplied(beforePath, afterPath string) (bool, string) {
	beforeMap := readFeedSeries(beforePath, "labuda_evaluator_content_detail_enforcement_applied_total")
	afterMap := readFeedSeries(afterPath, "labuda_evaluator_content_detail_enforcement_applied_total")
	if len(afterMap) == 0 {
		return false, "no labuda_evaluator_content_detail_enforcement_applied_total series present in the final scrape — counter never incremented; if the legacy 404 gate fired BEFORE the enforce pass, the enforce path was short-circuited (the legacy gate is strictly more conservative than the evaluator on this corpus)"
	}
	for labels, after := range afterMap {
		before := beforeMap[labels]
		if after > before {
			return true, fmt.Sprintf("content_detail enforcement_applied_total ticked at labels {%s}: %g → %g", labels, before, after)
		}
	}
	return false, fmt.Sprintf("%d content_detail enforcement_applied_total series present but none incremented between baseline and final scrape", len(afterMap))
}

func sniffDetailShadowContinuity(activePath, deletedPath string) string {
	active := readDetailShadowRequestTotal(activePath)
	deleted := readDetailShadowRequestTotal(deletedPath)
	switch {
	case deleted > active:
		return fmt.Sprintf("content_detail shadow continuity confirmed: labuda_evaluator_shadow_request_total{surface=content_detail} advanced %g → %g", active, deleted)
	case active > 0 && deleted == active:
		return fmt.Sprintf("content_detail shadow request_total observed at %g but did not advance between active and deleted snapshots", active)
	case active == 0 && deleted == 0:
		return "content_detail shadow request_total absent in both snapshots — runner did not dispatch (EVALUATOR_SHADOW_CONTENT_DETAIL_ENABLED?)"
	default:
		return fmt.Sprintf("content_detail shadow request_total active=%g deleted=%g", active, deleted)
	}
}

func readDetailShadowRequestTotal(path string) float64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var total float64
	for _, ln := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(ln, "labuda_evaluator_shadow_request_total{") {
			continue
		}
		if !strings.Contains(ln, `surface="content_detail"`) {
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

func finishDetailSummary(sum *detailSummary, cfg governanceDetailConfig, errMsg string) error {
	sum.Finished = time.Now().UTC().Format(time.RFC3339)
	out, _ := json.MarshalIndent(sum, "", "  ")
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, "summary.json"), out, 0o644); err != nil {
		return fmt.Errorf("write summary.json: %w", err)
	}
	var readme bytes.Buffer
	fmt.Fprintf(&readme, "scenario-governance-detail\n")
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
	if sum.ActiveDetailObservation != nil {
		o := sum.ActiveDetailObservation
		fmt.Fprintf(&readme, "  active_detail:  http=%d status=%q lifecycle=%q card.lifecycle=%q\n", o.HTTPStatus, o.StatusField, o.LifecycleField, o.CardLifecycle)
	} else {
		fmt.Fprintf(&readme, "  active_detail:  (not executed)\n")
	}
	if sum.DeletedDetailObservation != nil {
		o := sum.DeletedDetailObservation
		fmt.Fprintf(&readme, "  deleted_detail: http=%d (404 expected)\n", o.HTTPStatus)
	} else {
		fmt.Fprintf(&readme, "  deleted_detail: (not executed)\n")
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
