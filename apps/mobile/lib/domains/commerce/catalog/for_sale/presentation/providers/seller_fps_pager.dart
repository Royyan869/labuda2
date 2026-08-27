/// Simple FPS pager for CommerceResourcePicker.
///
/// Provides paginated access to seller's ForSales with load-more,
/// retry, and dedup semantics. Mirrors SellerAuctionsPagerController pattern.
library;

import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_controller.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';

final sellerFPSPagerProvider =
    NotifierProvider.autoDispose<SellerFPSPagerController, SellerFPSPagerState>(
      SellerFPSPagerController.new,
    );

class SellerFPSPagerState {
  final List<ForSale> items;
  final bool hasMore;
  final bool isInitialLoading;
  final bool isLoadingMore;
  final String? initialError;
  final String? loadMoreError;
  final String? ownerId;
  final int pageSize;

  const SellerFPSPagerState({
    required this.items,
    required this.hasMore,
    required this.isInitialLoading,
    required this.isLoadingMore,
    required this.initialError,
    required this.loadMoreError,
    required this.ownerId,
    required this.pageSize,
  });

  factory SellerFPSPagerState.initial({
    required String? ownerId,
    int pageSize = 20,
  }) {
    return SellerFPSPagerState(
      items: const [],
      hasMore: true,
      isInitialLoading: ownerId != null,
      isLoadingMore: false,
      initialError: null,
      loadMoreError: null,
      ownerId: ownerId,
      pageSize: pageSize,
    );
  }

  SellerFPSPagerState copyWith({
    List<ForSale>? items,
    bool? hasMore,
    bool? isInitialLoading,
    bool? isLoadingMore,
    String? initialError,
    String? loadMoreError,
    String? ownerId,
    int? pageSize,
    bool clearInitialError = false,
    bool clearLoadMoreError = false,
  }) {
    return SellerFPSPagerState(
      items: items ?? this.items,
      hasMore: hasMore ?? this.hasMore,
      isInitialLoading: isInitialLoading ?? this.isInitialLoading,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      initialError: clearInitialError
          ? null
          : initialError ?? this.initialError,
      loadMoreError: clearLoadMoreError
          ? null
          : loadMoreError ?? this.loadMoreError,
      ownerId: ownerId ?? this.ownerId,
      pageSize: pageSize ?? this.pageSize,
    );
  }
}

class SellerFPSPagerController extends Notifier<SellerFPSPagerState> {
  int _nextPage = 1;
  int _activeToken = 0;

  ForSaleController get _controller => ref.read(forSaleControllerProvider);

  @override
  SellerFPSPagerState build() {
    final authState = ref.watch(authControllerProvider);
    final user = switch (authState) {
      AuthStateAuthenticated(:final user) => user,
      _ => null,
    };
    final ownerId = user?.id;
    if (ownerId == null) return SellerFPSPagerState.initial(ownerId: null);
    unawaited(Future.microtask(loadInitial));
    return SellerFPSPagerState.initial(ownerId: ownerId);
  }

  Future<void> loadInitial() async {
    final ownerId = state.ownerId;
    if (ownerId == null) return;
    state = state.copyWith(isInitialLoading: true, clearInitialError: true);
    await _fetch(1, replaceExisting: true);
  }

  Future<void> loadMore() async {
    if (!state.hasMore || state.isLoadingMore || state.isInitialLoading) return;
    state = state.copyWith(isLoadingMore: true, clearLoadMoreError: true);
    await _fetch(_nextPage, replaceExisting: false);
  }

  Future<void> retryInitial() => loadInitial();
  Future<void> retryLoadMore() => loadMore();
  Future<void> refresh() async {
    _nextPage = 1;
    state = state.copyWith(
      hasMore: true,
      isInitialLoading: true,
      clearInitialError: true,
    );
    await _fetch(1, replaceExisting: true);
  }

  Future<void> _fetch(int page, {required bool replaceExisting}) async {
    final ownerId = state.ownerId;
    if (ownerId == null) return;
    final token = ++_activeToken;
    final snapshot = List<ForSale>.from(state.items);

    final result = await _controller.getSellerForSales(
      ownerId,
      page: page,
      pageSize: state.pageSize,
    );
    if (!ref.mounted || token != _activeToken) return;

    if (result.isError) {
      final err = result.error ?? 'Gagal memuat forSale';
      if (replaceExisting) {
        state = state.copyWith(
          items: snapshot,
          isInitialLoading: false,
          isLoadingMore: false,
          initialError: err,
        );
      } else {
        state = state.copyWith(
          items: snapshot,
          isLoadingMore: false,
          loadMoreError: err,
        );
      }
      return;
    }

    final incoming = _dedupe(result.data!);
    final merged = replaceExisting ? incoming : _merge(snapshot, incoming);
    final hasMore = incoming.length >= state.pageSize;
    _nextPage = page + 1;

    state = state.copyWith(
      items: merged,
      hasMore: hasMore,
      isInitialLoading: false,
      isLoadingMore: false,
      clearInitialError: true,
      clearLoadMoreError: true,
    );
  }

  List<ForSale> _dedupe(List<ForSale> items) {
    final seen = <String>{};
    return items.where((l) => seen.add(l.forSaleId)).toList();
  }

  List<ForSale> _merge(List<ForSale> existing, List<ForSale> incoming) {
    final seen = existing.map((l) => l.forSaleId).toSet();
    return [...existing, ...incoming.where((l) => seen.add(l.forSaleId))];
  }
}
