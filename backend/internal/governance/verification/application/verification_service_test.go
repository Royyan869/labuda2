//go:build integration

package application_test

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/verification/application"
	"github.com/labuda/backend/internal/governance/verification/entity"
	"github.com/labuda/backend/internal/platform/capability"
	verificationrepo "github.com/labuda/backend/internal/governance/verification/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// mockTransactor is a mock implementation of Transactor for testing.
type mockTransactor struct {
	executeFn func(ctx context.Context, fn func(tx db.Tx) error) error
}

func (m *mockTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	if m.executeFn != nil {
		return m.executeFn(ctx, fn)
	}
	return fn(&mockTx{})
}

// mockTx is a mock implementation of db.Tx for testing.
type mockTx struct {
	data map[uuid.UUID]*entity.SellerVerification
}

func (m *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *mockTx) Query(ctx context.Context, sql string, args ...interface{}) pgx.Rows {
	return nil
}

func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return &mockRow{}
}

func (m *mockTx) Commit(ctx context.Context) error {
	return nil
}

func (m *mockTx) Rollback(ctx context.Context) error {
	return nil
}

// mockRow is a mock implementation of pgx.Row.
type mockRow struct{}

func (m *mockRow) Scan(dest ...interface{}) error {
	return errors.New("not found")
}

// mockRepository is a mock implementation of SellerVerificationRepository.
type mockRepository struct {
	getBySellerIDFunc  func(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (*entity.SellerVerification, error)
	getForUpdateFunc   func(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (*entity.SellerVerification, error)
	createFunc         func(ctx context.Context, tx db.Tx, v *entity.SellerVerification) error
	updateFunc         func(ctx context.Context, tx db.Tx, v *entity.SellerVerification) error
}

type mockCapabilityAuthorizer struct {
	hasCapabilityFunc func(ctx context.Context, userID uuid.UUID, cap capability.Capability) (bool, error)
}

func (m *mockCapabilityAuthorizer) HasCapability(ctx context.Context, userID uuid.UUID, cap capability.Capability) (bool, error) {
	if m.hasCapabilityFunc != nil {
		return m.hasCapabilityFunc(ctx, userID, cap)
	}
	return true, nil
}

func newVerificationServiceForTest(transactor *mockTransactor, repo *mockRepository) *application.VerificationService {
	return application.NewVerificationService(transactor, repo, nil, nil, &mockCapabilityAuthorizer{})
}

func (m *mockRepository) GetBySellerID(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (*entity.SellerVerification, error) {
	if m.getBySellerIDFunc != nil {
		return m.getBySellerIDFunc(ctx, tx, sellerID)
	}
	return nil, nil
}

func (m *mockRepository) GetForUpdate(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (*entity.SellerVerification, error) {
	if m.getForUpdateFunc != nil {
		return m.getForUpdateFunc(ctx, tx, sellerID)
	}
	return nil, nil
}

func (m *mockRepository) Create(ctx context.Context, tx db.Tx, v *entity.SellerVerification) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, tx, v)
	}
	return nil
}

func (m *mockRepository) Update(ctx context.Context, tx db.Tx, v *entity.SellerVerification) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, tx, v)
	}
	return nil
}

func (m *mockRepository) Upsert(ctx context.Context, tx db.Tx, v *entity.SellerVerification) (bool, error) {
	return true, nil
}

func (m *mockRepository) ListByStatus(ctx context.Context, tx db.Tx, status entity.Status) ([]*entity.SellerVerification, error) {
	return nil, nil
}

