package entity

// ExternalProductMediaType identifies the kind of uploaded asset.
type ExternalProductMediaType string

const (
	ExternalProductMediaTypeImage ExternalProductMediaType = "image"
	ExternalProductMediaTypeVideo ExternalProductMediaType = "video"
)

// IsValid returns true if the media type is canonical.
func (t ExternalProductMediaType) IsValid() bool {
	switch t {
	case ExternalProductMediaTypeImage, ExternalProductMediaTypeVideo:
		return true
	default:
		return false
	}
}

// String returns the string representation.
func (t ExternalProductMediaType) String() string {
	return string(t)
}
