// SELLER EARNINGS — PAGE INFO SIMPLIFICATION CONTRACT.
//
// Owner decisions:
//   1. The detailed "Tentang Penghasilan" definitions move behind the
//      canonical AppBar Info surface (`AppDialog.info`).
//   2. "Butuh Bantuan Penarikan?" moves behind the same surface; both actions
//      (Panduan Penarikan, Hubungi Support) must remain wired.
//
// The page keeps every primary financial surface and the withdraw dialog.
// This locks the simplification and the residue purge (no broad raw-AlertDialog
// heuristic).
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

const _screen =
    'lib/domains/user/preference/seller/presentation/screens/'
    'seller_earnings_screen.dart';

String _read(String path) => File(path).readAsStringSync();

String _code(String path) => _read(path)
    .replaceAll('\r\n', '\n')
    .replaceAll(RegExp(r'/\*[\s\S]*?\*/'), '')
    .replaceAll(RegExp(r'//[^\n]*'), '');

void main() {
  group('Seller Earnings — Page Info simplification', () {
    test('AppBar owns the canonical Info trigger opening AppDialog.info', () {
      final source = _read(_screen);
      expect(source, contains('Icons.help_outline'));
      expect(source, contains('_showSellerInfo'));
      expect(source, contains('AppDialog.info('));
      expect(source, contains("'Tentang Penghasilan & Penarikan'"));
      expect(source, contains('_buildSellerInfoContent'));
    });

    test('Info content preserves the earnings/withdrawal facts', () {
      final source = _read(_screen);
      expect(source, contains("'Saldo Tersedia'"));
      expect(source, contains("'Saldo Tertahan'"));
      expect(source, contains('Minimum penarikan Rp'));
      expect(source, contains('WithdrawRequest.minAmount'));
      expect(source, contains('Biaya penarikan'));
      expect(source, contains('dipotong dari jumlah yang diminta'));
    });

    test('both withdrawal help actions remain wired', () {
      final source = _read(_screen);
      expect(source, contains("'Panduan Penarikan'"));
      expect(source, contains("'Hubungi Support'"));
      expect(source, contains('RoutePaths.helpCenter'));
      expect(source, contains('showPreChatFormRefactored('));
    });

    test('primary financial/withdrawal surfaces remain visible', () {
      final source = _read(_screen);
      // Four balance cards.
      expect(source, contains('_buildBalanceCard'));
      for (final title in const [
        "'Saldo Tersedia'",
        "'Total Penghasilan'",
        "'Saldo Tertahan'",
        "'Total Penarikan'",
      ]) {
        expect(source, contains(title));
      }
      expect(source, contains('_buildWithdrawalHistorySection'));
      expect(source, contains('_buildManageBankAccountCard'));
      expect(source, contains('_buildMinBalanceInfo'));
      expect(source, contains('_buildWithdrawButton'));
      // Withdrawal action remains the canonical dialog, untouched.
      expect(source, contains('showWithdrawDialog('));
    });

    test('obsolete inline informational surfaces are purged', () {
      final source = _code(_screen);
      for (final dead in const [
        '_buildInfoSection',
        '_buildInfoItem',
        '_buildWithdrawalHelpSection',
        "'Tentang Penghasilan'",
        "'Butuh Bantuan Penarikan?'",
      ]) {
        expect(
          source.contains(dead),
          isFalse,
          reason: 'seller earnings still contains obsolete "$dead"',
        );
      }
    });
  });
}
