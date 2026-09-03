package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	forSaleApp "github.com/labuda/backend/internal/commerce/forsale/application"
	forSaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	userRepositoryPkg "github.com/labuda/backend/internal/identity/user/repository"
	moderationRepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	platformevent "github.com/labuda/backend/internal/platform/event"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// auctionCanceller is the moderation-authority interface for auction enforcement.
// Implemented by *auctionApp.AuctionService; extracted here to allow test mocking.
type auctionCanceller interface {
	CancelForModeration(ctx context.Context, tx db.Tx, auctionID uuid.UUID) error
}

// ChatMessageModerationService is the chat boundary for moderation-driven
// hide/restore mutations plus room-list projection emission.
type ChatMessageModerationService interface {
	SoftHideForModeration(ctx context.Context, tx db.Tx, messageID uuid.UUID, deletedBy uuid.UUID, reason, moderationKey string) error
	RestoreFromModeration(ctx context.Context, tx db.Tx, messageID uuid.UUID, moderationKey string) error
}

// ModerationEventHandler handles moderation removal and restoration events.
//
// STRICT BOUNDARY RULES:
// - Only soft-deletes/restores resources via their respective services
// - NO financial mutations
// - NO order/negotiation modifications
// - NO verification/rating changes
//
// Event types handled:
// - moderation.content.removed       -> soft delete content via ContentService
// - moderation.comment.removed       -> soft delete comment via CommentService
// - moderation.for_sale.removed -> mark fixed-price sale withdrawn via ForSaleService
// - moderation.auction.removed       -> cancel auction via AuctionService.CancelForModeration (governance bypass)
// - moderation.user.suspended        -> suspend user account via UserRepository
// - moderation.chat_message.hidden   -> soft-hide chat message via chat boundary
// - moderation.{type}.restored       -> restore resource via respective service (appeal approved)
// - moderation.chat_message.restored -> restore soft-hidden chat message via chat boundary
type ModerationEventHandler struct {
	db               Transactor // *db.DB satisfies this; interface enables test injection
	contentService   *contentApp.ContentService
	commentService   *contentApp.CommentService
	forSaleService *forSaleApp.ForSaleService
	auctionService   auctionCanceller // interface: *auctionApp.AuctionService satisfies this
	userRepo         userRepositoryPkg.UserRepository
	chatMessageStore ChatMessageModerationService
	enfRepo          moderationRepo.EnforcementRepository
	log              *zap.Logger
}

// NewModerationEventHandler creates a new moderation event handler.
// The contentService parameter should be *contentApp.ContentService.
// The commentService parameter should be *contentApp.CommentService.
// The forSaleService parameter should be *forSaleApp.ForSaleService.
// The auctionService parameter should be *auctionApp.AuctionService.
// The userRepo parameter should be userRepositoryPkg.UserRepository.
// It accepts interface{} to avoid circular imports in worker setup.
func NewModerationEventHandler(
	db *db.DB,
	contentService interface{},
	commentService interface{},
	forSaleService interface{},
	auctionService interface{},
	userRepo interface{},
	chatMessageStore ChatMessageModerationService,
	enfRepo moderationRepo.EnforcementRepository,
	log *zap.Logger,
) *ModerationEventHandler {
	if log == nil {
		log = zap.NewNop()
	}
	// Type assertion to ensure we have the correct type for contentService
	cs, ok := contentService.(*contentApp.ContentService)
	if !ok && contentService != nil {
		log.Warn("contentService is not *contentApp.ContentService, content moderation events will not be processed")
	}
	// Type assertion to ensure we have the correct type for commentService
	cms, ok := commentService.(*contentApp.CommentService)
	if !ok && commentService != nil {
		log.Warn("commentService is not *contentApp.CommentService, comment moderation events will not be processed")
	}
	// Type assertion to ensure we have the correct type for forSaleService
	fps, ok := forSaleService.(*forSaleApp.ForSaleService)
	if !ok && forSaleService != nil {
		log.Warn("forSaleService is not *forSaleApp.ForSaleService, fixed-price sale moderation events will not be processed")
	}
	// Type assertion to auctionCanceller interface (satisfied by *auctionApp.AuctionService)
	var as auctionCanceller
	if auctionService != nil {
		if v, ok := auctionService.(auctionCanceller); ok {
			as = v
		} else {
			log.Warn("auctionService does not implement auctionCanceller, auction moderation events will not be processed")
		}
	}
	// Type assertion to ensure we have the correct type for userRepo
	ur, ok := userRepo.(userRepositoryPkg.UserRepository)
	if !ok && userRepo != nil {
		log.Warn("userRepo is not userRepositoryPkg.UserRepository, user moderation events will not be processed")
	}
	if chatMessageStore == nil {
		panic("NewModerationEventHandler: chatMessageStore must not be nil")
	}
	return &ModerationEventHandler{
		db:               db,
		contentService:   cs,
		commentService:   cms,
		forSaleService: fps,
		auctionService:   as,
		userRepo:         ur,
		chatMessageStore: chatMessageStore,
		enfRepo:          enfRepo,
		log:              log,
	}
}

