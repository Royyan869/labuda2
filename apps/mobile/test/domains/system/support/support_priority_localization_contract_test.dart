// I18N-16 — SUPPORT PRIORITY DISPLAY LOCALIZATION CONVERGENCE CONTRACT.
//
// CANONICAL TRUTH:
//   * Priority IDENTITY is the enum itself (`SupportPriority.name`, the
//     backend / API contract validated by `oneof=low medium high urgent`) — it
//     is NOT localized and never changes with the locale. There is deliberately
//     NO `wireValue` field on this enum.
//   * The priority DISPLAY LABEL is user-facing content owned by the ONE
//     canonical `AppLocalizations` authority (lib/l10n/app_en.arb +
//     lib/l10n/app_id.arb) and follows the active app locale.
//
// The negative half is the point: the purged `PriorityConfig.labelEn` /
// `labelId` fields and the hardcoded ALL-CAPS display strings must not
// quietly return.
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';
import 'package:labuda/domains/system/support/presentation/presentation.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/generated/app_localizations_en.dart';
import 'package:labuda/generated/app_localizations_id.dart';

const _enFile = 'lib/l10n/app_en.arb';
const _idFile = 'lib/l10n/app_id.arb';
const _config =
    'lib/domains/system/support/domain/entities/support_config.dart';
const _card =
    'lib/domains/system/support/presentation/widgets/'
    'support_ticket_card.dart';
const _resolver =
    'lib/domains/system/support/presentation/utils/support_priority_label.dart';

/// The ALL-CAPS labels the old hardcoded config carried. They must be gone.
const List<String> _obsoleteLabels = [
  'MENDESAK',
  'TINGGI',
  'SEDANG',
  'RENDAH',
  'URGENT',
  'HIGH',
  'MEDIUM',
  'LOW',
];

/// priority -> [Indonesian label, English label]
const Map<SupportPriority, List<String>> _labels =
    <SupportPriority, List<String>>{
      SupportPriority.low: ['Rendah', 'Low'],
      SupportPriority.medium: ['Sedang', 'Medium'],
      SupportPriority.high: ['Tinggi', 'High'],
      SupportPriority.urgent: ['Mendesak', 'Urgent'],
    };

/// priority -> immutable enum identity (must never be localized).
const Map<SupportPriority, String> _identity = <SupportPriority, String>{
  SupportPriority.low: 'low',
  SupportPriority.medium: 'medium',
  SupportPriority.high: 'high',
  SupportPriority.urgent: 'urgent',
};

String _read(String path) => File(path).readAsStringSync();

Map<String, dynamic> _arb(String path) =>
    jsonDecode(_read(path)) as Map<String, dynamic>;

SupportTicket _ticket(SupportPriority priority) => SupportTicket(
  id: 't1',
  userId: 'u1',
  userName: 'Buyer',
  category: SupportCategory.orderIssue,
  priority: priority,
  status: SupportStatus.open,
  createdAt: DateTime(2026, 1, 1),
);

Future<void> _pumpCard(
  WidgetTester tester,
  SupportPriority priority,
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
          body: SupportTicketCardRefactored(ticket: _ticket(priority)),
        ),
      ),
    ),
  );
  await tester.pump();
}

