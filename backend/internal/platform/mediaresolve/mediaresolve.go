package mediaresolve

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/labuda/backend/internal/platform/s3presign"
)

// Config controls how media references are projected for read paths.
//
// Resolution order:
//   - key or bucket URL -> CDN URL when CDNBaseURL is configured
//   - key or bucket URL -> fresh presigned GET URL in development
//   - valid external absolute URL -> passed through unchanged
//
// Invalid inputs return a MediaReferenceError.
type Config struct {
	PresignCfg s3presign.Config
	CDNBaseURL string
	ReadTTL    time.Duration
}

// MediaReferenceError reports a malformed or unsupported media reference.
type MediaReferenceError struct {
	Reference string
	Reason    string
}

func (e *MediaReferenceError) Error() string {
	if e.Reference == "" {
		return fmt.Sprintf("invalid media reference: %s", e.Reason)
	}
	return fmt.Sprintf("invalid media reference %q: %s", e.Reference, e.Reason)
}

var (
	defaultMu  sync.RWMutex
	defaultCfg Config
)

// SetDefaultConfig configures the package-level resolver used by helper
// functions. Call once during bootstrap.
func SetDefaultConfig(cfg Config) {
	cfg.CDNBaseURL = strings.TrimRight(strings.TrimSpace(cfg.CDNBaseURL), "/")
	if cfg.ReadTTL <= 0 {
		cfg.ReadTTL = 5 * time.Minute
	}

	defaultMu.Lock()
	defaultCfg = cfg
	defaultMu.Unlock()
}

// ResolveMediaReadURL resolves a media reference using the package default
// configuration.
func ResolveMediaReadURL(reference string) (string, error) {
	defaultMu.RLock()
	cfg := defaultCfg
	defaultMu.RUnlock()
	return ResolveMediaReadURLWithConfig(reference, cfg)
}

// ResolveMediaReadURLWithConfig resolves a reference to the canonical readable
// projection for read surfaces.
func ResolveMediaReadURLWithConfig(reference string, cfg Config) (string, error) {
	ref := strings.TrimSpace(reference)
	if ref == "" {
		return "", &MediaReferenceError{Reason: "empty reference"}
	}

	if u, err := url.Parse(ref); err == nil && u.IsAbs() {
		if key, ok := extractStorageKeyFromURL(u, cfg); ok {
			return resolveStorageKey(key, cfg)
		}
		if isAbsoluteCDNURL(u, cfg.CDNBaseURL) {
			return ref, nil
		}
		return ref, nil
	}

	if looksLikeStorageKey(ref) {
		return resolveStorageKey(ref, cfg)
	}

	return "", &MediaReferenceError{Reference: ref, Reason: "expected storage key or absolute URL"}
}

// NormalizeStorageReference converts legacy bucket URLs to storage keys while
// preserving valid external absolute URLs.
func NormalizeStorageReference(reference string) (string, error) {
	defaultMu.RLock()
	cfg := defaultCfg
	defaultMu.RUnlock()
	return NormalizeStorageReferenceWithConfig(reference, cfg)
}

// NormalizeStorageReferenceWithConfig normalizes a persisted reference to the
// canonical storage representation.
func NormalizeStorageReferenceWithConfig(reference string, cfg Config) (string, error) {
	ref := strings.TrimSpace(reference)
	if ref == "" {
		return "", &MediaReferenceError{Reason: "empty reference"}
	}

	if u, err := url.Parse(ref); err == nil && u.IsAbs() {
		if key, ok := extractStorageKeyFromURL(u, cfg); ok {
			return key, nil
		}
		if isAbsoluteCDNURL(u, cfg.CDNBaseURL) {
			key := strings.TrimPrefix(strings.TrimPrefix(u.Path, "/"), "/")
			if key == "" {
				return "", &MediaReferenceError{Reference: ref, Reason: "cdn URL missing object key"}
			}
			return key, nil
		}
		return ref, nil
	}

	if looksLikeStorageKey(ref) {
		return ref, nil
	}

	return "", &MediaReferenceError{Reference: ref, Reason: "expected storage key or absolute URL"}
}

