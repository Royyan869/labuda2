import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart';

class _FakeOrderRepository implements OrderRepository {
  final RepositoryResult<List<Order>> result;

  _FakeOrderRepository(this.result);

  @override
  Future<RepositoryResult<List<Order>>> getSellerOrders(
    GetOrdersParams params,
  ) async {
    return result;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  group('recentSellerOrdersProvider', () {
    test('completes with empty list when seller has no orders', () async {
      final container = ProviderContainer(
        overrides: [
          orderRepositoryProvider.overrideWithValue(
            _FakeOrderRepository(RepositoryResult.success(<Order>[])),
          ),
        ],
      );
      addTearDown(container.dispose);

      final orders = await container.read(
        recentSellerOrdersProvider('seller-1').future,
      );

      expect(orders, isEmpty);
      expect(
        container.read(recentSellerOrdersProvider('seller-1')).hasValue,
        isTrue,
      );
    });

    test('surfaces repository errors for retry UI', () async {
      final container = ProviderContainer(
        overrides: [
          orderRepositoryProvider.overrideWithValue(
            _FakeOrderRepository(RepositoryResult.error('backend failed')),
          ),
        ],
      );
      addTearDown(container.dispose);

      final subscription = container.listen(
        recentSellerOrdersProvider('seller-1'),
        (previous, next) {},
        fireImmediately: true,
      );
      addTearDown(subscription.close);

      await Future<void>.delayed(Duration.zero);

      final state = container.read(recentSellerOrdersProvider('seller-1'));
      expect(state.hasError, isTrue);
      expect(state.error, isA<Exception>());
    });
  });
}
