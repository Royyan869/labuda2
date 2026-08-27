import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/transaction/order/data/order_providers.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';

typedef OrderListQuery = ({
  String userId,
  bool isSeller,
  OrderStatus? status,
  int pageSize,
});

final orderListPagerProvider = NotifierProvider.autoDispose.family<
  OrderListPagerController,
  OrderListPagerState,
  OrderListQuery
>((query) => OrderListPagerController(query));

class OrderListPagerState {
  final OrderStatus? activeFilter;
  final List<Order> orders;
  final int currentPage;
  final int pageSize;
  final bool hasMore;
  final bool isInitialLoading;
  final bool isLoadMoreLoading;
  final bool isRefreshing;
  final String? initialError;
  final String? loadMoreError;
  final String? refreshError;

  const OrderListPagerState({
    required this.activeFilter,
    required this.orders,
    required this.currentPage,
    required this.pageSize,
    required this.hasMore,
    required this.isInitialLoading,
    required this.isLoadMoreLoading,
    required this.isRefreshing,
    required this.initialError,
    required this.loadMoreError,
    required this.refreshError,
  });

  factory OrderListPagerState.initial({
    required OrderStatus? activeFilter,
    required int pageSize,
  }) {
    return OrderListPagerState(
      activeFilter: activeFilter,
      orders: const [],
      currentPage: 0,
      pageSize: pageSize,
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

  bool get hasInitialContent => orders.isNotEmpty;

  OrderListPagerState copyWith({
    OrderStatus? activeFilter,
    List<Order>? orders,
    int? currentPage,
    int? pageSize,
    bool? hasMore,
    bool? isInitialLoading,
    bool? isLoadMoreLoading,
    bool? isRefreshing,
    String? initialError,
    String? loadMoreError,
    String? refreshError,
    bool clearInitialError = false,
    bool clearLoadMoreError = false,
    bool clearRefreshError = false,
  }) {
    return OrderListPagerState(
      activeFilter: activeFilter ?? this.activeFilter,
      orders: orders ?? this.orders,
      currentPage: currentPage ?? this.currentPage,
      pageSize: pageSize ?? this.pageSize,
      hasMore: hasMore ?? this.hasMore,
      isInitialLoading: isInitialLoading ?? this.isInitialLoading,
      isLoadMoreLoading: isLoadMoreLoading ?? this.isLoadMoreLoading,
      isRefreshing: isRefreshing ?? this.isRefreshing,
      initialError: clearInitialError ? null : initialError ?? this.initialError,
      loadMoreError: clearLoadMoreError ? null : loadMoreError ?? this.loadMoreError,
      refreshError: clearRefreshError ? null : refreshError ?? this.refreshError,
    );
  }
}

class OrderListPagerController extends Notifier<OrderListPagerState> {
  OrderListPagerController(this._query);

  final OrderListQuery _query;
  int _requestToken = 0;

  OrderRepository get _repository => ref.read(orderRepositoryProvider);

  @override
  OrderListPagerState build() {
    unawaited(Future.microtask(loadInitial));
    return OrderListPagerState.initial(
      activeFilter: _query.status,
      pageSize: _query.pageSize,
    );
  }

  Future<void> loadInitial() {
    return _fetchPage(
      page: 1,
      replaceExisting: true,
      preserveCurrentData: false,
      isRefresh: false,
    );
  }

  Future<void> refresh() {
    return _fetchPage(
      page: 1,
      replaceExisting: true,
      preserveCurrentData: true,
      isRefresh: true,
    );
  }

  Future<void> loadNextPage() {
    if (!state.canLoadMore) return Future.value();

    final nextPage = state.currentPage + 1;
    return _fetchPage(
      page: nextPage,
      replaceExisting: false,
      preserveCurrentData: true,
      isRefresh: false,
    );
  }

  Future<void> retryInitial() => loadInitial();

  Future<void> retryLoadMore() => loadNextPage();

  Future<void> _fetchPage({
    required int page,
    required bool replaceExisting,
    required bool preserveCurrentData,
    required bool isRefresh,
  }) async {
    final token = ++_requestToken;
    final snapshot = List<Order>.from(state.orders);

    if (replaceExisting) {
      state = state.copyWith(
        orders: preserveCurrentData ? snapshot : const [],
        currentPage: preserveCurrentData ? state.currentPage : 0,
        hasMore: preserveCurrentData ? state.hasMore : true,
        isInitialLoading: !preserveCurrentData,
        isLoadMoreLoading: false,
        isRefreshing: isRefresh,
        clearInitialError: true,
        clearLoadMoreError: true,
        clearRefreshError: true,
      );
    } else {
      state = state.copyWith(
        isLoadMoreLoading: true,
        clearLoadMoreError: true,
      );
    }

    final result = _query.isSeller
        ? await _repository.getSellerOrdersPage(
            GetOrdersParams(
              userId: _query.userId,
              status: _query.status,
              page: page,
              pageSize: _query.pageSize,
            ),
          )
        : await _repository.getBuyerOrdersPage(
            GetOrdersParams(
              userId: _query.userId,
              status: _query.status,
              page: page,
              pageSize: _query.pageSize,
            ),
          );

    if (!ref.mounted || token != _requestToken) {
      return;
    }

    if (result.isError || result.data == null) {
      final errorMessage = result.error ?? 'Failed to load orders';
      if (replaceExisting) {
        state = state.copyWith(
          orders: preserveCurrentData ? snapshot : const [],
          currentPage: preserveCurrentData ? state.currentPage : 0,
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
          orders: snapshot,
          isLoadMoreLoading: false,
          loadMoreError: errorMessage,
        );
      }
      return;
    }

    final pageResult = result.data!;
    final mergedOrders = replaceExisting
        ? _dedupeOrders(pageResult.orders)
        : _mergeOrders(snapshot, pageResult.orders);

    state = state.copyWith(
      orders: mergedOrders,
      currentPage: pageResult.page,
      pageSize: pageResult.pageSize,
      hasMore: pageResult.hasMore,
      isInitialLoading: false,
      isLoadMoreLoading: false,
      isRefreshing: false,
      clearInitialError: true,
      clearLoadMoreError: true,
      clearRefreshError: true,
    );
  }

  List<Order> _mergeOrders(List<Order> existing, List<Order> incoming) {
    final seenIds = existing.map((order) => order.id).toSet();
    final merged = List<Order>.from(existing);
    for (final order in incoming) {
      if (seenIds.add(order.id)) {
        merged.add(order);
      }
    }
    return merged;
  }

  List<Order> _dedupeOrders(List<Order> orders) {
    final seenIds = <String>{};
    final deduped = <Order>[];
    for (final order in orders) {
      if (seenIds.add(order.id)) {
        deduped.add(order);
      }
    }
    return deduped;
  }
}
