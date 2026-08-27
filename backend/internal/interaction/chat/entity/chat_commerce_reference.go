package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CommerceTargetType identifies the live commerce authority behind a chat reference.
type CommerceTargetType string

const (
	CommerceTargetTypeForSale CommerceTargetType = "for_sale"
	CommerceTargetTypeAuction        CommerceTargetType = "auction"
)

// ChatCommerceReferenceSnapshot is the immutable display snapshot stored for
// historical rendering only.
type ChatCommerceReferenceSnapshot struct {
	Title        string  `json:"title"`
	ImageURL     *string `json:"image_url,omitempty"`
	DisplayValue *int64  `json:"display_value,omitempty"`
	SellerID     string  `json:"seller_id,omitempty"`
	SellerName   string  `json:"seller_name,omitempty"`
	IsAvailable  bool    `json:"is_available"`
	IsSold       bool    `json:"is_sold"`
	IsClosed     bool    `json:"is_closed"`
	IsDeleted    bool    `json:"is_deleted"`
}

// ChatCommerceReference is the immutable commerce authority for a chat room.
//
// Identity is canonical on (room_id, target_type, target_id); the row ID is a
// stable surrogate key for easy reference from messages/UI.
type ChatCommerceReference struct {
	ID              uuid.UUID
	RoomID          uuid.UUID
	TargetType      CommerceTargetType
	TargetID        uuid.UUID
	CreatorID       uuid.UUID
	DisplaySnapshot json.RawMessage
	CreatedAt       time.Time
}

// Snapshot decodes the immutable display snapshot.
func (r *ChatCommerceReference) Snapshot() ChatCommerceReferenceSnapshot {
	if r == nil || len(r.DisplaySnapshot) == 0 {
		return ChatCommerceReferenceSnapshot{}
	}
	var snapshot ChatCommerceReferenceSnapshot
	if err := json.Unmarshal(r.DisplaySnapshot, &snapshot); err != nil {
		return ChatCommerceReferenceSnapshot{}
	}
	return snapshot
}
