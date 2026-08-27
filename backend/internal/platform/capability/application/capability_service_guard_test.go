package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
)

type capabilityServiceAuditLogger struct{}

func (capabilityServiceAuditLogger) Log(context.Context, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) error {
	return nil
}

func (capabilityServiceAuditLogger) LogSafe(context.Context, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) {
}

func (capabilityServiceAuditLogger) LogTx(context.Context, db.Tx, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) error {
	return nil
}

type capabilityServiceRepoMock struct {
	actorCaps   map[uuid.UUID]map[string]bool
	activeCaps  map[string]*capabilityEntity.UserCapability
	createCalls int
	revokeCalls int
}

func newCapabilityServiceRepoMock() *capabilityServiceRepoMock {
	return &capabilityServiceRepoMock{
		actorCaps:  make(map[uuid.UUID]map[string]bool),
		activeCaps: make(map[string]*capabilityEntity.UserCapability),
	}
}

func (m *capabilityServiceRepoMock) capKey(userID uuid.UUID, capStr string) string {
	return userID.String() + "|" + capStr
}

func (m *capabilityServiceRepoMock) Create(_ context.Context, _ interface{}, cap *capabilityEntity.UserCapability) error {
	m.createCalls++
	m.activeCaps[m.capKey(cap.UserID, cap.Capability)] = cap
	return nil
}

func (m *capabilityServiceRepoMock) GetByID(_ context.Context, _ interface{}, id uuid.UUID) (*capabilityEntity.UserCapability, error) {
	for _, cap := range m.activeCaps {
		if cap.ID == id {
			return cap, nil
		}
	}
	return nil, nil
}

func (m *capabilityServiceRepoMock) GetActiveCapability(_ context.Context, _ interface{}, userID uuid.UUID, capabilityStr string) (*capabilityEntity.UserCapability, error) {
	if cap, ok := m.activeCaps[m.capKey(userID, capabilityStr)]; ok && cap.RevokedAt == nil {
		return cap, nil
	}
	return nil, nil
}

func (m *capabilityServiceRepoMock) ListActiveCapabilities(_ context.Context, _ interface{}, userID uuid.UUID) ([]*capabilityEntity.UserCapability, error) {
	var result []*capabilityEntity.UserCapability
	for _, cap := range m.activeCaps {
		if cap.UserID == userID && cap.RevokedAt == nil {
			result = append(result, cap)
		}
	}
	return result, nil
}

func (m *capabilityServiceRepoMock) Revoke(_ context.Context, _ interface{}, id uuid.UUID, _ *interface{}) error {
	m.revokeCalls++
	for _, cap := range m.activeCaps {
		if cap.ID == id {
			now := time.Now()
			cap.RevokedAt = &now
			return nil
		}
	}
	return fmt.Errorf("capability not found")
}

func (m *capabilityServiceRepoMock) HasCapability(_ context.Context, _ interface{}, userID uuid.UUID, capabilityStr string) (bool, error) {
	if caps, ok := m.actorCaps[userID]; ok {
		return caps[capabilityStr], nil
	}
	return false, nil
}

func (m *capabilityServiceRepoMock) HasAnyCapability(_ context.Context, _ interface{}, userID uuid.UUID, capabilities []string) (bool, error) {
	for _, capStr := range capabilities {
		if has, _ := m.HasCapability(context.Background(), nil, userID, capStr); has {
			return true, nil
		}
	}
	return false, nil
}

func (m *capabilityServiceRepoMock) CountActiveCapabilities(_ context.Context, _ interface{}, userID uuid.UUID) (int, error) {
	caps, _ := m.ListActiveCapabilities(context.Background(), nil, userID)
	return len(caps), nil
}

func (m *capabilityServiceRepoMock) ListUsersByCapability(_ context.Context, _ interface{}, capabilityStr string) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	for userID, caps := range m.actorCaps {
		if caps[capabilityStr] {
			ids = append(ids, userID)
		}
	}
	return ids, nil
}

var _ audit.AdminAuditLogger = (*capabilityServiceAuditLogger)(nil)

var _ capabilityRepository = (*capabilityServiceRepoMock)(nil)

// capabilityRepository is a local alias so the mock can satisfy the interface
// without importing the repository package into the test body repeatedly.
type capabilityRepository interface {
	Create(ctx context.Context, tx interface{}, cap *capabilityEntity.UserCapability) error
	GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*capabilityEntity.UserCapability, error)
	GetActiveCapability(ctx context.Context, tx interface{}, userID uuid.UUID, capability string) (*capabilityEntity.UserCapability, error)
	ListActiveCapabilities(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*capabilityEntity.UserCapability, error)
	Revoke(ctx context.Context, tx interface{}, id uuid.UUID, revokedAt *interface{}) error
	HasCapability(ctx context.Context, tx interface{}, userID uuid.UUID, capability string) (bool, error)
	HasAnyCapability(ctx context.Context, tx interface{}, userID uuid.UUID, capabilities []string) (bool, error)
	CountActiveCapabilities(ctx context.Context, tx interface{}, userID uuid.UUID) (int, error)
	ListUsersByCapability(ctx context.Context, tx interface{}, capability string) ([]uuid.UUID, error)
}

