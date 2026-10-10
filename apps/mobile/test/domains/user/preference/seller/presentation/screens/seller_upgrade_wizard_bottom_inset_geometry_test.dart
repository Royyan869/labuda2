// SAFE-AREA-15 — SELLER UPGRADE WIZARD: BOTTOM-INSET GEOMETRY.
//
// Locks the Owner invariant on the wizard's bottom-docked navigation surface:
//   * the navigation surface (`SellerWizardNavigationButtons` →
//     `BottomActionBar`) is the bottom layer of the body Column, so it owns
//     the system bottom inset — EXACTLY ONCE, through the bar's own
//     `SafeArea(top: false)` (never a body-side second reservation);
//   * it consumes the REAL, LIVE inset: a system navigation bar appearing,
//     changing or disappearing moves the buttons with it (0 → 24 → 34 → 48);
//   * inset 0 leaves NO artificial fixed clearance — the buttons end exactly
//     at the bar's own design padding (`AppMetrics.p12`);
//   * the keyboard stays the Scaffold's business (`resizeToAvoidBottomInset`
//     body resize): open → body lifts, the bar's self-lift reads 0 because
//     the body's viewInsets are consumed; closed → buttons return to the
//     system-inset geometry with no stale gap.
//
// RENDER-BASED on purpose: measurements come from the actual rendered wizard
// under injected window metrics — never a source string, never a bare
// `find.byType(SafeArea)` presence check.
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/config/seller_upgrade_config_entity.dart';
import 'package:hishumi/core/config/seller_upgrade_config_provider.dart'
    as upgrade_config;
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_upgrade_wizard_screen.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/widgets/wizard/seller_wizard_navigation_buttons.dart';
import 'package:hishumi/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'package:hishumi/domains/user/profile/domain/entities/address_entity.dart';
import 'package:hishumi/domains/user/profile/domain/entities/profile_entity.dart'
    show FarmInfo, ProfileEntity, ProfileStats, UserVerificationInfo;
import 'package:hishumi/domains/user/profile/domain/repositories/i_address_repository.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/profile_stream_provider.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/widgets/bottom_action_bar.dart';

/// Step-0 primary CTA of the wizard nav bar (registration mode).
const String _primaryLabel = 'Lanjut Lengkapi Data';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;

  @override
  Future<void> forceRefreshAuthState() async {}
}

class _FakeAddressRepository implements IAddressRepository {
  @override
  Future<Result<List<AddressEntity>>> getAddressesByUserId(
    String userId,
  ) async => Result.success(const <AddressEntity>[]);

  @override
  Future<Result<AddressEntity?>> getPrimaryAddress(String userId) async =>
      Result.success(null);

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

AuthUser _registrationUser() {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: 'never-seller',
    createdAt: now,
    updatedAt: now,
    email: 'never-seller@example.com',
    username: 'never-seller',
    bio: 'Bio registration',
    phoneNumber: '+62111111111',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: false,
    sellerSubscriptionStatus: 'expired',
    hasMarketAuthority: false,
    sellerTier: SellerTier.sellerElite,
    isIdVerified: false,
    isFarmVerified: false,
    lifecycle: ContentLifecycle.active,
  );
}

ProfileEntity _profileFor(AuthUser user) => ProfileEntity(
  id: 'profile-${user.id}',
  userId: user.id,
  joinedAt: DateTime.utc(2026, 1, 1),
  stats: const ProfileStats(followersCount: 0, followingCount: 0),
  verification: UserVerificationInfo.fromAuthUser(user),
  contactInfo: null,
  farmInfo: const FarmInfo(farmName: 'Test Farm'),
);

/// Window metrics for the TEST VIEW (physical pixels, like a device).
///
/// Platform semantics: `padding` is whatever `viewInsets` (the keyboard) has
/// NOT consumed of `viewPadding` — an open keyboard zeroes the bottom padding
/// exactly like Android does, so the SafeArea yields to it.
void _setWindowInsets(
  WidgetTester tester, {
  double systemBottom = 0,
  double keyboard = 0,
}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(
    bottom: math.max(0.0, systemBottom - keyboard) * dpr,
  );
  tester.view.viewPadding = FakeViewPadding(bottom: systemBottom * dpr);
  tester.view.viewInsets = FakeViewPadding(bottom: keyboard * dpr);
}

Widget _wrap() {
  final AuthUser user = _registrationUser();
  final ProfileEntity profile = _profileFor(user);
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(AuthState.authenticated(user, emailVerified: true)),
      ),
      addressRepositoryProvider.overrideWithValue(_FakeAddressRepository()),
      upgrade_config.sellerUpgradeConfigProvider.overrideWith(
        (ref) async => const SellerUpgradeConfigEntity(
          yearlyFee: 250000,
          durationDays: 365,
          isEnabled: true,
          renewalReminderDays: 30,
        ),
      ),
      profileStreamProvider(
        user.id,
      ).overrideWith((ref) => Stream.value(profile)),
    ],
    child: MaterialApp.router(
      theme: AppTheme.lightTheme,
      routerConfig: GoRouter(
        initialLocation: RoutePaths.sellerUpgrade,
        routes: [
          GoRoute(
            path: RoutePaths.sellerUpgrade,
            builder: (context, state) => const SellerUpgradeWizardScreen(),
          ),
        ],
      ),
    ),
  );
}

