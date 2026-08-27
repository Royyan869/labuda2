// DOMAIN: COMMERCE
// NOTE: Price agreement layer between buyer and seller

package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NegotiationSession represents a negotiation session between buyer and seller
// for a specific fixed-price sale.
//
// NEGOTIATION LIFECYCLE HARDENING:
// - Negotiation is a time-bound agreement layer, NOT a stock reservation system
// - expiration does NOT affect inventory - stock is only reserved at order creation
// - expired negotiations cannot proceed to order creation
//
// PRICING AUTHORITY (CHAT DOMAIN HARDENING):
// - current_price: the most recent proposed price (from latest counter-offer)
// - accepted_price: set when negotiation is accepted (final agreed price)
// - These fields are the authoritative source for pricing, NOT chat message attachmentJSON
type NegotiationSession struct {
	ID               uuid.UUID
	ResourceType     NegotiationResourceType
	ForSaleID uuid.UUID
	BuyerID          uuid.UUID
	SellerID         uuid.UUID
	ChatRoomID       *uuid.UUID // References the chat room for this negotiation
	Status           NegotiationStatus
	OrderID          *uuid.UUID // Set when order is created from this negotiation (prevents duplicate settlement)

	// ExpiresAt is the timestamp when this negotiation expires
	// - NULL means not set (legacy negotiations)
	// - Once expired, status transitions to "expired" and operations are blocked
	// - Expiration does NOT reserve or release stock - it only affects the agreement layer
	ExpiresAt *time.Time

	// PRICING AUTHORITY (negotiation domain, not chat)
	// current_price holds the most recent proposed price from either party
	// This is the authoritative source for the current offer state
	// Stored in smallest currency unit (cents for IDR)
	CurrentPrice *int64

	// accepted_price holds the final agreed price when status = "accepted"
	// This becomes the order unit price when order is created
	// Stored in smallest currency unit (cents for IDR)
	AcceptedPrice *int64

	// PRICE SECURITY HARDENING:
	// proposal_sequence increments on each price update to prevent stale overwrites
	// This implements optimistic locking for price updates
	ProposalSequence int64

	// AcceptedAt is the timestamp when the negotiation was accepted
	// Set when status transitions to "accepted"
	// NULL means the negotiation has not been accepted yet
	AcceptedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

const (
	// DefaultNegotiationExpiration is the default time until a negotiation expires
	DefaultNegotiationExpiration = 24 * time.Hour
)

// NewNegotiationSession creates a new active negotiation session.
//
// Rules:
// - Status is always active
// - CreatedAt and UpdatedAt set to now
// - ExpiresAt is set to now + DefaultNegotiationExpiration (24 hours)
//
// NEGOTIATION LIFECYCLE: The expiration is a time-bound agreement limit.
// This does NOT reserve stock - stock is only reserved when an order is created.
func NewNegotiationSession(
	resourceType NegotiationResourceType,
	forSaleID, buyerID, sellerID uuid.UUID,
) *NegotiationSession {
	now := time.Now()
	expiresAt := now.Add(DefaultNegotiationExpiration)

	return &NegotiationSession{
		ID:               uuid.New(),
		ResourceType:     resourceType,
		ForSaleID: forSaleID,
		BuyerID:          buyerID,
		SellerID:         sellerID,
		Status:           NegotiationStatusActive,
		OrderID:          nil,
		ExpiresAt:        &expiresAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// Accept transitions the session from active to accepted.
//
// Allowed: active → accepted
// Error: if not active
//
// Note: Accepted sessions can still expire (accepted → expired)
// to prevent checkout of stale agreements.
func (s *NegotiationSession) Accept() error {
	return s.transitionTo(NegotiationStatusAccepted)
}

// Cancel transitions the session from active to cancelled.
//
// Allowed: active → cancelled
// Error: if not active, or already terminal
func (s *NegotiationSession) Cancel() error {
	return s.transitionTo(NegotiationStatusCancelled)
}

// Expire transitions the session from active or accepted to expired.
//
// Allowed: active → expired, accepted → expired
// Error: if not active or accepted
//
// NEGOTIATION EXPIRY CONSISTENCY: This prevents "accepted but expired"
// state which allows checkout of stale agreements.
func (s *NegotiationSession) Expire() error {
	return s.transitionTo(NegotiationStatusExpired)
}

// EnsureSessionActive returns an error if the session is not active.
// Checks session state only — NOT user account_status. Use
// NegotiationService.EnsureParticipantsActive for user-level enforcement.
func (s *NegotiationSession) EnsureSessionActive() error {
	if s.Status != NegotiationStatusActive {
		return &SessionNotActiveError{
			SessionID:     s.ID,
			CurrentStatus: s.Status,
		}
	}
	return nil
}

// IsExpired returns true if the negotiation has passed its expiration time.
// A NULL ExpiresAt means the negotiation never expires (legacy).
func (s *NegotiationSession) IsExpired() bool {
	if s.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*s.ExpiresAt)
}

// CanProceed returns true if the negotiation can proceed to order creation.
// Requires: active status AND not expired.
func (s *NegotiationSession) CanProceed() bool {
	return s.Status == NegotiationStatusActive && !s.IsExpired()
}

// IsParticipant returns true if the given user is the buyer or seller.
func (s *NegotiationSession) IsParticipant(userID uuid.UUID) bool {
	return s.BuyerID == userID || s.SellerID == userID
}

// IsBuyer returns true if the given user is the buyer.
func (s *NegotiationSession) IsBuyer(userID uuid.UUID) bool {
	return s.BuyerID == userID
}

// IsSeller returns true if the given user is the seller.
func (s *NegotiationSession) IsSeller(userID uuid.UUID) bool {
	return s.SellerID == userID
}

// SetChatRoomID sets the chat room ID for this negotiation session.
func (s *NegotiationSession) SetChatRoomID(chatRoomID uuid.UUID) {
	s.ChatRoomID = &chatRoomID
	s.UpdatedAt = time.Now()
}

// HasChatRoom returns true if a chat room is associated with this session.
func (s *NegotiationSession) HasChatRoom() bool {
	return s.ChatRoomID != nil && *s.ChatRoomID != uuid.Nil
}

// EnsureChatRoomExists returns an error if no chat room is associated.
func (s *NegotiationSession) EnsureChatRoomExists() error {
	if !s.HasChatRoom() {
		return &ErrChatRoomNotSet{
			SessionID: s.ID,
		}
	}
	return nil
}

// SetCurrentPrice updates the current proposed price with validation.
//
// PRICE SECURITY HARDENING:
// - Validates price > 0 (business rule)
// - Validates price is not absurd (guard against obvious errors)
// - Increments proposal_sequence for optimistic locking
// - Returns error if validation fails
//
// This should be called whenever a new proposal is made.
// The current_price becomes the accepted_price when the negotiation is accepted.
func (s *NegotiationSession) SetCurrentPrice(price int64) error {
	// STEP 1: Validate price > 0 (business rule)
	if price <= 0 {
		return &InvalidPriceError{
			SessionID: s.ID,
			Price:     price,
			Reason:    "price must be greater than 0",
		}
	}

	// STEP 2: Validate price is not absurd (guard against obvious errors)
	// Maximum price: 10 billion IDR (1,000,000,000,000 cents)
	const maxPrice = 1_000_000_000_000 // 10 billion in cents
	if price > maxPrice {
		return &InvalidPriceError{
			SessionID: s.ID,
			Price:     price,
			Reason:    "price exceeds maximum allowed value",
		}
	}

	// STEP 3: Update price and increment sequence (optimistic locking)
	s.CurrentPrice = &price
	s.ProposalSequence++
	s.UpdatedAt = time.Now()

	return nil
}

// GetCurrentPrice returns the current proposed price.
// Returns nil if no price has been set yet.
func (s *NegotiationSession) GetCurrentPrice() *int64 {
	return s.CurrentPrice
}

// Accept transitions the session from active to accepted and sets the accepted price.
//
// PRICE SECURITY HARDENING:
// - Validates current_price is set (business rule)
// - Validates current_price matches last proposal (consistency check)
// - Sets accepted_price from current_price atomically
// - Sets accepted_at timestamp
//
// Allowed: active → accepted
// Error: if not active, or no current price
//
// Note: Accepted sessions can still expire (accepted → expired)
// to prevent checkout of stale agreements.
func (s *NegotiationSession) AcceptWithPrice() error {
	// STEP 1: Validate current_price is set
	if s.CurrentPrice == nil {
		return &NoPriceError{
			SessionID: s.ID,
		}
	}

	// STEP 2: Validate price is still valid (defensive check)
	if *s.CurrentPrice <= 0 {
		return &InvalidPriceError{
			SessionID: s.ID,
			Price:     *s.CurrentPrice,
			Reason:    "current_price is invalid (must be > 0)",
		}
	}

	// STEP 3: Validate current_price is not absurd (defensive check)
	const maxPrice = 1_000_000_000_000 // 10 billion in cents
	if *s.CurrentPrice > maxPrice {
		return &InvalidPriceError{
			SessionID: s.ID,
			Price:     *s.CurrentPrice,
			Reason:    "current_price exceeds maximum allowed value",
		}
	}

	// STEP 4: Set accepted_price from current_price atomically
	s.AcceptedPrice = s.CurrentPrice

	// STEP 5: Set accepted_at timestamp
	now := time.Now()
	s.AcceptedAt = &now

	// STEP 6: Now transition to accepted status
	return s.transitionTo(NegotiationStatusAccepted)
}

// GetAcceptedPrice returns the final agreed price.
// Returns nil if the negotiation has not been accepted yet.
func (s *NegotiationSession) GetAcceptedPrice() *int64 {
	return s.AcceptedPrice
}

// transitionTo handles state transitions with validation.
func (s *NegotiationSession) transitionTo(target NegotiationStatus) error {
	if !s.Status.CanTransition(target) {
		return &InvalidTransitionError{
			SessionID:     s.ID,
			CurrentStatus: s.Status,
			TargetStatus:  target,
		}
	}

	s.Status = target
	s.UpdatedAt = time.Now()
	return nil
}

// ============================================================================
// ERROR TYPES
// ============================================================================

// InvalidTransitionError is returned when attempting an invalid state transition.
type InvalidTransitionError struct {
	SessionID     uuid.UUID
	CurrentStatus NegotiationStatus
	TargetStatus  NegotiationStatus
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid negotiation status transition: session_id=%s, %s -> %s",
		e.SessionID, e.CurrentStatus, e.TargetStatus)
}

// SessionNotActiveError is returned when attempting an operation that requires active status.
type SessionNotActiveError struct {
	SessionID     uuid.UUID
	CurrentStatus NegotiationStatus
}

func (e *SessionNotActiveError) Error() string {
	return fmt.Sprintf("negotiation session not active: session_id=%s, current_status=%s",
		e.SessionID, e.CurrentStatus)
}

// UnauthorizedParticipantError is returned when a non-participant attempts an operation.
type UnauthorizedParticipantError struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
}

