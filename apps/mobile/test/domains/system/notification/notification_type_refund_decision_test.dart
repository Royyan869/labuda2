// H2-C — Refund decision notification type + routing tests.
//
// Verifies:
//   1. The 2 refund decision NotificationType enum values exist with correct
//      backend event string mappings (refund.approved / .rejected).
//   2. NotificationType.fromString correctly resolves each refund decision
//      string to the right enum value (not the announcement fallback).
//   3. NotificationDisplayService returns correct icon/color for each type.

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/interfaces/i_notification_trigger.dart';
import 'package:labuda/domains/system/notification/domain/services/notification_display_service.dart';

void main() {
  // ============================================================================
  // 1. ENUM VALUES EXIST WITH CORRECT WIRE STRINGS
  // ============================================================================
  group('NotificationType — refund decision enum values', () {
    test('refundApproved has value refund.approved', () {
      expect(NotificationType.refundApproved.value, 'refund.approved');
    });

    test('refundRejected has value refund.rejected', () {
      expect(NotificationType.refundRejected.value, 'refund.rejected');
    });
  });

  // ============================================================================
  // 2. fromString RESOLVES CORRECTLY
  // ============================================================================
  group('NotificationType.fromString — refund decision wire strings', () {
    test('refund.approved → refundApproved', () {
      final t = NotificationType.fromString('refund.approved');
      expect(t, NotificationType.refundApproved);
      expect(
        t,
        isNot(NotificationType.announcement),
        reason: 'must not fall back to announcement fallback',
      );
    });

    test('refund.rejected → refundRejected', () {
      final t = NotificationType.fromString('refund.rejected');
      expect(t, NotificationType.refundRejected);
      expect(
        t,
        isNot(NotificationType.announcement),
        reason: 'must not fall back to announcement fallback',
      );
    });
  });

  // ============================================================================
  // 3. DISPLAY SERVICE — CORRECT ICON + COLOR
  // ============================================================================
  group('NotificationDisplayService — refund decision metadata', () {
    const service = NotificationDisplayService();

    test('refundApproved → checkCircle + green', () {
      final meta = service.getDisplayMetadata(NotificationType.refundApproved);
      expect(meta.icon, NotificationDisplayIcon.checkCircle);
      expect(meta.color, NotificationDisplayColor.green);
    });

    test('refundRejected → cancel + red', () {
      final meta = service.getDisplayMetadata(NotificationType.refundRejected);
      expect(meta.icon, NotificationDisplayIcon.cancel);
      expect(meta.color, NotificationDisplayColor.red);
    });
  });
}
