// I18N-13 — SUPPORT CATEGORY DISPLAY LOCALIZATION CONVERGENCE CONTRACT.
//
// CANONICAL TRUTH:
//   * Category IDENTITY is `SupportCategory.wireValue` (the backend / API
//     contract) — it is NOT localized and never changes with the locale.
//   * The category DISPLAY LABEL is user-facing content owned by the ONE
//     canonical `AppLocalizations` authority (lib/l10n/app_en.arb +
//     lib/l10n/app_id.arb) and follows the active app locale.
//
// The negative half is the point: the purged `CategoryConfig.nameId` /
// `nameEn` / `descriptionId` fields, the raw wire-value / enum-name display
// paths, and any duplicate localization authority must not quietly return.
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
const _chatCard = 'lib/domains/chat/chat/presentation/widgets/chat_card.dart';
const _chatDetail =
    'lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart';

/// category -> [Indonesian label, English label]
const Map<SupportCategory, List<String>> _labels =
    <SupportCategory, List<String>>{
      SupportCategory.orderIssue: ['Masalah Pesanan', 'Order Problems'],
      SupportCategory.paymentIssue: ['Masalah Pembayaran', 'Payment Issues'],
      SupportCategory.accountIssue: ['Bantuan Akun', 'Account Help'],
      SupportCategory.listingIssue: ['Masalah Produk', 'Listing Problems'],
      SupportCategory.shippingIssue: ['Masalah Pengiriman', 'Shipping Issues'],
      SupportCategory.refundRequest: ['Permintaan Refund', 'Refund Request'],
      SupportCategory.dispute: ['Sengketa', 'Dispute'],
      SupportCategory.technicalIssue: ['Bantuan Teknis', 'Technical Help'],
      SupportCategory.other: ['Lainnya', 'Other'],
    };

/// category -> immutable backend identity (must never be localized).
const Map<SupportCategory, String> _wire = <SupportCategory, String>{
  SupportCategory.orderIssue: 'order_issue',
  SupportCategory.paymentIssue: 'payment_issue',
  SupportCategory.accountIssue: 'account_issue',
  SupportCategory.listingIssue: 'listing_issue',
  SupportCategory.shippingIssue: 'shipping_issue',
  SupportCategory.refundRequest: 'refund_request',
  SupportCategory.dispute: 'dispute',
  SupportCategory.technicalIssue: 'technical_issue',
  SupportCategory.other: 'other',
};

String _read(String path) => File(path).readAsStringSync();

Map<String, dynamic> _arb(String path) =>
    jsonDecode(_read(path)) as Map<String, dynamic>;

SupportTicket _ticket(SupportCategory category) => SupportTicket(
  id: 't1',
  userId: 'u1',
  userName: 'Buyer',
  category: category,
  priority: SupportPriority.medium,
  status: SupportStatus.open,
  createdAt: DateTime(2026, 1, 1),
);

Future<void> _pumpCard(
  WidgetTester tester,
  SupportCategory category,
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
          body: SupportTicketCardRefactored(ticket: _ticket(category)),
        ),
      ),
    ),
  );
  await tester.pump();
}

