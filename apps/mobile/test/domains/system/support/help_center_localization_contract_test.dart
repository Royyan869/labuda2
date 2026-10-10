// I18N-11 — HELP CENTER CONTENT & LOCALIZATION CONVERGENCE CONTRACT.
//
// CANONICAL TRUTH: every Help Center string renders from AppLocalizations
// (lib/l10n/app_en.arb + lib/l10n/app_id.arb) and the article copy matches the
// owner-locked business rules:
//   * buyer confirmation window = 5 days from seller mark-ship, +3 days once;
//   * buyer-facing statuses = waiting for payment / being prepared / in
//     delivery / completed (never "Processing" or "Delivered"); there is NO
//     seller-confirmation step for a pending_payment order;
//   * For Sale: create = publish — no draft / review / approval stage;
//   * seller proceeds are withdrawable immediately after completion;
//   * seller verification = 1-2 business days;
//   * withdrawal = 1-3 business days AFTER admin approval;
//   * no invented SLAs (24h payment confirmation, refund 1-3 / 3-7, GoPay).
//
// The negative half is the point: the private `_Strings` bag, the obsolete
// literals, and the driven-in drift must not quietly return.
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/support/presentation/screens/help_center_screen.dart';
import 'package:hishumi/generated/app_localizations.dart';

const _enFile = 'lib/l10n/app_en.arb';
const _idFile = 'lib/l10n/app_id.arb';
const _screen =
    'lib/domains/system/support/presentation/screens/help_center_screen.dart';

/// Help Center UI shell keys; every `article*` value is collected by prefix.
const _shellKeys = <String>[
  'helpSupport',
  'howCanWeHelp',
  'helpCenterDescription',
  'searchHelpArticles',
  'quickHelp',
  'browseByCategory',
  'popularArticles',
  'orders',
  'payments',
  'selling',
  'account',
  'verification',
  'technical',
  'orderHelpSubtitle',
  'paymentHelpSubtitle',
  'sellingHelpSubtitle',
  'accountHelpSubtitle',
  'verificationHelpSubtitle',
  'technicalHelpSubtitle',
  'stillNeedHelp',
  'contactSupportDescription',
  'contactSupport',
  'helpArticle',
  'wasThisHelpful',
  'yes',
  'no',
  'feedbackThanks',
];

String _read(String path) => File(path).readAsStringSync();

/// Source with comments removed — prose may name a purged artifact; compiled
/// code may not. CRLF normalised so the comment regexes are deterministic.
String _code(String path) => _read(path)
    .replaceAll('\r\n', '\n')
    .replaceAll(RegExp(r'/\*[\s\S]*?\*/'), '')
    .replaceAll(RegExp(r'//[^\n]*'), '');

Map<String, dynamic> _arb(String path) =>
    jsonDecode(_read(path)) as Map<String, dynamic>;

/// Every user-facing Help Center value in one ARB (metadata maps excluded).
Map<String, String> _helpValues(String path) {
  final values = <String, String>{};
  _arb(path).forEach((key, value) {
    if (value is! String) return;
    if (key.startsWith('article') || _shellKeys.contains(key)) {
      values[key] = value;
    }
  });
  return values;
}

