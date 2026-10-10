import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/transaction/order/domain/domain.dart'
    as order_domain;
import 'package:hishumi/domains/commerce/transaction/order/presentation/widgets/dynamic_action_buttons.dart';
import 'package:hishumi/domains/commerce/transaction/order/presentation/widgets/order_action_label_resolver.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/generated/app_localizations_en.dart';
import 'package:hishumi/generated/app_localizations_id.dart';

/// I18N-07 — Order Action localization proof.
///
/// The 15 active backend `label_key` values must render the canonical
/// AppLocalizations resource for the active locale:
///   A. localization proof (id + en)
///   C. raw-key leakage proof
///   D. mixed-locale proof
///   E. obsolete residue proof
///
/// `label_key` stays the backend's semantic CTA contract; routing identity is
/// always `action.type` and is proven in the other order action test files.
const Map<String, List<String>> _activeLabels = <String, List<String>>{
  // labelKey: [indonesian resource, english resource]
  'action.mark_shipped': ['Kirim Pesanan', 'Ship Order'],
  'action.confirm_receipt': ['Terima Barang', 'Confirm Receipt'],
  'action.provide_evidence': ['Sediakan Bukti', 'Provide Evidence'],
  'action.cancel_order_overdue': ['Batalkan Pesanan', 'Cancel Order'],
  'action.pay_now': ['Bayar Sekarang', 'Pay Now'],
  'action.payment_continue': ['Lanjutkan Pembayaran', 'Continue Payment'],
  'action.payment_check_status': ['Cek Status Pembayaran', 'Check Payment Status'],
  'action.pay_again': ['Bayar Ulang', 'Retry Payment'],
  'action.cancel_order': ['Batalkan', 'Cancel'],
  'action.extend_confirmation': ['Perpanjang Konfirmasi', 'Extend Confirmation'],
  'action.request_refund': ['Ajukan Pengembalian', 'Request Refund'],
  'action.open_dispute': ['Buka Dispute', 'Open Dispute'],
  'action.update_tracking': ['Update Nomor Resi', 'Update Tracking'],
  'action.chat_seller': ['Chat Penjual', 'Chat Seller'],
  'action.contact_support': ['Hubungi Dukungan', 'Contact Support'],
};

order_domain.Action _action(String labelKey) {
  return order_domain.Action(
    type: 'pay', // identity irrelevant here: label rendering is label-driven
    labelKey: labelKey,
    enabled: true,
    endpoint: '/test',
    method: 'POST',
    requiresIdempotency: false,
    financial: false,
  );
}

Future<void> _pumpAction(
  WidgetTester tester,
  String labelKey,
  Locale locale,
) async {
  final decision = order_domain.DecisionContract(
    state: 'pending',
    primaryAction: _action(labelKey),
  );
  await tester.pumpWidget(
    ProviderScope(
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: locale,
        home: Scaffold(
          body: DynamicActionButtons(
            decision: decision,
            callbacks: ActionCallbacks(onAction: (_) {}),
          ),
        ),
      ),
    ),
  );
}

Set<String> _renderedTexts(WidgetTester tester) {
  return tester
      .widgetList<Text>(find.byType(Text))
      .map((t) => t.data ?? '')
      .toSet();
}

