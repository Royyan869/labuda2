// SELLER EARNINGS — WITHDRAWAL TIMESTAMP COMPOSITION GATE.
//
// Target: Seller Earnings withdrawal-history tile timestamp Row
// (`seller_earnings_screen.dart` `_buildWithdrawalTile`).
//
// Field: `withdrawal.createdAt` → relative age
// Authority: `TimeFormatService().formatTimeAgo` (canonical relative)
// Sibling: optional bank name via `Expanded` + maxLines1 + ellipsis
//
// This gate proves composition safety on the REAL SellerEarningsScreen
// across 320/360/412/500 × 1.0/1.3/2.0 with long Indonesian relative
// strings + long bank names. It does NOT change formatter or semantics.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/shared/domain/services/time_format_service.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/seller_earnings.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/withdrawal.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/providers/withdraw_notifier.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_earnings_screen.dart';
import 'package:hishumi/domains/user/preference/seller/seller_di.dart';
import 'package:hishumi/generated/app_localizations.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

const String _sellerId = 'seller-compose-1';
const String _longBank =
    'Bank Central Asia Cabang Utama KCP Sudirman Jakarta Pusat';
const String _accountNumber = '1234567890123456';

final DateTime _now = DateTime.now();
final DateTime _createdAt = _now.subtract(const Duration(days: 120));
final String _expectedRelative = const TimeFormatService().formatTimeAgo(
  _createdAt,
);
final String _longRelative = const TimeFormatService().formatTimeAgo(
  _now.subtract(const Duration(days: 1095)),
);

SellerEarnings _earnings() => SellerEarnings(
  sellerId: _sellerId,
  totalRevenue: 2500000,
  pendingRevenue: 300000,
  totalPlatformFees: 0,
  availableBalance: 125000,
  withdrawalFeeAmount: 5000,
  totalWithdrawn: 420000,
  totalWithdrawals: 1,
  totalCompletedOrders: 5,
  calculatedAt: _now,
  grossPayable: 130000,
);

Withdrawal _withdrawal({required DateTime createdAt}) => Withdrawal(
  id: 'w-1',
  sellerId: _sellerId,
  amount: 250000,
  feeAmount: 5000,
  status: WithdrawalStatus.completed,
  bankNameSnapshot: _longBank,
  accountNumberSnapshot: _accountNumber,
  createdAt: createdAt,
  updatedAt: createdAt,
);

class _StaticAuthController extends AuthController {
  _StaticAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

AuthUser _sellerUser() => AuthUser(
  id: _sellerId,
  createdAt: DateTime.utc(2026, 1, 1),
  updatedAt: DateTime.utc(2026, 1, 1),
  email: 'seller@example.com',
  username: 'seller_compose',
  isEmailVerified: true,
  accountStatus: AccountStatus.active,
  hasSellerProfile: true,
  sellerSubscriptionStatus: 'active',
  hasMarketAuthority: true,
  roles: const [],
  provider: AuthProvider.email,
);

Future<void> _pumpAt(
  WidgetTester tester,
  Size surface,
  double scale, {
  required DateTime createdAt,
}) async {
  await tester.binding.setSurfaceSize(surface);
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(
    ProviderScope(
      retry: (retryCount, error) => null,
      overrides: [
        authControllerProvider.overrideWith(
          () => _StaticAuthController(
            AuthState.authenticated(_sellerUser(), emailVerified: true),
          ),
        ),
        sellerEarningsProvider(_sellerId).overrideWith(
          (ref) async => _earnings(),
        ),
        withdrawalHistoryProvider.overrideWith(
          (ref) async => <Withdrawal>[_withdrawal(createdAt: createdAt)],
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.lightTheme,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: MediaQuery(
          data: MediaQueryData(
            size: surface,
            textScaler: TextScaler.linear(scale),
          ),
          child: const Scaffold(body: SellerEarningsScreen()),
        ),
      ),
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 50));
}

void main() {
  group('Seller Earnings withdrawal timestamp composition', () {
    test('formatter authority is TimeFormatService (canonical relative)', () {
      expect(_expectedRelative, '4 bulan lalu');
      expect(_longRelative, '3 tahun lalu');
      expect(
        const TimeFormatService().formatTimeAgo(_createdAt),
        _expectedRelative,
      );
      final String src = File(
        'lib/domains/user/preference/seller/presentation/screens/'
        'seller_earnings_screen.dart',
      ).readAsStringSync();
      expect(
        src.contains('TimeFormatService().formatTimeAgo'),
        isTrue,
      );
      expect(src.contains('DateFormat('), isFalse);
    });

    for (final DateTime createdAt in <DateTime>[_createdAt, _now.subtract(const Duration(days: 1095))]) {
      final String expected = const TimeFormatService().formatTimeAgo(
        createdAt,
      );
      for (final double width in _widths) {
        for (final double scale in _scales) {
          testWidgets(
            'withdrawal timestamp "$expected" safe at ${width}dp @scale $scale',
            (tester) async {
              await _pumpAt(
                tester,
                Size(width, 1200),
                scale,
                createdAt: createdAt,
              );

              expect(
                tester.takeException(),
                isNull,
                reason:
                    'seller earnings timestamp composition must not throw at '
                    '${width}dp @scale $scale for "$expected"',
              );

              // Canonical relative timestamp remains represented.
              expect(find.text(expected), findsWidgets);
              expect(find.text(_longBank), findsWidgets);

              // Timestamp and bank stay inside the surface.
              final Finder stamp = find.text(expected).first;
              final Rect stampRect = tester.getRect(stamp);
              expect(stampRect.left, greaterThanOrEqualTo(-0.5));
              expect(
                stampRect.right,
                lessThanOrEqualTo(width + 0.5),
                reason:
                    'timestamp wider than surface at ${width}dp @scale $scale',
              );

              final Finder bank = find.text(_longBank).first;
              final Rect bankRect = tester.getRect(bank);
              expect(bankRect.left, greaterThanOrEqualTo(-0.5));
              expect(
                bankRect.right,
                lessThanOrEqualTo(width + 0.5),
                reason:
                    'bank name wider than surface at ${width}dp @scale $scale',
              );
            },
          );
        }
      }
    }
  });
}
