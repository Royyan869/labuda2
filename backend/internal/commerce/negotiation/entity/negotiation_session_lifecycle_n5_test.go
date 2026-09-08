package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIsSettled(t *testing.T) {
	s := &NegotiationSession{ID: uuid.New(), Status: NegotiationStatusAccepted}
	require.False(t, s.IsSettled())
	oid := uuid.New()
	s.OrderID = &oid
	require.True(t, s.IsSettled())
	nilOID := uuid.Nil
	s.OrderID = &nilOID
	require.False(t, s.IsSettled(), "uuid.Nil should not be considered settled")
}

func TestExpire_SettledAcceptedIsRejected(t *testing.T) {
	s := &NegotiationSession{
		ID:        uuid.New(),
		Status:    NegotiationStatusAccepted,
		ExpiresAt: func() *time.Time { tm := time.Now().Add(-1 * time.Hour); return &tm }(),
	}
	oid := uuid.New()
	s.OrderID = &oid
	err := s.Expire()
	require.Error(t, err)
	require.Contains(t, err.Error(), "already settled")
	require.Equal(t, NegotiationStatusAccepted, s.Status, "status must not transition when settled")
}

func TestExpire_ActiveExpiredTransitions(t *testing.T) {
	exp := time.Now().Add(-1 * time.Hour)
	s := &NegotiationSession{ID: uuid.New(), Status: NegotiationStatusActive, ExpiresAt: &exp}
	require.True(t, s.IsExpired())
	require.NoError(t, s.Expire())
	require.Equal(t, NegotiationStatusExpired, s.Status)
}

func TestExpire_AcceptedUnsettledExpiredTransitions(t *testing.T) {
	exp := time.Now().Add(-1 * time.Hour)
	s := &NegotiationSession{ID: uuid.New(), Status: NegotiationStatusAccepted, ExpiresAt: &exp}
	require.NoError(t, s.Expire())
	require.Equal(t, NegotiationStatusExpired, s.Status)
}

func TestCanTransition_AcceptedToExpiredStillAllowedWhenUnsettled(t *testing.T) {
	require.True(t, NegotiationStatusAccepted.CanTransition(NegotiationStatusExpired))
}
