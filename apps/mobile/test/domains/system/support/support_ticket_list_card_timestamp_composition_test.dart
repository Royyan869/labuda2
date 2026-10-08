// SUPPORT TICKET LIST / CARD — RELATIVE TIMESTAMP COMPOSITION GATE.
//
// Targets:
//   1. SupportTicketsListScreen → _SupportTicketListItem header Row
//      field: updatedAt ?? createdAt → TimeFormatService
//   2. SupportTicketCardRefactored header Row
//      field: lastMessageAt ?? createdAt → TimeFormatService
//
// Authority (unchanged): TimeFormatService for relative last-activity age.
// This gate proves composition safety only. It does NOT touch Support Thread
// or Support Activity Timeline (CLOSED).
//
// Matrix: 320/360/412/500 × 1.0/1.3/2.0
// Fixtures: long Indonesian category/status/priority + long relative strings.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';
import 'package:labuda/domains/system/support/domain/repositories/support_repository.dart';
import 'package:labuda/domains/system/support/presentation/presentation.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/providers/authenticated_account_provider.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

const String _uid = 'buyer-1';
const String _ticketId = 't-compose-list';

/// Long Indonesian labels that compete for header width.
const SupportCategory _category = SupportCategory.paymentIssue; // Masalah Pembayaran
const SupportStatus _status = SupportStatus.waitingUser; // Menunggu User
const SupportPriority _priority = SupportPriority.urgent; // Mendesak

/// Long relative strings from the canonical engine.
final DateTime _now = DateTime.now();
final DateTime _lastActivity = _now.subtract(const Duration(days: 120));
final String _expectedRelative = const TimeFormatService().formatTimeAgo(
  _lastActivity,
);
final String _longRelative = const TimeFormatService().formatTimeAgo(
  _now.subtract(const Duration(days: 1095)),
);

class _FakeSupportRepository implements SupportRepository {
  _FakeSupportRepository(this.tickets);

  final List<SupportTicket> tickets;

  @override
  Future<Result<List<SupportTicket>>> getMyTickets({int limit = 50}) async =>
      Result.success(tickets);

  @override
  Future<Result<SupportTicket>> getTicket(String ticketId) async =>
      Result.success(tickets.first);

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

SupportTicket _ticket({
  required DateTime activityAt,
  DateTime? lastMessageAt,
}) => SupportTicket(
  id: _ticketId,
  userId: _uid,
  userName: 'Buyer Name Long Enough',
  category: _category,
  priority: _priority,
  status: _status,
  subject:
      'Pembayaran refund transfer bank virtual account BCA yang belum masuk '
      'ke rekening penjual setelah konfirmasi pesanan selesai',
  description: 'Deskripsi tiket dukungan yang cukup panjang',
  createdAt: activityAt.subtract(const Duration(days: 1)),
  updatedAt: activityAt,
  lastMessageAt: lastMessageAt,
);

Future<void> _pumpAt(
  WidgetTester tester,
  Size surface,
  double scale, {
  required Widget child,
}) async {
  await tester.binding.setSurfaceSize(surface);
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(
    ProviderScope(
      retry: (retryCount, error) => null,
      overrides: [
        authenticatedUserProvider.overrideWith(
          (ref) => AuthUser(
            id: _uid,
            createdAt: DateTime.utc(2026, 8, 1),
            updatedAt: DateTime.utc(2026, 8, 1),
            email: 'buyer@example.com',
            username: 'buyer',
            isEmailVerified: true,
            accountStatus: AccountStatus.active,
            roles: const [],
            provider: AuthProvider.email,
          ),
        ),
        supportRepositoryProvider.overrideWithValue(
          _FakeSupportRepository(<SupportTicket>[
            _ticket(activityAt: _lastActivity),
          ]),
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.lightTheme,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: MediaQuery(
          data: MediaQueryData(
            size: surface,
            textScaler: TextScaler.linear(scale),
          ),
          child: Scaffold(body: child),
        ),
      ),
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 50));
}

bool _underFlexible(WidgetTester tester, Finder textFinder) {
  final Finder ancestors = find.ancestor(
    of: textFinder,
    matching: find.byType(Flexible),
  );
  return ancestors.evaluate().isNotEmpty;
}

void main() {
  group('Support Tickets List — relative timestamp composition', () {
    test('formatter authority is TimeFormatService (canonical relative)', () {
      expect(_expectedRelative, '4 bulan lalu');
      expect(_longRelative, '3 tahun lalu');
      final String src = File(
        'lib/domains/system/support/presentation/screens/'
        'support_tickets_list_screen.dart',
      ).readAsStringSync();
      expect(
        src.contains(
          'TimeFormatService().formatTimeAgo(lastActivity)',
        ),
        isTrue,
      );
      expect(src.contains('DateFormat('), isFalse);
    });

    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets(
          'list header no layout exception at ${width}dp @scale $scale', (
          tester,
        ) async {
          await _pumpAt(
            tester,
            Size(width, 900),
            scale,
            child: const SupportTicketsListScreen(),
          );

          expect(
            tester.takeException(),
            isNull,
            reason:
                'support list timestamp composition must not throw at '
                '${width}dp @scale $scale',
          );

          expect(find.text(_expectedRelative), findsWidgets);
          expect(find.text('Masalah Pembayaran'), findsWidgets);
          expect(find.text('Menunggu User'), findsWidgets);

          final Finder stamp = find.text(_expectedRelative).first;
          final Rect rect = tester.getRect(stamp);
          expect(rect.left, greaterThanOrEqualTo(-0.5));
          expect(
            rect.right,
            lessThanOrEqualTo(width + 0.5),
            reason:
                'list timestamp wider than surface at '
                '${width}dp @scale $scale',
          );
        });
      }
    }

    testWidgets(
      'list timestamp is flex-bounded with compact strategy at 320 @ 2.0', (
      tester,
    ) async {
      await _pumpAt(
        tester,
        const Size(320, 900),
        2.0,
        child: const SupportTicketsListScreen(),
      );
      expect(tester.takeException(), isNull);

      final Finder stampFinder = find.text(_expectedRelative).first;
      expect(stampFinder, findsOneWidget);
      expect(
        _underFlexible(tester, stampFinder),
        isTrue,
        reason:
            'list relative timestamp must be Flexible-bounded in the header Row',
      );
      final Text timestamp = tester.widget<Text>(stampFinder);
      expect(timestamp.maxLines, 1);
      expect(timestamp.overflow, TextOverflow.ellipsis);
    });
  });

