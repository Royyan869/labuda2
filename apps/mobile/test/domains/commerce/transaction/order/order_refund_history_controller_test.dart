import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/order/data/order_providers.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/order/presentation/providers/order_refund_history_controller.dart';

class _FakeRefundRepository extends Fake implements RefundRepository {
  _FakeRefundRepository(this._handler);

  final Future<RepositoryResult<RefundHistoryPageResult>> Function(
    ListOrderRefundHistoryParams params,
  )
  _handler;

  final calls = <ListOrderRefundHistoryParams>[];

  @override
  Future<RepositoryResult<RefundHistoryPageResult>> listOrderRefundHistory(
    ListOrderRefundHistoryParams params,
  ) {
    calls.add(params);
    return _handler(params);
  }

  @override
  Stream<RefundRequest?> watchRefundByOrderId(String orderId) =>
      const Stream.empty();

  @override
  Future<RepositoryResult<RefundRequest>> approveRefund(
    String refundId, {
    String? notes,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<RefundRequest>> createRefund(
    CreateRefundParams params,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<Map<String, dynamic>>> escalateRefund(
    String refundId,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<RefundRequest>> getRefund(String refundId) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<RefundRequest?>> getRefundByOrderId(
    String orderId,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<List<RefundRequest>>> listBuyerRefunds(
    ListRefundsParams params,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<List<RefundRequest>>> listSellerRefunds(
    ListRefundsParams params,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<RefundRequest>> rejectRefund(
    String refundId, {
    String? notes,
  }) async {
    throw UnimplementedError();
  }
}

RefundRequest _refund({
  required String id,
  required RefundStatus status,
  required DateTime createdAt,
}) {
  return RefundRequest(
    id: id,
    orderId: 'order-1',
    buyerId: 'buyer-1',
    sellerId: 'seller-1',
    reason: RefundReason.itemNotReceived,
    status: status,
    refundAmount: 100000,
    createdAt: createdAt,
  );
}

Future<void> _settle() async {
  await Future<void>.delayed(Duration.zero);
  await Future<void>.delayed(Duration.zero);
}

void main() {
  group('orderRefundHistoryPagerProvider', () {
    test('loads newest-first history and paginates with cursor', () async {
      final newest = _refund(
        id: 'refund-3',
        status: RefundStatus.refunded,
        createdAt: DateTime.utc(2026, 7, 1, 12),
      );
      final middle = _refund(
        id: 'refund-2',
        status: RefundStatus.sellerRejected,
        createdAt: DateTime.utc(2026, 7, 1, 11),
      );
      final oldest = _refund(
        id: 'refund-1',
        status: RefundStatus.pendingSellerReview,
        createdAt: DateTime.utc(2026, 7, 1, 10),
      );

      final repo = _FakeRefundRepository((params) async {
        if (params.cursor == null) {
          expect(params.orderId, 'order-1');
          expect(params.pageSize, 2);
          return RepositoryResult.success(
            RefundHistoryPageResult(
              refunds: [newest, middle],
              nextCursor: 'cursor-2',
              hasMore: true,
              pageSize: params.pageSize,
            ),
          );
        }

        expect(params.cursor, 'cursor-2');
        return RepositoryResult.success(
          RefundHistoryPageResult(
            refunds: [oldest],
            nextCursor: null,
            hasMore: false,
            pageSize: params.pageSize,
          ),
        );
      });

      final container = ProviderContainer(
        overrides: [refundRepositoryProvider.overrideWithValue(repo)],
      );
      addTearDown(container.dispose);

      final provider = orderRefundHistoryPagerProvider((
        orderId: 'order-1',
        principalId: 'buyer-1',
        pageSize: 2,
      ));
      final sub = container.listen(provider, (_, __) {}, fireImmediately: true);
      addTearDown(sub.close);

      await _settle();

      final initial = container.read(provider);
      expect(initial.refunds.map((r) => r.id), ['refund-3', 'refund-2']);
      expect(initial.refunds.first.isRefunded, isTrue);
      expect(initial.refunds.last.isRejected, isTrue);
      expect(initial.hasMore, isTrue);
      expect(initial.nextCursor, isNotNull);
      expect(repo.calls.first.pageSize, 2);

      await container.read(provider.notifier).loadNextPage();
      await _settle();

      final afterLoadMore = container.read(provider);
      expect(afterLoadMore.refunds.map((r) => r.id), [
        'refund-3',
        'refund-2',
        'refund-1',
      ]);
      expect(afterLoadMore.hasMore, isFalse);
      expect(afterLoadMore.nextCursor, isNull);
      expect(repo.calls.length, 2);
    });

    test(
      'load-more failure preserves existing history and exposes error',
      () async {
        final first = _refund(
          id: 'refund-1',
          status: RefundStatus.pendingSellerReview,
          createdAt: DateTime.utc(2026, 7, 1, 12),
        );

        final repo = _FakeRefundRepository((params) async {
          if (params.cursor == null) {
            return RepositoryResult.success(
              RefundHistoryPageResult(
                refunds: [first],
                nextCursor: 'cursor-2',
                hasMore: true,
                pageSize: params.pageSize,
              ),
            );
          }
          return RepositoryResult.failure('load more failed');
        });

        final container = ProviderContainer(
          overrides: [refundRepositoryProvider.overrideWithValue(repo)],
        );
        addTearDown(container.dispose);

        final provider = orderRefundHistoryPagerProvider((
          orderId: 'order-1',
          principalId: 'buyer-1',
          pageSize: 1,
        ));
        final sub = container.listen(
          provider,
          (_, __) {},
          fireImmediately: true,
        );
        addTearDown(sub.close);

        await _settle();
        await container.read(provider.notifier).loadNextPage();
        await _settle();

        final state = container.read(provider);
        expect(state.refunds, hasLength(1));
        expect(state.refunds.first.id, 'refund-1');
        expect(state.loadMoreError, contains('load more failed'));
        expect(state.hasMore, isTrue);
      },
    );

    test(
      'dedupes refund ids across pages and refresh reflects mutations',
      () async {
        var currentPage = <RefundRequest>[
          _refund(
            id: 'refund-1',
            status: RefundStatus.pendingSellerReview,
            createdAt: DateTime.utc(2026, 7, 1, 12),
          ),
          _refund(
            id: 'refund-2',
            status: RefundStatus.sellerRejected,
            createdAt: DateTime.utc(2026, 7, 1, 11),
          ),
        ];

        final repo = _FakeRefundRepository((params) async {
          if (params.cursor == null) {
            return RepositoryResult.success(
              RefundHistoryPageResult(
                refunds: currentPage,
                nextCursor: 'cursor-2',
                hasMore: true,
                pageSize: params.pageSize,
              ),
            );
          }
          return RepositoryResult.success(
            RefundHistoryPageResult(
              refunds: [
                currentPage.last,
                _refund(
                  id: 'refund-3',
                  status: RefundStatus.refunded,
                  createdAt: DateTime.utc(2026, 7, 1, 10),
                ),
              ],
              nextCursor: null,
              hasMore: false,
              pageSize: params.pageSize,
            ),
          );
        });

        final container = ProviderContainer(
          overrides: [refundRepositoryProvider.overrideWithValue(repo)],
        );
        addTearDown(container.dispose);

        final provider = orderRefundHistoryPagerProvider((
          orderId: 'order-1',
          principalId: 'buyer-1',
          pageSize: 2,
        ));
        final sub = container.listen(
          provider,
          (_, __) {},
          fireImmediately: true,
        );
        addTearDown(sub.close);

        await _settle();
        await container.read(provider.notifier).loadNextPage();
        await _settle();

        final afterPaging = container.read(provider);
        expect(afterPaging.refunds.map((r) => r.id), [
          'refund-1',
          'refund-2',
          'refund-3',
        ]);

        currentPage = [
          _refund(
            id: 'refund-1',
            status: RefundStatus.refunded,
            createdAt: DateTime.utc(2026, 7, 1, 12),
          ),
        ];
        await container.read(provider.notifier).refresh();
        await _settle();

        final afterRefresh = container.read(provider);
        expect(afterRefresh.refunds.first.status, RefundStatus.refunded);
      },
    );

    test(
      'distinct order and principal keys keep independent history state',
      () async {
        final repo = _FakeRefundRepository((params) async {
          final suffix = params.orderId.endsWith('1') ? '1' : '2';
          return RepositoryResult.success(
            RefundHistoryPageResult(
              refunds: [
                _refund(
                  id: 'refund-$suffix',
                  status: RefundStatus.refunded,
                  createdAt: DateTime.utc(2026, 7, 1, 12),
                ),
              ],
              nextCursor: null,
              hasMore: false,
              pageSize: params.pageSize,
            ),
          );
        });

        final container = ProviderContainer(
          overrides: [refundRepositoryProvider.overrideWithValue(repo)],
        );
        addTearDown(container.dispose);

        final providerBuyer = orderRefundHistoryPagerProvider((
          orderId: 'order-1',
          principalId: 'buyer-1',
          pageSize: 1,
        ));
        final providerSeller = orderRefundHistoryPagerProvider((
          orderId: 'order-2',
          principalId: 'buyer-2',
          pageSize: 1,
        ));
        final sub1 = container.listen(
          providerBuyer,
          (_, __) {},
          fireImmediately: true,
        );
        final sub2 = container.listen(
          providerSeller,
          (_, __) {},
          fireImmediately: true,
        );
        addTearDown(sub1.close);
        addTearDown(sub2.close);

        await _settle();

        expect(container.read(providerBuyer).refunds.single.id, 'refund-1');
        expect(container.read(providerSeller).refunds.single.id, 'refund-2');
        expect(repo.calls.length, 2);
        expect(repo.calls.map((call) => call.orderId.toString()), [
          'order-1',
          'order-2',
        ]);
      },
    );
  });
}
