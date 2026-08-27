package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	forSaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forSaleRepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	negotiationImpl "github.com/labuda/backend/internal/commerce/negotiation/infrastructure/repository"
	negotiationRepo "github.com/labuda/backend/internal/commerce/negotiation/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// AccountStatusChecker is the service-layer interface for account status enforcement.
// Defined locally to avoid importing the auth package from this domain layer.
type AccountStatusChecker interface {
	EnsureActive(ctx context.Context, userID uuid.UUID) error
}

// BlockChecker is the service-layer interface for bidirectional block enforcement.
// Defined locally to avoid importing the social domain from this domain layer.
type BlockChecker interface {
	// IsBlockedInEitherDirection returns true if userA blocked userB OR userB blocked userA.
	IsBlockedInEitherDirection(ctx context.Context, userA, userB uuid.UUID) (bool, error)
}

// StartNegotiationRequest contains the parameters for starting a negotiation.
//
// RoomID / RoomOtherParticipantID are supplied by the chat delivery layer,
// which already resolved and participant-checked the room. The negotiation
// service does not depend on the chat domain (see STRICT BOUNDARY below) —
// it only receives plain UUIDs to (a) validate the room's counterparty is
// the resolved seller and (b) persist the session's chat_room_id so
// GetNegotiation/CreateOrderFromChat can later find it by that same room.
type StartNegotiationRequest struct {
	ResourceType           negotiationEntity.NegotiationResourceType
	ForSaleID       uuid.UUID
	BuyerID                uuid.UUID
	InitialPrice           int64
	Note                   string
	RoomID                 uuid.UUID
	RoomOtherParticipantID uuid.UUID
}

// SendCounterOfferRequest contains the parameters for sending a counter offer.
type SendCounterOfferRequest struct {
	SessionID uuid.UUID
	SenderID  uuid.UUID
	Price     int64
	Note      string
}

// AcceptNegotiationRequest contains the parameters for accepting a negotiation.
type AcceptNegotiationRequest struct {
	SessionID uuid.UUID
	SellerID  uuid.UUID
}

// CancelNegotiationRequest contains the parameters for cancelling a negotiation.
type CancelNegotiationRequest struct {
	SessionID uuid.UUID
	BuyerID   uuid.UUID
}

// NegotiationService handles negotiation business logic with proper locking and validation.
//
// STRICT BOUNDARY: This service does NOT:
// - Create orders
// - Modify ledger
// - Reserve stock
// - Modify resource prices
// - Emit financial events
// - Create chat rooms or send messages
//
// DOMAIN DECOUPLING:
// - Negotiation is independent of Chat domain
// - Chat room creation is handled by Chat domain via event consumers
// - negotiation.started event triggers chat room creation
// - negotiation.message_sent event triggers proposal messages
// - No direct chat service dependencies
type NegotiationService struct {
	db                 *db.DB
	negotiationRepo    negotiationRepo.Repository
	forSaleRepo forSaleRepo.ForSaleRepository
	outboxRepo         *outboxRepo.OutboxRepository
	statusChecker      AccountStatusChecker // Account status enforcement (service-layer authority)
	blockChecker       BlockChecker         // Block enforcement: denies new negotiation when block exists
	log                *zap.Logger
}

// NewNegotiationService creates a new NegotiationService.
func NewNegotiationService(
	db *db.DB,
	forSaleRepo forSaleRepo.ForSaleRepository,
	outboxRepo *outboxRepo.OutboxRepository,
	statusChecker AccountStatusChecker,
	blockChecker BlockChecker,
	log *zap.Logger,
) *NegotiationService {
	if log == nil {
		log = zap.NewNop()
	}

	return &NegotiationService{
		db:                 db,
		negotiationRepo:    negotiationImpl.NewNegotiationRepository(),
		forSaleRepo: forSaleRepo,
		outboxRepo:         outboxRepo,
		statusChecker:      statusChecker,
		blockChecker:       blockChecker,
		log:                log,
	}
}