void main() {
  group('A. localization proof — active label_keys follow the locale', () {
    testWidgets('Indonesian locale renders all 15 Indonesian resources', (
      tester,
    ) async {
      for (final entry in _activeLabels.entries) {
        await _pumpAction(tester, entry.key, const Locale('id'));
        expect(
          find.text(entry.value[0]),
          findsOneWidget,
          reason: '${entry.key} must render its Indonesian resource',
        );
      }
    });

    testWidgets('English locale renders all 15 English resources', (
      tester,
    ) async {
      for (final entry in _activeLabels.entries) {
        await _pumpAction(tester, entry.key, const Locale('en'));
        expect(
          find.text(entry.value[1]),
          findsOneWidget,
          reason: '${entry.key} must render its English resource',
        );
      }
    });
  });

  group('C. raw-key leakage proof', () {
    testWidgets('no active action renders an action.* key in any locale', (
      tester,
    ) async {
      for (final key in _activeLabels.keys) {
        for (final locale in const [Locale('id'), Locale('en')]) {
          await _pumpAction(tester, key, locale);
          final leaked = _renderedTexts(
            tester,
          ).where((text) => text.contains(RegExp(r'action\.[a-z_]')));
          expect(
            leaked,
            isEmpty,
            reason: '$key ($locale) leaked raw backend keys: $leaked',
          );
        }
      }
    });

    testWidgets(
      'unknown label_key renders the generic localized CTA, never '
      'a Title-Case fallback nor a raw key',
      (tester) async {
        await _pumpAction(tester, 'action.some_future_key', const Locale('id'));
        expect(find.text('Lanjutkan'), findsOneWidget);
        expect(find.text('Some Future Key'), findsNothing);
        expect(
          _renderedTexts(tester).where((t) => t.contains('action.')),
          isEmpty,
        );
      },
    );
  });

  group('D. mixed-locale proof', () {
    testWidgets('Indonesian locale never shows English action labels', (
      tester,
    ) async {
      final english = _activeLabels.values.map((v) => v[1]).toSet();
      for (final entry in _activeLabels.entries) {
        await _pumpAction(tester, entry.key, const Locale('id'));
        final mixed = _renderedTexts(tester).intersection(english);
        expect(
          mixed,
          isEmpty,
          reason: '${entry.key} under id locale rendered English: $mixed',
        );
      }
    });

    testWidgets('English locale never shows Indonesian action labels', (
      tester,
    ) async {
      final indonesian = _activeLabels.values.map((v) => v[0]).toSet();
      for (final entry in _activeLabels.entries) {
        await _pumpAction(tester, entry.key, const Locale('en'));
        final mixed = _renderedTexts(tester).intersection(indonesian);
        expect(
          mixed,
          isEmpty,
          reason: '${entry.key} under en locale rendered Indonesian: $mixed',
        );
      }
    });
  });

  group('E. obsolete residue proof', () {
    test('manual translation maps and Title-Case fallback are purged', () {
      final widgetSource = File(
        'lib/domains/commerce/transaction/order/presentation/widgets/'
        'dynamic_action_buttons_impl.dart',
      ).readAsStringSync();
      for (final residue in [
        '_primaryLocalize',
        '_secondaryLocalize',
        '_formatLabelKey',
        "'action.complete_order'",
        "'action.accept'",
        "'action.reject'",
        'Kirim Pesanan',
        'Chat Penjual',
      ]) {
        expect(
          widgetSource,
          isNot(contains(residue)),
          reason: '$residue must no longer live in the order action surface',
        );
      }

      final handlerSource = File(
        'lib/domains/commerce/transaction/order/presentation/screens/'
        'order_detail/order_action_handler.dart',
      ).readAsStringSync();
      expect(handlerSource, isNot(contains('Label: \${action.labelKey}')));
      expect(handlerSource, isNot(contains('blocked.messageKey')));
    });

    test('resolver is mechanical: getters only, no obsolete keys', () {
      final resolverSource = File(
        'lib/domains/commerce/transaction/order/presentation/widgets/'
        'order_action_label_resolver.dart',
      ).readAsStringSync();
      for (final residue in [
        "'action.complete_order'",
        "'action.accept'",
        "'action.reject'",
        'Pesanan',
        'Barang',
        'Bayar',
      ]) {
        expect(
          resolverSource,
          isNot(contains(residue)),
          reason: 'resolver must stay mechanical: $residue found',
        );
      }

      // Behavioral proof: known keys resolve to AppLocalizations values in
      // both locales; obsolete/unknown keys fall to the localized generic.
      final id = AppLocalizationsId();
      final en = AppLocalizationsEn();
      expect(
        resolveOrderActionLabel(id, 'action.mark_shipped'),
        'Kirim Pesanan',
      );
      expect(
        resolveOrderActionLabel(en, 'action.mark_shipped'),
        'Ship Order',
      );
      expect(resolveOrderActionLabel(id, 'action.provide_evidence'), 'Sediakan Bukti');
      expect(resolveOrderActionLabel(en, 'action.contact_support'), 'Contact Support');
      // Obsolete keys no longer translate (they were 'Terima Barang'/'Terima'/'Tolak').
      expect(resolveOrderActionLabel(id, 'action.complete_order'), 'Lanjutkan');
      expect(resolveOrderActionLabel(id, 'action.accept'), 'Lanjutkan');
      expect(resolveOrderActionLabel(id, 'action.reject'), 'Lanjutkan');
      expect(resolveOrderActionLabel(id, 'action.unknown_future'), 'Lanjutkan');
      expect(resolveOrderActionLabel(en, 'action.unknown_future'), 'Continue');
    });
  });
}
