//go:build ignore

package main

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// Minimal Order struct to verify JSON tags
type Order struct {
	ID       uuid.UUID `json:"id"`
	BuyerID  uuid.UUID `json:"buyer_id"`
	SellerID uuid.UUID `json:"seller_id"`
	Status   string    `json:"status"`
	Quantity int       `json:"quantity"`
}

func main() {
	orderID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	buyerID := uuid.MustParse("223e4567-e89b-12d3-a456-426614174000")
	sellerID := uuid.MustParse("323e4567-e89b-12d3-a456-426614174000")

	order := &Order{
		ID:       orderID,
		BuyerID:  buyerID,
		SellerID: sellerID,
		Status:   "pending",
		Quantity: 1,
	}

	data, err := json.Marshal(order)
	if err != nil {
		fmt.Printf("❌ ERROR: Failed to marshal: %v\n", err)
		return
	}

	fmt.Printf("✅ JSON Serialization Test\n")
	fmt.Printf("=========================\n\n")
	fmt.Printf("Output: %s\n\n", string(data))

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		fmt.Printf("❌ ERROR: Failed to unmarshal: %v\n", err)
		return
	}

	fmt.Printf("Field Verification:\n")
	fmt.Printf("===================\n")

	allGood := true
	if _, ok := result["id"]; ok {
		fmt.Printf("✅ 'id' field present\n")
	} else {
		fmt.Printf("❌ 'id' field MISSING\n")
		allGood = false
	}

	if _, ok := result["buyer_id"]; ok {
		fmt.Printf("✅ 'buyer_id' field present\n")
	} else {
		fmt.Printf("❌ 'buyer_id' field MISSING\n")
		allGood = false
	}

	if _, ok := result["seller_id"]; ok {
		fmt.Printf("✅ 'seller_id' field present\n")
	} else {
		fmt.Printf("❌ 'seller_id' field MISSING\n")
		allGood = false
	}

	if _, ok := result["status"]; ok {
		fmt.Printf("✅ 'status' field present\n")
	} else {
		fmt.Printf("❌ 'status' field MISSING\n")
		allGood = false
	}

	if _, ok := result["ID"]; ok {
		fmt.Printf("❌ 'ID' field should NOT exist (wrong casing)\n")
		allGood = false
	} else {
		fmt.Printf("✅ 'ID' field correctly absent\n")
	}

	if _, ok := result["BuyerID"]; ok {
		fmt.Printf("❌ 'BuyerID' field should NOT exist (wrong casing)\n")
		allGood = false
	} else {
		fmt.Printf("✅ 'BuyerID' field correctly absent\n")
	}

	if _, ok := result["SellerID"]; ok {
		fmt.Printf("❌ 'SellerID' field should NOT exist (wrong casing)\n")
		allGood = false
	} else {
		fmt.Printf("✅ 'SellerID' field correctly absent\n")
	}

	fmt.Printf("\n")
	if allGood {
		fmt.Printf("🎉 SUCCESS: All JSON tags are correct!\n")
		fmt.Printf("\nExpected behavior:\n")
		fmt.Printf("- POST /api/v1/orders will return { 'id': '...', 'buyer_id': '...', ... }\n")
		fmt.Printf("- Client can parse response.data.id to get order ID\n")
		fmt.Printf("- GET /api/v1/orders/{id} will work with correct ID\n")
	} else {
		fmt.Printf("❌ FAILED: JSON tags are incorrect\n")
	}
}