func TestCapabilityService_RequiresAuthority(t *testing.T) {
	repo := newCapabilityServiceRepoMock()
	svc := NewCapabilityService(repo, capabilityServiceAuditLogger{})

	actorID := uuid.New()
	targetID := uuid.New()

	if err := svc.AssignCapability(context.Background(), targetID, capability.CapGovernanceCapabilityAssign.String(), actorID); err == nil {
		t.Fatal("expected assign to fail without governance.capability.assign")
	}
	if err := svc.RevokeCapability(context.Background(), targetID, capability.CapGovernanceCapabilityAssign.String(), actorID); err == nil {
		t.Fatal("expected revoke to fail without governance.capability.assign")
	}
	if repo.createCalls != 0 || repo.revokeCalls != 0 {
		t.Fatalf("unexpected mutation calls: create=%d revoke=%d", repo.createCalls, repo.revokeCalls)
	}
}

func TestCapabilityService_AllowsAuthorityGrantToOthers(t *testing.T) {
	repo := newCapabilityServiceRepoMock()
	actorID := uuid.New()
	targetID := uuid.New()
	actorCaps := make(map[string]bool)
	actorCaps[capability.CapGovernanceCapabilityAssign.String()] = true
	repo.actorCaps[actorID] = actorCaps

	svc := NewCapabilityService(repo, capabilityServiceAuditLogger{})

	if err := svc.AssignCapability(context.Background(), targetID, capability.CapFinanceWithdrawRead.String(), actorID); err != nil {
		t.Fatalf("assign with authority failed: %v", err)
	}
	if repo.createCalls != 1 {
		t.Fatalf("createCalls=%d want 1", repo.createCalls)
	}

	repo.activeCaps[repo.capKey(targetID, capability.CapFinanceWithdrawRead.String())] = capabilityEntity.NewCapabilityGrant(targetID, capability.CapFinanceWithdrawRead.String(), &actorID)
	if err := svc.RevokeCapability(context.Background(), targetID, capability.CapFinanceWithdrawRead.String(), actorID); err != nil {
		t.Fatalf("revoke with authority failed: %v", err)
	}
	if repo.revokeCalls != 1 {
		t.Fatalf("revokeCalls=%d want 1", repo.revokeCalls)
	}
}

// P5-02: self-escalation preventive guard. An admin with
// governance.capability.assign must not be able to grant a capability to
// themselves — only to other users. See ErrSelfCapabilityGrantForbidden.
func TestCapabilityService_RejectsSelfGrant(t *testing.T) {
	repo := newCapabilityServiceRepoMock()
	actorID := uuid.New()
	actorCaps := make(map[string]bool)
	actorCaps[capability.CapGovernanceCapabilityAssign.String()] = true
	repo.actorCaps[actorID] = actorCaps

	svc := NewCapabilityService(repo, capabilityServiceAuditLogger{})

	err := svc.AssignCapability(context.Background(), actorID, capability.CapFinanceWithdrawReview.String(), actorID)
	if err == nil {
		t.Fatal("expected self-grant to be rejected")
	}
	var selfGrantErr *ErrSelfCapabilityGrantForbidden
	if !errors.As(err, &selfGrantErr) {
		t.Fatalf("expected ErrSelfCapabilityGrantForbidden, got %T: %v", err, err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("self-grant must not persist anything: createCalls=%d want 0", repo.createCalls)
	}
}

// P5-02: the guard must not block a second authorized operator granting the
// same capability to a different admin — only self-grant is forbidden.
func TestCapabilityService_AuthorizedOperatorCanGrantToAnotherAdmin(t *testing.T) {
	repo := newCapabilityServiceRepoMock()
	operatorID := uuid.New()
	otherAdminID := uuid.New()
	operatorCaps := make(map[string]bool)
	operatorCaps[capability.CapGovernanceCapabilityAssign.String()] = true
	repo.actorCaps[operatorID] = operatorCaps

	svc := NewCapabilityService(repo, capabilityServiceAuditLogger{})

	if err := svc.AssignCapability(context.Background(), otherAdminID, capability.CapGovernanceCapabilityAssign.String(), operatorID); err != nil {
		t.Fatalf("authorized operator granting to another admin should succeed: %v", err)
	}
	if repo.createCalls != 1 {
		t.Fatalf("createCalls=%d want 1", repo.createCalls)
	}
}

func TestCapabilityService_ListAllCapabilities_IncludesExternalProductReview(t *testing.T) {
	repo := newCapabilityServiceRepoMock()
	svc := NewCapabilityService(repo, capabilityServiceAuditLogger{})

	definitions := svc.ListAllCapabilities(context.Background())
	var found bool
	for _, def := range definitions {
		if def.Capability == capability.CapPromotionExternalProductReview.String() {
			found = true
			if def.Category != "Promotion" {
				t.Fatalf("category=%q want Promotion", def.Category)
			}
			if def.Description == "" {
				t.Fatal("description should not be empty")
			}
			if !def.Critical {
				t.Fatal("expected promotion external product review to be critical")
			}
			break
		}
	}

	if !found {
		t.Fatal("expected promotion.external_product.review in capability catalog")
	}
}


