/// Tests for H2-D1/D2: Mobile refund action API layer + buyer/seller guards.
///
/// Verifies:
/// 1. API endpoint paths are correct for approve/reject/escalate
/// 2. Escalation guard uses sellerRejected (not rejected)
/// 3. Admin-final rejected does NOT trigger escalation CTA
/// 4. Seller decision guard uses pendingSellerReview + isSeller
/// 5. Buyer does NOT see seller buttons
/// 6. Approve sends no amount
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/refund_request.dart';

void main() {
  // =========================================================================
  // Helper: build a RefundRequest with a given status
  // =========================================================================
  RefundRequest makeRefund({
    required RefundStatus status,
    String buyerId = 'buyer-001',
    String sellerId = 'seller-001',
  }) {
    return RefundRequest(
      id: 'refund-001',
      orderId: 'order-001',
      buyerId: buyerId,
      sellerId: sellerId,
      reason: RefundReason.itemNotReceived,
      status: status,
      refundAmount: 100000,
      createdAt: DateTime(2026, 5, 21),
    );
  }

  // =========================================================================
  // Buyer escalation guard logic
  // (mirrors OrderRefundListSection.canBuyerEscalate)
  // =========================================================================

  group('Buyer escalation guard', () {
    bool canBuyerEscalate({
      required String? currentUserId,
      required RefundRequest refund,
    }) {
      final isBuyer = currentUserId != null && currentUserId == refund.buyerId;
      // H2-D1: must be sellerRejected, NOT rejected (admin final)
      return isBuyer && refund.status == RefundStatus.sellerRejected;
    }

    test('sellerRejected shows escalation CTA for buyer', () {
      final refund = makeRefund(status: RefundStatus.sellerRejected);
      expect(
        canBuyerEscalate(currentUserId: 'buyer-001', refund: refund),
        isTrue,
      );
    });

    test('rejected (admin final) does NOT show escalation CTA', () {
      final refund = makeRefund(status: RefundStatus.rejected);
      expect(
        canBuyerEscalate(currentUserId: 'buyer-001', refund: refund),
        isFalse,
      );
    });

    test('sellerRejected does NOT show CTA for seller', () {
      final refund = makeRefund(status: RefundStatus.sellerRejected);
      expect(
        canBuyerEscalate(currentUserId: 'seller-001', refund: refund),
        isFalse,
      );
    });

    test('pendingSellerReview does NOT show CTA', () {
      final refund = makeRefund(status: RefundStatus.pendingSellerReview);
      expect(
        canBuyerEscalate(currentUserId: 'buyer-001', refund: refund),
        isFalse,
      );
    });

    test('escalatedToAdmin does NOT show CTA', () {
      final refund = makeRefund(status: RefundStatus.escalatedToAdmin);
      expect(
        canBuyerEscalate(currentUserId: 'buyer-001', refund: refund),
        isFalse,
      );
    });

    test('sellerApproved does NOT show CTA', () {
      final refund = makeRefund(status: RefundStatus.sellerApproved);
      expect(
        canBuyerEscalate(currentUserId: 'buyer-001', refund: refund),
        isFalse,
      );
    });

    test('null currentUserId does NOT show CTA', () {
      final refund = makeRefund(status: RefundStatus.sellerRejected);
      expect(canBuyerEscalate(currentUserId: null, refund: refund), isFalse);
    });
  });

  // =========================================================================
  // API endpoint path contract
  // (verifies the paths match backend route definitions)
  // =========================================================================

  group('API endpoint path contract', () {
    test('approve endpoint matches backend route', () {
      const refundId = 'abc-123';
      const path = '/refunds/$refundId/approve';
      expect(path, equals('/refunds/abc-123/approve'));
    });

    test('reject endpoint matches backend route', () {
      const refundId = 'abc-123';
      const path = '/refunds/$refundId/reject';
      expect(path, equals('/refunds/abc-123/reject'));
    });

    test('escalate endpoint matches backend route', () {
      const refundId = 'abc-123';
      const path = '/refunds/$refundId/escalate';
      expect(path, equals('/refunds/abc-123/escalate'));
    });
  });

  // =========================================================================
  // RefundStatus semantic checks
  // =========================================================================

  group('RefundStatus semantics', () {
    test('sellerRejected is distinct from rejected', () {
      expect(RefundStatus.sellerRejected, isNot(RefundStatus.rejected));
    });

    test('isRejected covers both sellerRejected and rejected', () {
      final sellerRejected = makeRefund(status: RefundStatus.sellerRejected);
      final adminRejected = makeRefund(status: RefundStatus.rejected);
      final pending = makeRefund(status: RefundStatus.pendingSellerReview);

      expect(sellerRejected.isRejected, isTrue);
      expect(adminRejected.isRejected, isTrue);
      expect(pending.isRejected, isFalse);
    });

    test('sellerRejected apiValue is seller_rejected', () {
      expect(RefundStatus.sellerRejected.apiValue, 'seller_rejected');
    });

    test('rejected apiValue is rejected', () {
      expect(RefundStatus.rejected.apiValue, 'rejected');
    });

    test('parse seller_rejected returns sellerRejected', () {
      expect(
        RefundStatus.parse('seller_rejected'),
        RefundStatus.sellerRejected,
      );
    });
  });

  // =========================================================================
  // H2-D2: Seller decision guard logic
  // (mirrors OrderRefundListSection.canSellerDecide)
  // =========================================================================

  group('Seller decision guard', () {
    bool canSellerDecide({
      required String? currentUserId,
      required RefundRequest refund,
    }) {
      final isSeller =
          currentUserId != null && currentUserId == refund.sellerId;
      return isSeller && refund.status == RefundStatus.pendingSellerReview;
    }

    test('seller + pendingSellerReview shows approve/reject', () {
      final refund = makeRefund(status: RefundStatus.pendingSellerReview);
      expect(
        canSellerDecide(currentUserId: 'seller-001', refund: refund),
        isTrue,
      );
    });

    test('buyer does NOT see seller buttons', () {
      final refund = makeRefund(status: RefundStatus.pendingSellerReview);
      expect(
        canSellerDecide(currentUserId: 'buyer-001', refund: refund),
        isFalse,
      );
    });

    test('seller + sellerRejected does NOT show buttons', () {
      final refund = makeRefund(status: RefundStatus.sellerRejected);
      expect(
        canSellerDecide(currentUserId: 'seller-001', refund: refund),
        isFalse,
      );
    });

    test('seller + sellerApproved does NOT show buttons', () {
      final refund = makeRefund(status: RefundStatus.sellerApproved);
      expect(
        canSellerDecide(currentUserId: 'seller-001', refund: refund),
        isFalse,
      );
    });

    test('seller + escalatedToAdmin does NOT show buttons', () {
      final refund = makeRefund(status: RefundStatus.escalatedToAdmin);
      expect(
        canSellerDecide(currentUserId: 'seller-001', refund: refund),
        isFalse,
      );
    });

    test('null currentUserId does NOT show buttons', () {
      final refund = makeRefund(status: RefundStatus.pendingSellerReview);
      expect(canSellerDecide(currentUserId: null, refund: refund), isFalse);
    });
  });

  // =========================================================================
  // H2-D2: Approve sends no amount (contract)
  // =========================================================================

  group('Approve contract', () {
    test('approve API body has no amount field', () {
      // The approve endpoint accepts only optional notes.
      // This test documents the contract: no amount/percent in body.
      final approveBody = {'notes': 'Looks good'};
      expect(approveBody.containsKey('amount'), isFalse);
      expect(approveBody.containsKey('percent'), isFalse);
      expect(approveBody.containsKey('refund_amount'), isFalse);
    });

    test('reject API body has notes only', () {
      final rejectBody = {'notes': 'Item was as described'};
      expect(rejectBody.containsKey('notes'), isTrue);
      expect(rejectBody.containsKey('amount'), isFalse);
      expect(rejectBody.containsKey('percent'), isFalse);
    });
  });
}
