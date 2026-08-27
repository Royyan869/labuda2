package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type resourceExistsMockTx struct {
	lastSQL  string
	lastArgs []any
	rowErr   error
}

var _ db.Tx = (*resourceExistsMockTx)(nil)

func (m *resourceExistsMockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *resourceExistsMockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *resourceExistsMockTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	m.lastSQL = sql
	m.lastArgs = append([]any(nil), args...)
	return resourceExistsMockRow{err: m.rowErr}
}

func (m *resourceExistsMockTx) Commit(_ context.Context) error   { return nil }
func (m *resourceExistsMockTx) Rollback(_ context.Context) error { return nil }

type resourceExistsMockRow struct {
	err error
}

func (r resourceExistsMockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return fmt.Errorf("unexpected scan target count: %d", len(dest))
	}

	switch d := dest[0].(type) {
	case *int:
		*d = 1
		return nil
	default:
		return fmt.Errorf("unexpected scan dest type %T", dest[0])
	}
}

func TestModerationRepositoryImpl_ResourceExists_UsesExpectedTablesAndFilters(t *testing.T) {
	repo := NewModerationRepository()
	resourceID := uuid.New()

	tests := []struct {
		name            string
		resourceType    entity.ResourceType
		wantTable       string
		wantDeletedAt   bool
		wantLimitClause bool
	}{
		{name: "content", resourceType: entity.ResourceTypeContent, wantTable: "contents", wantDeletedAt: true, wantLimitClause: true},
		{name: "comment", resourceType: entity.ResourceTypeComment, wantTable: "comments", wantDeletedAt: true, wantLimitClause: true},
		{name: "user", resourceType: entity.ResourceTypeUser, wantTable: "users", wantDeletedAt: true, wantLimitClause: true},
		{name: "chat_message", resourceType: entity.ResourceTypeChatMessage, wantTable: "chat_messages", wantDeletedAt: true, wantLimitClause: true},
		{name: "for_sale", resourceType: entity.ResourceTypeForSale, wantTable: "for_sales", wantDeletedAt: false, wantLimitClause: true},
		{name: "auction", resourceType: entity.ResourceTypeAuction, wantTable: "auctions", wantDeletedAt: false, wantLimitClause: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tx := &resourceExistsMockTx{}

			ok, err := repo.ResourceExists(context.Background(), tx, tc.resourceType, resourceID)

			require.NoError(t, err)
			assert.True(t, ok)
			assert.Contains(t, tx.lastSQL, "FROM "+tc.wantTable)
			assert.Contains(t, tx.lastSQL, "id = $1")
			if tc.wantDeletedAt {
				assert.Contains(t, tx.lastSQL, "deleted_at IS NULL")
			} else {
				assert.NotContains(t, tx.lastSQL, "deleted_at IS NULL")
			}
			if tc.wantLimitClause {
				assert.True(t, strings.Contains(tx.lastSQL, "LIMIT 1"))
			}
			require.Len(t, tx.lastArgs, 1)
			assert.Equal(t, resourceID, tx.lastArgs[0])
		})
	}
}

func TestModerationRepositoryImpl_ResourceExists_ReturnsFalseForMissingMarketplaceResource(t *testing.T) {
	repo := NewModerationRepository()
	tx := &resourceExistsMockTx{rowErr: pgx.ErrNoRows}

	ok, err := repo.ResourceExists(context.Background(), tx, entity.ResourceTypeForSale, uuid.New())

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Contains(t, tx.lastSQL, "FROM for_sales")
	assert.NotContains(t, tx.lastSQL, "deleted_at IS NULL")
}