func TestVerificationService_IsSellerVerified(t *testing.T) {
	t.Run("returns false when record not found", func(t *testing.T) {
		repo := &mockRepository{
			getBySellerIDFunc: func(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (*entity.SellerVerification, error) {
				return nil, nil // Not found
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		verified, err := service.IsSellerVerified(context.Background(), uuid.New())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if verified {
			t.Error("expected verified to be false when record not found")
		}
	})

	t.Run("returns false when status is unverified", func(t *testing.T) {
		sellerID := uuid.New()
		v := entity.NewSellerVerification(sellerID)
		// v.Status is StatusUnverified by default

		repo := &mockRepository{
			getBySellerIDFunc: func(ctx context.Context, tx db.Tx, sid uuid.UUID) (*entity.SellerVerification, error) {
				return v, nil
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		verified, err := service.IsSellerVerified(context.Background(), sellerID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if verified {
			t.Error("expected verified to be false for unverified seller")
		}
	})

	t.Run("returns false when status is pending", func(t *testing.T) {
		sellerID := uuid.New()
		v := entity.NewSellerVerification(sellerID)
		_ = v.Submit()

		repo := &mockRepository{
			getBySellerIDFunc: func(ctx context.Context, tx db.Tx, sid uuid.UUID) (*entity.SellerVerification, error) {
				return v, nil
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		verified, err := service.IsSellerVerified(context.Background(), sellerID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if verified {
			t.Error("expected verified to be false for pending seller")
		}
	})

	t.Run("returns false when status is rejected", func(t *testing.T) {
		sellerID := uuid.New()
		v := entity.NewSellerVerification(sellerID)
		_ = v.Submit()
		_ = v.Reject(uuid.New(), "invalid")

		repo := &mockRepository{
			getBySellerIDFunc: func(ctx context.Context, tx db.Tx, sid uuid.UUID) (*entity.SellerVerification, error) {
				return v, nil
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		verified, err := service.IsSellerVerified(context.Background(), sellerID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if verified {
			t.Error("expected verified to be false for rejected seller")
		}
	})

	t.Run("returns true when status is verified", func(t *testing.T) {
		sellerID := uuid.New()
		v := entity.NewSellerVerification(sellerID)
		_ = v.Submit()
		_ = v.Approve(uuid.New())

		repo := &mockRepository{
			getBySellerIDFunc: func(ctx context.Context, tx db.Tx, sid uuid.UUID) (*entity.SellerVerification, error) {
				return v, nil
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		verified, err := service.IsSellerVerified(context.Background(), sellerID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !verified {
			t.Error("expected verified to be true for verified seller")
		}
	})
}

func TestVerificationService_SubmitVerification(t *testing.T) {
	t.Run("creates new record when not exists", func(t *testing.T) {
		sellerID := uuid.New()
		var createdV *entity.SellerVerification

		repo := &mockRepository{
			getForUpdateFunc: func(ctx context.Context, tx db.Tx, sid uuid.UUID) (*entity.SellerVerification, error) {
				return nil, nil // Not found
			},
			createFunc: func(ctx context.Context, tx db.Tx, v *entity.SellerVerification) error {
				createdV = v
				return nil
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		err := service.SubmitVerification(context.Background(), sellerID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if createdV == nil {
			t.Fatal("expected record to be created")
		}
		if createdV.Status != entity.StatusPending {
			t.Errorf("expected status %s, got %s", entity.StatusPending, createdV.Status)
		}
		if createdV.SellerID != sellerID {
			t.Errorf("expected SellerID %s, got %s", sellerID, createdV.SellerID)
		}
	})

	t.Run("transitions existing record to pending", func(t *testing.T) {
		sellerID := uuid.New()
		v := entity.NewSellerVerification(sellerID) // StatusUnverified
		var updatedV *entity.SellerVerification

		repo := &mockRepository{
			getForUpdateFunc: func(ctx context.Context, tx db.Tx, sid uuid.UUID) (*entity.SellerVerification, error) {
				return v, nil
			},
			updateFunc: func(ctx context.Context, tx db.Tx, sv *entity.SellerVerification) error {
				updatedV = sv
				return nil
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		err := service.SubmitVerification(context.Background(), sellerID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updatedV == nil {
			t.Fatal("expected record to be updated")
		}
		if updatedV.Status != entity.StatusPending {
			t.Errorf("expected status %s, got %s", entity.StatusPending, updatedV.Status)
		}
	})

	t.Run("resubmits from rejected", func(t *testing.T) {
		sellerID := uuid.New()
		adminID := uuid.New()
		v := entity.NewSellerVerification(sellerID)
		_ = v.Submit()
		_ = v.Reject(adminID, "invalid")
		var updatedV *entity.SellerVerification

		repo := &mockRepository{
			getForUpdateFunc: func(ctx context.Context, tx db.Tx, sid uuid.UUID) (*entity.SellerVerification, error) {
				return v, nil
			},
			updateFunc: func(ctx context.Context, tx db.Tx, sv *entity.SellerVerification) error {
				updatedV = sv
				return nil
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		err := service.SubmitVerification(context.Background(), sellerID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updatedV == nil {
			t.Fatal("expected record to be updated")
		}
		if updatedV.Status != entity.StatusPending {
			t.Errorf("expected status %s, got %s", entity.StatusPending, updatedV.Status)
		}
		// Previous rejection data should be cleared
		if updatedV.RejectionReason != nil {
			t.Error("expected RejectionReason to be cleared on resubmit")
		}
	})
}

func TestVerificationService_ApproveVerification(t *testing.T) {
	t.Run("approves pending verification", func(t *testing.T) {
		sellerID := uuid.New()
		adminID := uuid.New()
		v := entity.NewSellerVerification(sellerID)
		_ = v.Submit()
		var updatedV *entity.SellerVerification

		repo := &mockRepository{
			getForUpdateFunc: func(ctx context.Context, tx db.Tx, sid uuid.UUID) (*entity.SellerVerification, error) {
				return v, nil
			},
			updateFunc: func(ctx context.Context, tx db.Tx, sv *entity.SellerVerification) error {
				updatedV = sv
				return nil
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		err := service.ApproveVerification(context.Background(), sellerID, adminID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updatedV == nil {
			t.Fatal("expected record to be updated")
		}
		if updatedV.Status != entity.StatusVerified {
			t.Errorf("expected status %s, got %s", entity.StatusVerified, updatedV.Status)
		}
		if updatedV.ReviewedBy == nil {
			t.Error("expected ReviewedBy to be set")
		}
		if *updatedV.ReviewedBy != adminID {
			t.Errorf("expected ReviewedBy %s, got %s", adminID, *updatedV.ReviewedBy)
		}
	})

	t.Run("returns error when record not found", func(t *testing.T) {
		sellerID := uuid.New()
		adminID := uuid.New()

		repo := &mockRepository{
			getForUpdateFunc: func(ctx context.Context, tx db.Tx, sid uuid.UUID) (*entity.SellerVerification, error) {
				return nil, nil // Not found
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		err := service.ApproveVerification(context.Background(), sellerID, adminID)
		if err == nil {
			t.Error("expected error when record not found")
		}
	})

	t.Run("returns error when status is not pending", func(t *testing.T) {
		sellerID := uuid.New()
		adminID := uuid.New()
		v := entity.NewSellerVerification(sellerID) // StatusUnverified

		repo := &mockRepository{
			getForUpdateFunc: func(ctx context.Context, tx db.Tx, sid uuid.UUID) (*entity.SellerVerification, error) {
				return v, nil
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		err := service.ApproveVerification(context.Background(), sellerID, adminID)
		if err == nil {
			t.Error("expected error when status is not pending")
		}
	})
}

func TestVerificationService_RejectVerification(t *testing.T) {
	t.Run("rejects pending verification", func(t *testing.T) {
		sellerID := uuid.New()
		adminID := uuid.New()
		reason := "document unclear"
		v := entity.NewSellerVerification(sellerID)
		_ = v.Submit()
		var updatedV *entity.SellerVerification

		repo := &mockRepository{
			getForUpdateFunc: func(ctx context.Context, tx db.Tx, sid uuid.UUID) (*entity.SellerVerification, error) {
				return v, nil
			},
			updateFunc: func(ctx context.Context, tx db.Tx, sv *entity.SellerVerification) error {
				updatedV = sv
				return nil
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		err := service.RejectVerification(context.Background(), sellerID, adminID, reason)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updatedV == nil {
			t.Fatal("expected record to be updated")
		}
		if updatedV.Status != entity.StatusRejected {
			t.Errorf("expected status %s, got %s", entity.StatusRejected, updatedV.Status)
		}
		if updatedV.RejectionReason == nil {
			t.Error("expected RejectionReason to be set")
		}
		if *updatedV.RejectionReason != reason {
			t.Errorf("expected RejectionReason %s, got %s", reason, *updatedV.RejectionReason)
		}
	})

	t.Run("returns error when record not found", func(t *testing.T) {
		sellerID := uuid.New()
		adminID := uuid.New()

		repo := &mockRepository{
			getForUpdateFunc: func(ctx context.Context, tx db.Tx, sid uuid.UUID) (*entity.SellerVerification, error) {
				return nil, nil // Not found
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		err := service.RejectVerification(context.Background(), sellerID, adminID, "test")
		if err == nil {
			t.Error("expected error when record not found")
		}
	})
}

func TestVerificationService_GetStatus(t *testing.T) {
	t.Run("returns unverified when record not found", func(t *testing.T) {
		repo := &mockRepository{
			getBySellerIDFunc: func(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (*entity.SellerVerification, error) {
				return nil, nil // Not found
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		status, err := service.GetStatus(context.Background(), uuid.New())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != entity.StatusUnverified {
			t.Errorf("expected status %s, got %s", entity.StatusUnverified, status)
		}
	})

	t.Run("returns actual status when record exists", func(t *testing.T) {
		sellerID := uuid.New()
		v := entity.NewSellerVerification(sellerID)
		_ = v.Submit()

		repo := &mockRepository{
			getBySellerIDFunc: func(ctx context.Context, tx db.Tx, sid uuid.UUID) (*entity.SellerVerification, error) {
				return v, nil
			},
		}
		transactor := &mockTransactor{}
		service := newVerificationServiceForTest(transactor, repo)

		status, err := service.GetStatus(context.Background(), sellerID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != entity.StatusPending {
			t.Errorf("expected status %s, got %s", entity.StatusPending, status)
		}
	})
}


