// SUPPORT TICKET THREAD — MESSAGE TIMESTAMP AUTHORITY CONVERGENCE GATE.
//
// Owner decision (authority purge):
//   Support Ticket message timestamps follow the Indonesian product locale
//   through the canonical TimeFormatService — consistent with the
//   ticket-created header on the same screen and the support list/card
//   surfaces.
//
// Removed authority (must not return):
//   private `_formatTimestamp` English relative-time engine
//     Just now / Nm ago / Nh ago / Nd ago / d/M/yyyy fallback
//
// This gate proves the REAL SupportTicketThreadScreen message-card
// timestamp renders canonical Indonesian TimeFormatService output across
// representative boundaries, remains inside the card, and carries no
// residue of the deleted English engine.
//
// It does NOT audit or change AppBar/sender-label English copy, support
// localization ARB, status/category labels, or any other timestamp family.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/shared/domain/services/time_format_service.dart';
import 'package:hishumi/domains/system/support/domain/domain.dart';
import 'package:hishumi/domains/system/support/domain/repositories/support_repository.dart';
import 'package:hishumi/domains/system/support/presentation/presentation.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/providers/authenticated_account_provider.dart';

const String _ticketId = 'thread-ts-1';
const String _screenPath =
    'lib/domains/system/support/presentation/screens/support_ticket_thread_screen.dart';

const String _messageBody = 'Halo, ada yang bisa kami bantu?';

class _FakeSupportRepository implements SupportRepository {
  _FakeSupportRepository({required this.messages});

  final List<SupportMessage> messages;

  @override
  Future<Result<SupportTicket>> getTicket(String ticketId) async =>
      Result.success(_ticket(ticketId));

  @override
  Future<Result<List<SupportMessage>>> getMessages(
    String ticketId, {
    int limit = 100,
  }) async => Result.success(messages);

  @override
  Future<Result<List<SupportEvent>>> getEvents(
    String ticketId, {
    int limit = 100,
  }) async => Result.success(const <SupportEvent>[]);

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
  Future<Result<List<SupportTicket>>> getMyTickets({int limit = 50}) =>
      throw UnimplementedError();

  @override
  Future<Result<void>> reopenTicket(ReopenTicketRequest request) =>
      throw UnimplementedError();

  @override
  Future<Result<void>> sendMessage({
    required String ticketId,
    required String message,
  }) => throw UnimplementedError();
}

SupportTicket _ticket(String id) => SupportTicket(
  id: id,
  userId: 'buyer-1',
  userName: 'Buyer',
  category: SupportCategory.paymentIssue,
  priority: SupportPriority.medium,
  status: SupportStatus.open,
  subject: 'Payment issue',
  createdAt: DateTime.now().subtract(const Duration(days: 30)),
);

AuthUser _user() => AuthUser(
  id: 'buyer-1',
  createdAt: DateTime.utc(2026, 8, 1),
  updatedAt: DateTime.utc(2026, 8, 1),
  email: 'buyer@example.com',
  username: 'buyer',
  isEmailVerified: true,
  accountStatus: AccountStatus.active,
  roles: const [],
  provider: AuthProvider.email,
);

SupportMessage _message({
  required String id,
  required DateTime createdAt,
  SupportSenderType senderType = SupportSenderType.admin,
}) => SupportMessage(
  id: id,
  roomId: _ticketId,
  senderId: 'admin-1',
  senderType: senderType,
  messageType: SupportMessageType.text,
  body: _messageBody,
  createdAt: createdAt,
);

/// Representative canonical boundaries. Expected strings are computed from
/// the SAME closed TimeFormatService the production call site uses, with an
/// explicit `now` reference so the matrix stays deterministic.
List<({String id, Duration age, String expected})> _fixtures() {
  final DateTime now = DateTime.now();
  final TimeFormatService service = const TimeFormatService();

  String expected(Duration age) =>
      service.formatTimeAgo(now.subtract(age), now: now);

  return <({String id, Duration age, String expected})>[
    (
      id: 'sub-minute',
      age: const Duration(seconds: 30),
      expected: expected(const Duration(seconds: 30)),
    ),
    (
      id: 'minutes',
      age: const Duration(minutes: 5),
      expected: expected(const Duration(minutes: 5)),
    ),
    (
      id: 'hours',
      age: const Duration(hours: 2),
      expected: expected(const Duration(hours: 2)),
    ),
    (
      id: 'days',
      age: const Duration(days: 3),
      expected: expected(const Duration(days: 3)),
    ),
    (
      id: 'weeks',
      age: const Duration(days: 14),
      expected: expected(const Duration(days: 14)),
    ),
    (
      id: 'months',
      age: const Duration(days: 120),
      expected: expected(const Duration(days: 120)),
    ),
    (
      id: 'years',
      age: const Duration(days: 1095),
      expected: expected(const Duration(days: 1095)),
    ),
  ];
}

