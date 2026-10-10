/// Seller Performance screen tests.
///
/// Proves the user-facing contract: canonical tier, rolling 90-day fulfillment
/// metrics, rating summary + 1-5 star distribution, and the loading / error /
/// zero-value states. No obsolete business metrics are exposed.
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/core/src/router/modules/seller_module.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/seller_performance.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_performance_screen.dart';
import 'package:hishumi/domains/user/preference/seller/seller_di.dart';

const _sellerId = 'seller-001';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

AuthUser _sellerUser() {
  final now = DateTime.utc(2026, 8, 1);
  return AuthUser(
    id: _sellerId,
    createdAt: now,
    updatedAt: now,
    email: 'seller@example.com',
    username: 'seller01',
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: true,
    sellerSubscriptionStatus: 'active',
    hasMarketAuthority: true,
  );
}

SellerPerformance _performance({
  String tier = 'pro',
  double fulfillmentRate = 0.967,
  int completedOrders = 100,
  int cancelledTimeout = 7,
  double averageRating = 4.8,
  int reviewCount = 210,
  int oneStar = 8,
  int twoStar = 12,
  int threeStar = 30,
  int fourStar = 45,
  int fiveStar = 152,
}) {
  return SellerPerformance(
    sellerId: _sellerId,
    tier: tier,
    fulfillmentRate: fulfillmentRate,
    completedOrders: completedOrders,
    cancelledTimeout: cancelledTimeout,
    averageRating: averageRating,
    reviewCount: reviewCount,
    oneStarCount: oneStar,
    twoStarCount: twoStar,
    threeStarCount: threeStar,
    fourStarCount: fourStar,
    fiveStarCount: fiveStar,
  );
}

Widget _buildApp({
  SellerPerformance? data,
  Object? error,
  bool loading = false,
  Future<SellerPerformance> Function()? override,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(
          AuthState.authenticated(_sellerUser(), emailVerified: true),
        ),
      ),
      sellerPerformanceProvider.overrideWith((ref, sellerId) {
        if (override != null) return override();
        if (error != null) return Future<SellerPerformance>.error(error);
        if (loading) return Completer<SellerPerformance>().future;
        return Future<SellerPerformance>.value(data);
      }),
    ],
    child: MaterialApp(
      theme: ThemeData(useMaterial3: true),
      home: const SellerPerformanceScreen(),
    ),
  );
}

void main() {
  group('Seller Performance screen', () {
    testWidgets('success state renders tier, fulfillment, rating and stars', (
      tester,
    ) async {
      await tester.pumpWidget(
        _buildApp(data: _performance()),
      );
      await tester.pumpAndSettle();

      // Header / overview.
      expect(find.text('Performa Penjual'), findsOneWidget);

      // Canonical tier (from the provider value).
      expect(find.text('Penjual Pro'), findsOneWidget);

      // Rolling 90-day fulfillment metrics.
      expect(find.text('Keandalan Pesanan'), findsOneWidget);
      expect(find.text('Data 90 hari terakhir'), findsOneWidget);
      expect(find.text('96.7%'), findsOneWidget);
      expect(find.text('100'), findsOneWidget); // completed orders
      expect(find.text('7'), findsOneWidget); // not shipped / timeout

      // Rating summary.
      expect(find.text('4.8'), findsOneWidget);
      expect(find.text('Dari 210 ulasan'), findsOneWidget);

      // 1-5 star distribution counts (chosen to be unique on screen).
      expect(find.text('152'), findsOneWidget);
      expect(find.text('45'), findsOneWidget);
      expect(find.text('30'), findsOneWidget);
      expect(find.text('12'), findsOneWidget);
      expect(find.text('8'), findsOneWidget);
    });

    testWidgets('renders canonical basic tier', (tester) async {
      await tester.pumpWidget(_buildApp(data: _performance(tier: 'basic')));
      await tester.pumpAndSettle();
      expect(find.text('Penjual Dasar'), findsOneWidget);
    });

    testWidgets('renders canonical elite tier', (tester) async {
      await tester.pumpWidget(_buildApp(data: _performance(tier: 'elite')));
      await tester.pumpAndSettle();
      expect(find.text('Penjual Elite'), findsOneWidget);
    });

    testWidgets('loading state shows a spinner, never fake zeros', (
      tester,
    ) async {
      await tester.pumpWidget(_buildApp(loading: true));
      await tester.pump();

      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      expect(find.text('96.7%'), findsNothing);
      expect(find.text('0.0%'), findsNothing);
    });

    testWidgets('error state shows retry and does not fake data', (
      tester,
    ) async {
      await tester.pumpWidget(_buildApp(error: Exception('boom')));
      await tester.pumpAndSettle();

      expect(find.text('Gagal memuat performa'), findsOneWidget);
      expect(find.widgetWithText(ElevatedButton, 'Coba Lagi'), findsOneWidget);
      expect(find.text('Penjual Pro'), findsNothing);
    });

    testWidgets('retry re-fetches and renders success', (tester) async {
      var shouldFail = true;
      await tester.pumpWidget(
        _buildApp(
          override: () {
            if (shouldFail) {
              return Future<SellerPerformance>.error(Exception('boom'));
            }
            return Future<SellerPerformance>.value(_performance());
          },
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Gagal memuat performa'), findsOneWidget);

      // Recover the backend, then tap retry — the screen must re-fetch and
      // render the success state.
      shouldFail = false;
      await tester.tap(find.widgetWithText(ElevatedButton, 'Coba Lagi'));
      await tester.pumpAndSettle();

      expect(find.text('Gagal memuat performa'), findsNothing);
      expect(find.text('Penjual Pro'), findsOneWidget);
    });

    testWidgets('zero values are valid data, not an error', (tester) async {
      await tester.pumpWidget(
        _buildApp(
          data: _performance(
            tier: 'basic',
            fulfillmentRate: 0.0,
            completedOrders: 0,
            cancelledTimeout: 0,
            averageRating: 0.0,
            reviewCount: 0,
            oneStar: 0,
            twoStar: 0,
            threeStar: 0,
            fourStar: 0,
            fiveStar: 0,
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Gagal memuat performa'), findsNothing);
      expect(find.text('0.0%'), findsOneWidget);
      expect(find.text('0'), findsWidgets); // completed + not shipped
      expect(find.text('Belum ada ulasan'), findsOneWidget);
      expect(find.text('Belum ada ulasan dari pembeli.'), findsOneWidget);
    });

    testWidgets('does not expose obsolete business metrics', (tester) async {
      await tester.pumpWidget(_buildApp(data: _performance()));
      await tester.pumpAndSettle();

      for (final forbidden in const [
        'Pendapatan',
        'Revenue',
        'AOV',
        'Rata-rata Nilai Pesanan',
        'Response',
        'Waktu Respons',
        'Return',
        'Repeat',
        'Tren',
        'Growth',
        'cancel_rate',
        'cancelled_timeout',
      ]) {
        expect(
          find.textContaining(forbidden),
          findsNothing,
          reason: 'obsolete/forbidden text "$forbidden" must not be rendered',
        );
      }
    });
  });

  group('Seller Performance route', () {
    test('canonical path is registered in SellerModule', () {
      expect(RoutePaths.sellerPerformance, '/seller/performance');
      final module = SellerModule();
      final paths = module.routes.map((r) => r.path).toList();
      expect(paths, contains(RoutePaths.sellerPerformance));
    });
  });
}
