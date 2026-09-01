//go:build integration

package http

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

// fakeAdminAuditLogger implements AdminAuditLogger (defined in report_handler.go)
// for handler tests that do not exercise audit logging.
type fakeAdminAuditLogger struct{}

func (fakeAdminAuditLogger) Log(_ context.Context, _ uuid.UUID, _ string, _ string, _ uuid.UUID, _ map[string]interface{}) error {
	return nil
}

func (fakeAdminAuditLogger) LogSafe(_ context.Context, _ uuid.UUID, _ string, _ string, _ uuid.UUID, _ map[string]interface{}) {
}

func (fakeAdminAuditLogger) LogTx(_ context.Context, _ db.Tx, _ uuid.UUID, _ string, _ string, _ uuid.UUID, _ map[string]interface{}) error {
	return nil
}
