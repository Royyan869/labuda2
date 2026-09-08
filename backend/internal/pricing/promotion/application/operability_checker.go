package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
)

// OperabilityCheckerImpl implements OperabilityChecker with real domain checks.
type OperabilityCheckerImpl struct {
	db          *db.DB
	auctionRepo *auctionRepo.AuctionRepository
}

// NewOperabilityCheckerImpl creates a new real operability checker. promotionRepo param kept for compat but ignored (legacy purged).
func NewOperabilityCheckerImpl(
	dbConn *db.DB,
	_ interface{},
) *OperabilityCheckerImpl {
	return &OperabilityCheckerImpl{
		db:          dbConn,
		auctionRepo: auctionRepo.NewAuctionRepository(),
	}
}

// rowQuerier is satisfied by both *pgxpool.Pool (reads outside any
// transaction) and db.Tx (reads inside a caller-owned transaction). It lets
// the canonical target-eligibility checks run on whichever connection the
// caller owns, so a transaction-boundary revalidation (CheckOperabilityTx)
// needs NO second pool acquisition and therefore cannot deadlock a small
// connection pool (each transaction already holds its own connection).
type rowQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// CheckOperability checks if a target is still operable for promotion.
// Returns (isOperable, reason, error). Pool-based read (no transaction).
func (c *OperabilityCheckerImpl) CheckOperability(
	ctx context.Context,
	targetType entity.TargetType,
	targetID *uuid.UUID,
) (bool, string, error) {
	return c.checkOperability(ctx, c.db.Pool(), "", time.Now(), targetType, targetID)
}

// CheckOperabilityTx re-runs the canonical target eligibility checks inside a
// caller-owned transaction, using the SAME transaction connection (no second
// pool acquisition — no connection-pool deadlock). The target row is read
// FOR UPDATE so the eligibility judgment serializes against concurrent
// target-state mutations: a target that lost canonical purchase availability
// before this point is observed here and can never bill. now must be the DB
// clock (server time authority), never the application clock.
func (c *OperabilityCheckerImpl) CheckOperabilityTx(
	ctx context.Context,
	tx db.Tx,
	now time.Time,
	targetType entity.TargetType,
	targetID *uuid.UUID,
) (bool, string, error) {
	return c.checkOperability(ctx, tx, " FOR UPDATE", now, targetType, targetID)
}

func (c *OperabilityCheckerImpl) checkOperability(
	ctx context.Context,
	q rowQuerier,
	lockClause string,
	now time.Time,
	targetType entity.TargetType,
	targetID *uuid.UUID,
) (bool, string, error) {
	switch targetType {
	case entity.TargetTypeForSale:
		if targetID == nil {
			return false, "for_sale_not_found", nil
		}
		return c.checkForSaleOperability(ctx, q, lockClause, now, *targetID)
	case entity.TargetTypeAuction:
		if targetID == nil {
			return false, "auction_not_found", nil
		}
		return c.checkAuctionOperability(ctx, q, lockClause, now, *targetID)
	case entity.TargetTypeExternalProduct:
		if targetID == nil {
			return false, "external_product_not_found", nil
		}
		return c.checkExternalProductOperability(ctx, q, lockClause, now, *targetID)
	default:
		return false, fmt.Sprintf("unknown target type: %s", targetType), nil
	}
}

// ValidateOwnership checks if the user owns the target they want to promote.
func (c *OperabilityCheckerImpl) ValidateOwnership(
	ctx context.Context,
	userID uuid.UUID,
	targetType entity.TargetType,
	targetID *uuid.UUID,
) error {
	switch targetType {
	case entity.TargetTypeForSale:
		if targetID == nil {
			return fmt.Errorf("fixed-price sale not found")
		}
		return c.validateForSaleOwnership(ctx, userID, *targetID)
	case entity.TargetTypeAuction:
		if targetID == nil {
			return fmt.Errorf("auction not found")
		}
		return c.validateAuctionOwnership(ctx, userID, *targetID)
	case entity.TargetTypeExternalProduct:
		if targetID == nil {
			return fmt.Errorf("external product not found")
		}
		return c.validateExternalProductOwnership(ctx, userID, *targetID)
	default:
		return fmt.Errorf("unknown target type: %s", targetType)
	}
}

// ========================================================================
// FIXED-PRICE SALE OPERABILITY CHECKS
// ========================================================================

