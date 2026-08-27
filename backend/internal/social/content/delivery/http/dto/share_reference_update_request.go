package dto

import (
	"bytes"
	"encoding/json"
)

// NullableShareReferenceRequest preserves omitted vs explicit null semantics
// for content update requests.
//
// - Set=false: share_reference was omitted from the JSON body.
// - Set=true, Value=nil: share_reference was explicitly set to null.
// - Set=true, Value!=nil: share_reference carried a concrete object payload.
type NullableShareReferenceRequest struct {
	Set   bool
	Value *ShareReferenceRequest
}

// UnmarshalJSON captures presence even when the JSON value is null.
func (r *NullableShareReferenceRequest) UnmarshalJSON(data []byte) error {
	r.Set = true

	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		r.Value = nil
		return nil
	}

	var value ShareReferenceRequest
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	r.Value = &value
	return nil
}
