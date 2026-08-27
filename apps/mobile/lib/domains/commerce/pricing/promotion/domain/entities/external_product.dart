import 'package:equatable/equatable.dart';

import 'external_product_media.dart';
import 'external_product_review_status.dart';

/// External product entity for promotion targeting.
///
/// Represents a URL-based product that goes through admin review
/// before it can be used as a promotion target.
///
/// Lifecycle: draft → pending_review → approved/rejected/hidden
class ExternalProduct extends Equatable {
  final String id;
  final String ownerUserId;
  final String title;
  final String? description;
  final String externalUrl;
  final String normalizedExternalUrl;
  final ExternalProductReviewStatus reviewStatus;
  final String? rejectionReason;
  final bool unsafeUrlFlag;
  final DateTime? submittedAt;
  final DateTime? approvedAt;
  final DateTime? rejectedAt;
  final DateTime? hiddenAt;
  final DateTime createdAt;
  final DateTime updatedAt;
  final List<ExternalProductMedia> media;
  final bool canEdit;
  final bool canSubmit;
  final bool canResubmit;
  final bool publicVisible;

  const ExternalProduct({
    required this.id,
    required this.ownerUserId,
    required this.title,
    this.description,
    required this.externalUrl,
    required this.normalizedExternalUrl,
    required this.reviewStatus,
    this.rejectionReason,
    required this.unsafeUrlFlag,
    this.submittedAt,
    this.approvedAt,
    this.rejectedAt,
    this.hiddenAt,
    required this.createdAt,
    required this.updatedAt,
    this.media = const [],
    required this.canEdit,
    required this.canSubmit,
    required this.canResubmit,
    required this.publicVisible,
  });

  /// Whether the review status shows a rejection/hidden reason
  bool get hasRejectionReason =>
      rejectionReason != null && rejectionReason!.isNotEmpty;

  /// Whether the product is approved and eligible for promotion activation
  bool get isApproved => reviewStatus == ExternalProductReviewStatus.approved;

  @override
  List<Object?> get props => [
    id,
    ownerUserId,
    title,
    description,
    externalUrl,
    normalizedExternalUrl,
    reviewStatus,
    rejectionReason,
    unsafeUrlFlag,
    submittedAt,
    approvedAt,
    rejectedAt,
    hiddenAt,
    createdAt,
    updatedAt,
    media,
    canEdit,
    canSubmit,
    canResubmit,
    publicVisible,
  ];
}
