import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';
import 'package:labuda/domains/system/support/domain/repositories/support_repository.dart';
import 'package:labuda/domains/system/support/presentation/presentation.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/providers/authenticated_account_provider.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/shared/widgets/loading_indicator.dart';
import 'package:labuda/shared/widgets/page_error_state.dart';

const _uid = 'buyer-1';

/// Scripted repository: the screen runs the REAL [supportTicketsProvider]
/// (canonical authority), so every read is observable at the producer
/// boundary.
class _ScriptedSupportRepository implements SupportRepository {
  _ScriptedSupportRepository({required this.onGetMyTickets});

  Future<Result<List<SupportTicket>>> Function(int call) onGetMyTickets;
  int listCalls = 0;

  @override
  Future<Result<List<SupportTicket>>> getMyTickets({int limit = 50}) {
    listCalls++;
    return onGetMyTickets(listCalls);
  }

  @override
  Future<Result<String>> createTicket({
    required String userId,
    required String userName,
    String? userAvatar,
    required SupportCategory category,
    SupportPriority priority = SupportPriority.medium,
    String? subject,
    String? description,
    String? linkedOrderId,
  }) => throw UnimplementedError();

  @override
  Future<Result<SupportTicket>> getTicket(String ticketId) =>
      throw UnimplementedError();

  @override
  Future<Result<void>> reopenTicket(ReopenTicketRequest request) =>
      throw UnimplementedError();

  @override
  Future<Result<List<SupportMessage>>> getMessages(
    String ticketId, {
    int limit = 100,
  }) => throw UnimplementedError();

  @override
  Future<Result<void>> sendMessage({
    required String ticketId,
    required String message,
  }) => throw UnimplementedError();

  @override
  Future<Result<List<SupportEvent>>> getEvents(
    String ticketId, {
    int limit = 100,
  }) => throw UnimplementedError();
}

AuthUser _user() => AuthUser(
  id: _uid,
  createdAt: DateTime.utc(2026, 8, 1),
  updatedAt: DateTime.utc(2026, 8, 1),
  email: 'buyer@example.com',
  username: 'buyer',
  isEmailVerified: true,
  accountStatus: AccountStatus.active,
  roles: const [],
  provider: AuthProvider.email,
);

SupportTicket _ticket(String id, {String subject = 'Payment issue'}) =>
    SupportTicket(
      id: id,
      userId: _uid,
      userName: 'Buyer',
      category: SupportCategory.paymentIssue,
      priority: SupportPriority.medium,
      status: SupportStatus.open,
      subject: subject,
      description: 'desc',
      createdAt: DateTime.utc(2026, 8, 1),
    );

Widget _wrap(SupportRepository repository) {
  final user = _user();
  return ProviderScope(
    // Retry is disabled so a failed load does not schedule timers.
    retry: (retryCount, error) => null,
    overrides: [
      authenticatedUserProvider.overrideWith((ref) => user),
      supportRepositoryProvider.overrideWithValue(repository),
    ],
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('id'),
      home: const SupportTicketsListScreen(),
    ),
  );
}

Future<void> _pump(
  WidgetTester tester,
  SupportRepository repository, {
  bool settle = true,
}) async {
  await tester.pumpWidget(_wrap(repository));
  if (settle) await tester.pumpAndSettle();
}

