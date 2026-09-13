import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/refund_request.dart';
import 'package:labuda/domains/commerce/transaction/order/presentation/screens/order_detail/order_refund_list_section.dart';

RefundRequest _refund({required String id, required RefundStatus status}) {
  return RefundRequest(
    id: id,
    orderId: 'order-1',
    buyerId: 'buyer-1',
    sellerId: 'seller-1',
    reason: RefundReason.itemNotReceived,
    status: status,
    refundAmount: 100000,
    createdAt: DateTime.utc(2026, 7, 1, 12),
  );
}

Widget _wrap(Widget child) {
  return MaterialApp(home: Scaffold(body: child));
}

void main() {
  setUpAll(() async {
    await initializeDateFormatting();
  });

  testWidgets('renders latest and historical refund entries', (tester) async {
    await tester.pumpWidget(
      _wrap(
        OrderRefundListSection(
          refunds: [
            _refund(
              id: 'refund-latest',
              status: RefundStatus.pendingSellerReview,
            ),
            _refund(id: 'refund-old-1', status: RefundStatus.refunded),
            _refund(id: 'refund-old-2', status: RefundStatus.sellerRejected),
          ],
          isDark: false,
          currentUserId: 'buyer-1',
          sellerId: 'seller-1',
        ),
      ),
    );

    expect(find.text('Permintaan Pengembalian'), findsOneWidget);
    expect(find.text('Awaiting Seller Review'), findsOneWidget);
    expect(find.text('Menunggu respon penjual'), findsOneWidget);
    expect(
      find.text('Ada 3 permintaan pengembalian untuk pesanan ini'),
      findsOneWidget,
    );
  });

  testWidgets('shows rejected state without presenting it as refunded', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        OrderRefundListSection(
          refunds: [
            _refund(id: 'refund-latest', status: RefundStatus.sellerRejected),
          ],
          isDark: false,
          currentUserId: 'buyer-1',
          sellerId: 'seller-1',
        ),
      ),
    );

    expect(find.text('Ditolak Penjual'), findsOneWidget);
    expect(find.text('Funds Returned'), findsNothing);
  });

  testWidgets('shows empty state as no section', (tester) async {
    await tester.pumpWidget(
      _wrap(
        const OrderRefundListSection(
          refunds: [],
          isDark: false,
          currentUserId: 'buyer-1',
          sellerId: 'seller-1',
        ),
      ),
    );

    expect(find.text('Permintaan Pengembalian'), findsNothing);
  });
}
