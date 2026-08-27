package http

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	withdrawrepo "github.com/labuda/backend/internal/finance/infrastructure/repository"
)

func TestWithdrawalToDetail_MapsSellerIdentity(t *testing.T) {
	w := &withdrawrepo.Withdrawal{
		ID:             uuid.New(),
		SellerID:       uuid.New(),
		SellerUsername: "yayan",
		SellerFarmName: "Farm Koi Nusantara",
		Amount:         125000,
		Status:         withdrawrepo.WithdrawalStatusRequested,
		CreatedAt:      time.Unix(1710000000, 0).Unix(),
		UpdatedAt:      time.Unix(1710003600, 0).Unix(),
	}

	detail := (&AdminPayoutHandler{}).withdrawalToDetail(w)

	if detail.SellerUsername == nil || *detail.SellerUsername != "yayan" {
		t.Fatalf("seller_username = %v, want yayan", detail.SellerUsername)
	}
	if detail.SellerFarmName == nil || *detail.SellerFarmName != "Farm Koi Nusantara" {
		t.Fatalf("seller_farm_name = %v, want Farm Koi Nusantara", detail.SellerFarmName)
	}

	wire, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal detail: %v", err)
	}
	if !strings.Contains(string(wire), `"seller_username":"yayan"`) {
		t.Fatalf("json payload missing seller_username: %s", string(wire))
	}
	if !strings.Contains(string(wire), `"seller_farm_name":"Farm Koi Nusantara"`) {
		t.Fatalf("json payload missing seller_farm_name: %s", string(wire))
	}
}


