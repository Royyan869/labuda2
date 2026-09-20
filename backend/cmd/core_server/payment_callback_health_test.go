package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/labuda/backend/internal/config"
)

// stubTransport makes the outbound probe fully deterministic and offline: no
// socket is opened, yet the real probeCallbackOutbound code path runs.
type stubTransport struct {
	status int
	err    error
}

func (s stubTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(strings.NewReader("stub")),
		Header:     make(http.Header),
	}, nil
}

func stubClient(status int, err error) *http.Client {
	return &http.Client{Transport: stubTransport{status: status, err: err}}
}

func callbackCfg(url string) *config.Config {
	return &config.Config{
		Server:   config.ServerConfig{Env: "development"},
		Midtrans: config.MidtransConfig{NotificationURL: url},
	}
}

const validCallbackURL = "https://labuda-dev.example.com" + canonicalPaymentWebhookPath

// --- pure assessment -------------------------------------------------------

func TestAssessPaymentCallbackHealth_States(t *testing.T) {
	cases := []struct {
		name          string
		cfg           *config.Config
		in            callbackProbeInput
		wantState     string
		wantDegraded  bool
		wantReasonSub string
	}{
		{
			name:          "not configured",
			cfg:           callbackCfg(""),
			in:            callbackProbeInput{OutboundProbe: callbackOutboundSkipped},
			wantState:     callbackStateNotConfigured,
			wantDegraded:  true,
			wantReasonSub: "MIDTRANS_NOTIFICATION_URL",
		},
		{
			name:          "nil config is not configured, not a panic",
			cfg:           nil,
			in:            callbackProbeInput{OutboundProbe: callbackOutboundSkipped},
			wantState:     callbackStateNotConfigured,
			wantDegraded:  true,
			wantReasonSub: "MIDTRANS_NOTIFICATION_URL",
		},
		{
			name:          "config invalid: non-canonical path",
			cfg:           callbackCfg("https://labuda-dev.example.com/api/v1/payments/webhook"),
			in:            callbackProbeInput{OutboundProbe: callbackOutboundSkipped},
			wantState:     callbackStateConfigInvalid,
			wantDegraded:  true,
			wantReasonSub: "must be",
		},
		{
			name:          "config invalid: http scheme",
			cfg:           callbackCfg("http://labuda-dev.example.com" + canonicalPaymentWebhookPath),
			in:            callbackProbeInput{OutboundProbe: callbackOutboundSkipped},
			wantState:     callbackStateConfigInvalid,
			wantDegraded:  true,
			wantReasonSub: "https",
		},
		{
			name: "host unresolvable outranks route-not-mounted",
			cfg:  callbackCfg(validCallbackURL),
			in: callbackProbeInput{
				RouteMounted:  false,
				DNSResolves:   false,
				DNSError:      "lookup labuda-dev.example.com: no such host",
				OutboundProbe: callbackOutboundSkipped,
			},
			wantState:     callbackStateHostUnresolved,
			wantDegraded:  true,
			wantReasonSub: "does not resolve",
		},
		{
			name: "route not mounted",
			cfg:  callbackCfg(validCallbackURL),
			in: callbackProbeInput{
				RouteMounted:  false,
				DNSResolves:   true,
				OutboundProbe: callbackOutboundReachable,
			},
			wantState:     callbackStateRouteNotMounted,
			wantDegraded:  true,
			wantReasonSub: canonicalPaymentWebhookPath,
		},
		{
			name: "ready",
			cfg:  callbackCfg(validCallbackURL),
			in: callbackProbeInput{
				RouteMounted:   true,
				DNSResolves:    true,
				OutboundProbe:  callbackOutboundReachable,
				OutboundDetail: "http_status=404",
			},
			wantState:    callbackStateReady,
			wantDegraded: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := assessPaymentCallbackHealth(tc.cfg, tc.in)

			if got.State != tc.wantState {
				t.Errorf("state = %q, want %q (reason=%q)", got.State, tc.wantState, got.Reason)
			}
			if got.Degraded != tc.wantDegraded {
				t.Errorf("degraded = %v, want %v", got.Degraded, tc.wantDegraded)
			}
			if tc.wantDegraded && got.Reason == "" {
				t.Error("a degraded callback view must always explain itself with a reason")
			}
			if tc.wantReasonSub != "" && !strings.Contains(got.Reason+got.ConfigError, tc.wantReasonSub) {
				t.Errorf("reason/config_error %q does not mention %q", got.Reason+got.ConfigError, tc.wantReasonSub)
			}

			// The two facts this process cannot establish must never be rendered
			// as healthy values, in ANY state.
			if got.PublicIngress != notVerifiableFromProcess {
				t.Errorf("public_ingress = %q, want %q", got.PublicIngress, notVerifiableFromProcess)
			}
			if got.GatewayDelivery != notVerifiableFromProcess {
				t.Errorf("gateway_delivery = %q, want %q", got.GatewayDelivery, notVerifiableFromProcess)
			}
		})
	}
}

