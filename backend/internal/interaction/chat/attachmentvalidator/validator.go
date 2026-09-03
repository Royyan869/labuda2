package attachmentvalidator

import "fmt"

// Valid attachment types
var validAttachmentTypes = map[string]bool{
	// Object References
	"reference": true,

	// Workflow Payloads
	"negotiation_offer":    true,
	"negotiation_proposal": true,
	"negotiation_result":   true,
	"shipping_quote":       true,

	// True Attachments
	"location": true,
}

// ValidationError represents an attachment validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// HasValidationErrors returns true when one or more validation errors exist.
func HasValidationErrors(errs []ValidationError) bool {
	return len(errs) > 0
}

// ValidateAttachmentJSON validates attachment JSON against strict schema.
//
// Canonical schema: { "type": "...", "data": {...} }.
func ValidateAttachmentJSON(attachmentJSON map[string]interface{}) []ValidationError {
	var validationErrors []ValidationError

	if attachmentJSON == nil {
		return validationErrors // Empty attachment is valid (optional field)
	}

	attachmentTypeRaw, hasType := attachmentJSON["type"]
	if !hasType {
		return []ValidationError{{Field: "type", Message: "Attachment type is required"}}
	}
	attachmentType, ok := attachmentTypeRaw.(string)
	if !ok {
		return []ValidationError{{Field: "type", Message: "Attachment type must be a string"}}
	}
	if !validAttachmentTypes[attachmentType] {
		return []ValidationError{{Field: "type", Message: fmt.Sprintf("Invalid attachment type: %s", attachmentType)}}
	}

	dataRaw, hasData := attachmentJSON["data"]
	if !hasData {
		return []ValidationError{{Field: "data", Message: "Attachment data is required"}}
	}
	data, ok := dataRaw.(map[string]interface{})
	if !ok {
		return []ValidationError{{Field: "data", Message: "Attachment data must be an object"}}
	}

	for key := range attachmentJSON {
		if key != "type" && key != "data" {
			validationErrors = append(validationErrors, ValidationError{
				Field:   key,
				Message: fmt.Sprintf("Unknown field: %s", key),
			})
		}
	}

	switch attachmentType {
	case "reference":
		validationErrors = append(validationErrors, validateReferenceAttachmentV2(data)...)
	case "negotiation_offer":
		validationErrors = append(validationErrors, validateNegotiationOfferAttachmentV2(data)...)
	case "negotiation_proposal":
		validationErrors = append(validationErrors, validateNegotiationProposalAttachmentV2(data)...)
	case "negotiation_result":
		validationErrors = append(validationErrors, validateNegotiationResultAttachmentV2(data)...)
	case "shipping_quote":
		validationErrors = append(validationErrors, validateShippingQuoteAttachmentV2(data)...)
	case "location":
		validationErrors = append(validationErrors, validateLocationAttachmentV2(data)...)
	}

	return validationErrors
}

func validateReferenceAttachmentV2(data map[string]interface{}) []ValidationError {
	var errs []ValidationError
	allowedFields := map[string]bool{
		"target_type": true,
		"target_id":   true,
		"preview":     true,
	}
	for key := range data {
		if !allowedFields[key] {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("data.%s", key), Message: fmt.Sprintf("Unknown field: %s", key)})
		}
	}
	targetTypeRaw, hasTargetType := data["target_type"]
	if !hasTargetType {
		errs = append(errs, ValidationError{Field: "data.target_type", Message: "target_type is required"})
	} else {
		targetType, ok := targetTypeRaw.(string)
		if !ok {
			errs = append(errs, ValidationError{Field: "data.target_type", Message: "target_type must be a string"})
		} else {
			validTargetTypes := map[string]bool{"for_sale": true, "auction": true, "post": true, "request": true, "profile": true}
			if !validTargetTypes[targetType] {
				errs = append(errs, ValidationError{Field: "data.target_type", Message: fmt.Sprintf("Invalid target_type: %s", targetType)})
			}
		}
	}
	if _, ok := data["target_id"]; !ok {
		errs = append(errs, ValidationError{Field: "data.target_id", Message: "target_id is required"})
	}
	previewRaw, hasPreview := data["preview"]
	if !hasPreview {
		errs = append(errs, ValidationError{Field: "data.preview", Message: "preview is required"})
		return errs
	}
	preview, ok := previewRaw.(map[string]interface{})
	if !ok {
		errs = append(errs, ValidationError{Field: "data.preview", Message: "preview must be an object"})
		return errs
	}
	previewAllowedFields := map[string]bool{
		"title": true, "imageUrl": true, "isAvailable": true, "isSold": true, "isClosed": true, "isDeleted": true,
	}
	for key := range preview {
		if !previewAllowedFields[key] {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("data.preview.%s", key), Message: fmt.Sprintf("Unknown field: %s", key)})
		}
	}
	if _, hasTitle := preview["title"]; !hasTitle {
		errs = append(errs, ValidationError{Field: "data.preview.title", Message: "preview.title is required"})
	}
	return errs
}

