// Package evaluator hosts the canonical visibility / governance evaluator
// and its observability seams for three surfaces: Feed, Content Detail,
// and Search Content.
//
// Business authority:
//
//   - EnforceFeed synchronously filters the /feed page slice. DENY rows
//     are dropped; TOMBSTONE / REDACT rows are kept with lifecycle
//     overrides; UNKNOWN rows are kept (fail-open).
//   - EnforceContentDetail synchronously gates /contents/:id. Any non-ALLOW
//     decision causes HTTP 404 (fail-CLOSED).
//   - EnforceSearchContent synchronously filters the /search/content page
//     slice. DENY rows are dropped; TOMBSTONE / REDACT rows are kept with
//     lifecycle overrides; UNKNOWN/input_invalid rows are dropped
//     (fail-CLOSED).
//
// All three enforce functions are unconditional — there is no mode
// parameter, no feature flag, no environment variable, and no
// constructor argument that can select an alternate business behavior.
// The handler always calls the enforce function; the business decision
// is deterministic from the pre-hydrated ViewerContext + TargetContext
// + entity inputs.
//
// Observability:
//
//   - FeedShadowRunner, ContentDetailShadowRunner, and
//     SearchContentShadowRunner are fire-and-forget goroutines dispatched
//     AFTER the handler writes the response. They emit bounded Prometheus
//     telemetry (decision distribution, divergence classification, overlay
//     completeness, latency). They never mutate the response, never
//     restore dropped rows, never change lifecycle overrides, and never
//     affect HTTP status codes. A nil runner is a documented no-op at
//     every call site.
//
// Precedence model:
//
//   - Actor lifecycle → target lifecycle → relationship → moderation →
//     visibility scope → public allow.
//   - Content Detail additionally evaluates admin/moderator bypass
//     (capability-gated) and block override.
package evaluator
