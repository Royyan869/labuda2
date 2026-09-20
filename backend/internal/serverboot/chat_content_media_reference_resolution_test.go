package serverboot

import (
	"testing"
	"time"

	"github.com/labuda/backend/internal/platform/mediaresolve"
	"github.com/labuda/backend/internal/platform/s3presign"
	"github.com/stretchr/testify/require"
)

// testContentMediaCDNBase is the CDN base installed by
// configureContentMediaResolver.
//
// A configured CDN base makes resolution DETERMINISTIC: a persisted storage key
// resolves to `<CDNBaseURL>/<key>`. That determinism is what makes the
// "resolved URL differs from the persisted reference" proof meaningful — under
// the zero config an absolute URL passes through unchanged, so a projection that
// silently skipped resolution (or defaulted to the raw reference) would still
// look green.
const testContentMediaCDNBase = "https://cdn.example.test/content-media"

// configureContentMediaResolver installs the canonical mediaresolve authority for
// the Chat Content projection media proofs and restores the neutral zero config
// afterwards. No other test in this package asserts resolved media URLs.
func configureContentMediaResolver(t *testing.T) {
	t.Helper()

	mediaresolve.SetDefaultConfig(mediaresolve.Config{
		PresignCfg: s3presign.Config{
			Region:    "ap-southeast-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-media",
		},
		CDNBaseURL: testContentMediaCDNBase,
		ReadTTL:    time.Minute,
	})
	t.Cleanup(func() {
		mediaresolve.SetDefaultConfig(mediaresolve.Config{})
	})
}

// The Chat Content projection reads persisted `content_media.media_url` values,
// so its URL helper must project a persisted storage reference through the
// canonical mediaresolve authority — never emit the persisted reference.
func TestResolveReadableMediaReference_ResolvesPersistedStorageReference(t *testing.T) {
	configureContentMediaResolver(t)

	const storageReference = "images/1749600000010_shared.jpg"

	got := resolveReadableMediaReference(storageReference)

	require.Equal(t, testContentMediaCDNBase+"/"+storageReference, got)
	require.NotEqual(t, storageReference, got, "the persisted reference must not be emitted raw")
}

// Absolute readable URLs (external CDNs, already-presigned reads) are preserved
// — the projection resolves references, it does not rewrite them.
func TestResolveReadableMediaReference_PreservesAbsoluteURL(t *testing.T) {
	configureContentMediaResolver(t)

	const absoluteURL = "https://cdn.example.com/legacy/media.jpg"

	require.Equal(t, absoluteURL, resolveReadableMediaReference(absoluteURL))
}

// A blank persisted reference resolves to "" so the caller can DROP the media
// entry instead of surfacing an empty URL as if it were renderable.
func TestResolveReadableMediaReference_BlankReferenceResolvesEmpty(t *testing.T) {
	configureContentMediaResolver(t)

	require.Equal(t, "", resolveReadableMediaReference("   "))
}
