import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/catalog/auction/data/auction_providers.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/domain.dart';

/// Canonical My Auctions filter options, in presentation order.
///
/// `null` is the synthetic "Semua" option (no status restriction); every other
/// entry is a canonical [AuctionStatus]. This is the single source for filter
/// identity, ordering and labels — there is no aggregate option.
const List<AuctionStatus?> kSellerAuctionFilters = <AuctionStatus?>[
  null, // Semua
  AuctionStatus.scheduled,
  AuctionStatus.active,
  AuctionStatus.waitingSettlement,
  AuctionStatus.ended,
  AuctionStatus.cancelled,
  AuctionStatus.lapsed,
];

/// Canonical label for a My Auctions filter. "Semua" is the only synthetic
/// option; status labels resolve through the single authority
/// [AuctionStatus.displayName].
String sellerAuctionFilterLabel(AuctionStatus? status) =>
    status == null ? 'Semua' : status.displayName;

final sellerAuctionsPagerProvider =
    NotifierProvider.autoDispose<
      SellerAuctionsPagerController,
      SellerAuctionsPagerState
    >(SellerAuctionsPagerController.new);

class SellerAuctionsPagerState {
  /// Canonical filter selection: `null` = Semua, otherwise exactly one
  /// [AuctionStatus].
  final AuctionStatus? activeFilter;
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
    AuctionStatus? activeFilter,
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
      ownerId != null &&
      hasMore &&
      !isInitialLoading &&
      !isLoadMoreLoading &&
      !isRefreshing;

  /// Canonical status-filtering rule for the loaded [auctions] collection.
  ///
  /// `null` (Semua) returns the collection unchanged; a non-null [status]
  /// returns only the auctions whose status exactly matches it. Source
  /// ordering is preserved and the result is non-growable.
  ///
  /// This is the SINGLE filtering authority. [visibleAuctions] projects the
  /// committed [activeFilter]; the screen projects a `TabBarView` page's own
  /// filter mid-swipe so an incoming page never shows the previously selected
  /// tab's collection before the controller settles.
  List<Auction> auctionsFor(AuctionStatus? status) => status == null
      ? auctions
      : auctions
            .where((auction) => auction.status == status)
            .toList(growable: false);

  List<Auction> get visibleAuctions => auctionsFor(activeFilter);

  bool get hasVisibleAuctions => visibleAuctions.isNotEmpty;

  SellerAuctionsPagerState copyWith({
    AuctionStatus? activeFilter,
    bool clearActiveFilter = false,
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
      activeFilter: clearActiveFilter
          ? null
          : (activeFilter ?? this.activeFilter),
      auctions: auctions ?? this.auctions,
      pageSize: pageSize ?? this.pageSize,
      hasMore: hasMore ?? this.hasMore,
      isInitialLoading: isInitialLoading ?? this.isInitialLoading,
      isLoadMoreLoading: isLoadMoreLoading ?? this.isLoadMoreLoading,
      isRefreshing: isRefreshing ?? this.isRefreshing,
      initialError: clearInitialError
          ? null
          : initialError ?? this.initialError,
      loadMoreError: clearLoadMoreError
          ? null
          : loadMoreError ?? this.loadMoreError,
      refreshError: clearRefreshError
          ? null
          : refreshError ?? this.refreshError,
      ownerId: ownerId ?? this.ownerId,
    );
  }
}

class SellerAuctionsPagerController extends Notifier<SellerAuctionsPagerState> {
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

  void setFilter(AuctionStatus? filter) {
    if (state.activeFilter == filter) return;
    state = state.copyWith(
      activeFilter: filter,
      clearActiveFilter: filter == null,
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
    // The backend cursor is the RFC3339 created_at of the last row — it
    // rejects anything else with 400 "Invalid cursor format". Sending the
    // auction id here is what made load-more fail on this screen.
    final cursor = state.auctions.isEmpty
        ? null
        : state.auctions.last.createdAt.toUtc().toIso8601String();
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
      state = state.copyWith(isLoadMoreLoading: true, clearLoadMoreError: true);
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
    final merged = replaceExisting ? incoming : _mergeById(snapshot, incoming);

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
