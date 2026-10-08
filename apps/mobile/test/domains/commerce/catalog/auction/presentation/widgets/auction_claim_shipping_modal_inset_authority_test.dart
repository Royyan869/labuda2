// SAFE-AREA-07 — AUCTION CLAIM MODAL KEYBOARD INSET AUTHORITY.
//
// The modal is a full-screen MaterialPageRoute with its own Scaffold. After
// the cleanup, the keyboard/system responsibilities live ONLY with the
// canonical authorities:
//   * Scaffold  — resizes the body against the keyboard (default
//                 resizeToAvoidBottomInset);
//   * BottomActionBar — owns system + keyboard inset for the bar.
// The modal content itself must carry NO viewInsets-derived spacer: its last
// scroll child is pure design spacing, identical with and without a keyboard.
//
//   A — keyboard hidden (inset 0 / 24): design spacing only, boundaries hold;
//   B — keyboard 300 (inset 0 / 24): the scroll content does NOT grow by the
//       keyboard height (a resurrected spacer reads ≈336 px here, not ≤150);
//   C — keyboard visible: body above the keyboard, BottomActionBar still owns
//       the bottom edge, last actionable field reachable above the keyboard;
//   D — source proof: the viewInsets mechanism is gone from the file.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_claim_shipping_modal.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/widgets/bottom_action_bar.dart';

Auction _auction() => Auction(
  id: 'auction-1',
  sellerId: 'seller-1',
  sellerUsername: 'yayan',
  sellerFarmName: 'Farm Koi Nusantara',
  title: 'Showa Koi 30cm',
  description: 'Premium showa',
  koiDetails: const KoiDetails(
    variety: 'Kohaku',
    sizeInCm: 0,
    ageInMonths: 0,
    gender: 'unknown',
    certificates: [],
  ),
  openingBid: 100000,
  currentBid: 100000,
  bidIncrement: 10000,
  startTime: DateTime.parse('2026-01-01T00:00:00.000Z'),
  endTime: DateTime.parse('2026-01-02T00:00:00.000Z'),
  status: AuctionStatus.active,
  createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
);

Future<String?> _onClaim({
  required String addressId,
  String? shippingSetupId,
  String? shippingQuoteId,
  String? chatId,
  String? discountCode,
  bool useCoins = false,
}) async => null;

/// Window metrics on the TEST VIEW — only the real window sees every layer.
void _setWindowInsets(
  WidgetTester tester, {
  double systemBottom = 0,
  double keyboard = 0,
}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(bottom: systemBottom * dpr);
  tester.view.viewPadding = FakeViewPadding(bottom: systemBottom * dpr);
  tester.view.viewInsets = FakeViewPadding(bottom: keyboard * dpr);
}

