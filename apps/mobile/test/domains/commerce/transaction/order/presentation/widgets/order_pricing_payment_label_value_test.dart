// F2 — ORDER PRICING / PAYMENT LABEL-VALUE COMPOSITION PROOF.
//
// Owner decisions (locked):
//   * monetary values are NEVER ellipsized and must remain fully visible;
//   * horizontal when label + value fit, vertical (label over value) fallback;
//   * no hardcoded device breakpoint, no MediaQuery width arithmetic,
//     no IntrinsicWidth, no new generic abstraction.
//
// Subjects are the real consumers (OrderBuyerPricingCard,
// OrderSellerPricingCard, OrderPaymentInfoCard) pumped across a
// WIDTH x TEXT-SCALE matrix with realistic Indonesian fixtures.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/transaction/order/domain/domain.dart';
import 'package:hishumi/domains/commerce/transaction/order/presentation/widgets/order_widgets.dart';
import 'package:hishumi/shared/utils/app_formatters.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const ShippingInfo _shippingInfo = ShippingInfo(
  recipientName: 'Pembeli Nusantara',
  phone: '081234567890',
  address: 'Jl. Kolam Koi No. 48, Kecamatan Blimbing, Kota Malang',
  method: ShippingMethod.bus,
  shippingCost: 25000,
);

Order _buyerOrder({bool withServiceFee = true, double? totalOverride}) {
  return Order(
    id: 'order-buyer-1',
    buyerId: 'buyer-1',
    sellerId: 'seller-1',
    items: const [],
    status: OrderStatus.pending,
    paymentMethodCode: 'bank_transfer',
    paymentStatus: PaymentStatus.paid,
    shippingInfo: _shippingInfo,
    pricing: OrderPricing(
      subtotal: 1250000,
      shippingCost: 25000,
      commissionAmount: 0,
      serviceFeeAmount: withServiceFee ? 5000 : null,
      totalPayableAmount: totalOverride ?? 1280000,
      totalBeforeCoinsAmount: 1275000,
    ),
    createdAt: DateTime(2026, 5, 16, 10, 0),
    paidAt: DateTime(2026, 5, 17, 14, 30),
    source: OrderSource.forSale,
  );
}

Order _sellerOrder() {
  return Order(
    id: 'order-seller-1',
    buyerId: 'buyer-1',
    sellerId: 'seller-1',
    items: const [],
    status: OrderStatus.pending,
    paymentMethodCode: 'bank_transfer',
    paymentStatus: PaymentStatus.paid,
    shippingInfo: _shippingInfo,
    pricing: const OrderPricing(
      subtotal: 1250000,
      shippingCost: 25000,
      commissionAmount: 62500,
      serviceFeeAmount: 5000,
      totalPayableAmount: 1280000,
      totalBeforeCoinsAmount: 1275000,
    ),
    createdAt: DateTime(2026, 5, 16, 10, 0),
    paidAt: DateTime(2026, 5, 17, 14, 30),
    source: OrderSource.forSale,
  );
}

// ---------------------------------------------------------------------------
// Harness (mirrors horizontal_composition_contract_test)
// ---------------------------------------------------------------------------

Widget _harness({
  required Size surface,
  required double scale,
  required Widget subject,
}) {
  return MaterialApp(
    theme: AppTheme.lightTheme,
    home: MediaQuery(
      data: MediaQueryData(size: surface, textScaler: TextScaler.linear(scale)),
      child: Scaffold(
        body: Center(
          child: SizedBox(
            width: surface.width,
            child: SingleChildScrollView(child: subject),
          ),
        ),
      ),
    ),
  );
}

Future<void> _pumpCard(
  WidgetTester tester, {
  required double width,
  required double scale,
  required Widget subject,
}) async {
  final Size surface = Size(width, 800);
  await tester.binding.setSurfaceSize(surface);
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(
    _harness(surface: surface, scale: scale, subject: subject),
  );
  await tester.pump();
}

/// No layout exception + card stays within the surface horizontally.
void _expectNoHorizontalOverflow(
  WidgetTester tester, {
  required Type cardType,
  required double width,
  required double scale,
}) {
  final Object? exception = tester.takeException();
  expect(
    exception,
    isNull,
    reason: 'layout overflow: $cardType at ${width}dp @scale $scale '
        '($exception)',
  );
  final Finder finder = find.byType(cardType);
  expect(finder, findsOneWidget, reason: '$cardType did not render');
  final Rect rect = tester.getRect(finder);
  expect(
    rect.right,
    lessThanOrEqualTo(width + 0.5),
    reason: '$cardType wider than surface at ${width}dp @scale $scale',
  );
  expect(rect.left, greaterThanOrEqualTo(-0.5));
}

