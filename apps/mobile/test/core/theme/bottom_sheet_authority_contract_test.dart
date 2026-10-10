// CANONICAL MODAL BOTTOM SHEET AUTHORITY CONTRACT.
//
// Locks the converged foundation:
//  * ONE authority family (`AppBottomSheet*`) + ONE handle (`AppDragHandle`);
//  * surface / shape / elevation owned by `bottomSheetTheme`;
//  * three presentation categories (action / selection / form-content) with
//    real consumers;
//  * NO persistent, draggable or bespoke second renderer;
//  * NO obsolete sheet artifacts.
//
// The negative half is the point: a contract that only proves the family exists
// cannot stop a raw bypass from quietly re-appearing beside it.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

/// Every Dart source under `lib/` (test files excluded on purpose: prose that
/// names a purged artifact is allowed, compiled code is not).
Iterable<File> _libSources() => Directory('lib')
    .listSync(recursive: true)
    .whereType<File>()
    .where((file) => file.path.endsWith('.dart'));

String _read(String path) => File(path).readAsStringSync();

/// Source with comments removed — prose may name a purged artifact or an
/// avoided API; compiled code may not. CRLF is normalised so the comment
/// regexes cannot leave a trailing `\r` on every line.
String _code(String path) => _read(path)
    .replaceAll('\r\n', '\n')
    .replaceAll(RegExp(r'/\*[\s\S]*?\*/'), '')
    .replaceAll(RegExp(r'//[^\n]*'), '');

