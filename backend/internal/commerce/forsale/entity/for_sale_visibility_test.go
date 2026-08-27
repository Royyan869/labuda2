package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDeriveVisibility(t *testing.T) {
	publishedAt := time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC)

	require.Equal(t, ForSaleVisibilityPublic, DeriveVisibility(ForSaleStatusActive, &publishedAt))
	require.Equal(t, ForSaleVisibilityPrivate, DeriveVisibility(ForSaleStatusActive, nil))
	require.Equal(t, ForSaleVisibilityPrivate, DeriveVisibility(ForSaleStatusDraft, &publishedAt))
	require.Equal(t, ForSaleVisibilityPrivate, DeriveVisibility(ForSaleStatusSold, &publishedAt))
	require.Equal(t, ForSaleVisibilityPrivate, DeriveVisibility(ForSaleStatusWithdrawn, &publishedAt))
}
