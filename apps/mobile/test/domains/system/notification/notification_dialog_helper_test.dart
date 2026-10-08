// Dialog authority — Slice #2: notification confirmation facade.
//
// The helper owns notification-specific SIDE-EFFECTS (provider call, list
// invalidation, result snackbar) and delegates dialog COMPOSITION to the
// canonical `AppDialog`. This file locks:
//  1. POSITIVE: confirm performs the action; cancel does nothing; the
//     read-count guard shows a snackbar and no dialog; the destructive action
//     carries `scheme.error`.
//  2. NEGATIVE: the helper no longer composes a raw dialog, the dead
//     showErrorDialog is gone, the app-bar consumer carries no raw dialog, and
//     the dead barrel export stays removed.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart' show AppTheme;
import 'package:labuda/core/interfaces/i_notification_trigger.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/presentation/helpers/notification_dialog_helper.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_list_provider.dart';

const _helperPath =
    'lib/domains/system/notification/presentation/helpers/'
    'notification_dialog_helper.dart';
const _appBarPath =
    'lib/domains/system/notification/presentation/widgets/'
    'notification_list_app_bar.dart';
const _barrelPath = 'lib/domains/system/notification/notification.dart';

NotificationEntity _notification(String id, {required bool isRead}) =>
    NotificationEntity(
      id: id,
      userId: 'u1',
      type: NotificationType.orderCreated,
      title: 't',
      body: 'b',
      isRead: isRead,
      createdAt: DateTime(2026, 1, 1),
    );

/// Builds the harness inline so the `overrides` list gets its type by
/// inference — Riverpod 3 does not publicly export the `Override` type name.
Widget _harness({
  required List<String> deleteAllCalls,
  required List<String> deleteReadCalls,
  required void Function(BuildContext, WidgetRef) onOpen,
}) => ProviderScope(
  overrides: [
    deleteAllNotificationsProvider.overrideWithValue((userId) async {
      deleteAllCalls.add(userId);
    }),
    deleteReadNotificationsProvider.overrideWithValue((userId) async {
      deleteReadCalls.add(userId);
      return 0;
    }),
  ],
  child: MaterialApp(
    theme: AppTheme.lightTheme,
    home: Scaffold(
      body: Builder(
        builder: (context) => Consumer(
          builder: (context, ref, _) => ElevatedButton(
            onPressed: () => onOpen(context, ref),
            child: const Text('open'),
          ),
        ),
      ),
    ),
  ),
);

