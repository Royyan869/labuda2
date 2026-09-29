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
/// - NO commerce objects (For Sale items, auctions, contests) - those belong in Marketplace
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
          precacheImage(NetworkImage(url), context).then(
            (_) {},
            onError: (_) {},
          ),
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
    // Handle loading, error, and data states
    if (feedState.errorMessage != null) {
      return _buildError(feedState.errorMessage!);
    }

    if (feedState.isLoading && feedState.items.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    final feedItems = feedState.items;

    if (feedItems.isEmpty) {
      return _buildEmptyState();
    }

    return RefreshIndicator(
      onRefresh: () async {
        resetPromotionExposureAttempts();
        ref.invalidate(feedProvider);
        await Future.delayed(AppMotion.quick);
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
                  child: Center(child: CircularProgressIndicator()),
                ),
              ),

            // Add spacing at bottom
            const SliverToBoxAdapter(child: SizedBox(height: 100)),
          ],
        ),
      ),
    );
  }

  Widget _buildEmptyState() {
    final scheme = Theme.of(context).colorScheme;

    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            // Icon with friendly emoji
            Container(
              width: 100,
              height: 100,
              decoration: BoxDecoration(
                color: scheme.primary.withValues(alpha: 0.1),
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.emoji_emotions_outlined,
                size: 48,
                color: scheme.primary,
              ),
            ),
            const SizedBox(height: 24),

            // Decision-based title
            Text(
              '🎯 Kamu ingin apa hari ini?',
              style: TextStyle(
                fontSize: AppType.s20,
                fontWeight: FontWeight.w700,
                color: scheme.onSurface,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 32),

            // PRIMARY ACTION: Cari & Beli Koi
            _buildPrimaryActionButton(
              icon: Icons.shopping_bag_outlined,
              label: 'Cari & Beli Koi',
              onTap: () => _navigateToMarketplace(context),
            ),
            const SizedBox(height: 12),

            // SECONDARY ACTION: Universal content composer
            _buildSecondaryActionButton(
              icon: Icons.post_add_outlined,
              label: 'Buat Konten',
              onTap: () => _navigateToCreateContent(context),
            ),
          ],
        ),
      ),
    );
  }

  /// Primary action button - filled style for main action
  Widget _buildPrimaryActionButton({
    required IconData icon,
    required String label,
    required VoidCallback onTap,
  }) {
    return SizedBox(
      width: 280,
      child: FilledButton.icon(
        icon: Icon(icon, size: 22),
        label: Text(
          label,
          style: const TextStyle(fontSize: AppType.s16, fontWeight: FontWeight.w600),
        ),
        onPressed: onTap,
        style: FilledButton.styleFrom(
          padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16, horizontal: AppMetrics.p24),
        ),
      ),
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
      width: 280,
      child: OutlinedButton.icon(
        icon: Icon(icon, size: 20),
        label: Text(
          label,
          style: TextStyle(
            fontSize: AppType.s15,
            fontWeight: FontWeight.w500,
            color: scheme.onSurface,
          ),
        ),
        onPressed: onTap,
        style: OutlinedButton.styleFrom(
          padding: const EdgeInsets.symmetric(vertical: AppMetrics.p14, horizontal: AppMetrics.p24),
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

  Widget _buildError(String error) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.error_outline,
            size: 64,
            color: scheme.error,
          ),
          const SizedBox(height: 16),
          const Text(
            'Feed belum bisa dimuat',
            style: TextStyle(fontSize: AppType.s18, fontWeight: FontWeight.w500),
          ),
          const SizedBox(height: 8),
          Text(
            error,
            style: TextStyle(
              fontSize: AppType.s14,
              color: scheme.onSurfaceVariant,
            ),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 16),
          ElevatedButton(
            onPressed: () => ref.refresh(feedProvider),
            child: const Text('Coba Lagi'),
          ),
        ],
      ),
    );
  }
}
