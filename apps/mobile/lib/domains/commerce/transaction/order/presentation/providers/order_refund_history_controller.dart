import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/transaction/order/data/order_providers.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';

typedef OrderRefundHistoryQuery = ({
  String orderId,
  String principalId,
  int pageSize,
});

const Object _unset = Object();

final orderRefundHistoryPagerProvider = NotifierProvider.autoDispose
    .family<
      OrderRefundHistoryPagerController,
      OrderRefundHistoryPagerState,
      OrderRefundHistoryQuery
    >((query) => OrderRefundHistoryPagerController(query));

class OrderRefundHistoryPagerState {
  final List<RefundRequest> refunds;
  final String? nextCursor;
  final bool hasMore;
  final bool isInitialLoading;
  final bool isLoadMoreLoading;
  final bool isRefreshing;
  final String? initialError;
  final String? loadMoreError;
  final String? refreshError;

  const OrderRefundHistoryPagerState({
    required this.refunds,
    required this.nextCursor,
    required this.hasMore,
    required this.isInitialLoading,
    required this.isLoadMoreLoading,
    required this.isRefreshing,
    required this.initialError,
    required this.loadMoreError,
    required this.refreshError,
  });

  factory OrderRefundHistoryPagerState.initial() {
    return const OrderRefundHistoryPagerState(
      refunds: [],
      nextCursor: null,
      hasMore: true,
      isInitialLoading: true,
      isLoadMoreLoading: false,
      isRefreshing: false,
      initialError: null,
      loadMoreError: null,
      refreshError: null,
    );
  }

  bool get canLoadMore =>
      hasMore && !isInitialLoading && !isLoadMoreLoading && !isRefreshing;

  bool get hasInitialContent => refunds.isNotEmpty;

  OrderRefundHistoryPagerState copyWith({
    List<RefundRequest>? refunds,
    Object? nextCursor = _unset,
    Object? hasMore = _unset,
    Object? isInitialLoading = _unset,
    Object? isLoadMoreLoading = _unset,
    Object? isRefreshing = _unset,
    Object? initialError = _unset,
    Object? loadMoreError = _unset,
    Object? refreshError = _unset,
    bool clearInitialError = false,
    bool clearLoadMoreError = false,
    bool clearRefreshError = false,
  }) {
    return OrderRefundHistoryPagerState(
      refunds: refunds ?? this.refunds,
      nextCursor: nextCursor == _unset
          ? this.nextCursor
          : nextCursor as String?,
      hasMore: hasMore == _unset ? this.hasMore : hasMore as bool,
      isInitialLoading: isInitialLoading == _unset
          ? this.isInitialLoading
          : isInitialLoading as bool,
      isLoadMoreLoading: isLoadMoreLoading == _unset
          ? this.isLoadMoreLoading
          : isLoadMoreLoading as bool,
      isRefreshing: isRefreshing == _unset
          ? this.isRefreshing
          : isRefreshing as bool,
      initialError: clearInitialError
          ? null
          : initialError == _unset
          ? this.initialError
          : initialError as String?,
      loadMoreError: clearLoadMoreError
          ? null
          : loadMoreError == _unset
          ? this.loadMoreError
          : loadMoreError as String?,
      refreshError: clearRefreshError
          ? null
          : refreshError == _unset
          ? this.refreshError
          : refreshError as String?,
    );
  }
}

