package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/admin/repository"
)

// TestWarningCountsMapping verifies that userDetailsFromRepo correctly
// propagates warning count fields from the repository struct to the
// handler DTO. This regression-locks the three warning visibility fields.
func TestWarningCountsMapping(t *testing.T) {
	cases := []struct {
		name               string
		warningCount       int
		activeWarningCount int
		severeWarningCount int
	}{
		{
			name:               "no warnings",
			warningCount:       0,
			activeWarningCount: 0,
			severeWarningCount: 0,
		},
		{
			name:               "active warnings, none severe",
			warningCount:       3,
			activeWarningCount: 2,
			severeWarningCount: 0,
		},
		{
			name:               "active warnings with severe subset",
			warningCount:       5,
			activeWarningCount: 3,
			severeWarningCount: 1,
		},
		{
			name:               "all active all severe",
			warningCount:       2,
			activeWarningCount: 2,
			severeWarningCount: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoDetails := repository.UserDetails{
				ID:                 uuid.New(),
				FirebaseUID:        "uid",
				Email:              "test@example.com",
				AccountStatus:      "active",
				Role:               "user",
				WarningCount:       tc.warningCount,
				ActiveWarningCount: tc.activeWarningCount,
				SevereWarningCount: tc.severeWarningCount,
			}

			dto := userDetailsFromRepo(repoDetails, nil)

			if dto.WarningCount != tc.warningCount {
				t.Errorf("WarningCount: want %d, got %d", tc.warningCount, dto.WarningCount)
			}
			if dto.ActiveWarningCount != tc.activeWarningCount {
				t.Errorf("ActiveWarningCount: want %d, got %d", tc.activeWarningCount, dto.ActiveWarningCount)
			}
			if dto.SevereWarningCount != tc.severeWarningCount {
				t.Errorf("SevereWarningCount: want %d, got %d", tc.severeWarningCount, dto.SevereWarningCount)
			}

			// Invariant: severe is a subset of active
			if dto.SevereWarningCount > dto.ActiveWarningCount {
				t.Errorf("severe (%d) must not exceed active (%d)", dto.SevereWarningCount, dto.ActiveWarningCount)
			}
		})
	}
}


