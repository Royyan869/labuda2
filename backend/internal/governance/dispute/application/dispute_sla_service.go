// Package application provides dispute SLA tracking and computation.
// This service computes SLA metrics from existing dispute data without requiring
// additional database storage or new domains.
package application

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/dispute/entity"
)

// SLA constants
const (
	// AdminResponseSLA is the maximum time for first admin response (2 hours)
	AdminResponseSLA = 2 * time.Hour
	// ResolutionSLA is the maximum time for dispute resolution (48 hours)
	ResolutionSLA = 48 * time.Hour
)

// DisputeSLAMetrics contains computed SLA metrics for a dispute.
type DisputeSLAMetrics struct {
	// Timings
	AdminResponseTime *time.Duration // Time from opened to first admin action
	ResolutionTime    *time.Duration // Time from opened to resolution
	WaitingBuyerTime  *time.Duration // Time spent waiting for buyer response
	WaitingSellerTime *time.Duration // Time spent waiting for seller response
	ActiveTime        *time.Duration // Total time admin spent actively working

	// SLA Status
	AdminResponseOverdue bool // Whether admin response SLA was breached
	ResolutionOverdue     bool // Whether resolution SLA was breached

	// Next Action
	NextAction string // "review", "wait_buyer", "wait_seller", "resolved", "auto_resolve"

	// Overdue Duration
	AdminResponseOverdueDuration  *time.Duration
	ResolutionOverdueDuration     *time.Duration
}

// DisputeSLAService computes SLA metrics for disputes.
type DisputeSLAService struct{}

// NewDisputeSLAService creates a new SLA service.
func NewDisputeSLAService() *DisputeSLAService {
	return &DisputeSLAService{}
}

// ComputeMetrics calculates SLA metrics for a dispute.
// This computation is done entirely from existing dispute data without additional storage.
func (s *DisputeSLAService) ComputeMetrics(dispute *entity.Dispute) *DisputeSLAMetrics {
	now := time.Now()
	metrics := &DisputeSLAMetrics{}

	// Calculate time since dispute was opened
	elapsed := now.Sub(dispute.OpenedAt)

	if dispute.IsResolved() {
		// RESOLVED DISPUTE - Compute final metrics
		s.computeResolvedMetrics(dispute, metrics)
	} else {
		// ACTIVE DISPUTE - Compute current state and next action
		s.computeActiveMetrics(dispute, elapsed, now, metrics)
	}

	return metrics
}

// computeResolvedMetrics computes metrics for resolved disputes.
func (s *DisputeSLAService) computeResolvedMetrics(
	dispute *entity.Dispute,
	metrics *DisputeSLAMetrics,
) {
	// Resolution time is the time from opened to resolved
	if dispute.ResolvedAt != nil {
		resolutionTime := dispute.ResolvedAt.Sub(dispute.OpenedAt)
		metrics.ResolutionTime = &resolutionTime

		// Check if resolution SLA was breached
		if resolutionTime > ResolutionSLA {
			metrics.ResolutionOverdue = true
			overdueDuration := resolutionTime - ResolutionSLA
			metrics.ResolutionOverdueDuration = &overdueDuration
		}

		// Admin response time is the time to resolution (we use resolution as proxy for admin action)
		// In a more sophisticated system, we'd track first admin message/action separately
		metrics.AdminResponseTime = &resolutionTime
		if resolutionTime > AdminResponseSLA {
			metrics.AdminResponseOverdue = true
			overdueDuration := resolutionTime - AdminResponseSLA
			metrics.AdminResponseOverdueDuration = &overdueDuration
		}
	}

	// For resolved disputes, next action is "resolved"
	metrics.NextAction = "resolved"

	// Active time is the same as resolution time (simplified)
	if metrics.ResolutionTime != nil {
		metrics.ActiveTime = metrics.ResolutionTime
	}
}

// computeActiveMetrics computes metrics for active (under_review) disputes.
func (s *DisputeSLAService) computeActiveMetrics(
	dispute *entity.Dispute,
	elapsed time.Duration,
	now time.Time,
	metrics *DisputeSLAMetrics,
) {
	// Check admin response SLA
	if elapsed > AdminResponseSLA {
		metrics.AdminResponseOverdue = true
		overdueDuration := elapsed - AdminResponseSLA
		metrics.AdminResponseOverdueDuration = &overdueDuration
		metrics.AdminResponseTime = &elapsed
	}

	// Check resolution SLA
	if elapsed > ResolutionSLA {
		metrics.ResolutionOverdue = true
		overdueDuration := elapsed - ResolutionSLA
		metrics.ResolutionOverdueDuration = &overdueDuration
	}

	// Determine next action based on dispute state
	metrics.NextAction = s.determineNextAction(dispute, elapsed)

	// Active time is the elapsed time so far
	metrics.ActiveTime = &elapsed
}

// determineNextAction computes the next action required for a dispute.
func (s *DisputeSLAService) determineNextAction(dispute *entity.Dispute, elapsed time.Duration) string {
	if !dispute.IsUnderReview() {
		return "resolved"
	}

	// Check if dispute is approaching or past auto-resolution timeout
	if dispute.ShouldBeOverdue() {
		return "review" // Urgent - needs immediate admin attention
	}

	if dispute.IsNearTimeout() {
		return "review" // Approaching timeout - needs admin attention
	}

	// Default next action is to review
	return "review"
}

// FormatDuration formats a duration in a human-readable way.
func FormatDuration(d *time.Duration) string {
	if d == nil {
		return "N/A"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 48 {
		days := hours / 24
	剩余hours := hours % 24
		return fmt.Sprintf("%dd %dh", days, 剩余hours)
	}

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}

	return fmt.Sprintf("%dm", minutes)
}

// GetSLASummary returns a human-readable SLA summary for display.
func (s *DisputeSLAService) GetSLASummary(metrics *DisputeSLAMetrics) string {
	if metrics.NextAction == "resolved" {
		return fmt.Sprintf("Resolved in %s", FormatDuration(metrics.ResolutionTime))
	}

	if metrics.ResolutionOverdue {
		return fmt.Sprintf("OVERDUE by %s", FormatDuration(metrics.ResolutionOverdueDuration))
	}

	if metrics.AdminResponseOverdue {
		return fmt.Sprintf("Admin response overdue by %s", FormatDuration(metrics.AdminResponseOverdueDuration))
	}

	return "Within SLA"
}

// ComputeSLAForDisputeList computes SLA metrics for a list of disputes.
// This is optimized for bulk operations.
func (s *DisputeSLAService) ComputeSLAForDisputeList(disputes []*entity.Dispute) map[uuid.UUID]*DisputeSLAMetrics {
	results := make(map[uuid.UUID]*DisputeSLAMetrics, len(disputes))

	for _, dispute := range disputes {
		results[dispute.ID] = s.ComputeMetrics(dispute)
	}

	return results
}


