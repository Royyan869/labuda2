package shared

// ViewerCapabilities is the canonical commerce detail authority payload.
//
// It is intentionally viewer-scoped: the same detail response can surface a
// different role and action set depending on the authenticated viewer.
type ViewerCapabilities struct {
	Role         string `json:"role"`
	CanManage    bool   `json:"can_manage"`
	CanEdit      bool   `json:"can_edit"`
	CanPromote   bool   `json:"can_promote"`
	CanChat      bool   `json:"can_chat"`
	CanNegotiate bool   `json:"can_negotiate"`
	CanBuy       bool   `json:"can_buy"`
	CanBid       bool   `json:"can_bid"`
	CanBuyNow    bool   `json:"can_buy_now"`
}