// EnsureParticipantsActive checks that both buyer and seller accounts are active.
// Called by transactional negotiation actions (counter-offer, accept).
// Suspended users cannot perform new negotiation actions.
// CancelNegotiation deliberately does NOT call this — suspended users may cancel.
func (s *NegotiationService) EnsureParticipantsActive(ctx context.Context, session *negotiationEntity.NegotiationSession) error {
	if s.statusChecker == nil {
		return nil
	}
	if err := s.statusChecker.EnsureActive(ctx, session.BuyerID); err != nil {
		return fmt.Errorf("buyer account not active: %w", err)
	}
	if err := s.statusChecker.EnsureActive(ctx, session.SellerID); err != nil {
		return fmt.Errorf("seller account not active: %w", err)
	}
	return nil
}

// StartNegotiation initiates a new negotiation session.
//
// TRANSACTION: All operations happen within transactions (ChatService and NegotiationService use their own).
// VALIDATION:
//   - Fixed-price sale exists and is negotiable
//   - Chat room's other participant (from caller's perspective) is exactly
//     the resolved seller — PASS_7B / F2, closes the room-counterparty gap
//   - No active session exists for this buyer+fixed-price sale
//   - Initial price > 0
//   - Buyer != Seller
//
// EMITS: negotiation.started event (non-financial, for chat attachment)
//
// NEGOTIATION → CHAT UNIFICATION (PASS_7B):
// - chat_room_id is set on the session directly from the caller-supplied,
//   participant-validated RoomID (see StartNegotiationRequest) — this is
//   the same room GetNegotiation/CreateOrderFromChat later look it up by.
// - The chat-domain consumer (NegotiationEventHandler) still separately
//   creates/resolves a room_type=negotiation room and posts the initial
//   proposal message there; that room is independent of chat_room_id and
//   is out of scope for this fix (see PASS_7B report, remaining findings).
// - No direct chatRepo usage in this service.
func (s *NegotiationService) StartNegotiation(
	ctx context.Context,
	req StartNegotiationRequest,
) (*negotiationEntity.NegotiationSession, error) {
	// Phase 1: Validate fixed-price sale and get seller_id (in transaction)
	var sellerID uuid.UUID
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		sellerID, err = s.validateForSaleAndGetSeller(ctx, tx, req.ResourceType, req.ForSaleID)
		return err
	})
	if err != nil {
		return nil, err
	}

	// Phase 1.5: Room-counterparty validation (PASS_7B / F2).
	// The chat room's other participant (from the buyer's perspective) must
	// be exactly the resolved seller. A room with an unrelated third party,
	// a support room (other participant = uuid.Nil), or an unrelated
	// direct/negotiation room must not be usable to start a negotiation for
	// this listing — room membership alone is not negotiation authority.
	if req.RoomOtherParticipantID != sellerID {
		return nil, &ErrNegotiationRoomMismatch{
			BuyerID:  req.BuyerID,
			SellerID: sellerID,
			RoomID:   req.RoomID,
		}
	}

	// Phase 2: Validate buyer != seller (can be done outside transaction)
	if req.BuyerID == sellerID {
		return nil, &ErrCannotNegotiateWithSelf{
			BuyerID:  req.BuyerID,
			SellerID: sellerID,
		}
	}

	// Phase 2.5: Block check — deny new negotiation if either party has blocked the other.
	// Existing sessions (created before the block) are not affected; they continue to terminal state.
	if s.blockChecker != nil {
		blocked, err := s.blockChecker.IsBlockedInEitherDirection(ctx, req.BuyerID, sellerID)
		if err != nil {
			return nil, fmt.Errorf("failed to check block relationship: %w", err)
		}
		if blocked {
			return nil, &ErrNegotiationBlockedByRelationship{
				BuyerID:  req.BuyerID,
				SellerID: sellerID,
			}
		}
	}

	// Phase 3: Check for existing active session (in transaction)
	var existingSession *negotiationEntity.NegotiationSession
	err = s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		existingSession, err = s.negotiationRepo.GetActiveSessionByResourceAndBuyer(
			ctx, tx, req.ResourceType, req.ForSaleID, req.BuyerID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check existing session: %w", err)
	}
	if existingSession != nil {
		return nil, &ErrActiveSessionExists{
			SessionID:        existingSession.ID,
			ForSaleID: req.ForSaleID,
			BuyerID:          req.BuyerID,
		}
	}

	// Phase 4: Create negotiation session with chat_room_id already set
	// (PASS_7B / F1). The room was validated in Phase 1.5 to have exactly
	// {buyer, seller} as participants, so it is the correct, permanent
	// chat_room_id for this session — no separate async write-back needed.
	var newSession *negotiationEntity.NegotiationSession
	err = s.db.WithTx(ctx, func(tx db.Tx) error {
		newSession = negotiationEntity.NewNegotiationSession(
			req.ResourceType,
			req.ForSaleID,
			req.BuyerID,
			sellerID,
		)
		newSession.SetChatRoomID(req.RoomID)
		// Set current_price from initial price with validation
		if err := newSession.SetCurrentPrice(req.InitialPrice); err != nil {
			return fmt.Errorf("failed to set initial price: %w", err)
		}

		if err := s.negotiationRepo.CreateSession(ctx, tx, newSession); err != nil {
			return fmt.Errorf("failed to create negotiation session: %w", err)
		}

		// Create audit log for initial proposal
		priceHistory := negotiationEntity.NewNegotiationPriceHistory(
			newSession.ID,
			newSession.ProposalSequence,
			nil,              // old_price is nil for initial proposal
			req.InitialPrice, // new_price
			req.BuyerID,
			"initial_proposal",
		)
		if err := s.negotiationRepo.CreatePriceHistoryEntry(ctx, tx, priceHistory); err != nil {
			return fmt.Errorf("failed to create price history: %w", err)
		}

		// Emit negotiation.started event (NON-FINANCIAL)
		// Chat domain will consume this to send the initial proposal message
		// directly into chat_room_id (PASS_8A / F4) — chat_room_id is included
		// here (unlike before Pass 7B) because it is now known synchronously
		// at session-creation time; the notification worker's DB-fallback
		// lookup remains as a defensive no-op for any other caller.
		payload := fmt.Sprintf(`{
			"session_id":"%s",
			"chat_room_id":"%s",
			"resource_type":"%s",
			"resource_id":"%s",
			"buyer_id":"%s",
			"seller_id":"%s",
			"initial_price":%d,
			"note":"%s"
		}`,
			newSession.ID,
			negotiationChatRoomIDStr(newSession.ChatRoomID),
			newSession.ResourceType,
			newSession.ForSaleID,
			newSession.BuyerID,
			newSession.SellerID,
			req.InitialPrice,
			req.Note,
		)

		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			"negotiation.started",
			newSession.ID,
			[]byte(payload),
		); err != nil {
			return fmt.Errorf("failed to insert outbox event: %w", err)
		}

		s.log.Info("Negotiation started",
			zap.String("session_id", newSession.ID.String()),
			zap.String("resource_type", string(newSession.ResourceType)),
			zap.String("for_sale_id", newSession.ForSaleID.String()),
			zap.String("buyer_id", newSession.BuyerID.String()),
			zap.String("seller_id", newSession.SellerID.String()),
			zap.Int64("initial_price", req.InitialPrice),
		)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return newSession, nil
}