void main() {
  final en = _helpValues(_enFile);
  final id = _helpValues(_idFile);
  final both = <String, Map<String, String>>{'en': en, 'id': id};

  group('single localization authority', () {
    test('the private _Strings bag is purged lib-wide', () {
      for (final file in Directory(
        'lib',
      ).listSync(recursive: true).whereType<File>()) {
        if (!file.path.endsWith('.dart')) continue;
        expect(
          _code(file.path),
          isNot(contains('_Strings')),
          reason: '${file.path} names the purged private string bag',
        );
      }
    });

    test('the screen renders from AppLocalizations, not literals', () {
      final screen = _code(_screen);
      expect(screen, contains('AppLocalizations.of(context)!'));
      // The canonical search-field factory contract survives the migration.
      expect(screen, contains('searchDecoration('));
      // No Help Center copy is re-hardcoded in the screen source: a Dart
      // literal would carry the value inside quotes.
      final offenders = <String>[];
      for (final values in [en, id]) {
        values.forEach((key, value) {
          if (value.length < 10) return;
          if (screen.contains("'$value'") || screen.contains('"$value"')) {
            offenders.add(key);
          }
        });
      }
      expect(
        offenders,
        isEmpty,
        reason: 'ARB values re-hardcoded in $_screen: $offenders',
      );
    });

    test('every key the screen consumes exists in both ARBs', () {
      final used = RegExp(
        r'l10n\.([a-zA-Z][a-zA-Z0-9]*)',
      ).allMatches(_code(_screen)).map((match) => match.group(1)!).toSet();
      expect(used, isNotEmpty);
      final enKeys = _arb(_enFile);
      final idKeys = _arb(_idFile);
      for (final key in used) {
        expect(enKeys.containsKey(key), isTrue, reason: 'en is missing $key');
        expect(idKeys.containsKey(key), isTrue, reason: 'id is missing $key');
      }
    });

    test('Help Center resources are at en/id parity', () {
      expect(id.keys.toSet(), en.keys.toSet());
    });

    test('obsolete cache articles are purged; Phase-3 articles exist', () {
      for (final path in const [_enFile, _idFile]) {
        final keys = _arb(path);
        expect(keys.containsKey('articleClearCache'), isFalse, reason: path);
        expect(
          keys.containsKey('articleClearCacheContent'),
          isFalse,
          reason: path,
        );
        for (final key in const [
          'articleWithdrawalFailed',
          'articleWithdrawalFailedContent',
          'articleForSaleNotVisible',
          'articleForSaleNotVisibleContent',
          'articleSellerPaymentPending',
          'articleSellerPaymentPendingContent',
          'articleOrderShipmentHelp',
          'articleOrderShipmentHelpContent',
          'articleItemNotReceived',
          'articleItemNotReceivedContent',
          'articleAppSlowOrNotLoading',
          'articleAppSlowOrNotLoadingContent',
        ]) {
          expect(
            keys.containsKey(key),
            isTrue,
            reason: '$path is missing $key',
          );
        }
      }
    });

    test('Help Center search stays promised but wired to no query yet', () {
      final screen = _code(_screen);
      // The promise (hint text) comes from the ARB...
      expect(screen, contains('l10n.searchHelpArticles'));
      // ...and the field still uses the canonical search visual class.
      expect(screen, contains('searchDecoration('));
    });

    test('Contact Support opens the canonical support flow', () {
      final screen = _code(_screen);
      expect(screen, contains('showPreChatFormRefactored('));
      expect(screen, isNot(contains('Navigator.of(context).pop(true)')));
    });
  });

  group('owner-locked business content', () {
    test('confirmation window is 5 days, never 3 days as the window', () {
      expect(en['articleConfirmDeliveryContent'], contains('5 days'));
      expect(
        en['articleConfirmDeliveryContent'],
        contains('Extend Confirmation'),
      );
      expect(en['articleConfirmDeliveryContent'], contains('3 days'));
      expect(id['articleConfirmDeliveryContent'], contains('5 hari'));
      expect(
        id['articleConfirmDeliveryContent'],
        contains('Perpanjang Konfirmasi'),
      );
      expect(id['articleConfirmDeliveryContent'], contains('3 hari'));
      for (final values in both.values) {
        for (final value in values.values) {
          expect(value, isNot(matches(RegExp(r'within 3 days|dalam 3 hari'))));
        }
      }
    });

    test('buyer-facing order statuses only', () {
      expect(en['articleTrackOrderContent'], contains('Waiting for Payment'));
      expect(en['articleTrackOrderContent'], contains('Being Prepared'));
      expect(en['articleTrackOrderContent'], contains('In Delivery'));
      expect(en['articleTrackOrderContent'], contains('Completed'));
      expect(en['articleCancelOrderContent'], contains('Waiting for Payment'));
      expect(id['articleTrackOrderContent'], contains('Menunggu Pembayaran'));
      expect(id['articleTrackOrderContent'], contains('Diproses'));
      expect(id['articleTrackOrderContent'], contains('Dalam Pengiriman'));
      expect(id['articleTrackOrderContent'], contains('Selesai'));
      // The removed internal/legacy statuses never return as user copy.
      for (final values in both.values) {
        for (final value in values.values) {
          expect(value, isNot(contains('Processing')));
          expect(value, isNot(contains('Delivered')));
        }
      }
      // The buyer never sees the obsolete "Contact Seller" label either.
      for (final values in both.values) {
        for (final value in values.values) {
          expect(value, isNot(contains("'Contact Seller'")));
        }
      }
    });

    test(
      'the obsolete seller-confirmation model never returns as help copy',
      () {
        for (final values in both.values) {
          for (final value in values.values) {
            expect(value, isNot(contains('Menunggu Konfirmasi')));
            expect(value, isNot(contains('Waiting for Confirmation')));
            expect(value, isNot(contains('menunggu penjual mengonfirmasi')));
            expect(value, isNot(contains('waiting for the seller to confirm')));
          }
        }
      },
    );

    test('For Sale visibility: create = publish, no review/approval stage', () {
      expect(en['articleForSaleNotVisibleContent'], contains('It is sold'));
      expect(en['articleForSaleNotVisibleContent'], contains('withdrawn'));
      expect(
        en['articleForSaleNotVisibleContent'],
        contains('as soon as you publish it'),
      );
      expect(id['articleForSaleNotVisibleContent'], contains('Sudah terjual'));
      expect(id['articleForSaleNotVisibleContent'], contains('Ditarik'));
      expect(
        id['articleForSaleNotVisibleContent'],
        contains('langsung tayang'),
      );
      for (final values in both.values) {
        for (final value in values.values) {
          expect(value, isNot(contains('pending review')));
          expect(value, isNot(contains('Pending review')));
          expect(value, isNot(contains('menunggu review')));
          expect(value, isNot(contains('menunggu persetujuan')));
        }
      }
    });

    test('seller proceeds are immediately withdrawable after completion', () {
      expect(
        en['articleSellerPaymentPendingContent'],
        contains('withdrawn immediately'),
      );
      expect(
        en['articleSellerPaymentPendingContent'],
        contains('no waiting period'),
      );
      expect(
        id['articleSellerPaymentPendingContent'],
        contains('langsung bisa ditarik'),
      );
      expect(
        id['articleSellerPaymentPendingContent'],
        contains('tidak ada masa tunggu'),
      );
      for (final values in both.values) {
        for (final value in values.values) {
          expect(value, isNot(matches(RegExp(r'3-7'))));
          expect(value, isNot(contains('mature')));
          expect(value, isNot(contains('jatuh tempo')));
        }
      }
    });

    test('seller verification is 1-2 business days', () {
      expect(
        en['articleSellerVerificationContent'],
        contains('1-2 business days'),
      );
      expect(en['articleSellerVerificationContent'], isNot(contains('1-3')));
      expect(
        id['articleSellerVerificationContent'],
        contains('1-2 hari kerja'),
      );
      expect(id['articleSellerVerificationContent'], isNot(contains('1-3')));
    });

    test('payment timing carries no 24-hour confirmation promise', () {
      for (final values in both.values) {
        for (final value in values.values) {
          expect(value, isNot(contains('within 24 hours')));
          expect(value, isNot(contains('dalam 24 jam')));
        }
      }
      expect(en['articleHowToPayContent'], contains('deadline'));
      expect(id['articleHowToPayContent'], contains('batas waktu'));
    });

    test('refund timing invents no SLA and no GoPay speed claim', () {
      expect(en['articleRefundTimeContent'], isNot(contains('GoPay')));
      expect(
        en['articleRefundTimeContent'],
        isNot(matches(RegExp(r'1-3 days|3-7'))),
      );
      expect(id['articleRefundTimeContent'], isNot(contains('GoPay')));
      expect(
        id['articleRefundTimeContent'],
        isNot(matches(RegExp(r'1-3 hari|3-7'))),
      );
    });

    test('withdrawal is 1-3 business days AFTER admin approval', () {
      expect(
        en['articleWithdrawalFailedContent'],
        contains('reviewed by admin first'),
      );
      expect(en['articleWithdrawalFailedContent'], contains('Once approved'));
      expect(
        en['articleWithdrawalFailedContent'],
        contains('1-3 business days'),
      );
      expect(
        id['articleWithdrawalFailedContent'],
        contains('ditinjau admin terlebih dahulu'),
      );
      expect(
        id['articleWithdrawalFailedContent'],
        contains('Setelah disetujui'),
      );
      expect(id['articleWithdrawalFailedContent'], contains('1-3 hari kerja'));
    });

    test('troubleshooting describes only supported app behavior', () {
      for (final values in both.values) {
        for (final value in values.values) {
          expect(value, isNot(contains('Clear Cache')));
          expect(value, isNot(contains('Hapus Cache')));
          expect(value, isNot(contains('cache')));
          expect(value, isNot(contains('Cache')));
        }
      }
      expect(en['articleAppSlowOrNotLoadingContent'], isNotEmpty);
      expect(id['articleAppSlowOrNotLoadingContent'], isNotEmpty);
    });

    test('canonical For Sale terminology in every Help Center value', () {
      for (final values in both.values) {
        for (final value in values.values) {
          expect(
            value,
            isNot(matches(RegExp(r'forsale|listing', caseSensitive: false))),
          );
        }
      }
      expect(en['articleCreateForSaleContent'], contains('For Sale'));
      expect(id['articleCreateForSaleContent'], contains('For Sale'));
    });

    test('helpfulness feedback claims no persistence it does not have', () {
      for (final values in both.values) {
        expect(
          values['feedbackThanks'],
          isNot(
            matches(
              RegExp(
                r'recorded|submitted|stored|saved|tersimpan|disimpan',
                caseSensitive: false,
              ),
            ),
          ),
        );
      }
    });
  });

  group('runtime locale proof', () {
    testWidgets('the active locale controls the Help Center language', (
      tester,
    ) async {
      Future<void> pumpLocale(Locale locale) async {
        await tester.pumpWidget(
          MaterialApp(
            theme: AppTheme.lightTheme,
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: locale,
            home: const HelpCenterScreen(),
          ),
        );
        await tester.pump();
      }

      await pumpLocale(const Locale('id'));
      expect(find.text('Bantuan & Dukungan'), findsOneWidget);
      expect(
        find.text('Bagaimana kami bisa membantu hari ini?'),
        findsOneWidget,
      );
      expect(find.text('How can we help you today?'), findsNothing);

      await pumpLocale(const Locale('en'));
      expect(find.text('Help & Support'), findsOneWidget);
      expect(find.text('How can we help you today?'), findsOneWidget);
      expect(find.text('Bagaimana kami bisa membantu hari ini?'), findsNothing);
    });
  });
}