// checkForSaleOperability checks if a fixed-price sale is still operable for promotion.
// Fixed-price sale is operable if:
// - Status == "active"
// - Visibility == "public"
// - QuantityAvailable > 0
func (c *OperabilityCheckerImpl) checkForSaleOperability(
	ctx context.Context,
	q rowQuerier,
	lockClause string,
	now time.Time,
	forSaleID uuid.UUID,
) (bool, string, error) {
	// Query fixed-price sale directly with minimal fields for operability check.
	// seller_id is now also projected to enable the expired-seller exclusion
	// below — promotions of expired-seller fixed-price sales must not surface on
	// /promotions/discover per owner doctrine.
	//
	// Reads from for_sales (the canonical table) joined via
	// product_id is not needed here — status/quantity/seller_id all live on
	// for_sales directly. The legacy `listings` table is write-dead.
	// for_sales has no visibility column; visibility is derived the
	// same way ForSaleRepositoryImpl.derivedVisibility does: active
	// status with a non-nil published_at.
	//
	// lockClause is " FOR UPDATE" on the transaction-boundary revalidation
	// path so the eligibility judgment serializes against concurrent
	// target-state mutations (sale, withdrawal); it is empty on plain reads.
	var status forsaleEntity.ForSaleStatus
	var quantityAvailable int
	var sellerID uuid.UUID
	var publishedAt *time.Time

	query := `
		SELECT status, quantity_available, seller_id, published_at
		FROM for_sales
		WHERE id = $1
	` + lockClause

	err := q.QueryRow(ctx, query, forSaleID).Scan(&status, &quantityAvailable, &sellerID, &publishedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, "for_sale_not_found", nil
		}
		return false, "", fmt.Errorf("failed to check fixed-price sale operability: %w", err)
	}

	// Check if fixed-price sale is available (using the same logic as fixed-price sale availability rules)
	if status != forsaleEntity.ForSaleStatusActive {
		// Map status to canonical stop reason
		switch status {
		case forsaleEntity.ForSaleStatusSold:
			return false, "for_sale_sold", nil
		case forsaleEntity.ForSaleStatusWithdrawn:
			return false, "for_sale_hidden", nil
		default:
			return false, "for_sale_unavailable", nil
		}
	}

	// Check visibility (derived: active + published)
	if publishedAt == nil {
		return false, "for_sale_hidden", nil
	}

	// Check quantity
	if quantityAvailable <= 0 {
		return false, "for_sale_sold", nil
	}

	// Seller governance: exclude promotions whose seller is ineligible.
	// Covers: account suspended/banned/removed, subscription expired,
	// verification suspended/revoked. Mirrors HasActiveSellerCapability gates.
	if ok, reason, err := c.sellerIsDiscoveryEligible(ctx, q, now, sellerID); !ok || err != nil {
		if err != nil {
			return false, "", err
		}
		return false, reason, nil
	}

	return true, "", nil
}

// validateForSaleOwnership validates that the user owns the fixed-price sale.
func (c *OperabilityCheckerImpl) validateForSaleOwnership(
	ctx context.Context,
	userID uuid.UUID,
	forSaleID uuid.UUID,
) error {
	var sellerID uuid.UUID

	query := `SELECT seller_id FROM for_sales WHERE id = $1`
	err := c.db.Pool().QueryRow(ctx, query, forSaleID).Scan(&sellerID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("fixed-price sale not found")
		}
		return fmt.Errorf("failed to validate fixed-price sale ownership: %w", err)
	}

	if sellerID != userID {
		return fmt.Errorf("user does not own this fixed-price sale")
	}

	return nil
}

// ========================================================================
// AUCTION OPERABILITY CHECKS
// ========================================================================

