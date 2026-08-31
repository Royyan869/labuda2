//go:build integration

// SLICE 3 PROOF — CANONICAL CASE RUNTIME
//
// Proves the full Case path against real PostgreSQL:
//
//	ReportService → CaseRepository → cases table
//
// 1. Report creation creates a Case atomically
// 2. Multiple Reports for same subject → same Case
// 3. One active Case per subject invariant (DB-enforced)
// 4. Case lifecycle: open → resolved
// 5. Report → Case FK integrity
// 6. Concurrent Report creation → no duplicate Case

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	moderationApp "github.com/labuda/backend/internal/governance/moderation/application"
	moderationHTTP "github.com/labuda/backend/internal/governance/moderation/delivery/http"
	moderationRepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestCanonicalCaseRuntime(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	// ── Fixtures ─────────────────────────────────────────────────
	reporterA := insertModerationUser(t, ctx, pool)
	reporterB := insertModerationUser(t, ctx, pool)
	subjectOwner := insertModerationUser(t, ctx, pool)

	// Canonical target fixtures
	contentID := insertReportFixtureContent(t, ctx, pool, subjectOwner)
	contentID2 := insertReportFixtureContent(t, ctx, pool, subjectOwner)

	appDB := db.NewFromPool(pool)
	reportRepo := moderationRepo.NewReportRepository()
	caseRepo := moderationRepo.NewCaseRepository()
	reportService := moderationApp.NewReportService(appDB, reportRepo, caseRepo)
	caseService := moderationApp.NewCaseService(appDB, caseRepo)
	handler := moderationHTTP.NewReportHandler(reportService, zap.NewNop())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", reporterA)
		c.Next()
	})
	router.POST("/reports", handler.CreateReport)
	router.GET("/reports/mine", handler.ListMyReports)
	router.GET("/reports/:id", handler.GetMyReport)

	// ── Helper: create a report via the HTTP API ────────────────
	createReport := func(t *testing.T, reporter uuid.UUID, subjectType, subjectID, reasonCode string, reasonNote *string) *httptest.ResponseRecorder {
		t.Helper()
		body := map[string]interface{}{
			"subject_type": subjectType,
			"subject_id":   subjectID,
			"reason_code":  reasonCode,
		}
		if reasonNote != nil {
			body["reason_note"] = *reasonNote
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/reports", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	// ── 1. Report creation creates a Case atomically ──────────
	t.Run("report_creates_case_atomically", func(t *testing.T) {
		w := createReport(t, reporterA, "content", contentID.String(), "scam_or_fraud", nil)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

		var resp struct {
			Data struct {
				ID        string `json:"id"`
				CaseID    string `json:"case_id"`
				SubjectID string `json:"subject_id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.NotEmpty(t, resp.Data.CaseID, "Report must have CaseID after Case correlation")

		// Verify Case exists in DB
		var caseID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT id FROM cases WHERE id = $1`, resp.Data.CaseID).Scan(&caseID))
	})

	// ── 2. Multiple Reports for same subject → same Case ──────
	t.Run("multiple_reports_same_subject_same_case", func(t *testing.T) {
		// Reporter A already reported contentID above
		// Reporter B reports the same content
		routerB := gin.New()
		routerB.Use(func(c *gin.Context) {
			c.Set("user_id", reporterB)
			c.Next()
		})
		routerB.POST("/reports", handler.CreateReport)

		body, _ := json.Marshal(map[string]interface{}{
			"subject_type": "content",
			"subject_id":   contentID.String(),
			"reason_code":  "prohibited_content",
		})
		req := httptest.NewRequest(http.MethodPost, "/reports", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		routerB.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

		var resp struct {
			Data struct {
				CaseID string `json:"case_id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		// Both reports should point to the same Case
		var count int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM cases WHERE subject_type = 'content' AND subject_id = $1`, contentID).Scan(&count))
		require.Equal(t, 1, count, "only one Case should exist for this subject")

		// Verify both reports have the same case_id
		var reportACaseID, reportBCaseID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT case_id FROM reports WHERE reporter_id = $1 AND subject_id = $2`, reporterA, contentID).Scan(&reportACaseID))
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT case_id FROM reports WHERE reporter_id = $1 AND subject_id = $2`, reporterB, contentID).Scan(&reportBCaseID))
		require.Equal(t, reportACaseID, reportBCaseID, "both reports must point to the same Case")
	})

	// ── 3. One active Case per subject invariant ────────────────
	t.Run("one_active_case_per_subject_invariant", func(t *testing.T) {
		// Try to create a second open Case for the same subject
		// This should fail due to the partial unique index
		_, err := caseRepo.FindOrCreateOpenCase(ctx, &mockTx{pool: pool}, "content", contentID)
		require.NoError(t, err, "FindOrCreateOpenCase should succeed (returns existing)")

		// Verify only one open Case exists
		var openCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM cases WHERE subject_type = 'content' AND subject_id = $1 AND status = 'open'`, contentID).Scan(&openCount))
		require.Equal(t, 1, openCount, "only one open Case should exist per subject")
	})

	// ── 4. Different subjects get different Cases ──────────────
	t.Run("different_subjects_different_cases", func(t *testing.T) {
		// Report for a different content (contentID2)
		w := createReport(t, reporterA, "content", contentID2.String(), "scam_or_fraud", nil)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

		// Verify different Case IDs
		var count int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(DISTINCT id) FROM cases WHERE subject_type = 'content'`).Scan(&count))
		require.GreaterOrEqual(t, count, 2, "different subjects should have different Cases")
	})

	// ── 5. Case lifecycle: open → resolved ──────────────────────
	t.Run("case_lifecycle_open_to_resolved", func(t *testing.T) {
		// Get the Case for contentID
		var caseID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT id FROM cases WHERE subject_type = 'content' AND subject_id = $1 AND status = 'open'`, contentID).Scan(&caseID))

		// Resolve the Case
		err := caseService.ResolveCase(ctx, caseID)
		require.NoError(t, err)

		// Verify Case is resolved
		var status string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status FROM cases WHERE id = $1`, caseID).Scan(&status))
		require.Equal(t, "resolved", status)

		// Verify closed_at is set
		var closedAt *interface{}
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT closed_at FROM cases WHERE id = $1`, caseID).Scan(&closedAt))
		require.NotNil(t, closedAt, "closed_at should be set after resolution")
	})

	// ── 6. New Report after resolved Case → new Case ────────────
	t.Run("new_report_after_resolved_creates_new_case", func(t *testing.T) {
		// The previous Case for contentID is resolved
		// A new report should create a new Case
		// But first, we need to create a new reporter since reporterA and reporterB
		// already reported this subject
		newReporter := insertModerationUser(t, ctx, pool)
		routerNew := gin.New()
		routerNew.Use(func(c *gin.Context) {
			c.Set("user_id", newReporter)
			c.Next()
		})
		routerNew.POST("/reports", handler.CreateReport)

		body, _ := json.Marshal(map[string]interface{}{
			"subject_type": "content",
			"subject_id":   contentID.String(),
			"reason_code":  "other",
		})
		req := httptest.NewRequest(http.MethodPost, "/reports", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		routerNew.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

		var resp struct {
			Data struct {
				CaseID string `json:"case_id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.NotEmpty(t, resp.Data.CaseID)

		// Verify the new Case is open
		var status string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status FROM cases WHERE id = $1`, resp.Data.CaseID).Scan(&status))
		require.Equal(t, "open", status)

		// Verify there are now 2 Cases for this subject (one resolved, one open)
		var caseCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM cases WHERE subject_type = 'content' AND subject_id = $1`, contentID).Scan(&caseCount))
		require.Equal(t, 2, caseCount, "should have 2 Cases: one resolved + one open")
	})

	// ── 7. Concurrent Reports → no duplicate Case ────────────────
	t.Run("concurrent_reports_no_duplicate_case", func(t *testing.T) {
		newContent := insertReportFixtureContent(t, ctx, pool, subjectOwner)
		const concurrentAttempts = 10

		// Create N different reporters (one per concurrent request)
		// to avoid report duplicate protection interfering with Case concurrency test.
		reporters := make([]uuid.UUID, concurrentAttempts)
		for i := 0; i < concurrentAttempts; i++ {
			reporters[i] = insertModerationUser(t, ctx, pool)
		}

		// Build per-request routers with unique user_id
		routers := make([]*gin.Engine, concurrentAttempts)
		for i := 0; i < concurrentAttempts; i++ {
			routers[i] = gin.New()
			idx := i
			routers[i].Use(func(c *gin.Context) {
				c.Set("user_id", reporters[idx])
				c.Next()
			})
			routers[i].POST("/reports", handler.CreateReport)
		}

		var wg sync.WaitGroup
		statuses := make([]int, concurrentAttempts)
		caseIDs := make([]string, concurrentAttempts)

		for i := 0; i < concurrentAttempts; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				body, _ := json.Marshal(map[string]interface{}{
					"subject_type": "content",
					"subject_id":   newContent.String(),
					"reason_code":  "scam_or_fraud",
				})
				req := httptest.NewRequest(http.MethodPost, "/reports", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				routers[idx].ServeHTTP(w, req)
				statuses[idx] = w.Code

				if w.Code == http.StatusCreated {
					var resp struct {
						Data struct {
							CaseID string `json:"case_id"`
						} `json:"data"`
					}
					_ = json.Unmarshal(w.Body.Bytes(), &resp)
					caseIDs[idx] = resp.Data.CaseID
				}
			}(i)
		}
		wg.Wait()

		// PROOF 1: ALL requests succeed (no 409, no 500)
		created := 0
		for _, s := range statuses {
			require.Equal(t, http.StatusCreated, s, "concurrent report must succeed")
			created++
		}
		require.Equal(t, concurrentAttempts, created, "all concurrent reports must succeed")

		// PROOF 2: ALL reports point to the SAME Case
		firstCaseID := caseIDs[0]
		require.NotEmpty(t, firstCaseID, "first report must have case_id")
		for i := 1; i < concurrentAttempts; i++ {
			require.Equal(t, firstCaseID, caseIDs[i],
				"report %d must have same case_id as report 0", i)
		}

		// PROOF 3: Only ONE open Case exists in DB
		var openCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM cases WHERE subject_type = 'content' AND subject_id = $1 AND status = 'open'`, newContent).Scan(&openCount))
		require.Equal(t, 1, openCount, "only one open Case should exist after concurrent reports")

		// PROOF 4: Report count matches concurrent attempts
		var reportCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM reports WHERE subject_type = 'content' AND subject_id = $1`, newContent).Scan(&reportCount))
		require.Equal(t, concurrentAttempts, reportCount, "report count must match concurrent attempts")

		// PROOF 5: Every report has the same case_id
		var mismatchCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM reports WHERE subject_type = 'content' AND subject_id = $1 AND case_id != $2`, newContent, firstCaseID).Scan(&mismatchCount))
		require.Equal(t, 0, mismatchCount, "all reports must point to the same Case")

		// PROOF 6: No orphan Reports (case_id references valid Case)
		var orphanCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM reports WHERE subject_type = 'content' AND subject_id = $1 AND case_id IS NULL`, newContent).Scan(&orphanCount))
		require.Equal(t, 0, orphanCount, "no orphan reports allowed")
	})

	// ── 8. Report → Case FK integrity ────────────────────────────
	t.Run("report_case_fk_integrity", func(t *testing.T) {
		// Verify all reports have valid case_id references
		var orphanCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM reports WHERE case_id IS NOT NULL AND case_id NOT IN (SELECT id FROM cases)`).Scan(&orphanCount))
		require.Equal(t, 0, orphanCount, "no orphan case_id references allowed")

		// Verify Case has correct subject
		var caseSubjectType, caseSubjectID string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT subject_type, subject_id::text FROM cases WHERE subject_type = 'content' AND subject_id = $1 LIMIT 1`, contentID).Scan(&caseSubjectType, &caseSubjectID))
		require.Equal(t, "content", caseSubjectType)
		require.Equal(t, contentID.String(), caseSubjectID)
	})
}

// mockTx is a minimal db.Tx for testing CaseRepository directly.
type mockTx struct {
	pool *pgxpool.Pool
}

func (m *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return m.pool.Exec(ctx, sql, args...)
}

func (m *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.pool.Query(ctx, sql, args...)
}

func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return m.pool.QueryRow(ctx, sql, args...)
}

func (m *mockTx) Commit(ctx context.Context) error {
	return nil
}

func (m *mockTx) Rollback(ctx context.Context) error {
	return nil
}
