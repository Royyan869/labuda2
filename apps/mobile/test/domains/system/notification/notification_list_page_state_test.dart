import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/interfaces/i_notification_trigger.dart';
import 'package:labuda/domains/system/notification/notification.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/shared/widgets/loading_indicator.dart';
import 'package:labuda/shared/widgets/page_error_state.dart';

const _uid = 'u1';

/// Scripted repository: the screen runs the REAL [notificationListProvider]
/// (canonical authority), so every subscription (initial, refresh, retry) is
/// observable at the producer boundary.
class _ScriptedNotificationRepository implements INotificationRepository {
  _ScriptedNotificationRepository({required this.onGetNotifications});

  Stream<Result<List<NotificationEntity>>> Function(int call)
  onGetNotifications;
  int listCalls = 0;

  @override
  Stream<Result<List<NotificationEntity>>> getNotifications({
    required String userId,
    int limit = 20,
  }) {
    listCalls++;
    return onGetNotifications(listCalls);
  }

  @override
  Future<Result<void>> markAsRead({required String notificationId}) async =>
      Result.success(null);

  @override
  Future<Result<void>> markAsReadByEntity({
    required String userId,
    required String entityType,
    required String entityId,
  }) async => Result.success(null);

  @override
  Future<Result<void>> markAllAsRead({required String userId}) async =>
      Result.success(null);

  @override
  Stream<Result<int>> getUnreadCount({required String userId}) =>
      const Stream.empty();

  @override
  Future<Result<NotificationPreferenceEntity>> getPreferences({
    required String userId,
  }) => throw UnimplementedError();

  @override
  Future<Result<void>> updatePreferences({
    required NotificationPreferenceEntity preferences,
  }) => throw UnimplementedError();

  @override
  Future<Result<void>> deleteNotification({
    required String notificationId,
  }) async => Result.success(null);

  @override
  Future<Result<void>> deleteAllNotifications({required String userId}) async =>
      Result.success(null);

  @override
  Future<Result<int>> deleteReadNotifications({required String userId}) async =>
      Result.success(0);
}

NotificationEntity _n(String id, {String title = 'Title'}) =>
    NotificationEntity(
      id: id,
      userId: _uid,
      type: NotificationType.orderCreated,
      title: '$title $id',
      body: 'Body $id',
      isRead: false,
      createdAt: DateTime(2026, 1, 1),
    );

Widget _wrap(INotificationRepository repository) {
  return ProviderScope(
    // Retry is disabled so a failed stream does not schedule timers.
    retry: (retryCount, error) => null,
    overrides: [notificationRepositoryProvider.overrideWithValue(repository)],
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('id'),
      home: const NotificationListScreen(userId: _uid),
    ),
  );
}

Future<void> _pump(
  WidgetTester tester,
  INotificationRepository repository, {
  bool settle = true,
}) async {
  await tester.pumpWidget(_wrap(repository));
  if (settle) await tester.pumpAndSettle();
}