// checkAuctionOperability checks if an auction is still promotable.
// Auction is promotable if:
// - Status is "scheduled" or "active" (publicly visible states)
// Non-promotable statuses: "draft", "ended", "cancelled"
//
// BUSINESS TRUTH: "draft" auctions are NOT promotable because:
// - They are not public visible
// - They are meant for editing, not exposure
// - Promoting a draft auction would be misleading to users
func (c *OperabilityCheckerImpl) checkAuctionOperability(
	ctx context.Context,
	q rowQuerier,
	lockClause string,
	now time.Time,
	auctionID uuid.UUID,
) (bool, string, error) {
	var status auctionEntity.Status
	var sellerID uuid.UUID

	// lockClause is " FOR UPDATE" on the transaction-boundary revalidation
	// path so the eligibility judgment serializes against concurrent auction
	// end/cancel mutations (auction end authority stays canonical: status is
	// the decision factor, end_at is the domain worker's trigger).
	query := `SELECT status, seller_id FROM auctions WHERE id = $1` + lockClause
	err := q.QueryRow(ctx, query, auctionID).Scan(&status, &sellerID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, "auction_not_found", nil
		}
		return false, "", fmt.Errorf("failed to check auction operability: %w", err)
	}

	// Check if auction is still promotable
	switch status {
	case auctionEntity.StatusDraft:
		return false, "auction_draft_not_promotable", nil
	case auctionEntity.StatusEnded:
		return false, "auction_ended", nil
	case auctionEntity.StatusCancelled:
		return false, "auction_cancelled", nil
	case auctionEntity.StatusScheduled, auctionEntity.StatusActive:
		// Seller governance: exclude auctions of ineligible sellers.
		if ok, reason, err := c.sellerIsDiscoveryEligible(ctx, q, now, sellerID); !ok || err != nil {
			if err != nil {
				return false, "", err
			}
			return false, reason, nil
		}
		return true, "", nil
	default:
		return false, "auction_unavailable", nil
	}
}

// sellerIsDiscoveryEligible checks full seller governance for promotion
// discovery visibility. This mirrors the market-facing time-bounded
// authority rule used by the activation handler, ensuring promotion
// discovery never exposes stronger visibility than what the write gate
// would allow.
//
// Gates checked:
//  1. Account: users.account_status = 'active' AND deleted_at IS NULL
//  2. Subscription interval: started_at <= now < expires_at
//     plus subscription status = 'active'
//
// Gate 2 from HasActiveSellerCapability (seller_profiles existence) is omitted
// because a fixed-price-sale/auction row implies the seller profile already exists.
func (c *OperabilityCheckerImpl) sellerIsDiscoveryEligible(
	ctx context.Context,
	q rowQuerier,
	now time.Time,
	sellerID uuid.UUID,
) (bool, string, error) {
	var accountStatus string
	var isDeleted bool
	var subscriptionStatus string
	var startedAt, expiresAt time.Time

	err := q.QueryRow(ctx, `
		SELECT
			COALESCE(u.account_status::text, ''),
			(u.deleted_at IS NOT NULL),
			COALESCE(ss.status::text, ''),
			ss.started_at,
			ss.expires_at
		FROM users u
		LEFT JOIN LATERAL (
			SELECT status, started_at, expires_at
			FROM seller_subscriptions
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT 1
		) ss ON true
		WHERE u.id = $1
	`, sellerID).Scan(&accountStatus, &isDeleted, &subscriptionStatus, &startedAt, &expiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, "seller_not_found", nil
		}
		return false, "", fmt.Errorf("failed to check seller eligibility: %w", err)
	}

	eligible, reason := sellerGovernanceEligible(accountStatus, isDeleted, subscriptionStatus, startedAt, expiresAt, now)
	if !eligible {
		return false, reason, nil
	}
	return true, "", nil
}

// checkExternalProductOperability checks whether an external product is eligible
// for public promotion.
func (c *OperabilityCheckerImpl) checkExternalProductOperability(
	ctx context.Context,
	q rowQuerier,
	lockClause string,
	now time.Time,
	externalProductID uuid.UUID,
) (bool, string, error) {
	type externalProductRow struct {
		ownerUserID           uuid.UUID
		reviewStatus          string
		normalizedExternalURL string
		deletedAtPresent      bool
	}

	var row externalProductRow
	query := `
		SELECT owner_user_id, review_status, normalized_external_url, (deleted_at IS NOT NULL)
		FROM external_products
		WHERE id = $1
	` + lockClause
	err := q.QueryRow(ctx, query, externalProductID).Scan(&row.ownerUserID, &row.reviewStatus, &row.normalizedExternalURL, &row.deletedAtPresent)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, "external_product_not_found", nil
		}
		return false, "", fmt.Errorf("failed to check external product operability: %w", err)
	}
	if row.deletedAtPresent {
		return false, "external_product_deleted", nil
	}
	if row.normalizedExternalURL == "" {
		return false, "external_product_invalid", nil
	}
	if entity.ExternalProductReviewStatus(row.reviewStatus) != entity.ExternalProductReviewStatusApproved {
		return false, "external_product_not_approved", nil
	}

	var mediaFound int
	err = q.QueryRow(ctx, `
		SELECT 1
		FROM external_product_media
		WHERE external_product_id = $1
		  AND deleted_at IS NULL
		  AND media_type IN ('image', 'video')
		ORDER BY sort_order ASC, created_at ASC
		LIMIT 1
	`, externalProductID).Scan(&mediaFound)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, "external_product_missing_media", nil
		}
		return false, "", fmt.Errorf("failed to check external product media: %w", err)
	}

	ok, reason, err := c.sellerIsDiscoveryEligible(ctx, q, now, row.ownerUserID)
	if err != nil {
		return false, "", err
	}
	if !ok {
		return false, reason, nil
	}

	return true, "", nil
}

