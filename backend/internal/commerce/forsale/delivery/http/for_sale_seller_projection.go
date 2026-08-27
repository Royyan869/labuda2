package http

import (
	"strings"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	identityusername "github.com/labuda/backend/internal/identity/username"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/mediaresolve"
)

// forSaleSellerProjection is the fixed-price-local seller identity
// projection.
//
// It keeps the user-identity axis and seller-business axis separate while
// avoiding any synthetic username fallback.
type forSaleSellerProjection struct {
	Author   publiccard.UserCard
	Seller   publiccard.SellerCard
	FarmName string
}

func buildForSaleSellerProjection(
	sellerID uuid.UUID,
	username string,
	avatarURL string,
	farmName string,
	accountStatus string,
	isDeleted bool,
	subscriptionStatus string,
	tier string,
) (forSaleSellerProjection, bool) {
	if sellerID == uuid.Nil {
		return forSaleSellerProjection{}, false
	}

	normalizedUsername := identityusername.Normalize(username)
	normalizedFarmName := strings.TrimSpace(farmName)
	normalizedAvatar := normalizeForSaleSellerAvatar(avatarURL)

	userLifecycle := string(viewercontext.PublicLifecycleStateUnavailable)
	sellerLifecycle := string(viewercontext.CoarsenSellerTrust(subscriptionStatus))

	if normalizedUsername != "" && identityusername.ValidateFormat(normalizedUsername) == nil {
		coarsenedLifecycle := viewercontext.CoarsenLifecycle(accountStatus, isDeleted)
		if coarsenedLifecycle == viewercontext.PublicLifecycleStateActive {
			userLifecycle = string(coarsenedLifecycle)

			user := publiccard.UserCard{
				ID:       sellerID,
				Username: normalizedUsername,
			}
			if normalizedAvatar != nil {
				user.AvatarURL = normalizedAvatar
			}
			user.Lifecycle = lifecyclePtr(userLifecycle)

			seller := publiccard.SellerCard{
				User: user,
			}
			if normalizedAvatar != nil {
				seller.AvatarURL = normalizedAvatar
			}
			if normalizedFarmName != "" {
				seller.FarmName = &normalizedFarmName
			}
			seller.Lifecycle = lifecyclePtr(sellerLifecycle)
			seller.Tier = publiccard.GatedSellerTier(tier, userLifecycle, sellerLifecycle)

			return forSaleSellerProjection{
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
	seller := publiccard.SellerCard{
		User: user,
	}
	if normalizedFarmName != "" {
		seller.FarmName = &normalizedFarmName
		seller.Lifecycle = lifecyclePtr(sellerLifecycle)
	} else {
		seller.Lifecycle = &unavailable
	}
	seller.Tier = publiccard.GatedSellerTier(tier, unavailable, sellerLifecycle)

	return forSaleSellerProjection{
		Author:   user,
		Seller:   seller,
		FarmName: normalizedFarmName,
	}, true
}

func normalizeForSaleSellerAvatar(avatarURL string) *string {
	trimmed := strings.TrimSpace(avatarURL)
	if trimmed == "" {
		return nil
	}
	if resolved, err := mediaresolve.ResolveMediaReadURL(trimmed); err == nil {
		return &resolved
	}
	return &trimmed
}

func lifecyclePtr(lifecycle string) *string {
	if lifecycle == "" {
		return nil
	}
	v := lifecycle
	return &v
}
