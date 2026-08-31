//go:build integration

// SLICE 2 PROOF — CANONICAL REPORT RUNTIME
//
// Proves the full Report path against real PostgreSQL:
//
//	HTTP handler → ReportService → ReportRepository → reports table
//
// 1. Every canonical target (content, comment, for_sale, auction, user) can be
//    reported with valid input.
// 2. chat_message and fixed_price_sale are rejected.
// 3. Reason codes outside the locked taxonomy are rejected.
// 4. reason_note is optional (report without note and with note both valid).
// 5. Non-existent targets are rejected.
// 6. Same reporter + same subject → duplicate rejected (race-safe unique index).
// 7. Different reporters + same subject → both valid.
// 8. Report rows are immutable (UPDATE rejected by trigger).
// 9. API contract: POST /reports → 201 with canonical report shape.

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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	moderationApp "github.com/labuda/backend/internal/governance/moderation/application"
	moderationHTTP "github.com/labuda/backend/internal/governance/moderation/delivery/http"
	moderationRepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestCanonicalReportRuntime(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	// ── Fixtures ─────────────────────────────────────────────────
	reporterA := insertModerationUser(t, ctx, pool)
	reporterB := insertModerationUser(t, ctx, pool)
	subjectOwner := insertModerationUser(t, ctx, pool)

	// Canonical target fixtures: content, comment, for_sale, auction, user.
	contentID := insertReportFixtureContent(t, ctx, pool, subjectOwner)
	commentID := insertReportFixtureComment(t, ctx, pool, subjectOwner, contentID)
	forSaleID := insertReportFixtureForSale(t, ctx, pool, subjectOwner)
	auctionID := insertReportFixtureAuction(t, ctx, pool, subjectOwner)

	// Non-existent target (a valid UUID with no row).
	missingID := uuid.New()

	appDB := db.NewFromPool(pool)
	reportService := moderationApp.NewReportService(appDB, moderationRepo.NewReportRepository())
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
	createReport := func(t *testing.T, subjectType, subjectID, reasonCode string, reasonNote *string) *httptest.ResponseRecorder {
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

	// ── 1. Valid reports for every canonical target ─────────────
	t.Run("valid_reports_all_canonical_targets", func(t *testing.T) {
		cases := []struct {
			subjectType string
			subjectID   string
		}{
			{"content", contentID.String()},
			{"comment", commentID.String()},
			{"for_sale", forSaleID.String()},
			{"auction", auctionID.String()},
			{"user", subjectOwner.String()},
		}
		for _, tc := range cases {
			w := createReport(t, tc.subjectType, tc.subjectID, "scam_or_fraud", nil)
			require.Equalf(t, http.StatusCreated, w.Code, "subject_type=%s body=%s", tc.subjectType, w.Body.String())

			var resp struct {
				Data struct {
					ID           string `json:"id"`
					ReporterID   string `json:"reporter_id"`
					SubjectType  string `json:"subject_type"`
					SubjectID    string `json:"subject_id"`
					ReasonCode   string `json:"reason_code"`
					EvidenceSnap struct {
						AuthorID string `json:"author_id"`
					} `json:"evidence_snapshot"`
					CreatedAt string `json:"created_at"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			require.Equal(t, tc.subjectType, resp.Data.SubjectType)
			require.Equal(t, tc.subjectID, resp.Data.SubjectID)
			require.Equal(t, "scam_or_fraud", resp.Data.ReasonCode)
			require.Equal(t, reporterA.String(), resp.Data.ReporterID)
			require.NotEmpty(t, resp.Data.ID)
			require.NotEmpty(t, resp.Data.CreatedAt)
			require.NotEmpty(t, resp.Data.EvidenceSnap.AuthorID, "evidence snapshot must capture subject author at report time")
		}
	})

	// ── 2. Invalid targets rejected ─────────────────────────────
	t.Run("invalid_targets_rejected", func(t *testing.T) {
		for _, tc := range []struct {
			subjectType string
		}{
			{"chat_message"},
			{"fixed_price_sale"},
			{"profile"},
			{"listing"},
		} {
			w := createReport(t, tc.subjectType, uuid.NewString(), "other", nil)
			require.Equalf(t, http.StatusBadRequest, w.Code, "subject_type=%s must be rejected", tc.subjectType)
		}
	})

	// ── 3. Invalid reason code rejected ─────────────────────────
	t.Run("invalid_reason_rejected", func(t *testing.T) {
		// Use a fresh subject so the duplicate guard does not interfere.
		extraContent := insertReportFixtureContent(t, ctx, pool, subjectOwner)
		for _, rc := range []string{"spam", "fake_product", "copyright", "harassment", "anything_else"} {
			w := createReport(t, "content", extraContent.String(), rc, nil)
			require.Equalf(t, http.StatusBadRequest, w.Code, "reason_code=%s must be rejected", rc)
		}
	})

	// ── 4. reason_note optional ─────────────────────────────────
	t.Run("reason_note_optional", func(t *testing.T) {
		// Without note.
		noNoteContent := insertReportFixtureContent(t, ctx, pool, subjectOwner)
		w := createReport(t, "content", noNoteContent.String(), "other", nil)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

		// With note.
		withNoteContent := insertReportFixtureContent(t, ctx, pool, subjectOwner)
		note := "User posted repeated scam links in comments."
		w = createReport(t, "content", withNoteContent.String(), "commerce_violation", &note)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	})

	// ── 5. Non-existent target rejected ─────────────────────────
	t.Run("non_existent_target_rejected", func(t *testing.T) {
		w := createReport(t, "content", missingID.String(), "other", nil)
		require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

		w = createReport(t, "user", missingID.String(), "other", nil)
		require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	})

	// ── 5b. Self-report denied (Owner decision, Business Truth §6) ─
	t.Run("self_report_denied", func(t *testing.T) {
		// The subject OWNER reports their own content → denied.
		ownContent := insertReportFixtureContent(t, ctx, pool, subjectOwner)

		// Switch router user to the subject owner.
		routerOwner := gin.New()
		routerOwner.Use(func(c *gin.Context) {
			c.Set("user_id", subjectOwner)
			c.Next()
		})
		routerOwner.POST("/reports", handler.CreateReport)

		body, _ := json.Marshal(map[string]interface{}{
			"subject_type": "content",
			"subject_id":   ownContent.String(),
			"reason_code":  "other",
		})
		req := httptest.NewRequest(http.MethodPost, "/reports", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		routerOwner.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), "cannot report your own")

		// No row was created.
		var count int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM reports WHERE reporter_id=$1 AND subject_id=$2`,
			subjectOwner, ownContent).Scan(&count))
		require.Equal(t, 0, count)
	})

	// ── 6. Duplicate: same reporter + same subject → rejected ──
	t.Run("duplicate_same_reporter_rejected", func(t *testing.T) {
		dupContent := insertReportFixtureContent(t, ctx, pool, subjectOwner)
		w1 := createReport(t, "content", dupContent.String(), "scam_or_fraud", nil)
		require.Equal(t, http.StatusCreated, w1.Code, w1.Body.String())

		w2 := createReport(t, "content", dupContent.String(), "other", nil)
		require.Equal(t, http.StatusConflict, w2.Code, "same reporter + same subject must be rejected")

		var count int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM reports WHERE reporter_id=$1 AND subject_type='content' AND subject_id=$2`,
			reporterA, dupContent).Scan(&count))
		require.Equal(t, 1, count)
	})

	// ── 6b. Duplicate: concurrent inserts (race safety) ────────
	t.Run("duplicate_race_safe", func(t *testing.T) {
		raceContent := insertReportFixtureContent(t, ctx, pool, subjectOwner)

		// Fire two concurrent create-report HTTP requests with the SAME reporter.
		// The unique index uniq_reports_one_per_reporter_subject is the final guard:
		// exactly one INSERT succeeds, the other gets 409.
		const attempts = 8
		var wg sync.WaitGroup
		statuses := make([]int, attempts)
		for i := 0; i < attempts; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				w := createReport(t, "content", raceContent.String(), "scam_or_fraud", nil)
				statuses[idx] = w.Code
			}(i)
		}
		wg.Wait()

		created := 0
		conflicted := 0
		for _, s := range statuses {
			switch s {
			case http.StatusCreated:
				created++
			case http.StatusConflict:
				conflicted++
			}
		}
		require.Equal(t, 1, created, "exactly one concurrent report must succeed: %v", statuses)
		require.Equal(t, attempts-1, conflicted, "all other concurrent reports must be rejected: %v", statuses)

		var count int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM reports WHERE reporter_id=$1 AND subject_id=$2`,
			reporterA, raceContent).Scan(&count))
		require.Equal(t, 1, count, "exactly one report row must exist after concurrent inserts")
	})

	// ── 7. Different reporters + same subject → both valid ─────
	t.Run("different_reporters_same_subject_valid", func(t *testing.T) {
		multiContent := insertReportFixtureContent(t, ctx, pool, subjectOwner)

		// Reporter A reports.
		w := createReport(t, "content", multiContent.String(), "scam_or_fraud", nil)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

		// Reporter B reports the same subject (different key row → valid).
		// Temporarily switch the router user to reporterB.
		routerB := gin.New()
		routerB.Use(func(c *gin.Context) {
			c.Set("user_id", reporterB)
			c.Next()
		})
		routerB.POST("/reports", handler.CreateReport)

		body, _ := json.Marshal(map[string]interface{}{
			"subject_type": "content",
			"subject_id":   multiContent.String(),
			"reason_code":  "prohibited_content",
		})
		req := httptest.NewRequest(http.MethodPost, "/reports", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		wB := httptest.NewRecorder()
		routerB.ServeHTTP(wB, req)
		require.Equal(t, http.StatusCreated, wB.Code, wB.Body.String())

		var count int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM reports WHERE subject_type='content' AND subject_id=$1`,
			multiContent).Scan(&count))
		require.Equal(t, 2, count, "two different reporters may report the same subject")
	})

	// ── 8. Immutability: report historical fields cannot mutate ─
	t.Run("reports_immutable", func(t *testing.T) {
		immContent := insertReportFixtureContent(t, ctx, pool, subjectOwner)
		w := createReport(t, "content", immContent.String(), "other", nil)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

		var reportID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT id FROM reports WHERE reporter_id=$1 AND subject_id=$2`,
			reporterA, immContent).Scan(&reportID))

		_, err := pool.Exec(ctx, `UPDATE reports SET reason_code='scam_or_fraud' WHERE id=$1`, reportID)
		require.Error(t, err, "UPDATE on reports must be rejected")
		require.Contains(t, err.Error(), "immutable")

		_, err = pool.Exec(ctx, `UPDATE reports SET reporter_id=$1 WHERE id=$2`, reporterB, reportID)
		require.Error(t, err, "reporter_id mutation must be rejected")

		// The row is unchanged.
		var reasonCode string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT reason_code FROM reports WHERE id=$1`, reportID).Scan(&reasonCode))
		require.Equal(t, "other", reasonCode)
	})

	// ── 9. Read-back: GET /reports/mine returns own reports ────
	t.Run("get_my_reports", func(t *testing.T) {
		myContent := insertReportFixtureContent(t, ctx, pool, subjectOwner)
		w := createReport(t, "content", myContent.String(), "other", nil)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

		req := httptest.NewRequest(http.MethodGet, "/reports/mine", nil)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req)
		require.Equal(t, http.StatusOK, w2.Code, w2.Body.String())

		var resp struct {
			Data struct {
				Reports []map[string]interface{} `json:"reports"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
		require.NotEmpty(t, resp.Data.Reports)
	})
}

