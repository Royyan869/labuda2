package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	supportEntity "github.com/labuda/backend/internal/governance/support/entity"
	"github.com/stretchr/testify/assert"
)

func TestTicketToResponse_EmitsCanonicalIdentityFields(t *testing.T) {
	now := time.Now()
	username := "yayan"
	farmName := "Farm Koi Nusantara"

	ticket := &supportEntity.Ticket{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		Username:       username,
		SellerFarmName: farmName,
		ChatRoomID:     uuid.New(),
		Category:       supportEntity.CategoryPayment,
		Priority:       supportEntity.PriorityMedium,
		Status:         supportEntity.StatusOpen,
		Escalation:     supportEntity.EscalationNone,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	resp := ticketToResponse(ticket)

	assert.Equal(t, username, resp["username"])
	assert.Equal(t, farmName, resp["seller_farm_name"])
}


