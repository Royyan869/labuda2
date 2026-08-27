package sellerdisplay

import (
	"strings"
)

// Identity is the canonical seller identity projection used by detail
// commerce responses.
type Identity struct {
	StoreName        string `json:"store_name"`
	StoreImageURL    string `json:"store_image_url"`
	Username         string `json:"username"`
	AvatarURL        string `json:"avatar_url"`
	PublicOriginLine string `json:"public_origin_line,omitempty"`
}

// FromInfo builds the canonical seller identity projection from sellerdisplay
// row data.
func FromInfo(info Info) Identity {
	return Identity{
		StoreName:        clean(info.FarmName),
		StoreImageURL:    clean(info.StoreImageURL),
		Username:         clean(info.Username),
		AvatarURL:        clean(info.AvatarURL),
		PublicOriginLine: clean(info.PublicOriginLine),
	}
}

// ProjectionMap builds the canonical seller_identity wire shape.
func ProjectionMap(info Info, resolveURL func(string) string) map[string]interface{} {
	identity := FromInfo(info)
	result := map[string]interface{}{
		"store_name":      identity.StoreName,
		"store_image_url": resolveURL(identity.StoreImageURL),
		"username":        identity.Username,
		"avatar_url":      resolveURL(identity.AvatarURL),
	}
	if identity.PublicOriginLine != "" {
		result["public_origin_line"] = identity.PublicOriginLine
	}
	return result
}

func clean(value string) string {
	return strings.TrimSpace(value)
}