// SendCounterOffer sends a new price proposal in an active negotiation.
//
// TRANSACTION: All operations happen within a single transaction.
// VALIDATION:
//   - Session exists and is active
//   - Sender is buyer or seller (participant)
//   - Chat room is associated with session
//   - Price > 0 (business rule)
//   - Price is not absurd (guard against obvious errors)
//   - Session is not expired
//
// EMITS: negotiation.message_sent event (non-financial, for chat update)
//
// NEGOTIATION → CHAT UNIFICATION:
// - Sends proposal via ChatService.SendMessage with MessageTypeNegotiationProposal
// - Updates current_price as the authoritative source
// - No longer creates negotiation_messages table entries
//
// PRICE SECURITY HARDENING:
// - Uses SetCurrentPrice() with validation
// - Proposal sequence increments automatically (optimistic locking)
func (s *NegotiationService) SendCounterOffer(
	ctx context.Context,
	req SendCounterOfferRequest,
) error {
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Step 1: Lock session for update
		session, err := s.negotiationRepo.GetSessionForUpdate(ctx, tx, req.SessionID)
		if err != nil {
			return fmt.Errorf("failed to lock session: %w", err)
		}

		// Step 2: Ensure session is active
		if err := session.EnsureSessionActive(); err != nil {
			return err
		}

		// Step 2.5: Check if session is expired
		if session.IsExpired() {
			return &ErrNegotiationExpired{
				SessionID: req.SessionID,
				ExpiresAt: session.ExpiresAt,
			}
		}

		// Step 2.6: Both participants must have active accounts.
		// Suspended users cannot send counter-offers.
		if err := s.EnsureParticipantsActive(ctx, session); err != nil {
			return err
		}

		// Step 3: Validate sender is a participant
		if !session.IsParticipant(req.SenderID) {
			return &negotiationEntity.UnauthorizedParticipantError{
				SessionID: req.SessionID,
				UserID:    req.SenderID,
			}
		}

		// Step 4: Update current_price with validation (authoritative source)
		// This will validate price > 0, price not absurd, and increment proposal_sequence
		oldPrice := session.CurrentPrice
		if err := session.SetCurrentPrice(req.Price); err != nil {
			return fmt.Errorf("failed to set current price: %w", err)
		}

		// Step 5.5: Create audit log for price change
		priceHistory := negotiationEntity.NewNegotiationPriceHistory(
			session.ID,
			session.ProposalSequence,
			oldPrice,  // old_price (nil for initial proposal)
			req.Price, // new_price
			req.SenderID,
			"counter_offer",
		)
		if err := s.negotiationRepo.CreatePriceHistoryEntry(ctx, tx, priceHistory); err != nil {
			return fmt.Errorf("failed to create price history: %w", err)
		}

		// Step 6: Persist changes
		if err := s.negotiationRepo.UpdateSession(ctx, tx, session); err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}

		// Step 7: Emit negotiation.message_sent event (NON-FINANCIAL)
		// chat_room_id is included so the notification worker can deeplink to the chat.
		payload := fmt.Sprintf(`{
			"session_id":"%s",
			"chat_room_id":"%s",
			"buyer_id":"%s",
			"seller_id":"%s",
			"sender_id":"%s",
			"price":%d,
			"proposal_sequence":%d
		}`,
			req.SessionID,
			negotiationChatRoomIDStr(session.ChatRoomID),
			session.BuyerID,
			session.SellerID,
			req.SenderID,
			req.Price,
			session.ProposalSequence,
		)

		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			"negotiation.message_sent",
			req.SessionID,
			[]byte(payload),
		); err != nil {
			return fmt.Errorf("failed to insert outbox event: %w", err)
		}

		s.log.Info("Negotiation counter-offer sent",
			zap.String("session_id", req.SessionID.String()),
			zap.String("sender_id", req.SenderID.String()),
			zap.Int64("price", req.Price),
			zap.Int64("proposal_sequence", session.ProposalSequence),
		)

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