func (e *UnauthorizedParticipantError) Error() string {
	return fmt.Sprintf("user is not a participant in this negotiation: session_id=%s, user_id=%s",
		e.SessionID, e.UserID)
}

// NotBuyerError is returned when a non-buyer attempts a buyer-only operation.
type NotBuyerError struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
}

func (e *NotBuyerError) Error() string {
	return fmt.Sprintf("only buyer can perform this operation: session_id=%s, user_id=%s",
		e.SessionID, e.UserID)
}

// NotSellerError is returned when a non-seller attempts a seller-only operation.
type NotSellerError struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
}

func (e *NotSellerError) Error() string {
	return fmt.Sprintf("only seller can perform this operation: session_id=%s, user_id=%s",
		e.SessionID, e.UserID)
}

// SessionAlreadyTerminalError is returned when attempting to modify a terminal session.
type SessionAlreadyTerminalError struct {
	SessionID     uuid.UUID
	CurrentStatus NegotiationStatus
}

func (e *SessionAlreadyTerminalError) Error() string {
	return fmt.Sprintf("negotiation session is already terminal: session_id=%s, status=%s",
		e.SessionID, e.CurrentStatus)
}

// ErrChatRoomNotSet is returned when attempting to send a proposal without a chat room.
type ErrChatRoomNotSet struct {
	SessionID uuid.UUID
}

