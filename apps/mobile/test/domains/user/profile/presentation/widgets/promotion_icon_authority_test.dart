// PROMOTION ICON AUTHORITY — convergence proof.
//
// Canonical: Icons.campaign_outlined, established by the Promotion feature
// itself (canonical list empty state), the NotificationDisplayIcon.campaign
// mapping, the upgrade wizard, and the feed promo badge.
// This suite proves the Settings 'Promosi' entry uses it and that related
// (distinct) concepts were not touched.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/seller_state.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/settings_marketing_section.dart';

void main() {
  Future<void> pumpSection(WidgetTester tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sellerCapabilityStatusProvider.overrideWithValue(
            SellerCapabilityStatus.active,
          ),
        ],
        child: MaterialApp(
          theme: AppTheme.lightTheme,
          home: Scaffold(
            body: SettingsMarketingSection(onNavigate: (_) {}),
          ),
        ),
      ),
    );
    await tester.pump();
  }

  IconData leadingOf(WidgetTester tester, String title) {
    final Finder tile = find.widgetWithText(ListTile, title);
    expect(tile, findsOneWidget, reason: '"$title" tile missing');
    final ListTile widget = tester.widget<ListTile>(tile);
    final Icon icon = widget.leading! as Icon;
    return icon.icon!;
  }

  group('Settings promotion entry uses the canonical icon', () {
    testWidgets('Promosi tile leading icon is campaign_outlined', (
      tester,
    ) async {
      await pumpSection(tester);
      expect(leadingOf(tester, 'Promosi'), Icons.campaign_outlined);
    });

    testWidgets('section header + Diskon tile untouched', (tester) async {
      await pumpSection(tester);
      // Section header keeps the filled campaign mark.
      final Finder headerRow = find.ancestor(
        of: find.text('Marketing & Promotion'),
        matching: find.byType(Row),
      );
      expect(headerRow, findsOneWidget);
      // Diskon keeps its distinct discount icon (different concept).
      expect(leadingOf(tester, 'Diskon'), Icons.discount_outlined);
    });

    testWidgets('Promosi still navigates to the promotion route key', (
      tester,
    ) async {
      String? navigated;
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sellerCapabilityStatusProvider.overrideWithValue(
              SellerCapabilityStatus.active,
            ),
          ],
          child: MaterialApp(
            theme: AppTheme.lightTheme,
            home: Scaffold(
              body: SettingsMarketingSection(
                onNavigate: (String key) => navigated = key,
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('Promosi'));
      expect(navigated, 'promotion');
    });
  });

  group('Canonical authority + no local duplicate', () {
    test('promotion feature owns campaign_outlined for this concept', () {
      final String list = File(
        'lib/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_list_screen.dart',
      ).readAsStringSync();
      expect(list.contains('Icons.campaign_outlined'), isTrue);
      expect(list.contains('Belum ada promosi'), isTrue);
    });

    test('no local promotion icon authority in settings scope', () {
      final String section = File(
        'lib/domains/user/profile/presentation/widgets/settings_marketing_section.dart',
      ).readAsStringSync();
      expect(section.contains('local_offer'), isFalse);
      expect(section.contains('campaign_outlined'), isTrue);
      expect(section.contains('static const Icon'), isFalse);
    });
  });
}
