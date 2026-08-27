package http

import (
	"strings"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	identityusername "github.com/labuda/backend/internal/identity/username"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/mediaresolve"
)

// searchCommerceSellerProjection is the Search-local seller/user projection
// used by for_sale and auction search responses.
//
// It keeps the user-identity axis and seller-business axis separate while
// preserving the flat compatibility fields that the existing Search clients
// consume.
type searchCommerceSellerProjection struct {
	Author   publiccard.UserCard
	Seller   publiccard.SellerCard
	FarmName string
}

func buildSearchCommerceSellerProjection(
	sellerID uuid.UUID,
	username string,
	avatarURL string,
	farmName string,
	accountStatus string,
	isDeleted bool,
	subscriptionStatus string,
) (searchCommerceSellerProjection, bool) {
	if sellerID == uuid.Nil {
		return searchCommerceSellerProjection{}, false
	}

	normalizedUsername := identityusername.Normalize(username)
	normalizedFarmName := strings.TrimSpace(farmName)
	normalizedAvatar := normalizeSearchCommerceSellerAvatar(avatarURL)

	userLifecycle := string(viewercontext.PublicLifecycleStateUnavailable)
	sellerLifecycle := string(viewercontext.CoarsenSellerTrust(subscriptionStatus))

	if normalizedUsername != "" && identityusername.ValidateFormat(normalizedUsername) == nil {
		coarsenedLifecycle := viewercontext.CoarsenLifecycle(accountStatus, isDeleted)
		if coarsenedLifecycle == viewercontext.PublicLifecycleStateActive {
			userLifecycle = string(coarsenedLifecycle)
			user := publiccard.UserCard{
				ID:        sellerID,
				Username:  normalizedUsername,
				AvatarURL: normalizedAvatar,
				Lifecycle: lifecyclePtr(userLifecycle),
			}
			seller := publiccard.NewSellerCard(user, normalizedFarmName)
			if normalizedFarmName == "" {
				unavailable := string(viewercontext.PublicLifecycleStateUnavailable)
				seller.Lifecycle = &unavailable
			} else {
				seller.Lifecycle = lifecyclePtr(sellerLifecycle)
			}
			return searchCommerceSellerProjection{
				Author:   user,
				Seller:   seller,
				FarmName: normalizedFarmName,
			}, true
		}
	}

	unavailable := string(viewercontext.PublicLifecycleStateUnavailable)
	user := publiccard.UserCard{
		ID:        sellerID,
		Username:  "",
		Lifecycle: &unavailable,
	}
	seller := publiccard.NewSellerCard(user, normalizedFarmName)
	if normalizedFarmName == "" {
		seller.Lifecycle = &unavailable
	} else {
		seller.Lifecycle = lifecyclePtr(sellerLifecycle)
	}

	return searchCommerceSellerProjection{
		Author:   user,
		Seller:   seller,
		FarmName: normalizedFarmName,
	}, true
}

func normalizeSearchCommerceSellerAvatar(avatarURL string) *string {
	trimmed := strings.TrimSpace(avatarURL)
	if trimmed == "" {
		return nil
	}
	if resolved, err := mediaresolve.ResolveMediaReadURL(trimmed); err == nil {
		return &resolved
	}
	return &trimmed
}

func lifecyclePtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
