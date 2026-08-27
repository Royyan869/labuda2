package http

import "testing"

func TestValidateAttachmentJSON_AcceptsCanonicalTypes(t *testing.T) {
	tests := []struct {
		name string
		att  map[string]interface{}
	}{
		{
			name: "reference",
			att: map[string]interface{}{
					"type": "reference",
					"data": map[string]interface{}{
					"target_type": "for_sale",
					"target_id":   "00000000-0000-0000-0000-000000000001",
					"preview": map[string]interface{}{
						"title": "Item A",
					},
				},
			},
		},
		{
			name: "profile_reference",
			att: map[string]interface{}{
				"type": "reference",
				"data": map[string]interface{}{
					"target_type": "profile",
					"target_id":   "00000000-0000-0000-0000-000000000005",
					"preview": map[string]interface{}{
						"title": "Owner Profile",
					},
				},
			},
		},
		{
			name: "negotiation_offer",
			att: map[string]interface{}{
				"type": "negotiation_offer",
				"data": map[string]interface{}{
					"negotiation_id": "n1",
					"for_sale_id": "fps1",
					"status":         "active",
					"preview": map[string]interface{}{
						"title": "Offer",
					},
				},
			},
		},
		{
			name: "negotiation_proposal",
			att: map[string]interface{}{
				"type": "negotiation_proposal",
				"data": map[string]interface{}{
					"session_id":        "s1",
					"proposal_sequence": float64(1),
					"price":             float64(100000),
				},
			},
		},
		{
			name: "negotiation_result",
			att: map[string]interface{}{
				"type": "negotiation_result",
				"data": map[string]interface{}{
					"negotiation_id": "n1",
					"for_sale_id": "fps1",
					"status":         "accepted",
					"preview": map[string]interface{}{
						"title": "Result",
					},
				},
			},
		},
		{
			name: "shipping_quote",
			att: map[string]interface{}{
				"type": "shipping_quote",
				"data": map[string]interface{}{
					"offer_id":            "o1",
					"linked_item_id":      "i1",
					"linked_item_type":    "for_sale",
					"for_sale_id": "fps1",
					"linked_item_name":    "Fixed-price sale title",
					"linked_item_price":   float64(1000),
					"shipping_type":       "manual",
					"shipping_type_name":  "Ongkir Manual",
					"shipping_type_emoji": "🚚",
					"rate":                float64(25000),
					"status":              "ACTIVE",
					"seller_id":           "seller-1",
				},
			},
		},
		{
			name: "shipping_quote_auction",
			att: map[string]interface{}{
				"type": "shipping_quote",
				"data": map[string]interface{}{
					"offer_id":            "o2",
					"linked_item_id":      "a1",
					"linked_item_type":    "auction",
					"auction_id":          "a1",
					"linked_item_name":    "Auction title",
					"linked_item_price":   float64(1200),
					"shipping_type":       "manual",
					"shipping_type_name":  "Ongkir Manual",
					"shipping_type_emoji": "🚚",
					"rate":                float64(25000),
					"status":              "ACTIVE",
					"seller_id":           "seller-1",
				},
			},
		},
		{
			name: "location",
			att: map[string]interface{}{
				"type": "location",
				"data": map[string]interface{}{
					"latitude":  float64(-6.2),
					"longitude": float64(106.8),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateAttachmentJSON(tt.att)
			if HasValidationErrors(errs) {
				t.Fatalf("expected no validation errors, got: %+v", errs)
			}
		})
	}
}

func TestValidateAttachmentJSON_RejectsLegacyWireTypes(t *testing.T) {
	legacyTypes := []string{"listing", "auction", "post", "request", "content"}
	for _, legacyType := range legacyTypes {
		t.Run(legacyType, func(t *testing.T) {
			errs := ValidateAttachmentJSON(map[string]interface{}{
				"type": legacyType,
				"data": map[string]interface{}{},
			})
			if !HasValidationErrors(errs) {
				t.Fatalf("expected validation errors for legacy type %q", legacyType)
			}
		})
	}
}

func TestValidateAttachmentJSON_RejectsGenericContentReferenceTarget(t *testing.T) {
	errs := ValidateAttachmentJSON(map[string]interface{}{
		"type": "reference",
		"data": map[string]interface{}{
			"target_type": "content",
			"target_id":   "00000000-0000-0000-0000-000000000099",
			"preview": map[string]interface{}{
				"title": "Content",
			},
		},
	})

	if !HasValidationErrors(errs) {
		t.Fatal("expected validation errors for generic content reference target")
	}
}

func TestValidateAttachmentJSON_ReferenceRequiresNestedCanonicalFields(t *testing.T) {
	errs := ValidateAttachmentJSON(map[string]interface{}{
		"type": "reference",
		"data": map[string]interface{}{
			"preview": map[string]interface{}{
				"title": "X",
			},
		},
	})
	if !HasValidationErrors(errs) {
		t.Fatal("expected validation errors for missing target_type/target_id")
	}
}

func TestValidateAttachmentJSON_RejectsLegacyShippingQuoteSellerName(t *testing.T) {
	errs := ValidateAttachmentJSON(map[string]interface{}{
		"type": "shipping_quote",
		"data": map[string]interface{}{
			"offer_id":         "o1",
			"linked_item_id":   "i1",
					"linked_item_type": "for_sale",
			"seller_name":      "Legacy Seller",
		},
	})

	if !HasValidationErrors(errs) {
		t.Fatal("expected validation errors for legacy seller_name on shipping_quote")
	}
}