// ── Target fixtures ─────────────────────────────────────────────

// insertReportFixtureContent inserts a contents row and returns its ID.
func insertReportFixtureContent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, author uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO contents (id, author_id, caption, visibility)
		VALUES ($1, $2, $3, 'public')`,
		id, author, "reported content caption")
	require.NoError(t, err)
	return id
}

// insertReportFixtureComment inserts a comments row and returns its ID.
func insertReportFixtureComment(t *testing.T, ctx context.Context, pool *pgxpool.Pool, author uuid.UUID, contentID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO comments (id, author_id, body, target_id, target_type)
		VALUES ($1, $2, $3, $4, 'content')`,
		id, author, "reported comment body", contentID)
	require.NoError(t, err)
	return id
}

// insertReportFixtureForSale inserts a for_sale + product row and returns the for_sale ID.
func insertReportFixtureForSale(t *testing.T, ctx context.Context, pool *pgxpool.Pool, seller uuid.UUID) uuid.UUID {
	t.Helper()
	productID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, variety, preparation_time, selling_surface)
		VALUES ($1, $2, $3, $4, 'standard', 'immediate', 'for_sale')`,
		productID, seller, "reported product", "product description")
	require.NoError(t, err)

	forSaleID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO for_sales (id, seller_id, product_id, status, price_per_unit)
		VALUES ($1, $2, $3, 'active', 100000)`,
		forSaleID, seller, productID)
	require.NoError(t, err)
	return forSaleID
}

// insertReportFixtureAuction inserts an auctions + product row and returns the auction ID.
func insertReportFixtureAuction(t *testing.T, ctx context.Context, pool *pgxpool.Pool, seller uuid.UUID) uuid.UUID {
	t.Helper()
	productID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, variety, preparation_time, selling_surface)
		VALUES ($1, $2, $3, $4, 'standard', 'immediate', 'auction')`,
		productID, seller, "reported auction product", "auction description")
	require.NoError(t, err)

	auctionID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO auctions (id, seller_id, product_id, start_price, bid_increment,
			start_at, end_at, status)
		VALUES ($1, $2, $3, 100000, 10000, now(), now() + interval '7 days', 'active')`,
		auctionID, seller, productID)
	require.NoError(t, err)
	return auctionID
}
