package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
)

// TargetEligibility re-reads the CANONICAL commerce target state at
// qualification time. The Delivery Ticket snapshot is never trusted for
// billing: a target that lost canonical eligibility after issuance must not
// produce a charge.
//
// The concrete implementation is the existing canonical OperabilityChecker
// (internal/pricing/promotion/application.OperabilityCheckerImpl), which
// reads For Sale / Auction / External Product authority directly. This
// domain deliberately does NOT duplicate commerce lifecycle state.
type TargetEligibility interface {
	// CheckOperability reports whether the target is still operable for
	// promotion. reason is a machine-readable ineligibility cause (e.g.
	// for_sale_sold, auction_ended). Pool-based read used by the
	// qualification pre-flight fast-reject gate.
	CheckOperability(ctx context.Context, targetType promoentity.TargetType, targetID *uuid.UUID) (bool, string, error)

	// CheckOperabilityTx re-runs the CANONICAL target eligibility checks
	// INSIDE a caller-owned transaction, on the SAME transaction connection
	// (no second pool acquisition — no connection-pool deadlock), with the
	// target row locked FOR UPDATE so the eligibility judgment serializes
	// against concurrent target-state mutations. This is the billing
	// authority: a target that lost canonical eligibility before this point
	// is observed here and can never produce a charge.
	CheckOperabilityTx(ctx context.Context, tx db.Tx, now time.Time, targetType promoentity.TargetType, targetID *uuid.UUID) (bool, string, error)
}
