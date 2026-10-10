// Contact-seller CTA is BUYER-ONLY (owner clarification).
//
//   buyer  + own order  → "Chat Penjual" / "Ingatkan Penjual" PRESENT
//   seller + own order  → "Chat Penjual" / "Ingatkan Penjual" ABSENT
//
// Behavior proof on the REAL OrderDetailScreen (same harness idiom as the
// SAFE-AREA-09A slot-authority test): the seller is the seller, so no
// contact-seller affordance may render anywhere on the seller surface. Support
// remains available to both parties.
import 'package:flutter/material.dart' hide Action;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/src/auth/app_role.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';
import 'package:hishumi/domains/commerce/transaction/order/domain/entities/order.dart'
    show Action;
import 'package:hishumi/domains/commerce/transaction/order/order.dart'
    hide Action;
import 'package:hishumi/domains/social/rating/rating.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/domains/user/identity/authentication/authentication.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';

const String _orderId = 'order-0001';
const String _buyerId = 'buyer-1';
const String _sellerId = 'seller-1';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._userId);

  final String _userId;

  @override
  AuthState build() {
    final now = DateTime.parse('2026-07-01T00:00:00.000Z');
    final user = AuthUser(
      id: _userId,
      createdAt: now,
      updatedAt: now,
      email: 'user@example.com',
      username: 'user1',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: false,
      sellerSubscriptionStatus: 'none',
      hasMarketAuthority: false,
      roles: const [UserRole.user],
      provider: AuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );
    return AuthState.authenticated(user, emailVerified: true);
  }
}

const _shippingInfo = ShippingInfo(
  recipientName: 'Buyer',
  phone: '08123456789',
  address: 'Some address',
  method: ShippingMethod.bus,
  shippingCost: 10000,
);

const _pricing = OrderPricing(
  subtotal: 100000,
  shippingCost: 10000,
  commissionAmount: 0,
  totalBeforeCoinsAmount: 110000,
  totalPayableAmount: 110000,
);

const _payDecision = DecisionContract(
  state: 'pending',
  primaryAction: Action(
    type: 'pay',
    labelKey: 'action.payment_continue',
    enabled: true,
    endpoint: '/api/v1/payments',
    method: 'POST',
    requiresIdempotency: true,
    financial: false,
  ),
);

const _shipDecision = DecisionContract(
  state: 'paid',
  primaryAction: Action(
    type: 'mark_shipped',
    labelKey: 'action.mark_shipped',
    enabled: true,
    endpoint: '/api/v1/orders/order-0001/ship',
    method: 'POST',
    requiresIdempotency: true,
    financial: false,
  ),
);

const _noActionDecision = DecisionContract(state: 'paid');

Order _order({
  required OrderStatus status,
  required PaymentStatus paymentStatus,
  required DecisionContract decision,
  String? overdueTier,
  bool? isOverdue,
  DateTime? readyToShipBy,
}) {
  return Order(
    id: _orderId,
    buyerId: _buyerId,
    sellerId: _sellerId,
    items: const [],
    status: status,
    paymentMethodCode: 'bank_transfer',
    paymentStatus: paymentStatus,
    shippingInfo: _shippingInfo,
    pricing: _pricing,
    createdAt: DateTime.utc(2026, 7, 1),
    source: OrderSource.forSale,
    decision: decision,
    overdueTier: overdueTier,
    isOverdue: isOverdue,
    readyToShipBy: readyToShipBy,
  );
}

Future<void> _pumpOrderDetail(
  WidgetTester tester, {
  required String authUserId,
  required Order order,
}) async {
  // Tall viewport so the ListView builds the body help/preparation sections
  // (they sit below the fold on the default 800x600 test surface).
  tester.view.physicalSize = const Size(1080, 4000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(
          () => _FakeAuthController(authUserId),
        ),
        watchOrderProvider(order.id).overrideWith((ref) => Stream.value(order)),
        refundsByOrderProvider(
          order.id,
        ).overrideWith((ref) => Stream.value(<RefundRequest>[])),
        hasUserRatedOrderProvider(
          orderId: _orderId,
          buyerId: _buyerId,
          sellerId: _sellerId,
        ).overrideWith((ref) async => false),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        theme: AppTheme.lightTheme,
        home: const OrderDetailScreen(orderId: _orderId),
      ),
    ),
  );
  await tester.pump();
  await tester.pump();
}

void main() {
  group('pending order — contact-seller is buyer-only', () {
    testWidgets('buyer sees "Chat Penjual" on their own pending order', (
      tester,
    ) async {
      await _pumpOrderDetail(
        tester,
        authUserId: _buyerId,
        order: _order(
          status: OrderStatus.pending,
          paymentStatus: PaymentStatus.pending,
          decision: _payDecision,
        ),
      );
      expect(find.text('Chat Penjual'), findsWidgets);
    });

    testWidgets('seller does NOT see "Chat Penjual" on their own order', (
      tester,
    ) async {
      await _pumpOrderDetail(
        tester,
        authUserId: _sellerId,
        order: _order(
          status: OrderStatus.pending,
          paymentStatus: PaymentStatus.pending,
          decision: _noActionDecision,
        ),
      );
      expect(find.text('Chat Penjual'), findsNothing);
    });
  });

  group('paid overdue order — preparation CTAs are buyer-only', () {
    Order paidOverdue(DecisionContract decision) => _order(
      status: OrderStatus.paid,
      paymentStatus: PaymentStatus.paid,
      decision: decision,
      overdueTier: 'severely_overdue',
      isOverdue: true,
      readyToShipBy: DateTime.utc(2026, 7, 2),
    );

    testWidgets('buyer sees contact-seller + support CTAs', (tester) async {
      await _pumpOrderDetail(
        tester,
        authUserId: _buyerId,
        order: paidOverdue(_noActionDecision),
      );
      expect(find.text('Chat Penjual'), findsWidgets);
      expect(find.text('Hubungi Support'), findsWidgets);
    });

    testWidgets('seller sees support but NO contact-seller CTA', (
      tester,
    ) async {
      await _pumpOrderDetail(
        tester,
        authUserId: _sellerId,
        order: paidOverdue(_shipDecision),
      );
      expect(find.text('Chat Penjual'), findsNothing);
      expect(find.text('Ingatkan Penjual'), findsNothing);
      expect(find.text('Hubungi Support'), findsWidgets);
    });
  });
}
