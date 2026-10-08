// I18N-18 — SUPPORT EVENT TIMELINE SURFACE CONTRACT.
//
// CANONICAL TRUTH:
//   * Event IDENTITY is the backend `event_type` wire value parsed by
//     `SupportEvent.parseEventType` — it is NOT localized and never changes
//     with the locale.
//   * Event DISPLAY copy (label + transition detail) is user-facing content
//     owned by the ONE canonical `AppLocalizations` authority and follows the
//     active app locale.
//   * The read route is canonical: ticket list -> tap -> ticket thread ->
//     Activity Timeline -> `SupportRepository.getEvents` ->
//     `GET /support/tickets/:id/events` (owner-only).
//
// The negative half is the point: the old presentation residue
// (`eventTypeLabel`, `description`, `iconName`) and any raw wire value must
// never reach the user-facing timeline.
import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';
import 'package:labuda/domains/system/support/domain/repositories/support_repository.dart';
import 'package:labuda/domains/system/support/presentation/presentation.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/generated/app_localizations_en.dart';
import 'package:labuda/generated/app_localizations_id.dart';
import 'package:labuda/shared/providers/authenticated_account_provider.dart';
import 'package:labuda/shared/widgets/loading_indicator.dart';

const _enFile = 'lib/l10n/app_en.arb';
const _idFile = 'lib/l10n/app_id.arb';
const _eventEntity =
    'lib/domains/system/support/domain/entities/support_event.dart';
const _eventResolver =
    'lib/domains/system/support/presentation/utils/support_event_label.dart';
const _timelineWidget =
    'lib/domains/system/support/presentation/widgets/support_activity_timeline.dart';
const _notifier =
    'lib/domains/system/support/presentation/providers/support_notifier.dart';
const _threadScreen =
    'lib/domains/system/support/presentation/screens/support_ticket_thread_screen.dart';
const _listScreen =
    'lib/domains/system/support/presentation/screens/support_tickets_list_screen.dart';
const _module = 'lib/core/src/router/modules/support_module.dart';
const _repositoryApi =
    'lib/domains/system/support/data/repositories/support_repository_api.dart';
const _datasource =
    'lib/domains/system/support/data/datasources/support_api_datasource.dart';
const _dto = 'lib/domains/system/support/data/dto/support_ticket_dto.dart';

/// event type -> immutable backend wire identity (must never be localized).
const Map<SupportEventType, String> _wire = <SupportEventType, String>{
  SupportEventType.ticketCreated: 'ticket_created',
  SupportEventType.ticketClaimed: 'ticket_claimed',
  SupportEventType.ticketWaitingUser: 'ticket_waiting_user',
  SupportEventType.statusChanged: 'status_changed',
  SupportEventType.priorityChanged: 'priority_changed',
  SupportEventType.categoryChanged: 'category_changed',
  SupportEventType.ticketResolved: 'ticket_resolved',
  SupportEventType.ticketClosed: 'ticket_closed',
  SupportEventType.ticketReopened: 'ticket_reopened',
  SupportEventType.adminAssigned: 'admin_assigned',
  SupportEventType.adminUnassigned: 'admin_unassigned',
  SupportEventType.ticketEscalated: 'ticket_escalated',
};

/// event type -> [Indonesian label, English label]
const Map<SupportEventType, List<String>>
_labels = <SupportEventType, List<String>>{
  SupportEventType.ticketCreated: ['Tiket Dibuat', 'Ticket Created'],
  SupportEventType.ticketClaimed: ['Tiket Ditangani', 'Ticket Claimed'],
  SupportEventType.ticketWaitingUser: [
    'Menunggu Balasan Anda',
    'Waiting for Your Reply',
  ],
  SupportEventType.statusChanged: ['Status Berubah', 'Status Changed'],
  SupportEventType.priorityChanged: ['Prioritas Berubah', 'Priority Changed'],
  SupportEventType.categoryChanged: ['Kategori Berubah', 'Category Changed'],
  SupportEventType.ticketResolved: ['Tiket Diselesaikan', 'Ticket Resolved'],
  SupportEventType.ticketClosed: ['Tiket Ditutup', 'Ticket Closed'],
  SupportEventType.ticketReopened: ['Tiket Dibuka Kembali', 'Ticket Reopened'],
  SupportEventType.adminAssigned: [
    'Agen Support Ditugaskan',
    'Support Agent Assigned',
  ],
  SupportEventType.adminUnassigned: [
    'Agen Support Dilepas',
    'Support Agent Unassigned',
  ],
  SupportEventType.ticketEscalated: [
    'Dieskalasi ke Sengketa',
    'Escalated to Dispute',
  ],
  SupportEventType.unknown: ['Aktivitas Tiket', 'Ticket Activity'],
};