// AcceptNegotiation accepts the current price proposal.
//
// TRANSACTION: All operations happen within a single transaction.
// VALIDATION:
//   - Session exists and is active
//   - Only seller can accept
//   - Session is not expired
//   - current_price must be set and valid
//   - current_price consistency check (defensive)
//
// PRICE SECURITY HARDENING:
// - Validates current_price before acceptance
// - Creates audit log for acceptance
// - Sets accepted_price atomically from current_price
//
// EMITS: negotiation.accepted event with "Buy" button indicator (NON-FINANCIAL)
// NOTE: Does NOT create an order - order creation is a separate user action.
func (s *NegotiationService) AcceptNegotiation(
	ctx context.Context,
	req AcceptNegotiationRequest,
) (*negotiationEntity.NegotiationSession, error) {
	var acceptedSession *negotiationEntity.NegotiationSession

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Step 1: Lock session for update
		session, err := s.negotiationRepo.GetSessionForUpdate(ctx, tx, req.SessionID)
		if err != nil {
			return fmt.Errorf("failed to lock session: %w", err)
		}

		// Step 2: Ensure session is active
		if err := session.EnsureSessionActive(); err != nil {
			return err
		}

		// Step 2.5: Check if session is expired
		if session.IsExpired() {
			return &ErrNegotiationExpired{
				SessionID: req.SessionID,
				ExpiresAt: session.ExpiresAt,
			}
		}

		// Step 2.6: Both participants must have active accounts.
		// Suspended users cannot accept negotiations.
		if err := s.EnsureParticipantsActive(ctx, session); err != nil {
			return err
		}

		// Step 3: Validate only seller can accept
		if !session.IsSeller(req.SellerID) {
			return &negotiationEntity.NotSellerError{
				SessionID: req.SessionID,
				UserID:    req.SellerID,
			}
		}

		// ============================================================
		// PREVENT MULTIPLE ACCEPTED NEGOTIATIONS
		// ============================================================
		// Check if there's already an accepted negotiation for this fixed-price sale+buyer.
		// This prevents confusing state where buyer has multiple accepted prices for the same sale.
		existingAccepted, err := s.negotiationRepo.GetAcceptedSessionByResourceAndBuyer(
			ctx, tx, session.ResourceType, session.ForSaleID, session.BuyerID)
		if err != nil {
			return fmt.Errorf("failed to check existing accepted sessions: %w", err)
		}
		if existingAccepted != nil && existingAccepted.ID != session.ID {
			return &negotiationEntity.ErrMultipleAcceptedNegotiations{
				BuyerID:          session.BuyerID,
				ForSaleID: session.ForSaleID,
				ExistingID:       existingAccepted.ID,
				NewID:            session.ID,
			}
		}

		// Step 4: Transition to accepted with price (sets accepted_price and accepted_at)
		// This includes validation of current_price (must be set and valid)
		if err := session.AcceptWithPrice(); err != nil {
			return err
		}

		// Step 5: Create audit log for price acceptance
		// This creates a price history entry for the acceptance event
		priceHistory := negotiationEntity.NewNegotiationPriceHistory(
			session.ID,
			session.ProposalSequence,
			session.CurrentPrice,   // old_price = current_price
			*session.AcceptedPrice, // new_price = accepted_price
			req.SellerID,
			"price_accepted",
		)
		if err := s.negotiationRepo.CreatePriceHistoryEntry(ctx, tx, priceHistory); err != nil {
			return fmt.Errorf("failed to create price history: %w", err)
		}

		// Step 6: Persist changes
		if err := s.negotiationRepo.UpdateSession(ctx, tx, session); err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}

		acceptedSession = session

		// Step 7: Emit accepted event with "Buy" button (NON-FINANCIAL)
		// The chat UI will show a "Buy" button to the buyer.
		// chat_room_id included so mobile notification deeplinks to negotiation chat,
		// not to an order (order does not exist yet at acceptance time).
		payload := fmt.Sprintf(`{
			"session_id":"%s",
			"chat_room_id":"%s",
			"resource_type":"%s",
			"resource_id":"%s",
			"buyer_id":"%s",
			"seller_id":"%s",
			"accepted_price":%d,
			"proposal_sequence":%d
		}`,
			session.ID,
			negotiationChatRoomIDStr(session.ChatRoomID),
			session.ResourceType,
			session.ForSaleID,
			session.BuyerID,
			session.SellerID,
			*session.AcceptedPrice,
			session.ProposalSequence,
		)

		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			"negotiation.accepted",
			session.ID,
			[]byte(payload),
		); err != nil {
			return fmt.Errorf("failed to insert outbox event: %w", err)
		}

		s.log.Info("Negotiation accepted",
			zap.String("session_id", req.SessionID.String()),
			zap.String("seller_id", req.SellerID.String()),
			zap.Int64("accepted_price", *session.AcceptedPrice),
			zap.Int64("proposal_sequence", session.ProposalSequence),
		)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return acceptedSession, nil
}

