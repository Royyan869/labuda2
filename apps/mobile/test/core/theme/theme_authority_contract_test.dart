// Tahap 0 — Theme authority contract (FOUNDATION, not a screen).
//
// CANONICAL TRUTH: `AppTheme.lightTheme` / `AppTheme.darkTheme` is the single
// colour/type authority. Widgets read from `Theme.of(context)`; `AppColors`
// is the raw token store for `app_colors.dart`/`app_theme.dart` only.
//
// This file locks the foundation in two halves:
//  1. Positive proof: both themes expose complete, scheme-derived slots with
//     proper M3 tonal direction (light containers step darker off the
//     surface, dark containers step lighter).
//  2. Negative gate: the migrated-UI registry. Directories listed here have
//     no competing colour authority (no palette binds, no raw Material
//     colours, no raw hex, no local isDark/brightness branches). The registry
//     starts with the already-migrated checkout scope and GROWS one domain
//     per scope — legacy domains stay out until their migration scope closes
//     them. Nothing here may ever shrink the registry.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';

/// Files or directories whose UI has been migrated to the scheme authority,
/// one entry per closed migration scope. Paths are relative to
/// `apps/mobile/lib/`.
const _migratedUiPaths = <String>[
  // Seed: checkout scope (proven by checkout_theme_authority_contract_test).
  // NOTE: the payment-result screens live in the same folder but belong to
  // the Payment Result domain, so the seed names the checkout screen file
  // explicitly instead of the whole screens dir.
  'domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart',
  'domains/commerce/transaction/checkout/presentation/widgets',
  // Auction domain CLOSED: every auction-owned UI surface reads the scheme
  // authority (detail screen + all detail widgets incl. claim modal, card,
  // bidding, create/seller/list screens, marketplace tab).
  'domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
  'domains/commerce/catalog/auction/presentation/screens/bidding_screen.dart',
  'domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart',
  'domains/commerce/catalog/auction/presentation/screens/seller_auctions_screen.dart',
  // auction_list_screen.dart REMOVED (dormant, unrouted browse surface) —
  // see marketplace_surface_contract_test. A registry entry for a deleted
  // file is dead, not a migration.
  'domains/commerce/catalog/auction/presentation/screens/seller_auction_draft_edit_screen.dart',
  'domains/commerce/catalog/auction/presentation/widgets/detail',
  'domains/commerce/catalog/auction/presentation/widgets/auction_card.dart',
  'features/marketplace/presentation/widgets/marketplace_auction_tab.dart',
  // For-sale domain CLOSED: sibling sale channel over Product (detail
  // screen + card + list + seller management + create/edit + picker +
  // marketplace tab).
  'domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart',
  'domains/commerce/catalog/for_sale/presentation/screens/for_sale_list_screen.dart',
  'domains/commerce/catalog/for_sale/presentation/screens/my_for_sales_screen.dart',
  'domains/commerce/catalog/for_sale/presentation/screens/create_for_sale_screen.dart',
  'domains/commerce/catalog/for_sale/presentation/screens/edit_for_sale_screen.dart',
  'domains/commerce/catalog/for_sale/presentation/widgets/for_sale_card.dart',
  'domains/commerce/catalog/for_sale/presentation/widgets/for_sale_picker_bottom_sheet.dart',
  // for_sale_media_handler.dart REMOVED (dead media path: for_sale converged
  // onto MediaGridUploader / MediaUploadOrchestrator, zero lib consumers).
  // A registry entry for a deleted file is dead, not a migration.
  'features/marketplace/presentation/widgets/marketplace_for_sale_tab.dart',
  // Small closures: marketplace shell, router error page, chat surfaces.
  'features/marketplace/presentation/screens/marketplace_screen.dart',
  'core/src/router/router_error_page.dart',
  'domains/chat/chat/presentation/screens/new_chat_screen.dart',
  'domains/chat/chat/presentation/widgets/new_chat_user_list_widget.dart',
  'domains/chat/chat/presentation/screens/chat_list_screen.dart',
  'domains/chat/chat/presentation/widgets/chat/chat_order_status_banner.dart',
  // User settings sections CLOSED as one slice.
  'domains/user/profile/presentation/widgets/settings_app_preferences_section.dart',
  'domains/user/profile/presentation/widgets/settings_account_management_section.dart',
  'domains/user/profile/presentation/widgets/settings_marketing_section.dart',
  'domains/user/profile/presentation/widgets/settings_profile_identity_section.dart',
  'domains/user/profile/presentation/widgets/settings_security_privacy_section.dart',
  'domains/user/profile/presentation/widgets/settings_support_section.dart',
  'domains/user/profile/presentation/screens/profile_screen/about_sections/about_section_about.dart',
  'domains/user/profile/presentation/screens/profile_screen/about_sections/about_section_contact.dart',
  'domains/user/profile/presentation/screens/profile_screen/about_sections/about_section_farm.dart',
  'domains/user/profile/presentation/screens/profile_screen/profile_about_tab.dart',
  'domains/user/profile/presentation/screens/profile_screen.dart',
  'domains/user/profile/presentation/screens/unified_edit_profile_screen.dart',
  'domains/user/profile/presentation/screens/address_list_screen.dart',
  'domains/user/profile/presentation/widgets/address_form_dialog.dart',
  'domains/user/profile/presentation/screens/address_list_screen/address_empty_state.dart',
  'domains/user/profile/presentation/screens/address_list_screen/address_info_dialog.dart',
  'domains/user/profile/presentation/screens/address_list_screen/address_sticky_button.dart',
  'domains/user/profile/presentation/widgets/add_edit_address_dialog/address_dialog_actions.dart',
  'domains/user/profile/presentation/widgets/add_edit_address_dialog/address_dialog_header.dart',
  'domains/user/profile/presentation/widgets/add_edit_address_dialog/address_form_fields.dart',
  'domains/user/profile/presentation/widgets/add_edit_address_dialog/address_map_picker_field.dart',
  'domains/user/profile/presentation/widgets/add_edit_address_dialog/address_purpose_field.dart',
  'domains/user/profile/presentation/widgets/personal_info/date_of_birth_picker.dart',
  'domains/user/profile/presentation/widgets/personal_info/phone_verification_field.dart',
  // Profile widget layer CLOSED as one slice: every widget under
  // presentation/widgets (incl. personal_information_section, bank dialogs,
  // profile feed tab, settings sections, phone verification family) and the
  // whole presentation/shared tree (profile_text_field, profile_state_view)
  // reads the scheme authority. Dead wizard/KTP residue was PURGED
  // (seller_wizard_step1/3, ktp_upload/preview, selfie_verification,
  // address_card_widget — zero callers, identity UI lives in
  // preference/seller/.../seller_verification_screen).
  'domains/user/profile/presentation/widgets',
  'domains/user/profile/presentation/shared',
  'domains/user/profile/presentation/screens/personal_information_screen.dart',
  // Profile screens layer CLOSED as one slice: every screen under
  // presentation/screens (security, profile_qr, settings, bank_account,
  // blocked_users, edit_profile sections, ktp/selfie camera rooms, address
  // list, about tabs, unified edit profile) reads the scheme authority.
  // Camera rooms + the QR export surface keep fixed pixels via scheme roles
  // (scrim = black, onPrimary = white in both modes) — photo/scanner-bound,
  // never a local theme branch. Dead settings subtree PURGED
  // (settings_role_cards.dart + settings_dialogs.dart — zero callers).
  'domains/user/profile/presentation/screens',
  // Shared dropdown/address family CLOSED (app dropdown, wilayah
  // province/city/district/village + search, state builders, decoration
  // helper, base container).
  'shared/widgets/app_dropdown.dart',
  'shared/widgets/wilayah/province_dropdown.dart',
  'shared/widgets/wilayah/city_dropdown.dart',
  'shared/widgets/wilayah/district_dropdown.dart',
  'shared/widgets/village_dropdown.dart',
  'shared/widgets/village_search_dropdown.dart',
  'shared/widgets/wilayah/dropdown_state_builders.dart',
  'shared/widgets/wilayah/dropdown_decoration_helper.dart',
  'shared/widgets/wilayah/base_dropdown_container.dart',
  'shared/widgets/wilayah_dropdown.dart',
  // Shared media scope CLOSED: grid uploader (+compact strip), app image
  // (all placeholders/shimmer/error states scheme-driven).
  'shared/widgets/media_grid_uploader.dart',
  'shared/widgets/app_image.dart',
  // Map/location shared scope CLOSED: accuracy warning + location card,
  // plus map viewer/player/crop paths above use scheme roles.
  'shared/widgets/map_picker/map_accuracy_indicator.dart',
  'shared/widgets/clickable_location_widget.dart',
  // Notification UI scope CLOSED: item, dismiss, appbar and empty state
  // use scheme roles; domain display colors are mapped to semantic roles.
  'domains/system/notification/presentation/widgets/notification_item_widget.dart',
  'domains/system/notification/presentation/widgets/notification_dismissible_item.dart',
  // System support + report scope CLOSED as one slice: help center,
  // ticket list/thread, pre-chat sheet, suggested messages, ticket card,
  // report screens/dialogs/description field/reason selector all read the
  // scheme authority. isDark/brightness forks PURGED; category greys and
  // Material palette map to scheme roles + status* semantic tokens.
  'domains/system/support/presentation',
  'domains/system/report/presentation',
  'domains/system/notification/presentation/widgets/notification_list_app_bar.dart',
  'domains/system/notification/presentation/widgets/notification_empty_state_widget.dart',
  'domains/system/notification/presentation/widgets/preference_toggle_widget.dart',
  'domains/system/notification/presentation/screens/notification_list_screen.dart',
  // Shared bottom-sheet foundation + media picker actions CLOSED.
  'shared/widgets/app_bottom_sheet_base.dart',
  'shared/widgets/app_bottom_sheet_actions.dart',
  'shared/widgets/app_bottom_sheet_media_picker.dart',
  'shared/widgets/link_picker_modal.dart',
  'shared/widgets/coordinate_preview_modal.dart',
  'shared/widgets/create_content_bottom_sheet.dart',
  'domains/system/notification/presentation/widgets/preference_toggle_widget.dart',
  'shared/widgets/app_bottom_sheet_media_picker.dart',
  'shared/widgets/media_viewer_widget.dart',
  'shared/widgets/media_carousel_widget.dart',
  'shared/widgets/media_viewer_navigation.dart',
  'shared/widgets/media_viewer_indicators.dart',
  'shared/widgets/carousel_indicators.dart',
  'shared/widgets/media_viewer_video_player.dart',
  'shared/widgets/carousel_video_player.dart',
  'shared/widgets/media_video_item.dart',
  'shared/widgets/media_image_item.dart',
  'shared/widgets/avatar_picker_options.dart',
  'shared/widgets/web_image_cropper.dart',
  'shared/widgets/custom_image_cropper.dart',
  'shared/widgets/flutter_crop_image.dart',
  // Shared viewer/player scope CLOSED: immersive dark rooms keep fixed
  // dark via scrim/onPrimary (identical pixels, photo-bound); shimmer,
  // progress, menus, badges scheme-driven. Crop painters take overlay
  // ink as ctor params (canvas has no context).
  'shared/widgets/media_viewer_widget.dart',
  'shared/widgets/media_carousel_widget.dart',
  'shared/widgets/media_viewer_navigation.dart',
  'shared/widgets/media_viewer_indicators.dart',
  'shared/widgets/carousel_indicators.dart',
  'shared/widgets/media_viewer_video_player.dart',
  'shared/widgets/carousel_video_player.dart',
  'shared/widgets/media_video_item.dart',
  'shared/widgets/media_image_item.dart',
  'shared/widgets/avatar_picker_options.dart',
  'shared/widgets/app_bottom_sheet_media_picker.dart',
  'shared/widgets/web_image_cropper.dart',
  'shared/widgets/custom_image_cropper.dart',
  'shared/widgets/flutter_crop_image.dart',
  // Transaction-periphery scope CLOSED: payment result (impl + sections),
  // payment webview + method picker, shipping setup/selector, and the whole
  // order presentation layer (detail/list screens, info/pricing/refund cards,
  // dialogs, action buttons) read the scheme authority.
  'domains/commerce/transaction/checkout/presentation/screens/payment_result_screen_impl.dart',
  'domains/commerce/transaction/checkout/presentation/screens/payment_result_screen_sections.dart',
  'domains/finance/transaction/payment/presentation',
  'domains/commerce/transaction/shipping/presentation',
  'domains/commerce/transaction/order/presentation',
  // Shared widgets scope CLOSED: every cross-domain widget reads the scheme
  // authority (avatar, badge, snackbar, app bar, back button, popup menu,
  // detail chip).
  'shared/widgets/profile_avatar.dart',
  'shared/widgets/seller_dual_avatar.dart',
  'shared/governance/seller_tier_badge.dart',
  'shared/widgets/popup_more_options_button.dart',
  'shared/widgets/app_snackbar.dart',
  'shared/widgets/app_bar_custom.dart',
  'shared/widgets/app_back_button.dart',
  'shared/widgets/detail_chip_widget.dart',
  'shared/widgets/detail_chip_types.dart',
  // Duplicate kills CLOSED: EmptyStateWidget purged (callers on
  // EmptyState), welcome theme-picker copy purged (canonical shared
  // showThemeSelectionSheet; welcome + full ThemeSelector migrated).
  'shared/widgets/theme_selector.dart',
  'domains/user/preference/onboarding/presentation/screens/welcome_screen.dart',
  // Seller domain CLOSED as one slice: every screen (dashboard, upgrade
  // wizard, earnings, verification, withdraw, renewal, shipping) and every
  // widget (profile store tab, upgrade card, withdraw dialog, wizard
  // step2/nav/preview/helpers) reads the scheme authority. isDark threading
  // was PURGED end-to-end (local brightness vars, method params, widget
  // fields + ctor args); status colours map to status*/primary* semantic
  // tokens; warning/error button ink maps to onPrimary/onError.
  'domains/user/preference/seller/presentation',
  'domains/user/preference/seller/presentation/widgets/profile_store_tab.dart',
  // Shared atoms scope CLOSED: button, text field, modal, empty state.
  // EmptyStateWidget is PURGED (duplicate authority killed: callers moved
  // to EmptyState, file + barrel export deleted).
  'shared/widgets/app_button.dart',
  'shared/widgets/app_text_field.dart',
  'shared/widgets/app_modal.dart',
  'shared/widgets/empty_state.dart',
  // Shared cards/badges scope CLOSED: status badge (dormant dot factory
  // purged, zero callers), base card, card header, list item trailing +
  // decorations, follow button, blocked banner, image badge overlays,
  // metric card.
  'shared/widgets/status_badge.dart',
  'shared/widgets/base_card.dart',
  'shared/widgets/card_header.dart',
  'shared/widgets/list_item/list_item_trailing.dart',
  'shared/widgets/list_item/list_item_decorations.dart',
  'shared/widgets/follow_button.dart',
  'shared/widgets/blocked_user_banner.dart',
  'shared/widgets/image_with_badge.dart',
  'shared/widgets/base_metric_card.dart',
  // Pricing/discount domain CLOSED as one slice: create/edit/list screens,
  // discount card, input field, management tooltip, and every
  // create_discount_form section (basic info, type, applies-to, validity,
  // limits) read the scheme authority. Local date-picker ThemeData override
  // was DELETED (theme warisan — the app theme already owns the picker).
  'domains/commerce/pricing/discount/presentation',
  // Pricing/promotion domain CLOSED as one slice: all five screens
  // (canonical analytics/create/list, external product detail/management)
  // read the scheme authority. Flat light-palette tokens (no isDark branch)
  // map straight to scheme roles; status accents map to statusInfo/warning
  // tokens; _statusColor threads BuildContext so the gray role comes from
  // the scheme; white card/sheet fills map to surface, button ink to
  // onPrimary; const Icon/SnackBar/Text/BoxDecoration parents that wrapped
  // palette literals were de-const'ed so the scheme read is legal.
  'domains/commerce/pricing/promotion/presentation',
  // Notification domain CLOSED as one slice: settings screen (raw-hex
  // slate palette PURGED — isDark fork + 6 raw hex + surface/heading/body
  // color threading deleted, screen now inherits AppBar/scaffold from the
  // authority), settings section, in-app banner (isDark param threading
  // PURGED from state methods), list content, badge widget, the whole
  // preference_groups family, dialog helper, plus the UI-producing
  // services (navigation service maintenance modal, fcm action mapper
  // banner action inks). Colors.* Material palette maps to status*/primary*
  // semantic tokens; scrim/shadow ink maps to scheme.shadow.
  // Shared loading/icon fallback scope CLOSED: FullScreenLoading scrim
  // maps to scheme.scrim (identical pixels, canonical M3 barrier role);
  // CustomRpIcon/RpIcon ink fallback maps to onSurface (fixes the old
  // AppColors.dark bind rendering near-invisible ink in dark mode).
  'shared/widgets/loading_indicator.dart',
  'shared/widgets/custom_rp_icon.dart',
  // Chat legacy surfaces CLOSED as one slice: chat card (support-category
  // Material switch mapped to status*/brand tokens, BuildContext threaded
  // into the timestamp helper), message bubble (bubble fills, meta ink,
  // reply-preview ink, media scrim/surface from scheme — the incoming-image
  // caption no longer hardcodes white on a light bubble), input area
  // (composer surface/shadow/disabled ink), unread badge, shipping-quote
  // modal (brand tint from scheme.primary, fields from scheme surfaces),
  // and chat detail surfaces (degraded header, online ink, error/empty
  // views, message-option sheets, report follow-up dialog, date header).
  'domains/chat/chat/presentation/widgets',
  'domains/chat/chat/presentation/screens',
  // Coins domain CLOSED as one slice: balance-card ink sitting on the fixed
  // coin gradient reads scheme.onPrimary instead of the forbidden
  // neutralWhite bind; the transaction-row border brightness fork is gone
  // (outlineVariant); amount deltas map to statusSuccess/statusError; the
  // history screen's grey.shade ramp maps to onSurface/onSurfaceVariant.
  // Coin brand tokens (coinPrimary/coinSecondary/coinGradient) stay — they
  // have no scheme role and remain legitimate.
  'domains/finance/wallet/coins/presentation',
  // Onboarding + saved-item + seller slice CLOSED: splash screen (isDark
  // fork, inline brightness forks, branching gradient and 20
  // neutral/darkGray binds PURGED — gradient is now
  // lowest/surface/lowest in both modes), saved-item badge (statusError ink
  // + scheme.shadow), whole preference tree reads the scheme authority.
  'domains/user/preference',
  // Shared camera / crop / map-picker / upload scope CLOSED: immersive
  // camera rooms keep fixed pixels through scheme roles (scrim = black,
  // onPrimary = white in both modes), crop overlays read scrim, map-picker
  // shadows read scheme.shadow, upload/attachment accents read scheme
  // roles, crop editors keep OS-chrome statusBarBrightness (gate-exempt).
  'shared/ui/src/screens/custom_camera_screen.dart',
  'shared/ui/src/widgets/text_input_widget_refactored.dart',
  'shared/ui/src/helpers/media_picker_helper.dart',
  'shared/widgets/flutter_crop_image.dart',
  'shared/widgets/attachment_widget.dart',
  'shared/widgets/interactive_map_picker_bottom_sheet.dart',
  'shared/widgets/map_picker',
  'shared/src/widgets/upload_task_utils.dart',
  'core/media/media_upload_orchestrator.dart',
  'features/search/search/presentation/utils/search_result_type_helper.dart',
  'domains/user/identity/authentication/presentation/screens/login_sessions_screen.dart',
  // Social surfaces CLOSED: share preview card (auction bid → scheme.primary,
  // budget → scheme.secondary, BuildContext threaded into the metadata
  // builders), content toolbar/metadata/modal accents and the detail
  // screen's media overlay + report ink read scheme roles.
  'domains/social/content/presentation',
  'domains/social/share/presentation',
  // Snackbar single-authority slice CLOSED: every per-screen toast that used
  // to paint itself (`SnackBar(backgroundColor:)`) now routes through the
  // canonical AppSnackBar.show{Success,Error,Info,Warning}; the
  // BuildContext.showSnackBar extension no longer forks on a Color argument.
  // The files below were opened by that sweep and stay locked here.
  'core/utils/notification_navigation_handler.dart',
  'core/src/utils/extensions/context_extensions.dart',
  'shared/widgets/external_link_interstitial.dart',
  'features/home/presentation/providers/feed_renderers.dart',
  'domains/social/comment/presentation/screens/discussion_screen.dart',
  'domains/social/comment/presentation/widgets/comment_input_with_commerce_reference.dart',
];