String _read(String path) => File(path).readAsStringSync();

Map<String, dynamic> _arb(String path) =>
    jsonDecode(_read(path)) as Map<String, dynamic>;

SupportEvent _event(
  SupportEventType type, {
  required DateTime at,
  String? oldStatus,
  String? newStatus,
  Map<String, dynamic>? metadata,
}) => SupportEvent(
  id: 'e-${type.name}',
  ticketId: 't1',
  eventType: type,
  oldStatus: oldStatus,
  newStatus: newStatus,
  metadata: metadata,
  createdAt: at,
);

/// Scripted repository: the timeline runs the REAL canonical
/// `supportTicketEventsProvider`, so the read is observable at the producer.
class _ScriptedSupportRepository implements SupportRepository {
  _ScriptedSupportRepository({required this.onGetEvents});

  Future<Result<List<SupportEvent>>> Function(int call) onGetEvents;
  int eventCalls = 0;

  @override
  Future<Result<List<SupportEvent>>> getEvents(
    String ticketId, {
    int limit = 100,
  }) {
    eventCalls++;
    return onGetEvents(eventCalls);
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
  Future<Result<List<SupportTicket>>> getMyTickets({int limit = 50}) =>
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
}

/// Repository for the end-to-end entry-point flow: real list screen → real
/// ticket thread → real timeline, all through the canonical read pipeline.
class _FlowRepository implements SupportRepository {
  final List<String> eventTicketIds = <String>[];

  @override
  Future<Result<List<SupportTicket>>> getMyTickets({int limit = 50}) async =>
      Result.success([_ticket('t42')]);

  @override
  Future<Result<SupportTicket>> getTicket(String ticketId) async =>
      Result.success(_ticket(ticketId));

  @override
  Future<Result<List<SupportMessage>>> getMessages(
    String ticketId, {
    int limit = 100,
  }) async => Result.success(const <SupportMessage>[]);