void main() {
  late List<String> deleteAllCalls;
  late List<String> deleteReadCalls;

  setUp(() {
    deleteAllCalls = <String>[];
    deleteReadCalls = <String>[];
  });

  group('notification confirmation facade — positive', () {
    testWidgets('delete-all: confirm deletes, reports and closes', (
      tester,
    ) async {
      await tester.pumpWidget(
        _harness(
          deleteAllCalls: deleteAllCalls,
          deleteReadCalls: deleteReadCalls,
          onOpen: (context, ref) =>
              NotificationDialogHelper.showDeleteAllConfirmation(
                context,
                ref,
                'u1',
              ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      expect(find.text('Delete All Notifications'), findsOneWidget);
      expect(find.text('Batal'), findsOneWidget);
      expect(find.text('Delete All'), findsOneWidget);

      final scheme = Theme.of(
        tester.element(find.byType(AlertDialog)),
      ).colorScheme;
      final confirm = tester.widget<ElevatedButton>(
        find.widgetWithText(ElevatedButton, 'Delete All'),
      );
      expect(
        confirm.style?.backgroundColor?.resolve(const <WidgetState>{}),
        scheme.error,
        reason: 'delete-all is a destructive confirmation',
      );

      await tester.tap(find.text('Delete All'));
      await tester.pumpAndSettle();

      expect(deleteAllCalls, <String>['u1']);
      expect(find.text('Semua notifikasi dihapus'), findsOneWidget);
      expect(find.byType(AlertDialog), findsNothing);
      await tester.pump(const Duration(seconds: 5));
    });

    testWidgets('delete-all: cancel performs no action', (tester) async {
      await tester.pumpWidget(
        _harness(
          deleteAllCalls: deleteAllCalls,
          deleteReadCalls: deleteReadCalls,
          onOpen: (context, ref) =>
              NotificationDialogHelper.showDeleteAllConfirmation(
                context,
                ref,
                'u1',
              ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();

      expect(deleteAllCalls, isEmpty);
      expect(find.byType(AlertDialog), findsNothing);
    });

    testWidgets('delete-read: zero read shows a snackbar, never a dialog', (
      tester,
    ) async {
      await tester.pumpWidget(
        _harness(
          deleteAllCalls: deleteAllCalls,
          deleteReadCalls: deleteReadCalls,
          onOpen: (context, ref) =>
              NotificationDialogHelper.showDeleteReadConfirmation(
                context,
                ref,
                'u1',
                [_notification('1', isRead: false)],
              ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      expect(find.byType(AlertDialog), findsNothing);
      expect(find.text('No read notifications to delete'), findsOneWidget);
      expect(deleteReadCalls, isEmpty);
      await tester.pump(const Duration(seconds: 5));
    });

    testWidgets('delete-read: confirm deletes the read batch', (tester) async {
      await tester.pumpWidget(
        _harness(
          deleteAllCalls: deleteAllCalls,
          deleteReadCalls: deleteReadCalls,
          onOpen: (context, ref) =>
              NotificationDialogHelper.showDeleteReadConfirmation(
                context,
                ref,
                'u1',
                [
                  _notification('1', isRead: true),
                  _notification('2', isRead: true),
                  _notification('3', isRead: false),
                ],
              ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      expect(find.text('Delete Read Notifications'), findsOneWidget);
      expect(
        find.text('Delete 2 read notifications? This action cannot be undone.'),
        findsOneWidget,
      );
      await tester.tap(find.text('Delete'));
      await tester.pumpAndSettle();

      expect(deleteReadCalls, <String>['u1']);
      expect(find.text('2 notifikasi dihapus'), findsOneWidget);
      expect(find.byType(AlertDialog), findsNothing);
      await tester.pump(const Duration(seconds: 5));
    });
  });

  group('notification confirmation facade — negative gate', () {
    test('the helper composes no raw dialog and holds no type authority', () {
      final source = File(_helperPath).readAsStringSync();
      for (final forbidden in <String>[
        'AlertDialog(',
        'showDialog(',
        'AppType',
        'fontSize:',
        'Colors.',
        'Color(0x',
      ]) {
        expect(
          source.contains(forbidden),
          isFalse,
          reason: '$forbidden is not allowed in the dialog facade',
        );
      }
    });

    test('the dead showErrorDialog helper stays deleted', () {
      final source = File(_helperPath).readAsStringSync();
      expect(source.contains('showErrorDialog'), isFalse);
    });

    test('the app-bar consumer carries no raw dialog and keeps the facade', () {
      final source = File(_appBarPath).readAsStringSync();
      expect(source.contains('AlertDialog('), isFalse);
      expect(source.contains('showDialog('), isFalse);
      expect(
        source.contains('NotificationDialogHelper.showDeleteAllConfirmation'),
        isTrue,
      );
      expect(
        source.contains('NotificationDialogHelper.showDeleteReadConfirmation'),
        isTrue,
      );
      expect(source.contains('NotificationDialogHelper.markAllAsRead'), isTrue);
    });

    test('the dead barrel export of the helper stays removed', () {
      final barrel = File(_barrelPath).readAsStringSync();
      expect(barrel.contains('notification_dialog_helper.dart'), isFalse);
    });

    test('the raw-dialog detector can actually fail (negative proof)', () {
      const planted = 'return AlertDialog(title: Text(t));';
      expect(planted.contains('AlertDialog('), isTrue);
      expect(
        'await AppDialog.confirm(context: context)'.contains('showDialog('),
        isFalse,
      );
    });
  });
}
