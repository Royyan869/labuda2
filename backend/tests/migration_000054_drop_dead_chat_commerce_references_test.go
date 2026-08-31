package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/testdb"
)

func TestMigration000054_DropsDeadChatCommerceReferences(t *testing.T) {
	tdb, _ := testdb.SetupDB(t)
	ctx := context.Background()

	exists := func(query string) bool {
		var ok bool
		err := tdb.Pool().QueryRow(ctx, query).Scan(&ok)
		require.NoError(t, err)
		return ok
	}

	// After migration 000054, the chat_commerce_references table must NOT exist.
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='chat_commerce_references')`),
		"chat_commerce_references table must be dropped by migration 000054")

	// After migration 000054, the enum must NOT exist.
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type t WHERE t.typname='chat_commerce_reference_target_type_enum')`),
		"chat_commerce_reference_target_type_enum must be dropped by migration 000054")
}
