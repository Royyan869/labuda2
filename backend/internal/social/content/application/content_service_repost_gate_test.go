package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
)

type repostGateTx struct {
	authorStatus  map[uuid.UUID]string
	authorDeleted map[uuid.UUID]bool
}

func (m *repostGateTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *repostGateTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *repostGateTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	status := "active"
	deleted := false
	if len(args) > 0 {
		if authorID, ok := args[0].(uuid.UUID); ok {
			if m.authorStatus != nil {
				if v, ok := m.authorStatus[authorID]; ok {
					status = v
				}
			}
			if m.authorDeleted != nil {
				if v, ok := m.authorDeleted[authorID]; ok {
					deleted = v
				}
			}
		}
	}
	return &repostGateRow{status: status, deleted: deleted}
}

func (m *repostGateTx) Commit(ctx context.Context) error {
	return nil
}

func (m *repostGateTx) Rollback(ctx context.Context) error {
	return nil
}

type repostGateRow struct {
	status  string
	deleted bool
	err     error
}

func (r *repostGateRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, d := range dest {
		switch v := d.(type) {
		case *uuid.UUID:
		case *string:
			if i == 0 {
				*v = r.status
			}
		case *bool:
			if i == 1 {
				*v = r.deleted
			}
		}
	}
	return nil
}

type repostGateRepo struct {
	contentrepo.ContentRepository
	target      *entity.Content
	contentByID map[uuid.UUID]*entity.Content
	occurrences map[uuid.UUID]*entity.ContentResourceOccurrence
	createCalls int
	lastCreated *entity.Content
}

func (r *repostGateRepo) Create(ctx context.Context, tx interface{}, content *entity.Content) error {
	r.createCalls++
	r.lastCreated = content
	return nil
}

func (r *repostGateRepo) CreateMedia(ctx context.Context, tx interface{}, media []*entity.ContentMedia) error {
	return nil
}

func (r *repostGateRepo) CreateResourceOccurrence(ctx context.Context, tx interface{}, occurrence *entity.ContentResourceOccurrence) error {
	if r.occurrences == nil {
		r.occurrences = map[uuid.UUID]*entity.ContentResourceOccurrence{}
	}
	r.occurrences[occurrence.ContentID] = occurrence
	return nil
}

func (r *repostGateRepo) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
	if r.contentByID != nil {
		if content, ok := r.contentByID[id]; ok {
			return content, nil
		}
	}
	if r.target != nil && r.target.ID == id {
		return r.target, nil
	}
	return nil, errors.New("content not found")
}

func (r *repostGateRepo) GetForUpdate(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
	return r.GetByID(ctx, tx, id)
}

func (r *repostGateRepo) Update(ctx context.Context, tx interface{}, content *entity.Content) error {
	return nil
}

func (r *repostGateRepo) ListByAuthor(ctx context.Context, tx interface{}, authorID uuid.UUID, limit int, cursor string) ([]*entity.Content, string, error) {
	return nil, "", nil
}

func (r *repostGateRepo) GetMedia(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]*entity.ContentMedia, error) {
	return nil, nil
}

func (r *repostGateRepo) GetResourceOccurrenceByContentID(ctx context.Context, tx interface{}, contentID uuid.UUID) (*entity.ContentResourceOccurrence, error) {
	if r.occurrences != nil {
		if occ, ok := r.occurrences[contentID]; ok {
			return occ, nil
		}
	}
	return nil, errors.New("content resource occurrence not found")
}

type activeAccountChecker struct{}