  group('Support Ticket Card — relative timestamp composition', () {
    test('formatter authority is TimeFormatService (canonical relative)', () {
      final String src = File(
        'lib/domains/system/support/presentation/widgets/'
        'support_ticket_card.dart',
      ).readAsStringSync();
      expect(
        src.contains('TimeFormatService().formatTimeAgo'),
        isTrue,
      );
      expect(src.contains('DateFormat('), isFalse);
    });

    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets(
          'card header no layout exception at ${width}dp @scale $scale', (
          tester,
        ) async {
          await _pumpAt(
            tester,
            Size(width, 900),
            scale,
            child: SupportTicketCardRefactored(
              ticket: _ticket(
                activityAt: _lastActivity,
                lastMessageAt: _lastActivity,
              ),
            ),
          );

          expect(
            tester.takeException(),
            isNull,
            reason:
                'support card timestamp composition must not throw at '
                '${width}dp @scale $scale',
          );

          expect(find.text(_expectedRelative), findsWidgets);
          expect(find.text('Mendesak'), findsWidgets);
          expect(find.text('Masalah Pembayaran'), findsWidgets);

          final Finder stamp = find.text(_expectedRelative).first;
          final Rect rect = tester.getRect(stamp);
          expect(rect.left, greaterThanOrEqualTo(-0.5));
          expect(
            rect.right,
            lessThanOrEqualTo(width + 0.5),
            reason:
                'card timestamp wider than surface at '
                '${width}dp @scale $scale',
          );
        });
      }
    }

    testWidgets(
      'card timestamp is flex-bounded with compact strategy at 320 @ 2.0', (
      tester,
    ) async {
      await _pumpAt(
        tester,
        const Size(320, 900),
        2.0,
        child: SupportTicketCardRefactored(
          ticket: _ticket(
            activityAt: _lastActivity,
            lastMessageAt: _lastActivity,
          ),
        ),
      );
      expect(tester.takeException(), isNull);

      final Finder stampFinder = find.text(_expectedRelative).first;
      expect(stampFinder, findsOneWidget);
      expect(
        _underFlexible(tester, stampFinder),
        isTrue,
        reason:
            'card relative timestamp must be Flexible-bounded in the header Row',
      );
      final Text timestamp = tester.widget<Text>(stampFinder);
      expect(timestamp.maxLines, 1);
      expect(timestamp.overflow, TextOverflow.ellipsis);
    });
  });
}
