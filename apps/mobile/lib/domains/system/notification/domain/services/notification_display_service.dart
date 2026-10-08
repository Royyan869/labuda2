/// Notification display domain service
///
/// Provides business logic for notification display properties.
/// This service is pure domain logic with no Flutter dependencies.
library;

import 'package:labuda/core/interfaces/i_notification_trigger.dart';

/// Display color enum (pure domain, no Flutter dependency)
enum NotificationDisplayColor {
  green,
  red,
  orange,
  blue,
  pink,
  indigo,
  deepOrange,
  teal,
  cyan,
  grey,
  purple,
}

/// Icon data enum (pure domain, no Flutter dependency)
enum NotificationDisplayIcon {
  shoppingBag,
  cancel,
  assignmentReturn,
  checkCircle,
  error,
  hourglassEmpty,
  verifiedUser,
  chat,
  security,
  favorite,
  comment,
  alternateEmail,
  article,
  campaign,
  build,
  supportAgent,
  warning,
  block,
  delete,
  gavel,
}

/// Notification display metadata
class NotificationDisplayMetadata {
  final NotificationDisplayIcon icon;
  final NotificationDisplayColor color;

  const NotificationDisplayMetadata({required this.icon, required this.color});
}

/// Notification display service
class NotificationDisplayService {
  const NotificationDisplayService();

  /// Get display metadata for a notification type
  NotificationDisplayMetadata getDisplayMetadata(NotificationType type) {
    return NotificationDisplayMetadata(
      icon: _getIcon(type),
      color: _getColor(type),
    );
  }