// CancelNegotiation cancels an active negotiation session.
//
// TRANSACTION: All operations happen within a single transaction.
// VALIDATION:
//   - Session exists and is active
//   - Session is not expired
//   - Only buyer can cancel
//
// EMITS: negotiation.cancelled event (NON-FINANCIAL)
//
// NEGOTIATION EXPIRY CONSISTENCY: Prevents cancellation of expired negotiations.
func (s *NegotiationService) CancelNegotiation(
	ctx context.Context,
	req CancelNegotiationRequest,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Step 1: Lock session for update
		session, err := s.negotiationRepo.GetSessionForUpdate(ctx, tx, req.SessionID)
		if err != nil {
			return fmt.Errorf("failed to lock session: %w", err)
		}

		// Step 2: Ensure session is active
		if err := session.EnsureSessionActive(); err != nil {
			return err
		}

		// Step 2.5: Check if session is expired
		// NEGOTIATION EXPIRY CONSISTENCY: Prevent cancellation of expired negotiations
		if session.IsExpired() {
			return &ErrNegotiationExpired{
				SessionID: req.SessionID,
				ExpiresAt: session.ExpiresAt,
			}
		}

		// Step 3: Validate only buyer can cancel
		if !session.IsBuyer(req.BuyerID) {
			return &negotiationEntity.NotBuyerError{
				SessionID: req.SessionID,
				UserID:    req.BuyerID,
			}
		}

		// Step 4: Transition to cancelled
		if err := session.Cancel(); err != nil {
			return err
		}

		// Step 5: Persist changes
		if err := s.negotiationRepo.UpdateSession(ctx, tx, session); err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}

		// Step 6: Emit cancelled event (NON-FINANCIAL)
		payload := fmt.Sprintf(`{
			"session_id":"%s",
			"chat_room_id":"%s",
			"buyer_id":"%s",
			"seller_id":"%s"
		}`,
			session.ID,
			func() string {
				if session.ChatRoomID != nil {
					return session.ChatRoomID.String()
				}
				return ""
			}(),
			req.BuyerID,
			session.SellerID,
		)

		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			"negotiation.cancelled",
			session.ID,
			[]byte(payload),
		); err != nil {
			return fmt.Errorf("failed to insert outbox event: %w", err)
		}

		s.log.Info("Negotiation cancelled",
			zap.String("session_id", req.SessionID.String()),
			zap.String("buyer_id", req.BuyerID.String()),
		)

		return nil
	})
}

