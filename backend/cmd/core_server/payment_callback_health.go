package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/labuda/backend/internal/config"
)

// ============================================================================
// PAYMENT CALLBACK READINESS (INFRA-2)
// ============================================================================
// The Midtrans callback endpoint is payment infrastructure: if the gateway
// cannot deliver its notification, a payment that succeeded at the gateway
// never settles in Labuda (PAY-INFRA-01). Readiness must therefore be able to
// tell an operator whether that path is usable — WITHOUT pretending that a
// syntactically valid URL is a working endpoint.
//
// HONESTY CONTRACT: every field below states only what was actually proven by
// THIS process. Two facts cannot be established from inside the backend and are
// therefore reported as unverifiable rather than assumed healthy:
//
//   - public ingress reachability (the process calling its own public URL does
//     not prove Midtrans can reach it — NAT/egress asymmetry is invisible here);
//   - actual gateway delivery (only a real delivered callback proves that).
//
// This file deliberately does NOT create a second health surface. It feeds the
// existing canonical readiness authority (evaluateReadiness / GET /health/ready).

const (
	callbackStateReady           = "ready"
	callbackStateNotConfigured   = "not_configured"
	callbackStateConfigInvalid   = "config_invalid"
	callbackStateHostUnresolved  = "host_unresolvable"
	callbackStateRouteNotMounted = "route_not_mounted"

	callbackOutboundReachable   = "reachable"
	callbackOutboundUnreachable = "unreachable"
	callbackOutboundSkipped     = "skipped"

	// notVerifiableFromProcess is the only honest verdict for a fact this
	// process cannot establish. It is never rendered as a healthy value.
	notVerifiableFromProcess = "not_verifiable_from_process"

	callbackResolveTimeout = 2 * time.Second
	callbackProbeTimeout   = 2 * time.Second
)

// PaymentCallbackHealth is the readiness view of the payment callback
// dependency. It is a flat, factual report: configuration validity, local route
// registration, and the environment-local reachability evidence — each labelled
// separately so a valid configuration can never be mistaken for a reachable
// endpoint.
type PaymentCallbackHealth struct {
	State    string `json:"state"`
	Degraded bool   `json:"degraded"`
	Reason   string `json:"reason,omitempty"`

	// Configuration facts (INFRA-1 rules; pure evaluation, no I/O).
	Configured  bool   `json:"configured"`
	ConfigError string `json:"config_error,omitempty"`
	Host        string `json:"host,omitempty"`
	Path        string `json:"path,omitempty"`

	// Local routing fact: the canonical callback route is registered on the
	// server that is answering this request.
	RouteMounted bool `json:"route_mounted"`

	// Environment-local reachability evidence.
	DNSResolves    bool   `json:"dns_resolves"`
	DNSError       string `json:"dns_error,omitempty"`
	OutboundProbe  string `json:"outbound_probe"` // reachable | unreachable | skipped
	OutboundDetail string `json:"outbound_detail,omitempty"`

	// Boundaries that cannot be established by this process.
	PublicIngress   string `json:"public_ingress"`
	GatewayDelivery string `json:"gateway_delivery"`
}

// callbackProbeInput carries the environment-local facts a probe observed.
type callbackProbeInput struct {
	RouteMounted   bool
	DNSResolves    bool
	DNSError       string
	OutboundProbe  string
	OutboundDetail string
}

// callbackResolver resolves a callback hostname. Injected so readiness tests
// never depend on the real internet.
type callbackResolver func(ctx context.Context, host string) error

// readinessCallbackProbe supplies the payment-callback readiness view for a
// single readiness request. Production wiring passes probePaymentCallbackHealth
// bound to the router-derived route-mounted fact; tests inject a fixed view so
// readiness stays deterministic and offline.
type readinessCallbackProbe func(ctx context.Context, cfg *config.Config) PaymentCallbackHealth

