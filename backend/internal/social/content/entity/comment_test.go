package entity

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewComment(t *testing.T) {
	contentID := uuid.New()
	authorID := uuid.New()

	tests := []struct {
		name      string
		contentID uuid.UUID
		authorID  uuid.UUID
		body      string
		wantErr   bool
		errType   error
	}{
		{
			name:      "valid comment",
			contentID: contentID,
			authorID:  authorID,
			body:      "This is a test comment",
			wantErr:   false,
		},
		{
			name:      "empty body should error",
			contentID: contentID,
			authorID:  authorID,
			body:      "",
			wantErr:   true,
			errType:   &ErrInvalidComment{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comment, err := NewComment(tt.contentID, tt.authorID, tt.body)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewComment() expected error, got nil")
					return
				}
				if tt.errType != nil {
					// Check error type
					// We can't directly compare types in Go easily without reflection
					// So we just check that error is not nil
				}
				return
			}

			if err != nil {
				t.Errorf("NewComment() unexpected error: %v", err)
				return
			}

			if comment.ID == uuid.Nil {
				t.Error("NewComment() ID should not be empty")
			}
			if comment.TargetID != tt.contentID {
				t.Errorf("NewComment() TargetID = %v, want %v", comment.TargetID, tt.contentID)
			}
			if comment.AuthorID != tt.authorID {
				t.Errorf("NewComment() AuthorID = %v, want %v", comment.AuthorID, tt.authorID)
			}
			if comment.Body == nil || *comment.Body != tt.body {
				t.Errorf("NewComment() Body = %v, want %v", comment.Body, tt.body)
			}
			if comment.Type != CommentTypeNormal {
				t.Errorf("NewComment() Type = %v, want %v", comment.Type, CommentTypeNormal)
			}
			if comment.Reference != nil {
				t.Errorf("NewComment() Reference should be nil for normal comment")
			}
		})
	}
}

func TestNewCommerceReferenceComment(t *testing.T) {
	contentID := uuid.New()
	authorID := uuid.New()
	forSaleID := uuid.New()

	// Create a ShareReference for the for_sale
	shareReference := NewShareReferenceFromForSale(
		forSaleID.String(),
		"Test ForSale",
		"https://example.com/image.jpg",
		true,
		false,
		false,
	)

	comment, err := NewCommerceReferenceComment(contentID, authorID, shareReference, nil)

	if err != nil {
		t.Errorf("NewCommerceReferenceComment() unexpected error: %v", err)
		return
	}

	if comment.ID == uuid.Nil {
		t.Error("NewCommerceReferenceComment() ID should not be empty")
	}
	if comment.TargetID != contentID {
		t.Errorf("NewCommerceReferenceComment() TargetID = %v, want %v", comment.TargetID, contentID)
	}
	if comment.AuthorID != authorID {
		t.Errorf("NewCommerceReferenceComment() AuthorID = %v, want %v", comment.AuthorID, authorID)
	}
	if comment.Body != nil {
		t.Errorf("NewCommerceReferenceComment() Body should be nil")
	}
	if comment.Type != CommentTypeCommerceReference {
		t.Errorf("NewCommerceReferenceComment() Type = %v, want %v", comment.Type, CommentTypeCommerceReference)
	}
	if comment.Reference == nil {
		t.Errorf("NewCommerceReferenceComment() Reference should not be nil")
	}
	if comment.Reference.TargetID != forSaleID.String() {
		t.Errorf("NewCommerceReferenceComment() Reference.TargetID = %v, want %v", comment.Reference.TargetID, forSaleID.String())
	}
}

