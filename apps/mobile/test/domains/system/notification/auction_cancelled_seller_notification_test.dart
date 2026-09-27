// Scope B — auction.cancelled.seller notification parity (mobile side).
//
// Backend wires auction.cancelled → notification type
// "auction.cancelled.seller" (seller notified ONLY on subscription-expired
// auto-cancel, routed by entity.CancelReason.NotifiesSeller()). This locks:
//   1. the enum value + wire string,
//   2. fromString resolution (must NOT fall back to announcement),
//   3. display metadata (cancel icon, red),
//   4. routing → auction detail via navigateToAuction(auctionId),
//   5. graceful no-op when auctionId is missing from data.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/interfaces/i_notification_trigger.dart';
import 'package:labuda/core/navigation/navigation_handler.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/services/notification_display_service.dart';
import 'package:labuda/domains/system/notification/services/notification_navigation_service.dart';

NotificationEntity _notification({
  required NotificationType type,
  required Map<String, dynamic> data,
}) {
  return NotificationEntity(
    id: 'notif-1',
    userId: 'user-1',
    type: type,
    title: 'Lelang Dibatalkan Otomatis',
    body: 'Langganan Anda telah berakhir sehingga lelang ini dibatalkan.',
    data: data,
    isRead: false,
    createdAt: DateTime(2026, 9, 26),
  );
}

class _CapturingNavigationHandler implements NavigationHandler {
  String? lastAuctionId;

  @override
  void navigateToAuction(String auctionId) {
    lastAuctionId = auctionId;
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  group('NotificationType — auctionCancelledSeller enum + wire string', () {
    test('auctionCancelledSeller has value auction.cancelled.seller', () {
      expect(
        NotificationType.auctionCancelledSeller.value,
        'auction.cancelled.seller',
      );
    });

    test('fromString resolves the wire string (not announcement fallback)', () {
      final t = NotificationType.fromString('auction.cancelled.seller');
      expect(t, NotificationType.auctionCancelledSeller);
      expect(
        t,
        isNot(NotificationType.announcement),
        reason: 'must not fall back to announcement fallback',
      );
    });
  });

  group('NotificationDisplayService — auctionCancelledSeller', () {
    test('renders cancel icon with red color', () {
      const service = NotificationDisplayService();
      final metadata = service
          .getDisplayMetadata(NotificationType.auctionCancelledSeller);

      expect(metadata.icon, NotificationDisplayIcon.cancel);
      expect(metadata.color, NotificationDisplayColor.red);
    });
  });

  group('NotificationNavigationService — auctionCancelledSeller', () {
    testWidgets('routes to auction detail via auctionId data', (tester) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(body: Text('home')),
        ),
      );
      final context = tester.element(find.text('home'));

      final handler = _CapturingNavigationHandler();
      final service = NotificationNavigationService(handler);

      await service.handleNotificationTap(
        context,
        _notification(
          type: NotificationType.auctionCancelledSeller,
          data: {'auctionId': 'auction-42'},
        ),
      );

      expect(handler.lastAuctionId, 'auction-42');
    });

    testWidgets('no-ops gracefully when auctionId is missing', (tester) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(body: Text('home')),
        ),
      );
      final context = tester.element(find.text('home'));

      final handler = _CapturingNavigationHandler();
      final service = NotificationNavigationService(handler);

      await service.handleNotificationTap(
        context,
        _notification(
          type: NotificationType.auctionCancelledSeller,
          data: const {},
        ),
      );

      expect(handler.lastAuctionId, isNull);
    });
  });
}
