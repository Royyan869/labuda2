package entity

import (
	"time"
)

// =============================================================================
// OVERDUE DISPLAY LAYER - GRADED ENFORCEMENT MODEL
// =============================================================================
//
// BUSINESS TRUTH:
// - Orders that are paid, not shipped, and past ready_to_ship_by are overdue
// - This is a DISPLAY/AWARENESS layer, NOT a status change
// - Order status remains "paid" - we add overdue metadata only
//
// POLICY:
// - Tier 1 (Day 0 overdue): Mild warning, "Ingatkan Penjual" CTA
// - Tier 2 (Day 3 overdue): Stronger warning, Chat/Support CTA, Buyer can cancel
// - Tier 3 (Day 7 overdue): Very late, Support CTA, Buyer can cancel
//
// PROHIBITED:
// - Do NOT change order status to "overdue"
// - Do NOT auto-cancel / auto-refund
// - DO allow buyer self-cancel for overdue orders (Tier 2+)
// =============================================================================

// OverdueTier represents the severity level of an overdue order
type OverdueTier string

const (
	// OverdueNone - Order is not overdue
	OverdueNone OverdueTier = "none"
	// OverdueTier1 - Day 0: Just passed ready_to_ship_by
	OverdueTier1 OverdueTier = "overdue"
	// OverdueTier2 - Day 3+: Significantly overdue
	OverdueTier2 OverdueTier = "severely_overdue"
	// OverdueTier3 - Day 7+: Critically overdue
	OverdueTier3 OverdueTier = "critical_overdue"
)

// OverdueInfo contains calculated overdue display information for an order
// This is computed on-demand and NOT persisted to the database
type OverdueInfo struct {
	Tier       OverdueTier `json:"tier"`                  // none, overdue, severely_overdue, critical_overdue
	DaysOverdue int         `json:"days_overdue"`          // Number of days past ready_to_ship_by (0 if not overdue)
	IsOverdue   bool        `json:"is_overdue"`            // Convenience boolean
}

// CalculateOverdueInfo computes the overdue display information for an order
//
// BUSINESS RULES:
// - Only applies to orders with status = "paid"
// - Requires ready_to_ship_by to be set
// - Calculates days past the deadline
// - Determines tier based on days overdue
//
// TIER CALCULATION:
// - 0 days overdue = Tier 1 (just became overdue)
// - 3-6 days overdue = Tier 2 (severely_overdue)
// - 7+ days overdue = Tier 3 (critical_overdue)
func (o *Order) CalculateOverdueInfo() OverdueInfo {
	// Default: not overdue
	info := OverdueInfo{
		Tier:       OverdueNone,
		DaysOverdue: 0,
		IsOverdue:   false,
	}

	// Only paid orders can be overdue
	if o.Status != StatusPaid {
		return info
	}

	// Must have a ready_to_ship_by deadline
	if o.ReadyToShipBy == nil {
		return info
	}

	now := time.Now()

	// Not overdue yet
	if now.Before(*o.ReadyToShipBy) || now.Equal(*o.ReadyToShipBy) {
		return info
	}

	// Calculate days overdue
	duration := now.Sub(*o.ReadyToShipBy)
	daysOverdue := int(duration.Hours() / 24)

	// Determine tier
	var tier OverdueTier
	switch {
	case daysOverdue < 3:
		tier = OverdueTier1 // 0-2 days overdue
	case daysOverdue < 7:
		tier = OverdueTier2 // 3-6 days overdue
	default:
		tier = OverdueTier3 // 7+ days overdue
	}

	return OverdueInfo{
		Tier:       tier,
		DaysOverdue: daysOverdue,
		IsOverdue:   true,
	}
}

// GetOverdueBadgeLabel returns the display label for the overdue badge
// Returns empty string if not overdue
func (o *Order) GetOverdueBadgeLabel() string {
	info := o.CalculateOverdueInfo()
	if !info.IsOverdue {
		return ""
	}

	switch info.Tier {
	case OverdueTier1:
		return "Melewati Estimasi"
	case OverdueTier2:
		return "Terlambat"
	case OverdueTier3:
		return "Sangat Terlambat"
	default:
		return ""
	}
}

// GetOverdueBadgeVariant returns the visual variant for the overdue badge
// Returns empty string if not overdue
func (o *Order) GetOverdueBadgeVariant() string {
	info := o.CalculateOverdueInfo()
	if !info.IsOverdue {
		return ""
	}

	switch info.Tier {
	case OverdueTier1:
		return "warning" // Orange/yellow
	case OverdueTier2:
		return "error"   // Red
	case OverdueTier3:
		return "error"   // Red (more intense)
	default:
		return ""
	}
}

// GetOverdueWarningMessage returns the warning message for overdue orders
// Returns empty string if not overdue
func (o *Order) GetOverdueWarningMessage() string {
	info := o.CalculateOverdueInfo()
	if !info.IsOverdue {
		return ""
	}

	switch info.Tier {
	case OverdueTier1:
		return "Pesanan ini sudah melewati estimasi siap kirim."
	case OverdueTier2:
		return "Pesanan ini terlambat dari estimasi siap kirim."
	case OverdueTier3:
		return "Pesanan ini sangat terlambat dari estimasi siap kirim."
	default:
		return ""
	}
}

// IsEligibleForReminder checks if an order is eligible for an overdue reminder
//
// BUSINESS RULES:
// - Order must be paid
// - Must be overdue
// - Must NOT have been shipped yet
func (o *Order) IsEligibleForReminder(lastReminderAt *time.Time) bool {
	info := o.CalculateOverdueInfo()
	return info.IsOverdue && o.Status == StatusPaid
}

// IsEligibleForBuyerCancelDueToOverdue checks if buyer can cancel the order due to overdue shipping
//
// BUSINESS RULES:
// - Order must be paid
// - Must be overdue by at least 3 days (Tier 2 or higher)
// - Must NOT have been shipped yet
// - This is a SLA enforcement - buyers can cancel if seller fails to ship on time
func (o *Order) IsEligibleForBuyerCancelDueToOverdue() bool {
	info := o.CalculateOverdueInfo()
	// Only Tier 2+ (3+ days overdue) allows buyer cancellation
	return info.IsOverdue && (info.Tier == OverdueTier2 || info.Tier == OverdueTier3) && o.Status == StatusPaid
}


