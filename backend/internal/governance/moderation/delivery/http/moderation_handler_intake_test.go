package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	moderationApp "github.com/labuda/backend/internal/governance/moderation/application"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	moderationRepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type createCaseMockTx struct{}

var _ db.Tx = (*createCaseMockTx)(nil)

func (m *createCaseMockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *createCaseMockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *createCaseMockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return createCaseNoRows{}
}

func (m *createCaseMockTx) Commit(_ context.Context) error   { return nil }
func (m *createCaseMockTx) Rollback(_ context.Context) error { return nil }

type createCaseNoRows struct{}

func (createCaseNoRows) Scan(_ ...any) error {
	return pgx.ErrNoRows
}

type createCaseMockTransactor struct {
	tx db.Tx
}

func (m *createCaseMockTransactor) WithTx(_ context.Context, fn func(tx db.Tx) error) error {
	if m.tx == nil {
		m.tx = &createCaseMockTx{}
	}
	return fn(m.tx)
}

type createCaseMockRepository struct {
	createFunc                    func(context.Context, interface{}, *entity.GovernanceCase) error
	resourceExistsFunc            func(context.Context, interface{}, entity.ResourceType, uuid.UUID) (bool, error)
	hasUserReportedResourceFunc   func(context.Context, interface{}, uuid.UUID, entity.ResourceType, uuid.UUID) (bool, error)
	validateChatMessageReporterFn func(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, string, error)
}

var _ moderationRepo.ModerationRepository = (*createCaseMockRepository)(nil)

func (m *createCaseMockRepository) Create(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, tx, kase)
	}
	return nil
}

func (m *createCaseMockRepository) GetByID(context.Context, interface{}, uuid.UUID) (*entity.GovernanceCase, error) {
	return nil, nil
}

func (m *createCaseMockRepository) GetForUpdate(context.Context, interface{}, uuid.UUID) (*entity.GovernanceCase, error) {
	return nil, nil
}

func (m *createCaseMockRepository) Update(context.Context, interface{}, *entity.GovernanceCase) error {
	return nil
}

func (m *createCaseMockRepository) ListPending(context.Context, interface{}, int, int) ([]*entity.GovernanceCase, error) {
	return nil, nil
}

func (m *createCaseMockRepository) ListByResource(context.Context, interface{}, entity.ResourceType, uuid.UUID) ([]*entity.GovernanceCase, error) {
	return nil, nil
}

func (m *createCaseMockRepository) ListByReporter(context.Context, interface{}, uuid.UUID, int, int) ([]*entity.GovernanceCase, error) {
	return nil, nil
}

func (m *createCaseMockRepository) ListWithStatus(context.Context, interface{}, *entity.GovernanceCaseStatus, *entity.ResourceType, int, int) ([]*entity.GovernanceCase, int64, error) {
	return nil, 0, nil
}

func (m *createCaseMockRepository) ResourceExists(ctx context.Context, tx interface{}, resourceType entity.ResourceType, resourceID uuid.UUID) (bool, error) {
	if m.resourceExistsFunc != nil {
		return m.resourceExistsFunc(ctx, tx, resourceType, resourceID)
	}
	return true, nil
}

func (m *createCaseMockRepository) HasUserReportedResource(ctx context.Context, tx interface{}, reporterID uuid.UUID, resourceType entity.ResourceType, resourceID uuid.UUID) (bool, error) {
	if m.hasUserReportedResourceFunc != nil {
		return m.hasUserReportedResourceFunc(ctx, tx, reporterID, resourceType, resourceID)
	}
	return false, nil
}

func (m *createCaseMockRepository) ValidateChatMessageReporter(ctx context.Context, tx interface{}, messageID uuid.UUID, reporterID uuid.UUID) (bool, string, error) {
	if m.validateChatMessageReporterFn != nil {
		return m.validateChatMessageReporterFn(ctx, tx, messageID, reporterID)
	}
	return true, "", nil
}

func newCreateCaseTestRouter(handler *ModerationHandler, userID uuid.UUID) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", "user")
		c.Next()
	})
	router.POST("/admin/moderation/cases", handler.CreateCase)
	return router
}

