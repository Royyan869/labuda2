package consumer

import (
	"testing"

	"github.com/google/uuid"
	chatvalidator "github.com/labuda/backend/internal/interaction/chat/attachmentvalidator"
)

func TestBuildNegotiationProposalFromStarted_Canonical(t *testing.T) {
	p := &NegotiationStartedPayload{
		SessionID:    uuid.New(),
		ResourceType: "for_sale",
		ResourceID:   uuid.New(),
		InitialPrice: 123456,
		Note:         "hello",
	}

	att := buildNegotiationProposalFromStarted(p)
	assertCanonicalNegotiationProposal(t, att)

	data := att["data"].(map[string]interface{})
	if _, ok := data["session_id"]; !ok {
		t.Fatal("expected data.session_id")
	}
	if _, ok := data["resource_type"]; !ok {
		t.Fatal("expected data.resource_type")
	}
	if _, ok := data["resource_id"]; !ok {
		t.Fatal("expected data.resource_id")
	}
	if _, ok := data["price"]; !ok {
		t.Fatal("expected data.price")
	}
	if _, ok := data["proposal_sequence"]; !ok {
		t.Fatal("expected data.proposal_sequence")
	}
}

func TestBuildNegotiationProposalFromMessageSent_Canonical(t *testing.T) {
	p := &NegotiationMessageSentPayload{
		SessionID:        uuid.New(),
		Price:            99999,
		ProposalSequence: 3,
	}

	att := buildNegotiationProposalFromMessageSent(p)
	assertCanonicalNegotiationProposal(t, att)

	if _, ok := att["session_id"]; ok {
		t.Fatal("flat root key session_id must not exist")
	}
	if _, ok := att["price"]; ok {
		t.Fatal("flat root key price must not exist")
	}
	if _, ok := att["proposal_sequence"]; ok {
		t.Fatal("flat root key proposal_sequence must not exist")
	}
}

func TestValidateCanonicalAttachmentJSON_NegotiationProposal(t *testing.T) {
	att := buildNegotiationProposalFromMessageSent(&NegotiationMessageSentPayload{
		SessionID:        uuid.New(),
		Price:            1000,
		ProposalSequence: 2,
	})
	if err := validateCanonicalAttachmentJSON(att); err != nil {
		t.Fatalf("expected canonical attachment to pass guard, got error: %v", err)
	}
}

func assertCanonicalNegotiationProposal(t *testing.T, att map[string]interface{}) {
	t.Helper()
	if got, ok := att["type"].(string); !ok || got != "negotiation_proposal" {
		t.Fatalf("expected type negotiation_proposal, got %#v", att["type"])
	}
	if _, ok := att["data"].(map[string]interface{}); !ok {
		t.Fatalf("expected data object, got %#v", att["data"])
	}
	if errs := chatvalidator.ValidateAttachmentJSON(att); chatvalidator.HasValidationErrors(errs) {
		t.Fatalf("expected canonical attachment to pass validator, got: %+v", errs)
	}
}
