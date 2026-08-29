import 'package:labuda/core/core.dart';

/// Notification Trigger Interface
///
/// Interface untuk trigger notifications dari modul lain.
/// Modul lain (chat, order, auth) inject interface ini tanpa
/// dependency langsung ke notification module.
///
/// Size: < 100 lines (per GUIDELINES)
abstract class INotificationTrigger {
  /// Send notification ke single user
  Future<Result<void>> sendNotification({
    required String userId,
    required NotificationType type,
    required String title,
    required String body,
    Map<String, dynamic>? data,
  });

  /// Send notification ke multiple users (batch)
  Future<Result<void>> sendNotificationBatch({
    required List<String> userIds,
    required NotificationType type,
    required String title,
    required String body,
    Map<String, dynamic>? data,
  });
}

/// Notification Types
///
/// Wire strings MUST match the notification `type` column value that the
/// backend inserts via InsertNotification / insertNotificationWithPolicy.
/// The outbox event type (e.g. "chat.message.sent") is separate from the
/// notification type stored in the DB (e.g. "chat_message").
///
/// REMOVED IN N3 (ghost types — no backend handler ever emitted these):
///   orderConfirmed, supportTicketClaimed, warningIssued, accountSuspended,
///   accountBanned, contentRemoved (old underscore wire), penaltyApplied,
///   appealSubmitted, appealApproved, appealRejected
enum NotificationType {
  // =============================================================================
  // ORDER NOTIFICATIONS
  // =============================================================================
  orderCreated('order.created'), // seller: new order to fulfill
  orderCreatedBuyer(
    'order.created.buyer',
  ), // buyer: order placement confirmation
  orderPaid('order.paid'), // seller: payment received
  orderPaidBuyer('order.paid.buyer'), // buyer: payment accepted
  orderShipped('order.shipped'),
  orderCompleted('order.completed'),
  orderCancelled('order.cancelled'),
  orderCancelledTimeout('order.cancelled_timeout'),
  orderExpired('order.expired'),
  orderRefunded('order.refunded'),
  orderPartiallyRefunded('order.partially_refunded'),
  orderDisputeOpen('order.dispute_open'),
  orderConfirmationExtended('order.confirmation_extended'),
  orderOverdueReminderSeller('order.overdue_reminder.seller'),
  orderOverdueReminderBuyer('order.overdue_reminder.buyer'),

  // =============================================================================
  // REFUND LIFECYCLE — D1A + H2-C
  // =============================================================================
  refundOpened('refund.opened'),
  refundApproved('refund.approved'),
  refundRejected('refund.rejected'),
  refundEscalated('refund.escalated'),

  // =============================================================================
  // DISPUTE
  // =============================================================================
  disputeOpened('dispute.opened'),
  disputeResolved('dispute.resolved'),
  disputeOverdue('dispute.overdue'), // G1: admin fanout — dispute aging
  disputeTimeoutEscalation(
    'dispute.timeout_escalation',
  ), // G1: admin fanout — dispute timeout escalated

  // =============================================================================
  // WITHDRAWAL / PAYOUT
  // =============================================================================
  withdrawalRequested('withdrawal.requested'),
  withdrawalApproved('withdrawal.approved'),
  withdrawalRejected('withdrawal.rejected'),
  withdrawalCompleted('withdrawal.completed'),
  withdrawalFailed('withdrawal.failed'),

  // =============================================================================
  // NEGOTIATION
  // =============================================================================
  negotiationStarted('negotiation.started'),
  negotiationMessageSent('negotiation.message_sent'),
  negotiationAccepted('negotiation.accepted'),
  negotiationExpired('negotiation.expired'),
  negotiationCancelled(
    'negotiation.cancelled',
  ), // B1: both parties notified on cancellation

  // =============================================================================
  // SELLER TIER (reputation) — B1
  // =============================================================================
  sellerTierUpgraded('seller.tier.upgraded'),
  sellerTierDowngraded('seller.tier.downgraded'),