// validateExternalProductOwnership validates that the user owns the external product.
func (c *OperabilityCheckerImpl) validateExternalProductOwnership(
	ctx context.Context,
	userID uuid.UUID,
	externalProductID uuid.UUID,
) error {
	var ownerID uuid.UUID
	err := c.db.Pool().QueryRow(ctx, `
		SELECT owner_user_id
		FROM external_products
		WHERE id = $1
		  AND deleted_at IS NULL
	`, externalProductID).Scan(&ownerID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("external product not found")
		}
		return fmt.Errorf("failed to validate external product ownership: %w", err)
	}
	if ownerID != userID {
		return fmt.Errorf("user does not own this external product")
	}
	return nil
}

// sellerGovernanceEligible is the pure decision function for the seller
// governance model. It mirrors the market-facing authority rule:
//
//	Gate 1: account_status = 'active' AND deleted_at IS NULL
//	Gate 2: subscription status = 'active'
//	Gate 3: started_at <= now < expires_at
//
// KYC / seller_verifications is intentionally not read here.
// This function is the SINGLE canonical site for the eligibility predicate.
// Both single-item (sellerIsDiscoveryEligible) and batch (GetOperableTargets*)
// call paths use it.
func sellerGovernanceEligible(
	accountStatus string,
	isDeleted bool,
	subscriptionStatus string,
	startedAt time.Time,
	expiresAt time.Time,
	now time.Time,
) (bool, string) {
	// Gate 1: Account must exist and not be soft-deleted or degraded
	if isDeleted {
		return false, "seller_removed"
	}
	if accountStatus != "active" {
		return false, "seller_account_inactive"
	}

	// Gate 2: Subscription must be active
	if subscriptionStatus != "active" {
		return false, "seller_inactive"
	}

	// Gate 3: Entitlement interval must be currently valid.
	if startedAt.IsZero() || expiresAt.IsZero() {
		return false, "seller_inactive"
	}
	if now.Before(startedAt) || !now.Before(expiresAt) {
		return false, "seller_inactive"
	}

	return true, ""
}

// CheckUserEligibility checks if a user is eligible for promotion discovery.
// This is the account-level gate for external product promotions where there
// is no target entity to derive a seller from.
func (c *OperabilityCheckerImpl) CheckUserEligibility(
	ctx context.Context,
	userID uuid.UUID,
) (bool, string, error) {
	var accountStatus string
	var isDeleted bool

	err := c.db.Pool().QueryRow(ctx, `
		SELECT COALESCE(account_status::text, ''), (deleted_at IS NOT NULL)
		FROM users WHERE id = $1
	`, userID).Scan(&accountStatus, &isDeleted)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, "user_not_found", nil
		}
		return false, "", fmt.Errorf("failed to check user eligibility: %w", err)
	}

	if isDeleted {
		return false, "user_removed", nil
	}
	if accountStatus != "active" {
		return false, "user_account_inactive", nil
	}

	return true, "", nil
}

// validateAuctionOwnership validates that the user owns the auction.
func (c *OperabilityCheckerImpl) validateAuctionOwnership(
	ctx context.Context,
	userID uuid.UUID,
	auctionID uuid.UUID,
) error {
	var sellerID uuid.UUID

	query := `SELECT seller_id FROM auctions WHERE id = $1`
	err := c.db.Pool().QueryRow(ctx, query, auctionID).Scan(&sellerID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("auction not found")
		}
		return fmt.Errorf("failed to validate auction ownership: %w", err)
	}

	if sellerID != userID {
		return fmt.Errorf("user does not own this auction")
	}

	return nil
}

// ========================================================================
// WORKER SAFETY NET
// ========================================================================

