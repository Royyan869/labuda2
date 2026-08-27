package evaluator

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Search shadow telemetry surface.
//
// PHASE C — SEARCH SHADOW SEAM STAGE 1 (TELEMETRY ONLY)
//
// All metrics live under the `search_shadow_` namespace per
// docs/05-rollout/search-endpoint-telemetry-enum-design.md §13.10 and
// are registered exactly once via the package-level promauto registrations
// below. Distinct from the feed seam's `labuda_evaluator_shadow_*`
// namespace so cross-surface metric collision is impossible.
//
// First-seam landing on /search/content registers ONLY the bounded
// label set this endpoint emits per docs/05-rollout/search-shadow-seam-
// landing-task-design.md §8.11. Every metric carries surface, endpoint,
// candidate_set_option labels — endpointless metrics and global
// agreement-% metrics are FORBIDDEN per docs/05-rollout/search-endpoint-
// telemetry-enum-design.md §15.
//
// Cardinality discipline (cells per metric ceiling per
// docs/05-rollout/search-endpoint-telemetry-enum-design.md §11.2):
//
//   - surface              : 1 value at this metric subset (search)
//   - endpoint             : 1 value at first-seam landing (search_content)
//   - candidate_set_option : 1 value at first-seam landing (option_a_handler_post_response)
//   - divergence_cell      : 7
//   - exposure_semantic    : 7
//   - unknown_reason       : 7
//   - unknown_source       : 6 (registered subset for /search/content)
//   - overlay              : 6 (registered subset)
//   - overlay_status       : 3
//   - lifecycle_category   : 3 (registered subset)
//   - public_lifecycle_state : 3
//
// FORBIDDEN labels (per §15 / §11.3):
//   - search query string content
//   - viewer / target / content / author / listing / auction IDs (raw UUIDs)
//   - email content
//   - raw account_status / *.status / deleted_at
//   - SQL error text / stack traces
//   - bid_count / popularity / rank_position / page (numeric per-row labels)
//   - per-endpoint overlay variants (e.g., content_moderation)
//   - emoji / non-ASCII tokens
//   - omitempty-toggled labels

const searchShadowMetricNamespace = "search_shadow"