/// Every rendered monetary value must be complete: no ellipsis, no line cap.
void _expectMoneyIntact(WidgetTester tester, List<String> moneyValues) {
  final List<Text> texts = tester
      .widgetList<Text>(find.byType(Text))
      .toList();
  for (final String money in moneyValues) {
    expect(find.text(money), findsOneWidget, reason: '$money not rendered');
    final Text widget = texts.firstWhere(
      (Text t) => t.data == money,
      orElse: () => throw TestFailure('Text widget for "$money" not found'),
    );
    expect(
      widget.overflow,
      isNot(TextOverflow.ellipsis),
      reason: 'monetary value "$money" must NEVER be ellipsized',
    );
    expect(
      widget.maxLines,
      isNull,
      reason: 'monetary value "$money" must not carry a line cap',
    );
  }
}

/// Label and value share one visual line (tops aligned).
void _expectHorizontal(WidgetTester tester, String label, String value) {
  final Rect labelRect = tester.getRect(find.text(label));
  final Rect valueRect = tester.getRect(find.text(value));
  expect(
    (labelRect.top - valueRect.top).abs(),
    lessThanOrEqualTo(2.0),
    reason: '"$label" / "$value" should share one line (label $labelRect, '
        'value $valueRect)',
  );
}

/// Value sits below the label (vertical recomposition).
void _expectVertical(WidgetTester tester, String label, String value) {
  final Rect labelRect = tester.getRect(find.text(label));
  final Rect valueRect = tester.getRect(find.text(value));
  expect(
    valueRect.top,
    greaterThanOrEqualTo(labelRect.bottom - 0.5),
    reason: '"$value" should recompose below "$label" (label $labelRect, '
        'value $valueRect)',
  );
}