func validateNegotiationOfferAttachmentV2(data map[string]interface{}) []ValidationError {
	var errs []ValidationError
	allowedFields := map[string]bool{"negotiation_id": true, "for_sale_id": true, "status": true, "preview": true}
	for key := range data {
		if !allowedFields[key] {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("data.%s", key), Message: fmt.Sprintf("Unknown field: %s", key)})
		}
	}
	if _, ok := data["negotiation_id"]; !ok {
		errs = append(errs, ValidationError{Field: "data.negotiation_id", Message: "negotiation_id is required"})
	}
	if _, ok := data["for_sale_id"]; !ok {
		errs = append(errs, ValidationError{Field: "data.for_sale_id", Message: "for_sale_id is required"})
	}
	if _, ok := data["status"]; !ok {
		errs = append(errs, ValidationError{Field: "data.status", Message: "status is required"})
	}
	previewRaw, hasPreview := data["preview"]
	if !hasPreview {
		errs = append(errs, ValidationError{Field: "data.preview", Message: "preview is required"})
		return errs
	}
	preview, ok := previewRaw.(map[string]interface{})
	if !ok {
		errs = append(errs, ValidationError{Field: "data.preview", Message: "preview must be an object"})
		return errs
	}
	previewAllowedFields := map[string]bool{"title": true, "imageUrl": true}
	for key := range preview {
		if !previewAllowedFields[key] {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("data.preview.%s", key), Message: fmt.Sprintf("Unknown field: %s", key)})
		}
	}
	if _, hasTitle := preview["title"]; !hasTitle {
		errs = append(errs, ValidationError{Field: "data.preview.title", Message: "preview.title is required"})
	}
	return errs
}

func validateNegotiationProposalAttachmentV2(data map[string]interface{}) []ValidationError {
	var errs []ValidationError
	allowedFields := map[string]bool{
		"session_id": true, "proposal_sequence": true, "price": true, "resource_type": true, "resource_id": true, "note": true,
	}
	for key := range data {
		if !allowedFields[key] {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("data.%s", key), Message: fmt.Sprintf("Unknown field: %s", key)})
		}
	}
	if _, ok := data["session_id"]; !ok {
		errs = append(errs, ValidationError{Field: "data.session_id", Message: "session_id is required"})
	}
	if _, ok := data["proposal_sequence"]; !ok {
		errs = append(errs, ValidationError{Field: "data.proposal_sequence", Message: "proposal_sequence is required"})
	}
	if _, ok := data["price"]; !ok {
		errs = append(errs, ValidationError{Field: "data.price", Message: "price is required"})
	}
	return errs
}

func validateNegotiationResultAttachmentV2(data map[string]interface{}) []ValidationError {
	var errs []ValidationError
	allowedFields := map[string]bool{"negotiation_id": true, "for_sale_id": true, "status": true, "preview": true}
	for key := range data {
		if !allowedFields[key] {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("data.%s", key), Message: fmt.Sprintf("Unknown field: %s", key)})
		}
	}
	if _, ok := data["negotiation_id"]; !ok {
		errs = append(errs, ValidationError{Field: "data.negotiation_id", Message: "negotiation_id is required"})
	}
	if _, ok := data["for_sale_id"]; !ok {
		errs = append(errs, ValidationError{Field: "data.for_sale_id", Message: "for_sale_id is required"})
	}
	if _, ok := data["status"]; !ok {
		errs = append(errs, ValidationError{Field: "data.status", Message: "status is required"})
	}
	previewRaw, hasPreview := data["preview"]
	if !hasPreview {
		errs = append(errs, ValidationError{Field: "data.preview", Message: "preview is required"})
		return errs
	}
	preview, ok := previewRaw.(map[string]interface{})
	if !ok {
		errs = append(errs, ValidationError{Field: "data.preview", Message: "preview must be an object"})
		return errs
	}
	previewAllowedFields := map[string]bool{"title": true, "imageUrl": true}
	for key := range preview {
		if !previewAllowedFields[key] {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("data.preview.%s", key), Message: fmt.Sprintf("Unknown field: %s", key)})
		}
	}
	if _, hasTitle := preview["title"]; !hasTitle {
		errs = append(errs, ValidationError{Field: "data.preview.title", Message: "preview.title is required"})
	}
	return errs
}

func validateShippingQuoteAttachmentV2(data map[string]interface{}) []ValidationError {
	var errs []ValidationError
	allowedFields := map[string]bool{
		"offer_id": true, "linked_item_id": true, "linked_item_type": true, "linked_item_name": true,
		"linked_item_image": true, "linked_item_price": true, "linked_item_buy_now_price": true,
		"for_sale_id": true, "auction_id": true, "shipping_type": true, "shipping_type_name": true,
		"shipping_type_emoji": true, "rate": true,
		"notes": true, "valid_until": true, "status": true, "seller_id": true,
	}
	for key := range data {
		if !allowedFields[key] {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("data.%s", key), Message: fmt.Sprintf("Unknown field: %s", key)})
		}
	}
	if _, ok := data["offer_id"]; !ok {
		errs = append(errs, ValidationError{Field: "data.offer_id", Message: "offer_id is required"})
	}
	if _, ok := data["linked_item_id"]; !ok {
		errs = append(errs, ValidationError{Field: "data.linked_item_id", Message: "linked_item_id is required"})
	}
	if _, ok := data["linked_item_type"]; !ok {
		errs = append(errs, ValidationError{Field: "data.linked_item_type", Message: "linked_item_type is required"})
	}
	return errs
}

func validateLocationAttachmentV2(data map[string]interface{}) []ValidationError {
	var errs []ValidationError
	allowedFields := map[string]bool{"latitude": true, "longitude": true, "placeName": true, "address": true}
	for key := range data {
		if !allowedFields[key] {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("data.%s", key), Message: fmt.Sprintf("Unknown field: %s", key)})
		}
	}
	if _, ok := data["latitude"]; !ok {
		errs = append(errs, ValidationError{Field: "data.latitude", Message: "latitude is required"})
	}
	if _, ok := data["longitude"]; !ok {
		errs = append(errs, ValidationError{Field: "data.longitude", Message: "longitude is required"})
	}
	return errs
}