var (
	searchShadowRequestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: searchShadowMetricNamespace,
		Name:      "request_total",
		Help:      "Number of search shadow evaluation requests dispatched per endpoint. Denominator for per-endpoint shadow rates per docs/05-rollout/search-endpoint-telemetry-enum-design.md §13.1.",
	}, []string{"surface", "endpoint", "candidate_set_option"})

	searchShadowDecisionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: searchShadowMetricNamespace,
		Name:      "decision_total",
		Help:      "Per-row shadow evaluator decision distribution by canonical exposure semantic per docs/05-rollout/search-endpoint-telemetry-enum-design.md §13.2. Authority is unaffected; legacy decisions remain the only runtime visibility authority.",
	}, []string{"surface", "endpoint", "candidate_set_option", "exposure_semantic"})

	searchShadowDivergenceTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: searchShadowMetricNamespace,
		Name:      "divergence_total",
		Help:      "Per-row legacy-vs-shadow divergence cell distribution per docs/05-rollout/search-endpoint-telemetry-enum-design.md §13.6. legacy_deny_* cells are RESERVED-NOT-EMITTED on Option A per search-shadow-seam-architecture.md §3.2.",
	}, []string{"surface", "endpoint", "candidate_set_option", "divergence_cell"})

	searchShadowUnknownTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: searchShadowMetricNamespace,
		Name:      "unknown_total",
		Help:      "Per-row UNKNOWN events with bounded reason and source classification per docs/05-rollout/search-endpoint-telemetry-enum-design.md §13.3. Per-source decomposition is the canonical operational triage path.",
	}, []string{"surface", "endpoint", "candidate_set_option", "unknown_reason", "unknown_source"})

	searchShadowOverlayStatusTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: searchShadowMetricNamespace,
		Name:      "overlay_status_total",
		Help:      "Per-overlay completeness telemetry per shadow run per docs/05-rollout/search-endpoint-telemetry-enum-design.md §13.4. Used to measure ViewerContext propagation (BLOCKER-002) and lifecycle/moderation hydration (BLOCKER-008) on the search seam.",
	}, []string{"surface", "endpoint", "candidate_set_option", "overlay", "overlay_status"})

	searchShadowLifecycleStateTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: searchShadowMetricNamespace,
		Name:      "lifecycle_state_total",
		Help:      "Per-row lifecycle state observation per shadow run per docs/05-rollout/search-endpoint-telemetry-enum-design.md §13.5. Coarsened to the canonical Public Lifecycle State; raw account_status / *.status / deleted_at FORBIDDEN as labels per §8.5.",
	}, []string{"surface", "endpoint", "candidate_set_option", "lifecycle_category", "public_lifecycle_state"})

	searchShadowHydrationErrorTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: searchShadowMetricNamespace,
		Name:      "hydration_error_total",
		Help:      "Per-overlay hydration failure events per docs/05-rollout/search-endpoint-telemetry-enum-design.md §13.7. SQL error text FORBIDDEN as label; sanitized to bounded enum classifications.",
	}, []string{"surface", "endpoint", "candidate_set_option", "unknown_source"})

	searchShadowLatencySeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: searchShadowMetricNamespace,
		Name:      "latency_seconds",
		Help:      "Per-runner latency (per-row evaluation + telemetry emission) in seconds per docs/05-rollout/search-endpoint-telemetry-enum-design.md §13.8. Independent of legacy response latency.",
		Buckets:   []float64{0.001, 0.005, 0.010, 0.025, 0.050, 0.100, 0.250, 0.500, 1.000, 2.500},
	}, []string{"surface", "endpoint", "candidate_set_option"})

	searchShadowDenominatorHealthTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: searchShadowMetricNamespace,
		Name:      "denominator_health_total",
		Help:      "Emitted on candidate_set_incomplete UNKNOWN events per docs/05-rollout/search-endpoint-telemetry-enum-design.md §13.9. A non-zero rate undermines per-endpoint denominator interpretation of all other metrics.",
	}, []string{"surface", "endpoint", "candidate_set_option"})

	searchShadowAnonymousTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: searchShadowMetricNamespace,
		Name:      "anonymous_total",
		Help:      "Number of search shadow runs whose ViewerContext was AnonymousViewer per docs/05-rollout/search-shadow-seam-architecture.md §9.1. /search/content is Pattern A — anonymous traffic is legitimate; the metric supports per-endpoint anonymous-rate baselines.",
	}, []string{"surface", "endpoint", "candidate_set_option"})

	// BATCH 3A — Evaluator promotion prerequisite counters.
	//
	// These two metrics close the operational gap between "shadow observes
	// divergence" and "we are ready to enforce". Both keep cardinality
	// bounded; values come from the small AdapterMode + AdapterReason
	// enumerations declared in search_content_adapter.go.
	searchEvaluatorWouldEnforceDecisionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: searchShadowMetricNamespace,
		Name:      "would_enforce_decision_total",
		Help:      "Per-row count of how the adapter WOULD classify the decision if /search/content were running in enforce mode. Emitted unconditionally in shadow mode for promotion safety telemetry (Batch 3A). Labels are bounded to the SearchContentDecisionReason enum.",
	}, []string{"surface", "endpoint", "candidate_set_option", "adapter_reason"})

	searchEvaluatorEnforceModeTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: searchShadowMetricNamespace,
		Name:      "enforce_mode_total",
		Help:      "Per-request count of the operating mode (shadow|enforce) of the /search/content evaluator integration. Used to correlate would_enforce_decision_total rates with the route's current operating mode (Batch 3A).",
	}, []string{"surface", "endpoint", "candidate_set_option", "mode"})

	// BATCH 3B — Per-row enforcement action counter. Fires ONLY from the
	// synchronous EnforceSearchContent helper when the handler is in
	// enforce mode AND the adapter decision required an action
	// ("drop" — row excluded; "lifecycle_override" — card lifecycle
	// coarsened). The dominant pass-through case is intentionally NOT
	// counted (it equals decision_total{semantic=allow}).
	searchEvaluatorEnforcementAppliedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: searchShadowMetricNamespace,
		Name:      "enforcement_applied_total",
		Help:      "Per-row count of enforcement actions taken by the handler synchronous pass in /search/content enforce mode. Bounded labels: action in {drop, lifecycle_override}. Pass-through (no action) is intentionally not counted.",
	}, []string{"surface", "endpoint", "candidate_set_option", "action"})
)

// searchShadowMetrics is the minimal facade so non-telemetry code does
// not import prometheus directly and tests can construct the runner
// without the registry.
type searchShadowMetrics struct{}

func newSearchShadowMetrics() *searchShadowMetrics { return &searchShadowMetrics{} }

func (m *searchShadowMetrics) recordRequest(endpoint SearchEndpoint, opt CandidateSetOption) {
	searchShadowRequestTotal.WithLabelValues(string(SurfaceSearch), string(endpoint), string(opt)).Inc()
}

