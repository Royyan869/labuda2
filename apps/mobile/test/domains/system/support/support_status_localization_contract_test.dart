// I18N-15 — SUPPORT STATUS DISPLAY LOCALIZATION CONVERGENCE CONTRACT.
//
// CANONICAL TRUTH:
//   * Status IDENTITY is `SupportStatus.wireValue` (the lifecycle / API
//     contract) — it is NOT localized and never changes with the locale.
//   * The status DISPLAY LABEL is user-facing content owned by the ONE
//     canonical `AppLocalizations` authority (lib/l10n/app_en.arb +
//     lib/l10n/app_id.arb) and follows the active app locale.
//
// The negative half is the point: the old competing authority
// (`StatusConfig.labelId` / `labelEn` and the hardcoded English
// `_getSupportStatusLabel`) must not quietly return.
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/support/domain/domain.dart';
import 'package:hishumi/domains/system/support/presentation/presentation.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/generated/app_localizations_en.dart';
import 'package:hishumi/generated/app_localizations_id.dart';

const _enFile = 'lib/l10n/app_en.arb';
const _idFile = 'lib/l10n/app_id.arb';
const _config =
    'lib/domains/system/support/domain/entities/support_config.dart';
const _chatDetail =
    'lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart';
const _resolver =
    'lib/domains/system/support/presentation/utils/support_status_label.dart';

/// status -> [Indonesian label, English label]
const Map<SupportStatus, List<String>> _labels = <SupportStatus, List<String>>{
  SupportStatus.open: ['Baru', 'Open'],
  SupportStatus.inProgress: ['Diproses', 'In Progress'],
  SupportStatus.waitingUser: ['Menunggu User', 'Waiting User'],
  SupportStatus.resolved: ['Selesai', 'Resolved'],
  SupportStatus.closed: ['Ditutup', 'Closed'],
};

/// status -> immutable backend identity (must never be localized).
const Map<SupportStatus, String> _wire = <SupportStatus, String>{
  SupportStatus.open: 'open',
  SupportStatus.inProgress: 'in_progress',
  SupportStatus.waitingUser: 'waiting_user',
  SupportStatus.resolved: 'resolved',
  SupportStatus.closed: 'closed',
};

String _read(String path) => File(path).readAsStringSync();

Map<String, dynamic> _arb(String path) =>
    jsonDecode(_read(path)) as Map<String, dynamic>;

SupportTicket _ticket(SupportStatus status) => SupportTicket(
  id: 't1',
  userId: 'u1',
  userName: 'Buyer',
  category: SupportCategory.orderIssue,
  priority: SupportPriority.medium,
  status: status,
  createdAt: DateTime(2026, 1, 1),
);

Future<void> _pumpCard(
  WidgetTester tester,
  SupportStatus status,
  Locale locale,
) async {
  await tester.pumpWidget(
    ProviderScope(
      child: MaterialApp(
        theme: AppTheme.lightTheme,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: locale,
        home: Scaffold(
          body: SupportTicketCardRefactored(ticket: _ticket(status)),
        ),
      ),
    ),
  );
  await tester.pump();
}

