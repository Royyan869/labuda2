import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/transaction/order/order.dart'
    as order_domain;
import 'package:hishumi/domains/commerce/transaction/order/presentation/screens/order_detail/order_action_handler.dart';
import 'package:hishumi/generated/app_localizations.dart';

/// Widget test host with the canonical localization wiring (I18N-07): any
/// label resolution goes through AppLocalizations, so every pump must provide
/// delegates. Explicit locale keeps label assertions deterministic.
Widget _localizedApp({required Widget home, Locale locale = const Locale('en')}) {
  return MaterialApp(
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    locale: locale,
    home: home,
  );
}

order_domain.Order _order() {
  return order_domain.Order(
    id: 'order-1',
    buyerId: 'buyer-1',
    sellerId: 'seller-1',
    items: const [],
    status: order_domain.OrderStatus.pending,
    paymentMethodCode: 'bank_transfer',
    paymentStatus: order_domain.PaymentStatus.pending,
    shippingInfo: const order_domain.ShippingInfo(
      recipientName: 'Buyer',
      phone: '08123456789',
      address: 'Some address',
      method: order_domain.ShippingMethod.bus,
      shippingCost: 10000,
    ),
    pricing: const order_domain.OrderPricing(
      subtotal: 100000,
      shippingCost: 10000,
      commissionAmount: 0,
      totalBeforeCoinsAmount: 110000,
      totalPayableAmount: 110000,
    ),
    createdAt: DateTime.utc(2026, 7, 1),
    source: order_domain.OrderSource.forSale,
  );
}

order_domain.Action _payAction() {
  return const order_domain.Action(
    type: 'pay',
    labelKey: 'action.pay_now',
    enabled: true,
    endpoint: '/api/v1/payments',
    method: 'POST',
    requiresIdempotency: true,
    financial: true,
  );
}

/// Canonical seller action after buyer payment: paid → shipped.
order_domain.Action _markShippedAction() {
  return const order_domain.Action(
    type: 'mark_shipped',
    labelKey: 'action.mark_shipped',
    enabled: true,
    endpoint: '/api/v1/orders/order-1/ship',
    method: 'POST',
    requiresIdempotency: true,
    financial: false,
  );
}

order_domain.Action _blockedAction() {
  return const order_domain.Action(
    type: 'example',
    labelKey: 'action.example',
    enabled: false,
    blocked: order_domain.ActionBlockedReason(
      action: 'example',
      messageKey: 'action.example.blocked',
      reason: 'Tidak tersedia saat ini',
      code: 'BLOCKED',
    ),
    endpoint: '/api/v1/orders/order-1/example',
    method: 'POST',
    requiresIdempotency: false,
    financial: false,
  );
}

order_domain.Action _unknownAction() {
  return const order_domain.Action(
    type: 'mystery',
    labelKey: 'action.mystery',
    enabled: true,
    endpoint: '/api/v1/orders/order-1/mystery',
    method: 'POST',
    requiresIdempotency: false,
    financial: false,
  );
}

order_domain.Action _completeAction() {
  return const order_domain.Action(
    type: 'complete',
    labelKey: 'action.complete',
    enabled: true,
    endpoint: '/api/v1/orders/order-1/complete',
    method: 'POST',
    requiresIdempotency: false,
    financial: true,
  );
}

order_domain.Action _extendConfirmationAction() {
  return const order_domain.Action(
    type: 'extend_confirmation',
    labelKey: 'action.extend_confirmation',
    enabled: true,
    endpoint: '/api/v1/orders/order-1/extend-confirmation',
    method: 'POST',
    requiresIdempotency: false,
    financial: false,
  );
}

order_domain.Action _contactSellerAction() {
  return const order_domain.Action(
    type: 'contact_seller',
    labelKey: 'action.chat_seller',
    enabled: true,
    endpoint: '/chat/direct/seller-1',
    method: 'POST',
    requiresIdempotency: false,
    financial: false,
  );
}

order_domain.Action _contactSupportAction() {
  return const order_domain.Action(
    type: 'contact_support',
    labelKey: 'action.contact_support',
    enabled: true,
    endpoint: '/support/tickets',
    method: 'POST',
    requiresIdempotency: false,
    financial: false,
  );
}

OrderActionHandler _handler(
  BuildContext context, {
  void Function(order_domain.Order)? onPayNow,
  void Function(String orderId, String sellerId)? onShipOrder,
  void Function(String orderId, String buyerId)? onConfirmDelivery,
  void Function(String orderId)? onExtendConfirmation,
  VoidCallback? onRequestSupport,
  VoidCallback? onChatSeller,
}) {
  return OrderActionHandler(
    order: _order(),
    context: context,
    onShipOrder: (orderId, sellerId, proofData) {
      onShipOrder?.call(orderId, sellerId);
    },
    onConfirmDelivery: onConfirmDelivery ?? (orderId, buyerId) {},
    onExtendConfirmation: onExtendConfirmation ?? (orderId) {},
    onRefundRequestRequest:
        ({
          required String orderId,
          required double orderSubtotal,
          required String buyerId,
          required String sellerId,
        }) {},
    onRate: (orderId, fromUserId, toUserId, rating, review) {},
    onPayNow: onPayNow ?? (_) {},
    onCancelOrder: (orderId, reason) {},
    onOpenDispute: ({required String orderId}) {},
    onRequestSupport: onRequestSupport ?? () {},
    onChatSeller: onChatSeller,
  );
}

