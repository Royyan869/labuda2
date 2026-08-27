/// Review status for external products.
///
/// Lifecycle: draft → pending_review → approved / rejected / request_changes / hidden
/// - request_changes: admin identified improvements needed; owner may resubmit after fixing.
/// - rejected: hard rejection; owner may resubmit.
/// - hidden: admin has hidden the product from public view.
/// Material edits on approved products return them to pending_review (backend enforced).
enum ExternalProductReviewStatus {
  draft('draft'),
  pendingReview('pending_review'),
  approved('approved'),
  rejected('rejected'),
  requestChanges('request_changes'),
  hidden('hidden');

  const ExternalProductReviewStatus(this.value);

  final String value;

  static ExternalProductReviewStatus fromString(String value) {
    return ExternalProductReviewStatus.values.firstWhere(
      (s) => s.value == value,
      orElse: () => ExternalProductReviewStatus.draft,
    );
  }
}