void main() {
  final en = AppLocalizationsEn();
  final id = AppLocalizationsId();

  group('A. localization proof — every status label follows the locale', () {
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

    test('the mapping covers the full canonical lifecycle', () {
      expect(_labels.keys.toSet(), SupportStatus.values.toSet());
    });
  });

  group('B. runtime locale proof — a real consumer follows active locale', () {
    testWidgets('support ticket card renders the active-locale status label', (
      tester,
    ) async {
      for (final locale in const [Locale('id'), Locale('en')]) {
        for (final entry in _labels.entries) {
          await _pumpCard(tester, entry.key, locale);
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

    testWidgets('locale switching changes the rendered status label', (
      tester,
    ) async {
      await _pumpCard(tester, SupportStatus.inProgress, const Locale('id'));
      expect(find.text('Diproses'), findsOneWidget);
      expect(find.text('In Progress'), findsNothing);

      await _pumpCard(tester, SupportStatus.inProgress, const Locale('en'));
      expect(find.text('In Progress'), findsOneWidget);
      expect(find.text('Diproses'), findsNothing);
    });

    test('the chat status sheet consumes the same localized resolver', () {
      final source = _read(_chatDetail);
      expect(source, contains('supportStatus?.label(context.l10n)'));
      expect(source, isNot(contains('StatusConfig')));
    });
  });

  group(
    'C. identity preservation — wireValue is the sole, locale-free identity',
    () {
      test('every status maps to its canonical backend wire value', () {
        for (final entry in _wire.entries) {
          expect(entry.key.wireValue, entry.value);
          expect(SupportStatus.fromWire(entry.value), entry.key);
        }
      });

      test('identity never equals a localized display label', () {
        final labels = _labels.values.expand((v) => v).toSet();
        for (final entry in _wire.entries) {
          expect(entry.key.wireValue, isNot(entry.key.label(id)));
          expect(entry.key.wireValue, isNot(entry.key.label(en)));
          expect(labels, isNot(contains(entry.key.wireValue)));
        }
      });

      test('the wire taxonomy is a closed, locale-independent set', () {
        expect(
          SupportStatus.values.map((s) => s.wireValue).toList(),
          _wire.values.toList(),
        );
      });

      test('the resolver never invents a sixth status', () {
        final source = _read(_resolver);
        expect(source, isNot(contains('UNKNOWN')));
        expect(source, isNot(contains('unknown')));
        expect(source, isNot(contains('N/A')));
      });
    },
  );

  group('D. raw technical identity never leaks to display', () {
    testWidgets('no rendered text is a raw wire value or enum name', (
      tester,
    ) async {
      for (final locale in const [Locale('id'), Locale('en')]) {
        for (final status in SupportStatus.values) {
          await _pumpCard(tester, status, locale);
          final expected =
              (locale.languageCode == 'id'
                      ? _labels[status]![0]
                      : _labels[status]![1])
                  .toUpperCase();
          final rendered = tester
              .widgetList<Text>(find.byType(Text))
              .map((t) => (t.data ?? '').toUpperCase())
              .toList();
          // A single-word English label may legitimately coincide with the raw
          // identity ("Open" / "Resolved" / "Closed"); only non-coinciding raw
          // forms are meaningful leak signatures.
          final rawForms = <String>{
            status.wireValue.toUpperCase(),
            status.wireValue.replaceAll('_', '').toUpperCase(),
            status.name.toUpperCase(),
          }..remove(expected);
          for (final raw in rawForms) {
            expect(
              rendered.contains(raw),
              isFalse,
              reason: 'raw identity "$raw" leaked to the UI ($locale)',
            );
          }
        }
      }
    });

    test('the old competing chat authority is gone', () {
      final source = _read(_chatDetail);
      expect(source, isNot(contains('_getSupportStatusLabel')));
      expect(source, isNot(contains('IN PROGRESS')));
      expect(source, isNot(contains('WAITING FOR YOU')));
      expect(source, isNot(contains("'OPEN'")));
    });
  });

  group('E. obsolete residue proof — the purged label fields are gone', () {
    test('StatusConfig no longer declares labelEn/labelId', () {
      final source = _read(_config).replaceAll('\r\n', '\n');
      final start = source.indexOf('class StatusConfig');
      final end = source.indexOf('class GreetingTemplates');
      expect(start, greaterThanOrEqualTo(0));
      final block = source.substring(start, end);
      expect(RegExp(r'\blabelEn\b').hasMatch(block), isFalse);
      expect(RegExp(r'\blabelId\b').hasMatch(block), isFalse);
      for (final value in _labels.values.expand((v) => v)) {
        expect(
          block,
          isNot(contains(value)),
          reason: 'StatusConfig re-hardcodes "$value"',
        );
      }
    });

    test('StatusConfig keeps its canonical visual metadata', () {
      final source = _read(_config).replaceAll('\r\n', '\n');
      final start = source.indexOf('class StatusConfig');
      final end = source.indexOf('class GreetingTemplates');
      final block = source.substring(start, end);
      expect(block, contains('colorValue'));
      expect(block, contains('icon'));
      for (final status in SupportStatus.values) {
        expect(
          block,
          contains('SupportStatus.${status.name}: StatusConfig('),
          reason: 'StatusConfig must still map ${status.name}',
        );
      }
    });

    test('the whole mobile lib is free of the old chat status helper', () {
      final offenders = <String>[];
      for (final file in Directory(
        'lib',
      ).listSync(recursive: true).whereType<File>()) {
        if (!file.path.endsWith('.dart')) continue;
        final source = _read(file.path);
        if (source.contains('_getSupportStatusLabel')) offenders.add(file.path);
      }
      expect(
        offenders,
        isEmpty,
        reason: 'obsolete helper resurfaced: $offenders',
      );
    });
  });

  group('F. single localization authority', () {
    test('every supportStatus key exists in both ARBs at parity', () {
      final enKeys = _arb(_enFile);
      final idKeys = _arb(_idFile);
      for (final key in const [
        'supportStatusOpen',
        'supportStatusInProgress',
        'supportStatusWaitingUser',
        'supportStatusResolved',
        'supportStatusClosed',
      ]) {
        expect(enKeys.containsKey(key), isTrue, reason: 'en missing $key');
        expect(idKeys.containsKey(key), isTrue, reason: 'id missing $key');
      }
    });

    test('each Indonesian status label is authored exactly once', () {
      // Scoped to the supportStatus* namespace: unrelated features may reuse a
      // common word (e.g. the order lifecycle already owns "Selesai").
      final values = _arb(_idFile).entries
          .where((e) => e.key.startsWith('supportStatus'))
          .map((e) => e.value)
          .whereType<String>()
          .toList();
      expect(values.length, _labels.length);
      for (final label in _labels.values.map((v) => v[0])) {
        expect(
          values.where((v) => v == label).length,
          1,
          reason: 'duplicate authority for "$label" in $_idFile',
        );
      }
    });

    test('no ARB value carries a raw wire identity as a label', () {
      for (final path in const [_enFile, _idFile]) {
        final values = _arb(path).values.whereType<String>().toList();
        for (final wire in _wire.values) {
          // Only the literal snake_case contract value (or its ALL-CAPS form)
          // proves a leak; a natural-language word that coincides with a
          // single-word wire value ("Open") is legitimate.
          expect(
            values,
            isNot(contains(wire)),
            reason: '$path exposes raw identity "$wire" as user content',
          );
          expect(
            values,
            isNot(contains(wire.toUpperCase())),
            reason: '$path exposes raw identity "${wire.toUpperCase()}"',
          );
        }
      }
    });

    test('the resolver declares no label copy of its own', () {
      final source = _read(_resolver);
      for (final value in _labels.values.expand((v) => v)) {
        expect(
          source,
          isNot(contains("'$value'")),
          reason: 'resolver re-hardcodes "$value"',
        );
        expect(
          source,
          isNot(contains('"$value"')),
          reason: 'resolver re-hardcodes "$value"',
        );
      }
    });
  });
}
