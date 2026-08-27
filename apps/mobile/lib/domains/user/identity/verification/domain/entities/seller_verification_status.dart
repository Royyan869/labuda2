/// Seller Verification Status Entities
///
/// Contract types for seller verification lifecycle, document review status,
/// and document types. Backend authority: backend/internal/governance/verification/entity/
library;

// =============================================================================
// SellerVerificationStatus — 8 states (matches seller_verification.go)
// =============================================================================

enum SellerVerificationStatus {
  notSubmitted,
  pendingReview,
  needsResubmission,
  approved,
  rejected,
  suspended,
  revoked,
  underInvestigation,
}

extension SellerVerificationStatusExtension on SellerVerificationStatus {
  String get apiValue {
    switch (this) {
      case SellerVerificationStatus.notSubmitted:
        return 'not_submitted';
      case SellerVerificationStatus.pendingReview:
        return 'pending_review';
      case SellerVerificationStatus.needsResubmission:
        return 'needs_resubmission';
      case SellerVerificationStatus.approved:
        return 'approved';
      case SellerVerificationStatus.rejected:
        return 'rejected';
      case SellerVerificationStatus.suspended:
        return 'suspended';
      case SellerVerificationStatus.revoked:
        return 'revoked';
      case SellerVerificationStatus.underInvestigation:
        return 'under_investigation';
    }
  }

  String get displayName {
    switch (this) {
      case SellerVerificationStatus.notSubmitted:
        return 'Belum Diajukan';
      case SellerVerificationStatus.pendingReview:
        return 'Menunggu Tinjauan';
      case SellerVerificationStatus.needsResubmission:
        return 'Perlu Pengajuan Ulang';
      case SellerVerificationStatus.approved:
        return 'Disetujui';
      case SellerVerificationStatus.rejected:
        return 'Ditolak';
      case SellerVerificationStatus.suspended:
        return 'Ditangguhkan';
      case SellerVerificationStatus.revoked:
        return 'Dicabut';
      case SellerVerificationStatus.underInvestigation:
        return 'Dalam Investigasi';
    }
  }

  bool get isActive => this == SellerVerificationStatus.approved;
  bool get isTerminal =>
      this == SellerVerificationStatus.revoked ||
      this == SellerVerificationStatus.rejected;
  bool get isPending =>
      this == SellerVerificationStatus.pendingReview ||
      this == SellerVerificationStatus.needsResubmission;

  static SellerVerificationStatus parse(String? value) {
    switch (value?.toLowerCase()) {
      case 'pending_review':
        return SellerVerificationStatus.pendingReview;
      case 'needs_resubmission':
        return SellerVerificationStatus.needsResubmission;
      case 'approved':
        return SellerVerificationStatus.approved;
      case 'rejected':
        return SellerVerificationStatus.rejected;
      case 'suspended':
        return SellerVerificationStatus.suspended;
      case 'revoked':
        return SellerVerificationStatus.revoked;
      case 'under_investigation':
        return SellerVerificationStatus.underInvestigation;
      case 'not_submitted':
      default:
        return SellerVerificationStatus.notSubmitted;
    }
  }
}

// =============================================================================
// VerificationReviewStatus — 4 states (matches verification_document.go)
// =============================================================================

enum VerificationReviewStatus { notSubmitted, pending, approved, rejected }

extension VerificationReviewStatusExtension on VerificationReviewStatus {
  String get apiValue {
    switch (this) {
      case VerificationReviewStatus.notSubmitted:
        return 'not_submitted';
      case VerificationReviewStatus.pending:
        return 'pending';
      case VerificationReviewStatus.approved:
        return 'approved';
      case VerificationReviewStatus.rejected:
        return 'rejected';
    }
  }

  static VerificationReviewStatus parse(String? value) {
    switch (value?.toLowerCase()) {
      case 'pending':
        return VerificationReviewStatus.pending;
      case 'approved':
        return VerificationReviewStatus.approved;
      case 'rejected':
        return VerificationReviewStatus.rejected;
      case 'not_submitted':
      default:
        return VerificationReviewStatus.notSubmitted;
    }
  }
}

// =============================================================================
// VerificationDocumentType — 2 types (KYC minimal scope: owner decision)
// Business docs (NPWP/SIUP/NIB/business_other) removed — migration 000205.
// =============================================================================

enum VerificationDocumentType { identityKtp, identitySelfie }

extension VerificationDocumentTypeExtension on VerificationDocumentType {
  String get apiValue {
    switch (this) {
      case VerificationDocumentType.identityKtp:
        return 'identity_ktp';
      case VerificationDocumentType.identitySelfie:
        return 'identity_selfie';
    }
  }

  String get displayName {
    switch (this) {
      case VerificationDocumentType.identityKtp:
        return 'KTP';
      case VerificationDocumentType.identitySelfie:
        return 'Selfie dengan KTP';
    }
  }

  static VerificationDocumentType parse(String? value) {
    switch (value?.toLowerCase()) {
      case 'identity_selfie':
        return VerificationDocumentType.identitySelfie;
      case 'identity_ktp':
      default:
        return VerificationDocumentType.identityKtp;
    }
  }
}
