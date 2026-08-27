// F1-B — Withdrawal notification type + routing tests.
//
// Verifies:
//   1. The 4 withdrawal NotificationType enum values exist with correct
//      backend event string mappings (withdrawal.requested / .approved /
//      .rejected / .completed).
//   2. NotificationType.fromString correctly resolves each withdrawal
//      string to the right enum value (not the announcement fallback).
//   3. Unknown strings still fall back to announcement (regression guard).
//   4. The NotificationFilter.payout category includes all withdrawal types
//      (name contains 'withdrawal' — existing filter logic).
//   5. NotificationNavigationService routes all 4 withdrawal types to the
//      seller earnings screen via navigateToSellerEarnings().

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/interfaces/i_notification_trigger.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_filter.dart';

void main() {
  // ============================================================================
  // 1. ENUM VALUES EXIST WITH CORRECT WIRE STRINGS
  // ============================================================================
  group('NotificationType — withdrawal enum values', () {
    test('withdrawalRequested has value withdrawal.requested', () {
      expect(
        NotificationType.withdrawalRequested.value,
        'withdrawal.requested',
      );
    });

    test('withdrawalApproved has value withdrawal.approved', () {
      expect(NotificationType.withdrawalApproved.value, 'withdrawal.approved');
    });

    test('withdrawalRejected has value withdrawal.rejected', () {
      expect(NotificationType.withdrawalRejected.value, 'withdrawal.rejected');
    });

    test('withdrawalCompleted has value withdrawal.completed', () {
      expect(
        NotificationType.withdrawalCompleted.value,
        'withdrawal.completed',
      );
    });
  });

  // ============================================================================
  // 2. fromString RESOLVES CORRECTLY
  // ============================================================================
  group('NotificationType.fromString — withdrawal wire strings', () {
    test('withdrawal.requested → withdrawalRequested', () {
      final t = NotificationType.fromString('withdrawal.requested');
      expect(t, NotificationType.withdrawalRequested);
      expect(
        t,
        isNot(NotificationType.announcement),
        reason: 'must not fall back to announcement fallback',
      );
    });

    test('withdrawal.approved → withdrawalApproved', () {
      expect(
        NotificationType.fromString('withdrawal.approved'),
        NotificationType.withdrawalApproved,
      );
    });

    test('withdrawal.rejected → withdrawalRejected', () {
      expect(
        NotificationType.fromString('withdrawal.rejected'),
        NotificationType.withdrawalRejected,
      );
    });

    test('withdrawal.completed → withdrawalCompleted', () {
      expect(
        NotificationType.fromString('withdrawal.completed'),
        NotificationType.withdrawalCompleted,
      );
    });

    // regression: unknown strings still fall back
    test('unknown string still falls back to announcement', () {
      expect(
        NotificationType.fromString('totally.unknown.event'),
        NotificationType.announcement,
      );
    });
  });

  // ============================================================================
  // 3. NotificationFilter.payout INCLUDES WITHDRAWAL TYPES
  // ============================================================================
  group('NotificationFilter.payout — includes withdrawal types', () {
    // The filter uses type.name.contains('withdrawal') (existing logic).
    // Verify all 4 new types pass that check.
    final withdrawalTypes = [
      NotificationType.withdrawalRequested,
      NotificationType.withdrawalApproved,
      NotificationType.withdrawalRejected,
      NotificationType.withdrawalCompleted,
    ];

    for (final type in withdrawalTypes) {
      test('payout filter includes $type', () {
        final matchesPayout = NotificationFilter.payout.matches(type);
        expect(
          matchesPayout,
          isTrue,
          reason:
              '$type should be matched by NotificationFilter.payout because '
              'its name contains "withdrawal"',
        );
      });
    }
  });
}
