package mediaref

// Media processing status vocabulary — one item, one state.
//
// Lifecycle (video): a video row is born `processing` at create time (the
// file exists in S3 but the remux Lambda may not have produced the poster
// frame yet) and the media-readiness worker flips it to `ready` once the
// poster HEADs 200, or `failed` past the timeout. Images are born `ready`.
// Legacy rows without a status read as `ready` (NormalizeStatus).
//
// Clients render: ready = normal; processing = poster/static mat with no
// error styling and no retry affordance; failed = error mat with
// replace affordance. A processing item must never look broken.
type MediaStatus string

const (
	MediaStatusProcessing MediaStatus = "processing"
	MediaStatusReady      MediaStatus = "ready"
	MediaStatusFailed     MediaStatus = "failed"
)

// NormalizeStatus maps any persisted status (including "" from legacy rows)
// onto the vocabulary. Unknown values fail closed to processing — never
// present an unverified item as ready, never condemn it as failed.
func NormalizeStatus(s string) MediaStatus {
	switch MediaStatus(s) {
	case MediaStatusReady:
		return MediaStatusReady
	case MediaStatusFailed:
		return MediaStatusFailed
	default:
		return MediaStatusProcessing
	}
}

// StatusForNewRow is the write-path rule: videos start processing, images
// start ready. One function so content / comment / commerce agree.
func StatusForNewRow(mediaType string) MediaStatus {
	if mediaType == "video" {
		return MediaStatusProcessing
	}
	return MediaStatusReady
}