void main() {
  group('SupportTicketsListScreen — canonical authority and initial state', () {
    testWidgets('initial request shows LoadingIndicator, never empty/error', (
      tester,
    ) async {
      final gate = Completer<Result<List<SupportTicket>>>();
      final repository = _ScriptedSupportRepository(
        onGetMyTickets: (_) => gate.future,
      );
      await _pump(tester, repository, settle: false);
      await tester.pump();
      await tester.pump();

      expect(find.byType(LoadingIndicator), findsOneWidget);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete(Result.success([_ticket('t1')]));
      await tester.pumpAndSettle();
      expect(find.text('Payment issue'), findsOneWidget);
      expect(repository.listCalls, 1);
    });

    testWidgets('successful non-empty result renders the tickets', (
      tester,
    ) async {
      final repository = _ScriptedSupportRepository(
        onGetMyTickets: (_) =>
            Future.value(Result.success([_ticket('t1'), _ticket('t2')])),
      );
      await _pump(tester, repository);

      expect(find.text('Payment issue'), findsNWidgets(2));
      expect(find.text('View Ticket'), findsNWidgets(2));
    });

    testWidgets(
      'successful zero-result shows EmptyState with first-use action',
      (tester) async {
        final repository = _ScriptedSupportRepository(
          onGetMyTickets: (_) =>
              Future.value(Result.success(const <SupportTicket>[])),
        );
        await _pump(tester, repository);

        expect(find.byType(EmptyState), findsOneWidget);
        expect(find.text('Belum ada tiket bantuan'), findsOneWidget);
        expect(find.byType(PageErrorState), findsNothing);
        expect(find.byType(LoadingIndicator), findsNothing);
        expect(find.text('Buat Tiket'), findsWidgets);
      },
    );
  });

  group('SupportTicketsListScreen — initial failure and retry', () {
    testWidgets('failure with no data shows PageErrorState, retry reloads', (
      tester,
    ) async {
      final repository = _ScriptedSupportRepository(
        onGetMyTickets: (call) => call == 1
            ? Future.value(Result.error('INTERNAL_SERVER_ERROR: sql: no rows'))
            : Future.value(Result.success([_ticket('t1')])),
      );
      await _pump(tester, repository);

      // CANONICAL error surface: safe localized copy only — the raw backend
      // text must never reach the screen.
      expect(find.byType(PageErrorState), findsOneWidget);
      expect(find.text('Terjadi Kesalahan'), findsOneWidget);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsOneWidget,
      );
      expect(find.text('Coba Lagi'), findsOneWidget);
      expect(find.textContaining('INTERNAL_SERVER_ERROR'), findsNothing);
      expect(find.textContaining('sql:'), findsNothing);
      expect(find.text('Failed to load tickets'), findsNothing);
      expect(find.byType(EmptyState), findsNothing);

      // Retry re-executes the canonical load.
      expect(repository.listCalls, 1);
      await tester.tap(find.widgetWithText(ElevatedButton, 'Coba Lagi'));
      await tester.pumpAndSettle();

      expect(repository.listCalls, 2);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.text('Payment issue'), findsOneWidget);
    });
  });

  group('SupportTicketsListScreen — refresh preserves data', () {
    testWidgets('existing tickets stay visible with refresh indicator', (
      tester,
    ) async {
      final gate = Completer<Result<List<SupportTicket>>>();
      final repository = _ScriptedSupportRepository(
        onGetMyTickets: (call) => call == 1
            ? Future.value(Result.success([_ticket('t1')]))
            : gate.future,
      );
      await _pump(tester, repository);
      expect(find.text('Payment issue'), findsOneWidget);

      await tester.fling(
        find.byType(CustomScrollView),
        const Offset(0, 400),
        1000,
      );
      // Allow the RefreshIndicator to fire onRefresh and the reload to start
      // (bounded: the gate stays open, so never settle here).
      for (var i = 0; i < 50 && repository.listCalls < 2; i++) {
        await tester.pump(const Duration(milliseconds: 50));
      }
      expect(repository.listCalls, 2);

      // Refresh must not clear the list into full loading.
      expect(find.text('Payment issue'), findsOneWidget);
      expect(find.byType(LinearProgressIndicator), findsOneWidget);
      expect(find.byType(LoadingIndicator), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete(Result.success([_ticket('t2', subject: 'Refund request')]));
      await tester.pumpAndSettle();

      expect(find.text('Refund request'), findsOneWidget);
      expect(find.text('Payment issue'), findsNothing);
      expect(find.byType(LinearProgressIndicator), findsNothing);
    });

    testWidgets(
      'refresh failure keeps rows with inline banner, retry recovers',
      (tester) async {
        final repository = _ScriptedSupportRepository(
          onGetMyTickets: (call) {
            if (call == 1) return Future.value(Result.success([_ticket('t1')]));
            if (call == 2) {
              return Future.value(Result.error('HTTP 500: boom-refresh'));
            }
            return Future.value(
              Result.success([_ticket('t2', subject: 'Refund request')]),
            );
          },
        );
        await _pump(tester, repository);
        expect(find.text('Payment issue'), findsOneWidget);

        await tester.fling(
          find.byType(CustomScrollView),
          const Offset(0, 400),
          1000,
        );
        await tester.pumpAndSettle();

        // Valid data is preserved; failure renders inline, never full-page.
        expect(find.text('Payment issue'), findsOneWidget);
        expect(find.byType(PageErrorState), findsNothing);
        expect(
          find.text('Data belum bisa dimuat. Silakan coba lagi.'),
          findsOneWidget,
        );
        expect(find.widgetWithText(TextButton, 'Coba Lagi'), findsOneWidget);
        expect(find.textContaining('boom-refresh'), findsNothing);

        // Retry executes refresh; success replaces stale data and clears banner.
        await tester.tap(find.widgetWithText(TextButton, 'Coba Lagi'));
        await tester.pumpAndSettle();

        expect(repository.listCalls, 3);
        expect(find.text('Refund request'), findsOneWidget);
        expect(find.text('Payment issue'), findsNothing);
        expect(
          find.text('Data belum bisa dimuat. Silakan coba lagi.'),
          findsNothing,
        );
      },
    );
  });

  group('SupportTicketsListScreen — negative proof (static contract)', () {
    String screenSource() => File(
      'lib/domains/system/support/presentation/screens/support_tickets_list_screen.dart',
    ).readAsStringSync().replaceAll('\r\n', '\n');

    test('canonical renderers own every page state', () {
      final src = screenSource();
      expect(src.contains('LoadingIndicator('), isTrue);
      expect(src.contains('PageErrorState('), isTrue);
      expect(src.contains('EmptyState('), isTrue);
      expect(src.contains('LinearProgressIndicator'), isTrue);
    });

    test('no raw spinner, FutureBuilder, or custom error renderer remains', () {
      final src = screenSource();
      expect(src.contains('FutureBuilder'), isFalse);
      expect(src.contains('CircularProgressIndicator('), isFalse);
      expect(src.contains('_ticketsFuture'), isFalse);
      expect(src.contains('setState'), isFalse);
    });

    test('no raw technical error reaches the widget tree', () {
      final src = screenSource();
      expect(src.contains('result?.error'), isFalse);
      expect(src.contains('result.error'), isFalse);
      expect(src.contains('error.toString()'), isFalse);
      expect(src.contains('Text(error'), isFalse);
    });

    test('one canonical list authority is consumed by the screen', () {
      final src = screenSource();
      expect(src.contains('ref.watch(supportTicketsProvider)'), isTrue);
      expect(src.contains('supportRepositoryProvider'), isFalse);
    });

    test('the dead support_state.dart is purged', () {
      expect(
        File(
          'lib/domains/system/support/presentation/providers/support_state.dart',
        ).existsSync(),
        isFalse,
      );
      final barrel = File(
        'lib/domains/system/support/support.dart',
      ).readAsStringSync();
      expect(barrel.contains('support_state.dart'), isFalse);
    });
  });
}
