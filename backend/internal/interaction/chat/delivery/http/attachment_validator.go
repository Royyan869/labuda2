package http

import (
	"errors"

	chatvalidator "github.com/labuda/backend/internal/interaction/chat/attachmentvalidator"
)

// AttachmentValidationError represents an attachment validation error.
type AttachmentValidationError = chatvalidator.ValidationError

// ValidateAttachmentJSON validates attachment JSON against strict schema.
func ValidateAttachmentJSON(attachmentJSON map[string]interface{}) []AttachmentValidationError {
	return chatvalidator.ValidateAttachmentJSON(attachmentJSON)
}

// HasValidationErrors returns true when there are validation errors.
func HasValidationErrors(errs []AttachmentValidationError) bool {
	return chatvalidator.HasValidationErrors(errs)
}

// ErrInvalidAttachment is returned when attachment validation fails.
var ErrInvalidAttachment = errors.New("invalid attachment structure")


