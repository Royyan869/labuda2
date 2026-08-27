package application

// Storage-key provenance tests for SubmitKYCDocuments (section B of the
// KYC final security audit).
//
// The provenance check validates that the caller-supplied storage_key strings
// have the expected prefix `kyc/{userID}/{docType}/` before opening a
// transaction. A noopTransactor that panics if WithTx is called proves the
// check fires before any DB access.

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/verification/infrastructure/repository"
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// neverCalledTransactor fails the test if WithTx is ever reached.
// Provenance validation must short-circuit before opening a transaction.
type neverCalledTransactor struct {
	t *testing.T
}

func (n *neverCalledTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	n.t.Helper()
	n.t.Fatal("WithTx must not be called when the storage key fails provenance validation")
	return nil
}

func newProvenanceSvc(t *testing.T) *VerificationDocumentService {
	t.Helper()
	return NewVerificationDocumentService(
		&neverCalledTransactor{t: t},
		repository.NewVerificationDocumentRepository(),
		repository.NewSellerVerificationRepository(),
		outboxrepo.NewOutboxRepository(nil),
	)
}

// TestSubmitKYCDocuments_KTPKey_WrongUser_Rejected verifies that a ktp_storage_key
// containing a different user's UUID is rejected before DB access.
func TestSubmitKYCDocuments_KTPKey_WrongUser_Rejected(t *testing.T) {
	svc := newProvenanceSvc(t)
	userID := uuid.New()
	otherID := uuid.New()

	err := svc.SubmitKYCDocuments(
		context.Background(),
		userID,
		"Test Name",
		"1234567890",
		"kyc/"+otherID.String()+"/identity_ktp/ktp.jpg", // wrong user
		"kyc/"+userID.String()+"/identity_selfie/selfie.jpg",
	)
	if err == nil {
		t.Fatal("expected error for wrong-user ktp_storage_key, got nil")
	}
	if !strings.Contains(err.Error(), "ktp_storage_key") {
		t.Errorf("error should mention ktp_storage_key, got: %v", err)
	}
}

// TestSubmitKYCDocuments_SelfieKey_WrongUser_Rejected verifies that a
// selfie_storage_key containing a different user's UUID is rejected.
func TestSubmitKYCDocuments_SelfieKey_WrongUser_Rejected(t *testing.T) {
	svc := newProvenanceSvc(t)
	userID := uuid.New()
	otherID := uuid.New()

	err := svc.SubmitKYCDocuments(
		context.Background(),
		userID,
		"Test Name",
		"1234567890",
		"kyc/"+userID.String()+"/identity_ktp/ktp.jpg",
		"kyc/"+otherID.String()+"/identity_selfie/selfie.jpg", // wrong user
	)
	if err == nil {
		t.Fatal("expected error for wrong-user selfie_storage_key, got nil")
	}
	if !strings.Contains(err.Error(), "selfie_storage_key") {
		t.Errorf("error should mention selfie_storage_key, got: %v", err)
	}
}

// TestSubmitKYCDocuments_KTPKeyInSelfieSlot_Rejected verifies that passing a
// ktp-typed key in the selfie position is rejected (cross-doctype swap).
func TestSubmitKYCDocuments_KTPKeyInSelfieSlot_Rejected(t *testing.T) {
	svc := newProvenanceSvc(t)
	userID := uuid.New()

	err := svc.SubmitKYCDocuments(
		context.Background(),
		userID,
		"Test Name",
		"1234567890",
		"kyc/"+userID.String()+"/identity_ktp/ktp.jpg",
		"kyc/"+userID.String()+"/identity_ktp/ktp.jpg", // ktp key reused in selfie slot
	)
	if err == nil {
		t.Fatal("expected error for ktp key in selfie slot, got nil")
	}
	if !strings.Contains(err.Error(), "selfie_storage_key") {
		t.Errorf("error should mention selfie_storage_key, got: %v", err)
	}
}

// TestSubmitKYCDocuments_NonKYCNamespace_Rejected verifies that keys outside
// the kyc/ namespace (e.g. from the general media upload path) are rejected.
func TestSubmitKYCDocuments_NonKYCNamespace_Rejected(t *testing.T) {
	svc := newProvenanceSvc(t)
	userID := uuid.New()

	err := svc.SubmitKYCDocuments(
		context.Background(),
		userID,
		"Test Name",
		"1234567890",
		"images/evil.jpg", // general media upload namespace — must be blocked
		"kyc/"+userID.String()+"/identity_selfie/selfie.jpg",
	)
	if err == nil {
		t.Fatal("expected error for non-kyc ktp key, got nil")
	}
	if !strings.Contains(err.Error(), "ktp_storage_key") {
		t.Errorf("error should mention ktp_storage_key, got: %v", err)
	}
}