// ExpireSession transitions a negotiation session from active to expired.
// This is called by the NegotiationExpireWorker when a session passes its expires_at time.
//
// NEGOTIATION LIFECYCLE HARDENING:
// - Expired sessions cannot proceed to order creation
// - Expiration does NOT affect inventory - it only affects the agreement layer
// - This operation is idempotent - calling on already-expired session is safe
func (s *NegotiationService) ExpireSession(
	ctx context.Context,
	sessionID uuid.UUID,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		// Step 1: Lock session for update
		session, err := s.negotiationRepo.GetSessionForUpdate(ctx, tx, sessionID)
		if err != nil {
			return fmt.Errorf("failed to lock session: %w", err)
		}

		// Step 2: Transition to expired (idempotent - safe if already expired)
		if err := session.Expire(); err != nil {
			// If already in a terminal state, that's fine for the worker
			if session.Status.IsTerminal() {
				s.log.Debug("Negotiation already in terminal state",
					zap.String("session_id", sessionID.String()),
					zap.String("status", string(session.Status)),
				)
				return nil
			}
			return err
		}

		// Step 3: Persist changes
		if err := s.negotiationRepo.UpdateSession(ctx, tx, session); err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}

		// Step 4: Emit negotiation.expired event (NON-FINANCIAL)
		// chat_room_id included so mobile notification deeplinks to negotiation chat.
		payload := fmt.Sprintf(`{
			"session_id":"%s",
			"chat_room_id":"%s",
			"buyer_id":"%s"
		}`,
			session.ID,
			negotiationChatRoomIDStr(session.ChatRoomID),
			session.BuyerID,
		)

		if err := s.outboxRepo.InsertEvent(
			ctx, tx,
			"negotiation.expired",
			session.ID,
			[]byte(payload),
		); err != nil {
			return fmt.Errorf("failed to insert outbox event: %w", err)
		}

		s.log.Info("Negotiation expired",
			zap.String("session_id", sessionID.String()),
		)

		return nil
	})
}

