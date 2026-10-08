// NOTIFICATION ITEM WIDGET — HORIZONTAL COMPOSITION REGRESSION GATE.
//
// Contract: lib/shared/widgets/docs/horizontal_composition_contract.md
//
// The horizontal foundation gate proved a REAL overflow in
// NotificationItemWidget: the timestamp row (notification_item_widget.dart)
// held a non-flex dynamic time `Text` beside bounded badges, overflowing
// 32 px at 320 dp x 1.3 (available 204, content 236).
//
// Convergence (horizontal composition only — no data/behaviour change):
//   clock icon  : bounded, intrinsic
//   timestamp   : Flexible, one-line ellipsis  (the only element that yields)
//   status badge: bounded, intrinsic, ADJACENT (stays visible)
//
// This gate runs the SAME matrix as the foundation (320/360/412/500 x
// 1.0/1.3/2.0), uses long/dynamic timestamps the data model can produce, and
// asserts: no layout exception, widget renders once, title/body/timestamp/badge
// metadata all remain present, and every relevant box stays inside the surface.
// The original 320 x 1.3 case is pinned explicitly.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/interfaces/i_notification_trigger.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/presentation/widgets/notification_item_widget.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

const String _title =
    'Pesanan baru dari Koi Farm Nusantara Jaya Sentosa menunggu konfirmasi '
    'penjual sekarang juga';
const String _body =
    'Pembeli telah membuat pesanan dan menunggu konfirmasi Anda sebelum '
    'batas waktu berakhir.';

NotificationEntity _notification({
  required NotificationType type,
  required bool isRead,
  required Duration age,
}) {
  return NotificationEntity(
    id: 'n-1',
    userId: 'u-1',
    type: type,
    title: _title,
    body: _body,
    isRead: isRead,
    createdAt: DateTime.now().subtract(age),
  );
}

/// A bounded slot (mirrors the foundation gate's `surface - 24`) inside a
/// vertical scroll, so a vertical overflow cannot masquerade as horizontal.
Widget _harness({
  required Size surface,
  required double scale,
  required NotificationEntity notification,
}) {
  return MaterialApp(
    theme: AppTheme.lightTheme,
    home: MediaQuery(
      data: MediaQueryData(
        size: surface,
        textScaler: TextScaler.linear(scale),
      ),
      child: Scaffold(
        body: Center(
          child: SizedBox(
            width: surface.width - 24,
            child: SingleChildScrollView(
              child: NotificationItemWidget(
                notification: notification,
                onTap: () {},
              ),
            ),
          ),
        ),
      ),
    ),
  );
}

Future<void> _expectClean(
  WidgetTester tester, {
  required Size surface,
  required double scale,
  required NotificationEntity notification,
  required String? expectedBadge,
}) async {
  await tester.binding.setSurfaceSize(surface);
  addTearDown(() => tester.binding.setSurfaceSize(null));

  await tester.pumpWidget(
    _harness(surface: surface, scale: scale, notification: notification),
  );
  await tester.pump();

  final Object? exception = tester.takeException();
  expect(
    exception,
    isNull,
    reason:
        'notification overflow at ${surface.width}dp @scale $scale '
        '($exception)',
  );

  expect(find.byType(NotificationItemWidget), findsOneWidget);

  // Key metadata must remain present (not dropped to hide the overflow).
  expect(find.text(_title), findsOneWidget, reason: 'title missing');
  expect(find.text(_body), findsOneWidget, reason: 'body missing');
  expect(
    find.text(notification.timeAgo),
    findsOneWidget,
    reason: 'timestamp text missing',
  );

  // Every relevant box stays inside the surface.
  final double surfaceRight = surface.width;
  final List<Finder> boxes = <Finder>[find.byType(NotificationItemWidget)];
  if (expectedBadge != null) {
    expect(find.text(expectedBadge), findsOneWidget, reason: 'badge missing');
    boxes.add(find.text(expectedBadge));
  }
  for (final Finder f in boxes) {
    final Rect rect = tester.getRect(f);
    expect(
      rect.right,
      lessThanOrEqualTo(surfaceRight + 0.5),
      reason: 'a box exceeded the surface at ${surface.width}dp @scale $scale',
    );
  }
}

void main() {
  // The exact case the foundation gate proved failing.
  testWidgets('ORIGINAL FAILING CASE: 320dp x 1.3 no longer overflows', (
    tester,
  ) async {
    await _expectClean(
      tester,
      surface: const Size(320, 640),
      scale: 1.3,
      notification: _notification(
        type: NotificationType.orderCreated,
        isRead: false,
        age: Duration.zero,
      ),
      expectedBadge: 'BARU',
    );
  });

  group('NotificationItemWidget survives the width x text-scale matrix', () {
    final List<({String name, NotificationEntity notification, String? badge})>
    cases = <({String name, NotificationEntity notification, String? badge})>[
      (
        name: 'BARU / "Just now"',
        notification: _notification(
          type: NotificationType.orderCreated,
          isRead: false,
          age: Duration.zero,
        ),
        badge: 'BARU',
      ),
      (
        name: 'BARU / "59 minutes ago"',
        notification: _notification(
          type: NotificationType.orderCreated,
          isRead: false,
          age: const Duration(minutes: 59),
        ),
        badge: 'BARU',
      ),
      (
        // Not recent (>= 24h) and not action-required: no badge renders, but
        // the (long) relative timestamp alone must still not overflow.
        name: 'no badge / long relative ("100 weeks ago")',
        notification: _notification(
          type: NotificationType.orderCreated,
          isRead: false,
          age: const Duration(days: 700),
        ),
        badge: null,
      ),
      (
        name: 'action-required / "Perlu tindakan"',
        notification: _notification(
          type: NotificationType.moderationWarningIssued,
          isRead: false,
          age: const Duration(minutes: 59),
        ),
        badge: 'Perlu tindakan',
      ),
    ];

    for (final c in cases) {
      testWidgets(c.name, (tester) async {
        for (final double width in _widths) {
          for (final double scale in _scales) {
            await _expectClean(
              tester,
              surface: Size(width, 640),
              scale: scale,
              notification: c.notification,
              expectedBadge: c.badge,
            );
          }
        }
      });
    }
  });
}