// IsReversibleReason returns true if the operability reason represents a
// reversible (temporary) condition that should PAUSE rather than STOP.
//
// Classification table:
//
//	REVERSIBLE (pause):
//	  seller_inactive           — subscription expired, can renew
//	  for_sale_hidden                    — withdrawn by seller, can be re-activated
//	  for_sale_moderated                 — moderation action, can be restored by appeal
//
//	PERMANENT (stop + finalize):
//	  seller_removed            — account deleted
//	  seller_account_inactive   — account suspended/banned
//	  seller_not_found          — account gone
//	  for_sale_sold              — fixed-price sale sold, stock gone
//	  for_sale_not_found         — fixed-price sale deleted
//	  for_sale_unavailable       — catch-all non-active status
//	  for_sale_expired                   — sale expired
//	  auction_ended             — auction ended
//	  auction_cancelled         — auction cancelled
//	  auction_deleted           — auction deleted
//	  auction_moderated         — auction moderated (auctions don't restore)
//	  auction_draft_not_promotable — auction in draft
//	  auction_unavailable       — catch-all
//	  user_removed              — user deleted
//	  user_account_inactive     — user suspended
//	  user_not_found            — user gone
func IsReversibleReason(reason string) bool {
	switch reason {
	case "seller_inactive",
		"for_sale_hidden",
		"for_sale_moderated":
		return true
	default:
		return false
	}
}

// Legacy sweep for promotion_instances purged — canonical is promotion_contracts queue.
// Stub to keep OperabilityRecommendationSource interface satisfied without legacy table access.
func (c *OperabilityCheckerImpl) SweepInactivePromotions(ctx context.Context, limit int) ([]OperabilityRecommendation, error) {
	return nil, nil
}
func (c *OperabilityCheckerImpl) SweepPausedPromotions(ctx context.Context, limit int) ([]OperabilityRecommendation, error) {
	return nil, nil
}

type operabilityEvaluation struct {
	operable   bool
	reversible bool
	reason     string
}
func (c *OperabilityCheckerImpl) evaluateCandidate(ctx context.Context, cand sweepCandidate) (operabilityEvaluation, error) {
	operable, reason, err := c.CheckOperability(ctx, cand.TargetType, cand.TargetID)
	if err != nil {
		return operabilityEvaluation{}, err
	}
	if operable {
		return operabilityEvaluation{operable: true, reason: reason}, nil
	}
	return operabilityEvaluation{operable: false, reversible: IsReversibleReason(reason), reason: reason}, nil
}
type sweepCandidate struct {
	ID          uuid.UUID
	OwnershipID uuid.UUID
	UserID      uuid.UUID
	TargetType  entity.TargetType
	TargetID    *uuid.UUID
}

// mapReasonToStopReason maps an operability check reason to a canonical StopReason.
func (c *OperabilityCheckerImpl) mapReasonToStopReason(
	targetType entity.TargetType,
	reason string,
) entity.StopReason {
	// Seller/user governance reasons apply to all target types.
	switch reason {
	case "seller_removed", "seller_account_inactive", "seller_not_found",
		"user_removed", "user_account_inactive", "user_not_found":
		return entity.StopReasonSellerGovernance
	case "seller_inactive":
		return entity.StopReasonSellerGovernance
	}

	switch targetType {
	case entity.TargetTypeForSale:
		switch reason {
		case "for_sale_sold":
			return entity.StopReasonForSaleSold
		case "for_sale_hidden":
			return entity.StopReasonForSaleHidden
		case "for_sale_deleted":
			return entity.StopReasonForSaleDeleted
		case "for_sale_moderated":
			return entity.StopReasonForSaleModerated
		case "for_sale_expired":
			return entity.StopReasonForSaleExpired
		default:
			return entity.StopReasonForSaleSold
		}
	case entity.TargetTypeAuction:
		switch reason {
		case "auction_ended":
			return entity.StopReasonAuctionEnded
		case "auction_cancelled":
			return entity.StopReasonAuctionCancelled
		case "auction_deleted":
			return entity.StopReasonAuctionDeleted
		case "auction_moderated":
			return entity.StopReasonAuctionModerated
		default:
			return entity.StopReasonAuctionEnded
		}
	default:
		return entity.StopReasonUserCancelled
	}
}

// ========================================================================
// READ-TIME FILTERING HELPERS
// ========================================================================