// moderationRemovedPayload represents the payload of a moderation.removed event.
type moderationRemovedPayload struct {
	DecisionID    string  `json:"decision_id,omitempty"`
	EnforcementID string  `json:"enforcement_id,omitempty"`
	CaseID        string  `json:"case_id"`
	ResourceType  string  `json:"resource_type"`
	ResourceID    string  `json:"resource_id"`
	DecisionNote  *string `json:"decision_note,omitempty"`
}

// moderationRestoredPayload represents the payload of a moderation.restored event.
type moderationRestoredPayload struct {
	CaseID       string `json:"case_id"`
	AppealID     string `json:"appeal_id"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
}

// Handle processes a moderation removal or restoration event.
//
// Flow:
// 1. Determine event type (removed/restored)
// 2. Parse payload based on event type
// 3. Route to appropriate handler based on resource_type
// 4. Soft-delete or restore the resource via domain service
//
// Returns error only on critical failures (triggers retry).
// Idempotent: safe to retry on same resource.
func (h *ModerationEventHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	h.log.Info("Handling moderation event",
		zap.String("event_type", event.EventType),
		zap.String("event_id", event.ID.String()),
	)

	// Determine if this is a removal or restoration event
	isRestoration := false
	if len(event.EventType) > 0 {
		// Event type format: moderation.{resource_type}.{action}
		// Extract the action part (last segment)
		parts := splitEventSuffix(event.EventType)
		if len(parts) > 0 && parts[len(parts)-1] == "restored" {
			isRestoration = true
		}
	}

	if isRestoration {
		return h.handleRestoration(ctx, event)
	}

	// Handle removal events
	var payload moderationRemovedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.log.Error("Failed to parse moderation payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		// Don't retry on parse error - payload is malformed
		return nil
	}

	resourceID, err := uuid.Parse(payload.ResourceID)
	if err != nil {
		h.log.Error("Invalid resource ID in payload",
			zap.String("event_id", event.ID.String()),
			zap.String("resource_id", payload.ResourceID),
			zap.Error(err),
		)
		// Don't retry on invalid ID
		return nil
	}

	// Route based on resource type
	switch payload.ResourceType {
	case "content":
		return h.handleContentRemoved(ctx, resourceID, payload)

	case "comment":
		return h.handleCommentRemoved(ctx, resourceID, payload)

	case "for_sale":
		return h.handleForSaleRemoved(ctx, resourceID, payload)

	case "auction":
		return h.handleAuctionRemoved(ctx, resourceID, payload)

	case "user":
		return h.handleUserAction(ctx, resourceID, payload)

	case "chat_message":
		return h.handleChatMessageHidden(ctx, resourceID, payload)

	default:
		h.log.Warn("Unknown resource type in moderation event",
			zap.String("resource_type", payload.ResourceType),
			zap.String("event_id", event.ID.String()),
		)
		// Don't fail - future resource types may be added
		return nil
	}
}

// handleRestoration processes restoration events.
func (h *ModerationEventHandler) handleRestoration(ctx context.Context, event platformevent.OutboxEvent) error {
	// Parse restoration payload
	var payload moderationRestoredPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		h.log.Error("Failed to parse moderation restoration payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		// Don't retry on parse error - payload is malformed
		return nil
	}

	resourceID, err := uuid.Parse(payload.ResourceID)
	if err != nil {
		h.log.Error("Invalid resource ID in restoration payload",
			zap.String("event_id", event.ID.String()),
			zap.String("resource_id", payload.ResourceID),
			zap.Error(err),
		)
		// Don't retry on invalid ID
		return nil
	}

	// Route based on resource type
	switch payload.ResourceType {
	case "content":
		return h.handleContentRestored(ctx, resourceID, payload)

	case "comment":
		return h.handleCommentRestored(ctx, resourceID, payload)

	case "for_sale":
		return h.handleForSaleRestored(ctx, resourceID, payload)

	case "auction":
		return h.handleAuctionRestored(ctx, resourceID, payload)

	case "user":
		return h.handleUserRestored(ctx, resourceID, payload)

	case "chat_message":
		return h.handleChatMessageRestored(ctx, resourceID, payload)

	default:
		h.log.Warn("Unknown resource type in moderation restoration event",
			zap.String("resource_type", payload.ResourceType),
			zap.String("event_id", event.ID.String()),
		)
		// Don't fail - future resource types may be added
		return nil
	}
}

// splitEventSuffix splits an event type by dots and returns the parts.
func splitEventSuffix(eventType string) []string {
	parts := make([]string, 0)
	current := ""
	for _, c := range eventType {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// parseEnforcementID extracts the enforcement_id from a payload.
// Returns nil if the payload has no enforcement_id (legacy events).
func parseEnforcementID(payload moderationRemovedPayload) *uuid.UUID {
	if payload.EnforcementID == "" {
		return nil
	}
	id, err := uuid.Parse(payload.EnforcementID)
	if err != nil {
		return nil
	}
	return &id
}

// enforceLifecycle runs the canonical enforcement lifecycle within a transaction:
//
//	MarkProcessing → target mutation → MarkSucceeded
//
// If enforcementID is nil (legacy event), the lifecycle is skipped.
// If MarkProcessing fails, the tx rolls back (infra error).
// If target mutation fails, the tx rolls back and outbox retries.
// If MarkSucceeded fails (extremely rare), the tx rolls back and outbox retries.
//
// Returns any error from the lifecycle (caller must decide tx outcome).
func (h *ModerationEventHandler) enforceLifecycle(
	ctx context.Context,
	tx db.Tx,
	enforcementID *uuid.UUID,
	targetFn func() error,
) error {
	if enforcementID == nil || h.enfRepo == nil {
		return targetFn()
	}

	// Step 1: Mark enforcement as processing.
	if err := h.enfRepo.MarkProcessing(ctx, tx, *enforcementID); err != nil {
		return fmt.Errorf("mark enforcement processing failed: %w", err)
	}

	// Step 2: Execute target-domain mutation.
	if err := targetFn(); err != nil {
		return err
	}

	// Step 3: Mark enforcement as succeeded.
	if err := h.enfRepo.MarkSucceeded(ctx, tx, *enforcementID); err != nil {
		return fmt.Errorf("mark enforcement succeeded failed: %w", err)
	}

	return nil
}

// handleContentRemoved soft-deletes content via ContentService.
//
// CANONICAL ENFORCEMENT LIFECYCLE: MarkProcessing → soft-delete → MarkSucceeded
// All within one transaction for atomicity. If any step fails, the entire
// transaction rolls back and the outbox worker retries.
//
// Idempotent: calling on already-deleted content returns nil.
func (h *ModerationEventHandler) handleContentRemoved(
	ctx context.Context,
	contentID uuid.UUID,
	payload moderationRemovedPayload,
) error {
	h.log.Info("Soft-deleting content due to moderation removal",
		zap.String("content_id", contentID.String()),
		zap.String("case_id", payload.CaseID),
	)

	// Guard: contentService not configured
	if h.contentService == nil {
		h.log.Warn("ContentService not configured, skipping content soft-delete")
		return nil // Don't retry - configuration issue
	}

	enforcementID := parseEnforcementID(payload)

	// Canonical enforcement lifecycle within a single transaction:
	// MarkProcessing → soft-delete → MarkSucceeded
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.enforceLifecycle(ctx, tx, enforcementID, func() error {
			return h.contentService.SoftDeleteForModeration(ctx, tx, contentID)
		})
	})

	if err != nil {
		h.log.Error("Failed to soft-delete content for moderation",
			zap.String("content_id", contentID.String()),
			zap.String("case_id", payload.CaseID),
			zap.Error(err),
		)
		return fmt.Errorf("soft-delete content failed: %w", err)
	}

	h.log.Info("Successfully soft-deleted content for moderation",
		zap.String("content_id", contentID.String()),
		zap.String("case_id", payload.CaseID),
	)

	return nil
}

// handleCommentRemoved soft-deletes comment via CommentService.
//
// CANONICAL ENFORCEMENT LIFECYCLE: MarkProcessing → soft-delete → MarkSucceeded
// All within one transaction for atomicity.
//
// Idempotent: calling on already-deleted comment returns nil.
func (h *ModerationEventHandler) handleCommentRemoved(
	ctx context.Context,
	commentID uuid.UUID,
	payload moderationRemovedPayload,
) error {
	h.log.Info("Soft-deleting comment due to moderation removal",
		zap.String("comment_id", commentID.String()),
		zap.String("case_id", payload.CaseID),
	)

	// Guard: commentService not configured
	if h.commentService == nil {
		h.log.Warn("CommentService not configured, skipping comment soft-delete")
		return nil // Don't retry - configuration issue
	}

	enforcementID := parseEnforcementID(payload)

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.enforceLifecycle(ctx, tx, enforcementID, func() error {
			return h.commentService.SoftDeleteForModeration(ctx, tx, commentID)
		})
	})

	if err != nil {
		h.log.Error("Failed to soft-delete comment for moderation",
			zap.String("comment_id", commentID.String()),
			zap.String("case_id", payload.CaseID),
			zap.Error(err),
		)
		return fmt.Errorf("soft-delete comment failed: %w", err)
	}

	h.log.Info("Successfully soft-deleted comment for moderation",
		zap.String("comment_id", commentID.String()),
		zap.String("case_id", payload.CaseID),
	)

	return nil
}

// handleContentRestored restores content via ContentService.
//
// Creates its own transaction for the restore operation.
// Idempotent: calling on already-restored content returns nil.
func (h *ModerationEventHandler) handleContentRestored(
	ctx context.Context,
	contentID uuid.UUID,
	payload moderationRestoredPayload,
) error {
	h.log.Info("Restoring content due to approved appeal",
		zap.String("content_id", contentID.String()),
		zap.String("case_id", payload.CaseID),
		zap.String("appeal_id", payload.AppealID),
	)

	// Guard: contentService not configured
	if h.contentService == nil {
		h.log.Warn("ContentService not configured, skipping content restoration")
		return nil // Don't retry - configuration issue
	}

	// Execute restoration in a new transaction
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.contentService.RestoreFromModeration(ctx, tx, contentID)
	})

	if err != nil {
		h.log.Error("Failed to restore content for approved appeal",
			zap.String("content_id", contentID.String()),
			zap.String("case_id", payload.CaseID),
			zap.String("appeal_id", payload.AppealID),
			zap.Error(err),
		)
		// Return error to trigger retry
		return fmt.Errorf("restore content failed: %w", err)
	}

	h.log.Info("Successfully restored content for approved appeal",
		zap.String("content_id", contentID.String()),
		zap.String("case_id", payload.CaseID),
		zap.String("appeal_id", payload.AppealID),
	)

	return nil
}

// handleCommentRestored restores comment via CommentService.
//
// Creates its own transaction for the restore operation.
// Idempotent: calling on already-restored comment returns nil.
func (h *ModerationEventHandler) handleCommentRestored(
	ctx context.Context,
	commentID uuid.UUID,
	payload moderationRestoredPayload,
) error {
	h.log.Info("Restoring comment due to approved appeal",
		zap.String("comment_id", commentID.String()),
		zap.String("case_id", payload.CaseID),
		zap.String("appeal_id", payload.AppealID),
	)

	// Guard: commentService not configured
	if h.commentService == nil {
		h.log.Warn("CommentService not configured, skipping comment restoration")
		return nil // Don't retry - configuration issue
	}

	// Execute restoration in a new transaction
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.commentService.RestoreFromModeration(ctx, tx, commentID)
	})

	if err != nil {
		h.log.Error("Failed to restore comment for approved appeal",
			zap.String("comment_id", commentID.String()),
			zap.String("case_id", payload.CaseID),
			zap.String("appeal_id", payload.AppealID),
			zap.Error(err),
		)
		// Return error to trigger retry
		return fmt.Errorf("restore comment failed: %w", err)
	}

	h.log.Info("Successfully restored comment for approved appeal",
		zap.String("comment_id", commentID.String()),
		zap.String("case_id", payload.CaseID),
		zap.String("appeal_id", payload.AppealID),
	)

	return nil
}

// handleForSaleRemoved handles fixed-price sale enforcement.
//
// CANONICAL ENFORCEMENT LIFECYCLE: MarkProcessing → Withdraw → MarkSucceeded
// All within one transaction for atomicity.
//
// IDEMPOTENT: Safe to retry - Withdraw() validates fixed-price sale state.
// InvalidTransitionError (already terminal) is handled INSIDE enforceLifecycle
// so that the lifecycle always passes through processing → succeeded.
func (h *ModerationEventHandler) handleForSaleRemoved(
	ctx context.Context,
	forSaleID uuid.UUID,
	payload moderationRemovedPayload,
) error {
	h.log.Info("Handling fixed-price sale moderation removal",
		zap.String("for_sale_id", forSaleID.String()),
		zap.String("case_id", payload.CaseID),
		zap.String("decision_note", fmt.Sprintf("%v", payload.DecisionNote)),
	)

	// Guard: forSaleService not configured
	if h.forSaleService == nil {
		h.log.Warn("ForSaleService not configured, skipping fixed-price sale enforcement",
			zap.String("for_sale_id", forSaleID.String()),
		)
		return nil // Don't retry - configuration issue
	}

	enforcementID := parseEnforcementID(payload)

	// Canonical enforcement lifecycle within a single transaction.
	// InvalidTransitionError is caught INSIDE the target function so that
	// MarkProcessing → MarkSucceeded always executes atomically.
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.enforceLifecycle(ctx, tx, enforcementID, func() error {
			withdrawErr := h.forSaleService.Withdraw(ctx, tx, forSaleID)
			if withdrawErr != nil {
				// IDEMPOTENCY: If fixed-price sale is already in a terminal state
				// (withdrawn/sold), Withdraw() returns InvalidTransitionError.
				// The sale is no longer purchasable, which is the moderation intent.
				var ite *forSaleEntity.InvalidTransitionError
				if errors.As(withdrawErr, &ite) {
					h.log.Info("Fixed-price sale already in terminal state, treating as idempotent",
						zap.String("for_sale_id", forSaleID.String()),
						zap.String("case_id", payload.CaseID),
						zap.String("current_status", string(ite.CurrentStatus)),
						zap.String("target_status", string(ite.TargetStatus)),
					)
					return nil // Skip mutation, lifecycle proceeds to MarkSucceeded
				}
				return withdrawErr // Real failure — TX rolls back, outbox retries
			}
			return nil
		})
	})

	if err != nil {
		h.log.Error("Failed to withdraw fixed-price sale for moderation",
			zap.String("for_sale_id", forSaleID.String()),
			zap.String("case_id", payload.CaseID),
			zap.Error(err),
		)
		return fmt.Errorf("withdraw fixed-price sale failed: %w", err)
	}

	h.log.Info("Successfully withdrew fixed-price sale for moderation",
		zap.String("for_sale_id", forSaleID.String()),
		zap.String("case_id", payload.CaseID),
	)

	return nil
}

// handleAuctionRemoved handles auction moderation enforcement.
//
// CANONICAL ENFORCEMENT LIFECYCLE: MarkProcessing → CancelForModeration → MarkSucceeded
// All within one transaction for atomicity.
//
// IDEMPOTENT: Terminal states (ended, cancelled) return
// InvalidTransitionError which is handled INSIDE enforceLifecycle so that
// the lifecycle always passes through processing → succeeded.
func (h *ModerationEventHandler) handleAuctionRemoved(
	ctx context.Context,
	auctionID uuid.UUID,
	payload moderationRemovedPayload,
) error {
	h.log.Info("Handling auction moderation removal",
		zap.String("auction_id", auctionID.String()),
		zap.String("case_id", payload.CaseID),
	)

	if h.auctionService == nil {
		h.log.Warn("AuctionService not configured, skipping auction enforcement",
			zap.String("auction_id", auctionID.String()),
		)
		return nil // Don't retry - configuration issue
	}

	enforcementID := parseEnforcementID(payload)

	// Canonical enforcement lifecycle within a single transaction.
	// InvalidTransitionError is caught INSIDE the target function so that
	// MarkProcessing → MarkSucceeded always executes atomically.
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.enforceLifecycle(ctx, tx, enforcementID, func() error {
			cancelErr := h.auctionService.CancelForModeration(ctx, tx, auctionID)
			if cancelErr != nil {
				// IDEMPOTENCY: Terminal states (ended, cancelled) cannot
				// transition to cancelled. Auction is no longer active.
				var ite *auctionEntity.InvalidTransitionError
				if errors.As(cancelErr, &ite) {
					h.log.Info("Auction already in terminal state, treating as idempotent",
						zap.String("auction_id", auctionID.String()),
						zap.String("case_id", payload.CaseID),
						zap.String("current_status", string(ite.CurrentStatus)),
					)
					return nil // Skip mutation, lifecycle proceeds to MarkSucceeded
				}
				return cancelErr // Real failure — TX rolls back, outbox retries
			}
			return nil
		})
	})

	if err != nil {
		h.log.Error("Failed to cancel auction for moderation",
			zap.String("auction_id", auctionID.String()),
			zap.String("case_id", payload.CaseID),
			zap.Error(err),
		)
		return fmt.Errorf("cancel auction failed: %w", err)
	}

	h.log.Info("Successfully cancelled auction for moderation",
		zap.String("auction_id", auctionID.String()),
		zap.String("case_id", payload.CaseID),
	)

	return nil
}

// handleUserAction handles user enforcement.
//
// CANONICAL ENFORCEMENT LIFECYCLE: MarkProcessing → suspend user → MarkSucceeded
// All within one transaction for atomicity.
//
// IDEMPOTENT: Only updates if not already suspended.
func (h *ModerationEventHandler) handleUserAction(
	ctx context.Context,
	userID uuid.UUID,
	payload moderationRemovedPayload,
) error {
	h.log.Info("Handling user moderation action",
		zap.String("user_id", userID.String()),
		zap.String("case_id", payload.CaseID),
		zap.String("decision_note", fmt.Sprintf("%v", payload.DecisionNote)),
	)

	// Guard: userRepo not configured
	if h.userRepo == nil {
		h.log.Warn("UserRepository not configured, skipping user enforcement",
			zap.String("user_id", userID.String()),
		)
		return nil // Don't retry - configuration issue
	}

	enforcementID := parseEnforcementID(payload)

	// Canonical enforcement lifecycle within a single transaction.
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.enforceLifecycle(ctx, tx, enforcementID, func() error {
			// Lock user for update
			user, err := h.userRepo.GetByIDForUpdate(ctx, tx, userID)
			if err != nil {
				return fmt.Errorf("failed to lock user: %w", err)
			}

			// IDEMPOTENCY CHECK: Only update if not already suspended
			if user.AccountStatus == "suspended" {
				h.log.Info("User already suspended, skipping update",
					zap.String("user_id", userID.String()),
				)
				return nil // Already suspended - idempotent
			}

			// Update account status to suspended
			user.AccountStatus = "suspended"
			user.UpdatedAt = time.Now()

			// Persist the change
			if err := h.userRepo.Update(ctx, tx, user); err != nil {
				return fmt.Errorf("failed to update user status: %w", err)
			}

			return nil
		})
	})

	if err != nil {
		h.log.Error("Failed to suspend user for moderation",
			zap.String("user_id", userID.String()),
			zap.String("case_id", payload.CaseID),
			zap.Error(err),
		)
		return fmt.Errorf("suspend user failed: %w", err)
	}

	h.log.Info("Successfully suspended user for moderation",
		zap.String("user_id", userID.String()),
		zap.String("case_id", payload.CaseID),
	)

	return nil
}

// handleForSaleRestored handles fixed-price sale restoration (appeal approved).
//
// ENFORCEMENT: Restore fixed-price sale to active via ForSaleService.RestoreFromModeration().
// This reverses the moderation Withdraw() that was applied on removal.
//
// GUARD: Only restores withdrawn fixed-price sales. Sold sales (inventory claimed)
// and already-active sales are handled gracefully (error logged, no retry).
//
// IDEMPOTENT: Safe to retry — RestoreFromModeration() is idempotent for
// already-active sales.
func (h *ModerationEventHandler) handleForSaleRestored(
	ctx context.Context,
	forSaleID uuid.UUID,
	payload moderationRestoredPayload,
) error {
	h.log.Info("Restoring fixed-price sale due to approved appeal",
		zap.String("for_sale_id", forSaleID.String()),
		zap.String("case_id", payload.CaseID),
		zap.String("appeal_id", payload.AppealID),
	)

	// Guard: forSaleService not configured
	if h.forSaleService == nil {
		h.log.Warn("ForSaleService not configured, skipping fixed-price sale restoration",
			zap.String("for_sale_id", forSaleID.String()),
		)
		return nil // Don't retry — configuration issue
	}

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.forSaleService.RestoreFromModeration(ctx, tx, forSaleID)
	})

	if err != nil {
		// GUARD: sold or draft fixed-price sale — restoration not applicable, do not retry.
		// These are genuine cases (seller sold during moderation review, etc.).
		if isNonRetryableRestoreError(err) {
			h.log.Warn("Fixed-price sale not eligible for moderation restore (sold or unexpected status) — skipping",
				zap.String("for_sale_id", forSaleID.String()),
				zap.String("case_id", payload.CaseID),
				zap.Error(err),
			)
			return nil
		}
		h.log.Error("Failed to restore fixed-price sale for moderation",
			zap.String("for_sale_id", forSaleID.String()),
			zap.String("case_id", payload.CaseID),
			zap.Error(err),
		)
		return fmt.Errorf("restore fixed-price sale failed: %w", err)
	}

	h.log.Info("Successfully restored fixed-price sale from moderation",
		zap.String("for_sale_id", forSaleID.String()),
		zap.String("case_id", payload.CaseID),
	)

	return nil
}

// isNonRetryableRestoreError returns true if the restore error should not
// trigger a retry (sold inventory, draft, or not-found with no-op intent).
func isNonRetryableRestoreError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "status is sold") ||
		strings.Contains(msg, "unexpected status") ||
		strings.Contains(msg, "fixed-price sale not found")
}

// handleAuctionRestored handles auction restoration (appeal approved).
//
// INTENTIONALLY UNSUPPORTED: Auction restoration via moderation appeal is not
// implemented. Auctions that are cancelled by moderation cannot be automatically
// restarted because:
//   - Auction timing is deterministic (scheduled start/end times must be reset)
//   - Bid state cannot be safely reconstructed
//   - Claim/settlement windows may have passed
//
// Seller action required: create a new auction after the appeal is resolved.
//
// This is NOT a silent no-op bug — it is an explicit, documented decision.
// The notification (moderation.auction.restored) informs the seller the appeal
// succeeded; the seller understands they must re-list.
func (h *ModerationEventHandler) handleAuctionRestored(
	ctx context.Context,
	auctionID uuid.UUID,
	payload moderationRestoredPayload,
) error {
	h.log.Info("Auction moderation appeal approved — seller must create new auction (restoration intentionally unsupported)",
		zap.String("auction_id", auctionID.String()),
		zap.String("case_id", payload.CaseID),
		zap.String("appeal_id", payload.AppealID),
	)
	return nil
}

// handleUserRestored handles user restoration (appeal approved).
//
// ENFORCEMENT: Reactivate user account by setting account_status = 'active'.
// This restores user capabilities and allows normal actions.
//
// IDEMPOTENT: Only updates if not already active.
func (h *ModerationEventHandler) handleUserRestored(
	ctx context.Context,
	userID uuid.UUID,
	payload moderationRestoredPayload,
) error {
	h.log.Info("Restoring user due to approved appeal",
		zap.String("user_id", userID.String()),
		zap.String("case_id", payload.CaseID),
		zap.String("appeal_id", payload.AppealID),
	)

	// Guard: userRepo not configured
	if h.userRepo == nil {
		h.log.Warn("UserRepository not configured, skipping user restoration",
			zap.String("user_id", userID.String()),
		)
		return nil // Don't retry - configuration issue
	}

	// Execute restoration in a new transaction
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		// Lock user for update
		user, err := h.userRepo.GetByIDForUpdate(ctx, tx, userID)
		if err != nil {
			return fmt.Errorf("failed to lock user: %w", err)
		}

		// IDEMPOTENCY CHECK: Only update if not already active
		if user.AccountStatus == "active" {
			h.log.Info("User already active, skipping update",
				zap.String("user_id", userID.String()),
			)
			return nil // Already active - idempotent
		}

		// BAN PERMANENCE GUARD: Moderation restoration MUST NOT revive
		// banned accounts. Ban reversal requires explicit admin unban
		// with governance.user.unban capability.
		if user.AccountStatus == "banned" {
			h.log.Warn("Cannot restore banned user via moderation appeal — requires explicit admin unban",
				zap.String("user_id", userID.String()),
				zap.String("case_id", payload.CaseID),
				zap.String("appeal_id", payload.AppealID),
			)
			return nil // Do not retry — this is a policy decision, not a transient error
		}

		// Update account status to active
		user.AccountStatus = "active"
		user.UpdatedAt = time.Now()

		// Persist the change
		if err := h.userRepo.Update(ctx, tx, user); err != nil {
			return fmt.Errorf("failed to update user status: %w", err)
		}

		return nil
	})

	if err != nil {
		h.log.Error("Failed to restore user for approved appeal",
			zap.String("user_id", userID.String()),
			zap.String("case_id", payload.CaseID),
			zap.String("appeal_id", payload.AppealID),
			zap.Error(err),
		)
		// Return error to trigger retry
		return fmt.Errorf("restore user failed: %w", err)
	}

	h.log.Info("Successfully restored user for approved appeal",
		zap.String("user_id", userID.String()),
		zap.String("case_id", payload.CaseID),
		zap.String("appeal_id", payload.AppealID),
	)

	return nil
}

// handleChatMessageHidden soft-hides a chat message via chat boundary.
//
// CANONICAL ENFORCEMENT LIFECYCLE: MarkProcessing → hide → MarkSucceeded
// All within one transaction for atomicity.
//
// IDEMPOTENT: repository operation is guarded by deleted_at IS NULL.
func (h *ModerationEventHandler) handleChatMessageHidden(
	ctx context.Context,
	messageID uuid.UUID,
	payload moderationRemovedPayload,
) error {
	h.log.Info("Soft-hiding chat message due to moderation enforcement",
		zap.String("message_id", messageID.String()),
		zap.String("case_id", payload.CaseID),
	)

	enforcementID := parseEnforcementID(payload)
	reason := fmt.Sprintf("Moderation: hidden by admin (case %s)", payload.CaseID)

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.enforceLifecycle(ctx, tx, enforcementID, func() error {
			return h.chatMessageStore.SoftHideForModeration(ctx, tx, messageID, uuid.Nil, reason, payload.CaseID)
		})
	})

	if err != nil {
		h.log.Error("Failed to soft-hide chat message for moderation",
			zap.String("message_id", messageID.String()),
			zap.String("case_id", payload.CaseID),
			zap.Error(err),
		)
		return fmt.Errorf("soft-hide chat message failed: %w", err)
	}

	h.log.Info("Successfully soft-hid chat message for moderation",
		zap.String("message_id", messageID.String()),
		zap.String("case_id", payload.CaseID),
	)

	return nil
}

// handleChatMessageRestored restores a soft-hidden chat message via chat boundary.
//
// IDEMPOTENT: repository operation is guarded by deleted_at IS NOT NULL.
func (h *ModerationEventHandler) handleChatMessageRestored(
	ctx context.Context,
	messageID uuid.UUID,
	payload moderationRestoredPayload,
) error {
	h.log.Info("Restoring chat message due to approved appeal",
		zap.String("message_id", messageID.String()),
		zap.String("case_id", payload.CaseID),
		zap.String("appeal_id", payload.AppealID),
	)

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.chatMessageStore.RestoreFromModeration(ctx, tx, messageID, payload.AppealID)
	})

	if err != nil {
		h.log.Error("Failed to restore chat message for approved appeal",
			zap.String("message_id", messageID.String()),
			zap.String("case_id", payload.CaseID),
			zap.String("appeal_id", payload.AppealID),
			zap.Error(err),
		)
		return fmt.Errorf("restore chat message failed: %w", err)
	}

	h.log.Info("Successfully restored chat message for approved appeal",
		zap.String("message_id", messageID.String()),
		zap.String("case_id", payload.CaseID),
		zap.String("appeal_id", payload.AppealID),
	)

	return nil
}



