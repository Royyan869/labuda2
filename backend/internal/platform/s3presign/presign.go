// Package s3presign generates AWS Signature V4 pre-signed URLs for private
// S3 operations without requiring the AWS SDK.
//
// Two operations are supported:
//   - PresignPUT  — short-lived upload URL for a specific content-type.
//   - PresignGET  — short-lived read URL for a private object.
//
// The caller supplies the Config (bucket, region, credentials) and a TTL.
// No network calls are made; the URL is assembled and signed locally.
//
// Signing algorithm: AWS Signature Version 4 (SigV4), presigned-URL variant.
// Reference: https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-query-string-auth.html
package s3presign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Config holds S3 bucket coordinates and credentials for URL signing.
type Config struct {
	Region    string
	AccessKey string
	SecretKey string
	Bucket    string
}

// Validate returns a descriptive error when a required field is missing.
func (c Config) Validate() error {
	switch {
	case c.Region == "":
		return fmt.Errorf("s3presign: Region required")
	case c.AccessKey == "":
		return fmt.Errorf("s3presign: AccessKey required")
	case c.SecretKey == "":
		return fmt.Errorf("s3presign: SecretKey required")
	case c.Bucket == "":
		return fmt.Errorf("s3presign: Bucket required")
	}
	return nil
}

// PresignPUT returns a pre-signed PUT URL for uploading a private S3 object.
//
// The PUT request MUST include the header Content-Type: <contentType>.
// The content-type is embedded in the signature so any mismatch will be
// rejected by AWS.
//
// ttl must be positive; AWS caps SigV4 presigned URL lifetime at 7 days.
func PresignPUT(cfg Config, key, contentType string, ttl time.Duration) (string, error) {
	return presignAt(cfg, "PUT", key, contentType, ttl, time.Now().UTC())
}

// PresignGET returns a pre-signed GET URL for reading a private S3 object.
//
// ttl must be positive; AWS caps SigV4 presigned URL lifetime at 7 days.
func PresignGET(cfg Config, key string, ttl time.Duration) (string, error) {
	return presignAt(cfg, "GET", key, "", ttl, time.Now().UTC())
}

// presignAt is the testable core — callers supply an explicit timestamp so
// tests can assert on URL content without caring about wall-clock values.
func presignAt(cfg Config, method, key, contentType string, ttl time.Duration, now time.Time) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	if key == "" {
		return "", fmt.Errorf("s3presign: key required")
	}
	if method == "PUT" && contentType == "" {
		return "", fmt.Errorf("s3presign: contentType required for PUT")
	}
	seconds := int64(ttl.Seconds())
	if seconds <= 0 {
		return "", fmt.Errorf("s3presign: ttl must be positive")
	}

	host := cfg.Bucket + ".s3." + cfg.Region + ".amazonaws.com"
	datetime := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	credential := cfg.AccessKey + "/" + date + "/" + cfg.Region + "/s3/aws4_request"

	// Signed headers:
	//   PUT — "content-type;host" (alphabetical; client must send Content-Type)
	//   GET — "host"
	var signedHeaders, canonicalHeaders string
	if method == "PUT" {
		signedHeaders = "content-type;host"
		canonicalHeaders = "content-type:" + contentType + "\nhost:" + host + "\n"
	} else {
		signedHeaders = "host"
		canonicalHeaders = "host:" + host + "\n"
	}

	// Canonical query string (sorted alphabetically by encoded key name).
	// Parameter order: Algorithm < Credential < Date < Expires < SignedHeaders.
	cqs := "X-Amz-Algorithm=AWS4-HMAC-SHA256" +
		"&X-Amz-Credential=" + awsEncode(credential) +
		"&X-Amz-Date=" + datetime + // datetime is unreserved; no encoding needed
		"&X-Amz-Expires=" + fmt.Sprintf("%d", seconds) +
		"&X-Amz-SignedHeaders=" + awsEncode(signedHeaders)

	canonicalURI := "/" + encodePathSegments(key)

	// Canonical request:
	//   METHOD\nURI\nQueryString\nHeaders\n\nSignedHeaders\nUNSIGNED-PAYLOAD
	canonicalReq := strings.Join([]string{
		method,
		canonicalURI,
		cqs,
		canonicalHeaders, // already ends with \n; blank line separates headers from signed-headers
		signedHeaders,
		"UNSIGNED-PAYLOAD",
	}, "\n")

	// String to sign.
	scope := date + "/" + cfg.Region + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + datetime + "\n" + scope + "\n" + hexSHA256(canonicalReq)

	// Derive signing key and compute hex signature.
	signingKey := deriveSigningKey(cfg.SecretKey, date, cfg.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	url := "https://" + host + canonicalURI + "?" + cqs + "&X-Amz-Signature=" + signature
	return url, nil
}

// buildPresignQuery is exposed for tests that need to assert on individual params.
// Production code uses presignAt which builds the query inline.
func buildPresignQuery(credential, datetime, expiresStr, signedHeaders string) string {
	return "X-Amz-Algorithm=AWS4-HMAC-SHA256" +
		"&X-Amz-Credential=" + awsEncode(credential) +
		"&X-Amz-Date=" + datetime +
		"&X-Amz-Expires=" + expiresStr +
		"&X-Amz-SignedHeaders=" + awsEncode(signedHeaders)
}

// encodePathSegments encodes each path segment independently with awsEncode
// and re-joins them with '/'. This preserves path separators while encoding
// all other special characters within a segment.
func encodePathSegments(key string) string {
	segments := strings.Split(key, "/")
	for i, s := range segments {
		segments[i] = awsEncode(s)
	}
	return strings.Join(segments, "/")
}

// awsEncode applies the AWS percent-encoding rules:
//   - Unreserved characters (A-Z a-z 0-9 - _ . ~) are passed through unchanged.
//   - All other bytes are encoded as %XX with uppercase hex digits.
//
// This differs from url.QueryEscape which encodes spaces as '+' and does not
// encode characters such as '*'.
func awsEncode(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	const hexChars = "0123456789ABCDEF"
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUnreserved(c) {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hexChars[c>>4])
			b.WriteByte(hexChars[c&0xf])
		}
	}
	return b.String()
}

func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == '~'
}

// deriveSigningKey computes the AWS Sig V4 signing key via the HMAC-SHA256
// derivation chain: "AWS4"+secret → date → region → service → "aws4_request".
func deriveSigningKey(secret, date, region, service string) []byte {
	k1 := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	k2 := hmacSHA256(k1, []byte(region))
	k3 := hmacSHA256(k2, []byte(service))
	return hmacSHA256(k3, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hexSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}


