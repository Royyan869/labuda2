package models

import "testing"

func TestPaymentWebhookEventDB_MarkAsManualReview(t *testing.T) {
	e := &PaymentWebhookEventDB{}
	e.MarkAsManualReview("unknown status")

	if e.Status != "manual_review" {
		t.Fatalf("expected manual_review, got %s", e.Status)
	}
	if e.ProcessedAt == nil {
		t.Fatal("expected processed_at to be set")
	}
}

func TestPaymentWebhookEventDB_MarkAsQuarantined(t *testing.T) {
	e := &PaymentWebhookEventDB{}
	e.MarkAsQuarantined("malformed payload")

	if e.Status != "quarantined" {
		t.Fatalf("expected quarantined, got %s", e.Status)
	}
	if e.ProcessedAt == nil {
		t.Fatal("expected processed_at to be set")
	}
}

func TestPaymentWebhookEventDB_MarkAsTerminalReview(t *testing.T) {
	e := &PaymentWebhookEventDB{}
	e.MarkAsTerminalReview("max retries")

	if e.Status != "terminal_review" {
		t.Fatalf("expected terminal_review, got %s", e.Status)
	}
	if e.ProcessedAt == nil {
		t.Fatal("expected processed_at to be set")
	}
}