// GetExpiredSessions retrieves active negotiations that have passed their expires_at time.
// Used by the NegotiationExpireWorker to find sessions to expire.
// Uses FOR UPDATE SKIP LOCKED for concurrent worker support.
func (s *NegotiationService) GetExpiredSessions(
	ctx context.Context,
	limit int,
) ([]*negotiationEntity.NegotiationSession, error) {
	var sessions []*negotiationEntity.NegotiationSession
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		sessions, err = s.negotiationRepo.GetExpiredSessions(ctx, tx, limit)
		return err
	})

	return sessions, err
}

// GetSession retrieves a negotiation session without locking.
func (s *NegotiationService) GetSession(
	ctx context.Context,
	sessionID uuid.UUID,
) (*negotiationEntity.NegotiationSession, error) {
	var session *negotiationEntity.NegotiationSession
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		session, err = s.negotiationRepo.GetSession(ctx, tx, sessionID)
		return err
	})

	return session, err
}

// ListNegotiations retrieves negotiations for a user with cursor pagination.
// TODO: Implement proper query logic with cursor pagination.
func (s *NegotiationService) ListNegotiations(
	ctx context.Context,
	userID uuid.UUID,
	cursor string,
	limit int,
) ([]*negotiationEntity.NegotiationSession, string, error) {
	// Stub: returns empty list for now
	// This method is not part of the fixed-price-sale contract hardening scope
	return []*negotiationEntity.NegotiationSession{}, "", nil
}

// NOTE: ListMessages has been removed as part of Negotiation → Chat unification.
// Messages should now be retrieved via ChatService.ListMessages using the chat_room_id
// associated with the negotiation session.

// validateForSaleAndGetSeller validates that a fixed-price sale exists and returns its seller ID.
//
// CONTRACT ENFORCEMENT:
// - Fixed-price sale exists and is active
// - Fixed-price sale has negotiation_enabled = true
//
// NOTE: offer and collection domains have been removed.
// Negotiation now only supports fixed-price-sale resource type.
func (s *NegotiationService) validateForSaleAndGetSeller(
	ctx context.Context,
	tx db.Tx,
	resourceType negotiationEntity.NegotiationResourceType,
	forSaleID uuid.UUID,
) (uuid.UUID, error) {
	// Only fixed-price-sale resource type is supported
	if resourceType != negotiationEntity.NegotiationResourceForSale {
		return uuid.Nil, &ErrResourceTypeNotImplemented{
			ResourceType: resourceType,
		}
	}

	// Fetch fixed-price sale for validation
	forSale, err := s.forSaleRepo.GetByID(ctx, tx, forSaleID)
	if err != nil {
		return uuid.Nil, &ErrResourceNotFound{
			ResourceType:     resourceType,
			ForSaleID: forSaleID,
		}
	}

	// Guard: Listing must be active
	if forSale.Status != forSaleEntity.ForSaleStatusActive {
		return uuid.Nil, &ErrResourceNotNegotiable{
			ResourceType:     resourceType,
			ForSaleID: forSaleID,
			Reason:           fmt.Sprintf("fixed-price sale status is %s, not active", forSale.Status),
		}
	}

	// CONTRACT ENFORCEMENT: Listing must have negotiation enabled
	if !forSale.NegotiationEnabled {
		return uuid.Nil, &ErrResourceNotNegotiable{
			ResourceType:     resourceType,
			ForSaleID: forSaleID,
			Reason:           "negotiation is disabled for this fixed-price sale",
		}
	}

	return forSale.SellerID, nil
}

