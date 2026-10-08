// SAFE-AREA-29 — FOLLOW LIST SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the single canonical bottom system-window authority on
// FollowListScreen (a STANDALONE pushed route — profile_module builds it
// directly, so no MainScreen shell bar can own the inset):
//
//   * system bottom inset → the body `SafeArea` wrapping the
//     `RefreshIndicator`/`CustomScrollView` (LIVE: the viewport bottom
//     tracks 0 / 24 / 34 / 48 as the window metrics change; a fixed
//     clearance could not);
//   * design spacing      → the list `SliverPadding(vertical: p8)` — a
//     constant measured BELOW the live inset, never a stand-in for it;
//   * top inset           → the primary AppBar (title + search strip);
//     the Scaffold strips body top padding when an AppBar exists, so
//     "AppBar bottom == viewport top" must hold at every inset;
//   * keyboard            → the screen's search TextField lives in the
//     AppBar bottom; the keyboard is the Scaffold's business
//     (`resizeToAvoidBottomInset` default) and is asserted as a single
//     body shrink with no SafeArea double-lift.
//
// Geometry is measured on the REAL screen (populated and empty states)
// with injected window metrics.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/follow/data/follow_providers.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';
import 'package:labuda/domains/social/follow/presentation/screens/follow_list_screen.dart';
import 'package:labuda/domains/social/follow/presentation/widgets/user_card.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/widgets/empty_state.dart';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => const AuthState.unauthenticated();
}

/// Static repository: the screen runs the REAL followers/following stream
/// providers against these lists, so the loaded state is the production
/// render path. Members outside the geometry path are forwarded loudly
/// via noSuchMethod (a call would fail the test instead of being masked).
class _StaticFollowRepository implements IFollowRepository {
  _StaticFollowRepository(this._followers);

  final List<FollowableUser> _followers;

  @override
  Stream<List<FollowableUser>> watchFollowers(String userId) =>
      Stream.value(_followers);