// assessPaymentCallbackHealth is the PURE decision function: given the
// configuration and the observed environment-local facts, it produces the
// readiness view. No I/O, so it can be tested deterministically.
//
// Severity doctrine (mirrors the existing readiness semantics for critical
// money-safety detectors and the payout completion loop):
//
//   - Any callback defect sets Degraded, which makes readiness fail OUTSIDE
//     development while development stays ready-but-visible. An unresolvable
//     callback host is a proven silent-loss class, not a cosmetic warning.
//   - Outbound probe failure does NOT set Degraded. A process-local outbound
//     failure cannot distinguish "our egress is blocked" from "the public
//     endpoint is down", so it is reported as evidence, never as a gate.
//
// Precedence: not_configured > config_invalid > host_unresolvable >
// route_not_mounted > ready.
func assessPaymentCallbackHealth(cfg *config.Config, in callbackProbeInput) PaymentCallbackHealth {
	health := PaymentCallbackHealth{
		RouteMounted:    in.RouteMounted,
		DNSResolves:     in.DNSResolves,
		DNSError:        in.DNSError,
		OutboundProbe:   in.OutboundProbe,
		OutboundDetail:  in.OutboundDetail,
		Path:            canonicalPaymentWebhookPath,
		PublicIngress:   notVerifiableFromProcess,
		GatewayDelivery: notVerifiableFromProcess,
	}

	raw := ""
	if cfg != nil {
		raw = strings.TrimSpace(cfg.Midtrans.NotificationURL)
	}
	health.Configured = raw != ""

	if !health.Configured {
		health.State = callbackStateNotConfigured
		health.Degraded = true
		health.Reason = "MIDTRANS_NOTIFICATION_URL is not set; the gateway has no repository-owned callback target"
		return health
	}

	if err := validateMidtransNotificationURL(raw); err != nil {
		health.State = callbackStateConfigInvalid
		health.Degraded = true
		health.ConfigError = err.Error()
		health.Reason = "callback configuration is structurally invalid: " + err.Error()
		return health
	}

	if parsed, err := url.Parse(raw); err == nil {
		health.Host = parsed.Hostname()
		if trimmedPath := strings.TrimRight(parsed.Path, "/"); trimmedPath != "" {
			health.Path = trimmedPath
		}
	}

	if !in.DNSResolves {
		health.State = callbackStateHostUnresolved
		health.Degraded = true
		health.Reason = fmt.Sprintf(
			"callback host %q does not resolve in this environment; the payment gateway cannot deliver to it", health.Host)
		return health
	}

	if !in.RouteMounted {
		health.State = callbackStateRouteNotMounted
		health.Degraded = true
		health.Reason = fmt.Sprintf(
			"canonical callback route POST %s is not registered on this server", canonicalPaymentWebhookPath)
		return health
	}

	health.State = callbackStateReady
	return health
}

// lookupCallbackHost is the production resolver used by readiness.
func lookupCallbackHost(ctx context.Context, host string) error {
	_, err := net.DefaultResolver.LookupHost(ctx, host)
	return err
}

// callbackProbeClient is the shared client used for the outbound probe.
// Redirects are not followed: any HTTP response — including the 404 a GET gets
// on the POST-only callback route — proves the host answers over HTTPS, and
// chasing a redirect to another host would prove nothing about the callback
// target.
var callbackProbeClient = &http.Client{
	Timeout: callbackProbeTimeout,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// probeCallbackOutbound issues exactly one GET against the callback URL. Any
// HTTP response counts as reachable. It never performs a payment operation: the
// canonical route accepts POST only, so a GET cannot trigger settlement.
func probeCallbackOutbound(ctx context.Context, client *http.Client, callbackURL string) (string, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, callbackURL, nil)
	if err != nil {
		return callbackOutboundUnreachable, "build request: " + err.Error()
	}

	resp, err := client.Do(req)
	if err != nil {
		return callbackOutboundUnreachable, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))

	return callbackOutboundReachable, fmt.Sprintf("http_status=%d", resp.StatusCode)
}

// probePaymentCallbackHealthWith composes the observed facts. The resolver and
// client are parameters so tests can supply fakes and stay offline.
//
// The probes only run when the configuration itself is sound: probing an
// unusable target would report noise as if it were signal.
func probePaymentCallbackHealthWith(
	ctx context.Context,
	cfg *config.Config,
	routeMounted bool,
	resolve callbackResolver,
	client *http.Client,
) PaymentCallbackHealth {
	in := callbackProbeInput{
		RouteMounted:  routeMounted,
		OutboundProbe: callbackOutboundSkipped,
	}

	raw := ""
	if cfg != nil {
		raw = strings.TrimSpace(cfg.Midtrans.NotificationURL)
	}

	if raw != "" && validateMidtransNotificationURL(raw) == nil {
		parsed, parseErr := url.Parse(raw)
		if parseErr == nil && resolve != nil {
			resolveCtx, cancel := context.WithTimeout(ctx, callbackResolveTimeout)
			if err := resolve(resolveCtx, parsed.Hostname()); err != nil {
				in.DNSError = err.Error()
			} else {
				in.DNSResolves = true
			}
			cancel()
		}

		if in.DNSResolves && client != nil {
			probeCtx, cancel := context.WithTimeout(ctx, callbackProbeTimeout)
			in.OutboundProbe, in.OutboundDetail = probeCallbackOutbound(probeCtx, client, raw)
			cancel()
		}
	}

	return assessPaymentCallbackHealth(cfg, in)
}

// probePaymentCallbackHealth is the production entry point used by the
// canonical readiness handler.
func probePaymentCallbackHealth(ctx context.Context, cfg *config.Config, routeMounted bool) PaymentCallbackHealth {
	return probePaymentCallbackHealthWith(ctx, cfg, routeMounted, lookupCallbackHost, callbackProbeClient)
}

// callbackRouteMounted reports whether the canonical payment callback route is
// actually registered on this router. It is read from the router itself so the
// readiness response cannot claim a route exists when it does not.
func callbackRouteMounted(router *gin.Engine) bool {
	if router == nil {
		return false
	}
	for _, route := range router.Routes() {
		if route.Method == http.MethodPost && route.Path == canonicalPaymentWebhookPath {
			return true
		}
	}
	return false
}
