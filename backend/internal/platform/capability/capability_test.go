package capability

import "testing"

// TestGovernanceAuctionCancel_IsValidAndCatalogued proves the PASS_5B
// admin emergency auction cancel capability is registered in both places
// the capability system checks: IsValid (used by AssignCapability to
// reject unknown strings) and AllCapabilities (used for catalog listing).
func TestGovernanceAuctionCancel_IsValidAndCatalogued(t *testing.T) {
	if !IsValid(CapGovernanceAuctionCancel.String()) {
		t.Fatal("expected governance.auction.cancel to be a valid capability")
	}

	found := false
	for _, c := range AllCapabilities() {
		if c == CapGovernanceAuctionCancel {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected governance.auction.cancel in AllCapabilities()")
	}

	if CapGovernanceAuctionCancel.String() != "governance.auction.cancel" {
		t.Fatalf("unexpected capability string: %s", CapGovernanceAuctionCancel.String())
	}
}

// TestModerationAppealRead_IsValidAndCatalogued proves the PASS_13B
// appeal-specific read capability is registered in both places the
// capability system checks, and is distinct from both
// CapModerationCaseRead (generic case reading) and CapModerationAppealReview
// (the review/decision mutation) — appeal content requires its own trust
// boundary, not inherited from case-read.
func TestModerationAppealRead_IsValidAndCatalogued(t *testing.T) {
	if !IsValid(CapModerationAppealRead.String()) {
		t.Fatal("expected moderation.appeal.read to be a valid capability")
	}

	found := false
	for _, c := range AllCapabilities() {
		if c == CapModerationAppealRead {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected moderation.appeal.read in AllCapabilities()")
	}

	if CapModerationAppealRead.String() != "moderation.appeal.read" {
		t.Fatalf("unexpected capability string: %s", CapModerationAppealRead.String())
	}

	if CapModerationAppealRead == CapModerationCaseRead {
		t.Fatal("moderation.appeal.read must be distinct from moderation.case.read")
	}
	if CapModerationAppealRead == CapModerationAppealReview {
		t.Fatal("moderation.appeal.read must be distinct from moderation.appeal.review")
	}
}

// TestModerationEvidenceRead_IsValidAndCatalogued proves the dedicated
// evidence-read capability is registered and distinct from case-read.
func TestModerationEvidenceRead_IsValidAndCatalogued(t *testing.T) {
	if !IsValid(CapModerationEvidenceRead.String()) {
		t.Fatal("expected moderation.evidence.read to be a valid capability")
	}

	found := false
	for _, c := range AllCapabilities() {
		if c == CapModerationEvidenceRead {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected moderation.evidence.read in AllCapabilities()")
	}

	if CapModerationEvidenceRead.String() != "moderation.evidence.read" {
		t.Fatalf("unexpected capability string: %s", CapModerationEvidenceRead.String())
	}

	if CapModerationEvidenceRead == CapModerationCaseRead {
		t.Fatal("moderation.evidence.read must be distinct from moderation.case.read")
	}
}