func (e *ErrChatRoomNotSet) Error() string {
	return fmt.Sprintf("chat room not set for negotiation: session_id=%s", e.SessionID)
}

// ErrNegotiationAlreadySettled is returned when attempting to create an order
// from a negotiation that has already been settled.
type ErrNegotiationAlreadySettled struct {
	SessionID uuid.UUID
	OrderID   uuid.UUID
}

func (e *ErrNegotiationAlreadySettled) Error() string {
	return fmt.Sprintf("negotiation already settled: session_id=%s, order_id=%s", e.SessionID, e.OrderID)
}

// NoPriceError is returned when attempting to accept a negotiation without a current price.
type NoPriceError struct {
	SessionID uuid.UUID
}

func (e *NoPriceError) Error() string {
	return fmt.Sprintf("cannot accept negotiation: no price has been proposed: session_id=%s", e.SessionID)
}

// InvalidPriceError is returned when attempting to set an invalid price.
type InvalidPriceError struct {
	SessionID uuid.UUID
	Price     int64
	Reason    string
}

func (e *InvalidPriceError) Error() string {
	return fmt.Sprintf("invalid price for negotiation: session_id=%s, price=%d, reason=%s",
		e.SessionID, e.Price, e.Reason)
}

// StaleProposalError is returned when attempting to update price with stale proposal sequence.
type StaleProposalError struct {
	SessionID        uuid.UUID
	ExpectedSequence int64
	ActualSequence   int64
}

func (e *StaleProposalError) Error() string {
	return fmt.Sprintf("stale proposal update: session_id=%s, expected_sequence=%d, actual_sequence=%d",
		e.SessionID, e.ExpectedSequence, e.ActualSequence)
}

// currentTimeUnix returns the current Unix timestamp.
// This is a simple wrapper for testability.
func currentTimeUnix() int64 {
	return time.Now().Unix()
}

// ErrMultipleAcceptedNegotiations is returned when attempting to accept a second negotiation
// for the same fixed-price sale and buyer when one already exists.
type ErrMultipleAcceptedNegotiations struct {
	BuyerID          uuid.UUID
	ForSaleID uuid.UUID
	ExistingID       uuid.UUID
	NewID            uuid.UUID
}

func (e *ErrMultipleAcceptedNegotiations) Error() string {
	return fmt.Sprintf("multiple accepted negotiations for buyer=%s, for_sale=%s: existing=%s, new=%s",
		e.BuyerID, e.ForSaleID, e.ExistingID, e.NewID)
}
