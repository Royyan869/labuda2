package application

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/platform/capability"
	"github.com/labuda/backend/internal/governance/verification/entity"
	"github.com/labuda/backend/internal/governance/verification/infrastructure/repository"
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// ---------------------------------------------------------------------------
// OUTBOX ATOMICITY — emitEventTx failure must roll back the transaction
// ---------------------------------------------------------------------------
//
// Regression guard for Batch 5 fix: verification lifecycle events are
// STRICT_EVENT_ATOMIC. If InsertEvent fails, the entire transaction fails
// and the status flip does not commit.

type verificationMockDB struct {
	txFactory func() db.Tx
}

func (m *verificationMockDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(m.txFactory())
}

// verificationOutboxFailTx is a mock tx that succeeds for all SQL except
// INSERT INTO outbox, which returns a simulated failure.
type verificationOutboxFailTx struct {
	outboxInsertCalled bool
	updateCalled       bool
}

func (t *verificationOutboxFailTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	// GetForUpdate — return a pending verification
	return &verificationMockRow{}
}

func (t *verificationOutboxFailTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (t *verificationOutboxFailTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "INSERT INTO outbox") {
		t.outboxInsertCalled = true
		return pgconn.CommandTag{}, fmt.Errorf("simulated outbox write failure")
	}
	if strings.Contains(sql, "UPDATE") || strings.Contains(sql, "INSERT") {
		t.updateCalled = true
	}
	return pgconn.NewCommandTag("1"), nil
}

func (t *verificationOutboxFailTx) Commit(ctx context.Context) error   { return nil }
func (t *verificationOutboxFailTx) Rollback(ctx context.Context) error { return nil }

// verificationMockRow returns a pending-review SellerVerification with
// the 9-column scan shape used by SellerVerificationRepository.GetForUpdate.
type verificationMockRow struct{}

func (r *verificationMockRow) Scan(dest ...any) error {
	// 9 columns: id, seller_id, status, submitted_at, reviewed_at, reviewed_by, reason, created_at, updated_at
	for i, d := range dest {
		switch i {
		case 0: // id
			*d.(*uuid.UUID) = uuid.New()
		case 1: // seller_id
			*d.(*uuid.UUID) = uuid.New()
		case 2: // status
			*d.(*entity.Status) = entity.StatusPendingReview
		}
		// Remaining fields stay zero-valued (nil pointers / zero times)
	}
	return nil
}

type verificationMockAuditLogger struct{}

var _ audit.AdminAuditLogger = (*verificationMockAuditLogger)(nil)

func (m *verificationMockAuditLogger) Log(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	return nil
}

func (m *verificationMockAuditLogger) LogSafe(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) {
}

func (m *verificationMockAuditLogger) LogTx(ctx context.Context, tx db.Tx, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	return nil
}

type verificationAllowAllAuthorizer struct{}

func (verificationAllowAllAuthorizer) HasCapability(_ context.Context, _ uuid.UUID, _ capability.Capability) (bool, error) {
	return true, nil
}

// ---------------------------------------------------------------------------
// DOCUMENT-LEVEL OUTBOX ATOMICITY — RejectDocument
// ---------------------------------------------------------------------------

// documentOutboxFailTx intercepts the document GetByID QueryRow and fails on
// INSERT INTO outbox. The scan shape matches VerificationDocumentRepository.GetByID
// (12 columns: migration 000206 dropped document_url).
type documentOutboxFailTx struct {
	outboxInsertCalled bool
}

func (t *documentOutboxFailTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &documentMockRow{userID: uuid.New()}
}

func (t *documentOutboxFailTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (t *documentOutboxFailTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "INSERT INTO outbox") {
		t.outboxInsertCalled = true
		return pgconn.CommandTag{}, fmt.Errorf("simulated outbox write failure")
	}
	return pgconn.NewCommandTag("1"), nil
}

func (t *documentOutboxFailTx) Commit(ctx context.Context) error   { return nil }
func (t *documentOutboxFailTx) Rollback(ctx context.Context) error { return nil }

