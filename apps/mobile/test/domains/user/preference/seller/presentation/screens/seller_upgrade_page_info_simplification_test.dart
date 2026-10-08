// DAFTAR SELLER — PAGE INFO SIMPLIFICATION CONTRACT.
//
// Owner decisions:
//   A) "What You Get" (For Sale / Auction / Promotion, valid while subscription
//      is active) stays visible on the Package step; Seller status does NOT
//      expire.
//   B) `Lifecycle: {status}` is not shown to the user.
//   C) Fee + 365-day duration are the single package disclosure; the KYC/payout
//      and payment-activation explanations move to the canonical Info surface
//      and must not be repeated as warnings on every step.
//
// The canonical Page Info surface is `AppDialog.info`, opened from a
// page-owned AppBar trigger. This test locks the simplification and the
// residue purge (no broad raw-AlertDialog heuristic).
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

const _wizard =
    'lib/domains/user/preference/seller/presentation/screens/'
    'seller_upgrade_wizard_screen.dart';
const _preview =
    'lib/domains/user/preference/seller/presentation/widgets/wizard/'
    'seller_wizard_preview_widget.dart';

String _read(String path) => File(path).readAsStringSync();

String _code(String path) => _read(path)
    .replaceAll('\r\n', '\n')
    .replaceAll(RegExp(r'/\*[\s\S]*?\*/'), '')
    .replaceAll(RegExp(r'//[^\n]*'), '');

void main() {
  group('Daftar Seller — Page Info simplification', () {
    test('AppBar owns the canonical Info trigger opening AppDialog.info', () {
      final source = _read(_wizard);
      expect(source, contains("Icons.help_outline"));
      expect(source, contains('_showSellerInfo'));
      expect(source, contains('AppDialog.info('));
      expect(source, contains("title: 'Tentang Daftar Seller'"));
      expect(source, contains('_buildSellerInfoContent'));
    });

    test('package step keeps fee, 365-day duration and What You Get', () {
      final source = _read(_wizard);
      expect(source, contains('_buildPaidPlanCard'));
      expect(
        source,
        contains('AppFormatters.formatCurrency(upgradeConfig.yearlyFee)'),
      );
      expect(source, contains(r'/${upgradeConfig.durationDays} hari'));
      expect(source, contains('_buildFeaturesList'));
      expect(source, contains("'Buat For Sale'"));
      expect(source, contains("'Buat Auction'"));
      expect(source, contains("'Buat Promotion'"));
      expect(source, contains("'Berlaku selama subscription aktif'"));
    });

    test('expiry wording keeps Seller status, only capability expires', () {
      final source = _read(_wizard);
      expect(source, contains('tetapi status Seller Anda tetap'));
      expect(source, isNot(contains('Seller access stays active')));
      expect(source, isNot(contains('Seller status expires')));
      expect(source, isNot(contains('status Seller berakhir')));
    });

    test('KYC/payout explanation is consolidated into the Info surface once', () {
      final source = _code(_wizard);
      expect(
        RegExp('KYC').allMatches(source).length,
        1,
        reason: 'the KYC/payout explanation must live once, in AppDialog.info',
      );
      expect(source, contains('berlaku 365 hari'));
      // No KYC copy anywhere before the Info builder.
      final beforeInfo = source.substring(
        0,
        source.indexOf('void _showSellerInfo()'),
      );
      expect(beforeInfo.contains('KYC'), isFalse);
      expect(beforeInfo.contains('review bank'), isFalse);
    });

    test('secondary inline explanations are purged from wizard + preview', () {
      const dead = <String>[
        'Lifecycle:',
        'Fee sourced from backend config',
        '_buildModeBanner',
        '_buildStatusCard',
        "'Read only from your profile.'",
        'Username is read only when already saved',
        'canonical onboarding flow',
        'Seller access stays active',
        'Payment activates seller authority',
        'Payment is required before seller authority becomes active',
        'Onboarding hanya dipanggil setelah prerequisites valid',
        'Email status tetap read-only',
      ];
      for (final path in const [_wizard, _preview]) {
        final source = _read(path);
        for (final needle in dead) {
          expect(
            source.contains(needle),
            isFalse,
            reason: '$path still contains obsolete copy "$needle"',
          );
        }
      }
      // `SellerState` / the mode banner are gone from the wizard entirely.
      expect(_code(_wizard), isNot(contains('SellerState')));
      expect(_code(_wizard), isNot(contains('seller_state.dart')));
    });

    test('protected transaction/legal/prerequisite surfaces remain', () {
      final wizard = _read(_wizard);
      expect(wizard, contains('_buildPaymentSection'));
      expect(wizard, contains('PaymentMethodTrigger'));
      expect(wizard, contains('_buildPrimaryAddressSection'));
      expect(
        wizard,
        contains('CanonicalPhoneValidator.validationMessage'),
      );
      final preview = _read(_preview);
      expect(preview, contains('_buildTermsAgreement'));
      expect(preview, contains('Seller Terms'));
      expect(preview, contains('Package & Fee'));
      expect(preview, contains('Account Prerequisites'));
      expect(preview, contains('Store Information'));
    });
  });
}
