library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/services/notification_navigation_service.dart';

class _RecordingNavigationHandler implements NavigationHandler {
  String? lastExternalProductId;
  int sellerDashboardCallCount = 0;

  @override
  void navigateToExternalProductDetail(String productId) {
    lastExternalProductId = productId;
  }

  @override
  void navigateToSellerDashboard() {
    sellerDashboardCallCount++;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

NotificationEntity _notification({
  required NotificationType type,
  Map<String, dynamic> data = const {},
}) {
  return NotificationEntity(
    id: 'notif-ep-1',
    userId: 'user-1',
    type: type,
    title: 'External Product Review',
    body: 'Your external product was reviewed',
    data: data,
    isRead: false,
    createdAt: DateTime(2026, 6, 10),
  );
}

void main() {
  group('External product review notification navigation', () {
    late _RecordingNavigationHandler handler;
    late NotificationNavigationService service;

    setUp(() {
      handler = _RecordingNavigationHandler();
      service = NotificationNavigationService(handler);
    });

    testWidgets('approved with externalProductId routes to product detail', (
      tester,
    ) async {
      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: Text('notif-list'))),
      );
      final context = tester.element(find.text('notif-list'));

      await service.handleNotificationTap(
        context,
        _notification(
          type: NotificationType.externalProductReviewApproved,
          data: {
            'externalProductId': 'ep-abc123',
            'title': 'My Product',
            'reviewStatus': 'approved',
          },
        ),
      );

      expect(handler.lastExternalProductId, 'ep-abc123');
      expect(handler.sellerDashboardCallCount, 0);
    });

    testWidgets('rejected with externalProductId routes to product detail', (
      tester,
    ) async {
      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: Text('notif-list'))),
      );
      final context = tester.element(find.text('notif-list'));

      await service.handleNotificationTap(
        context,
        _notification(
          type: NotificationType.externalProductReviewRejected,
          data: {
            'externalProductId': 'ep-rejected',
            'title': 'My Product',
            'reviewStatus': 'rejected',
            'reason': 'Bad URL',
          },
        ),
      );

      expect(handler.lastExternalProductId, 'ep-rejected');
      expect(handler.sellerDashboardCallCount, 0);
    });

    testWidgets(
      'request_changes with externalProductId routes to product detail',
      (tester) async {
        await tester.pumpWidget(
          const MaterialApp(home: Scaffold(body: Text('notif-list'))),
        );
        final context = tester.element(find.text('notif-list'));

        await service.handleNotificationTap(
          context,
          _notification(
            type: NotificationType.externalProductReviewRequestChanges,
            data: {
              'externalProductId': 'ep-changes',
              'title': 'My Product',
              'reviewStatus': 'request_changes',
            },
          ),
        );

        expect(handler.lastExternalProductId, 'ep-changes');
        expect(handler.sellerDashboardCallCount, 0);
      },
    );

    testWidgets('hidden with externalProductId routes to product detail', (
      tester,
    ) async {
      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: Text('notif-list'))),
      );
      final context = tester.element(find.text('notif-list'));

      await service.handleNotificationTap(
        context,
        _notification(
          type: NotificationType.externalProductReviewHidden,
          data: {
            'externalProductId': 'ep-hidden',
            'title': 'My Product',
            'reviewStatus': 'hidden',
          },
        ),
      );

      expect(handler.lastExternalProductId, 'ep-hidden');
      expect(handler.sellerDashboardCallCount, 0);
    });

    testWidgets('missing externalProductId falls back to seller dashboard', (
      tester,
    ) async {
      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: Text('notif-list'))),
      );
      final context = tester.element(find.text('notif-list'));

      await service.handleNotificationTap(
        context,
        _notification(
          type: NotificationType.externalProductReviewApproved,
          data: {
            'title': 'My Product',
            'reviewStatus': 'approved',
            // no externalProductId key
          },
        ),
      );

      expect(handler.lastExternalProductId, isNull);
      expect(handler.sellerDashboardCallCount, 1);
    });

    testWidgets('empty externalProductId falls back to seller dashboard', (
      tester,
    ) async {
      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: Text('notif-list'))),
      );
      final context = tester.element(find.text('notif-list'));

      await service.handleNotificationTap(
        context,
        _notification(
          type: NotificationType.externalProductReviewRejected,
          data: {
            'externalProductId': '',
            'title': 'My Product',
            'reviewStatus': 'rejected',
          },
        ),
      );

      expect(handler.lastExternalProductId, isNull);
      expect(handler.sellerDashboardCallCount, 1);
    });

    testWidgets('null data payload falls back to seller dashboard', (
      tester,
    ) async {
      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: Text('notif-list'))),
      );
      final context = tester.element(find.text('notif-list'));

      await service.handleNotificationTap(
        context,
        NotificationEntity(
          id: 'notif-null',
          userId: 'user-1',
          type: NotificationType.externalProductReviewApproved,
          title: 'Review',
          body: 'Body',
          data: null,
          isRead: false,
          createdAt: DateTime(2026, 6, 10),
        ),
      );

      expect(handler.lastExternalProductId, isNull);
      expect(handler.sellerDashboardCallCount, 1);
    });
  });
}
