package http

// IDOR / KYC-namespace guard tests for AdminVerificationHandler.GetDocumentViewURL
// (section E of the KYC final security audit).
//
// Even when ListDocuments confirms that the document belongs to the queried
// seller, the handler must refuse to generate a presigned GET URL when the
// document's storage_key is outside the kyc/ namespace.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

	verificationApp "github.com/labuda/backend/internal/governance/verification/application"
	verificationEntity "github.com/labuda/backend/internal/governance/verification/entity"
	"github.com/labuda/backend/internal/governance/verification/infrastructure/repository"
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// ─── stub presigner ──────────────────────────────────────────────────────────

type idorStubPresigner struct{}

func (s *idorStubPresigner) PresignPUT(key, contentType string, ttl time.Duration) (string, error) {
	return "https://s3.example.com/put", nil
}

func (s *idorStubPresigner) PresignGET(key string, ttl time.Duration) (string, error) {
	return "https://s3.example.com/get", nil
}

// ─── mock pgx.Rows returning pre-built documents ─────────────────────────────

type idorDocRows struct {
	docs []*verificationEntity.VerificationDocument
	pos  int
}

func (r *idorDocRows) Close()                                     {}
func (r *idorDocRows) Err() error                                 { return nil }
func (r *idorDocRows) CommandTag() pgconn.CommandTag              { return pgconn.CommandTag{} }
func (r *idorDocRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *idorDocRows) Values() ([]any, error)                     { return nil, nil }
func (r *idorDocRows) RawValues() [][]byte                        { return nil }
func (r *idorDocRows) Conn() *pgx.Conn                            { return nil }

func (r *idorDocRows) Next() bool {
	r.pos++
	return r.pos <= len(r.docs)
}

// Scan populates the 12 columns expected by scanRow after migration 000206:
// id, user_id, document_type, storage_key, document_name, status,
// rejection_note, submitted_at, reviewed_at, reviewed_by, created_at, updated_at.
func (r *idorDocRows) Scan(dest ...any) error {
	d := r.docs[r.pos-1]
	sk := d.StorageKey
	*dest[0].(*uuid.UUID) = d.ID
	*dest[1].(*uuid.UUID) = d.UserID
	*dest[2].(*verificationEntity.DocumentType) = d.DocumentType
	*dest[3].(**string) = &sk
	*dest[4].(*string) = d.DocumentName
	*dest[5].(*verificationEntity.ReviewStatus) = d.Status
	// positions 6-11: rejection_note, submitted_at, reviewed_at, reviewed_by,
	// created_at, updated_at — left zero/nil; safe for read path.
	return nil
}

// ─── mock db.Tx returning the given documents on Query ──────────────────────

type idorStubTx struct {
	docs []*verificationEntity.VerificationDocument
}

func (t *idorStubTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &idorDocRows{docs: t.docs}, nil
}

func (t *idorStubTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	// Not called in ListDocuments path.
	return &idorNilRow{}
}

func (t *idorStubTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *idorStubTx) Commit(_ context.Context) error   { return nil }
func (t *idorStubTx) Rollback(_ context.Context) error { return nil }

type idorNilRow struct{}

func (r *idorNilRow) Scan(dest ...any) error { return nil }

// ─── mock db.Transactor ──────────────────────────────────────────────────────

type idorStubDB struct {
	docs []*verificationEntity.VerificationDocument
}

func (d *idorStubDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(&idorStubTx{docs: d.docs})
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestGetDocumentViewURL_NonKYCKey_Returns403 verifies that a document whose
// storage_key is outside the kyc/ namespace triggers the namespace guard even
// when the document legitimately belongs to the queried seller.
func TestGetDocumentViewURL_NonKYCKey_Returns403(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sellerID := uuid.New()
	docID := uuid.New()

	// Document owned by the correct seller but with a non-kyc/ key.
	badDoc := &verificationEntity.VerificationDocument{
		ID:           docID,
		UserID:       sellerID,
		DocumentType: verificationEntity.DocumentTypeIdentityKTP,
		StorageKey:   "images/evil.jpg", // general media namespace — must be blocked
		DocumentName: "evil",
		Status:       verificationEntity.ReviewStatusPending,
	}

	stubDB := &idorStubDB{docs: []*verificationEntity.VerificationDocument{badDoc}}
	docSvc := verificationApp.NewVerificationDocumentService(
		stubDB,
		repository.NewVerificationDocumentRepository(),
		repository.NewSellerVerificationRepository(),
		outboxrepo.NewOutboxRepository(nil),
	)

	handler := NewAdminVerificationHandler(nil, docSvc, zap.NewNop())
	handler.SetPresigner(&idorStubPresigner{})

	r := gin.New()
	r.GET("/seller-verifications/:seller_id/documents/:document_id/view-url",
		handler.GetDocumentViewURL)

	req := httptest.NewRequest(
		http.MethodGet,
		"/seller-verifications/"+sellerID.String()+"/documents/"+docID.String()+"/view-url",
		nil,
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-kyc/ storage key, got %d: %s",
			w.Code, w.Body.String())
	}
}

// TestGetDocumentViewURL_ValidKYCKey_Returns200 verifies that a document with
// a well-formed kyc/ storage key proceeds to presigning and returns 200.
func TestGetDocumentViewURL_ValidKYCKey_Returns200(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sellerID := uuid.New()
	docID := uuid.New()

	goodDoc := &verificationEntity.VerificationDocument{
		ID:           docID,
		UserID:       sellerID,
		DocumentType: verificationEntity.DocumentTypeIdentityKTP,
		StorageKey:   "kyc/" + sellerID.String() + "/identity_ktp/ktp.jpg",
		DocumentName: "KTP_Test",
		Status:       verificationEntity.ReviewStatusPending,
	}

	stubDB := &idorStubDB{docs: []*verificationEntity.VerificationDocument{goodDoc}}
	docSvc := verificationApp.NewVerificationDocumentService(
		stubDB,
		repository.NewVerificationDocumentRepository(),
		repository.NewSellerVerificationRepository(),
		outboxrepo.NewOutboxRepository(nil),
	)

	handler := NewAdminVerificationHandler(nil, docSvc, zap.NewNop())
	handler.SetPresigner(&idorStubPresigner{})

	r := gin.New()
	r.GET("/seller-verifications/:seller_id/documents/:document_id/view-url",
		handler.GetDocumentViewURL)

	req := httptest.NewRequest(
		http.MethodGet,
		"/seller-verifications/"+sellerID.String()+"/documents/"+docID.String()+"/view-url",
		nil,
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK for valid kyc/ storage key, got %d: %s",
			w.Code, w.Body.String())
	}
}