void main() {
  final en = AppLocalizationsEn();
  final id = AppLocalizationsId();

  group('A. localization proof — every category label follows the locale', () {
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
      expect(_labels.keys.toSet(), SupportCategory.values.toSet());
    });
  });

  group('B. runtime locale proof — a real consumer follows active locale', () {
    testWidgets(
      'support ticket card renders the active-locale category label',
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

    testWidgets('locale switching changes the rendered label', (tester) async {
      await _pumpCard(tester, SupportCategory.orderIssue, const Locale('id'));
      expect(find.text('Masalah Pesanan'), findsOneWidget);
      expect(find.text('Order Problems'), findsNothing);

      await _pumpCard(tester, SupportCategory.orderIssue, const Locale('en'));
      expect(find.text('Order Problems'), findsOneWidget);
      expect(find.text('Masalah Pesanan'), findsNothing);
    });
  });

  group(
    'C. identity preservation — wireValue is the sole, locale-free identity',
    () {
      test('every category maps to its canonical backend wire value', () {
        for (final entry in _wire.entries) {
          expect(entry.key.wireValue, entry.value);
          expect(SupportCategory.fromWire(entry.value), entry.key);
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
          SupportCategory.values.map((c) => c.wireValue).toList(),
          _wire.values.toList(),
        );
      });
    },
  );

  group('D. raw technical identity never leaks to display', () {
    testWidgets('no rendered text is a raw wire value or enum name', (
      tester,
    ) async {
      for (final locale in const [Locale('id'), Locale('en')]) {
        for (final category in SupportCategory.values) {
          await _pumpCard(tester, category, locale);
          final expected =
              (locale.languageCode == 'id'
                      ? _labels[category]![0]
                      : _labels[category]![1])
                  .toUpperCase();
          final rendered = tester
              .widgetList<Text>(find.byType(Text))
              .map((t) => (t.data ?? '').toUpperCase())
              .toList();
          // A single-word English label may legitimately coincide with the raw
          // identity ("Dispute" / "Other"); only non-coinciding raw forms are
          // meaningful leak signatures.
          final rawForms = <String>{
            category.wireValue.toUpperCase(),
            category.wireValue.replaceAll('_', '').toUpperCase(),
            category.name.toUpperCase(),
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

    test('chat surfaces no longer render raw category identity', () {
      expect(_read(_chatCard), isNot(contains('wireValue.toUpperCase()')));
      expect(_read(_chatDetail), isNot(contains('supportCategory?.name')));
      expect(
        _read(_chatDetail),
        isNot(contains('supportCategory?.name.toUpperCase()')),
      );
    });
  });

  group(
    'E. obsolete residue proof — the hardcoded label fields are purged',
    () {
      test('CategoryConfig no longer declares nameId/nameEn/descriptionId', () {
        final source = _read(_config);
        for (final purged in const ['nameId', 'nameEn', 'descriptionId']) {
          expect(
            RegExp('\\b$purged\\b').hasMatch(source),
            isFalse,
            reason: '$purged must no longer live in $_config',
          );
        }
      });

      test('the whole mobile lib is free of the purged field names', () {
        final offenders = <String>[];
        for (final file in Directory(
          'lib',
        ).listSync(recursive: true).whereType<File>()) {
          if (!file.path.endsWith('.dart')) continue;
          final source = _read(file.path);
          for (final purged in const ['nameId', 'nameEn', 'descriptionId']) {
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

      test('the category config carries no label string authority', () {
        final source = _read(_config).replaceAll('\r\n', '\n');
        // The four config classes may legitimately carry labels elsewhere
        // (Priority/Status), so this assertion is scoped to the CategoryConfig
        // block only.
        final start = source.indexOf('class CategoryConfig');
        final end = source.indexOf('class PriorityConfig');
        expect(start, greaterThanOrEqualTo(0));
        final block = source.substring(start, end);
        for (final value in _labels.values.expand((v) => v)) {
          expect(
            block,
            isNot(contains(value)),
            reason: 'CategoryConfig re-hardcodes "$value"',
          );
        }
      });
    },
  );

  group('F. single localization authority', () {
    test('every supportCategory key exists in both ARBs at parity', () {
      final enKeys = _arb(_enFile);
      final idKeys = _arb(_idFile);
      for (final key in const [
        'supportCategoryOrderIssue',
        'supportCategoryPaymentIssue',
        'supportCategoryAccountIssue',
        'supportCategoryListingIssue',
        'supportCategoryShippingIssue',
        'supportCategoryRefundRequest',
        'supportCategoryDispute',
        'supportCategoryTechnicalIssue',
        'supportCategoryOther',
      ]) {
        expect(enKeys.containsKey(key), isTrue, reason: 'en missing $key');
        expect(idKeys.containsKey(key), isTrue, reason: 'id missing $key');
      }
    });

    test('each Indonesian category label is authored exactly once', () {
      // Scoped to the supportCategory* namespace: unrelated features may
      // legitimately reuse a common word (the account-deactivation reasons
      // already own "Lainnya").
      final values = _arb(_idFile).entries
          .where((e) => e.key.startsWith('supportCategory'))
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
          // proves a leak; a natural-language word that happens to coincide
          // with a single-word wire value ("Other", "Dispute") is legitimate.
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
      final source = _read(
        'lib/domains/system/support/presentation/utils/'
        'support_category_label.dart',
      );
      // Quoted forms only: the generated getter names legitimately contain the
      // English category words as substrings (e.g. `supportCategoryDispute`).
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