void main() {
  group('NotificationListScreen — canonical authority and initial state', () {
    testWidgets('initial request shows LoadingIndicator, never empty/error', (
      tester,
    ) async {
      final gate = Completer<void>();
      final repository = _ScriptedNotificationRepository(
        onGetNotifications: (_) async* {
          await gate.future;
          yield Result.success([_n('n1')]);
        },
      );
      await _pump(tester, repository, settle: false);
      await tester.pump();
      await tester.pump();

      expect(find.byType(LoadingIndicator), findsOneWidget);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete();
      await tester.pumpAndSettle();
      expect(find.text('Title n1'), findsOneWidget);
      expect(repository.listCalls, 1);
    });

    testWidgets('initial success with data renders the list', (tester) async {
      final repository = _ScriptedNotificationRepository(
        onGetNotifications: (_) =>
            Stream.value(Result.success([_n('n1'), _n('n2')])),
      );
      await _pump(tester, repository);

      expect(find.text('Title n1'), findsOneWidget);
      expect(find.text('Title n2'), findsOneWidget);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);
    });

    testWidgets('successful zero-result shows EmptyState', (tester) async {
      final repository = _ScriptedNotificationRepository(
        onGetNotifications: (_) =>
            Stream.value(Result.success(const <NotificationEntity>[])),
      );
      await _pump(tester, repository);

      expect(find.byType(EmptyState), findsOneWidget);
      expect(find.text('Belum ada notifikasi'), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.byType(LoadingIndicator), findsNothing);
    });
  });

  group('NotificationListScreen — initial failure and retry', () {
    testWidgets('initial failure shows PageErrorState, never Empty', (
      tester,
    ) async {
      final repository = _ScriptedNotificationRepository(
        onGetNotifications: (_) =>
            Stream.value(Result.error('INTERNAL_SERVER_ERROR: sql: no rows')),
      );
      await _pump(tester, repository);

      // CANONICAL error surface: controlled localized copy only — the raw
      // backend text must never reach the screen, and failure is never Empty.
      expect(find.byType(PageErrorState), findsOneWidget);
      expect(find.text('Terjadi Kesalahan'), findsOneWidget);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsOneWidget,
      );
      expect(find.text('Coba Lagi'), findsOneWidget);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.textContaining('INTERNAL_SERVER_ERROR'), findsNothing);
      expect(find.textContaining('sql:'), findsNothing);
      expect(find.text('Failed to Load Notifications'), findsNothing);
    });

    testWidgets('retry re-executes the canonical request', (tester) async {
      final repository = _ScriptedNotificationRepository(
        onGetNotifications: (call) => call == 1
            ? Stream.value(Result.error('boom-initial'))
            : Stream.value(Result.success([_n('n1')])),
      );
      await _pump(tester, repository);
      expect(find.byType(PageErrorState), findsOneWidget);
      expect(repository.listCalls, 1);

      await tester.tap(find.widgetWithText(ElevatedButton, 'Coba Lagi'));
      await tester.pumpAndSettle();

      expect(repository.listCalls, 2);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.text('Title n1'), findsOneWidget);
    });
  });

  group('NotificationListScreen — refresh preserves data', () {
    testWidgets('existing notifications stay visible during refresh', (
      tester,
    ) async {
      final gate = Completer<void>();
      final repository = _ScriptedNotificationRepository(
        onGetNotifications: (call) => call == 1
            ? Stream.value(Result.success([_n('n1')]))
            : (() async* {
                await gate.future;
                yield Result.success([_n('n2')]);
              })(),
      );
      await _pump(tester, repository);
      expect(find.text('Title n1'), findsOneWidget);

      await tester.fling(find.byType(ListView), const Offset(0, 400), 1000);
      for (var i = 0; i < 50 && repository.listCalls < 2; i++) {
        await tester.pump(const Duration(milliseconds: 50));
      }
      expect(repository.listCalls, 2);

      // Refresh must not clear the list into full loading.
      expect(find.text('Title n1'), findsOneWidget);
      expect(find.byType(LinearProgressIndicator), findsOneWidget);
      expect(find.byType(LoadingIndicator), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete();
      await tester.pumpAndSettle();

      expect(find.text('Title n2'), findsOneWidget);
      expect(find.text('Title n1'), findsNothing);
      expect(find.byType(LinearProgressIndicator), findsNothing);
    });

    testWidgets(
      'refresh failure keeps data with inline banner, retry recovers',
      (tester) async {
        final repository = _ScriptedNotificationRepository(
          onGetNotifications: (call) {
            if (call == 1) return Stream.value(Result.success([_n('n1')]));
            if (call == 2) {
              return Stream.value(Result.error('HTTP 500: boom-refresh'));
            }
            return Stream.value(Result.success([_n('n2')]));
          },
        );
        await _pump(tester, repository);
        expect(find.text('Title n1'), findsOneWidget);

        await tester.fling(find.byType(ListView), const Offset(0, 400), 1000);
        await tester.pumpAndSettle();

        // Valid data is preserved; failure renders inline, never full-page.
        expect(find.text('Title n1'), findsOneWidget);
        expect(find.byType(PageErrorState), findsNothing);
        expect(
          find.text('Data belum bisa dimuat. Silakan coba lagi.'),
          findsOneWidget,
        );
        expect(find.widgetWithText(TextButton, 'Coba Lagi'), findsOneWidget);
        expect(find.textContaining('boom-refresh'), findsNothing);

        await tester.tap(find.widgetWithText(TextButton, 'Coba Lagi'));
        await tester.pumpAndSettle();

        expect(repository.listCalls, 3);
        expect(find.text('Title n2'), findsOneWidget);
        expect(find.text('Title n1'), findsNothing);
        expect(
          find.text('Data belum bisa dimuat. Silakan coba lagi.'),
          findsNothing,
        );
      },
    );
  });

  group('NotificationListScreen — negative proof (static contract)', () {
    String screenSource() => File(
      'lib/domains/system/notification/presentation/screens/notification_list_screen.dart',
    ).readAsStringSync().replaceAll('\r\n', '\n');

    String contentSource() => File(
      'lib/domains/system/notification/presentation/widgets/notification_list_content.dart',
    ).readAsStringSync().replaceAll('\r\n', '\n');

    test('canonical renderers own every page state', () {
      final src = screenSource();
      expect(src.contains('LoadingIndicator('), isTrue);
      expect(src.contains('PageErrorState('), isTrue);
      expect(src.contains('EmptyState('), isTrue);
      expect(contentSource().contains('LinearProgressIndicator'), isTrue);
    });

    test('no raw spinner, custom error, or raw error rendering remains', () {
      final src = screenSource();
      expect(src.contains('CircularProgressIndicator'), isFalse);
      expect(src.contains('_buildErrorState'), isFalse);
      expect(src.contains('error.toString()'), isFalse);
      expect(src.contains('Text(error'), isFalse);
      expect(src.contains('Failed to Load Notifications'), isFalse);
    });

    test('one canonical read authority is consumed by the screen', () {
      final src = screenSource();
      expect(
        src.contains('ref.watch(notificationListProvider(userId))'),
        isTrue,
      );
      expect(src.contains('filteredNotificationsProvider'), isFalse);
      // Filtering is presentation-level over the canonical value.
      expect(src.contains('.where((n) => filterState.filter.matches'), isTrue);
    });

    test('the duplicate filtered authority is purged', () {
      expect(
        File(
          'lib/domains/system/notification/presentation/providers/filtered_notification_provider.dart',
        ).existsSync(),
        isFalse,
      );
      expect(contentSource().contains('ref.invalidate'), isFalse);
    });

    test('the duplicate custom empty renderer is purged', () {
      expect(
        File(
          'lib/domains/system/notification/presentation/widgets/notification_empty_state_widget.dart',
        ).existsSync(),
        isFalse,
      );
      final barrel = File(
        'lib/domains/system/notification/notification.dart',
      ).readAsStringSync();
      expect(barrel.contains('notification_empty_state_widget.dart'), isFalse);
      expect(contentSource().contains('NotificationEmptyStateWidget'), isFalse);
    });
  });
}
