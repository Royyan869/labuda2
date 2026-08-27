package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalProductDraftValidation(t *testing.T) {
	dbTime := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	ownerID := uuid.New()

	product, err := NewExternalProductDraft(ownerID, "Fresh Fish", ptrString("Ocean caught"), "https://example.com/fish", dbTime)
	require.NoError(t, err)
	require.NotNil(t, product)

	assert.Equal(t, ownerID, product.OwnerUserID)
	assert.Equal(t, "Fresh Fish", product.Title)
	assert.Equal(t, ExternalProductReviewStatusDraft, product.ReviewStatus)
	assert.True(t, product.IsEditableByOwner())
	assert.True(t, product.CanSubmit())
	assert.False(t, product.CanResubmit())
	assert.False(t, product.IsPubliclyEligibleReviewStatus())
}

func TestExternalProductValidationHelpers(t *testing.T) {
	title, err := ValidateTitle("  Great Product  ")
	require.NoError(t, err)
	assert.Equal(t, "Great Product", title)

	_, err = ValidateTitle("")
	require.Error(t, err)

	normalized, err := ValidateExternalURL("HTTPS://Example.com/path?q=1")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/path?q=1", normalized)

	_, err = ValidateExternalURL("ftp://example.com")
	require.Error(t, err)

	assert.NoError(t, ValidateMediaType(ExternalProductMediaTypeImage))
	assert.NoError(t, ValidateMediaType(ExternalProductMediaTypeVideo))
	assert.Error(t, ValidateMediaType(ExternalProductMediaType("audio")))
}

func TestExternalProductLifecycleTransitions(t *testing.T) {
	dbTime := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	ownerID := uuid.New()
	product, err := NewExternalProductDraft(ownerID, "Product", nil, "https://example.com/product", dbTime)
	require.NoError(t, err)

	require.NoError(t, product.Submit(dbTime))
	assert.Equal(t, ExternalProductReviewStatusPendingReview, product.ReviewStatus)
	assert.Equal(t, dbTime, *product.SubmittedAt)
	assert.False(t, product.CanSubmit())

	product.ReviewStatus = ExternalProductReviewStatusRejected
	require.NoError(t, product.Resubmit(dbTime))
	assert.Equal(t, ExternalProductReviewStatusPendingReview, product.ReviewStatus)

	product.ReviewStatus = ExternalProductReviewStatusApproved
	require.NoError(t, product.ApplyOwnerUpdate(ExternalProductUpdateInput{
		Title: ptrString("Updated"),
	}, dbTime))
	assert.Equal(t, ExternalProductReviewStatusPendingReview, product.ReviewStatus)
	assert.Equal(t, "Updated", product.Title)

	product.ReviewStatus = ExternalProductReviewStatusHidden
	assert.False(t, product.CanResubmit())
	assert.Error(t, product.Resubmit(dbTime))
}

func TestExternalProductMediaValidation(t *testing.T) {
	dbTime := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	productID := uuid.New()
	media, err := NewExternalProductMedia(
		productID,
		ExternalProductMediaTypeImage,
		"s3://bucket/key",
		"https://cdn.example.com/key.jpg",
		ptrString("https://cdn.example.com/thumb.jpg"),
		1,
		nil,
		dbTime,
	)
	require.NoError(t, err)
	assert.Equal(t, productID, media.ExternalProductID)
	assert.Equal(t, ExternalProductMediaTypeImage, media.MediaType)

	_, err = NewExternalProductMedia(uuid.Nil, ExternalProductMediaTypeImage, "key", "https://example.com", nil, 0, nil, dbTime)
	require.Error(t, err)
}

func TestExternalProductReviewHistoryValidation(t *testing.T) {
	dbTime := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	productID := uuid.New()
	adminID := uuid.New()
	from := ExternalProductReviewStatusDraft

	history, err := NewExternalProductReviewHistory(productID, &from, ExternalProductReviewStatusPendingReview, ptrString("submit"), &adminID, nil, dbTime)
	require.NoError(t, err)
	assert.Equal(t, productID, history.ExternalProductID)

	_, err = NewExternalProductReviewHistory(uuid.Nil, nil, ExternalProductReviewStatusPendingReview, nil, &adminID, nil, dbTime)
	require.Error(t, err)
}

func ptrString(v string) *string {
	return &v
}