void main() {
  group('theme foundation positive proof', () {
    test('app.dart wires AppTheme as the single runtime authority', () {
      final source = File('lib/app.dart').readAsStringSync();
      // ONE authority: MaterialApp must resolve themes exclusively through
      // AppTheme's static ThemeData pair, never a factory/helper that could
      // silently resolve a different palette.
      expect(source.contains('theme: AppTheme.lightTheme'), isTrue);
      expect(source.contains('darkTheme: AppTheme.darkTheme'), isTrue);
      expect(source.contains('themeMode: themeMode'), isTrue);
    });

    test('both themes carry the canonical scheme roles', () {
      for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
        final s = theme.colorScheme;
        // Identity roles the rest of the app already depends on — unchanged.
        expect(s.primary, AppColors.primaryRed);
        expect(s.onPrimary, AppColors.neutralWhite);
        // Foundation roles Tahap 0 introduced — must exist and differ from
        // the flat defaults (pure black/white outline, containers == surface).
        expect(s.onSurfaceVariant, isNot(s.onSurface));
        expect(s.outlineVariant, isNot(s.onSurface));
        expect(s.surfaceContainerHighest, isNot(s.surface));
        expect(s.surfaceContainerLow, isNot(isNull));
      }
    });

    test('tonal direction is proper M3 in both modes', () {
      final light = AppTheme.lightTheme.colorScheme;
      final dark = AppTheme.darkTheme.colorScheme;
      // Light containers step DARKER off the surface; dark steps LIGHTER.
      expect(
        light.surfaceContainerHighest.computeLuminance(),
        lessThan(light.surface.computeLuminance()),
      );
      expect(
        dark.surfaceContainerHighest.computeLuminance(),
        greaterThan(dark.surface.computeLuminance()),
      );
      // Lowest stays at/beyond the surface on each side.
      expect(
        light.surfaceContainerLowest.computeLuminance(),
        greaterThanOrEqualTo(light.surface.computeLuminance()),
      );
      expect(
        dark.surfaceContainerLowest.computeLuminance(),
        lessThanOrEqualTo(dark.surface.computeLuminance()),
      );
    });

    test('component themes are pinned to scheme roles (both modes)', () {
      for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
        final s = theme.colorScheme;
        expect(theme.scaffoldBackgroundColor, s.surface);
        expect(theme.dialogTheme.backgroundColor, s.surfaceContainerHigh);
        expect(
          theme.bottomSheetTheme.backgroundColor,
          s.surfaceContainerLow,
        );
        expect(theme.dividerTheme.color, s.outlineVariant);
        expect(theme.listTileTheme.iconColor, s.onSurfaceVariant);
        expect(theme.snackBarTheme.backgroundColor, s.inverseSurface);
        expect(theme.snackBarTheme.actionTextColor, s.inversePrimary);
        // Text defaults resolve to the scheme surface ink in both modes.
        expect(theme.textTheme.bodyLarge?.color, s.onSurface);
        expect(theme.textTheme.labelLarge?.color, s.onSurface);
      }
    });

    test('dark and light stay distinct authorities', () {
      final light = AppTheme.lightTheme.colorScheme;
      final dark = AppTheme.darkTheme.colorScheme;
      expect(dark.surface, isNot(light.surface));
      expect(dark.onSurface, isNot(light.onSurface));
      expect(dark.surfaceContainerHighest, isNot(light.surfaceContainerHighest));
      expect(dark.outlineVariant, isNot(light.outlineVariant));
    });
  });

  group('migrated-UI registry negative gate', () {
    test('registry never shrinks below its seed', () {
      expect(
        _migratedUiPaths,
        contains(
          'domains/commerce/transaction/checkout/presentation/widgets',
        ),
      );
    });

    test('no migrated UI file owns a colour authority', () {
      // Palette binds, raw Material colours, raw hex, and local theme
      // branches are the competing authorities Tahap 0 killed. Brand/semantic
      // tokens with no scheme role (coin*, koi*, status*) remain legitimate
      // via AppColors and are NOT matched here. OS-chrome lines
      // (statusBarBrightness / statusBarIconBrightness in immersive
      // dark-room editors) are skipped: they address the OS, not widgets.
      final forbidden = RegExp(
        r'AppColors\.(neutral\w*|darkGray\w*|light|dark|neutral|primaryRed|primaryBlue)\b'
        r'|Colors\.(white|black|grey|gray|green|orange|red|blue|yellow|amber)\b'
        r'|Color\(0x'
        r'|isDark'
        r'|brightness\s*=='
        r'|Brightness\.dark',
      );
      final osChrome = RegExp(r'statusBar(Brightness|IconBrightness)');

      final violations = <String>[];
      final files = <File>[];
      for (final path in _migratedUiPaths) {
        final type = FileSystemEntity.typeSync('lib/$path');
        expect(
          type,
          isNot(FileSystemEntityType.notFound),
          reason: 'migrated registry path missing: lib/$path',
        );
        if (type == FileSystemEntityType.file) {
          files.add(File('lib/$path'));
        } else {
          files.addAll(
            Directory('lib/$path')
                .listSync(recursive: true)
                .whereType<File>()
                .where((f) => f.path.endsWith('.dart')),
          );
        }
      }
      for (final file in files) {
        final lines = file.readAsLinesSync();
        for (var i = 0; i < lines.length; i++) {
          if (osChrome.hasMatch(lines[i])) continue;
          if (forbidden.hasMatch(lines[i])) {
            violations.add(
              '${file.path.replaceAll(r'\', '/')}:${i + 1}: '
              '${lines[i].trim()}',
            );
          }
        }
      }
      expect(
        violations,
        isEmpty,
        reason: 'migrated UI owns no colour authority:\n'
            '${violations.join('\n')}',
      );
    });
  });

  group('foundation zombie/alias gate', () {
    test('AppColors carries no backward-compat colour aliases', () {
      final source = File(
        'lib/core/src/theme/app_colors.dart',
      ).readAsStringSync();
      // Killed in Scope F: flat light/dark/neutral binds had no scheme
      // meaning and invited off-authority colour picks. Must not return.
      expect(source.contains('Color light ='), isFalse);
      expect(source.contains('Color dark ='), isFalse);
      expect(source.contains('Color neutral ='), isFalse);
    });

    test('ThemeState exposes no brightness-branch helpers', () {
      final source = File(
        'lib/core/src/theme/theme_provider.dart',
      ).readAsStringSync();
      // Zombie helpers purged in Scope F: zero callers — widgets read
      // Theme.of(context), never ThemeState brightness forks.
      expect(source.contains('isDarkMode'), isFalse);
      expect(source.contains('getCurrentBrightness'), isFalse);
    });

    test('no second ThemeData builder survives in the theme layer', () {
      final source = File(
        'lib/core/src/theme/theme_provider.dart',
      ).readAsStringSync();
      // Killed: ThemeHelper.getThemeData duplicated AppTheme's job by
      // building ThemeData from a Brightness fork — a second authority.
      expect(source.contains('ThemeHelper'), isFalse);
      expect(source.contains('getThemeData'), isFalse);
    });

    test('component factory authority stays deleted', () {
      // Killed: the factory resolved component palettes off a locked light
      // brightness, competing with Theme.of(context). Must not be recreated.
      expect(
        File('lib/shared/ui/factory/component_factory.dart').existsSync(),
        isFalse,
      );
      expect(
        Directory('lib/shared/ui/factory').existsSync(),
        isFalse,
      );
    });

    test('base_component owns no colour/size authority', () {
      final source = File(
        'lib/shared/ui/base/base_component.dart',
      ).readAsStringSync();
      // Killed: ComponentSize/ComponentSpacing(value) duplicated the theme's
      // layout scale and had zero consumers.
      expect(source.contains('ComponentSize'), isFalse);
      expect(source.contains('ComponentSpacing'), isFalse);
    });
  });

  group('snackbar single-authority gate', () {
    test('showSnackBar extension owns no colour decision', () {
      final source = File(
        'lib/core/src/utils/extensions/context_extensions.dart',
      ).readAsStringSync();
      // Killed: the extension forked error/success/info off a raw
      // `backgroundColor` argument — the same decision AppSnackBar already
      // owns. It now delegates with no colour parameter at all.
      expect(source.contains('backgroundColor'), isFalse);
      expect(source.contains('AppSnackBar.showInfo'), isTrue);
    });

    test('no per-screen SnackBar re-decides its palette', () {
      // Killed: ~40 call sites passed `backgroundColor:` into a raw SnackBar,
      // a second authority beside AppSnackBar's type → colour map. Every
      // toast goes through AppSnackBar.show{Success,Error,Info,Warning}; a
      // bare SnackBar may still exist but must not paint itself.
      final violations = <String>[];
      final files = Directory('lib')
          .listSync(recursive: true)
          .whereType<File>()
          .where((f) => f.path.endsWith('.dart'));
      for (final file in files) {
        final path = file.path.replaceAll(r'\', '/');
        if (path.endsWith('shared/widgets/app_snackbar.dart')) continue;
        final src = file.readAsStringSync();
        var from = 0;
        while (true) {
          final idx = src.indexOf('SnackBar(', from);
          if (idx < 0) break;
          // Only the bare Material widget counts: `ScaffoldMessenger…
          // showSnackBar(` and `AppSnackBar(` are not scoped here.
          final before = idx == 0 ? '' : src[idx - 1];
          final bare = !RegExp(r'[A-Za-z0-9_]').hasMatch(before);
          var depth = 0;
          var end = idx + 'SnackBar'.length;
          for (; end < src.length; end++) {
            final ch = src[end];
            if (ch == '(') depth++;
            if (ch == ')') {
              depth--;
              if (depth == 0) break;
            }
          }
          final block = src.substring(idx, end + 1);
          if (bare && block.contains('backgroundColor')) {
            final line = '\n'.allMatches(src.substring(0, idx)).length + 1;
            violations.add('$path:$line');
          }
          from = idx + 1;
        }
      }
      expect(
        violations,
        isEmpty,
        reason: 'SnackBar painted outside the AppSnackBar authority:\n'
            '${violations.join('\n')}',
      );
    });
  });
}
