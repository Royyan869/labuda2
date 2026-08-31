// DOMAIN: Moderation Domain (governance/moderation/)
// RESPONSIBILITY: Canonical Report intake record
//
// SLICE 2: This entity replaces the rejected GovernanceCase Report intake.
// Canonical authority: LABUDA — CANONICAL MODERATION SPECIFICATION v1 §6.

package entity

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ReportTargetType represents the canonical moderation target of a Report.
//
// Canonical target set (Specification v1 §12, Design v1 §5):
//
//	content, comment, for_sale, auction, user
//
// `profile` is canonically represented by `user` (no separate `profile`
// target). `chat_message` and `fixed_price_sale` are NOT canonical targets
// and are rejected by this runtime.
type ReportTargetType string

const (
	ReportTargetContent ReportTargetType = "content"
	ReportTargetComment ReportTargetType = "comment"
	ReportTargetForSale ReportTargetType = "for_sale"
	ReportTargetAuction ReportTargetType = "auction"
	ReportTargetUser    ReportTargetType = "user"
)

// IsValid returns true if the target type is in the canonical set.
func (t ReportTargetType) IsValid() bool {
	switch t {
	case ReportTargetContent, ReportTargetComment, ReportTargetForSale, ReportTargetAuction, ReportTargetUser:
		return true
	default:
		return false
	}
}

// String returns the string representation.
func (t ReportTargetType) String() string {
	return string(t)
}

// ReportReasonCode is the locked report reason taxonomy (Slice 2 §5).
//
// reason_code is backend-owned and validated server-side. No dynamic reason
// configuration, no policy engine, no database-configurable taxonomy.
type ReportReasonCode string

const (
	ReportReasonScamOrFraud          ReportReasonCode = "scam_or_fraud"
	ReportReasonProhibitedContent    ReportReasonCode = "prohibited_content"
	ReportReasonHarassmentOrAbuse    ReportReasonCode = "harassment_or_abuse"
	ReportReasonImpersonation        ReportReasonCode = "impersonation"
	ReportReasonMisleadingInformation ReportReasonCode = "misleading_information"
	ReportReasonCommerceViolation    ReportReasonCode = "commerce_violation"
	ReportReasonOther                ReportReasonCode = "other"
)

// IsValid returns true if the reason code is in the locked taxonomy.
func (c ReportReasonCode) IsValid() bool {
	switch c {
	case ReportReasonScamOrFraud,
		ReportReasonProhibitedContent,
		ReportReasonHarassmentOrAbuse,
		ReportReasonImpersonation,
		ReportReasonMisleadingInformation,
		ReportReasonCommerceViolation,
		ReportReasonOther:
		return true
	default:
		return false
	}
}

// String returns the string representation.
func (c ReportReasonCode) String() string {
	return string(c)
}

// EvidenceSnapshot is the minimal immutable evidence snapshot stored with a
// Report (Specification v1 §7, Design v1 §26, Business Truth §23).
//
// It captures the context of the subject AT REPORT TIME so governance history
// does not depend entirely on the current live object. It is NOT an authority:
// the live target domain remains authoritative for current state. It is NOT a
// full object archival system, NOT event sourcing, NOT domain versioning.
//
// Fields are target-appropriate minimal metadata (the same shape the legacy
// moderation preview exposed — author + title/caption + lifecycle status).
type EvidenceSnapshot struct {
	// AuthorID / AuthorUsername identify the owning user of the subject.
	AuthorID       string `json:"author_id,omitempty"`
	AuthorUsername string `json:"author_username,omitempty"`

	// Title is the subject title where applicable (for_sale/auction product
	// title, content caption, comment body, etc.).
	Title string `json:"title,omitempty"`

	// Text is a truncated preview of the subject content (caption/body).
	Text string `json:"text,omitempty"`

	// Status is the subject lifecycle status at report time (e.g. for_sale
	// status, account_status, content visibility).
	Status string `json:"status,omitempty"`

	// ContentType is the subject type discriminator where relevant
	// (e.g. post/repost for content, normal/commerce_reference for comment).
	ContentType string `json:"content_type,omitempty"`

	// IsDeleted records whether the subject was already soft-deleted at
	// report time.
	IsDeleted bool `json:"is_deleted,omitempty"`
}

// MarshalJSON implements json.Marshaler for storage as jsonb.
func (s EvidenceSnapshot) MarshalJSON() ([]byte, error) {
	type alias EvidenceSnapshot
	return json.Marshal(alias(s))
}

// UnmarshalJSON implements json.Unmarshaler for reading jsonb storage.
func (s *EvidenceSnapshot) UnmarshalJSON(data []byte) error {
	type alias EvidenceSnapshot
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*s = EvidenceSnapshot(a)
	return nil
}

// Report is an immutable historical intake record stating that a user
// reported a subject.
//
// Report is NOT a Case, NOT a Decision, NOT an Enforcement (Specification v1 §2).
// One report = one record. Multiple users may report the same subject.
//
// Report has no update path: reporter_id, subject_type, subject_id,
// reason_code, reason_note, evidence_snapshot, and created_at are immutable
// after creation.
type Report struct {
	ID               uuid.UUID
	ReporterID       uuid.UUID
	SubjectType      ReportTargetType
	SubjectID        uuid.UUID
	ReasonCode       ReportReasonCode
	ReasonNote       *string
	EvidenceSnapshot *EvidenceSnapshot
	CaseID           *uuid.UUID // nullable correlation field; correlation is NOT Slice 2 scope
	CreatedAt        time.Time
}

// NewReport creates a new canonical Report.
func NewReport(
	reporterID uuid.UUID,
	subjectType ReportTargetType,
	subjectID uuid.UUID,
	reasonCode ReportReasonCode,
	reasonNote *string,
	evidence *EvidenceSnapshot,
) *Report {
	return &Report{
		ID:               uuid.New(),
		ReporterID:       reporterID,
		SubjectType:      subjectType,
		SubjectID:        subjectID,
		ReasonCode:       reasonCode,
		ReasonNote:       reasonNote,
		EvidenceSnapshot: evidence,
		CreatedAt:        time.Now().UTC(),
	}
}

// ErrInvalidReportTarget is returned when the subject type is not canonical.
type ErrInvalidReportTarget struct {
	SubjectType string
}

func (e *ErrInvalidReportTarget) Error() string {
	return fmt.Sprintf("invalid report target type: %s. Canonical targets: content, comment, for_sale, auction, user", e.SubjectType)
}

// ErrInvalidReasonCode is returned when the reason code is outside the locked taxonomy.
type ErrInvalidReasonCode struct {
	ReasonCode string
}

func (e *ErrInvalidReasonCode) Error() string {
	return fmt.Sprintf("invalid reason code: %s. Allowed codes: scam_or_fraud, prohibited_content, harassment_or_abuse, impersonation, misleading_information, commerce_violation, other", e.ReasonCode)
}