Future<void> _pumpThread(
  WidgetTester tester, {
  required List<SupportMessage> messages,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      retry: (retryCount, error) => null,
      overrides: [
        authenticatedUserProvider.overrideWith((ref) => _user()),
        supportRepositoryProvider.overrideWithValue(
          _FakeSupportRepository(messages: messages),
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.lightTheme,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: const SupportTicketThreadScreen(ticketId: _ticketId),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  group('Support Ticket Thread message timestamp — TimeFormatService authority', () {
    // Pin the Owner-locked progression shapes once at suite start.
    test('TimeFormatService progression shapes are the expected Indonesian set', () {
      final TimeFormatService service = const TimeFormatService();
      final DateTime now = DateTime.now();

      expect(
        service.formatTimeAgo(now.subtract(const Duration(seconds: 30)), now: now),
        'baru saja',
      );
      expect(
        service.formatTimeAgo(now.subtract(const Duration(minutes: 5)), now: now),
        '5 menit lalu',
      );
      expect(
        service.formatTimeAgo(now.subtract(const Duration(hours: 2)), now: now),
        '2 jam lalu',
      );
      expect(
        service.formatTimeAgo(now.subtract(const Duration(days: 3)), now: now),
        '3 hari lalu',
      );
      expect(
        service.formatTimeAgo(now.subtract(const Duration(days: 14)), now: now),
        '2 minggu lalu',
      );
      expect(
        service.formatTimeAgo(now.subtract(const Duration(days: 120)), now: now),
        '4 bulan lalu',
      );
      expect(
        service.formatTimeAgo(now.subtract(const Duration(days: 1095)), now: now),
        '3 tahun lalu',
      );
    });

    for (final fixture in _fixtures()) {
      testWidgets(
        'message-card renders canonical Indonesian timestamp: ${fixture.id}',
        (tester) async {
          final DateTime createdAt = DateTime.now().subtract(fixture.age);
          final String expected = const TimeFormatService().formatTimeAgo(
            createdAt,
          );

          await _pumpThread(
            tester,
            messages: <SupportMessage>[
              _message(
                id: 'msg-${fixture.id}',
                createdAt: createdAt,
                senderType: fixture.id == 'sub-minute'
                    ? SupportSenderType.user
                    : SupportSenderType.admin,
              ),
            ],
          );

          expect(
            tester.takeException(),
            isNull,
            reason:
                'thread message-card layout must not throw for ${fixture.id}',
          );

          // Message body remains represented on the real screen.
          expect(find.text(_messageBody), findsOneWidget);

          // Canonical Indonesian timestamp renders inside the message card.
          expect(
            find.text(expected),
            findsOneWidget,
            reason:
                'canonical TimeFormatService output for ${fixture.id} must '
                'render (expected "$expected")',
          );

          final Text timestamp = tester.widget<Text>(find.text(expected));
          expect(timestamp.data, isNot(contains('ago')));
          expect(timestamp.data, isNot(equals('Just now')));
          expect(
            timestamp.data,
            isNot(contains('/')),
            reason: 'no local d/M/yyyy fallback may render',
          );

          // Explicit negative proof against the deleted English engine shapes.
          expect(find.text('Just now'), findsNothing);
          expect(find.textContaining(' ago'), findsNothing);
        },
      );
    }

    testWidgets(
      'single boundary message renders the exact canonical string in-card',
      (tester) async {
        final DateTime now = DateTime.now();
        const Duration age = Duration(minutes: 5);
        final String expected = const TimeFormatService().formatTimeAgo(
          now.subtract(age),
          now: now,
        );
        expect(expected, '5 menit lalu');

        await _pumpThread(
          tester,
          messages: <SupportMessage>[
            _message(
              id: 'msg-minutes',
              createdAt: now.subtract(age),
              senderType: SupportSenderType.user,
            ),
          ],
        );

        expect(tester.takeException(), isNull);
        expect(find.text(expected), findsOneWidget);
        expect(find.text(_messageBody), findsOneWidget);
        // Card still carries the canonical sender + body presentation.
        expect(find.text('You'), findsOneWidget);
        expect(find.text('Support Team'), findsNothing);
      },
    );

    test('production source has no local English relative-time engine', () {
      final String source = File(_screenPath)
          .readAsStringSync()
          .replaceAll('\r\n', '\n');

      expect(source.contains('_formatTimestamp'), isFalse);
      expect(source.contains('Just now'), isFalse);
      expect(source.contains('m ago'), isFalse);
      expect(source.contains('h ago'), isFalse);
      expect(source.contains('d ago'), isFalse);
      // Sole production caller routes through the canonical authority.
      expect(
        source.contains(
          'TimeFormatService().formatTimeAgo(message.createdAt)',
        ),
        isTrue,
      );
    });
  });
}