Future<void> _pumpModal(WidgetTester tester) async {
  await tester.pumpWidget(
    ProviderScope(
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: AuctionClaimShippingModal(auction: _auction(), onClaim: _onClaim),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

Future<void> _scrollToBottom(WidgetTester tester) async {
  await tester.drag(find.byType(SingleChildScrollView), const Offset(0, -4000));
  await tester.pumpAndSettle();
}

void main() {
  final Finder scrollFinder = find.byType(SingleChildScrollView);
  final Finder fieldFinder = find.byType(TextField);
  final Finder barFinder = find.byType(BottomActionBar);

  group('SAFE-AREA-07 (A) — keyboard hidden: design spacing only', () {
    for (final double inset in const <double>[0, 24]) {
      testWidgets('inset $inset: bottom spacing stays design-sized', (
        tester,
      ) async {
        addTearDown(tester.view.reset);
        _setWindowInsets(tester, systemBottom: inset, keyboard: 0);
        await _pumpModal(tester);
        await _scrollToBottom(tester);

        expect(fieldFinder, findsOneWidget);

        final Rect viewport = tester.getRect(scrollFinder);
        final Rect field = tester.getRect(fieldFinder);
        final Rect bar = tester.getRect(barFinder);
        final Rect surface = tester.getRect(find.byType(Scaffold));

        // Design spacing: present, but never sized like a system/keyboard
        // reservation (the fixed content gaps below the field cap this well
        // under 250 px; a keyboard/inset reservation would be far larger).
        final double gap = viewport.bottom - field.bottom;
        expect(gap, greaterThanOrEqualTo(16));
        expect(gap, lessThan(250), reason: 'bottom gap $gap looks reserved');

        // Bar owns the bottom edge incl. the system inset; the body sits
        // above the bar.
        expect(bar.bottom, closeTo(surface.bottom, 0.01));
        expect(
          viewport.bottom,
          lessThanOrEqualTo(surface.bottom - inset + 0.01),
        );
      });
    }
  });

  group('SAFE-AREA-07 (B) — keyboard 300: no second keyboard reservation', () {
    for (final double inset in const <double>[0, 24]) {
      testWidgets('inset $inset: scroll content does not grow by 300', (
        tester,
      ) async {
        addTearDown(tester.view.reset);
        _setWindowInsets(tester, systemBottom: inset, keyboard: 300);
        await _pumpModal(tester);
        await _scrollToBottom(tester);

        expect(fieldFinder, findsOneWidget);

        final Rect viewport = tester.getRect(scrollFinder);
        final Rect field = tester.getRect(fieldFinder);
        final Rect surface = tester.getRect(find.byType(Scaffold));

        // The gap under the last field = design gaps + design spacer + scroll
        // padding only (≈52 px, +48 if the optional coin row renders). A
        // resurrected viewInsets spacer lands at ≈336 px here.
        final double gap = viewport.bottom - field.bottom;
        expect(
          gap,
          greaterThanOrEqualTo(32),
          reason: 'design spacing below the last field disappeared ($gap)',
        );
        expect(
          gap,
          lessThan(150),
          reason:
              'the scroll content carried a keyboard-sized band '
              '($gap px ≥ 300 keyboard) — a second keyboard reservation',
        );

        // Framework ownership: the body itself is above the keyboard.
        expect(
          viewport.bottom,
          lessThanOrEqualTo(surface.bottom - 300 + 0.01),
          reason: 'the Scaffold body did not resize against the keyboard',
        );
      });
    }
  });

  group('SAFE-AREA-07 (C) — keyboard visible: reachable + bar-owned edge', () {
    testWidgets('last field stays above the keyboard; bar owns the edge', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 24, keyboard: 300);
      await _pumpModal(tester);
      await _scrollToBottom(tester);

      final Rect viewport = tester.getRect(scrollFinder);
      final Rect field = tester.getRect(fieldFinder);
      final Rect bar = tester.getRect(barFinder);
      final Rect surface = tester.getRect(find.byType(Scaffold));

      // BottomActionBar remains the bottom system/keyboard authority.
      expect(bar.bottom, closeTo(surface.bottom, 0.01));
      expect(
        viewport.bottom,
        lessThanOrEqualTo(surface.bottom - 300 + 0.01),
        reason: 'the body overlaps the keyboard',
      );

      // The last actionable control is visible inside the viewport and
      // clears the keyboard — removing the spacer created no obstruction.
      expect(fieldFinder, findsOneWidget);
      expect(field.bottom, lessThanOrEqualTo(viewport.bottom));
      expect(field.top, greaterThanOrEqualTo(viewport.top - 0.01));
      expect(field.bottom, lessThanOrEqualTo(surface.bottom - 300 + 0.01));
    });
  });

  group('SAFE-AREA-07 (D) — source proof', () {
    test('the obsolete viewInsets spacer mechanism is gone', () {
      final String source = File(
        'lib/domains/commerce/catalog/auction/presentation/widgets/'
        'detail/auction_claim_shipping_modal.dart',
      ).readAsStringSync().replaceAll('\r\n', '\n');

      for (final String banned in const <String>[
        'viewInsets',
        'viewPadding',
        'bottomPadding',
        'MediaQuery.of',
      ]) {
        expect(
          source,
          isNot(contains(banned)),
          reason:
              '`$banned` re-introduced a modal-owned keyboard/system-inset '
              'calculation — the Scaffold and BottomActionBar own them',
        );
      }

      // The replacement is the constant design spacer.
      expect(source, contains('const SizedBox(height: AppMetrics.p16)'));
      expect(source, isNot(RegExp(r'bottomPadding\s*[>:]')));
    });
  });
}