func (m *searchShadowMetrics) recordDecision(endpoint SearchEndpoint, opt CandidateSetOption, semantic SearchExposureSemantic) {
	searchShadowDecisionTotal.WithLabelValues(string(SurfaceSearch), string(endpoint), string(opt), string(semantic)).Inc()
}

func (m *searchShadowMetrics) recordDivergence(endpoint SearchEndpoint, opt CandidateSetOption, cell SearchDivergenceCell) {
	searchShadowDivergenceTotal.WithLabelValues(string(SurfaceSearch), string(endpoint), string(opt), string(cell)).Inc()
}

func (m *searchShadowMetrics) recordUnknown(endpoint SearchEndpoint, opt CandidateSetOption, reason SearchUnknownReason, source SearchUnknownSource) {
	if reason == SearchUnknownReasonNone {
		return
	}
	searchShadowUnknownTotal.WithLabelValues(string(SurfaceSearch), string(endpoint), string(opt), string(reason), string(source)).Inc()
}

func (m *searchShadowMetrics) recordOverlayStatus(endpoint SearchEndpoint, opt CandidateSetOption, overlay SearchOverlay, status SearchOverlayStatus) {
	searchShadowOverlayStatusTotal.WithLabelValues(string(SurfaceSearch), string(endpoint), string(opt), string(overlay), string(status)).Inc()
}

func (m *searchShadowMetrics) recordLifecycleState(endpoint SearchEndpoint, opt CandidateSetOption, category SearchLifecycleCategory, state SearchPublicLifecycleState) {
	searchShadowLifecycleStateTotal.WithLabelValues(string(SurfaceSearch), string(endpoint), string(opt), string(category), string(state)).Inc()
}

func (m *searchShadowMetrics) recordHydrationError(endpoint SearchEndpoint, opt CandidateSetOption, source SearchUnknownSource) {
	searchShadowHydrationErrorTotal.WithLabelValues(string(SurfaceSearch), string(endpoint), string(opt), string(source)).Inc()
}

func (m *searchShadowMetrics) observeLatency(endpoint SearchEndpoint, opt CandidateSetOption, seconds float64) {
	searchShadowLatencySeconds.WithLabelValues(string(SurfaceSearch), string(endpoint), string(opt)).Observe(seconds)
}

func (m *searchShadowMetrics) recordDenominatorHealth(endpoint SearchEndpoint, opt CandidateSetOption) {
	searchShadowDenominatorHealthTotal.WithLabelValues(string(SurfaceSearch), string(endpoint), string(opt)).Inc()
}

func (m *searchShadowMetrics) recordAnonymous(endpoint SearchEndpoint, opt CandidateSetOption) {
	searchShadowAnonymousTotal.WithLabelValues(string(SurfaceSearch), string(endpoint), string(opt)).Inc()
}

// recordWouldEnforceDecision emits the per-row adapter classification
// (would_enforce_decision_total). Called from the shadow runner once per
// evaluated row alongside recordDecision; the existing decision_total
// metric captures the raw evaluator semantic, while this metric reports
// the adapter's enforcement-ready outcome. See AdaptSearchContentDecision.
func (m *searchShadowMetrics) recordWouldEnforceDecision(endpoint SearchEndpoint, opt CandidateSetOption, reason SearchContentDecisionReason) {
	if reason == SearchContentDecisionReasonNone {
		// ALLOW path with no override produces no metric emission; the
		// dominant agreement cell is captured by divergence_total.
		return
	}
	searchEvaluatorWouldEnforceDecisionTotal.WithLabelValues(string(SurfaceSearch), string(endpoint), string(opt), string(reason)).Inc()
}

// recordEnforceMode emits the per-request operating-mode label
// (enforce_mode_total). Called once per shadow run from runShadow.
func (m *searchShadowMetrics) recordEnforceMode(endpoint SearchEndpoint, opt CandidateSetOption, mode SearchContentAdapterMode) {
	searchEvaluatorEnforceModeTotal.WithLabelValues(string(SurfaceSearch), string(endpoint), string(opt), string(mode)).Inc()
}

// recordEnforcementApplied emits enforcement_applied_total for one
// row-level action taken by EnforceSearchContent. Pass-through (no
// action) MUST short-circuit at the caller — this method silently no-ops
// on SearchContentEnforcementActionNone so accidental no-op emissions
// do not inflate the metric.
func (m *searchShadowMetrics) recordEnforcementApplied(action SearchContentEnforcementAction) {
	if action == SearchContentEnforcementActionNone {
		return
	}
	searchEvaluatorEnforcementAppliedTotal.WithLabelValues(
		string(SurfaceSearch),
		string(SearchEndpointContent),
		string(CandidateSetOptionAHandlerPostResponse),
		string(action),
	).Inc()
}


