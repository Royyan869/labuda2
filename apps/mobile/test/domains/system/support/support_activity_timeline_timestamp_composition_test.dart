// SUPPORT ACTIVITY TIMELINE — TIMESTAMP COMPOSITION GATE.
//
// Target: SupportActivityTimeline absolute event timestamps
// (`AppFormatters.formatDateTime(event.createdAt)`).
//
// Authority (unchanged):
//   Absolute date/time UI → AppFormatters.formatDateTime (id_ID).
//   Body labels/details → AppLocalizations (I18N-18).
//
// Composition under audit:
//   Row(fixed icon leading, Expanded body, bare trailing Text timestamp)
//
// This gate proves the REAL SupportActivityTimeline across
// 320/360/412/500 × 1.0/1.3/2.0 with Indonesian locale and long canonical
// absolute datetime + long competing body text. It does NOT touch Support
// Thread / List / Card timestamp consumers or TimeFormatService.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/support/domain/domain.dart';
import 'package:hishumi/domains/system/support/domain/repositories/support_repository.dart';
import 'package:hishumi/domains/system/support/presentation/presentation.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/utils/app_formatters.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

const String _ticketId = 't-compose-1';
const String _timelinePath =
    'lib/domains/system/support/presentation/widgets/'
    'support_activity_timeline.dart';

/// Longest practical Indonesian absolute datetime (Dec + late time).
final DateTime _createdAt = DateTime(2025, 12, 31, 23, 59);
final String _canonicalTimestamp = AppFormatters.formatDateTime(_createdAt);

class _FakeSupportRepository implements SupportRepository {
  _FakeSupportRepository(this.events);

  final List<SupportEvent> events;

  @override
  Future<Result<List<SupportEvent>>> getEvents(
    String ticketId, {
    int limit = 100,
  }) async => Result.success(events);

  @override
  Future<Result<List<SupportTicket>>> getMyTickets({int limit = 50}) =>
      throw UnimplementedError();

  @override
  Future<Result<SupportTicket>> getTicket(String ticketId) =>
      throw UnimplementedError();

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
}

/// Long competing body: longest localized waiting-user label + status
/// transition detail (Indonesian).
List<SupportEvent> _events() => <SupportEvent>[
  SupportEvent(
    id: 'e-waiting',
    ticketId: _ticketId,
    eventType: SupportEventType.ticketWaitingUser,
    oldStatus: 'open',
    newStatus: 'waiting_user',
    createdAt: _createdAt,
  ),
  SupportEvent(
    id: 'e-escalated',
    ticketId: _ticketId,
    eventType: SupportEventType.ticketEscalated,
    createdAt: _createdAt,
  ),
];

Future<void> _pumpAt(
  WidgetTester tester,
  Size surface,
  double scale,
) async {
  await tester.binding.setSurfaceSize(surface);
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(
    ProviderScope(
      retry: (retryCount, error) => null,
      overrides: [
        supportRepositoryProvider.overrideWithValue(
          _FakeSupportRepository(_events()),
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
          child: Scaffold(
            body: SizedBox(
              width: surface.width,
              height: surface.height,
              child: SupportActivityTimeline(ticketId: _ticketId),
            ),
          ),
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
  setUpAll(() async {
    await initializeDateFormatting('id_ID');
  });

  group('SupportActivityTimeline timestamp composition', () {
    test('canonical absolute timestamp authority is AppFormatters', () {
      expect(_canonicalTimestamp, '31 Des 2025, 23:59');
      expect(
        AppFormatters.formatDateTime(_createdAt),
        _canonicalTimestamp,
      );
      final String source = File(_timelinePath)
          .readAsStringSync()
          .replaceAll('\r\n', '\n');
      expect(
        source.contains(
          'AppFormatters.formatDateTime(event.createdAt)',
        ),
        isTrue,
      );
      // No competing local formatter in this widget.
      expect(source.contains('DateFormat('), isFalse);
      expect(source.contains('_formatDateTime'), isFalse);
    });

    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets(
          'no layout exception at ${width}dp @scale $scale', (tester) async {
          await _pumpAt(tester, Size(width, 900), scale);

          expect(
            tester.takeException(),
            isNull,
            reason:
                'timeline timestamp composition must not throw at '
                '${width}dp @scale $scale',
          );

          // Canonical Indonesian absolute timestamp remains represented.
          expect(find.text(_canonicalTimestamp), findsWidgets);

          // First-listed competing body copy remains represented (ListView
          // builds visible items only; waiting-user event is first).
          expect(find.text('Menunggu Balasan Anda'), findsWidgets);

          // Content stays inside the surface.
          final Finder stamp = find.text(_canonicalTimestamp).first;
          final Rect rect = tester.getRect(stamp);
          expect(rect.left, greaterThanOrEqualTo(-0.5));
          expect(
            rect.right,
            lessThanOrEqualTo(width + 0.5),
            reason:
                'timestamp wider than surface at ${width}dp @scale $scale',
          );
        });
      }
    }

    testWidgets(
      'tightest cell keeps timestamp flex-bounded with compact strategy', (
      tester,
    ) async {
      await _pumpAt(tester, const Size(320, 900), 2.0);
      expect(tester.takeException(), isNull);

      final Finder stampFinder = find.text(_canonicalTimestamp).first;
      expect(stampFinder, findsOneWidget);

      final Text timestamp = tester.widget<Text>(stampFinder);
      expect(timestamp.data, _canonicalTimestamp);

      // Composition contract: dynamic trailing timestamp must be flex-bounded
      // and declare maxLines:1 + ellipsis (same principle as UserHeader).
      expect(
        _underFlexible(tester, stampFinder),
        isTrue,
        reason:
            'absolute timestamp must be Flexible-bounded in the timeline Row',
      );
      expect(timestamp.maxLines, 1);
      expect(timestamp.overflow, TextOverflow.ellipsis);
    });
  });
}
