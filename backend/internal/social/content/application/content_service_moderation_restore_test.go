package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
)

type moderationRestoreRepo struct {
	content          *entity.Content
	getForUpdateHits int
	updateHits       int
	updatedContent   *entity.Content
}

var _ contentrepo.ContentRepository = (*moderationRestoreRepo)(nil)

func (r *moderationRestoreRepo) Create(ctx context.Context, tx interface{}, content *entity.Content) error {
	return nil
}

func (r *moderationRestoreRepo) CreateMedia(ctx context.Context, tx interface{}, media []*entity.ContentMedia) error {
	return nil
}

func (r *moderationRestoreRepo) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
	if r.content == nil {
		return nil, errors.New("content not found")
	}
	return r.content, nil
}

func (r *moderationRestoreRepo) GetForUpdate(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Content, error) {
	r.getForUpdateHits++
	if r.content == nil {
		return nil, errors.New("content not found")
	}
	return r.content, nil
}

func (r *moderationRestoreRepo) Update(ctx context.Context, tx interface{}, content *entity.Content) error {
	r.updateHits++
	r.updatedContent = content
	return nil
}

func (r *moderationRestoreRepo) ListByAuthor(ctx context.Context, tx interface{}, authorID uuid.UUID, viewerID uuid.UUID, limit int, cursor string) ([]*entity.Content, string, error) {
	return nil, "", nil
}

func (r *moderationRestoreRepo) GetMedia(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]*entity.ContentMedia, error) {
	return nil, nil
}

func (r *moderationRestoreRepo) GetTagsByContentID(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]string, error) {
	return []string{}, nil
}

func (r *moderationRestoreRepo) InsertTags(ctx context.Context, tx interface{}, contentID uuid.UUID, tags []string) error {
	return nil
}

func (r *moderationRestoreRepo) InsertMentionedUsers(ctx context.Context, tx interface{}, contentID uuid.UUID, userIDs []uuid.UUID) error {
	return nil
}

func (r *moderationRestoreRepo) GetMentionedUserIDs(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]uuid.UUID, error) {
	return []uuid.UUID{}, nil
}

func newModerationRestoreService(content *entity.Content) (*ContentService, *moderationRestoreRepo) {
	repo := &moderationRestoreRepo{content: content}
	return &ContentService{contentRepo: repo}, repo
}

func TestSoftDeleteForModeration_SetsDeletedState(t *testing.T) {
	ownerID := uuid.New()
	content := &entity.Content{
		ID:       uuid.New(),
		AuthorID: ownerID,
		Status:   entity.StatusActive,
	}

	svc, repo := newModerationRestoreService(content)

	if err := svc.SoftDeleteForModeration(context.Background(), nil, content.ID); err != nil {
		t.Fatalf("SoftDeleteForModeration() error = %v", err)
	}
	if repo.getForUpdateHits != 1 {
		t.Fatalf("GetForUpdate hits = %d; want 1", repo.getForUpdateHits)
	}
	if repo.updateHits != 1 {
		t.Fatalf("Update hits = %d; want 1", repo.updateHits)
	}
	if content.Status != entity.StatusDeleted {
		t.Fatalf("Status = %s; want %s", content.Status, entity.StatusDeleted)
	}
	if content.DeletedAt == nil {
		t.Fatal("DeletedAt should be set")
	}
}

func TestRestoreFromModeration_NormalContentClearsDeletedAtAndRestoresActive(t *testing.T) {
	ownerID := uuid.New()
	deletedAt := time.Now().Add(-time.Hour)
	content := &entity.Content{
		ID:        uuid.New(),
		AuthorID:  ownerID,
		Status:    entity.StatusDeleted,
		IsHidden:  false,
		DeletedAt: &deletedAt,
	}

	svc, repo := newModerationRestoreService(content)

	if err := svc.RestoreFromModeration(context.Background(), nil, content.ID); err != nil {
		t.Fatalf("RestoreFromModeration() error = %v", err)
	}
	if repo.updateHits != 1 {
		t.Fatalf("Update hits = %d; want 1", repo.updateHits)
	}
	if content.Status != entity.StatusActive {
		t.Fatalf("Status = %s; want %s", content.Status, entity.StatusActive)
	}
	if content.DeletedAt != nil {
		t.Fatal("DeletedAt should be cleared")
	}
	if content.IsHidden {
		t.Fatal("IsHidden should remain false")
	}
	if content.AuthorID != ownerID {
		t.Fatalf("AuthorID changed: got %s want %s", content.AuthorID, ownerID)
	}
}

func TestRestoreFromModeration_HiddenContentPreservesHiddenFlag(t *testing.T) {
	ownerID := uuid.New()
	deletedAt := time.Now().Add(-time.Hour)
	content := &entity.Content{
		ID:        uuid.New(),
		AuthorID:  ownerID,
		Status:    entity.StatusDeleted,
		IsHidden:  true,
		DeletedAt: &deletedAt,
	}

	svc, _ := newModerationRestoreService(content)

	if err := svc.RestoreFromModeration(context.Background(), nil, content.ID); err != nil {
		t.Fatalf("RestoreFromModeration() error = %v", err)
	}
	if !content.IsHidden {
		t.Fatal("IsHidden should remain true")
	}
}

func TestDeleteContent_UserDeleteBehaviorUnchanged(t *testing.T) {
	ownerID := uuid.New()
	content := &entity.Content{
		ID:       uuid.New(),
		AuthorID: ownerID,
		Status:   entity.StatusActive,
	}

	svc, repo := newModerationRestoreService(content)

	if err := svc.DeleteContent(context.Background(), nil, ownerID, content.ID); err != nil {
		t.Fatalf("DeleteContent() error = %v", err)
	}
	if repo.updateHits != 1 {
		t.Fatalf("Update hits = %d; want 1", repo.updateHits)
	}
	if content.Status != entity.StatusDeleted {
		t.Fatalf("Status = %s; want %s", content.Status, entity.StatusDeleted)
	}
	if content.DeletedAt == nil {
		t.Fatal("DeletedAt should be set")
	}
}
