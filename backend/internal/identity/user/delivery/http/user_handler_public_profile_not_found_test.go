package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	userApp "github.com/labuda/backend/internal/identity/user/application"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

type notFoundTx struct{}

func (t *notFoundTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *notFoundTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (t *notFoundTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}

func (t *notFoundTx) Commit(ctx context.Context) error { return nil }

func (t *notFoundTx) Rollback(ctx context.Context) error { return nil }

type notFoundDB struct{}

func (d *notFoundDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(&notFoundTx{})
}

func (d *notFoundDB) Pool() *pgxpool.Pool { return nil }

type notFoundProfileRepo struct{}

func (r *notFoundProfileRepo) SoftDeleteUser(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (bool, error) {
	return false, nil
}

func (r *notFoundProfileRepo) GetPublicInfo(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	isOwnProfile bool,
) (*userEntity.UserPublicInfo, error) {
	return nil, nil
}

func (r *notFoundProfileRepo) GetMyProfile(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*userEntity.MyProfileResponse, error) {
	return nil, nil
}

func (r *notFoundProfileRepo) GetByIDForUpdate(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*userEntity.User, error) {
	return nil, nil
}

func (r *notFoundProfileRepo) Update(
	ctx context.Context,
	tx db.Tx,
	user *userEntity.User,
) error {
	return nil
}

func TestGetPublicUser_ReturnsNotFoundForMissingUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := userApp.NewUserProfileService(
		&notFoundProfileRepo{},
		nil,
		nil,
		nil,
		nil,
		&notFoundDB{},
	)
	handler := NewUserHandler(svc, nil, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	targetID := uuid.New()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+targetID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: targetID.String()}}

	handler.GetPublicUser(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want %d", w.Code, http.StatusNotFound)
	}
}
