// PAGE INFO CONVERGENCE CONTRACT.
//
// CANONICAL TRUTH: `AppDialog.info` (`lib/shared/widgets/app_dialog.dart`) is
// the ONE Page Info / Help surface. This file locks the convergence + purge
// that removed the duplicate information dialogs:
//   * DiscountManagementInfoTooltip (trigger + raw AlertDialog)
//   * _showCoinInfo
//   * _showAddressInfoDialog
//   * _showVerificationHelpDialog
//   * _showMissingRequirementsDialog
//   * _showAccountBlockedDialog (verification + wizard)
//
// The negative half is the point: a contract that only proves the authority
// exists cannot stop an obsolete competitor from quietly re-appearing.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// Every Dart source under `lib/` (test prose may name a purged artifact, but
/// compiled code may not).
Iterable<File> _libSources() => Directory('lib')
    .listSync(recursive: true)
    .whereType<File>()
    .where((file) => file.path.endsWith('.dart'));

String _read(String path) => File(path).readAsStringSync();

/// Source with comments removed — prose may name a purged artifact; compiled
/// code may not. CRLF normalised so the comment regexes are deterministic.
String _code(String path) => _read(path)
    .replaceAll('\r\n', '\n')
    .replaceAll(RegExp(r'/\*[\s\S]*?\*/'), '')
    .replaceAll(RegExp(r'//[^\n]*'), '');

void main() {
  group('Page Info convergence — canonical authority', () {
    test('the obsolete discount tooltip authority is fully purged', () {
      const tooltipPath =
          'lib/domains/commerce/pricing/discount/presentation/widgets/'
          'discount_management_info_tooltip.dart';
      expect(
        File(tooltipPath).existsSync(),
        isFalse,
        reason: 'the competing tooltip authority must stay deleted',
      );
      for (final file in _libSources()) {
        expect(
          _code(file.path),
          isNot(contains('DiscountManagementInfoTooltip')),
          reason: '${file.path} names the purged discount info authority',
        );
      }
    });

    test('the obsolete private info-dialog implementations are purged', () {
      const dead = <String>[
        '_showCoinInfo',
        '_showAddressInfoDialog',
        '_showVerificationHelpDialog',
        '_showMissingRequirementsDialog',
        '_showAccountBlockedDialog',
      ];
      final offenders = <String>[];
      for (final file in _libSources()) {
        final source = _code(file.path);
        for (final name in dead) {
          if (source.contains(name)) {
            offenders.add('${file.path} still names $name');
          }
        }
      }
      expect(offenders, isEmpty, reason: offenders.join('\n'));
    });

    test('migrated consumers route their information through AppDialog.info', () {
      const migrated = <String, List<String>>{
        // The page owns the trigger; AppDialog.info is the surface.
        'lib/domains/commerce/pricing/discount/presentation/screens/'
            'seller_discount_list_screen.dart': [
          'AppDialog.info',
          "'Discount Management Guide'",
        ],
        'lib/domains/finance/wallet/coins/presentation/widgets/'
            'coin_balance_card.dart': ['AppDialog.info', "'Tentang LABUDA Coins'"],
        'lib/domains/user/profile/presentation/screens/address_list_screen.dart':
            ['AppDialog.info', "'Address Information'"],
        'lib/domains/user/preference/seller/presentation/screens/'
            'seller_verification_screen.dart': [
          'AppDialog.info',
          "'Bantuan Verifikasi'",
          "'Akun Ditangguhkan'",
          "'Akun Diblokir'",
        ],
        'lib/domains/user/preference/seller/presentation/screens/'
            'seller_upgrade_wizard_screen.dart': [
          'AppDialog.info',
          "'Lengkapi Prasyarat Seller'",
          "'Akun Ditangguhkan'",
          "'Akun Diblokir'",
        ],
      };
      migrated.forEach((path, needles) {
        final source = _read(path);
        for (final needle in needles) {
          expect(
            source.contains(needle),
            isTrue,
            reason: '$path must route its info surface through $needle',
          );
        }
      });
    });

    test('info-only migrated files compose no raw AlertDialog', () {
      for (final path in const [
        'lib/domains/commerce/pricing/discount/presentation/screens/'
            'seller_discount_list_screen.dart',
        'lib/domains/finance/wallet/coins/presentation/widgets/'
            'coin_balance_card.dart',
        'lib/domains/user/preference/seller/presentation/screens/'
            'seller_verification_screen.dart',
      ]) {
        expect(
          _code(path),
          isNot(contains('AlertDialog(')),
          reason: '$path must consume AppDialog.info, not a raw AlertDialog',
        );
      }
    });

    test('verification help keeps its navigation actions and wiring', () {
      final source = _code(
        'lib/domains/user/preference/seller/presentation/screens/'
        'seller_verification_screen.dart',
      );
      // The two required actions survive as content actions on the surface.
      expect(source, contains("'Lihat Panduan'"));
      expect(source, contains("'Hubungi Support'"));
      // ...and stay wired to their canonical destinations.
      expect(source, contains("helpCategoryPath('verification')"));
      expect(source, contains('showPreChatFormRefactored('));
      // The banner still exposes the help entry point.
      expect(source, contains('_openVerificationHelp'));
    });

    test('no second Page Info / generic help dialog authority exists', () {
      for (final file in _libSources()) {
        final source = _code(file.path);
        for (final name in const <String>[
          'PageInfoDialog',
          'PageInfoSheet',
          'PageInfoSurface',
          'GenericHelpDialog',
          'VerificationHelpDialog',
          'AppInfoDialog',
          'AppHelpDialog',
          'HelpDialog',
          'InfoDialog',
        ]) {
          expect(
            RegExp('\\bclass\\s+$name\\b').hasMatch(source),
            isFalse,
            reason: '${file.path} introduces a second authority ($name)',
          );
        }
      }
    });
  });
}