  /// Get icon for notification type
  NotificationDisplayIcon _getIcon(NotificationType type) {
    switch (type) {
      // Order
      case NotificationType.orderCreated:
      case NotificationType.orderCreatedBuyer:
      case NotificationType.orderPaidBuyer:
      case NotificationType.orderPaid:
      case NotificationType.orderShipped:
        return NotificationDisplayIcon.shoppingBag;
      case NotificationType.orderExpired:
      case NotificationType.orderCancelled:
      case NotificationType.orderCancelledTimeout:
        return NotificationDisplayIcon.cancel;
      case NotificationType.orderCompleted:
        return NotificationDisplayIcon.checkCircle;
      case NotificationType.orderRefunded:
      case NotificationType.orderPartiallyRefunded:
        return NotificationDisplayIcon.assignmentReturn;
      case NotificationType.orderDisputeOpen:
        return NotificationDisplayIcon.gavel;
      case NotificationType.orderConfirmationExtended:
      case NotificationType.orderOverdueReminderSeller:
      case NotificationType.orderOverdueReminderBuyer:
        return NotificationDisplayIcon.hourglassEmpty;
      // Refund / dispute
      case NotificationType.refundOpened:
        return NotificationDisplayIcon.assignmentReturn;
      case NotificationType.refundApproved:
        return NotificationDisplayIcon.checkCircle;
      case NotificationType.refundRejected:
        return NotificationDisplayIcon.cancel;
      case NotificationType.refundEscalated:
        return NotificationDisplayIcon.gavel;
      case NotificationType.disputeOpened:
        return NotificationDisplayIcon.gavel;
      case NotificationType.disputeResolved:
        return NotificationDisplayIcon.checkCircle;
      case NotificationType.disputeOverdue:
      case NotificationType.disputeTimeoutEscalation:
        return NotificationDisplayIcon.hourglassEmpty;
      // Withdrawal
      case NotificationType.withdrawalRequested:
      case NotificationType.withdrawalApproved:
      case NotificationType.withdrawalCompleted:
      case NotificationType.withdrawalRejected:
        return NotificationDisplayIcon.checkCircle;
      case NotificationType.withdrawalFailed:
        return NotificationDisplayIcon.error;
      // Negotiation
      case NotificationType.negotiationStarted:
      case NotificationType.negotiationMessageSent:
      case NotificationType.negotiationAccepted:
      case NotificationType.negotiationExpired:
        return NotificationDisplayIcon.article;
      case NotificationType.negotiationCancelled:
        return NotificationDisplayIcon.cancel;
      // Verification (document-level)
      case NotificationType.verificationDocumentApproved:
      case NotificationType.sellerVerificationApproved:
        return NotificationDisplayIcon.verifiedUser;
      case NotificationType.verificationDocumentRejected:
      case NotificationType.sellerVerificationRejected:
        return NotificationDisplayIcon.cancel;
      case NotificationType.sellerVerificationSubmitted:
      case NotificationType.sellerVerificationNeedsResubmission:
        return NotificationDisplayIcon.article;
      case NotificationType.sellerVerificationSuspended:
        return NotificationDisplayIcon.block;
      case NotificationType.sellerVerificationRevoked:
        return NotificationDisplayIcon.cancel;
      case NotificationType.sellerVerificationUnderInvestigation:
        return NotificationDisplayIcon.warning;
      case NotificationType.sellerVerificationRestored:
        return NotificationDisplayIcon.verifiedUser;
      // Seller subscription
      case NotificationType.sellerSubscriptionExpiring:
      case NotificationType.sellerSubscriptionExpired:
        return NotificationDisplayIcon.warning;
      // Seller tier — B1
      case NotificationType.sellerTierUpgraded:
        return NotificationDisplayIcon.checkCircle;
      case NotificationType.sellerTierDowngraded:
        return NotificationDisplayIcon.warning;
      // Auction
      case NotificationType.auctionBidPlaced:
        return NotificationDisplayIcon.shoppingBag;
      case NotificationType.auctionWaitingSettlement:
        return NotificationDisplayIcon.gavel;
      case NotificationType.auctionSellerHasWinner:
        return NotificationDisplayIcon.gavel;
      case NotificationType.auctionEndedNoWinner:
        return NotificationDisplayIcon.warning;
      case NotificationType.auctionBnrSeller:
      case NotificationType.auctionBnrWinner:
        return NotificationDisplayIcon.warning;
      // Moderation
      case NotificationType.moderationContentRemoved:
      case NotificationType.moderationCommentRemoved:
      case NotificationType.moderationForSaleRemoved:
      case NotificationType.moderationAuctionRemoved:
        return NotificationDisplayIcon.delete;
      case NotificationType.moderationUserSuspended:
      case NotificationType.moderationWarningIssued:
        return NotificationDisplayIcon.warning;
      case NotificationType.moderationContentRestored:
      case NotificationType.moderationCommentRestored:
      case NotificationType.moderationForSaleRestored:
      case NotificationType.moderationAuctionRestored:
      case NotificationType.moderationUserRestored:
        return NotificationDisplayIcon.checkCircle;
      // Support
      case NotificationType.supportTicketCreated:
      case NotificationType.supportTicketResolved:
      case NotificationType.supportTicketClosed:
      case NotificationType.supportTicketWaitingUser:
      case NotificationType.supportTicketUserResponded:
        return NotificationDisplayIcon.supportAgent;
      // Social
      case NotificationType.chatMessage:
        return NotificationDisplayIcon.chat;
      case NotificationType.contentLiked:
        return NotificationDisplayIcon.favorite;
      case NotificationType.comment:
      case NotificationType.commentReply:
        return NotificationDisplayIcon.comment;
      case NotificationType.contentMentioned:
        return NotificationDisplayIcon.alternateEmail;
      case NotificationType.userFollowed:
      case NotificationType.sellerResponse:
        return NotificationDisplayIcon.article;
      // External product review
      case NotificationType.externalProductReviewApproved:
        return NotificationDisplayIcon.checkCircle;
      case NotificationType.externalProductReviewRejected:
      case NotificationType.externalProductReviewHidden:
        return NotificationDisplayIcon.cancel;
      case NotificationType.externalProductReviewRequestChanges:
        return NotificationDisplayIcon.article;
      // Marketing / system
      case NotificationType.promotion:
      case NotificationType.announcement:
        return NotificationDisplayIcon.campaign;
      case NotificationType.systemMaintenance:
        return NotificationDisplayIcon.build;
    }
  }

