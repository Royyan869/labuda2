package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	chatattachmentvalidator "github.com/labuda/backend/internal/interaction/chat/attachmentvalidator"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
)

type panicTransactor struct{}

func (p *panicTransactor) WithTx(context.Context, func(tx db.Tx) error) error {
	panic("transaction must not start for invalid attachment_json")
}

func TestSendMessage_RejectsLegacyReferenceAttachmentBeforeTx(t *testing.T) {
	svc := &Service{
		db:          &panicTransactor{},
		rateLimiter: rate.NewRateLimiter(),
	}

	body := "hello"
	_, err := svc.SendMessage(
		context.Background(),
		uuid.New(),
		uuid.New(),
		chatEntity.MessageTypeText,
		&body,
		map[string]interface{}{
			"type": "reference",
			"data": map[string]interface{}{
				"target_type": "for_sale",
				"target_id":   uuid.New().String(),
				"preview": map[string]interface{}{
					"title": "Legacy for_sale",
				},
			},
		},
		nil,
		nil,
		uuid.NewString(),
		nil,
	)

	if err != chatRepo.ErrInvalidAttachment {
		t.Fatalf("expected ErrInvalidAttachment, got %v", err)
	}
}

func TestValidateAttachmentJSON_RejectsLegacyReferenceAtServiceBoundary(t *testing.T) {
	errs := chatattachmentvalidator.ValidateAttachmentJSON(map[string]interface{}{
		"type": "reference",
		"data": map[string]interface{}{
			"target_type": "auction",
			"target_id":   uuid.New().String(),
			"preview": map[string]interface{}{
				"title": "Legacy auction",
			},
		},
	})
	if !chatattachmentvalidator.HasValidationErrors(errs) {
		t.Fatal("expected validation errors for legacy reference attachment")
	}
}
