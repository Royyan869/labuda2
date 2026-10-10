package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	notificationentity "github.com/hishumi/backend/internal/interaction/notification/entity"
	"go.uber.org/zap"
)

func (h *NotificationEventHandler) handleWithdrawalRequested(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p WithdrawalPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	withdrawalID, err := uuid.Parse(p.WithdrawalID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid withdrawal_id: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"withdrawalId": p.WithdrawalID,
	}

	// Notify SELLER — they caused this request themselves; the actor is the
	// seller, never a sentinel.
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, notificationentity.UserActor(sellerID), "withdrawal.requested", withdrawalID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert seller notification failed: %w", err)
	}

	// Admin fanout: notify all admins holding finance.withdraw.review.
	if h.capabilityLister == nil {
		return info, nil
	}

	adminIDs, lErr := h.capabilityLister.ListUsersByCapability(ctx, "finance.withdraw.review")
	if lErr != nil {
		h.log.Error("withdrawal.requested: failed to list admins by capability",
			zap.String("withdrawal_id", p.WithdrawalID),
			zap.String("seller_id", p.SellerID),
			zap.String("capability", "finance.withdraw.review"),
			zap.Error(lErr),
		)
		// Return error so outbox retries. Seller notification is dedup-safe
		// (ON CONFLICT DO NOTHING) so replay won't create a duplicate.
		return notificationInfo{}, fmt.Errorf("withdrawal.requested: list admins by capability: %w", lErr)
	}

	if len(adminIDs) == 0 {
		h.log.Warn("withdrawal.requested: no admins hold finance.withdraw.review",
			zap.String("withdrawal_id", p.WithdrawalID),
		)
		return info, nil
	}

	// Admin notification data includes seller_id and amount for review context.
	adminData := map[string]interface{}{
		"withdrawalId": p.WithdrawalID,
		"sellerId":     p.SellerID,
		"amount":       p.Amount,
	}

	var delivered, failed int
	for _, adminID := range adminIDs {
		_, insertErr := h.insertNotificationWithPolicy(
			ctx,
			adminID, notificationentity.UserActor(sellerID),
			"withdrawal.requested",
			withdrawalID,
			adminData,
		)
		if insertErr != nil {
			failed++
			h.log.Error("withdrawal.requested: admin fanout insert failed",
				zap.String("withdrawal_id", p.WithdrawalID),
				zap.String("seller_id", p.SellerID),
				zap.String("admin_id", adminID.String()),
				zap.String("capability", "finance.withdraw.review"),
				zap.Error(insertErr),
			)
			continue
		}
		delivered++
	}

	// Total admin fanout failure → return error so outbox retries.
	// Seller notification already committed; retry will re-insert seller
	// (DB-deduplicated) and retry admin fanout.
	if delivered == 0 && failed > 0 {
		return notificationInfo{}, fmt.Errorf("withdrawal.requested: all %d admin fanout inserts failed for withdrawal %s", failed, p.WithdrawalID)
	}

	return info, nil
}