void main() {
  group('bottom sheet authority contract', () {
    test('the canonical family exists and exports the three categories', () {
      for (final path in const [
        'lib/shared/widgets/app_bottom_sheet.dart',
        'lib/shared/widgets/app_bottom_sheet_base.dart',
        'lib/shared/widgets/app_bottom_sheet_actions.dart',
        'lib/shared/widgets/app_bottom_sheet_list_selection.dart',
      ]) {
        expect(File(path).existsSync(), isTrue, reason: 'missing $path');
      }
      final barrel = _read('lib/shared/widgets/app_bottom_sheet.dart');
      expect(barrel, contains("export 'app_bottom_sheet_base.dart'"));
      expect(barrel, contains("export 'app_bottom_sheet_actions.dart'"));
      expect(barrel, contains("export 'app_bottom_sheet_list_selection.dart'"));
    });

    test('AppDragHandle is the single drag-handle authority', () {
      final definitions = <String>[];
      for (final file in _libSources()) {
        if (RegExp(
          r'\bclass\s+AppDragHandle\b',
        ).hasMatch(file.readAsStringSync())) {
          definitions.add(file.path.replaceAll(r'\', '/'));
        }
      }
      expect(definitions, ['lib/shared/widgets/app_bottom_sheet_base.dart']);
    });

    test(
      'the theme owns the sheet surface, shape and elevation (both modes)',
      () {
        for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
          final scheme = theme.colorScheme;
          final sheet = theme.bottomSheetTheme;

          expect(sheet.backgroundColor, scheme.surfaceContainerLow);
          expect(sheet.surfaceTintColor, Colors.transparent);
          expect(sheet.elevation, AppElevation.none);
          expect(sheet.modalElevation, AppElevation.none);

          final shape = sheet.shape;
          expect(shape, isA<RoundedRectangleBorder>());
          final radius = (shape! as RoundedRectangleBorder).borderRadius;
          expect(radius, isA<BorderRadius>());
          final corners = radius as BorderRadius;
          expect(corners.topLeft.x, AppShape.r20);
          expect(corners.topRight.x, AppShape.r20);
          expect(corners.bottomLeft, Radius.zero);
          expect(corners.bottomRight, Radius.zero);
        }
      },
    );

    test('action builder has real consumers', () {
      for (final path in const [
        'lib/shared/widgets/carousel_video_player.dart',
        'lib/shared/widgets/media_viewer_video_player.dart',
        'lib/domains/chat/chat/presentation/screens/chat_list_screen.dart',
        'lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart',
        'lib/domains/commerce/pricing/promotion/presentation/screens/'
            'external_product_detail_screen.dart',
      ]) {
        expect(
          _read(path),
          contains('AppBottomSheetActions.showActions'),
          reason: '$path must use the canonical action sheet',
        );
      }
    });

    test('selection builder has real consumers', () {
      for (final path in const [
        'lib/shared/widgets/theme_selector.dart',
        'lib/shared/widgets/language_selector.dart',
        'lib/domains/finance/transaction/payment/presentation/widgets/'
            'payment_method_picker_sheet.dart',
      ]) {
        expect(
          _read(path),
          contains('AppBottomSheetListSelection.showListSelection'),
          reason: '$path must use the canonical selection sheet',
        );
      }
    });

    test(
      'commerce resource picker uses the canonical base, not a raw modal',
      () {
        final source = _code(
          'lib/domains/social/comment/presentation/widgets/'
          'commerce_resource_picker.dart',
        );
        expect(source, contains('AppBottomSheetBase.show'));
        expect(source, isNot(contains('showModalBottomSheet')));
      },
    );

    test('base builder is used for form/content', () {
      expect(
        _read(
          'lib/domains/social/content/presentation/widgets/create_content/'
          'content_modals.dart',
        ),
        contains('AppBottomSheetBase.show'),
        reason:
            'the last compatibility-facade consumer must address the '
            'canonical builder directly',
      );
      expect(
        _read(
          'lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart',
        ),
        contains('AppBottomSheetBase.show'),
      );
    });

    test('the selection builder scrolls correctly', () {
      final source = _code(
        'lib/shared/widgets/app_bottom_sheet_list_selection.dart',
      );
      expect(
        source,
        isNot(contains('NeverScrollableScrollPhysics')),
        reason: 'the selection body must actually scroll',
      );
      expect(
        source,
        isNot(contains('shrinkWrap: true')),
        reason: 'no shrinkWrapped nested list as the primary body',
      );
    });

    test('every modal sheet is scroll-controlled', () {
      final calls = RegExp(r'showModalBottomSheet\s*(<[^>]+>)?\s*\(');
      for (final file in _libSources()) {
        final source = _code(file.path);
        if (!calls.hasMatch(source)) continue;
        expect(
          source,
          contains('isScrollControlled: true'),
          reason: '${file.path} must present a scroll-controlled modal sheet',
        );
      }
    });

    test('the address form is not draggable', () {
      final source = _code(
        'lib/domains/user/profile/presentation/widgets/address_form_dialog.dart',
      );
      expect(source, isNot(contains('DraggableScrollableSheet(')));
      // The handle is owned once by the base sheet: the form body carries no
      // local handle, and hosts keep the base defaults that render it.
      expect(source, isNot(contains('AppDragHandle')));
    });

    test('the address form fills the sheet allocation, never a ceiling', () {
      const path =
          'lib/domains/user/profile/presentation/widgets/address_form_dialog.dart';
      final source = _code(path);
      // The ceiling is the SHEET's alone (base owns `availableHeight * 0.9`);
      // the body consumes the sheet's live content allocation instead.
      // Re-spelling the ceiling at body level is the BOTTOMSHEET-02-FIT-GAP
      // duplicate: it claimed `handle + wrap + spacer` (72+N px) the content
      // region never had and parked the CTA below the content clip.
      expect(
        source,
        contains('AppBottomSheetBase.contentAllocationOf(context)'),
        reason: '$path must fill the sheet\'s allocated content region',
      );
      expect(
        source,
        isNot(contains('availableHeight')),
        reason: '$path re-spells the sheet ceiling — a second authority',
      );
      expect(
        source,
        isNot(contains('0.9')),
        reason: '$path re-applies the sheet ceiling fraction',
      );
      for (final raw in const [
        'MediaQuery.sizeOf(context).height',
        'size.height * 0.9',
      ]) {
        expect(
          source,
          isNot(contains(raw)),
          reason: '$path still measures raw screen height ($raw)',
        );
      }
      // The finite inner constraint itself stays: the body's
      // `Column > Expanded > ListView` needs the bound inside the base scroll.
      expect(source, contains('ConstrainedBox'));
      expect(source, contains('maxHeight'));
    });

    test('the address form bar uses the embedded lifted-sheet mode', () {
      const path =
          'lib/domains/user/profile/presentation/widgets/address_form_dialog.dart';
      final source = _code(path);
      // The hosting sheet already owns keyboard movement, so the embedded
      // bar must not rise a second time — AND the body must not re-spell the
      // lift either (BOTTOMSHEET-02: the ListView tail is design spacing;
      // geometry proof lives in the address-form inset test).
      expect(source, contains('embeddedInLiftedSheet: true'));
      expect(
        source,
        isNot(contains('viewInsets.bottom')),
        reason: '$path re-spells a body-owned keyboard reservation',
      );
    });

    test('the embedded bar mode defaults off and keeps its authorities', () {
      final source = _code('lib/shared/widgets/bottom_action_bar.dart');
      expect(source, contains('this.embeddedInLiftedSheet = false'));
      // Self-lift for unlifted placements and Safe Area handling stay owned.
      expect(source, contains('MediaQuery.viewInsetsOf(context).bottom'));
      expect(source, contains('SafeArea('));
      // Context boundary (BOTTOMSHEET-03): embedded mode spends NO second
      // system-bottom reservation — the base spacer is the one authority.
      expect(
        source,
        contains('bottom: !embeddedInLiftedSheet'),
        reason:
            'the bar must gate its SafeArea bottom on the existing embedded '
            'context — an unconditional SafeArea bottom is the duplicate '
            'system-bottom reservation (BOTTOMSHEET-03)',
      );
    });

    test('the base offers no caller-pinned sheet height', () {
      final source = _code('lib/shared/widgets/app_bottom_sheet_base.dart');
      // The sheet fit model is exactly ceiling + live content allocation; a
      // caller-pinned sheet height is a second fit authority beside them
      // (BOTTOMSHEET-04: unused by every consumer, purged).
      expect(source, isNot(contains('double? height')));
      expect(source, isNot(contains('height: height')));
    });

    test('no persistent or draggable sheet architecture remains', () {
      for (final file in _libSources()) {
        final source = _code(file.path);
        expect(
          source,
          isNot(contains('DraggableScrollableSheet(')),
          reason: '${file.path} still uses a draggable sheet',
        );
        expect(
          source,
          isNot(contains('showBottomSheet(')),
          reason: '${file.path} still uses the persistent showBottomSheet API',
        );
        expect(
          source,
          isNot(contains('showDragHandle: true')),
          reason:
              '${file.path} uses the framework drag handle instead of '
              'AppDragHandle',
        );
      }
    });

    test('the ONLY raw showModalBottomSheet calls are the renderer + exception', () {
      // Every raw `showModalBottomSheet(` call must be one of:
      //  1. the canonical family's OWN renderer; or
      //  2. the one genuine bespoke exception (interactive map).
      // Address-form hosts and picker consumers were expected to converge to a
      // canonical family builder or the `AddressFormDialog` body — if any of
      // them still holds a raw call, it is an ordinary bypass and this fails.
      const allowedRawCallFiles = <String>{
        'lib/shared/widgets/app_bottom_sheet_base.dart',
        'lib/shared/widgets/interactive_map_picker_bottom_sheet.dart',
      };
      // The call may be generic (`showModalBottomSheet<T>(`) or not.
      final call = RegExp(r'showModalBottomSheet\s*(<[^>]*>)?\s*\(');
      final rawCallFiles = <String>[];
      for (final file in _libSources()) {
        if (!call.hasMatch(_code(file.path))) continue;
        rawCallFiles.add(file.path.replaceAll(r'\', '/'));
      }
      rawCallFiles.sort();

      // Non-vacuous: the raw surface is EXACTLY the canonical renderer plus the
      // genuine bespoke map exception — nothing else.
      expect(
        rawCallFiles,
        allowedRawCallFiles.toList()..sort(),
        reason:
            'the raw showModalBottomSheet surface must be exactly the canonical '
            'renderer plus the genuine bespoke map exception, but found:\n'
            '${rawCallFiles.join('\n')}',
      );
    });

    test('the address-form hosts use the canonical base, not a raw modal', () {
      // Hosts present the canonical `AddressFormDialog` body through the
      // canonical builder — they own no raw presentation authority.
      const hosts = <String>[
        'lib/domains/user/profile/presentation/screens/address_list_screen.dart',
        'lib/domains/user/preference/seller/presentation/screens/'
            'seller_upgrade_wizard_screen.dart',
        'lib/domains/user/profile/presentation/widgets/'
            'address_picker_sheet.dart',
        'lib/domains/user/profile/presentation/widgets/'
            'address_selection_summary.dart',
      ];
      for (final path in hosts) {
        final source = _code(path);
        expect(
          source,
          contains('AppBottomSheetBase.show'),
          reason: '$path must host the canonical address form via the base',
        );
        expect(
          source,
          contains('AddressFormDialog('),
          reason: '$path must present the canonical AddressFormDialog body',
        );
        // Hosts keep the base defaults, which render the single base-owned
        // drag handle — disabling it here would leave the sheet handle-less.
        expect(
          source,
          isNot(contains('enableDrag:')),
          reason: '$path must not override the base drag contract',
        );
        expect(
          source,
          isNot(contains('showDragHandle:')),
          reason: '$path must not override the base handle contract',
        );
        expect(
          source,
          isNot(contains('showModalBottomSheet')),
          reason: '$path still owns a raw bottom-sheet presentation authority',
        );
      }
    });

    test('no second bottom-sheet renderer, service or facade exists', () {
      for (final file in _libSources()) {
        final source = _code(file.path);
        for (final name in const ['BottomSheetService', 'SheetManager']) {
          expect(
            source,
            isNot(contains(name)),
            reason: '${file.path} introduces a competing sheet authority',
          );
        }
      }
    });

    test('the duplicate playback-speed sheets are gone', () {
      for (final path in const [
        'lib/shared/widgets/carousel_video_player.dart',
        'lib/shared/widgets/media_viewer_video_player.dart',
      ]) {
        expect(
          _code(path),
          isNot(contains('showModalBottomSheet')),
          reason: '$path must not carry its own playback-speed sheet',
        );
      }
    });

    test('obsolete sheet artifacts are purged', () {
      for (final path in const [
        'lib/shared/widgets/app_bottom_sheet_settings.dart',
        'lib/shared/widgets/link_picker_modal.dart',
        'lib/shared/widgets/link_picker/link_list_item.dart',
        'lib/shared/widgets/link_picker/link_picker_tab_label.dart',
        'lib/domains/commerce/catalog/for_sale/presentation/widgets/'
            'for_sale_picker_bottom_sheet.dart',
        'lib/domains/system/report/presentation/dialogs/'
            'report_submission_dialog.dart',
        'lib/domains/social/share/presentation/widgets/'
            'share_to_chat_dialog.dart',
      ]) {
        expect(
          File(path).existsSync(),
          isFalse,
          reason: 'obsolete sheet file resurrected: $path',
        );
      }
    });

    test('no code names a purged sheet artifact', () {
      const purged = [
        // The compatibility facade class: pure delegation, one historical
        // consumer — converged to `AppBottomSheetBase.show` and removed.
        'AppBottomSheet',
        'AppBottomSheetSettings',
        'SettingsItem',
        'LinkPickerModal',
        'ForSalePickerBottomSheet',
        'ReportSubmissionDialog',
        'ShareToChatDialog',
      ];
      final offenders = <String>[];
      for (final file in _libSources()) {
        final source = _code(file.path);
        for (final name in purged) {
          if (RegExp('\\b$name\\b').hasMatch(source)) {
            offenders.add('${file.path} names $name');
          }
        }
      }
      expect(offenders, isEmpty, reason: offenders.join('\n'));
    });
  });
}