  @override
  Future<Result<List<SupportEvent>>> getEvents(
    String ticketId, {
    int limit = 100,
  }) async {
    eventTicketIds.add(ticketId);
    return Result.success([
      _event(SupportEventType.ticketCreated, at: DateTime.utc(2026, 1, 1, 10)),
      _event(
        SupportEventType.ticketResolved,
        at: DateTime.utc(2026, 1, 2, 10),
        newStatus: 'resolved',
      ),
    ]);
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
  createdAt: DateTime.utc(2026, 1, 1),
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

GoRouter _supportRouter() => GoRouter(
  initialLocation: RoutePaths.supportTickets,
  routes: [
    GoRoute(
      path: RoutePaths.supportTickets,
      builder: (context, state) => const SupportTicketsListScreen(),
    ),
    GoRoute(
      path: RoutePaths.supportTicketThread,
      builder: (context, state) => SupportTicketThreadScreen(
        ticketId: state.pathParameters['ticketId'] ?? '',
      ),
    ),
  ],
);

/// Pumps the REAL timeline section under the canonical provider graph.
Future<void> _pumpTimeline(
  WidgetTester tester,
  SupportRepository repository,
  Locale locale, {
  bool settle = true,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      // Retry is disabled so a failed load never schedules timers.
      retry: (retryCount, error) => null,
      overrides: [supportRepositoryProvider.overrideWithValue(repository)],
      child: MaterialApp(
        theme: AppTheme.lightTheme,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: locale,
        home: const Scaffold(body: SupportActivityTimeline(ticketId: 't1')),
      ),
    ),
  );
  if (settle) await tester.pumpAndSettle();
}

List<String> _renderedTexts(WidgetTester tester) => tester
    .widgetList<Text>(find.byType(Text))
    .map((t) => (t.data ?? '').toUpperCase())
    .toList();

void main() {
  final en = AppLocalizationsEn();
  final id = AppLocalizationsId();

  setUpAll(() async {
    await initializeDateFormatting('id_ID');
  });

  group('A. localization proof — every event label follows the locale', () {
    test('Indonesian resources', () {
      for (final entry in _labels.entries) {
        expect(
          entry.key.label(id),
          entry.value[0],
          reason: '${entry.key.name} must render its Indonesian resource',
        );
      }
    });

    test('English resources', () {
      for (final entry in _labels.entries) {
        expect(
          entry.key.label(en),
          entry.value[1],
          reason: '${entry.key.name} must render its English resource',
        );
      }
    });

    test('the mapping covers the full canonical event taxonomy', () {
      expect(_labels.keys.toSet(), SupportEventType.values.toSet());
    });
  });

  group(
    'B. runtime locale proof — the real timeline follows active locale',
    () {
      testWidgets('the timeline renders the active-locale event label', (
        tester,
      ) async {
        for (final locale in const [Locale('id'), Locale('en')]) {
          for (final entry in _labels.entries) {
            final repository = _ScriptedSupportRepository(
              onGetEvents: (_) => Future.value(
                Result.success([
                  _event(entry.key, at: DateTime.utc(2026, 1, 1, 12)),
                ]),
              ),
            );
            await _pumpTimeline(tester, repository, locale);
            final expected = locale.languageCode == 'id'
                ? entry.value[0]
                : entry.value[1];
            expect(
              find.text(expected),
              findsOneWidget,
              reason: '${entry.key.name} @ $locale must render "$expected"',
            );
          }
        }
      });

      testWidgets('locale switching changes the rendered event label', (
        tester,
      ) async {
        final repository = _ScriptedSupportRepository(
          onGetEvents: (_) => Future.value(
            Result.success([
              _event(
                SupportEventType.ticketResolved,
                at: DateTime.utc(2026, 1, 1),
              ),
            ]),
          ),
        );
        await _pumpTimeline(tester, repository, const Locale('id'));
        expect(find.text('Tiket Diselesaikan'), findsOneWidget);
        expect(find.text('Ticket Resolved'), findsNothing);

        await _pumpTimeline(tester, repository, const Locale('en'));
        expect(find.text('Ticket Resolved'), findsOneWidget);
        expect(find.text('Tiket Diselesaikan'), findsNothing);
      });

      testWidgets(
        'a status transition renders both sides from canonical labels',
        (tester) async {
          final repository = _ScriptedSupportRepository(
            onGetEvents: (_) => Future.value(
              Result.success([
                _event(
                  SupportEventType.statusChanged,
                  at: DateTime.utc(2026, 1, 1),
                  oldStatus: 'waiting_user',
                  newStatus: 'in_progress',
                ),
              ]),
            ),
          );
          await _pumpTimeline(tester, repository, const Locale('id'));
          expect(find.text('Dari Menunggu User ke Diproses'), findsOneWidget);

          await _pumpTimeline(tester, repository, const Locale('en'));
          expect(find.text('From Waiting User to In Progress'), findsOneWidget);
        },
      );

      testWidgets(
        'priority and category transitions use their own authorities',
        (tester) async {
          final repository = _ScriptedSupportRepository(
            onGetEvents: (_) => Future.value(
              Result.success([
                _event(
                  SupportEventType.priorityChanged,
                  at: DateTime.utc(2026, 1, 1),
                  metadata: {
                    'old_priority': 'medium',
                    'new_priority': 'urgent',
                  },
                ),
                _event(
                  SupportEventType.categoryChanged,
                  at: DateTime.utc(2026, 1, 2),
                  metadata: {
                    'old_category': 'order_issue',
                    'new_category': 'payment_issue',
                  },
                ),
              ]),
            ),
          );
          await _pumpTimeline(tester, repository, const Locale('id'));
          expect(find.text('Dari Sedang ke Mendesak'), findsOneWidget);
          expect(
            find.text('Dari Masalah Pesanan ke Masalah Pembayaran'),
            findsOneWidget,
          );
        },
      );

      testWidgets('an unrecognised transition identity is omitted, never raw', (
        tester,
      ) async {
        final repository = _ScriptedSupportRepository(
          onGetEvents: (_) => Future.value(
            Result.success([
              _event(
                SupportEventType.statusChanged,
                at: DateTime.utc(2026, 1, 1),
                oldStatus: 'legacy_status',
                newStatus: 'in_progress',
              ),
            ]),
          ),
        );
        await _pumpTimeline(tester, repository, const Locale('en'));
        expect(find.text('Status Changed'), findsOneWidget);
        expect(find.textContaining('legacy_status'), findsNothing);
        expect(find.textContaining('LEGACY_STATUS'), findsNothing);
      });
    },
  );

  group('C. identity preservation — the wire value is the sole identity', () {
    test('every canonical backend event_type maps to its identity', () {
      for (final entry in _wire.entries) {
        expect(
          SupportEvent.parseEventType(entry.value),
          entry.key,
          reason: '${entry.value} must resolve to ${entry.key.name}',
        );
      }
    });

    test('an unknown event_type degrades to the explicit unknown identity', () {
      expect(
        SupportEvent.parseEventType('not_a_real_event'),
        SupportEventType.unknown,
      );
    });

    test('identity never equals a localized display label', () {
      final labels = _labels.values.expand((v) => v).toSet();
      for (final entry in _wire.entries) {
        expect(labels, isNot(contains(entry.value)));
      }
    });

    test('the API DTO delegates to the ONE canonical event-type parser', () {
      final src = _read(_dto);
      expect(src.contains('SupportEvent.parseEventType(eventType)'), isTrue);
      expect(src.contains('_parseEventType'), isFalse);
    });
  });

  group('D. raw display leak — no wire identity reaches the timeline', () {
    testWidgets('no rendered text is a raw wire value or enum name', (
      tester,
    ) async {
      for (final locale in const [Locale('id'), Locale('en')]) {
        for (final entry in _wire.entries) {
          final repository = _ScriptedSupportRepository(
            onGetEvents: (_) => Future.value(
              Result.success([
                _event(
                  entry.key,
                  at: DateTime.utc(2026, 1, 1),
                  oldStatus: 'open',
                  newStatus: 'closed',
                  metadata: {
                    'old_priority': 'low',
                    'new_priority': 'urgent',
                    'old_category': 'order_issue',
                    'new_category': 'other',
                  },
                ),
              ]),
            ),
          );
          await _pumpTimeline(tester, repository, locale);
          final rendered = _renderedTexts(tester);
          final rawForms = <String>{
            entry.value.toUpperCase(),
            entry.value.replaceAll('_', '').toUpperCase(),
            entry.key.name.toUpperCase(),
            'UNKNOWN',
          };
          for (final raw in rawForms) {
            expect(
              rendered.contains(raw),
              isFalse,
              reason: 'raw identity "$raw" leaked to the timeline ($locale)',
            );
          }
        }
      }
    });

    test('the timeline never renders the purged presentation residue', () {
      final src = _read(_timelineWidget);
      expect(src.contains('eventTypeLabel'), isFalse);
      expect(src.contains('event.description'), isFalse);
      expect(src.contains('iconName'), isFalse);
    });
  });

  group('E. obsolete residue — the dead presentation metadata is gone', () {
    test('the event entity declares no label/description/icon getter', () {
      final src = _read(_eventEntity).replaceAll('\r\n', '\n');
      expect(src.contains('eventTypeLabel'), isFalse);
      expect(src.contains('iconName'), isFalse);
      expect(RegExp(r'String get description').hasMatch(src), isFalse);
      for (final label in _labels.values.expand((v) => v)) {
        expect(
          src,
          isNot(contains("'$label'")),
          reason: 'the entity re-hardcodes "$label"',
        );
      }
    });

    test('no support-domain file still references the residue', () {
      final offenders = <String>[];
      for (final file in Directory(
        'lib/domains/system/support',
      ).listSync(recursive: true).whereType<File>()) {
        if (!file.path.endsWith('.dart')) continue;
        final src = _read(file.path);
        if (src.contains('eventTypeLabel') || src.contains('iconName')) {
          offenders.add(file.path);
        }
      }
      expect(
        offenders,
        isEmpty,
        reason: 'obsolete residue resurfaced: $offenders',
      );
    });
  });

  group('F. single localization authority', () {
    test('every supportEvent* key exists in both ARBs at parity', () {
      final enKeys = _arb(_enFile);
      final idKeys = _arb(_idFile);
      for (final key in const [
        'supportEventTicketCreated',
        'supportEventTicketClaimed',
        'supportEventTicketWaitingUser',
        'supportEventStatusChanged',
        'supportEventPriorityChanged',
        'supportEventCategoryChanged',
        'supportEventTicketResolved',
        'supportEventTicketClosed',
        'supportEventTicketReopened',
        'supportEventAdminAssigned',
        'supportEventAdminUnassigned',
        'supportEventTicketEscalated',
        'supportEventUnknown',
        'supportEventTransition',
        'supportTimelineTitle',
        'supportTimelineEmpty',
      ]) {
        expect(enKeys.containsKey(key), isTrue, reason: 'en missing $key');
        expect(idKeys.containsKey(key), isTrue, reason: 'id missing $key');
      }
    });

    test('each Indonesian event label is authored exactly once', () {
      final values = _arb(_idFile).entries
          .where((e) => e.key.startsWith('supportEvent'))
          .map((e) => e.value)
          .whereType<String>()
          .toList();
      for (final label in _labels.values.map((v) => v[0])) {
        expect(
          values.where((v) => v == label).length,
          1,
          reason: 'duplicate authority for "$label" in $_idFile',
        );
      }
    });

    test('no ARB supportEvent* value is a raw wire identity', () {
      for (final path in const [_enFile, _idFile]) {
        final arb = _arb(path);
        final values = arb.entries
            .where((e) => e.key.startsWith('supportEvent'))
            .map((e) => e.value)
            .whereType<String>()
            .toList();
        for (final wire in _wire.values) {
          expect(
            values,
            isNot(contains(wire)),
            reason: '$path exposes raw identity "$wire" as user content',
          );
        }
      }
    });

    test('the resolver declares no label copy of its own', () {
      final src = _read(_eventResolver);
      for (final label in _labels.values.expand((v) => v)) {
        expect(
          src,
          isNot(contains("'$label'")),
          reason: 'the resolver re-hardcodes "$label"',
        );
        expect(
          src,
          isNot(contains('"$label"')),
          reason: 'the resolver re-hardcodes "$label"',
        );
      }
    });

    test('the resolver is exhaustive and collision-free', () {
      final labels = <String>[];
      for (final type in SupportEventType.values) {
        final value = type.label(id);
        expect(value, isNotEmpty);
        labels.add(value);
      }
      expect(labels.toSet().length, SupportEventType.values.length);
    });
  });

  group('G. empty / error / loading — canonical section states', () {
    testWidgets('a first request in flight shows the canonical loader', (
      tester,
    ) async {
      final gate = Completer<Result<List<SupportEvent>>>();
      final repository = _ScriptedSupportRepository(
        onGetEvents: (_) => gate.future,
      );
      await _pumpTimeline(
        tester,
        repository,
        const Locale('id'),
        settle: false,
      );
      await tester.pump();
      await tester.pump();

      expect(find.byType(LoadingIndicator), findsOneWidget);

      gate.complete(
        Result.success([
          _event(SupportEventType.ticketCreated, at: DateTime.utc(2026, 1, 1)),
        ]),
      );
      await tester.pumpAndSettle();
      expect(find.byType(LoadingIndicator), findsNothing);
      expect(find.text('Tiket Dibuat'), findsOneWidget);
    });

    testWidgets('a successful zero-result shows the localized empty state', (
      tester,
    ) async {
      final repository = _ScriptedSupportRepository(
        onGetEvents: (_) =>
            Future.value(Result.success(const <SupportEvent>[])),
      );
      await _pumpTimeline(tester, repository, const Locale('id'));
      expect(find.text('Belum ada aktivitas tercatat'), findsOneWidget);
    });

    testWidgets('a failed request shows safe copy plus a working retry', (
      tester,
    ) async {
      final repository = _ScriptedSupportRepository(
        onGetEvents: (call) => call == 1
            ? Future.value(Result.error('INTERNAL_SERVER_ERROR: sql: no rows'))
            : Future.value(
                Result.success([
                  _event(
                    SupportEventType.ticketCreated,
                    at: DateTime.utc(2026, 1, 1),
                  ),
                ]),
              ),
      );
      await _pumpTimeline(tester, repository, const Locale('id'));

      expect(find.textContaining('INTERNAL_SERVER_ERROR'), findsNothing);
      expect(find.textContaining('sql'), findsNothing);
      expect(find.text('Coba Lagi'), findsOneWidget);

      await tester.tap(find.widgetWithText(TextButton, 'Coba Lagi'));
      await tester.pumpAndSettle();

      expect(repository.eventCalls, 2);
      expect(find.text('Tiket Dibuat'), findsOneWidget);
      expect(find.text('Coba Lagi'), findsNothing);
    });

    testWidgets('entries render in canonical chronology (oldest first)', (
      tester,
    ) async {
      final repository = _ScriptedSupportRepository(
        onGetEvents: (_) => Future.value(
          // Deliberately newest-first, as the backend returns them.
          Result.success([
            _event(
              SupportEventType.ticketClosed,
              at: DateTime.utc(2026, 1, 3),
              newStatus: 'closed',
            ),
            _event(
              SupportEventType.ticketCreated,
              at: DateTime.utc(2026, 1, 1),
            ),
            _event(
              SupportEventType.ticketClaimed,
              at: DateTime.utc(2026, 1, 2),
              newStatus: 'in_progress',
            ),
          ]),
        ),
      );
      await _pumpTimeline(tester, repository, const Locale('en'));

      final texts = _renderedTexts(tester);
      expect(
        texts.indexOf('TICKET CREATED'),
        lessThan(texts.indexOf('TICKET CLAIMED')),
      );
      expect(
        texts.indexOf('TICKET CLAIMED'),
        lessThan(texts.indexOf('TICKET CLOSED')),
      );
    });
  });

  group('H. wiring proof — the canonical read chain is reachable', () {
    test('the ticket list navigates into the canonical thread route', () {
      final src = _read(_listScreen);
      expect(src.contains('_navigateToTicket'), isTrue);
      expect(
        src.contains('RoutePaths.supportTicketThreadPath(ticketId)'),
        isTrue,
      );
    });

    test('the router registers the ticket thread route', () {
      final src = _read(_module);
      expect(src.contains('RoutePaths.supportTicketThread,'), isTrue);
      expect(
        src.contains('SupportTicketThreadScreen(ticketId: ticketId)'),
        isTrue,
      );
    });

    test('the thread screen mounts the timeline above the conversation', () {
      final src = _read(_threadScreen);
      final timeline = src.indexOf(
        'SupportActivityTimeline(ticketId: widget.ticketId)',
      );
      final messages = src.indexOf('_buildMessagesList(ticketAsync)');
      expect(timeline, greaterThanOrEqualTo(0));
      expect(messages, greaterThanOrEqualTo(0));
      expect(timeline, lessThan(messages));
    });

    test('the timeline reads through the ONE canonical provider', () {
      final src = _read(_timelineWidget);
      expect(
        src.contains('ref.watch(supportTicketEventsProvider(ticketId))'),
        isTrue,
      );
      expect(src.contains('supportRepositoryProvider'), isFalse);
    });

    test('the provider delegates to the canonical repository read', () {
      final src = _read(_notifier);
      expect(src.contains('supportTicketEventsProvider'), isTrue);
      expect(src.contains('repository.getEvents(ticketId)'), isTrue);
    });

    test('the repository read reaches the owner-only events contract', () {
      final repo = _read(_repositoryApi);
      expect(repo.contains('_datasource.getEvents(ticketId'), isTrue);

      final ds = _read(_datasource);
      expect(ds.contains(r"'$_basePath/tickets/$ticketId/events'"), isTrue);
    });
  });

  group('I. entry-point proof — list → tap → thread → localized timeline', () {
    testWidgets(
      'tapping a ticket opens the canonical thread and loads its timeline',
      (tester) async {
        final repository = _FlowRepository();
        await tester.pumpWidget(
          ProviderScope(
            retry: (retryCount, error) => null,
            overrides: [
              authenticatedUserProvider.overrideWith((ref) => _user()),
              supportRepositoryProvider.overrideWithValue(repository),
            ],
            child: MaterialApp.router(
              theme: AppTheme.lightTheme,
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              locale: const Locale('id'),
              routerConfig: _supportRouter(),
            ),
          ),
        );
        await tester.pumpAndSettle();

        // Stage 1 — the canonical ticket list renders the owner's ticket.
        expect(find.text('Payment issue'), findsOneWidget);

        // Stage 2 — the row action navigates into the canonical thread route.
        await tester.tap(find.text('View Ticket'));
        await tester.pumpAndSettle();
        expect(find.byType(SupportTicketThreadScreen), findsOneWidget);

        // Stage 3 — the read reached the canonical events contract carrying
        // the ticket id from the route.
        expect(repository.eventTicketIds, ['t42']);

        // Stage 4 — the localized activity timeline is visible on the thread.
        expect(find.text('Riwayat Aktivitas'), findsOneWidget);
        expect(find.text('Tiket Dibuat'), findsOneWidget);
        expect(find.text('Tiket Diselesaikan'), findsOneWidget);

        // Stage 5 — no raw wire identity reached the screen.
        for (final text in _renderedTexts(tester)) {
          expect(text.contains('TICKET_CREATED'), isFalse);
          expect(text.contains('TICKET_RESOLVED'), isFalse);
          expect(text.contains('UNKNOWN'), isFalse);
        }
      },
    );
  });
}