// TestAssessPaymentCallbackHealth_OutboundFailureNeverDegrades proves the
// deliberate severity choice at the source: an outbound-probe failure is
// evidence, never a gate.
func TestAssessPaymentCallbackHealth_OutboundFailureNeverDegrades(t *testing.T) {
	got := assessPaymentCallbackHealth(callbackCfg(validCallbackURL), callbackProbeInput{
		RouteMounted:   true,
		DNSResolves:    true,
		OutboundProbe:  callbackOutboundUnreachable,
		OutboundDetail: "dial tcp: connection refused",
	})

	if got.State != callbackStateReady {
		t.Errorf("state = %q, want %q", got.State, callbackStateReady)
	}
	if got.Degraded {
		t.Error("an unreachable outbound probe must not degrade the callback view by itself")
	}
	if got.OutboundProbe != callbackOutboundUnreachable {
		t.Errorf("outbound_probe = %q, want %q", got.OutboundProbe, callbackOutboundUnreachable)
	}
}

// --- composed probe --------------------------------------------------------

// TestProbePaymentCallbackHealthWith_SkipsProbesWhenConfigInvalid proves an
// unusable configuration is reported as such without probing: probing an
// invalid target would present noise as signal.
func TestProbePaymentCallbackHealthWith_SkipsProbesWhenConfigInvalid(t *testing.T) {
	resolveCalls, probeCalls := 0, 0

	got := probePaymentCallbackHealthWith(
		context.Background(),
		callbackCfg("https://labuda-dev.example.com/api/v1/payments/webhook"),
		true,
		func(context.Context, string) error { resolveCalls++; return nil },
		&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			probeCalls++
			return nil, errors.New("must not be called")
		})},
	)

	if got.State != callbackStateConfigInvalid {
		t.Errorf("state = %q, want %q", got.State, callbackStateConfigInvalid)
	}
	if got.OutboundProbe != callbackOutboundSkipped {
		t.Errorf("outbound_probe = %q, want %q (no probing of an invalid target)", got.OutboundProbe, callbackOutboundSkipped)
	}
	if resolveCalls != 0 || probeCalls != 0 {
		t.Errorf("probes must not run for an invalid configuration (resolve=%d probe=%d)", resolveCalls, probeCalls)
	}
}

// TestProbePaymentCallbackHealthWith_ResolveFailureSkipsOutbound proves a
// non-resolving callback host is reported honestly and short-circuits before any
// HTTP is attempted.
func TestProbePaymentCallbackHealthWith_ResolveFailureSkipsOutbound(t *testing.T) {
	probeCalls := 0

	got := probePaymentCallbackHealthWith(
		context.Background(),
		callbackCfg(validCallbackURL),
		true,
		func(context.Context, string) error { return errors.New("lookup: no such host") },
		&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			probeCalls++
			return nil, errors.New("must not be called")
		})},
	)

	if got.State != callbackStateHostUnresolved {
		t.Errorf("state = %q, want %q", got.State, callbackStateHostUnresolved)
	}
	if got.DNSResolves {
		t.Error("dns_resolves must be false when resolution fails")
	}
	if !strings.Contains(got.DNSError, "no such host") {
		t.Errorf("dns_error should carry the resolver failure, got %q", got.DNSError)
	}
	if got.OutboundProbe != callbackOutboundSkipped {
		t.Errorf("outbound_probe = %q, want %q", got.OutboundProbe, callbackOutboundSkipped)
	}
	if probeCalls != 0 {
		t.Errorf("outbound probe must not run when the host does not resolve (calls=%d)", probeCalls)
	}
}

