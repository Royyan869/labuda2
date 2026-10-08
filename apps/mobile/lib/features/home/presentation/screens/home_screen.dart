import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/features/home/home.dart';

/// Home Screen - Feed display dengan Clean Architecture
///
/// FEED / DISCOVERY QUALITY PASS V1:
///
/// PRODUCT CONTRACT:
/// - Home Feed is a SOCIAL-first timeline
/// - Displays: Universal content and reposts only (no commerce shelf)
/// - NO commerce objects (For Sale items, auctions) - those belong in Marketplace
/// - NO "Sedang Laku Hari Ini" shelf — promotion shelf will be reintroduced only via canonical promotion delivery when promotion is active
/// - Reposts are clearly distinguished with canonical RepostAttributionBar
/// - No fake engagement counts (hidden instead of showing "0")
class HomeScreen extends ConsumerStatefulWidget {
  const HomeScreen({super.key});

  @override
  ConsumerState<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends ConsumerState<HomeScreen> {
  // Distance from the bottom of the scroll extent (in logical pixels) at
  // which the next feed page should be fetched. Keep this large enough to
  // hide the network round-trip, small enough that we don't prefetch when
  // the user is barely scrolling.
  static const double _loadMoreThreshold = 400.0;

  @override
  void initState() {
    super.initState();
    // Set global callback untuk refresh feed dari mana saja
    setGlobalFeedRefreshCallback(() {
      if (mounted) {
        resetPromotionExposureAttempts();
        ref.invalidate(feedProvider);
      }
    });
  }

  /// Decide whether the current scroll position should trigger a
  /// pagination fetch, and dispatch it if so.
  ///
  /// All guards are checked against the most recent [FeedState]:
  ///   - skip while the initial load is in flight (isLoading)
  ///   - skip while a previous loadMore is in flight (isLoadingMore)
  ///   - skip after backend has reported has_more=false (hasReachedMax)
  ///   - skip while the feed is in an error state — user retries via the
  ///     "Try Again" button, not by scrolling
  ///   - skip if we haven't actually reached the threshold yet
  ///
  /// The notifier additionally holds a synchronous private lock, so even
  /// if the scroll listener fires faster than state can propagate, only
  /// one network request can be in flight at a time.
  void _maybeLoadMore(ScrollMetrics metrics, FeedState state) {
    if (state.isLoading) return;
    if (state.isLoadingMore) return;
    if (state.hasReachedMax) return;
    if (state.errorMessage != null) return;
    if (metrics.pixels < metrics.maxScrollExtent - _loadMoreThreshold) return;
    final before = state.items.length;
    ref.read(feedProvider.notifier).loadMore().then((_) {
      if (!mounted) return;
      _precacheNewItems(before);
    });
  }

  /// Decode the next page's first images while the user is still 400px away,
  /// so below-fold cards paint from cache. Bounded: images only ([MediaType]
  /// is the authority, never the URL suffix), first media per item, max 6
  /// per page. Failures are silent by design — the card loads normally.
  void _precacheNewItems(int before) {
    final items = ref.read(feedProvider).items;
    var queued = 0;
    for (var i = before; i < items.length && queued < 6; i++) {
      final media = items[i].media;
      if (media.isEmpty) continue;
      final first = media.first;
      if (first.type != MediaType.image) continue;
      final url = first.originalUrl.trim();
      if (url.isEmpty) continue;
      queued++;
      try {
        unawaited(
          precacheImage(
            NetworkImage(url),
            context,
          ).then((_) {}, onError: (_) {}),
        );
      } catch (_) {}
    }
  }

  @override
  Widget build(BuildContext context) {
    final feedState = ref.watch(feedProvider);
    return _buildFeedContent(feedState);
  }

  Widget _buildFeedContent(FeedState feedState) {
    // LOADING FOUNDATION (owner-locked):
    // - No data yet → first-load states only: loading, PageErrorState, or
    //   EmptyState. Loading is never Empty and Error is never Empty.
    // - Data present → items stay visible during refresh; update progress and
    //   refresh failure render inline, never as full-page loading/error.
    final feedItems = feedState.items;
    if (feedItems.isEmpty) {
      if (feedState.isLoading) {
        return const Center(child: LoadingIndicator());
      }
      if (feedState.errorMessage != null) {
        return _buildError();
      }
      if (feedState.isRefreshing) {
        return const Center(child: LoadingIndicator());
      }
      return _buildEmptyState();
    }

    return RefreshIndicator(
      onRefresh: () async {
        resetPromotionExposureAttempts();
        await ref.read(feedProvider.notifier).refresh();
      },
      child: NotificationListener<ScrollNotification>(
        onNotification: (notification) {
          // Drive pagination from the scroll position. Returning false
          // lets the notification continue propagating to RefreshIndicator
          // and any ancestor scroll observers.
          if (notification is ScrollUpdateNotification ||
              notification is ScrollEndNotification) {
            _maybeLoadMore(notification.metrics, feedState);
          }
          return false;
        },
        child: CustomScrollView(
          slivers: [
            // Upload progress indicator
            const SliverToBoxAdapter(child: UploadProgressWidget()),

            // Refresh update indicator: thin progress while last-known-good
            // items stay visible. Never a full-page loading swap.
            if (feedState.isRefreshing)
              const SliverToBoxAdapter(
                child: LinearProgressIndicator(minHeight: 2),
              ),

            // Refresh failure indication: old data stays, failure renders
            // inline with retry. Never a full-page error swap.
            if (feedState.refreshError != null)
              SliverToBoxAdapter(child: _buildRefreshErrorBanner(feedState)),

            // Feed items
            SliverList(
              delegate: SliverChildBuilderDelegate((context, index) {
                final item = feedItems[index];
                return FeedCardFactory.buildCardForFeedItem(item, ref);
              }, childCount: feedItems.length),
            ),

            // Bottom pagination spinner. Rendered inside the scroll view
            // so it appears beneath the last loaded card without shifting
            // layout. Stays hidden when no fetch is in flight.
            if (feedState.isLoadingMore)
              const SliverToBoxAdapter(
                child: Padding(
                  padding: EdgeInsets.symmetric(vertical: AppMetrics.p16),
                  child: Center(child: LoadingIndicator.small()),
                ),
              ),
          ],
        ),
      ),
    );
  }

  /// Minimum bounded refresh-failure indication: persistent inline banner
  /// with safe localized copy ([pageErrorMessage]) and a retry action.
  /// Not a new foundation — composition of canonical tokens for this screen.
  Widget _buildRefreshErrorBanner(FeedState feedState) {
    final scheme = Theme.of(context).colorScheme;
    final l10n = context.l10n;
    return Semantics(
      container: true,
      liveRegion: true,
      child: Container(
        width: double.infinity,
        margin: const EdgeInsets.fromLTRB(
          AppMetrics.p16,
          AppMetrics.p12,
          AppMetrics.p16,
          AppMetrics.p4,
        ),
        padding: const EdgeInsets.symmetric(
          horizontal: AppMetrics.p12,
          vertical: AppMetrics.p8,
        ),
        decoration: BoxDecoration(
          color: scheme.errorContainer,
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(color: scheme.error),
        ),
        child: Row(
          children: [
            Icon(
              Icons.refresh_outlined,
              size: AppIconSize.action,
              color: scheme.onErrorContainer,
            ),
            const SizedBox(width: AppMetrics.p8),
            Expanded(
              child: Text(
                l10n.pageErrorMessage,
                style: context.typeRoles.bodyDense.copyWith(
                  color: scheme.onErrorContainer,
                ),
              ),
            ),
            TextButton(
              onPressed: feedState.isRefreshing
                  ? null
                  : () => ref.read(feedProvider.notifier).refresh(),
              child: Text(l10n.retryAction),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildEmptyState() {
    final l10n = context.l10n;

    // First-use empty: the canonical state carries exactly ONE primary
    // action (explore the marketplace). "Buat Konten" stays a secondary
    // affordance rendered outside the state, so no entry point is lost.
    return Column(
      children: [
        Expanded(
          child: EmptyState(
            icon: Icons.emoji_emotions_outlined,
            title: l10n.homeFirstUseTitle,
            actionLabel: l10n.exploreMarketplaceAction,
            onAction: () => _navigateToMarketplace(context),
          ),
        ),
        Padding(
          padding: const EdgeInsets.only(bottom: AppMetrics.p32),
          child: _buildSecondaryActionButton(
            icon: Icons.post_add_outlined,
            label: 'Buat Konten',
            onTap: () => _navigateToCreateContent(context),
          ),
        ),
      ],
    );
  }

  /// Secondary action button - outlined style
  Widget _buildSecondaryActionButton({
    required IconData icon,
    required String label,
    required VoidCallback onTap,
  }) {
    final scheme = Theme.of(context).colorScheme;

    return SizedBox(
      width: AppContentSize.actionWidth,
      child: OutlinedButton.icon(
        icon: Icon(icon, size: AppIconSize.action),
        label: Text(
          label,
          style: Theme.of(context).textTheme.labelLarge?.copyWith(
            fontWeight: FontWeight.w500,
            color: scheme.onSurface,
          ),
        ),
        onPressed: onTap,
        style: OutlinedButton.styleFrom(
          padding: const EdgeInsets.symmetric(
            vertical: AppMetrics.p16,
            horizontal: AppMetrics.p24,
          ),
        ),
      ),
    );
  }

  void _navigateToCreateContent(BuildContext context) {
    // Trigger create content bottom sheet via navigation
    context.push(RoutePaths.createContent);
  }

  void _navigateToMarketplace(BuildContext context) {
    context.push(RoutePaths.forSales);
  }

  /// CANONICAL page-level load error (PageErrorState). The raw feed
  /// [FeedState.errorMessage] never reaches the screen — safe localized copy
  /// only; the state value stays provider-side evidence for logging.
  Widget _buildError() {
    return PageErrorState(onRetry: () => ref.refresh(feedProvider));
  }
}