func (a activeAccountChecker) EnsureActive(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (a activeAccountChecker) GetStatus(ctx context.Context, userID uuid.UUID) (string, error) {
	return "active", nil
}

func (a activeAccountChecker) IsBanned(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

func newRepostGateService(target *entity.Content) (*ContentService, *repostGateRepo) {
	repo := &repostGateRepo{
		target: target,
	}
	if target != nil {
		repo.contentByID = map[uuid.UUID]*entity.Content{
			target.ID: target,
		}
	}
	return &ContentService{
		contentRepo:          repo,
		accountStatusChecker: activeAccountChecker{},
	}, repo
}

func newRepostGateServiceWithRepo(repo *repostGateRepo) *ContentService {
	return &ContentService{
		contentRepo:          repo,
		accountStatusChecker: activeAccountChecker{},
	}
}

func TestCreateRepost_AllowsActiveContent(t *testing.T) {
	ctx := context.Background()
	callerID := uuid.New()
	originalContentID := uuid.New()

	originalAuthorID := uuid.New()
	tx := &repostGateTx{
		authorStatus: map[uuid.UUID]string{
			originalAuthorID: "active",
		},
	}
	service, repo := newRepostGateService(&entity.Content{
		ID:        originalContentID,
		AuthorID:  originalAuthorID,
		Status:    entity.StatusActive,
		Caption:   ptrString("original post"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	req := &CreateRepostRequest{
		OriginalContentID:       originalContentID,
		Caption:                 "repost caption",
		OriginalContentTitle:    "original post",
		OriginalContentImageURL: "https://example.com/image.jpg",
	}

	content, err := service.CreateRepost(ctx, tx, callerID, req)

	if err != nil {
		t.Fatalf("CreateRepost unexpectedly failed for active post: %v", err)
	}
	if repo.createCalls != 1 {
		t.Fatalf("CreateRepost write count = %d, want 1", repo.createCalls)
	}
	if content == nil || !content.IsRepost() {
		t.Fatal("expected repost content to be created")
	}
}

func TestCreateRepost_RejectsTargetAuthorLifecycle(t *testing.T) {
	cases := []struct {
		name          string
		status        string
		deleted       bool
		errorFragment string
	}{
		{name: "suspended", status: "suspended", errorFragment: "content not found"},
		{name: "banned", status: "banned", errorFragment: "content not found"},
		{name: "soft-deleted", status: "active", deleted: true, errorFragment: "content not found"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			callerID := uuid.New()
			originalContentID := uuid.New()
			originalAuthorID := uuid.New()
			tx := &repostGateTx{
				authorStatus: map[uuid.UUID]string{
					originalAuthorID: tc.status,
				},
				authorDeleted: map[uuid.UUID]bool{
					originalAuthorID: tc.deleted,
				},
			}

			service, repo := newRepostGateService(&entity.Content{
				ID:        originalContentID,
				AuthorID:  originalAuthorID,
				Status:    entity.StatusActive,
				Caption:   ptrString("original post"),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			})

			req := &CreateRepostRequest{
				OriginalContentID:       originalContentID,
				Caption:                 "repost caption",
				OriginalContentTitle:    "original post",
				OriginalContentImageURL: "https://example.com/image.jpg",
			}

			content, err := service.CreateRepost(ctx, tx, callerID, req)

			if err == nil {
				t.Fatal("CreateRepost unexpectedly succeeded for non-active target author")
			}
			if !strings.Contains(err.Error(), tc.errorFragment) {
				t.Fatalf("CreateRepost error = %v; want fragment %q", err, tc.errorFragment)
			}
			if repo.createCalls != 0 {
				t.Fatalf("CreateRepost write count = %d, want 0", repo.createCalls)
			}
			if content != nil {
				t.Fatal("expected repost content to be nil on validation error")
			}
		})
	}
}

func TestCreateRepost_AllowsActiveContentWithoutType(t *testing.T) {
	ctx := context.Background()
	callerID := uuid.New()
	originalContentID := uuid.New()
	originalAuthorID := uuid.New()
	tx := &repostGateTx{
		authorStatus: map[uuid.UUID]string{
			originalAuthorID: "active",
		},
	}

	service, repo := newRepostGateService(&entity.Content{
		ID:        originalContentID,
		AuthorID:  originalAuthorID,
		Status:    entity.StatusActive,
		Caption:   ptrString("original request"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	req := &CreateRepostRequest{
		OriginalContentID:       originalContentID,
		Caption:                 "repost caption",
		OriginalContentTitle:    "original request",
		OriginalContentImageURL: "https://example.com/image.jpg",
	}

	content, err := service.CreateRepost(ctx, tx, callerID, req)

	if err != nil {
		t.Fatalf("CreateRepost unexpectedly failed for active request: %v", err)
	}
	if repo.createCalls != 1 {
		t.Fatalf("CreateRepost write count = %d, want 1", repo.createCalls)
	}
	if content == nil || !content.IsRepost() {
		t.Fatal("expected repost content to be created")
	}
}

func TestCreateRepost_RejectsDeletedRequest(t *testing.T) {
	ctx := context.Background()
	tx := &repostGateTx{}
	callerID := uuid.New()
	originalContentID := uuid.New()

	service, repo := newRepostGateService(&entity.Content{
		ID:        originalContentID,
		AuthorID:  uuid.New(),
		Status:    entity.StatusDeleted,
		Caption:   ptrString("deleted request"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	req := &CreateRepostRequest{
		OriginalContentID:       originalContentID,
		Caption:                 "repost caption",
		OriginalContentTitle:    "deleted request",
		OriginalContentImageURL: "https://example.com/image.jpg",
	}

	content, err := service.CreateRepost(ctx, tx, callerID, req)

	if err == nil {
		t.Fatal("CreateRepost should reject deleted requests")
	}
	if got := err.Error(); got == "" || (!contains(got, "deleted content") && !contains(got, "content not found")) {
		t.Fatalf("error = %q, want deleted-content or not-found rejection", got)
	}
	if repo.createCalls != 0 {
		t.Fatalf("CreateRepost write count = %d, want 0", repo.createCalls)
	}
	if content != nil {
		t.Fatal("expected no content to be created for deleted request")
	}
}

func TestGetContentPublic_EnforcesTargetAuthorLifecycle(t *testing.T) {
	cases := []struct {
		name          string
		targetStatus  string
		targetDeleted bool
		wantSuccess   bool
	}{
		{name: "suspended", targetStatus: "suspended"},
		{name: "banned", targetStatus: "banned"},
		{name: "soft-deleted", targetStatus: "active", targetDeleted: true},
		{name: "active", targetStatus: "active", wantSuccess: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			currentID := uuid.New()
			targetID := uuid.New()
			currentAuthorID := uuid.New()
			targetAuthorID := uuid.New()

			current := &entity.Content{
				ID:               currentID,
				AuthorID:         currentAuthorID,
				Status:           entity.StatusActive,
				OriginalAuthorID: &targetAuthorID,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}

			target := &entity.Content{
				ID:        targetID,
				AuthorID:  targetAuthorID,
				Status:    entity.StatusActive,
				Caption:   ptrString("target"),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if tc.targetDeleted {
				target.Status = entity.StatusDeleted
				target.DeletedAt = ptrTime(time.Now())
			}

			repo := &repostGateRepo{
				contentByID: map[uuid.UUID]*entity.Content{
					currentID: current,
					targetID:  target,
				},
				occurrences: map[uuid.UUID]*entity.ContentResourceOccurrence{
					currentID: entity.NewContentResourceOccurrence(currentID, currentAuthorID, &entity.ContentResourceOccurrenceIdentity{
						Operation:    entity.ContentResourceOccurrenceOperationShareToFeed,
						ResourceType: entity.ContentResourceOccurrenceResourceTypeContent,
						ResourceID:   targetID,
					}),
				},
			}
			service := newRepostGateServiceWithRepo(repo)
			tx := &repostGateTx{
				authorStatus: map[uuid.UUID]string{
					currentAuthorID: "active",
					targetAuthorID:  tc.targetStatus,
				},
				authorDeleted: map[uuid.UUID]bool{
					currentAuthorID: false,
					targetAuthorID:  tc.targetDeleted,
				},
			}

			got, err := service.GetContentPublic(ctx, tx, currentID)
			if tc.wantSuccess {
				if err != nil {
					t.Fatalf("GetContentPublic unexpectedly failed: %v", err)
				}
				if got == nil || got.ID != currentID {
					t.Fatalf("GetContentPublic returned %#v, want current content", got)
				}
				return
			}

			if err == nil {
				t.Fatal("GetContentPublic unexpectedly succeeded for non-active target author")
			}
			if !strings.Contains(err.Error(), "content not found") {
				t.Fatalf("GetContentPublic error = %v; want content-not-found response", err)
			}
			if got != nil {
				t.Fatalf("GetContentPublic returned %#v; want nil", got)
			}
		})
	}
}

// SC-4: Repost of hidden (admin-moderated) content must be rejected at write-time.
func TestCreateRepost_RejectsHiddenPost(t *testing.T) {
	ctx := context.Background()
	tx := &repostGateTx{}
	callerID := uuid.New()
	originalContentID := uuid.New()

	service, repo := newRepostGateService(&entity.Content{
		ID:        originalContentID,
		AuthorID:  uuid.New(),
		Status:    entity.StatusActive,
		IsHidden:  true, // moderated/admin-hidden
		Caption:   ptrString("hidden post"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	req := &CreateRepostRequest{
		OriginalContentID:       originalContentID,
		Caption:                 "repost caption",
		OriginalContentTitle:    "hidden post",
		OriginalContentImageURL: "https://example.com/image.jpg",
	}

	content, err := service.CreateRepost(ctx, tx, callerID, req)

	if err == nil {
		t.Fatal("CreateRepost should reject hidden (moderated) content")
	}
	if repo.createCalls != 0 {
		t.Fatalf("CreateRepost write count = %d, want 0 for hidden content", repo.createCalls)
	}
	if content != nil {
		t.Fatal("expected no content to be created for hidden target")
	}
}

// SC-4: Repost of hidden request must also be rejected (not just posts).
func TestCreateRepost_RejectsHiddenRequest(t *testing.T) {
	ctx := context.Background()
	tx := &repostGateTx{}
	callerID := uuid.New()
	originalContentID := uuid.New()

	service, repo := newRepostGateService(&entity.Content{
		ID:        originalContentID,
		AuthorID:  uuid.New(),
		Status:    entity.StatusActive,
		IsHidden:  true,
		Caption:   ptrString("hidden request"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	req := &CreateRepostRequest{
		OriginalContentID:       originalContentID,
		Caption:                 "repost caption",
		OriginalContentTitle:    "hidden request",
		OriginalContentImageURL: "https://example.com/image.jpg",
	}

	content, err := service.CreateRepost(ctx, tx, callerID, req)

	if err == nil {
		t.Fatal("CreateRepost should reject hidden (moderated) request")
	}
	if repo.createCalls != 0 {
		t.Fatalf("CreateRepost write count = %d, want 0 for hidden request", repo.createCalls)
	}
	if content != nil {
		t.Fatal("expected no content to be created for hidden request")
	}
}

func ptrString(v string) *string {
	return &v
}

func ptrTime(v time.Time) *time.Time {
	return &v
}

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})())
}
