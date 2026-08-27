import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';

enum SellerAuctionFilter {
  all,
  draft,
  scheduled,
  active,
  finished,
}

extension SellerAuctionFilterX on SellerAuctionFilter {
  String get label {
    switch (this) {
      case SellerAuctionFilter.all:
        return 'Semua';
      case SellerAuctionFilter.draft:
        return 'Draft';
      case SellerAuctionFilter.scheduled:
        return 'Terjadwal';
      case SellerAuctionFilter.active:
        return 'Aktif';
      case SellerAuctionFilter.finished:
        return 'Selesai';
    }
  }

  bool matches(Auction auction) {
    switch (this) {
      case SellerAuctionFilter.all:
        return true;
      case SellerAuctionFilter.draft:
        return auction.status == AuctionStatus.draft;
      case SellerAuctionFilter.scheduled:
        return auction.status == AuctionStatus.scheduled;
      case SellerAuctionFilter.active:
        return auction.status == AuctionStatus.active;
      case SellerAuctionFilter.finished:
        return auction.status == AuctionStatus.waitingSettlement ||
            auction.status == AuctionStatus.ended ||
            auction.status == AuctionStatus.expiredBNR ||
            auction.status == AuctionStatus.cancelled;
    }
  }
}

final sellerAuctionsPagerProvider = NotifierProvider.autoDispose<
  SellerAuctionsPagerController,
  SellerAuctionsPagerState
>(SellerAuctionsPagerController.new);

class SellerAuctionsPagerState {
  final SellerAuctionFilter activeFilter;
  final List<Auction> auctions;
  final int pageSize;
  final bool hasMore;
  final bool isInitialLoading;
  final bool isLoadMoreLoading;
  final bool isRefreshing;
  final String? initialError;
  final String? loadMoreError;
  final String? refreshError;
  final String? ownerId;

  const SellerAuctionsPagerState({
    required this.activeFilter,
    required this.auctions,
    required this.pageSize,
    required this.hasMore,
    required this.isInitialLoading,
    required this.isLoadMoreLoading,
    required this.isRefreshing,
    required this.initialError,
    required this.loadMoreError,
    required this.refreshError,
    required this.ownerId,
  });

  factory SellerAuctionsPagerState.initial({
    required String? ownerId,
    SellerAuctionFilter activeFilter = SellerAuctionFilter.all,
    int pageSize = 20,
  }) {
    return SellerAuctionsPagerState(
      activeFilter: activeFilter,
      auctions: const [],
      pageSize: pageSize,
      hasMore: true,
      isInitialLoading: ownerId != null,
      isLoadMoreLoading: false,
      isRefreshing: false,
      initialError: null,
      loadMoreError: null,
      refreshError: null,
      ownerId: ownerId,
    );
  }

  bool get canLoadMore =>
      ownerId != null && hasMore && !isInitialLoading && !isLoadMoreLoading && !isRefreshing;

  List<Auction> get visibleAuctions =>
      activeFilter == SellerAuctionFilter.all
          ? auctions
          : auctions.where(activeFilter.matches).toList(growable: false);

  bool get hasVisibleAuctions => visibleAuctions.isNotEmpty;

  SellerAuctionsPagerState copyWith({
    SellerAuctionFilter? activeFilter,
    List<Auction>? auctions,
    int? pageSize,
    bool? hasMore,
    bool? isInitialLoading,
    bool? isLoadMoreLoading,
    bool? isRefreshing,
    String? initialError,
    String? loadMoreError,
    String? refreshError,
    String? ownerId,
    bool clearInitialError = false,
    bool clearLoadMoreError = false,
    bool clearRefreshError = false,
  }) {
    return SellerAuctionsPagerState(
      activeFilter: activeFilter ?? this.activeFilter,
      auctions: auctions ?? this.auctions,
      pageSize: pageSize ?? this.pageSize,
      hasMore: hasMore ?? this.hasMore,
      isInitialLoading: isInitialLoading ?? this.isInitialLoading,
      isLoadMoreLoading: isLoadMoreLoading ?? this.isLoadMoreLoading,
      isRefreshing: isRefreshing ?? this.isRefreshing,
      initialError: clearInitialError ? null : initialError ?? this.initialError,
      loadMoreError: clearLoadMoreError ? null : loadMoreError ?? this.loadMoreError,
      refreshError: clearRefreshError ? null : refreshError ?? this.refreshError,
      ownerId: ownerId ?? this.ownerId,
    );
  }
}