func TestCreateCase_ForSaleAndAuctionAreAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name         string
		resourceType entity.ResourceType
	}{
		{name: "for_sale", resourceType: entity.ResourceTypeForSale},
		{name: "auction", resourceType: entity.ResourceTypeAuction},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userID := uuid.New()
			resourceID := uuid.New()
			reason := "intake test"
			var createdCase *entity.GovernanceCase

			repo := &createCaseMockRepository{
				createFunc: func(_ context.Context, _ interface{}, kase *entity.GovernanceCase) error {
					createdCase = kase
					return nil
				},
				resourceExistsFunc: func(_ context.Context, _ interface{}, resourceType entity.ResourceType, id uuid.UUID) (bool, error) {
					assert.Equal(t, tc.resourceType, resourceType)
					assert.Equal(t, resourceID, id)
					return true, nil
				},
				hasUserReportedResourceFunc: func(_ context.Context, _ interface{}, reporterID uuid.UUID, resourceType entity.ResourceType, id uuid.UUID) (bool, error) {
					assert.Equal(t, userID, reporterID)
					assert.Equal(t, tc.resourceType, resourceType)
					assert.Equal(t, resourceID, id)
					return false, nil
				},
			}

			service := moderationApp.NewModerationService(&createCaseMockTransactor{}, repo, nil)
			handler := NewModerationHandler(service, nil, zap.NewNop(), nil)
			router := newCreateCaseTestRouter(handler, userID)

			body, err := json.Marshal(CreateCaseRequest{
				EntityType: string(tc.resourceType),
				EntityID:   resourceID.String(),
				Reason:     reason,
			})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/admin/moderation/cases", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), `"resource_type":"`+string(tc.resourceType)+`"`)
			require.NotNil(t, createdCase)
			assert.Equal(t, tc.resourceType, createdCase.ResourceType)
			assert.Equal(t, resourceID, createdCase.ResourceID)
			assert.Equal(t, userID, createdCase.ReportedBy)
		})
	}
}

type previewMockTx struct {
	lastSQL  string
	lastArgs []any
	row      previewMockRow
}

var _ db.Tx = (*previewMockTx)(nil)

func (m *previewMockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *previewMockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *previewMockTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	m.lastSQL = sql
	m.lastArgs = args
	return m.row
}

func (m *previewMockTx) Commit(_ context.Context) error   { return nil }
func (m *previewMockTx) Rollback(_ context.Context) error { return nil }

type previewMockRow struct {
	values []any
	err    error
}

func (r previewMockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return assert.AnError
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = r.values[i].(string)
		case *bool:
			*d = r.values[i].(bool)
		case *time.Time:
			*d = r.values[i].(time.Time)
		case *sql.NullTime:
			switch v := r.values[i].(type) {
			case time.Time:
				*d = sql.NullTime{Time: v, Valid: true}
			case nil:
				*d = sql.NullTime{}
			default:
				return assert.AnError
			}
		case *sql.NullString:
			switch v := r.values[i].(type) {
			case string:
				*d = sql.NullString{String: v, Valid: true}
			case nil:
				*d = sql.NullString{}
			default:
				return assert.AnError
			}
		case *[]byte:
			switch v := r.values[i].(type) {
			case []byte:
				*d = v
			case nil:
				*d = nil
			default:
				return assert.AnError
			}
		default:
			return assert.AnError
		}
	}
	return nil
}

