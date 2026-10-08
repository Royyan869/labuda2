import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_analytics_read.dart';
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_analytics_screen.dart';
import 'package:labuda/domains/user/preference/seller/seller_di.dart';

void main() {
  group('SellerAnalyticsScreen', () {
    SellerAnalytics _analytics() => const SellerAnalytics(
      summary: SellerAnalyticsSummary(
        totalViews30d: 120,
        productsWithViews30d: 3,
        productsSold30d: 1,
      ),
      products: [
        SellerAnalyticsProduct(
          productId: 'p1',
          title: 'Koi A',
          surfaceType: 'for_sale',
          state: 'sold',
          views30d: 100,
          sold: true,
          bidCount30d: 0,
        ),
        SellerAnalyticsProduct(
          productId: 'p2',
          title: 'Koi B',
          surfaceType: 'auction',
          state: 'active',
          views30d: 20,
          sold: false,
          bidCount30d: 7,
        ),
        SellerAnalyticsProduct(
          productId: 'p3',
          title: 'Koi C',
          surfaceType: 'auction',
          state: 'ended',
          views30d: 0,
          sold: true,
          bidCount30d: 0,
        ),
      ],
    );

    const sellerId = 'seller-001';

    AuthUser _sellerUser() => AuthUser(
      id: sellerId,
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: DateTime.utc(2026, 1, 1),
      email: 'seller@example.com',
      username: 'seller',
      isEmailVerified: true,
      roles: const [UserRole.user],
      provider: AuthProvider.email,
      hasSellerProfile: true,
      sellerSubscriptionStatus: 'active',
      hasMarketAuthority: true,
    );

    Widget _buildApp({required SellerAnalytics analytics}) {
      return ProviderScope(
        overrides: [
          authControllerProvider.overrideWith(
            () => _FakeSellerAuthController(
              AuthState.authenticated(_sellerUser(), emailVerified: true),
            ),
          ),
          sellerAnalyticsProvider(sellerId).overrideWith(
            (ref) async => analytics,
          ),
        ],
        child: const MaterialApp(home: SellerAnalyticsScreen()),
      );
    }

    testWidgets('renders summary and mixed For Sale + Auction rows', (
      tester,
    ) async {
      await tester.pumpWidget(_buildApp(analytics: _analytics()));
      await tester.pumpAndSettle();

      // Summary numbers.
      expect(find.text('120'), findsOneWidget);
      expect(find.text('3'), findsOneWidget);
      expect(find.text('1'), findsOneWidget);

      // Product titles (For Sale + Auction mixed).
      expect(find.text('Koi A'), findsOneWidget);
      expect(find.text('Koi B'), findsOneWidget);
      expect(find.text('Koi C'), findsOneWidget);

      // Sold / not-sold badges.
      expect(find.text('Terjual'), findsNWidgets(2)); // Koi A + Koi C
      expect(find.text('Belum terjual'), findsOneWidget); // Koi B

      // Bid count only on the auction row.
      expect(find.text('7 bid'), findsOneWidget);
      expect(find.text('100 views'), findsOneWidget);
    });

    testWidgets('renders zero-data empty state', (tester) async {
      await tester.pumpWidget(_buildApp(analytics: SellerAnalytics.empty));
      await tester.pumpAndSettle();

      expect(find.text('0'), findsNWidgets(3)); // three zero summary cards
      expect(find.text('Belum ada produk.'), findsOneWidget);
    });
  });
}

class _FakeSellerAuthController extends AuthController {
  _FakeSellerAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}