  @override
  Stream<List<FollowableUser>> watchFollowing(String userId) =>
      Stream.value(const <FollowableUser>[]);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

FollowableUser _user(int i) => FollowableUser(
  id: 'user-$i',
  username: 'user$i',
  avatar: null,
  userType: UserType.buyer,
  lifecycle: 'active',
  followersCount: i,
  followingCount: i,
);

/// Enough cards that the list always exceeds one viewport, so the
/// scroll-end contract is exercised for real.
List<FollowableUser> _users(int count) => [
  for (var i = 1; i <= count; i++) _user(i),
];

/// Injects window metrics on the TEST VIEW (same idiom as
/// SAFE-AREA-10/17/27/28) so every inset below is the REAL, LIVE one.
void _setInsets(
  WidgetTester tester, {
  required double bottom,
  double keyboard = 0,
}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(top: 24 * dpr, bottom: bottom * dpr);
  tester.view.viewPadding = FakeViewPadding(
    top: 24 * dpr,
    bottom: bottom * dpr,
  );
  tester.view.viewInsets = FakeViewPadding(bottom: keyboard * dpr);
}

Future<void> _pump(
  WidgetTester tester, {
  required double inset,
  double keyboard = 0,
  List<FollowableUser>? users,
}) async {
  addTearDown(tester.view.reset);
  _setInsets(tester, bottom: inset, keyboard: keyboard);

  await tester.pumpWidget(
    ProviderScope(
      // No retry: a static stream must never schedule timers.
      retry: (retryCount, error) => null,
      overrides: [
        authControllerProvider.overrideWith(_FakeAuthController.new),
        followRepositoryProvider.overrideWithValue(
          _StaticFollowRepository(users ?? _users(12)),
        ),
      ],
      child: MaterialApp(
        home: FollowListScreen(
          userId: 'owner-1',
          type: FollowListType.followers,
          username: 'Owner',
        ),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

ScrollableState _scrollableOf(WidgetTester tester) =>
    tester.state<ScrollableState>(
      find
          .descendant(
            of: find.byType(CustomScrollView),
            matching: find.byType(Scrollable),
          )
          .first,
    );

/// Scrolls to the REAL end: `maxScrollExtent` is an estimate until the
/// children at the new offset are laid out, so re-jump until stable.
Future<void> _scrollToEnd(WidgetTester tester) async {
  final ScrollableState scrollable = _scrollableOf(tester);
  for (var i = 0; i < 4; i++) {
    scrollable.position.jumpTo(scrollable.position.maxScrollExtent);
    await tester.pump();
  }
}

/// Measured geometry row at [inset], after scrolling to the very end.
/// Printed so every run documents the raw numbers behind the contract.
Future<Map<String, double>> _measure(
  WidgetTester tester, {
  required double inset,
}) async {
  final Rect surface = tester.getRect(find.byType(Scaffold));
  final Finder scrollFinder = find.byType(CustomScrollView);
  final Rect scrollRect = tester.getRect(scrollFinder);
  final Rect appBarRect = tester.getRect(find.byType(AppBar));

  final ScrollableState scrollable = _scrollableOf(tester);
  await _scrollToEnd(tester);

  final Rect lastCard = tester.getRect(find.byType(UserCard).last);

  final double viewport = scrollRect.height;
  final double maxScroll = scrollable.position.maxScrollExtent;
  final double contentExtent = viewport + maxScroll;
  final double regionStart = surface.bottom - inset;
  final double gap = regionStart - lastCard.bottom;
  final double reachable =
      (lastCard.top >= scrollRect.top - 0.01 &&
          lastCard.bottom <= scrollRect.bottom + 0.01)
      ? 1
      : 0;

  // ignore: avoid_print
  print(
    'GEOM inset=$inset '
    'pixels=${scrollable.position.pixels.toStringAsFixed(2)} '
    'viewport=${viewport.toStringAsFixed(2)} '
    'maxScroll=${maxScroll.toStringAsFixed(2)} '
    'contentExtent=${contentExtent.toStringAsFixed(2)} '
    'lastCardBottom=${lastCard.bottom.toStringAsFixed(2)} '
    'scrollBottom=${scrollRect.bottom.toStringAsFixed(2)} '
    'surfaceBottom=${surface.bottom.toStringAsFixed(2)} '
    'systemRegionStart=${regionStart.toStringAsFixed(2)} '
    'gap=$gap reachable=$reachable '
    'appBarBottom=${appBarRect.bottom.toStringAsFixed(2)} '
    'scrollTop=${scrollRect.top.toStringAsFixed(2)}',
  );

  return <String, double>{
    'viewport': viewport,
    'maxScroll': maxScroll,
    'contentExtent': contentExtent,
    'lastCardBottom': lastCard.bottom,
    'scrollBottom': scrollRect.bottom,
    'surfaceBottom': surface.bottom,
    'regionStart': regionStart,
    'gap': gap,
    'reachable': reachable,
    'appBarBottom': appBarRect.bottom,
    'scrollTop': scrollRect.top,
  };
}

/// Populated-state contract at [inset]:
/// 1. the viewport follows the LIVE inset (body SafeArea);
/// 2. the AppBar hands over the body with no phantom top space;
/// 3. the scroll path is real (maxScrollExtent > 0);
/// 4. at scroll-end the last card sits exactly the design `p8` ABOVE the
///    system region — never inside it, and the 8 px never grows into a
///    fixed inset stand-in;
/// 5. the last card is fully reachable inside the viewport.
Future<void> _expectLoadedGeometry(
  WidgetTester tester, {
  required double inset,
}) async {
  final Map<String, double> m = await _measure(tester, inset: inset);

  expect(
    m['scrollBottom'],
    closeTo(m['surfaceBottom']! - inset, 0.01),
    reason:
        'the scroll viewport bottom must follow the live system inset '
        '($inset) — the body SafeArea, not a fixed constant, owns the '
        'bottom inset',
  );

  expect(
    m['scrollTop'],
    closeTo(m['appBarBottom']!, 0.01),
    reason:
        'the body must start exactly at the AppBar bottom (title + search '
        'strip) — the body SafeArea must not add phantom top space '
        '(top ${m['scrollTop']} vs appBar bottom ${m['appBarBottom']})',
  );

  expect(
    m['maxScroll'],
    greaterThan(0),
    reason: 'the list must actually scroll for the end contract to hold',
  );

  expect(
    m['gap'],
    closeTo(8, 0.01),
    reason:
        'at scroll-end the gap to the system region must be exactly the '
        'design SliverPadding (vertical AppMetrics.p8 = 8) at inset '
        '$inset: negative would mean overlap (${m['gap']}), larger would '
        'mean a fixed clearance standing in for the live inset',
  );

  expect(
    m['lastCardBottom']!,
    lessThanOrEqualTo(m['regionStart']! + 0.01),
    reason:
        'the last card (${m['lastCardBottom']}) must stay outside the system '
        'region (start ${m['regionStart']}) at inset $inset',
  );

  expect(
    m['reachable'],
    1,
    reason:
        'the last card must be fully visible inside the viewport at '
        'scroll-end at inset $inset',
  );
}

void main() {
  group('SAFE-AREA-29 — populated list: live inset geometry', () {
    testWidgets('inset 0 — design spacing only, no phantom reservation', (
      tester,
    ) async {
      await _pump(tester, inset: 0);
      expect(find.byType(UserCard), findsWidgets);
      await _expectLoadedGeometry(tester, inset: 0);
    });

    testWidgets('inset 24 — last card clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 24);
      await _expectLoadedGeometry(tester, inset: 24);
    });

    testWidgets('inset 34 — last card clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 34);
      await _expectLoadedGeometry(tester, inset: 34);
    });

    testWidgets('inset 48 — last card clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 48);
      await _expectLoadedGeometry(tester, inset: 48);
    });
  });

  group('SAFE-AREA-29 — authority proof', () {
    testWidgets(
      'the viewport follows the system inset 1:1 while the content extent '
      'stays constant (no double inset, no dead space)',
      (tester) async {
        await _pump(tester, inset: 0);
        final Map<String, double> at0 = await _measure(tester, inset: 0);

        // System bar appears (48 px): only a live authority moves the
        // viewport's bottom boundary.
        _setInsets(tester, bottom: 48);
        await tester.pump();
        await tester.pump();
        final Map<String, double> at48 = await _measure(tester, inset: 48);

        expect(
          at0['scrollBottom']! - at48['scrollBottom']!,
          closeTo(48, 0.01),
          reason:
              'the scroll viewport bottom must follow the system inset — a '
              'fixed clearance would not move',
        );

        // The viewport shrinks exactly by the inset; the content extent is
        // invariant: no nested scrollable absorbs the inset a second time
        // (that would grow the content extent by the same 48 px).
        expect(
          at48['viewport']!,
          closeTo(at0['viewport']! - 48, 0.01),
          reason: 'the viewport must shrink exactly by the live inset',
        );
        expect(
          at48['contentExtent']!,
          closeTo(at0['contentExtent']!, 0.01),
          reason:
              'the content extent must stay constant across insets — growth '
              'would prove a second, duplicate inset absorption',
        );

        // Exactly ONE canonical SafeArea authority owns the body scroll
        // geometry: count SafeArea ANCESTORS of the scrollable. (The
        // MaterialAppBar's internal SafeArea is `bottom: false` in a
        // sibling slot — it can never own the body's bottom inset.)
        int safeAreaAncestors = 0;
        tester.element(find.byType(CustomScrollView)).visitAncestorElements((
          element,
        ) {
          if (element.widget is SafeArea) safeAreaAncestors++;
          return true;
        });
        expect(
          safeAreaAncestors,
          1,
          reason:
              'exactly one SafeArea must sit between the scrollable and the '
              'root — no duplicate ownership, none missing',
        );

        // Static source check: the screen owns exactly one SafeArea.
        final String src = File(
          'lib/domains/social/follow/presentation/screens/follow_list_screen.dart',
        ).readAsStringSync();
        expect(
          'SafeArea('.allMatches(src).length,
          1,
          reason:
              'the screen must own exactly one SafeArea authority — no '
              'duplicate ownership',
        );
      },
    );

    testWidgets(
      'keyboard (search TextField in the AppBar strip) is the Scaffold\'s '
      'business: one body shrink, no SafeArea double-lift',
      (tester) async {
        await _pump(tester, inset: 0, keyboard: 300);

        final Rect surface = tester.getRect(find.byType(Scaffold));
        final Rect scrollRect = tester.getRect(find.byType(CustomScrollView));

        expect(
          scrollRect.bottom,
          closeTo(surface.bottom - 300, 0.01),
          reason:
              'the body must shrink exactly once by the keyboard '
              'viewInsets (Scaffold resize) — a SafeArea double-lift would '
              'push the viewport further up than surface.bottom - 300',
        );
      },
    );

    testWidgets('empty state shares the same viewport authority', (
      tester,
    ) async {
      await _pump(tester, inset: 34, users: const <FollowableUser>[]);
      expect(find.byType(EmptyState), findsOneWidget);

      final Rect surface = tester.getRect(find.byType(Scaffold));
      final Rect scrollRect = tester.getRect(find.byType(CustomScrollView));
      expect(
        scrollRect.bottom,
        closeTo(surface.bottom - 34, 0.01),
        reason:
            'the empty state (SliverFillRemaining) must end at the same '
            'SafeArea-owned boundary at inset 34',
      );
    });
  });
}
