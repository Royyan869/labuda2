package main

// Dev-only admin endpoint for hot-arming the webhook drop middleware.
//
// MOUNTED ONLY when the underlying filter reports Armable() == true, which
// itself requires cfg.Server.Env="development" AND WEBHOOK_DROP_ENABLED=true.
// In any other environment the route does not exist on the gin router —
// there is no production code path that can reach this handler.
//
// REQUEST CONTRACT (POST /dev/webhook-drop/arm):
//
//	{
//	  "midtrans_order_id": "<gateway order_id (string)>",
//	  "scenario_tag":      "<short tag, e.g. SCENARIO_D11_2026_05_12>",
//	  "reason":            "<free-text justification for forensic replay>"
//	}
//
// All three fields are required. Reason and scenario_tag are recorded on
// the WEBHOOK_DROP_ARMED structured log emitted by webhookDropFilter.Arm.
//
// CONSTITUTIONAL NOTES:
//   - No auth middleware: the dev+env guards inside the filter are the
//     only acceptable security envelope here. The route is only mounted
//     when those guards already passed.
//   - No probabilistic / random drop: explicit allowlist only.
//   - The endpoint NEVER drops settlement payloads — it merely arms an
//     order_id. The middleware is what observes payloads and decides; the
//     scenario operator is responsible for arming AFTER the settlement
//     callback has been processed.

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// webhookDropArmRequest is the JSON contract for POST /dev/webhook-drop/arm.
type webhookDropArmRequest struct {
	MidtransOrderID string `json:"midtrans_order_id"`
	ScenarioTag     string `json:"scenario_tag"`
	Reason          string `json:"reason"`
}

// webhookDropArmHandler builds the gin handler for the arm endpoint. The
// caller is expected to invoke this only when filter.Armable() is true.
func webhookDropArmHandler(filter *webhookDropFilter) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req webhookDropArmRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_json_body",
				"details": err.Error(),
			})
			return
		}
		req.MidtransOrderID = strings.TrimSpace(req.MidtransOrderID)
		req.ScenarioTag = strings.TrimSpace(req.ScenarioTag)
		req.Reason = strings.TrimSpace(req.Reason)
		if req.MidtransOrderID == "" || req.ScenarioTag == "" || req.Reason == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "midtrans_order_id, scenario_tag, and reason are all required",
			})
			return
		}
		if err := filter.Arm(req.MidtransOrderID, req.ScenarioTag, req.Reason); err != nil {
			if errors.Is(err, errFilterNotArmable) {
				// Defense-in-depth: filter refused even though route was
				// mounted. Should be unreachable in practice.
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"error": "webhook_drop_filter_not_armable",
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "arm_failed",
				"details": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":            "armed",
			"midtrans_order_id": req.MidtransOrderID,
			"scenario_tag":      req.ScenarioTag,
		})
	}
}