void main() {
  final en = AppLocalizationsEn();
  final id = AppLocalizationsId();

  group('A. localization proof — every priority label follows the locale', () {
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

    test('the mapping covers the full canonical taxonomy', () {
      expect(_labels.keys.toSet(), SupportPriority.values.toSet());
    });
  });

  group('B. runtime locale proof — a real consumer follows active locale', () {
    testWidgets(
      'support ticket card renders the active-locale priority label',
      (tester) async {
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
      },
    );

    testWidgets('locale switching changes the rendered priority label', (
      tester,
    ) async {
      await _pumpCard(tester, SupportPriority.urgent, const Locale('id'));
      expect(find.text('Mendesak'), findsOneWidget);
      expect(find.text('Urgent'), findsNothing);

      await _pumpCard(tester, SupportPriority.urgent, const Locale('en'));
      expect(find.text('Urgent'), findsOneWidget);
      expect(find.text('Mendesak'), findsNothing);
    });

    test('the card badge consumes the canonical resolver', () {
      final source = _read(_card);
      expect(source, contains('ticket.priority.label(context.l10n)'));
      expect(source, isNot(contains('priorityConfig.labelId')));
      expect(source, isNot(contains('priorityConfig.labelEn')));
    });
  });

  group('C. identity preservation — enum name is the sole identity', () {
    test('every priority maps to its canonical backend identity', () {
      for (final entry in _identity.entries) {
        expect(entry.key.name, entry.value);
        expect(
          SupportPriority.values.firstWhere((e) => e.name == entry.value),
          entry.key,
        );
      }
    });

    test('the identity taxonomy is a closed, locale-independent set', () {
      expect(
        SupportPriority.values.map((p) => p.name).toList(),
        _identity.values.toList(),
      );
    });

    test('identity never equals a localized display label', () {
      final labels = _labels.values.expand((v) => v).toSet();
      for (final entry in _identity.entries) {
        expect(entry.key.name, isNot(entry.key.label(id)));
        expect(entry.key.name, isNot(entry.key.label(en)));
        expect(labels, isNot(contains(entry.key.name)));
      }
    });

    test('the priority enum gained no wireValue field', () {
      final source = _read(
        'lib/domains/system/support/domain/entities/support_ticket.dart',
      );
      final start = source.indexOf('enum SupportPriority');
      expect(start, greaterThanOrEqualTo(0));
      // Stop at the enum's own closing brace: the *next* declaration's doc
      // comment legitimately mentions `wireValue` for SupportStatus.
      final end = source.indexOf('}', start);
      final block = source.substring(start, end);
      expect(block, isNot(contains('wireValue')));
      for (final identity in _identity.values) {
        expect(block, contains('  $identity,'));
      }
    });

    test('the resolver never invents a fifth priority', () {
      final source = _read(_resolver);
      expect(source, isNot(contains('UNKNOWN')));
      expect(source, isNot(contains('unknown')));
      expect(source, isNot(contains('N/A')));
    });
  });

  group('D. raw technical identity never leaks to display', () {
    testWidgets('no rendered text is a raw enum name', (tester) async {
      for (final locale in const [Locale('id'), Locale('en')]) {
        for (final priority in SupportPriority.values) {
          await _pumpCard(tester, priority, locale);
          final expected =
              (locale.languageCode == 'id'
                      ? _labels[priority]![0]
                      : _labels[priority]![1])
                  .toUpperCase();
          final rendered = tester
              .widgetList<Text>(find.byType(Text))
              .map((t) => (t.data ?? '').toUpperCase())
              .toList();
          // The English single-word labels legitimately coincide with the raw
          // identity ("Low" / "High"); only non-coinciding raw forms are
          // meaningful leak signatures (this keeps the check real for `id`).
          final rawForms = <String>{priority.name.toUpperCase()}
            ..remove(expected);
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

    test('the obsolete ALL-CAPS labels are gone from the whole app', () {
      final offenders = <String>[];
      for (final file in Directory(
        'lib',
      ).listSync(recursive: true).whereType<File>()) {
        if (!file.path.endsWith('.dart')) continue;
        final source = _read(file.path);
        for (final obsolete in _obsoleteLabels) {
          if (source.contains("'$obsolete'")) {
            offenders.add('${file.path}: $obsolete');
          }
        }
      }
      expect(
        offenders,
        isEmpty,
        reason: 'obsolete labels resurfaced: $offenders',
      );
    });
  });

  group('E. obsolete residue proof — the purged label fields are gone', () {
    test('PriorityConfig no longer declares labelEn/labelId', () {
      final source = _read(_config).replaceAll('\r\n', '\n');
      final start = source.indexOf('class PriorityConfig');
      final end = source.indexOf('class StatusConfig');
      expect(start, greaterThanOrEqualTo(0));
      final block = source.substring(start, end);
      expect(RegExp(r'\blabelEn\b').hasMatch(block), isFalse);
      expect(RegExp(r'\blabelId\b').hasMatch(block), isFalse);
      for (final value in _labels.values.expand((v) => v)) {
        expect(
          block,
          isNot(contains(value)),
          reason: 'PriorityConfig re-hardcodes "$value"',
        );
      }
      for (final obsolete in _obsoleteLabels) {
        expect(block, isNot(contains(obsolete)));
      }
    });

    test('PriorityConfig keeps its canonical metadata and sorting logic', () {
      final source = _read(_config).replaceAll('\r\n', '\n');
      final start = source.indexOf('class PriorityConfig');
      final end = source.indexOf('class StatusConfig');
      final block = source.substring(start, end);
      expect(block, contains('colorValue'));
      expect(block, contains('icon'));
      expect(block, contains('static int getOrder(SupportPriority priority)'));
      for (final priority in SupportPriority.values) {
        expect(
          block,
          contains('SupportPriority.${priority.name}: PriorityConfig('),
          reason: 'PriorityConfig must still map ${priority.name}',
        );
      }
    });

    test('the whole mobile lib is free of the purged label fields', () {
      final offenders = <String>[];
      for (final file in Directory(
        'lib',
      ).listSync(recursive: true).whereType<File>()) {
        if (!file.path.endsWith('.dart')) continue;
        final source = _read(file.path);
        for (final purged in const ['labelEn', 'labelId']) {
          if (RegExp('\\b$purged\\b').hasMatch(source)) {
            offenders.add('${file.path}: $purged');
          }
        }
      }
      expect(
        offenders,
        isEmpty,
        reason: 'purged fields resurfaced: $offenders',
      );
    });
  });

  group('F. single localization authority', () {
    test('every supportPriority key exists in both ARBs at parity', () {
      final enKeys = _arb(_enFile);
      final idKeys = _arb(_idFile);
      for (final key in const [
        'supportPriorityLow',
        'supportPriorityMedium',
        'supportPriorityHigh',
        'supportPriorityUrgent',
      ]) {
        expect(enKeys.containsKey(key), isTrue, reason: 'en missing $key');
        expect(idKeys.containsKey(key), isTrue, reason: 'id missing $key');
      }
    });

    test('each Indonesian priority label is authored exactly once', () {
      // Scoped to the supportPriority* namespace: unrelated features may reuse
      // a common word.
      final values = _arb(_idFile).entries
          .where((e) => e.key.startsWith('supportPriority'))
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

    test('no ARB value carries a raw enum identity as a label', () {
      for (final path in const [_enFile, _idFile]) {
        final values = _arb(path).values.whereType<String>().toList();
        for (final identity in _identity.values) {
          expect(
            values,
            isNot(contains(identity)),
            reason: '$path exposes raw identity "$identity" as user content',
          );
          expect(
            values,
            isNot(contains(identity.toUpperCase())),
            reason: '$path exposes raw identity "${identity.toUpperCase()}"',
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