// handleWithdrawalApproved processes withdrawal.approved events.
// Notifies the seller that their withdrawal has been approved.
func (h *NotificationEventHandler) handleWithdrawalApproved(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p WithdrawalPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	withdrawalID, err := uuid.Parse(p.WithdrawalID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid withdrawal_id: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"withdrawalId": p.WithdrawalID,
	}

	// Notify SELLER — the payload carries the approving admin; legacy events
	// without one degrade to the system actor (never dropped, never nil).
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, notificationentity.ParseUserActor(p.ApprovedBy), "withdrawal.approved", withdrawalID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleWithdrawalRejected processes withdrawal.rejected events.
// Notifies the seller that their withdrawal has been rejected.
func (h *NotificationEventHandler) handleWithdrawalRejected(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p WithdrawalPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	withdrawalID, err := uuid.Parse(p.WithdrawalID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid withdrawal_id: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"withdrawalId": p.WithdrawalID,
	}

	// Notify SELLER — the payload carries the rejecting admin.
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, notificationentity.ParseUserActor(p.RejectedBy), "withdrawal.rejected", withdrawalID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleWithdrawalCompleted processes withdrawal.completed events.
// Notifies the seller that their withdrawal has been completed.
func (h *NotificationEventHandler) handleWithdrawalCompleted(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p WithdrawalPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	withdrawalID, err := uuid.Parse(p.WithdrawalID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid withdrawal_id: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"withdrawalId": p.WithdrawalID,
	}

	// Notify SELLER — payout completion is a system (gateway/worker) fact.
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, notificationentity.SystemActor(), "withdrawal.completed", withdrawalID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleWithdrawalFailed processes withdrawal.failed events.
// Notifies the seller that their payout failed and funds were returned.
func (h *NotificationEventHandler) handleWithdrawalFailed(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p WithdrawalPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	withdrawalID, err := uuid.Parse(p.WithdrawalID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid withdrawal_id: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"withdrawalId": p.WithdrawalID,
	}

	// Notify SELLER — payout failure is a system (gateway/worker) fact.
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, notificationentity.SystemActor(), "withdrawal.failed", withdrawalID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// =============================================================================
// VERIFICATION NOTIFICATION HANDLERS
// =============================================================================

// handleVerificationDocumentApproved processes verification.document.approved events.
// Notifies the user that their verification document has been approved.
func (h *NotificationEventHandler) handleVerificationDocumentApproved(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p VerificationDocumentPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	documentID, err := uuid.Parse(p.DocumentID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid document_id: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid user_id: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"documentId":   p.DocumentID,
		"documentType": p.DocumentType,
	}

	// Notify USER — the payload carries the approving admin.
	info, err := h.insertNotificationWithPolicy(ctx, userID, notificationentity.ParseUserActor(p.ApprovedBy), "verification.document.approved", documentID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleVerificationDocumentRejected processes verification.document.rejected events.
// Notifies the user that their verification document has been rejected.
func (h *NotificationEventHandler) handleVerificationDocumentRejected(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p VerificationDocumentPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	documentID, err := uuid.Parse(p.DocumentID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid document_id: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid user_id: %w", err)
	}

	// Navigation data for mobile
	data := map[string]interface{}{
		"documentId":   p.DocumentID,
		"documentType": p.DocumentType,
		"reason":       p.Reason,
	}

	// Notify USER — the payload carries the rejecting admin.
	info, err := h.insertNotificationWithPolicy(ctx, userID, notificationentity.ParseUserActor(p.RejectedBy), "verification.document.rejected", documentID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	return info, nil
}

// handleSellerVerificationLifecycle processes seller.verification.* events
// (submitted / approved / rejected / needs_resubmission). Each event is
// delivered as a notification to the seller; the navigation data carries
// the canonical status and reason so the mobile UI can render the banner
// (and recourse path on negative outcomes) without a follow-up call.
func (h *NotificationEventHandler) handleSellerVerificationLifecycle(
	ctx context.Context,
	payload []byte,
	notifyType, title, body string,
) (notificationInfo, error) {
	var p SellerVerificationPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"sellerId": p.SellerID,
		"status":   p.Status,
	}
	if p.Reason != "" {
		data["reason"] = p.Reason
	}

	// Notify SELLER (seller lifecycle event). Every lifecycle transition is an
	// admin decision, so the actor is the reviewing admin that the payload
	// names; an event naming none has no known human cause and degrades to the
	// system actor. The seller is the SUBJECT of the decision, never its cause.
	actor := notificationentity.ParseUserActor(p.reviewerID())

	// insertNotificationWithPolicy populates title/body from getTitleAndBody; we override
	// below with the caller-supplied copy which carries context-specific text from the
	// Handle() switch (approved / rejected / needs_resubmission / submitted).
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, actor, notifyType, sellerID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert notification failed: %w", err)
	}
	info.title = title
	info.body = body
	return info, nil
}

// handleSellerVerificationSubmitted processes seller.verification.submitted.
// It preserves the existing seller notification and adds admin fanout to all
// users holding seller.verification.review so the verification queue is not
// polling-only.
func (h *NotificationEventHandler) handleSellerVerificationSubmitted(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p SellerVerificationPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	sellerID, err := uuid.Parse(p.SellerID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid seller_id: %w", err)
	}

	data := map[string]interface{}{
		"sellerId": p.SellerID,
		"status":   p.Status,
	}
	if p.Reason != "" {
		data["reason"] = p.Reason
	}

	// 1. Notify SELLER — the seller caused their own submission.
	info, err := h.insertNotificationWithPolicy(ctx, sellerID, notificationentity.UserActor(sellerID), "seller.verification.submitted", sellerID, data)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("insert seller notification failed: %w", err)
	}
	info.title = "Verifikasi Diterima"
	info.body = "Pengajuan verifikasi Anda telah diterima dan akan ditinjau admin."

	// 2. Admin fanout: notify all admins holding seller.verification.review.
	if h.capabilityLister == nil {
		return info, nil
	}

	adminIDs, lErr := h.capabilityLister.ListUsersByCapability(ctx, "seller.verification.review")
	if lErr != nil {
		h.log.Error("seller.verification.submitted: failed to list admins by capability",
			zap.String("seller_id", p.SellerID),
			zap.String("capability", "seller.verification.review"),
			zap.Error(lErr),
		)
		// Return error so outbox retries. Seller notification is dedup-safe
		// (ON CONFLICT DO NOTHING) so replay won't create a duplicate.
		return notificationInfo{}, fmt.Errorf("seller.verification.submitted: list admins by capability: %w", lErr)
	}

	if len(adminIDs) == 0 {
		h.log.Warn("seller.verification.submitted: no admins hold seller.verification.review",
			zap.String("seller_id", p.SellerID),
		)
		return info, nil
	}

	// Admin notification data includes seller_id and status for review context.
	adminData := map[string]interface{}{
		"sellerId": p.SellerID,
		"status":   p.Status,
	}

	var delivered, failed int
	for _, adminID := range adminIDs {
		_, insertErr := h.insertNotificationWithPolicy(
			ctx,
			adminID, notificationentity.UserActor(sellerID),
			"seller.verification.submitted",
			sellerID,
			adminData,
		)
		if insertErr != nil {
			failed++
			h.log.Error("seller.verification.submitted: admin fanout insert failed",
				zap.String("seller_id", p.SellerID),
				zap.String("admin_id", adminID.String()),
				zap.String("capability", "seller.verification.review"),
				zap.Error(insertErr),
			)
			continue
		}
		delivered++
	}

	// Total admin fanout failure → return error so outbox retries.
	// Seller notification already committed; retry will re-insert seller
	// (DB-deduplicated) and retry admin fanout.
	if delivered == 0 && failed > 0 {
		return notificationInfo{}, fmt.Errorf("seller.verification.submitted: all %d admin fanout inserts failed for seller %s", failed, p.SellerID)
	}

	return info, nil
}

// =============================================================================
// AUCTION NOTIFICATION HANDLERS
// =============================================================================

// handleAuctionBidPlaced processes auction.bid.placed events.
// Notifies the seller that a new bid has been placed on their auction.
// Seller ID is resolved from DB (not present in the bid payload).
func (h *NotificationEventHandler) handleSellerSubscriptionExpiringLegacy(ctx context.Context, payload []byte) (notificationInfo, error) {
	// Payload uses map[string]any with uuid.UUID values serialized as strings.
	var p SellerSubscriptionExpiringPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid user_id: %w", err)
	}

	subscriptionID, err := uuid.Parse(p.SubscriptionID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid subscription_id: %w", err)
	}

	data := map[string]interface{}{
		"subscriptionId":  subscriptionID.String(),
		"daysUntilExpiry": p.DaysUntilExpiry,
	}

	return h.insertNotificationWithPolicy(
		ctx,
		userID, notificationentity.SystemActor(), // subscription reminder is platform-caused
		"seller.subscription.expiring",
		subscriptionID,
		data,
	)
}

// handleSellerSubscriptionExpiring processes seller.subscription.expiring events.
// Notifies the seller that their subscription will expire soon.
func (h *NotificationEventHandler) handleSellerSubscriptionExpiring(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p SellerSubscriptionExpiringPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid user_id: %w", err)
	}

	subscriptionID, err := uuid.Parse(p.SubscriptionID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid subscription_id: %w", err)
	}

	data := map[string]interface{}{
		"subscriptionId":  subscriptionID.String(),
		"expiresAt":       p.ExpiresAt,
		"daysUntilExpiry": p.DaysUntilExpiry,
	}

	return h.insertNotificationWithPolicy(
		ctx,
		userID, notificationentity.SystemActor(), // subscription reminder is platform-caused
		"seller.subscription.expiring",
		subscriptionID,
		data,
	)
}

// handleSellerSubscriptionExpired processes seller.subscription.expired events.
// Notifies the seller that their subscription has expired and market authority is lost.
func (h *NotificationEventHandler) handleSellerSubscriptionExpired(ctx context.Context, payload []byte) (notificationInfo, error) {
	var p SellerSubscriptionExpiredPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return notificationInfo{}, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid user_id: %w", err)
	}

	subscriptionID, err := uuid.Parse(p.SubscriptionID)
	if err != nil {
		return notificationInfo{}, fmt.Errorf("invalid subscription_id: %w", err)
	}

	data := map[string]interface{}{
		"subscriptionId": subscriptionID.String(),
	}

	return h.insertNotificationWithPolicy(
		ctx,
		userID, notificationentity.SystemActor(), // subscription expiry is platform-caused
		"seller.subscription.expired",
		subscriptionID,
		data,
	)
}

// =============================================================================
// NOTIFICATION INSERTER IMPLEMENTATION
// =============================================================================

// NotificationServiceInserter implements NotificationInserter using the notification repository.