class SellerAuctionsPagerController
    extends Notifier<SellerAuctionsPagerState> {
  static const int _pageSize = 20;

  String? _lastOwnerId;
  int _activeRequestToken = 0;

  AuctionRepository get _repository => ref.read(auctionRepositoryProvider);

  @override
  SellerAuctionsPagerState build() {
    final authState = ref.watch(authControllerProvider);
    final user = switch (authState) {
      AuthStateAuthenticated(:final user) => user,
      _ => null,
    };
    final ownerId = user?.id;

    if (_lastOwnerId != ownerId) {
      _lastOwnerId = ownerId;
      _activeRequestToken++;
    }

    if (ownerId == null) {
      return SellerAuctionsPagerState.initial(ownerId: null);
    }

    unawaited(Future.microtask(loadInitial));
    return SellerAuctionsPagerState.initial(
      ownerId: ownerId,
      pageSize: _pageSize,
    );
  }

  void setFilter(SellerAuctionFilter filter) {
    if (state.activeFilter == filter) return;
    state = state.copyWith(
      activeFilter: filter,
      clearInitialError: true,
      clearLoadMoreError: true,
      clearRefreshError: true,
    );
  }

  Future<void> loadInitial() {
    final ownerId = state.ownerId;
    if (ownerId == null) return Future.value();

    return _fetchPage(
      pageToken: null,
      replaceExisting: true,
      preserveCurrentData: false,
      isRefresh: false,
    );
  }

  Future<void> refresh() {
    final ownerId = state.ownerId;
    if (ownerId == null) return Future.value();

    return _fetchPage(
      pageToken: null,
      replaceExisting: true,
      preserveCurrentData: true,
      isRefresh: true,
    );
  }

  Future<void> loadMore() {
    if (!state.canLoadMore) return Future.value();
    final cursor = state.auctions.isEmpty ? null : state.auctions.last.id;
    return _fetchPage(
      pageToken: cursor,
      replaceExisting: false,
      preserveCurrentData: true,
      isRefresh: false,
    );
  }

  Future<void> retryInitial() => loadInitial();

  Future<void> retryLoadMore() => loadMore();

  Future<void> _fetchPage({
    required String? pageToken,
    required bool replaceExisting,
    required bool preserveCurrentData,
    required bool isRefresh,
  }) async {
    final ownerId = state.ownerId;
    if (ownerId == null) return;

    final token = ++_activeRequestToken;
    final snapshot = List<Auction>.from(state.auctions);

    if (replaceExisting) {
      state = state.copyWith(
        auctions: preserveCurrentData ? snapshot : const [],
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

    final result = await _repository.getUserAuctions(
      sellerId: ownerId,
      status: null,
      limit: state.pageSize,
      lastAuctionId: pageToken,
    );

    if (!ref.mounted || token != _activeRequestToken) {
      return;
    }

    if (result.isError || result.data == null) {
      final errorMessage = result.error ?? 'Gagal memuat lelang';
      if (replaceExisting) {
        state = state.copyWith(
          auctions: preserveCurrentData ? snapshot : const [],
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
          auctions: snapshot,
          isLoadMoreLoading: false,
          loadMoreError: errorMessage,
        );
      }
      return;
    }

    final incoming = _dedupeById(result.data!);
    final merged = replaceExisting
        ? incoming
        : _mergeById(snapshot, incoming);

    state = state.copyWith(
      auctions: merged,
      hasMore: incoming.length >= state.pageSize,
      isInitialLoading: false,
      isLoadMoreLoading: false,
      isRefreshing: false,
      clearInitialError: true,
      clearLoadMoreError: true,
      clearRefreshError: true,
    );
  }

  List<Auction> _mergeById(List<Auction> existing, List<Auction> incoming) {
    final seenIds = existing.map((auction) => auction.id).toSet();
    final merged = List<Auction>.from(existing);
    for (final auction in incoming) {
      if (seenIds.add(auction.id)) {
        merged.add(auction);
      }
    }
    return merged;
  }

  List<Auction> _dedupeById(List<Auction> auctions) {
    final seenIds = <String>{};
    final deduped = <Auction>[];
    for (final auction in auctions) {
      if (seenIds.add(auction.id)) {
        deduped.add(auction);
      }
    }
    return deduped;
  }
}
