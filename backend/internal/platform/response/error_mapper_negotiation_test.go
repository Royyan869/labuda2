package response

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	negotiationApp "github.com/labuda/backend/internal/commerce/negotiation/application"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
)

// TestMapErrorToResponse_NegotiationErrors is the PASS_8A / F3 regression
// suite: every negotiation business rejection must map to a stable 4xx
// status/code, never the generic 500 default.
func TestMapErrorToResponse_NegotiationErrors(t *testing.T) {
	sessionID := uuid.New()
	forSaleID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	roomID := uuid.New()

	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "room mismatch",
			err:        &negotiationApp.ErrNegotiationRoomMismatch{BuyerID: buyerID, SellerID: sellerID, RoomID: roomID},
			wantStatus: http.StatusForbidden,
			wantCode:   "NEGOTIATION_ROOM_MISMATCH",
		},
		{
			name:       "self negotiation",
			err:        &negotiationApp.ErrCannotNegotiateWithSelf{BuyerID: buyerID, SellerID: buyerID},
			wantStatus: http.StatusBadRequest,
			wantCode:   "CANNOT_NEGOTIATE_WITH_SELF",
		},
		{
			name:       "blocked relationship",
			err:        &negotiationApp.ErrNegotiationBlockedByRelationship{BuyerID: buyerID, SellerID: sellerID},
			wantStatus: http.StatusForbidden,
			wantCode:   "NEGOTIATION_BLOCKED",
		},
		{
			name:       "active session exists",
			err:        &negotiationApp.ErrActiveSessionExists{SessionID: sessionID, ForSaleID: forSaleID, BuyerID: buyerID},
			wantStatus: http.StatusConflict,
			wantCode:   "NEGOTIATION_ACTIVE_SESSION_EXISTS",
		},
		{
			name:       "resource not found",
			err:        &negotiationApp.ErrResourceNotFound{ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID},
			wantStatus: http.StatusNotFound,
			wantCode:   "NEGOTIATION_RESOURCE_NOT_FOUND",
		},
		{
			name:       "resource not negotiable",
			err:        &negotiationApp.ErrResourceNotNegotiable{ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, Reason: "negotiation disabled"},
			wantStatus: http.StatusConflict,
			wantCode:   "NEGOTIATION_RESOURCE_NOT_NEGOTIABLE",
		},
		{
			name:       "resource type not implemented",
			err:        &negotiationApp.ErrResourceTypeNotImplemented{ResourceType: negotiationEntity.NegotiationResourceType("auction")},
			wantStatus: http.StatusBadRequest,
			wantCode:   "NEGOTIATION_RESOURCE_TYPE_UNSUPPORTED",
		},
		{
			name:       "negotiation expired",
			err:        &negotiationApp.ErrNegotiationExpired{SessionID: sessionID},
			wantStatus: http.StatusConflict,
			wantCode:   "NEGOTIATION_EXPIRED",
		},
		{
			name:       "unauthorized participant",
			err:        &negotiationEntity.UnauthorizedParticipantError{SessionID: sessionID, UserID: buyerID},
			wantStatus: http.StatusForbidden,
			wantCode:   "NEGOTIATION_UNAUTHORIZED_PARTICIPANT",
		},
		{
			name:       "not buyer",
			err:        &negotiationEntity.NotBuyerError{SessionID: sessionID, UserID: sellerID},
			wantStatus: http.StatusForbidden,
			wantCode:   "NEGOTIATION_BUYER_ONLY",
		},
		{
			name:       "not seller",
			err:        &negotiationEntity.NotSellerError{SessionID: sessionID, UserID: buyerID},
			wantStatus: http.StatusForbidden,
			wantCode:   "NEGOTIATION_SELLER_ONLY",
		},
		{
			name:       "session not active",
			err:        &negotiationEntity.SessionNotActiveError{SessionID: sessionID, CurrentStatus: negotiationEntity.NegotiationStatusCancelled},
			wantStatus: http.StatusConflict,
			wantCode:   "NEGOTIATION_INVALID_STATE",
		},
		{
			name:       "session already terminal",
			err:        &negotiationEntity.SessionAlreadyTerminalError{SessionID: sessionID, CurrentStatus: negotiationEntity.NegotiationStatusExpired},
			wantStatus: http.StatusConflict,
			wantCode:   "NEGOTIATION_INVALID_STATE",
		},
		{
			name:       "invalid transition",
			err:        &negotiationEntity.InvalidTransitionError{SessionID: sessionID, CurrentStatus: negotiationEntity.NegotiationStatusCancelled, TargetStatus: negotiationEntity.NegotiationStatusAccepted},
			wantStatus: http.StatusConflict,
			wantCode:   "NEGOTIATION_INVALID_STATE",
		},
		{
			name:       "invalid price",
			err:        &negotiationEntity.InvalidPriceError{SessionID: sessionID, Price: -1, Reason: "must be > 0"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "NEGOTIATION_INVALID_PRICE",
		},
		{
			name:       "no price",
			err:        &negotiationEntity.NoPriceError{SessionID: sessionID},
			wantStatus: http.StatusBadRequest,
			wantCode:   "NEGOTIATION_INVALID_PRICE",
		},
		{
			name:       "stale proposal",
			err:        &negotiationEntity.StaleProposalError{SessionID: sessionID, ExpectedSequence: 2, ActualSequence: 3},
			wantStatus: http.StatusConflict,
			wantCode:   "NEGOTIATION_STALE_PROPOSAL",
		},
		{
			name:       "chat room not set",
			err:        &negotiationEntity.ErrChatRoomNotSet{SessionID: sessionID},
			wantStatus: http.StatusConflict,
			wantCode:   "NEGOTIATION_CHAT_ROOM_NOT_SET",
		},
		{
			name:       "multiple accepted negotiations",
			err:        &negotiationEntity.ErrMultipleAcceptedNegotiations{BuyerID: buyerID, ForSaleID: forSaleID, ExistingID: sessionID, NewID: uuid.New()},
			wantStatus: http.StatusConflict,
			wantCode:   "NEGOTIATION_MULTIPLE_ACCEPTED",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mapping := MapErrorToResponse(tc.err)
			if mapping.StatusCode != tc.wantStatus {
				t.Errorf("StatusCode = %d, want %d", mapping.StatusCode, tc.wantStatus)
			}
			if mapping.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", mapping.Code, tc.wantCode)
			}
			if mapping.StatusCode == http.StatusInternalServerError {
				t.Errorf("%s: fell through to generic 500 — F3 regression", tc.name)
			}
		})
	}
}
