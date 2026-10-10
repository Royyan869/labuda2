package application

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	userEntity "github.com/hishumi/backend/internal/identity/user/domain/entity"
)

// The self session snapshot (GET /users/me) must carry seller store identity.
//
// Store identity has exactly one authority — `seller_profiles` — and both
// projections (self and public) must expose it under the same wire name. If the
// self snapshot omits it, the mobile session snapshot has no store name to
// render and every surface that pairs the store name with the username has to
// wait for a second, separately-timed fetch.
func TestSelfSnapshotCarriesStoreIdentity(t *testing.T) {
	svc, userID := newSellerStateFixtureProfileService(nil)

	state, err := svc.getSellerState(context.Background(), &fakeTx{}, userID)
	if err != nil {
		t.Fatalf("getSellerState: %v", err)
	}
	if state.StoreName == nil || *state.StoreName != "Fresh Store" {
		t.Fatalf("seller state store name = %v, want Fresh Store", state.StoreName)
	}

	selfDTO := svc.entityToUserDTO(
		&userEntity.User{
			ID:            userID,
			AccountStatus: "active",
		},
		[]string{"user"},
		state,
	)

	if selfDTO.StoreName == nil || *selfDTO.StoreName != "Fresh Store" {
		t.Fatalf("self snapshot store name = %v, want Fresh Store", selfDTO.StoreName)
	}

	// The contract the mobile parsers read is the wire name, not the Go field.
	raw, err := json.Marshal(selfDTO)
	if err != nil {
		t.Fatalf("marshal self snapshot: %v", err)
	}
	if !strings.Contains(string(raw), `"store_name":"Fresh Store"`) {
		t.Fatalf("self snapshot wire lacks store_name: %s", raw)
	}
}
