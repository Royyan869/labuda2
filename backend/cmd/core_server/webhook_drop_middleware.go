package main

// Webhook drop middleware — dev-only, gateway-loss simulator.
//
// Phase 1B corpus generation needs to reproduce "webhook never reached us"
// for drift classes D1, D3, D4, D11. The cleanest simulation is to let
// Midtrans believe delivery succeeded (200 OK, no retries) while suppressing
// the canonical handler. This file implements that gate strictly behind two
// guards so production traffic cannot accidentally trigger it:
//
//   1. cfg.Server.Env must be "development".
//   2. WEBHOOK_DROP_ENABLED must be "true".
//
// Either guard missing → the filter no-ops (passes through). Both guards
// satisfied → the filter is ARMABLE: it can receive midtrans_order_ids to
// suppress, either pre-loaded from WEBHOOK_DROP_MIDTRANS_ORDER_IDS at boot
// OR hot-armed at runtime via the dev-only admin endpoint
// (POST /dev/webhook-drop/arm; see webhook_drop_admin.go).
//
// D11 specifically needs hot-arm: the settlement webhook for an order must
// pass through normally, and the LATER refund webhook for the same order_id
// must be dropped. Pre-loading the order_id at boot would also drop the
// settlement callback. Hot-arm between settlement and refund dispatch keeps
// the "drop AFTER gateway accept" invariant intact.

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/config"
	"go.uber.org/zap"
)

// errFilterNotArmable is returned by Arm when the filter is in its
// production no-op state (either env != development or WEBHOOK_DROP_ENABLED
// is not "true"). The admin endpoint translates this to 503.
var errFilterNotArmable = errors.New("webhook_drop_filter_not_armable")

type webhookDropFilter struct {
	// armable is the production-safety gate: env=development AND
	// WEBHOOK_DROP_ENABLED=true. When false, both the middleware and the
	// admin Arm() are inert.
	armable   bool
	targetIDs map[string]struct{}
	log       *zap.Logger
	mu        sync.RWMutex
}

// newWebhookDropFilter constructs the filter from env + cfg. Returns a
// no-op filter when either guard is absent so callers can always install
// the middleware unconditionally.
//
// If WEBHOOK_DROP_MIDTRANS_ORDER_IDS is set at boot, the listed order_ids
// are pre-armed. Additional ids can be added at runtime via Arm() while the
// filter remains armable.
func newWebhookDropFilter(cfg *config.Config, log *zap.Logger) *webhookDropFilter {
	f := &webhookDropFilter{log: log, targetIDs: map[string]struct{}{}}
	if cfg.Server.Env != "development" {
		return f
	}
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("WEBHOOK_DROP_ENABLED")), "true") {
		return f
	}
	f.armable = true

	raw := strings.TrimSpace(os.Getenv("WEBHOOK_DROP_MIDTRANS_ORDER_IDS"))
	if raw != "" {
		for _, id := range strings.Split(raw, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				f.targetIDs[id] = struct{}{}
			}
		}
	}

	preloaded := make([]string, 0, len(f.targetIDs))
	for id := range f.targetIDs {
		preloaded = append(preloaded, id)
	}
	log.Warn("webhook_drop_middleware_armable",
		zap.String("env", cfg.Server.Env),
		zap.Strings("preloaded_midtrans_order_ids", preloaded),
	)
	return f
}

// Armable reports whether the filter can accept Arm() calls. Used by route
// wiring to decide whether to mount the dev-only admin endpoint at all.
func (f *webhookDropFilter) Armable() bool {
	return f.armable
}

// Arm registers an additional midtrans_order_id for suppression. The call
// fails closed (errFilterNotArmable) when the filter is in its production
// no-op state, guaranteeing this method has no effect outside of an
// explicitly enabled dev environment.
//
// scenarioTag and reason are mandatory operator-supplied annotations and
// are emitted on the WEBHOOK_DROP_ARMED log line so future forensic replay
// can attribute each suppressed webhook to a specific scenario run.
func (f *webhookDropFilter) Arm(midtransOrderID, scenarioTag, reason string) error {
	if !f.armable {
		return errFilterNotArmable
	}
	midtransOrderID = strings.TrimSpace(midtransOrderID)
	scenarioTag = strings.TrimSpace(scenarioTag)
	reason = strings.TrimSpace(reason)
	if midtransOrderID == "" || scenarioTag == "" || reason == "" {
		return errors.New("midtrans_order_id, scenario_tag, reason are all required")
	}
	armedAt := time.Now().UTC()
	f.mu.Lock()
	f.targetIDs[midtransOrderID] = struct{}{}
	f.mu.Unlock()
	f.log.Warn("WEBHOOK_DROP_ARMED",
		zap.String("midtrans_order_id", midtransOrderID),
		zap.String("scenario_tag", scenarioTag),
		zap.Time("armed_at", armedAt),
		zap.String("reason", reason),
	)
	return nil
}

// Middleware returns a gin handler that 200-OKs an inbound Midtrans
// webhook whose payload `order_id` is in the configured drop list. The
// downstream handler is bypassed (c.Abort) so canonical webhook ingestion
// side-effects do not occur. Used to simulate webhook-in-transit loss for
// D1/D3/D4/D11 corpus generation.
func (f *webhookDropFilter) Middleware() gin.HandlerFunc {
	if !f.armable {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			f.log.Warn("webhook_drop_read_body_failed", zap.Error(err))
			c.Next()
			return
		}
		// Always restore the body for the downstream handler. If we don't
		// drop, the canonical webhook ingestion still needs to read it.
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		var probe struct {
			OrderID string `json:"order_id"`
		}
		if jsonErr := json.Unmarshal(body, &probe); jsonErr != nil {
			c.Next()
			return
		}
		if probe.OrderID == "" {
			c.Next()
			return
		}
		f.mu.RLock()
		_, drop := f.targetIDs[probe.OrderID]
		f.mu.RUnlock()
		if !drop {
			c.Next()
			return
		}
		f.log.Warn("webhook_drop_applied",
			zap.String("midtrans_order_id", probe.OrderID),
		)
		c.AbortWithStatusJSON(http.StatusOK, gin.H{
			"status_code":    "200",
			"status_message": "webhook_drop_middleware_acknowledged",
		})
	}
}
