package evaluator

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Shadow telemetry surface.
//
// All metrics live under the `labuda_evaluator_shadow_` namespace and are
// registered exactly once via the package-level promauto registrations
// below. Cardinality discipline (Phase C Task E):
//
//   - surface  : enumerated SurfaceLabel (currently {feed})
//   - decision : enumerated ShadowDecision (5 values)
//   - category : enumerated DivergenceCategory (5 values; this seam emits 3)
//   - reason   : enumerated UnknownReason (4 non-empty values)
//   - overlay  : enumerated OverlayKind (5 values)
//   - status   : enumerated OverlayStatus (3 values)
//   - source   : enumerated HydrationSource (3 values)
//
// No user_id, item_id, author_id, or per-tenant dimension is ever emitted.

const metricNamespace = "labuda_evaluator_shadow"

var (
	metricRequestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "request_total",
		Help:      "Number of shadow evaluation requests dispatched per surface. Denominator for shadow rates.",
	}, []string{"surface"})

	metricDecisionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "decision_total",
		Help:      "Per-item shadow decision distribution (allow|deny|tombstone|redact|unknown). Authority is unaffected; legacy decisions remain the only runtime visibility authority.",
	}, []string{"surface", "decision"})

	metricDivergenceTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "divergence_total",
		Help:      "Per-item legacy-vs-shadow divergence classification. Per Shadow Mode Doctrine, legacy_deny_* cells are unobservable on this surface and are never emitted; only legacy_allow_* and shadow_unknown cells appear.",
	}, []string{"surface", "category"})

	metricUnknownTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "unknown_total",
		Help:      "Reasons the shadow evaluator returned UNKNOWN. UNKNOWN is healthier than false certainty (Doctrine — No Fake Operability).",
	}, []string{"surface", "reason"})

	metricOverlayStatusTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "overlay_status_total",
		Help:      "Per-overlay hydration status counts (present|missing|error) at shadow request time. Used to measure ViewerContext Contract overlay propagation (BLOCKER-002 observability).",
	}, []string{"surface", "overlay", "status"})

	metricAnonymousTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "anonymous_total",
		Help:      "Number of shadow runs with an anonymous viewer. Feed runtime requires authenticated identity; nonzero values here indicate caller-side construction defects.",
	}, []string{"surface"})

	metricHydrationErrorTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "hydration_error_total",
		Help:      "Per-source overlay hydration errors. Each error degrades a downstream decision to UNKNOWN; never to a synthesized fallback.",
	}, []string{"surface", "source"})

	metricEvaluationLatencyMs = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricNamespace,
		Name:      "evaluation_latency_ms",
		Help:      "End-to-end shadow evaluation latency in milliseconds (overlay hydration + per-item evaluation + telemetry emission). Independent of runtime response latency.",
		Buckets:   []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500},
	}, []string{"surface"})
)

// shadowMetrics is a minimal facade so non-telemetry code does not import
// prometheus directly and so tests can substitute a no-op.
type shadowMetrics struct{}

func newShadowMetrics() *shadowMetrics { return &shadowMetrics{} }

func (m *shadowMetrics) recordRequest(surface SurfaceLabel) {
	metricRequestTotal.WithLabelValues(string(surface)).Inc()
}

func (m *shadowMetrics) recordDecision(surface SurfaceLabel, decision ShadowDecision) {
	metricDecisionTotal.WithLabelValues(string(surface), string(decision)).Inc()
}

func (m *shadowMetrics) recordDivergence(surface SurfaceLabel, category DivergenceCategory) {
	metricDivergenceTotal.WithLabelValues(string(surface), string(category)).Inc()
}

func (m *shadowMetrics) recordUnknown(surface SurfaceLabel, reason UnknownReason) {
	if reason == UnknownReasonNone {
		return
	}
	metricUnknownTotal.WithLabelValues(string(surface), string(reason)).Inc()
}

func (m *shadowMetrics) recordOverlayStatus(surface SurfaceLabel, overlay OverlayKind, status OverlayStatus) {
	metricOverlayStatusTotal.WithLabelValues(string(surface), string(overlay), string(status)).Inc()
}

func (m *shadowMetrics) recordAnonymous(surface SurfaceLabel) {
	metricAnonymousTotal.WithLabelValues(string(surface)).Inc()
}

func (m *shadowMetrics) recordHydrationError(surface SurfaceLabel, source HydrationSource) {
	metricHydrationErrorTotal.WithLabelValues(string(surface), string(source)).Inc()
}

func (m *shadowMetrics) observeLatency(surface SurfaceLabel, ms float64) {
	metricEvaluationLatencyMs.WithLabelValues(string(surface)).Observe(ms)
}