// TestProbePaymentCallbackHealthWith_ReachableAndUnreachable proves the composed
// probe reports both outbound outcomes without touching the network.
func TestProbePaymentCallbackHealthWith_ReachableAndUnreachable(t *testing.T) {
	resolveOK := func(context.Context, string) error { return nil }

	reachable := probePaymentCallbackHealthWith(
		context.Background(), callbackCfg(validCallbackURL), true, resolveOK, stubClient(http.StatusNotFound, nil))
	if reachable.State != callbackStateReady {
		t.Fatalf("state = %q, want %q", reachable.State, callbackStateReady)
	}
	if reachable.OutboundProbe != callbackOutboundReachable {
		t.Errorf("outbound_probe = %q, want %q", reachable.OutboundProbe, callbackOutboundReachable)
	}
	if !strings.Contains(reachable.OutboundDetail, "http_status=404") {
		t.Errorf("outbound_detail should carry the HTTP status, got %q", reachable.OutboundDetail)
	}
	if reachable.Degraded {
		t.Error("a reachable callback endpoint must not be degraded")
	}

	unreachable := probePaymentCallbackHealthWith(
		context.Background(), callbackCfg(validCallbackURL), true, resolveOK,
		stubClient(0, errors.New("dial tcp: connection refused")))
	if unreachable.State != callbackStateReady {
		t.Errorf("state = %q, want %q (an outbound failure is evidence, not a defect)", unreachable.State, callbackStateReady)
	}
	if unreachable.OutboundProbe != callbackOutboundUnreachable {
		t.Errorf("outbound_probe = %q, want %q", unreachable.OutboundProbe, callbackOutboundUnreachable)
	}
	if unreachable.Degraded {
		t.Error("an unreachable outbound probe must not degrade the callback view")
	}
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// --- outbound probe safety -------------------------------------------------

// TestProbeCallbackOutbound_UsesGETAgainstARealServer proves the probe performs
// a plain GET against the callback URL and nothing else. The canonical callback
// route accepts POST only, so a readiness check can never trigger settlement.
func TestProbeCallbackOutbound_UsesGETAgainstARealServer(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		// Simulate the real route shape: POST is the only accepted method.
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	verdict, detail := probeCallbackOutbound(context.Background(), server.Client(), server.URL+canonicalPaymentWebhookPath)

	if gotMethod != http.MethodGet {
		t.Errorf("probe method = %q, want %q — a readiness check must never POST to the callback route", gotMethod, http.MethodGet)
	}
	if gotPath != canonicalPaymentWebhookPath {
		t.Errorf("probe path = %q, want %q", gotPath, canonicalPaymentWebhookPath)
	}
	if verdict != callbackOutboundReachable {
		t.Errorf("verdict = %q, want %q (any HTTP response proves the host answers)", verdict, callbackOutboundReachable)
	}
	if !strings.Contains(detail, "404") {
		t.Errorf("detail should report the observed status, got %q", detail)
	}
}

// TestProbeCallbackOutbound_UnreachableOnTransportFailure proves a transport
// failure is reported as unreachable rather than as an error or a healthy value.
func TestProbeCallbackOutbound_UnreachableOnTransportFailure(t *testing.T) {
	verdict, detail := probeCallbackOutbound(
		context.Background(),
		stubClient(0, errors.New("dial tcp 203.0.113.7:443: connect: connection refused")),
		validCallbackURL,
	)

	if verdict != callbackOutboundUnreachable {
		t.Errorf("verdict = %q, want %q", verdict, callbackOutboundUnreachable)
	}
	if !strings.Contains(detail, "connection refused") {
		t.Errorf("detail should carry the transport error, got %q", detail)
	}
}

// --- route-mounted fact ----------------------------------------------------

// TestCallbackRouteMounted proves the fact is read from the router itself, so
// readiness cannot claim a route exists when it does not.
func TestCallbackRouteMounted(t *testing.T) {
	if callbackRouteMounted(nil) {
		t.Error("a nil router must not report the callback route as mounted")
	}

	empty := gin.New()
	if callbackRouteMounted(empty) {
		t.Error("an empty router must not report the callback route as mounted")
	}

	postOnly := gin.New()
	postOnly.POST(canonicalPaymentWebhookPath, func(c *gin.Context) {})
	if !callbackRouteMounted(postOnly) {
		t.Error("expected the canonical POST callback route to be detected")
	}

	wrongMethod := gin.New()
	wrongMethod.GET(canonicalPaymentWebhookPath, func(c *gin.Context) {})
	if callbackRouteMounted(wrongMethod) {
		t.Error("a GET-only route at the canonical path must not satisfy the POST callback contract")
	}

	wrongPath := gin.New()
	wrongPath.POST("/api/v1/payments/webhook", func(c *gin.Context) {})
	if callbackRouteMounted(wrongPath) {
		t.Error("a POST route at the legacy path must not satisfy the canonical callback contract")
	}
}