// documentMockRow returns a pending VerificationDocument with the 12-column
// scan shape used by VerificationDocumentRepository.GetByID (migration 000206
// dropped document_url; storage_key at position 3).
type documentMockRow struct {
	userID uuid.UUID
}

func (r *documentMockRow) Scan(dest ...any) error {
	// 12 scan destinations matching GetByID:
	// 0:  doc.ID            (*uuid.UUID)
	// 1:  doc.UserID        (*uuid.UUID)
	// 2:  doc.DocumentType  (*entity.DocumentType)
	// 3:  storageKey        (**string)  — nullable TEXT
	// 4:  doc.DocumentName  (*string)
	// 5:  doc.Status        (*entity.ReviewStatus)
	// 6:  rejectionNote     (**string)
	// 7:  doc.SubmittedAt   (*time.Time)
	// 8:  reviewedAt        (**time.Time)
	// 9:  reviewedBy        (**uuid.UUID)
	// 10: doc.CreatedAt     (*time.Time)
	// 11: doc.UpdatedAt     (*time.Time)
	storageKeyVal := "s3/test_ktp.jpg"
	for i, d := range dest {
		switch i {
		case 0: // doc.ID
			*d.(*uuid.UUID) = uuid.New()
		case 1: // doc.UserID
			*d.(*uuid.UUID) = r.userID
		case 2: // doc.DocumentType
			*d.(*entity.DocumentType) = entity.DocumentTypeIdentityKTP
		case 3: // storageKey — **string
			*d.(**string) = &storageKeyVal
		case 4: // doc.DocumentName
			*d.(*string) = "KTP_Test"
		case 5: // doc.Status
			*d.(*entity.ReviewStatus) = entity.ReviewStatusPending
		}
		// Remaining fields (rejectionNote, SubmittedAt, reviewedAt, reviewedBy,
		// CreatedAt, UpdatedAt) stay zero-valued — safe for Reject() path.
	}
	return nil
}

func TestRejectDocument_OutboxFailure_RollsBackTransaction(t *testing.T) {
	spy := &documentOutboxFailTx{}

	svc := NewVerificationDocumentService(
		&verificationMockDB{txFactory: func() db.Tx { return spy }},
		repository.NewVerificationDocumentRepository(),
		repository.NewSellerVerificationRepository(),
		outboxrepo.NewOutboxRepository(nil),
	)

	err := svc.RejectDocument(context.Background(), uuid.New(), uuid.New(), "document unclear")

	if err == nil {
		t.Fatal("RejectDocument must return error when outbox insert fails (STRICT_EVENT_ATOMIC)")
	}
	if !spy.outboxInsertCalled {
		t.Fatal("outbox insert was never attempted — test wiring error")
	}
	if !strings.Contains(err.Error(), "outbox verification.document.rejected") {
		t.Errorf("error should reference outbox event type, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// LIFECYCLE-LEVEL OUTBOX ATOMICITY — ApproveVerification (Batch 5 regression)
// ---------------------------------------------------------------------------

func TestApproveVerification_OutboxFailure_RollsBackTransaction(t *testing.T) {
	spy := &verificationOutboxFailTx{}

	svc := NewVerificationService(
		&verificationMockDB{txFactory: func() db.Tx { return spy }},
		repository.NewSellerVerificationRepository(),
		&verificationMockAuditLogger{},
		outboxrepo.NewOutboxRepository(nil),
		verificationAllowAllAuthorizer{},
	)

	err := svc.ApproveVerification(context.Background(), uuid.New(), uuid.New())

	if err == nil {
		t.Fatal("ApproveVerification must return error when outbox insert fails (STRICT_EVENT_ATOMIC)")
	}
	if !spy.outboxInsertCalled {
		t.Fatal("outbox insert was never attempted — test wiring error")
	}
	if !strings.Contains(err.Error(), "outbox seller.verification.approved") {
		t.Errorf("error should reference outbox event type, got: %v", err)
	}
}