void main() {
  setUpAll(() async {
    // AppFormatters.formatDateTime uses DateFormat(..., 'id_ID'), whose
    // symbols are not loaded in the test isolate by default.
    await initializeDateFormatting('id_ID');
  });

  group('F2 buyer pricing matrix: no overflow, money complete', () {
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('buyer card at ${width}dp @scale $scale', (tester) async {
          final Order order = _buyerOrder();
          await _pumpCard(
            tester,
            width: width,
            scale: scale,
            subject: OrderBuyerPricingCard(order: order),
          );
          _expectNoHorizontalOverflow(
            tester,
            cardType: OrderBuyerPricingCard,
            width: width,
            scale: scale,
          );
          _expectMoneyIntact(tester, <String>[
            AppFormatters.formatCurrency(1250000),
            AppFormatters.formatCurrency(25000),
            AppFormatters.formatCurrency(5000),
            AppFormatters.formatCurrency(1280000),
          ]);
          // Existing semantic labels unchanged.
          expect(find.text('Rincian Pembayaran'), findsOneWidget);
          expect(find.text('Harga Produk'), findsOneWidget);
          expect(find.text('Ongkir + Packing'), findsOneWidget);
          expect(find.text('Biaya Layanan Pembayaran'), findsOneWidget);
          expect(find.text('Total Pembayaran'), findsOneWidget);
        });
      }
    }
  });

  group('F2 seller pricing matrix: no overflow, money complete', () {
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('seller card at ${width}dp @scale $scale', (tester) async {
          await _pumpCard(
            tester,
            width: width,
            scale: scale,
            subject: OrderSellerPricingCard(order: _sellerOrder()),
          );
          _expectNoHorizontalOverflow(
            tester,
            cardType: OrderSellerPricingCard,
            width: width,
            scale: scale,
          );
          _expectMoneyIntact(tester, <String>[
            AppFormatters.formatCurrency(1250000),
            AppFormatters.formatCurrency(25000),
          ]);
          expect(find.text('Rincian Pendapatan'), findsOneWidget);
          expect(find.text('Pendapatan Bersih'), findsOneWidget);
          // Finance-boundary pointer (not money): fully visible, never cut.
          expect(find.text('Lihat di Dashboard Penjual'), findsOneWidget);
        });
      }
    }
  });

  group('F2 payment info matrix: no overflow, money complete', () {
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('payment card at ${width}dp @scale $scale', (tester) async {
          final Order order = _buyerOrder();
          final String money = AppFormatters.formatCurrency(1280000);
          final String date = AppFormatters.formatDateTime(
            DateTime(2026, 5, 17, 14, 30),
          );
          await _pumpCard(
            tester,
            width: width,
            scale: scale,
            subject: OrderPaymentInfoCard(order: order),
          );
          // F5 converged the card header, so the card is clean end to end.
          _expectNoHorizontalOverflow(
            tester,
            cardType: OrderPaymentInfoCard,
            width: width,
            scale: scale,
          );
          _expectMoneyIntact(tester, <String>[money]);
          expect(find.text('Info Pembayaran'), findsOneWidget);
          expect(find.text('Metode'), findsOneWidget);
          expect(find.text('Transfer Bank'), findsOneWidget);
          expect(find.text('Total'), findsOneWidget);
          expect(find.text('Tanggal Bayar'), findsOneWidget);
          expect(find.text(date), findsOneWidget);
        });
      }
    }
  });

  group('F2 pending-fee + long money fixtures', () {
    testWidgets(
      '"Akan dihitung server" stays complete at 320dp @scale 2.0',
      (tester) async {
        await _pumpCard(
          tester,
          width: 320,
          scale: 2.0,
          subject: OrderBuyerPricingCard(
            order: _buyerOrder(withServiceFee: false),
          ),
        );
        _expectNoHorizontalOverflow(
          tester,
          cardType: OrderBuyerPricingCard,
          width: 320,
          scale: 2.0,
        );
        expect(find.text('Akan dihitung server'), findsOneWidget);
        _expectMoneyIntact(tester, <String>[
          AppFormatters.formatCurrency(1250000),
          AppFormatters.formatCurrency(25000),
          AppFormatters.formatCurrency(1280000),
        ]);
      },
    );

    testWidgets(
      'long monetary value Rp 125.000.000 stays complete at 320dp @scale 2.0',
      (tester) async {
        await _pumpCard(
          tester,
          width: 320,
          scale: 2.0,
          subject: OrderBuyerPricingCard(
            order: _buyerOrder(totalOverride: 125000000),
          ),
        );
        _expectNoHorizontalOverflow(
          tester,
          cardType: OrderBuyerPricingCard,
          width: 320,
          scale: 2.0,
        );
        expect(
          find.text(AppFormatters.formatCurrency(125000000)),
          findsOneWidget,
        );
        _expectMoneyIntact(tester, <String>[
          AppFormatters.formatCurrency(125000000),
        ]);
      },
    );
  });

  group('F2 composition direction', () {
    testWidgets('horizontal preserved when content fits (500dp @1.0)', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        width: 500,
        scale: 1.0,
        subject: OrderBuyerPricingCard(order: _buyerOrder()),
      );
      _expectNoHorizontalOverflow(
        tester,
        cardType: OrderBuyerPricingCard,
        width: 500,
        scale: 1.0,
      );
      _expectHorizontal(
        tester,
        'Harga Produk',
        AppFormatters.formatCurrency(1250000),
      );
    });

    testWidgets('vertical recomposition when constrained (320dp @2.0)', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        width: 320,
        scale: 2.0,
        subject: OrderBuyerPricingCard(
          order: _buyerOrder(withServiceFee: false),
        ),
      );
      _expectNoHorizontalOverflow(
        tester,
        cardType: OrderBuyerPricingCard,
        width: 320,
        scale: 2.0,
      );
      // Long label + value cannot share one line at this width/scale.
      _expectVertical(tester, 'Biaya Layanan Pembayaran', 'Akan dihitung server');
    });

    testWidgets('payment Total recomposes vertically when constrained', (
      tester,
    ) async {
      final String money = AppFormatters.formatCurrency(125000000);
      await _pumpCard(
        tester,
        width: 320,
        scale: 2.0,
        subject: OrderPaymentInfoCard(
          order: _buyerOrder(totalOverride: 125000000),
        ),
      );
      _expectNoHorizontalOverflow(
        tester,
        cardType: OrderPaymentInfoCard,
        width: 320,
        scale: 2.0,
      );
      _expectVertical(tester, 'Total', money);
    });

    testWidgets(
      'payment header stays contained (F5 fix; was KNOWN RESIDUE)',
      (tester) async {
        await _pumpCard(
          tester,
          width: 320,
          scale: 2.0,
          subject: OrderPaymentInfoCard(order: _buyerOrder()),
        );
        // F5 converged the header (Expanded title + intact badge), so the
        // card is now clean end to end — including the previously
        // overflowing title/badge row.
        _expectNoHorizontalOverflow(
          tester,
          cardType: OrderPaymentInfoCard,
          width: 320,
          scale: 2.0,
        );
        final Rect cardRect = tester.getRect(
          find.byType(OrderPaymentInfoCard),
        );
        final Rect titleRect = tester.getRect(find.text('Info Pembayaran'));
        expect(
          titleRect.right,
          lessThanOrEqualTo(cardRect.right + 0.5),
          reason: 'header title must stay inside the card',
        );
        expect(find.text('LUNAS'), findsOneWidget);
      },
    );
  });
}