  /// Get color for notification type
  NotificationDisplayColor _getColor(NotificationType type) {
    switch (type) {
      // Order
      case NotificationType.orderCreated:
      case NotificationType.orderCreatedBuyer:
      case NotificationType.orderPaidBuyer:
      case NotificationType.orderPaid:
      case NotificationType.orderShipped:
        return NotificationDisplayColor.green;
      case NotificationType.orderExpired:
      case NotificationType.orderCancelled:
      case NotificationType.orderCancelledTimeout:
        return NotificationDisplayColor.red;
      case NotificationType.orderCompleted:
        return NotificationDisplayColor.green;
      case NotificationType.orderRefunded:
      case NotificationType.orderPartiallyRefunded:
        return NotificationDisplayColor.orange;
      case NotificationType.orderDisputeOpen:
        return NotificationDisplayColor.deepOrange;
      case NotificationType.orderConfirmationExtended:
      case NotificationType.orderOverdueReminderSeller:
      case NotificationType.orderOverdueReminderBuyer:
        return NotificationDisplayColor.orange;
      // Refund / dispute
      case NotificationType.refundOpened:
        return NotificationDisplayColor.orange;
      case NotificationType.refundApproved:
        return NotificationDisplayColor.green;
      case NotificationType.refundRejected:
        return NotificationDisplayColor.red;
      case NotificationType.refundEscalated:
        return NotificationDisplayColor.deepOrange;
      case NotificationType.disputeOpened:
        return NotificationDisplayColor.deepOrange;
      case NotificationType.disputeResolved:
        return NotificationDisplayColor.blue;
      case NotificationType.disputeOverdue:
        return NotificationDisplayColor.orange;
      case NotificationType.disputeTimeoutEscalation:
        return NotificationDisplayColor.red;
      // Withdrawal
      case NotificationType.withdrawalRequested:
        return NotificationDisplayColor.orange;
      case NotificationType.withdrawalApproved:
      case NotificationType.withdrawalCompleted:
        return NotificationDisplayColor.green;
      case NotificationType.withdrawalRejected:
      case NotificationType.withdrawalFailed:
        return NotificationDisplayColor.red;
      // Negotiation
      case NotificationType.negotiationStarted:
      case NotificationType.negotiationMessageSent:
        return NotificationDisplayColor.blue;
      case NotificationType.negotiationAccepted:
        return NotificationDisplayColor.green;
      case NotificationType.negotiationExpired:
        return NotificationDisplayColor.red;
      case NotificationType.negotiationCancelled:
        return NotificationDisplayColor.red;
      // Verification
      case NotificationType.verificationDocumentApproved:
      case NotificationType.sellerVerificationApproved:
        return NotificationDisplayColor.green;
      case NotificationType.verificationDocumentRejected:
      case NotificationType.sellerVerificationRejected:
        return NotificationDisplayColor.red;
      case NotificationType.sellerVerificationSubmitted:
      case NotificationType.sellerVerificationNeedsResubmission:
      case NotificationType.sellerVerificationSuspended:
      case NotificationType.sellerVerificationUnderInvestigation:
        return NotificationDisplayColor.orange;
      case NotificationType.sellerVerificationRevoked:
        return NotificationDisplayColor.red;
      case NotificationType.sellerVerificationRestored:
        return NotificationDisplayColor.green;
      // Seller subscription
      case NotificationType.sellerSubscriptionExpiring:
        return NotificationDisplayColor.orange;
      case NotificationType.sellerSubscriptionExpired:
        return NotificationDisplayColor.red;
      // Seller tier — B1
      case NotificationType.sellerTierUpgraded:
        return NotificationDisplayColor.green;
      case NotificationType.sellerTierDowngraded:
        return NotificationDisplayColor.orange;
      // Auction
      case NotificationType.auctionBidPlaced:
        return NotificationDisplayColor.green;
      case NotificationType.auctionWaitingSettlement:
        return NotificationDisplayColor.green;
      case NotificationType.auctionSellerHasWinner:
        return NotificationDisplayColor.green;
      case NotificationType.auctionEndedNoWinner:
        return NotificationDisplayColor.orange;
      case NotificationType.auctionBnrSeller:
        return NotificationDisplayColor.orange;
      case NotificationType.auctionBnrWinner:
        return NotificationDisplayColor.red;
      // Moderation
      case NotificationType.moderationContentRemoved:
      case NotificationType.moderationCommentRemoved:
      case NotificationType.moderationForSaleRemoved:
      case NotificationType.moderationAuctionRemoved:
      case NotificationType.moderationUserSuspended:
      case NotificationType.moderationWarningIssued:
        return NotificationDisplayColor.red;
      case NotificationType.moderationContentRestored:
      case NotificationType.moderationCommentRestored:
      case NotificationType.moderationForSaleRestored:
      case NotificationType.moderationAuctionRestored:
      case NotificationType.moderationUserRestored:
        return NotificationDisplayColor.green;
      // Support
      case NotificationType.supportTicketCreated:
      case NotificationType.supportTicketResolved:
      case NotificationType.supportTicketClosed:
      case NotificationType.supportTicketWaitingUser:
      case NotificationType.supportTicketUserResponded:
        return NotificationDisplayColor.blue;
      // Social
      case NotificationType.chatMessage:
        return NotificationDisplayColor.blue;
      case NotificationType.contentLiked:
        return NotificationDisplayColor.pink;
      case NotificationType.comment:
      case NotificationType.commentReply:
        return NotificationDisplayColor.indigo;
      case NotificationType.contentMentioned:
        return NotificationDisplayColor.deepOrange;
      case NotificationType.userFollowed:
      case NotificationType.sellerResponse:
        return NotificationDisplayColor.teal;
      // External product review
      case NotificationType.externalProductReviewApproved:
        return NotificationDisplayColor.green;
      case NotificationType.externalProductReviewRejected:
      case NotificationType.externalProductReviewHidden:
        return NotificationDisplayColor.red;
      case NotificationType.externalProductReviewRequestChanges:
        return NotificationDisplayColor.orange;
      // Marketing / system
      case NotificationType.promotion:
      case NotificationType.announcement:
        return NotificationDisplayColor.cyan;
      case NotificationType.systemMaintenance:
        return NotificationDisplayColor.grey;
    }
  }
}
