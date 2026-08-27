package s3presign

import (
	"strings"
	"testing"
	"time"
)

// testCfg is a valid Config used across all tests.
var testCfg = Config{
	Region:    "ap-southeast-1",
	AccessKey: "AKIAIOSFODNN7EXAMPLE",
	SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	Bucket:    "my-private-bucket",
}

var testNow = time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

// ─── Config.Validate ───────────────────────────────────────────────────────

func TestValidate_MissingRegion(t *testing.T) {
	c := testCfg
	c.Region = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for empty Region")
	}
}

func TestValidate_MissingAccessKey(t *testing.T) {
	c := testCfg
	c.AccessKey = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for empty AccessKey")
	}
}

func TestValidate_MissingSecretKey(t *testing.T) {
	c := testCfg
	c.SecretKey = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for empty SecretKey")
	}
}

func TestValidate_MissingBucket(t *testing.T) {
	c := testCfg
	c.Bucket = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for empty Bucket")
	}
}

func TestValidate_OK(t *testing.T) {
	if err := testCfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── PresignPUT ────────────────────────────────────────────────────────────

func TestPresignPUT_ReturnsHTTPS(t *testing.T) {
	url, err := presignAt(testCfg, "PUT", "images/test.jpg", "image/jpeg", time.Hour, testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(url, "https://") {
		t.Errorf("expected https:// prefix, got: %s", url)
	}
}

func TestPresignPUT_CorrectHost(t *testing.T) {
	url, err := presignAt(testCfg, "PUT", "images/test.jpg", "image/jpeg", time.Hour, testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "https://my-private-bucket.s3.ap-southeast-1.amazonaws.com/"
	if !strings.HasPrefix(url, expected) {
		t.Errorf("expected host prefix %q, got: %s", expected, url)
	}
}

func TestPresignPUT_ContainsKey(t *testing.T) {
	url, err := presignAt(testCfg, "PUT", "kyc/user123/identity_ktp/photo.jpg", "image/jpeg", time.Hour, testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(url, "/kyc/user123/identity_ktp/photo.jpg?") {
		t.Errorf("key not found in URL: %s", url)
	}
}

func TestPresignPUT_SignedHeadersIncludesContentType(t *testing.T) {
	url, err := presignAt(testCfg, "PUT", "images/test.jpg", "image/jpeg", time.Hour, testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// content-type;host encoded: ; → %3B
	if !strings.Contains(url, "X-Amz-SignedHeaders=content-type%3Bhost") {
		t.Errorf("expected SignedHeaders=content-type%%3Bhost in URL: %s", url)
	}
}

func TestPresignPUT_ExpiresEncodedCorrectly(t *testing.T) {
	url, err := presignAt(testCfg, "PUT", "images/test.jpg", "image/jpeg", 30*time.Minute, testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(url, "X-Amz-Expires=1800") {
		t.Errorf("expected X-Amz-Expires=1800 in URL: %s", url)
	}
}

func TestPresignPUT_ContainsSignature(t *testing.T) {
	url, err := presignAt(testCfg, "PUT", "images/test.jpg", "image/jpeg", time.Hour, testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(url, "X-Amz-Signature=") {
		t.Errorf("missing X-Amz-Signature in URL: %s", url)
	}
	// Signature should be 64 hex chars (SHA256 output)
	sigIdx := strings.Index(url, "X-Amz-Signature=") + len("X-Amz-Signature=")
	sig := url[sigIdx:]
	if len(sig) != 64 {
		t.Errorf("expected 64-char hex signature, got %d chars: %s", len(sig), sig)
	}
}

func TestPresignPUT_EmptyKey_Error(t *testing.T) {
	_, err := presignAt(testCfg, "PUT", "", "image/jpeg", time.Hour, testNow)
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestPresignPUT_EmptyContentType_Error(t *testing.T) {
	_, err := presignAt(testCfg, "PUT", "images/test.jpg", "", time.Hour, testNow)
	if err == nil {
		t.Fatal("expected error for empty contentType on PUT")
	}
}

func TestPresignPUT_NegativeTTL_Error(t *testing.T) {
	_, err := presignAt(testCfg, "PUT", "images/test.jpg", "image/jpeg", -time.Hour, testNow)
	if err == nil {
		t.Fatal("expected error for negative TTL")
	}
}

func TestPresignPUT_ZeroTTL_Error(t *testing.T) {
	_, err := presignAt(testCfg, "PUT", "images/test.jpg", "image/jpeg", 0, testNow)
	if err == nil {
		t.Fatal("expected error for zero TTL")
	}
}

// ─── PresignGET ────────────────────────────────────────────────────────────

func TestPresignGET_SignedHeadersHostOnly(t *testing.T) {
	url, err := presignAt(testCfg, "GET", "kyc/user123/ktp.jpg", "", 5*time.Minute, testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(url, "X-Amz-SignedHeaders=host") {
		t.Errorf("expected SignedHeaders=host in URL: %s", url)
	}
	// Must NOT include content-type in signed headers for GET
	if strings.Contains(url, "content-type") {
		t.Errorf("content-type must not appear in GET signed headers: %s", url)
	}
}

func TestPresignGET_EmptyKey_Error(t *testing.T) {
	_, err := presignAt(testCfg, "GET", "", "", 5*time.Minute, testNow)
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestPresignGET_5MinExpiry(t *testing.T) {
	url, err := presignAt(testCfg, "GET", "kyc/doc.jpg", "", 5*time.Minute, testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(url, "X-Amz-Expires=300") {
		t.Errorf("expected X-Amz-Expires=300 in URL: %s", url)
	}
}

func TestPresignGET_AlgorithmParam(t *testing.T) {
	url, err := presignAt(testCfg, "GET", "kyc/doc.jpg", "", 5*time.Minute, testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(url, "X-Amz-Algorithm=AWS4-HMAC-SHA256") {
		t.Errorf("expected X-Amz-Algorithm=AWS4-HMAC-SHA256 in URL: %s", url)
	}
}

// ─── awsEncode ─────────────────────────────────────────────────────────────

func TestAwsEncode_UnreservedPassThrough(t *testing.T) {
	input := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.~"
	if got := awsEncode(input); got != input {
		t.Errorf("expected unreserved chars unchanged, got: %s", got)
	}
}

func TestAwsEncode_SpaceEncodedAs20(t *testing.T) {
	if got := awsEncode("hello world"); got != "hello%20world" {
		t.Errorf("expected 'hello%%20world', got: %s", got)
	}
}

func TestAwsEncode_SlashEncodedAs2F(t *testing.T) {
	if got := awsEncode("a/b"); got != "a%2Fb" {
		t.Errorf("expected 'a%%2Fb', got: %s", got)
	}
}

func TestAwsEncode_SemicolonEncodedAs3B(t *testing.T) {
	if got := awsEncode("content-type;host"); got != "content-type%3Bhost" {
		t.Errorf("expected 'content-type%%3Bhost', got: %s", got)
	}
}

// ─── encodePathSegments ────────────────────────────────────────────────────

func TestEncodePathSegments_PreservesSlashes(t *testing.T) {
	if got := encodePathSegments("kyc/user123/ktp.jpg"); got != "kyc/user123/ktp.jpg" {
		t.Errorf("expected path unchanged, got: %s", got)
	}
}

func TestEncodePathSegments_EncodesSpacesInSegment(t *testing.T) {
	if got := encodePathSegments("kyc/my file.jpg"); got != "kyc/my%20file.jpg" {
		t.Errorf("expected 'kyc/my%%20file.jpg', got: %s", got)
	}
}

// ─── Credential encoding ──────────────────────────────────────────────────

func TestPresignPUT_CredentialSlashesEncoded(t *testing.T) {
	url, err := presignAt(testCfg, "PUT", "test.jpg", "image/jpeg", time.Hour, testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Credential contains slashes which should be %2F-encoded
	if !strings.Contains(url, "X-Amz-Credential=AKIAIOSFODNN7EXAMPLE%2F") {
		t.Errorf("expected %%-encoded credential slashes in URL: %s", url)
	}
}

// ─── Determinism ──────────────────────────────────────────────────────────

func TestPresignPUT_SameInputSameOutput(t *testing.T) {
	url1, _ := presignAt(testCfg, "PUT", "test.jpg", "image/jpeg", time.Hour, testNow)
	url2, _ := presignAt(testCfg, "PUT", "test.jpg", "image/jpeg", time.Hour, testNow)
	if url1 != url2 {
		t.Errorf("presignAt must be deterministic for same inputs\n  url1=%s\n  url2=%s", url1, url2)
	}
}

func TestPresignGET_DifferentKeyDifferentURL(t *testing.T) {
	url1, _ := presignAt(testCfg, "GET", "kyc/ktp.jpg", "", 5*time.Minute, testNow)
	url2, _ := presignAt(testCfg, "GET", "kyc/selfie.jpg", "", 5*time.Minute, testNow)
	if url1 == url2 {
		t.Error("different keys must produce different presigned URLs")
	}
}