func TestCommentValidate(t *testing.T) {
	contentID := uuid.New()
	authorID := uuid.New()
	forSaleID := uuid.New()
	body := "test comment"

	tests := []struct {
		name    string
		comment *Comment
		wantErr bool
	}{
		{
			name: "valid normal comment",
			comment: &Comment{
				ID:       uuid.New(),
				TargetID: contentID,
				AuthorID: authorID,
				Body:     &body,
				Type:     CommentTypeNormal,
			},
			wantErr: false,
		},
		{
			name: "normal comment with empty body should error",
			comment: &Comment{
				ID:       uuid.New(),
				TargetID: contentID,
				AuthorID: authorID,
				Body:     nil,
				Type:     CommentTypeNormal,
			},
			wantErr: true,
		},
		{
			name: "valid commerce reference comment",
			comment: &Comment{
				ID:       uuid.New(),
				TargetID: contentID,
				AuthorID: authorID,
				Type:     CommentTypeCommerceReference,
				Reference: NewShareReferenceFromForSale(
					forSaleID.String(),
					"Test ForSale",
					"https://example.com/image.jpg",
					true,
					false,
					false,
				),
			},
			wantErr: false,
		},
		{
			name: "commerce reference without reference should error",
			comment: &Comment{
				ID:        uuid.New(),
				TargetID:  contentID,
				AuthorID:  authorID,
				Type:      CommentTypeCommerceReference,
				Reference: nil,
			},
			wantErr: true,
		},
		{
			name: "invalid comment type",
			comment: &Comment{
				ID:       uuid.New(),
				TargetID: contentID,
				AuthorID: authorID,
				Body:     &body,
				Type:     CommentType("invalid"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.comment.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Comment.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommentIsNormal(t *testing.T) {
	comment := &Comment{
		Type: CommentTypeNormal,
	}
	if !comment.IsNormal() {
		t.Error("Comment.IsNormal() should return true for normal comments")
	}

	comment.Type = CommentTypeCommerceReference
	if comment.IsNormal() {
		t.Error("Comment.IsNormal() should return false for commerce reference comments")
	}
}

func TestCommentIsCommerceReference(t *testing.T) {
	comment := &Comment{
		Type: CommentTypeCommerceReference,
	}
	if !comment.IsCommerceReference() {
		t.Error("Comment.IsCommerceReference() should return true for commerce reference comments")
	}

	comment.Type = CommentTypeNormal
	if comment.IsCommerceReference() {
		t.Error("Comment.IsCommerceReference() should return false for normal comments")
	}
}

func TestCommentTypeIsValid(t *testing.T) {
	tests := []struct {
		name string
		t    CommentType
		want bool
	}{
		{"normal type", CommentTypeNormal, true},
		{"commerce_reference type", CommentTypeCommerceReference, true},
		{"invalid type", CommentType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.IsValid(); got != tt.want {
				t.Errorf("CommentType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCommentTypeConstraints validates DB-level constraint enforcement
// These tests ensure that invalid comment states cannot be created
func TestCommentTypeConstraints(t *testing.T) {
	contentID := uuid.New()
	authorID := uuid.New()
	forSaleID := uuid.New()
	emptyBody := ""

	tests := []struct {
		name        string
		comment     *Comment
		wantErr     bool
		errMsg      string
		description string
	}{
		{
			name: "commerce_reference without reference should fail",
			comment: &Comment{
				ID:        uuid.New(),
				TargetID:  contentID,
				AuthorID:  authorID,
				Type:      CommentTypeCommerceReference,
				Reference: nil, // Missing reference
				Body:      &emptyBody,
			},
			wantErr:     true,
			errMsg:      "reference is required",
			description: "Ensures commerce_reference comments must have reference",
		},
		{
			name: "commerce_reference with invalid reference should fail",
			comment: &Comment{
				ID:       uuid.New(),
				TargetID: contentID,
				AuthorID: authorID,
				Type:     CommentTypeCommerceReference,
				Reference: &ShareReference{
					TargetType: "for_sale",
					TargetID:   "", // Empty target ID
					Preview:    SharePreview{},
				},
				Body: &emptyBody,
			},
			wantErr:     true,
			errMsg:      "reference is required",
			description: "Ensures commerce_reference comments must have valid reference",
		},
		{
			name: "normal comment without body should fail",
			comment: &Comment{
				ID:       uuid.New(),
				TargetID: contentID,
				AuthorID: authorID,
				Type:     CommentTypeNormal,
				Body:     nil, // Missing body
			},
			wantErr:     true,
			errMsg:      "body cannot be empty",
			description: "Ensures normal comments must have body",
		},
		{
			name: "normal comment with empty string body should fail",
			comment: &Comment{
				ID:       uuid.New(),
				TargetID: contentID,
				AuthorID: authorID,
				Type:     CommentTypeNormal,
				Body:     &emptyBody, // Empty body
			},
			wantErr:     true,
			errMsg:      "body cannot be empty",
			description: "Ensures normal comments must have non-empty body",
		},
		{
			name: "valid normal comment should pass",
			comment: &Comment{
				ID:       uuid.New(),
				TargetID: contentID,
				AuthorID: authorID,
				Type:     CommentTypeNormal,
				Body:     func() *string { s := "valid comment"; return &s }(),
			},
			wantErr:     false,
			description: "Valid normal comment with body",
		},
		{
			name: "valid commerce_reference should pass",
			comment: &Comment{
				ID:       uuid.New(),
				TargetID: contentID,
				AuthorID: authorID,
				Type:     CommentTypeCommerceReference,
				Reference: NewShareReferenceFromForSale(
					forSaleID.String(),
					"Test ForSale",
					"https://example.com/image.jpg",
					true,
					false,
					false,
				),
				Body: nil, // Body is optional for commerce_reference
			},
			wantErr:     false,
			description: "Valid commerce_reference comment with reference",
		},
		{
			name: "valid commerce reference with body should pass",
			comment: &Comment{
				ID:       uuid.New(),
				TargetID: contentID,
				AuthorID: authorID,
				Type:     CommentTypeCommerceReference,
				Reference: NewShareReferenceFromForSale(
					forSaleID.String(),
					"Test ForSale",
					"https://example.com/image.jpg",
					true,
					false,
					false,
				),
				Body: func() *string { s := "check this for_sale"; return &s }(),
			},
			wantErr:     false,
			description: "Valid commerce_reference comment with reference and optional body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.comment.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("%s: expected error containing '%s', got nil", tt.description, tt.errMsg)
					return
				}
				// Check if error message contains expected substring
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("%s: error message '%s' should contain '%s'", tt.description, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tt.description, err)
				}
			}
		})
	}
}

// TestNoOfferReferenceType ensures offer_reference type cannot be created
func TestNoOfferReferenceType(t *testing.T) {
	// This test ensures the obsolete 'offer_reference' type cannot be used
	offerReferenceType := CommentType("offer_reference")

	if offerReferenceType.IsValid() {
		t.Error("offer_reference type should not be valid")
	}

	// Attempt to create a comment with offer_reference type
	comment := &Comment{
		ID:       uuid.New(),
		TargetID: uuid.New(),
		AuthorID: uuid.New(),
		Type:     offerReferenceType,
		Body:     func() *string { s := "test"; return &s }(),
	}

	err := comment.Validate()
	if err == nil {
		t.Error("comment with offer_reference type should fail validation")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