  // =============================================================================
  // VERIFICATION — document-level
  // =============================================================================
  verificationDocumentApproved('verification.document.approved'),
  verificationDocumentRejected('verification.document.rejected'),

  // =============================================================================
  // SELLER VERIFICATION — seller lifecycle (distinct from document-level)
  // =============================================================================
  sellerVerificationSubmitted('seller.verification.submitted'),
  sellerVerificationApproved('seller.verification.approved'),
  sellerVerificationRejected('seller.verification.rejected'),
  sellerVerificationNeedsResubmission('seller.verification.needs_resubmission'),
  sellerVerificationSuspended('seller.verification.suspended'),
  sellerVerificationRevoked('seller.verification.revoked'),
  sellerVerificationUnderInvestigation(
    'seller.verification.under_investigation',
  ),
  sellerVerificationRestored('seller.verification.restored'),

  // =============================================================================
  // SELLER SUBSCRIPTION
  // =============================================================================
  sellerSubscriptionExpiring('seller.subscription.expiring'),
  sellerSubscriptionExpired('seller.subscription.expired'),

  // =============================================================================
  // AUCTION
  // =============================================================================
  auctionBidPlaced('auction.bid.placed'),
  auctionWaitingSettlement('auction.waiting_settlement'),
  auctionSellerHasWinner(
    'auction.seller_has_winner',
  ), // P14: seller — winner pending claim
  auctionEndedNoWinner(
    'auction.ended_no_winner',
  ), // P14: seller — auction closed without bids
  auctionBnrSeller('auction.bnr_seller'),
  auctionBnrWinner('auction.bnr_winner'),

  // =============================================================================
  // MODERATION
  // =============================================================================
  moderationContentRemoved('moderation.content.removed'),
  moderationCommentRemoved('moderation.comment.removed'),
  moderationForSaleRemoved('moderation.for_sale.removed'),
  moderationAuctionRemoved('moderation.auction.removed'),
  moderationContentRestored('moderation.content.restored'),
  moderationCommentRestored('moderation.comment.restored'),
  moderationForSaleRestored('moderation.for_sale.restored'),
  moderationAuctionRestored('moderation.auction.restored'),
  moderationUserSuspended('moderation.user.suspended'),
  moderationUserRestored('moderation.user.restored'),
  moderationWarningIssued('moderation.warning.issued'),

  // =============================================================================
  // SUPPORT
  // =============================================================================
  supportTicketCreated('support.ticket.created'),
  supportTicketResolved('support.ticket.resolved'),
  supportTicketClosed('support.ticket.closed'),
  supportTicketWaitingUser(
    'support.ticket_waiting_user',
  ), // underscore before 'user', not dot
  supportTicketUserResponded('support.ticket.user_responded'),

  // =============================================================================
  // SOCIAL
  // =============================================================================
  chatMessage('chat_message'),
  userFollowed('user.followed'),
  contentLiked('content.liked'),
  comment('comment'),
  commentReply('comment_reply'),
  sellerResponse('seller.response'),
  mention('mention'), // Flutter-side only: MentionNotificationService

  // =============================================================================
  // EXTERNAL PRODUCT REVIEW — owner-facing review decision notifications
  // =============================================================================
  externalProductReviewApproved('external_product.review.approved'),
  externalProductReviewRejected('external_product.review.rejected'),
  externalProductReviewRequestChanges(
    'external_product.review.request_changes',
  ),
  externalProductReviewHidden('external_product.review.hidden'),

  // =============================================================================
  // MARKETING / SYSTEM
  // =============================================================================
  promotion('promotion'),
  announcement(
    'announcement',
  ), // also the fallback for unrecognised types (see fromString)
  systemMaintenance('system_maintenance');

  const NotificationType(this.value);
  final String value;

  /// Get enum from string value.
  /// Falls back to [announcement] for unrecognised backend types so new
  /// backend types never crash the app.
  static NotificationType fromString(String value) {
    return NotificationType.values.firstWhere(
      (type) => type.value == value,
      orElse: () => NotificationType.announcement,
    );
  }
}