class OrderRefundHistoryPagerController
    extends Notifier<OrderRefundHistoryPagerState> {
  OrderRefundHistoryPagerController(this._query);

  final OrderRefundHistoryQuery _query;
  int _requestToken = 0;

  RefundRepository get _repository => ref.read(refundRepositoryProvider);

  @override
  OrderRefundHistoryPagerState build() {
    unawaited(Future.microtask(loadInitial));
    return OrderRefundHistoryPagerState.initial();
  }

  Future<void> loadInitial() {
    return _fetchPage(
      replaceExisting: true,
      preserveCurrentData: false,
      isRefresh: false,
      cursor: null,
    );
  }

  Future<void> refresh() {
    return _fetchPage(
      replaceExisting: true,
      preserveCurrentData: true,
      isRefresh: true,
      cursor: null,
    );
  }

  Future<void> loadNextPage() {
    if (!state.canLoadMore) return Future.value();
    if (state.nextCursor == null || state.nextCursor!.isEmpty) {
      return Future.value();
    }
    return _fetchPage(
      replaceExisting: false,
      preserveCurrentData: true,
      isRefresh: false,
      cursor: state.nextCursor,
    );
  }

  Future<void> retryInitial() => loadInitial();

  Future<void> retryLoadMore() => loadNextPage();

  Future<void> _fetchPage({
    required bool replaceExisting,
    required bool preserveCurrentData,
    required bool isRefresh,
    required String? cursor,
  }) async {
    final token = ++_requestToken;
    final snapshot = List<RefundRequest>.from(state.refunds);

    if (replaceExisting) {
      state = state.copyWith(
        refunds: preserveCurrentData ? snapshot : const [],
        nextCursor: preserveCurrentData ? state.nextCursor : null,
        hasMore: preserveCurrentData ? state.hasMore : true,
        isInitialLoading: !preserveCurrentData,
        isLoadMoreLoading: false,
        isRefreshing: isRefresh,
        clearInitialError: true,
        clearLoadMoreError: true,
        clearRefreshError: true,
      );
    } else {
      state = state.copyWith(isLoadMoreLoading: true, clearLoadMoreError: true);
    }

    final result = await _repository.listOrderRefundHistory(
      ListOrderRefundHistoryParams(
        orderId: _query.orderId,
        cursor: cursor,
        pageSize: _query.pageSize,
      ),
    );

    if (!ref.mounted || token != _requestToken) {
      return;
    }

    if (result.isError || result.data == null) {
      final errorMessage = result.error ?? 'Failed to load refund history';
      if (replaceExisting) {
        state = state.copyWith(
          refunds: preserveCurrentData ? snapshot : const [],
          nextCursor: preserveCurrentData ? state.nextCursor : null,
          hasMore: preserveCurrentData ? state.hasMore : true,
          isInitialLoading: false,
          isLoadMoreLoading: false,
          isRefreshing: false,
          initialError: preserveCurrentData ? state.initialError : errorMessage,
          refreshError: isRefresh ? errorMessage : state.refreshError,
          clearLoadMoreError: true,
        );
      } else {
        state = state.copyWith(
          refunds: snapshot,
          isLoadMoreLoading: false,
          loadMoreError: errorMessage,
        );
      }
      return;
    }

    final pageResult = result.data!;
    final mergedRefunds = replaceExisting
        ? _dedupeRefunds(pageResult.refunds)
        : _mergeRefunds(snapshot, pageResult.refunds);

    state = state.copyWith(
      refunds: mergedRefunds,
      nextCursor: pageResult.nextCursor,
      hasMore: pageResult.hasMore,
      isInitialLoading: false,
      isLoadMoreLoading: false,
      isRefreshing: false,
      clearInitialError: true,
      clearLoadMoreError: true,
      clearRefreshError: true,
    );
  }

  List<RefundRequest> _mergeRefunds(
    List<RefundRequest> existing,
    List<RefundRequest> incoming,
  ) {
    final seenIds = existing.map((refund) => refund.id).toSet();
    final merged = List<RefundRequest>.from(existing);
    for (final refund in incoming) {
      if (seenIds.add(refund.id)) {
        merged.add(refund);
      }
    }
    return merged;
  }

  List<RefundRequest> _dedupeRefunds(List<RefundRequest> refunds) {
    final seenIds = <String>{};
    final deduped = <RefundRequest>[];
    for (final refund in refunds) {
      if (seenIds.add(refund.id)) {
        deduped.add(refund);
      }
    }
    return deduped;
  }
}
