import 'package:go_router/go_router.dart';
import 'base_module.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart';

/// Order Module
///
/// Mengelola semua routes yang berkaitan dengan orders:
/// - Order List (view all orders)
/// - Order Detail (view specific order)
/// - Payment Result (payment status reconciliation)
///
/// Module ini mengelola order-related navigation.
class OrderModule extends BaseModule {
  @override
  List<GoRoute> get routes => [
    // Order List Route
    GoRoute(
      path: '/orders',
      name: 'orders',
      builder: (context, state) => const OrderListScreen(isSeller: false),
    ),

    // Order Detail Route
    GoRoute(
      path: '/orders/:orderId',
      name: 'orderDetail',
      builder: (context, state) {
        final orderId = state.pathParameters['orderId']!;
        return OrderDetailScreen(orderId: orderId);
      },
    ),
  ];

  @override
  Future<void> initialize() async {
    // Register order-related services if needed
    // Services akan diregister di sini ketika diperlukan
  }

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {
    // Cleanup order module resources
  }

  @override
  String get moduleName => 'OrderModule';
}