Future<BuildContext> _pumpContext(WidgetTester tester) async {
  late BuildContext capturedContext;
  await tester.pumpWidget(
    _localizedApp(
      home: Builder(
        builder: (context) {
          capturedContext = context;
          return const SizedBox.shrink();
        },
      ),
    ),
  );
  return capturedContext;
}

/// Balanced-brace extractor so the negative gate is bounded to the two info
/// methods instead of the whole file (which still hosts confirmations/forms).
String? _methodBody(String source, String signature) {
  final start = source.indexOf(signature);
  if (start < 0) return null;
  final open = source.indexOf('{', start);
  if (open < 0) return null;
  var depth = 0;
  for (var i = open; i < source.length; i++) {
    final ch = source[i];
    if (ch == '{') depth++;
    if (ch == '}') {
      depth--;
      if (depth == 0) return source.substring(open, i + 1);
    }
  }
  return null;
}

void main() {
  testWidgets('pay action routes to onPayNow', (tester) async {
    late BuildContext capturedContext;
    var payNowCalled = false;

    await tester.pumpWidget(
      _localizedApp(
        home: Builder(
          builder: (context) {
            capturedContext = context;
            return const SizedBox.shrink();
          },
        ),
      ),
    );

    final handler = OrderActionHandler(
      order: _order(),
      context: capturedContext,
      onShipOrder: (orderId, sellerId, proofData) {},
      onConfirmDelivery: (orderId, buyerId) {},
      onExtendConfirmation: (orderId) {},
      onRefundRequestRequest:
          ({
            required String orderId,
            required double orderSubtotal,
            required String buyerId,
            required String sellerId,
          }) {},
      onRate: (orderId, fromUserId, toUserId, rating, review) {},
      onPayNow: (order) {
        payNowCalled = true;
      },
      onCancelOrder: (orderId, reason) {},
      onOpenDispute: ({required String orderId}) {},
      onRequestSupport: () {},
    );

    await handler.handleAction(_payAction());

    expect(payNowCalled, isTrue);
  });

  testWidgets(
    'canonical seller mark_shipped routes to onShipOrder',
    (tester) async {
      final context = await _pumpContext(tester);
      final shipped = <String>[];
      final handler = _handler(
        context,
        onShipOrder: (orderId, sellerId) => shipped.add('$orderId/$sellerId'),
      );

      final future = handler.handleAction(_markShippedAction());
      await tester.pumpAndSettle();
      // Dialog title + submit button share the label; assert the button.
      expect(find.text('Konfirmasi Pengiriman'), findsWidgets);
      expect(
        find.widgetWithText(ElevatedButton, 'Konfirmasi Pengiriman'),
        findsOneWidget,
      );

      await tester.enterText(find.byType(TextField).first, 'JNE123456789');
      await tester.tap(
        find.widgetWithText(ElevatedButton, 'Konfirmasi Pengiriman'),
      );
      await tester.pumpAndSettle();
      await future;

      expect(shipped, ['order-1/seller-1']);
    },
  );

  testWidgets('blocked action shows the info notice; OK closes it', (
    tester,
  ) async {
    late BuildContext capturedContext;
    await tester.pumpWidget(
      _localizedApp(
        home: Builder(
          builder: (context) {
            capturedContext = context;
            return const SizedBox.shrink();
          },
        ),
      ),
    );

    await _handler(capturedContext).handleAction(_blockedAction());
    await tester.pumpAndSettle();

    expect(find.text('Action Not Available'), findsOneWidget);
    expect(find.text('Tidak tersedia saat ini'), findsOneWidget);

    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(find.text('Action Not Available'), findsNothing);
  });

  testWidgets('unknown action shows the info notice; Close closes it', (
    tester,
  ) async {
    late BuildContext capturedContext;
    await tester.pumpWidget(
      _localizedApp(
        home: Builder(
          builder: (context) {
            capturedContext = context;
            return const SizedBox.shrink();
          },
        ),
      ),
    );

    await _handler(capturedContext).handleAction(_unknownAction());
    await tester.pumpAndSettle();

    expect(find.text('Action Info'), findsOneWidget);
    expect(find.text('Type: mystery'), findsOneWidget);

    // I18N-07: raw `action.*` keys never reach the user — the dialog shows
    // the canonical localized label (resolver default = generic CTA).
    expect(find.text('Label: action.mystery'), findsNothing);
    expect(find.text('Continue'), findsOneWidget);

    await tester.tap(find.text('Close'));
    await tester.pumpAndSettle();
    expect(find.text('Action Info'), findsNothing);
  });

  test('the info notices compose through AppDialog.info only', () {
    final source = File(
      'lib/domains/commerce/transaction/order/presentation/screens/'
      'order_detail/order_action_handler.dart',
    ).readAsStringSync();

    final blocked = _methodBody(source, 'void _showBlockedMessage(');
    final unknown = _methodBody(source, 'void _showUnknownActionInfo(');
    expect(blocked, isNotNull);
    expect(unknown, isNotNull);

    for (final body in <String>[blocked!, unknown!]) {
      expect(body.contains('AlertDialog('), isFalse);
      expect(body.contains('showDialog('), isFalse);
      expect(body.contains('AppDialog.info('), isTrue);
    }
    expect(blocked.contains("closeLabel: 'OK'"), isTrue);
    expect(unknown.contains("closeLabel: 'Close'"), isTrue);
  });

  testWidgets(
    'complete order: cancel is a no-op, confirm runs the existing completion',
    (tester) async {
      final context = await _pumpContext(tester);
      final completed = <String>[];
      final handler = _handler(
        context,
        onConfirmDelivery: (orderId, buyerId) =>
            completed.add('$orderId/$buyerId'),
      );

      final cancelFuture = handler.handleAction(_completeAction());
      await tester.pumpAndSettle();
      expect(find.text('Terima Barang'), findsOneWidget);
      expect(
        find.textContaining('pembayaran akan diteruskan ke penjual'),
        findsOneWidget,
      );
      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();
      await cancelFuture;
      expect(completed, isEmpty);

      final confirmFuture = handler.handleAction(_completeAction());
      await tester.pumpAndSettle();
      await tester.tap(find.text('Ya, Terima Barang'));
      await tester.pumpAndSettle();
      await confirmFuture;
      expect(completed, ['order-1/buyer-1']);
    },
  );

  testWidgets(
    'extend confirmation: cancel is a no-op, confirm runs the existing extension',
    (tester) async {
      final context = await _pumpContext(tester);
      final extended = <String>[];
      final handler = _handler(
        context,
        onExtendConfirmation: (orderId) => extended.add(orderId),
      );

      final cancelFuture = handler.handleAction(_extendConfirmationAction());
      await tester.pumpAndSettle();
      expect(find.text('Perpanjang Konfirmasi'), findsOneWidget);
      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();
      await cancelFuture;
      expect(extended, isEmpty);

      final confirmFuture = handler.handleAction(_extendConfirmationAction());
      await tester.pumpAndSettle();
      await tester.tap(find.text('Perpanjang'));
      await tester.pumpAndSettle();
      await confirmFuture;
      expect(extended, ['order-1']);
    },
  );

  testWidgets('contact_seller routes to the existing chat callback', (
    tester,
  ) async {
    final context = await _pumpContext(tester);
    var chatCalled = false;
    final handler = _handler(context, onChatSeller: () => chatCalled = true);

    await handler.handleAction(_contactSellerAction());
    await tester.pumpAndSettle();

    expect(chatCalled, isTrue);
    expect(find.text('Action Info'), findsNothing);
  });

  testWidgets('contact_support routes to the existing support callback', (
    tester,
  ) async {
    final context = await _pumpContext(tester);
    var supportCalled = false;
    final handler = _handler(
      context,
      onRequestSupport: () => supportCalled = true,
    );

    await handler.handleAction(_contactSupportAction());
    await tester.pumpAndSettle();

    expect(supportCalled, isTrue);
    expect(find.text('Action Info'), findsNothing);
  });

  testWidgets('blocked action without a human reason never shows a raw key', (
    tester,
  ) async {
    late BuildContext capturedContext;
    await tester.pumpWidget(
      _localizedApp(
        home: Builder(
          builder: (context) {
            capturedContext = context;
            return const SizedBox.shrink();
          },
        ),
      ),
    );

    const action = order_domain.Action(
      type: 'example',
      labelKey: 'action.example',
      enabled: false,
      blocked: order_domain.ActionBlockedReason(
        action: 'example',
        messageKey: 'action.example.blocked',
        code: 'BLOCKED',
      ),
      endpoint: '/api/v1/orders/order-1/example',
      method: 'POST',
      requiresIdempotency: false,
      financial: false,
    );

    await _handler(capturedContext).handleAction(action);
    await tester.pumpAndSettle();

    expect(find.text('action.example.blocked'), findsNothing);
    expect(
      find.text('An error occurred. Please try again.'),
      findsOneWidget,
    );
  });

  test('the confirmation family composes through AppDialog.confirm only', () {
    final source = File(
      'lib/domains/commerce/transaction/order/presentation/screens/'
      'order_detail/order_action_handler.dart',
    ).readAsStringSync();

    final complete = _methodBody(source, 'Future<void> _handleCompleteOrder(');
    final extend = _methodBody(
      source,
      'Future<void> _showExtendConfirmationDialog(',
    );
    expect(complete, isNotNull);
    expect(extend, isNotNull);

    for (final body in <String>[complete!, extend!]) {
      expect(body.contains('AlertDialog('), isFalse);
      expect(body.contains('showDialog('), isFalse);
      expect(body.contains('AppDialog.confirm('), isTrue);
    }
  });
}