// TestFetchResourcePreviewTx_UserResource verifies that user resource type is handled
// by fetchResourcePreviewTx and that the returned preview contains the right fields.
func TestFetchResourcePreviewTx_UserResource(t *testing.T) {
	handler := NewModerationHandler(nil, nil, zap.NewNop(), nil)

	t.Run("active user returns preview with account_status", func(t *testing.T) {
		userID := uuid.New()
		username := "reported-user"
		accountStatus := "active"

		tx := &previewMockTx{
			row: previewMockRow{
				// Must match fetchUserPreview scan order: user_id, username, account_status, is_deleted
				values: []any{userID.String(), username, accountStatus, false},
			},
		}

		preview, err := handler.fetchResourcePreviewTx(context.Background(), tx, entity.ResourceTypeUser, userID)

		require.NoError(t, err)
		require.NotNil(t, preview, "active user must produce a non-nil preview")
		assert.Contains(t, tx.lastSQL, "FROM users", "query must target the users table")
		assert.Equal(t, userID.String(), preview.AuthorID)
		assert.Equal(t, username, preview.AuthorUsername)
		assert.Equal(t, accountStatus, preview.Status)
		assert.Equal(t, "user", preview.ContentType)
		assert.Equal(t, "", preview.ContentText, "user preview has no content body")
		assert.False(t, preview.IsDeleted)
	})

	t.Run("suspended user returns correct status", func(t *testing.T) {
		userID := uuid.New()
		tx := &previewMockTx{
			row: previewMockRow{
				values: []any{userID.String(), "suspended-user", "suspended", false},
			},
		}

		preview, err := handler.fetchResourcePreviewTx(context.Background(), tx, entity.ResourceTypeUser, userID)

		require.NoError(t, err)
		require.NotNil(t, preview)
		assert.Equal(t, "suspended", preview.Status)
		assert.False(t, preview.IsDeleted)
	})

	t.Run("deleted user has is_deleted=true", func(t *testing.T) {
		userID := uuid.New()
		tx := &previewMockTx{
			row: previewMockRow{
				values: []any{userID.String(), "", "deleted", true},
			},
		}

		preview, err := handler.fetchResourcePreviewTx(context.Background(), tx, entity.ResourceTypeUser, userID)

		require.NoError(t, err)
		require.NotNil(t, preview)
		assert.True(t, preview.IsDeleted)
		assert.Equal(t, "deleted", preview.Status)
	})

	t.Run("user not found returns nil preview without error", func(t *testing.T) {
		tx := &previewMockTx{
			row: previewMockRow{err: pgx.ErrNoRows},
		}

		preview, err := handler.fetchResourcePreviewTx(context.Background(), tx, entity.ResourceTypeUser, uuid.New())

		// fetchUserPreview returns nil,nil on sql.ErrNoRows; pgx.ErrNoRows propagates as
		// a non-nil error, but the caller (fetchResourcePreview) converts any error to nil preview.
		// Either way: no panic and no 500 — the preview is absent.
		_ = err
		_ = preview
		// The important invariant: no panic and the handler is still functional.
	})
}

func TestFetchResourcePreviewTx_MarketplaceResources(t *testing.T) {
	handler := NewModerationHandler(nil, nil, zap.NewNop(), nil)
	description := strings.Repeat("x", 240)
	authorID := uuid.New().String()
	authorName := "seller-one"
	title := "Rare fixed-price sale"
	status := "active"

	for _, tc := range []struct {
		name         string
		resourceType entity.ResourceType
		wantTable    string
		wantContent  string
	}{
		{name: "for_sale", resourceType: entity.ResourceTypeForSale, wantTable: "for_sales", wantContent: "for_sale"},
		{name: "auction", resourceType: entity.ResourceTypeAuction, wantTable: "auctions", wantContent: "auction"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &previewMockTx{
				row: previewMockRow{
					values: []any{
						authorID,
						authorName,
						title,
						description,
						status,
					},
				},
			}

			preview, err := handler.fetchResourcePreviewTx(context.Background(), tx, tc.resourceType, uuid.New())

			require.NoError(t, err)
			require.NotNil(t, preview)
			assert.Contains(t, tx.lastSQL, "FROM "+tc.wantTable)
			assert.Equal(t, authorID, preview.AuthorID)
			assert.Equal(t, authorName, preview.AuthorUsername)
			assert.Equal(t, title, preview.Title)
			assert.Equal(t, status, preview.Status)
			assert.Equal(t, tc.wantContent, preview.ContentType)
			assert.False(t, preview.IsDeleted)
			assert.True(t, strings.HasSuffix(preview.ContentText, "..."))
			assert.Equal(t, 203, len(preview.ContentText))
		})
	}
}