// GetOperableTargetsForForSales returns fixed-price sale IDs that are operable from a list.
// This is used by discovery layers to filter out dead targets.
// Includes full seller governance (account + time-bounded subscription interval).
func (c *OperabilityCheckerImpl) GetOperableTargetsForForSales(
	ctx context.Context,
	forSaleIDs []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	if len(forSaleIDs) == 0 {
		return map[uuid.UUID]bool{}, nil
	}

	result := make(map[uuid.UUID]bool)
	now := time.Now()

	// Query fixed-price sales with seller governance join.
	// Mirrors checkForSaleOperability + sellerIsDiscoveryEligible in one batch query.
	// for_sales has no visibility column; visibility is derived
	// (active + published_at not null) the same way the single-item check does.
	query := `
		SELECT
			fps.id, fps.status, fps.quantity_available, fps.published_at,
			COALESCE(u.account_status::text, ''),
			(u.deleted_at IS NOT NULL),
			COALESCE(ss.status::text, ''),
			ss.started_at,
			ss.expires_at
		FROM for_sales fps
		LEFT JOIN users u ON u.id = fps.seller_id
		LEFT JOIN LATERAL (
			SELECT status, started_at, expires_at
			FROM seller_subscriptions
			WHERE user_id = fps.seller_id
			ORDER BY created_at DESC
			LIMIT 1
		) ss ON true
		WHERE fps.id = ANY($1)
	`

	rows, err := c.db.Pool().Query(ctx, query, forSaleIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query fixed-price sale operability: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var status forsaleEntity.ForSaleStatus
		var quantityAvailable int
		var publishedAt *time.Time
		var accountStatus string
		var isDeleted bool
		var subscriptionStatus string
		var startedAt, expiresAt time.Time

		err := rows.Scan(&id, &status, &quantityAvailable, &publishedAt,
			&accountStatus, &isDeleted, &subscriptionStatus, &startedAt, &expiresAt)
		if err != nil {
			continue
		}

		// Target operability
		targetOk := status == forsaleEntity.ForSaleStatusActive &&
			publishedAt != nil &&
			quantityAvailable > 0

		// Seller governance (canonical pure function)
		sellerOk, _ := sellerGovernanceEligible(accountStatus, isDeleted, subscriptionStatus, startedAt, expiresAt, now)

		result[id] = targetOk && sellerOk
	}

	// Mark any IDs not found as non-operable
	for _, id := range forSaleIDs {
		if _, exists := result[id]; !exists {
			result[id] = false
		}
	}

	return result, nil
}

// GetOperableTargetsForAuction returns auction IDs that are operable from a list.
// Includes full seller governance (account + time-bounded subscription interval).
func (c *OperabilityCheckerImpl) GetOperableTargetsForAuction(
	ctx context.Context,
	auctionIDs []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	if len(auctionIDs) == 0 {
		return map[uuid.UUID]bool{}, nil
	}

	result := make(map[uuid.UUID]bool)

	query := `
		SELECT
			a.id, a.status, a.end_at,
			COALESCE(u.account_status::text, ''),
			(u.deleted_at IS NOT NULL),
			COALESCE(ss.status::text, ''),
			ss.started_at,
			ss.expires_at
		FROM auctions a
		LEFT JOIN users u ON u.id = a.seller_id
		LEFT JOIN LATERAL (
			SELECT status, started_at, expires_at
			FROM seller_subscriptions
			WHERE user_id = a.seller_id
			ORDER BY created_at DESC
			LIMIT 1
		) ss ON true
		WHERE a.id = ANY($1)
	`

	rows, err := c.db.Pool().Query(ctx, query, auctionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query auction operability: %w", err)
	}
	defer rows.Close()

	now := time.Now()

	for rows.Next() {
		var id uuid.UUID
		var status auctionEntity.Status
		var endAt time.Time
		var accountStatus string
		var isDeleted bool
		var subscriptionStatus string
		var startedAt, expiresAt time.Time

		err := rows.Scan(&id, &status, &endAt,
			&accountStatus, &isDeleted, &subscriptionStatus, &startedAt, &expiresAt)
		if err != nil {
			continue
		}

		// Target operability
		targetOk := (status == auctionEntity.StatusScheduled || status == auctionEntity.StatusActive) &&
			(status != auctionEntity.StatusActive || now.Before(endAt))

		// Seller governance (canonical pure function)
		sellerOk, _ := sellerGovernanceEligible(accountStatus, isDeleted, subscriptionStatus, startedAt, expiresAt, now)

		result[id] = targetOk && sellerOk
	}

	// Mark any IDs not found as non-operable
	for _, id := range auctionIDs {
		if _, exists := result[id]; !exists {
			result[id] = false
		}
	}

	return result, nil
}