Future<void> _pumpWizard(WidgetTester tester) async {
  await tester.pumpWidget(_wrap());
  await tester.pumpAndSettle();
}

/// Bottom of the Scaffold surface (600 in the default test view).
double _surfaceBottom(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

/// Bottom edge of the nav surface itself — must stay anchored to the body
/// bottom (it IS the bottom surface of the body Column).
double _barBottom(WidgetTester tester) =>
    tester.getRect(find.byType(BottomActionBar)).bottom;

/// Bottom edge of the primary CTA inside the nav bar.
double _primaryBottom(WidgetTester tester) =>
    tester.getBottomRight(find.widgetWithText(ElevatedButton, _primaryLabel)).dy;

void main() {
  testWidgets(
    'system inset = 0: the nav CTA ends at the bar design padding, '
    'with no artificial fixed clearance',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);
      await _pumpWizard(tester);

      expect(find.byType(SellerUpgradeWizardScreen), findsOneWidget);
      expect(find.byType(SellerWizardNavigationButtons), findsOneWidget);
      expect(find.byType(BottomActionBar), findsOneWidget);

      final double surface = _surfaceBottom(tester);
      // The bar is the bottom surface: it reaches the body bottom edge.
      expect(_barBottom(tester), closeTo(surface, 0.01));
      // No inset → the ONLY space below the CTA is the bar's own p12 design
      // padding. A leftover inset-sized gap (or a fixed 80/96/100 stand-in)
      // would break this.
      expect(
        _primaryBottom(tester),
        closeTo(surface - AppMetrics.p12, 0.01),
        reason:
            'nav CTA carried an artificial bottom clearance at inset 0 '
            '(bottom ${_primaryBottom(tester)} != ${surface - AppMetrics.p12})',
      );
    },
  );

  testWidgets(
    'system inset = 24 / 34 / 48: the nav CTA follows the LIVE inset exactly',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 24);
      await _pumpWizard(tester);

      final Map<double, double> bottoms = <double, double>{};
      for (final double inset in const <double>[24, 34, 48]) {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpAndSettle();

        final double surface = _surfaceBottom(tester);
        final double ctaBottom = _primaryBottom(tester);
        bottoms[inset] = ctaBottom;

        expect(
          ctaBottom,
          closeTo(surface - inset - AppMetrics.p12, 0.01),
          reason:
              'the nav CTA did not consume the live system inset $inset '
              '(bottom $ctaBottom, expected '
              '${surface - inset - AppMetrics.p12})',
        );
        expect(
          ctaBottom,
          lessThanOrEqualTo(surface - inset),
          reason:
              'the nav CTA entered the system navigation region at inset '
              '$inset',
        );
        expect(
          _barBottom(tester),
          closeTo(surface, 0.01),
          reason: 'the nav surface must stay anchored to the body bottom',
        );
      }

      // A changed inset moves the CTA 1:1 — no stale gap, no stand-in.
      expect(
        bottoms[24]! - bottoms[48]!,
        closeTo(24, 0.01),
        reason: 'the nav CTA did not track a changed system inset 1:1',
      );
      expect(
        bottoms[34]! - bottoms[48]!,
        closeTo(14, 0.01),
        reason: 'the nav CTA did not track a changed system inset 1:1',
      );
    },
  );

  testWidgets(
    'keyboard open: the Scaffold lifts the body, the bar adds no duplicate '
    'keyboard clearance — and closing it leaves no stale gap',
    (tester) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 24, keyboard: 300);
      await _pumpWizard(tester);

      final double surface = _surfaceBottom(tester);

      // Keyboard handling stays the Scaffold's (`resizeToAvoidBottomInset`):
      // the body bottom is surface - viewInsets, and the bar's own self-lift
      // reads 0 inside the body (viewInsets consumed) — never a second
      // keyboard reservation stacked on the body resize.
      expect(
        _primaryBottom(tester),
        closeTo(surface - 300 - AppMetrics.p12, 0.01),
        reason:
            'nav CTA with keyboard open must be body bottom '
            '(${surface - 300}) minus the bar design padding p12 '
            '(got ${_primaryBottom(tester)} — a duplicate keyboard lift '
            'would push it higher)',
      );
      expect(
        _primaryBottom(tester),
        lessThanOrEqualTo(surface - 300),
        reason: 'the nav CTA was pushed under the keyboard',
      );
      expect(
        _barBottom(tester),
        closeTo(surface - 300, 0.01),
        reason: 'the nav surface must stay anchored to the resized body',
      );

      // Keyboard closes → the CTA returns to the live system-inset
      // position; a keyboard-sized gap must not survive.
      _setWindowInsets(tester, systemBottom: 24, keyboard: 0);
      await tester.pumpAndSettle();
      expect(
        _primaryBottom(tester),
        closeTo(surface - 24 - AppMetrics.p12, 0.01),
        reason: 'a stale keyboard-sized gap survived the closed keyboard',
      );
    },
  );
}