// ResolveMediaReadURLs resolves a batch of references.
func ResolveMediaReadURLs(references []string) ([]string, error) {
	out := make([]string, 0, len(references))
	for _, ref := range references {
		if strings.TrimSpace(ref) == "" {
			continue
		}
		resolved, err := ResolveMediaReadURL(ref)
		if err != nil {
			return nil, err
		}
		out = append(out, resolved)
	}
	return out, nil
}

// NormalizeMediaReferences converts a batch of references to storage keys or
// preserved external URLs.
func NormalizeMediaReferences(references []string) ([]string, error) {
	out := make([]string, 0, len(references))
	for _, ref := range references {
		if strings.TrimSpace(ref) == "" {
			continue
		}
		normalized, err := NormalizeStorageReference(ref)
		if err != nil {
			return nil, err
		}
		out = append(out, normalized)
	}
	return out, nil
}

func resolveStorageKey(key string, cfg Config) (string, error) {
	key = strings.TrimSpace(key)
	if !looksLikeStorageKey(key) {
		return "", &MediaReferenceError{Reference: key, Reason: "invalid storage key"}
	}

	if cfg.CDNBaseURL != "" {
		return strings.TrimRight(cfg.CDNBaseURL, "/") + "/" + key, nil
	}
	if err := cfg.PresignCfg.Validate(); err != nil {
		return "", &MediaReferenceError{Reference: key, Reason: err.Error()}
	}
	return s3presign.PresignGET(cfg.PresignCfg, key, cfg.ReadTTL)
}

func extractStorageKeyFromURL(u *url.URL, cfg Config) (string, bool) {
	host := strings.ToLower(strings.TrimSpace(u.Host))
	if host == "" {
		return "", false
	}

	bucket := strings.ToLower(strings.TrimSpace(cfg.PresignCfg.Bucket))
	hostBucketPrefix := bucket + ".s3."
	hostLegacy := bucket + ".s3.amazonaws.com"
	pathStylePrefix := "s3."
	pathStyleLegacy := "s3.amazonaws.com"

	if strings.HasPrefix(host, hostBucketPrefix) && strings.HasSuffix(host, ".amazonaws.com") || host == hostLegacy {
		key := strings.TrimPrefix(u.Path, "/")
		if key == "" {
			return "", false
		}
		return key, true
	}

	if strings.HasPrefix(host, pathStylePrefix) && strings.HasSuffix(host, ".amazonaws.com") || host == pathStyleLegacy {
		segments := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		if len(segments) < 2 || strings.TrimSpace(segments[0]) != bucket {
			return "", false
		}
		key := strings.Join(segments[1:], "/")
		if key == "" {
			return "", false
		}
		return key, true
	}

	return "", false
}

func isAbsoluteCDNURL(u *url.URL, cdnBaseURL string) bool {
	cdnBaseURL = strings.TrimRight(strings.TrimSpace(cdnBaseURL), "/")
	if cdnBaseURL == "" {
		return false
	}
	cdn, err := url.Parse(cdnBaseURL)
	if err != nil || !cdn.IsAbs() {
		return false
	}
	return strings.EqualFold(u.Scheme, cdn.Scheme) &&
		strings.EqualFold(u.Host, cdn.Host) &&
		strings.HasPrefix(u.Path, cdn.Path)
}

func looksLikeStorageKey(ref string) bool {
	if strings.Contains(ref, "://") {
		return false
	}
	if strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "\\") {
		return false
	}
	if strings.Contains(ref, `\`) {
		return false
	}
	if strings.Contains(ref, "..") {
		return false
	}
	cleaned := path.Clean(ref)
	return cleaned != "." && cleaned == ref
}
