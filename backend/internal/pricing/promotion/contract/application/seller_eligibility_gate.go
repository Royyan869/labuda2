package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/db"
)

// ============================================================================
// PRODUCTION SELLER ELIGIBILITY GATE (PHASE 4A COMPOSITION)
// ============================================================================
//
// SellerEligibilityGate is the governance authority PromotionContractService
// defers to at contract creation. This file provides the ONLY production
// implementation. It is a pure delegation adapter: it carries NO promotion
// business rules and duplicates NO subscription/seller logic.
//
// The delegated authority is auth.RoleChecker.HasActiveSellerCapability
// (RoleCheckerDB in production). That is the canonical market-facing seller
// authority consumed by:
//   - For Sale public creation (for_sale_service.go)
//   - Auction creation / schedule / cancel gates (auction_service.go)
//   - Order market-authority guard (order_creation_service.go)
//   - RequireSellerMiddleware (route-level gate used by seller mutation
//     endpoints, including the legacy promotion purchase/activation surface)
//
// HasActiveSellerCapability requires, against DB truth:
//   1. account_status active (not suspended/banned/removed),
//   2. seller profile exists,
//   3. an active seller subscription whose interval satisfies
//      started_at <= now < expires_at.
//
// The gate is invoked inside the contract Create transaction. The role
// checker reads through its own pool connection — the exact same pattern the
// canonical For Sale/Auction services use when they call
// HasActiveSellerCapability inside their creation transactions, so this is
// consistent with existing production authority usage.
// ============================================================================

// RoleCheckerSellerEligibilityGate implements SellerEligibilityGate by
// delegating to the canonical auth.RoleChecker seller-capability authority.
type RoleCheckerSellerEligibilityGate struct {
	roleChecker auth.RoleChecker
}

// NewRoleCheckerSellerEligibilityGate wires the delegating gate.
// roleChecker must be the production RoleCheckerDB instance.
func NewRoleCheckerSellerEligibilityGate(roleChecker auth.RoleChecker) *RoleCheckerSellerEligibilityGate {
	return &RoleCheckerSellerEligibilityGate{roleChecker: roleChecker}
}

var _ SellerEligibilityGate = (*RoleCheckerSellerEligibilityGate)(nil)

// EnsureCanPromote reports whether the seller may create a promotion contract.
// tx is accepted for interface compatibility; the canonical authority
// (RoleChecker) is pool-backed and is used identically by the For Sale and
// Auction creation transactions. No promotion-side eligibility rule exists here.
func (g *RoleCheckerSellerEligibilityGate) EnsureCanPromote(ctx context.Context, _ db.Tx, sellerID uuid.UUID) error {
	if g == nil || g.roleChecker == nil {
		return fmt.Errorf("promotion seller eligibility gate is not wired")
	}
	ok, err := g.roleChecker.HasActiveSellerCapability(ctx, sellerID)
	if err != nil {
		return fmt.Errorf("promotion seller eligibility check failed: %w", err)
	}
	if !ok {
		return auth.ErrMarketAuthorityRequired
	}
	return nil
}