// negotiationChatRoomIDStr returns the string form of a nullable chat room UUID.
// Returns empty string when nil, so payload JSON fields are safe to embed directly.
func negotiationChatRoomIDStr(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// ============================================================================
// ERROR TYPES
// ============================================================================

// ErrCannotNegotiateWithSelf is returned when buyer tries to negotiate with themselves.
type ErrCannotNegotiateWithSelf struct {
	BuyerID  uuid.UUID
	SellerID uuid.UUID
}

func (e *ErrCannotNegotiateWithSelf) Error() string {
	return fmt.Sprintf("cannot negotiate with yourself: buyer_id=%s, seller_id=%s",
		e.BuyerID, e.SellerID)
}

// ErrNegotiationRoomMismatch is returned when the chat room supplied to
// StartNegotiation does not have the resolved seller as its other
// participant (PASS_7B / F2) — e.g. an unrelated direct room, a support
// room, or a room shared with a third party who is not the seller.
type ErrNegotiationRoomMismatch struct {
	BuyerID  uuid.UUID
	SellerID uuid.UUID
	RoomID   uuid.UUID
}

func (e *ErrNegotiationRoomMismatch) Error() string {
	return fmt.Sprintf("chat room %s counterparty is not the seller: buyer_id=%s, seller_id=%s",
		e.RoomID, e.BuyerID, e.SellerID)
}

// ErrActiveSessionExists is returned when an active session already exists.
type ErrActiveSessionExists struct {
	SessionID        uuid.UUID
	ForSaleID uuid.UUID
	BuyerID          uuid.UUID
}

func (e *ErrActiveSessionExists) Error() string {
	return fmt.Sprintf("active negotiation session already exists: session_id=%s, for_sale_id=%s, buyer_id=%s",
		e.SessionID, e.ForSaleID, e.BuyerID)
}

// ErrResourceNotFound is returned when the resource to negotiate doesn't exist.
type ErrResourceNotFound struct {
	ResourceType     negotiationEntity.NegotiationResourceType
	ForSaleID uuid.UUID
}

func (e *ErrResourceNotFound) Error() string {
	return fmt.Sprintf("fixed-price sale not found: resource_type=%s, for_sale_id=%s",
		e.ResourceType, e.ForSaleID)
}

// ErrResourceNotNegotiable is returned when the resource exists but cannot be negotiated.
type ErrResourceNotNegotiable struct {
	ResourceType     negotiationEntity.NegotiationResourceType
	ForSaleID uuid.UUID
	Reason           string
}

func (e *ErrResourceNotNegotiable) Error() string {
	return fmt.Sprintf("fixed-price sale not negotiable: resource_type=%s, for_sale_id=%s, reason=%s",
		e.ResourceType, e.ForSaleID, e.Reason)
}

// ErrResourceTypeNotImplemented is returned for resource types not yet implemented.
type ErrResourceTypeNotImplemented struct {
	ResourceType negotiationEntity.NegotiationResourceType
}

func (e *ErrResourceTypeNotImplemented) Error() string {
	return fmt.Sprintf("resource type not yet implemented: resource_type=%s",
		e.ResourceType)
}

// ErrInvalidResourceType is returned for invalid resource types.
type ErrInvalidResourceType struct {
	ResourceType negotiationEntity.NegotiationResourceType
}

func (e *ErrInvalidResourceType) Error() string {
	return fmt.Sprintf("invalid resource type: resource_type=%s",
		e.ResourceType)
}

// ErrNegotiationExpired is returned when attempting to operate on an expired negotiation.
// NEGOTIATION LIFECYCLE: Expired negotiations cannot proceed to order creation.
// This does NOT affect inventory - expiration only affects the agreement layer.
type ErrNegotiationExpired struct {
	SessionID uuid.UUID
	ExpiresAt *time.Time
}

func (e *ErrNegotiationExpired) Error() string {
	if e.ExpiresAt == nil {
		return fmt.Sprintf("negotiation expired: session_id=%s", e.SessionID)
	}
	return fmt.Sprintf("negotiation expired: session_id=%s, expired_at=%s",
		e.SessionID, e.ExpiresAt)
}

// ErrNegotiationBlockedByRelationship is returned when a block exists between buyer and seller.
// A new negotiation cannot be started if either party has blocked the other.
// Existing sessions (created before the block) continue unaffected until terminal state.
type ErrNegotiationBlockedByRelationship struct {
	BuyerID  uuid.UUID
	SellerID uuid.UUID
}

func (e *ErrNegotiationBlockedByRelationship) Error() string {
	return fmt.Sprintf("cannot start negotiation: block relationship exists between buyer %s and seller %s",
		e.BuyerID, e.SellerID)
}
